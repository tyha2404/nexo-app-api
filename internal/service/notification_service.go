package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"go.uber.org/zap"
)

type NotificationService interface {
	GetVapidPublicKey() string
	Subscribe(ctx context.Context, userID uuid.UUID, req *dto.SubscribePushRequest) error
	Unsubscribe(ctx context.Context, req *dto.UnsubscribePushRequest) error
	SendPushToUser(ctx context.Context, userID uuid.UUID, payload *dto.PushNotificationPayload) (int, error)
	SendTestPush(ctx context.Context, userID uuid.UUID, req *dto.TestPushRequest) error
}

type notificationService struct {
	repo            repository.PushSubscriptionRepository
	vapidPublicKey  string
	vapidPrivateKey string
	vapidSubject    string
	logger          *zap.Logger
}

func NewNotificationService(
	repo repository.PushSubscriptionRepository,
	cfg *config.Config,
	logger *zap.Logger,
) NotificationService {
	return &notificationService{
		repo:            repo,
		vapidPublicKey:  cfg.VapidPublicKey,
		vapidPrivateKey: cfg.VapidPrivateKey,
		vapidSubject:    cfg.VapidSubject,
		logger:          logger,
	}
}

func (s *notificationService) GetVapidPublicKey() string {
	return s.vapidPublicKey
}

func (s *notificationService) Subscribe(ctx context.Context, userID uuid.UUID, req *dto.SubscribePushRequest) error {
	deviceType := req.DeviceType
	if deviceType == "" {
		deviceType = "ios"
	}

	sub := &model.PushSubscription{
		UserID:     userID,
		Endpoint:   req.Endpoint,
		P256dh:     req.Keys.P256dh,
		Auth:       req.Keys.Auth,
		UserAgent:  req.UserAgent,
		DeviceType: deviceType,
	}

	if err := s.repo.Upsert(ctx, sub); err != nil {
		s.logger.Error("failed to upsert push subscription", zap.Error(err), zap.String("userID", userID.String()))
		return fmt.Errorf("không thể lưu đăng ký thông báo: %w", err)
	}

	s.logger.Info("push subscription registered successfully", zap.String("userID", userID.String()), zap.String("deviceType", deviceType))
	return nil
}

func (s *notificationService) Unsubscribe(ctx context.Context, req *dto.UnsubscribePushRequest) error {
	if req.Endpoint == "" {
		return nil
	}
	return s.repo.DeleteByEndpoint(ctx, req.Endpoint)
}

func (s *notificationService) SendPushToUser(ctx context.Context, userID uuid.UUID, payload *dto.PushNotificationPayload) (int, error) {
	subs, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to list subscriptions: %w", err)
	}

	if len(subs) == 0 {
		s.logger.Warn("no active push subscriptions for user", zap.String("userID", userID.String()))
		return 0, nil
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal push payload: %w", err)
	}

	successCount := 0
	var lastErr error
	for _, subRecord := range subs {
		sub := &webpush.Subscription{
			Endpoint: subRecord.Endpoint,
			Keys: webpush.Keys{
				P256dh: subRecord.P256dh,
				Auth:   subRecord.Auth,
			},
		}

		resp, err := webpush.SendNotification(payloadJSON, sub, &webpush.Options{
			Subscriber:      s.vapidSubject,
			VAPIDPublicKey:  s.vapidPublicKey,
			VAPIDPrivateKey: s.vapidPrivateKey,
			TTL:             60,
		})

		if err != nil {
			s.logger.Warn("failed to send web push notification",
				zap.Error(err),
				zap.String("endpoint", subRecord.Endpoint),
				zap.String("userID", userID.String()),
			)
			lastErr = err
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		// Handle expired or stale subscription (404 Not Found or 410 Gone)
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
			s.logger.Info("deleting stale subscription", zap.String("endpoint", subRecord.Endpoint))
			_ = s.repo.DeleteByEndpoint(ctx, subRecord.Endpoint)
		} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			successCount++
		} else {
			respBodyStr := string(respBody)
			s.logger.Warn("web push gateway responded with error status",
				zap.Int("statusCode", resp.StatusCode),
				zap.String("responseBody", respBodyStr),
				zap.String("endpoint", subRecord.Endpoint),
				zap.String("userID", userID.String()),
			)
			if respBodyStr != "" {
				lastErr = fmt.Errorf("cổng push trả về lỗi HTTP %d: %s", resp.StatusCode, respBodyStr)
			} else {
				lastErr = fmt.Errorf("cổng push trả về mã lỗi HTTP %d", resp.StatusCode)
			}
		}
	}

	if successCount == 0 && len(subs) > 0 && lastErr != nil {
		return 0, fmt.Errorf("không thể gửi thông báo tới thiết bị (%w). Vui lòng thử tắt và bật lại thông báo để làm mới đăng ký", lastErr)
	}

	return successCount, nil
}

func (s *notificationService) SendTestPush(ctx context.Context, userID uuid.UUID, req *dto.TestPushRequest) error {
	title := req.Title
	if title == "" {
		title = "Nexo Financial - Thông báo thử nghiệm"
	}
	body := req.Body
	if body == "" {
		body = "🎉 Tuyệt vời! Thiết bị iPhone của bạn đã kết nối Web Push Notification thành công."
	}
	url := req.URL
	if url == "" {
		url = "/"
	}

	payload := &dto.PushNotificationPayload{
		Title: title,
		Body:  body,
		URL:   url,
		Tag:   "nexo-test-push",
	}

	count, err := s.SendPushToUser(ctx, userID, payload)
	if err != nil {
		return err
	}

	if count == 0 {
		return fmt.Errorf("chưa tìm thấy thiết bị nào đã đăng ký nhận thông báo của bạn. Vui lòng tắt và bật lại nút thông báo")
	}

	return nil
}
