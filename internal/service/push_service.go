package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"go.uber.org/zap"
)

// ErrPushDisabled is returned when VAPID keys are not configured.
var ErrPushDisabled = errors.New("web push is not configured")

const (
	// pushSendTimeout bounds one background fan-out to all of a user's devices.
	pushSendTimeout = 15 * time.Second
	// pushTTLSeconds is how long the push service keeps an undelivered message.
	pushTTLSeconds = 24 * 60 * 60
	// maxPushTextRunes keeps title/body well under the 4KB payload limit.
	maxPushTextRunes = 200
)

// PushSender delivers one encrypted message to one subscription and returns
// the push service's HTTP status code.
type PushSender interface {
	Send(ctx context.Context, sub model.PushSubscription, payload []byte) (int, error)
}

// VAPIDConfig holds the application server keys.
type VAPIDConfig struct {
	PublicKey  string
	PrivateKey string
	Subject    string
}

// Enabled reports whether both keys are present.
func (c VAPIDConfig) Enabled() bool { return c.PublicKey != "" && c.PrivateKey != "" }

type webPushSender struct {
	cfg    VAPIDConfig
	client *http.Client
}

// NewWebPushSender returns a PushSender backed by webpush-go.
func NewWebPushSender(cfg VAPIDConfig) PushSender {
	return &webPushSender{cfg: cfg, client: &http.Client{Timeout: 10 * time.Second}}
}

func (s *webPushSender) Send(ctx context.Context, sub model.PushSubscription, payload []byte) (int, error) {
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{Auth: sub.Auth, P256dh: sub.P256dh},
	}, &webpush.Options{
		HTTPClient:      s.client,
		Subscriber:      s.cfg.Subject,
		VAPIDPublicKey:  s.cfg.PublicKey,
		VAPIDPrivateKey: s.cfg.PrivateKey,
		TTL:             pushTTLSeconds,
		Urgency:         webpush.UrgencyHigh,
	})
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("push service responded %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

// PushService manages subscriptions and sends notifications.
type PushService interface {
	Enabled() bool
	PublicKey() (string, error)
	Subscribe(ctx context.Context, userID uuid.UUID, req dto.SubscribePushRequest, userAgent string) error
	Unsubscribe(ctx context.Context, userID uuid.UUID, endpoint string) error
	// SendToUser delivers the payload to every device of the user and returns
	// how many deliveries succeeded. Subscriptions answered with 404/410 are deleted.
	SendToUser(ctx context.Context, userID uuid.UUID, payload dto.PushPayload) (int, error)
	// NotifyUser sends in the background with a timeout; it never blocks the
	// caller and only logs failures. No-op when push is disabled.
	NotifyUser(userID uuid.UUID, payload dto.PushPayload)
	// Wait blocks until background sends finish (tests and graceful shutdown).
	Wait()
}

type pushService struct {
	repo   repository.PushSubscriptionRepository
	sender PushSender
	cfg    VAPIDConfig
	logger *zap.Logger
	wg     sync.WaitGroup
}

func NewPushService(repo repository.PushSubscriptionRepository, sender PushSender, cfg VAPIDConfig, logger *zap.Logger) PushService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &pushService{repo: repo, sender: sender, cfg: cfg, logger: logger}
}

func (s *pushService) Enabled() bool {
	return s != nil && s.cfg.Enabled() && s.repo != nil && s.sender != nil
}

func (s *pushService) PublicKey() (string, error) {
	if !s.Enabled() {
		return "", ErrPushDisabled
	}
	return s.cfg.PublicKey, nil
}

func detectDeviceType(userAgent string) string {
	ua := strings.ToLower(userAgent)
	switch {
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		return "ios"
	case strings.Contains(ua, "android"):
		return "android"
	default:
		return "web"
	}
}

func (s *pushService) Subscribe(ctx context.Context, userID uuid.UUID, req dto.SubscribePushRequest, userAgent string) error {
	if !s.Enabled() {
		return ErrPushDisabled
	}
	sub := &model.PushSubscription{
		UserID:     userID,
		Endpoint:   strings.TrimSpace(req.Endpoint),
		P256dh:     strings.TrimSpace(req.Keys.P256dh),
		Auth:       strings.TrimSpace(req.Keys.Auth),
		DeviceType: detectDeviceType(userAgent),
	}
	if ua := strings.TrimSpace(userAgent); ua != "" {
		if len(ua) > 512 {
			ua = ua[:512]
		}
		sub.UserAgent = &ua
	}
	return s.repo.Upsert(ctx, sub)
}

func (s *pushService) Unsubscribe(ctx context.Context, userID uuid.UUID, endpoint string) error {
	// Allowed even when push is disabled so clients can always clean up.
	if s.repo == nil {
		return nil
	}
	return s.repo.DeleteForUser(ctx, userID, strings.TrimSpace(endpoint))
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func (s *pushService) SendToUser(ctx context.Context, userID uuid.UUID, payload dto.PushPayload) (int, error) {
	if !s.Enabled() {
		return 0, ErrPushDisabled
	}
	payload.Title = truncateRunes(payload.Title, maxPushTextRunes)
	payload.Body = truncateRunes(payload.Body, maxPushTextRunes)
	if payload.URL == "" {
		payload.URL = "/#/"
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	subs, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return 0, err
	}
	delivered := 0
	var errs []error
	for _, sub := range subs {
		status, sendErr := s.sender.Send(ctx, sub, body)
		if status == http.StatusNotFound || status == http.StatusGone {
			// The browser unsubscribed or the subscription expired.
			if delErr := s.repo.DeleteByEndpoint(ctx, sub.Endpoint); delErr != nil {
				errs = append(errs, delErr)
			}
			continue
		}
		if sendErr != nil {
			errs = append(errs, sendErr)
			continue
		}
		delivered++
	}
	return delivered, errors.Join(errs...)
}

func (s *pushService) NotifyUser(userID uuid.UUID, payload dto.PushPayload) {
	if !s.Enabled() {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("push notify panicked", zap.Any("panic", r))
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), pushSendTimeout)
		defer cancel()
		if _, err := s.SendToUser(ctx, userID, payload); err != nil {
			s.logger.Warn("push notification failed", zap.String("user_id", userID.String()), zap.Error(err))
		}
	}()
}

func (s *pushService) Wait() { s.wg.Wait() }
