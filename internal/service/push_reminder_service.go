package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"go.uber.org/zap"
)

const (
	// debtReminderDays: debts due today or tomorrow.
	debtReminderDays = 1
	// statementReminderDays: credit-card statements due within 2 days.
	statementReminderDays = 2
)

// PushReminderService sends the daily due-date reminders.
type PushReminderService interface {
	// SendDailyReminders notifies due debts and credit-card statements for the
	// app-timezone day of `now`. Each item is notified at most once per day.
	// Returns how many notifications were sent.
	SendDailyReminders(ctx context.Context, now time.Time) (int, error)
}

type pushReminderService struct {
	repo   repository.ReminderRepository
	push   PushService
	logger *zap.Logger

	mu   sync.Mutex
	day  string
	sent map[string]struct{}
}

func NewPushReminderService(repo repository.ReminderRepository, push PushService, logger *zap.Logger) PushReminderService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &pushReminderService{repo: repo, push: push, logger: logger, sent: map[string]struct{}{}}
}

// markSent records key for today and reports whether it was new.
func (s *pushReminderService) markSent(day, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.day != day {
		s.day = day
		s.sent = map[string]struct{}{}
	}
	if _, ok := s.sent[key]; ok {
		return false
	}
	s.sent[key] = struct{}{}
	return true
}

func (s *pushReminderService) unmark(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sent, key)
}

func dueLabel(due, today time.Time) string {
	switch util.DateString(due) {
	case util.DateString(today):
		return "hôm nay"
	case util.DateString(today.AddDate(0, 0, 1)):
		return "ngày mai"
	default:
		return "ngày " + due.In(util.AppLocation()).Format("02/01")
	}
}

// DebtReminderPayload builds the notification for a debt due soon.
func DebtReminderPayload(d model.Debt, today time.Time) dto.PushPayload {
	remaining := d.TotalAmount - d.PaidAmount
	label := dueLabel(*d.DueDate, today)
	title := "Khoản nợ đến hạn " + label
	body := fmt.Sprintf("%s: còn %s ₫ cần trả", d.Title, formatVND(remaining))
	if d.Type == model.DebtTypeReceivable {
		title = "Khoản cho vay đến hạn thu " + label
		body = fmt.Sprintf("%s: còn %s ₫ cần thu", d.Title, formatVND(remaining))
	}
	return dto.PushPayload{Title: title, Body: body, URL: "/#/debts", Tag: "debt-" + d.ID.String()}
}

// StatementReminderPayload builds the notification for a statement due soon.
func StatementReminderPayload(st model.CreditCardStatement, today time.Time) dto.PushPayload {
	card := "Thẻ tín dụng"
	if st.Wallet != nil && st.Wallet.Name != "" {
		card = st.Wallet.Name
	}
	remaining := st.StatementBalance - st.PaidAmount
	return dto.PushPayload{
		Title: "Sao kê thẻ tín dụng đến hạn " + dueLabel(st.DueDate, today),
		Body: fmt.Sprintf("%s: còn %s ₫ (tối thiểu %s ₫), hạn %s", card, formatVND(remaining),
			formatVND(st.MinimumPayment), st.DueDate.Format("02/01")),
		URL: "/#/",
		Tag: "statement-" + st.ID.String(),
	}
}

func (s *pushReminderService) SendDailyReminders(ctx context.Context, now time.Time) (int, error) {
	if s.push == nil || !s.push.Enabled() {
		return 0, ErrPushDisabled
	}
	today := util.StartOfDay(now)
	day := util.DateString(today)

	debts, err := s.repo.ListDebtsDueBetween(ctx, day, util.DateString(today.AddDate(0, 0, debtReminderDays)))
	if err != nil {
		return 0, fmt.Errorf("list due debts: %w", err)
	}
	stmts, err := s.repo.ListStatementsDueBetween(ctx, day, util.DateString(today.AddDate(0, 0, statementReminderDays)))
	if err != nil {
		return 0, fmt.Errorf("list due statements: %w", err)
	}

	type reminder struct {
		key     string
		userID  uuid.UUID
		payload dto.PushPayload
	}
	reminders := make([]reminder, 0, len(debts)+len(stmts))
	for _, d := range debts {
		if d.DueDate != nil {
			reminders = append(reminders, reminder{"debt:" + d.ID.String(), d.UserID, DebtReminderPayload(d, today)})
		}
	}
	for _, st := range stmts {
		reminders = append(reminders, reminder{"statement:" + st.ID.String(), st.UserID, StatementReminderPayload(st, today)})
	}

	sent := 0
	var errs []error
	for _, r := range reminders {
		if !s.markSent(day, r.key) {
			continue
		}
		if _, err := s.push.SendToUser(ctx, r.userID, r.payload); err != nil {
			s.unmark(r.key) // allow a retry in a later run today
			errs = append(errs, err)
			continue
		}
		sent++
	}

	if len(errs) > 0 {
		s.logger.Warn("some push reminders failed", zap.Int("failed", len(errs)), zap.Error(errors.Join(errs...)))
	}
	return sent, errors.Join(errs...)
}
