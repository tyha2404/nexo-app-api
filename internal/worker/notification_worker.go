package worker

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"github.com/tyha2404/nexo-app-api/internal/service"
)

type NotificationWorker struct {
	cron         *cron.Cron
	notifService service.NotificationService
	stmtRepo     repository.CreditCardStatementRepository
	debtRepo     repository.DebtRepository
	targetRepo   repository.TargetRepository
	txRepo       repository.TransactionRepository
	pushSubRepo  repository.PushSubscriptionRepository
	logger       *zap.Logger

	mu          sync.RWMutex
	sentHistory map[string]time.Time
}

func NewNotificationWorker(
	notifService service.NotificationService,
	stmtRepo repository.CreditCardStatementRepository,
	debtRepo repository.DebtRepository,
	targetRepo repository.TargetRepository,
	txRepo repository.TransactionRepository,
	pushSubRepo repository.PushSubscriptionRepository,
	logger *zap.Logger,
) *NotificationWorker {
	return &NotificationWorker{
		cron:         cron.New(),
		notifService: notifService,
		stmtRepo:     stmtRepo,
		debtRepo:     debtRepo,
		targetRepo:   targetRepo,
		txRepo:       txRepo,
		pushSubRepo:  pushSubRepo,
		logger:       logger,
		sentHistory:  make(map[string]time.Time),
	}
}

func (w *NotificationWorker) hasSentToday(key string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	lastSent, exists := w.sentHistory[key]
	if !exists {
		return false
	}
	// Expire keys older than 20 hours
	return time.Since(lastSent) < 20*time.Hour
}

func (w *NotificationWorker) markSent(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.sentHistory[key] = time.Now()

	// Clean up very old keys
	if len(w.sentHistory) > 1000 {
		cutoff := time.Now().Add(-24 * time.Hour)
		for k, v := range w.sentHistory {
			if v.Before(cutoff) {
				delete(w.sentHistory, k)
			}
		}
	}
}

func formatVND(amount float64) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}
	intPart := int64(math.Round(amount))
	str := fmt.Sprintf("%d", intPart)
	n := len(str)
	if n <= 3 {
		return sign + str + " ₫"
	}
	var res []byte
	rem := n % 3
	if rem > 0 {
		res = append(res, str[:rem]...)
	}
	for i := rem; i < n; i += 3 {
		if len(res) > 0 {
			res = append(res, '.')
		}
		res = append(res, str[i:i+3]...)
	}
	return sign + string(res) + " ₫"
}

// Start registers the cron jobs and runs a light startup check
func (w *NotificationWorker) Start(ctx context.Context) {
	// 1. Check Credit Card Statements & Debt Due Dates daily at 08:30
	_, err := w.cron.AddFunc("30 8 * * *", func() {
		w.logger.Info("running morning notification worker (Credit Cards & Debts)...")
		w.ProcessCreditCardDueReminders(context.Background())
		w.ProcessDebtDueReminders(context.Background())
	})
	if err != nil {
		w.logger.Error("failed to schedule morning notification cron", zap.Error(err))
	}

	// 2. Check Budget & Spending Overrun daily at 12:00 and 18:00
	_, err = w.cron.AddFunc("0 12,18 * * *", func() {
		w.logger.Info("running budget & target overrun notification worker...")
		w.ProcessBudgetOverrunAlerts(context.Background())
	})
	if err != nil {
		w.logger.Error("failed to schedule budget overrun cron", zap.Error(err))
	}

	// 3. Evening Expense Logging Reminder daily at 21:00
	_, err = w.cron.AddFunc("0 21 * * *", func() {
		w.logger.Info("running evening expense logging reminder worker...")
		w.ProcessDailyExpenseLoggingReminder(context.Background())
	})
	if err != nil {
		w.logger.Error("failed to schedule evening logging reminder cron", zap.Error(err))
	}

	w.cron.Start()
	w.logger.Info("notification workers scheduled successfully (08:30, 12:00, 18:00, 21:00)")

	// Background startup check after a brief delay
	go func() {
		time.Sleep(5 * time.Second)
		w.logger.Info("running startup notification check...")
		w.ProcessCreditCardDueReminders(ctx)
		w.ProcessDebtDueReminders(ctx)
	}()
}

// Stop gracefully stops the worker cron
func (w *NotificationWorker) Stop() {
	w.logger.Info("stopping notification worker...")
	ctx := w.cron.Stop()
	<-ctx.Done()
	w.logger.Info("notification worker stopped")
}

// ProcessCreditCardDueReminders checks all credit card statements and reminds users on 3d, 1d, 0d, or overdue
func (w *NotificationWorker) ProcessCreditCardDueReminders(ctx context.Context) {
	userIDs, err := w.pushSubRepo.GetSubscribedUserIDs(ctx)
	if err != nil {
		w.logger.Error("failed to fetch subscribed users for cc reminder", zap.Error(err))
		return
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	for _, userID := range userIDs {
		statements, err := w.stmtRepo.List(ctx, userID, nil, nil, nil)
		if err != nil {
			w.logger.Warn("failed to fetch statements for user", zap.String("userID", userID.String()), zap.Error(err))
			continue
		}

		for _, stmt := range statements {
			if stmt.Status == model.StatementStatusPaid {
				continue
			}
			remaining := stmt.StatementBalance - stmt.PaidAmount
			if remaining <= 0 {
				continue
			}

			dueDateMidnight := time.Date(stmt.DueDate.Year(), stmt.DueDate.Month(), stmt.DueDate.Day(), 0, 0, 0, 0, time.Local)
			daysLeft := int(dueDateMidnight.Sub(today).Hours() / 24)

			walletName := "Thẻ tín dụng"
			if stmt.Wallet != nil && stmt.Wallet.Name != "" {
				walletName = stmt.Wallet.Name
			}

			var title, body, tag string
			dueDateStr := stmt.DueDate.Format("02/01/2006")

			switch {
			case daysLeft == 3:
				title = "💳 Nhắc nhở: Sao kê thẻ sắp đến hạn"
				body = fmt.Sprintf("Sao kê %s cần thanh toán %s vào ngày %s (còn 3 ngày).", walletName, formatVND(remaining), dueDateStr)
				tag = fmt.Sprintf("stmt-3d-%s-%s", stmt.ID.String(), today.Format("2006-01-02"))
			case daysLeft == 1:
				title = "⚠️ Khẩn cấp: Hạn thanh toán thẻ vào NGÀY MAI"
				body = fmt.Sprintf("Sao kê %s (%s) sẽ đến hạn vào ngày mai (%s). Hãy thanh toán để tránh phát sinh lãi!", walletName, formatVND(remaining), dueDateStr)
				tag = fmt.Sprintf("stmt-1d-%s-%s", stmt.ID.String(), today.Format("2006-01-02"))
			case daysLeft == 0:
				title = "🚨 HÔM NAY là hạn chót thanh toán sao kê thẻ!"
				body = fmt.Sprintf("Hạn chót thanh toán sao kê %s là hôm nay (%s). Thanh toán ngay để bảo vệ điểm tín dụng CIC!", walletName, formatVND(remaining))
				tag = fmt.Sprintf("stmt-0d-%s-%s", stmt.ID.String(), today.Format("2006-01-02"))
			case daysLeft < 0 && daysLeft >= -7:
				title = "❌ Cảnh báo: Sao kê thẻ đã quá hạn!"
				body = fmt.Sprintf("Sao kê %s đã quá hạn %d ngày (%s). Vui lòng thanh toán gấp để tránh nợ xấu!", walletName, -daysLeft, formatVND(remaining))
				tag = fmt.Sprintf("stmt-overdue-%s-%s", stmt.ID.String(), today.Format("2006-01-02"))
			default:
				continue
			}

			if w.hasSentToday(tag) {
				continue
			}

			payload := &dto.PushNotificationPayload{
				Title: title,
				Body:  body,
				URL:   "/wallets",
				Tag:   tag,
			}

			if count, err := w.notifService.SendPushToUser(ctx, userID, payload); err == nil && count > 0 {
				w.markSent(tag)
				w.logger.Info("sent credit card due reminder", zap.String("userID", userID.String()), zap.String("tag", tag))
			}
		}
	}
}

// ProcessDebtDueReminders sends reminders for upcoming debt/loan due dates
func (w *NotificationWorker) ProcessDebtDueReminders(ctx context.Context) {
	userIDs, err := w.pushSubRepo.GetSubscribedUserIDs(ctx)
	if err != nil {
		w.logger.Error("failed to fetch subscribed users for debt reminder", zap.Error(err))
		return
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	for _, userID := range userIDs {
		debts, err := w.debtRepo.FindByUserID(ctx, userID, "", model.DebtStatusPending)
		if err != nil {
			continue
		}

		for _, d := range debts {
			if d.DueDate == nil {
				continue
			}
			remaining := d.TotalAmount - d.PaidAmount
			if remaining <= 0 {
				continue
			}

			dueDateMidnight := time.Date(d.DueDate.Year(), d.DueDate.Month(), d.DueDate.Day(), 0, 0, 0, 0, time.Local)
			daysLeft := int(dueDateMidnight.Sub(today).Hours() / 24)

			if daysLeft < 0 || daysLeft > 3 {
				continue
			}

			var title, body string
			dueDateStr := d.DueDate.Format("02/01/2006")
			tag := fmt.Sprintf("debt-%s-%d-%s", d.ID.String(), daysLeft, today.Format("2006-01-02"))

			if w.hasSentToday(tag) {
				continue
			}

			debtName := d.Title
			if debtName == "" {
				debtName = "Khoản nợ"
			}

			if d.Type == model.DebtTypePayable {
				if daysLeft == 0 {
					title = "🚨 Hạn trả nợ là hôm nay"
					body = fmt.Sprintf("Khoản nợ '%s' (%s) cần thanh toán hôm nay.", debtName, formatVND(remaining))
				} else {
					title = "📝 Nhắc nhở khoản nợ sắp đến hạn"
					body = fmt.Sprintf("Khoản nợ '%s' (%s) sẽ đến hạn vào ngày %s (còn %d ngày).", debtName, formatVND(remaining), dueDateStr, daysLeft)
				}
			} else {
				if daysLeft == 0 {
					title = "💰 Khoản phải thu đến hạn hôm nay"
					body = fmt.Sprintf("Khoản cho vay '%s' (%s) đến hạn thu hôm nay.", debtName, formatVND(remaining))
				} else {
					title = "💰 Khoản phải thu sắp đến hạn"
					body = fmt.Sprintf("Khoản cho vay '%s' (%s) sẽ đến hạn thu vào ngày %s (còn %d ngày).", debtName, formatVND(remaining), dueDateStr, daysLeft)
				}
			}

			payload := &dto.PushNotificationPayload{
				Title: title,
				Body:  body,
				URL:   "/debts",
				Tag:   tag,
			}

			if count, err := w.notifService.SendPushToUser(ctx, userID, payload); err == nil && count > 0 {
				w.markSent(tag)
				w.logger.Info("sent debt reminder", zap.String("userID", userID.String()), zap.String("tag", tag))
			}
		}
	}
}

// ProcessBudgetOverrunAlerts alerts users if they exceed 80% or 100% of their monthly budget target
func (w *NotificationWorker) ProcessBudgetOverrunAlerts(ctx context.Context) {
	userIDs, err := w.pushSubRepo.GetSubscribedUserIDs(ctx)
	if err != nil {
		w.logger.Error("failed to fetch subscribed users for budget alert", zap.Error(err))
		return
	}

	now := time.Now()
	month := int(now.Month())
	year := now.Year()
	todayStr := now.Format("2006-01-02")

	startOfMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Nanosecond)

	for _, userID := range userIDs {
		target, err := w.targetRepo.GetTarget(ctx, userID, model.TargetTypeExpense, month, year)
		if err != nil || target == nil || target.TargetAmount <= 0 {
			continue
		}

		spent, err := w.targetRepo.GetMonthlyTotalByCategoryType(ctx, userID, model.CategoryTypeExpense, startOfMonth, endOfMonth)
		if err != nil {
			continue
		}

		ratio := spent / target.TargetAmount
		var title, body, tag string

		if ratio >= 1.0 {
			tag = fmt.Sprintf("budget-overrun-100-%s-%d-%d-%s", userID.String(), year, month, todayStr)
			title = "🚨 Cảnh báo: Vượt 100% hạn mức chi tiêu tháng!"
			body = fmt.Sprintf("Bạn đã chi %s (vượt mức kế hoạch %s). Hãy cân nhắc cắt giảm chi tiêu không thiết yếu!", formatVND(spent), formatVND(target.TargetAmount))
		} else if ratio >= 0.8 {
			tag = fmt.Sprintf("budget-overrun-80-%s-%d-%d-%s", userID.String(), year, month, todayStr)
			title = "⚠️ Cảnh báo: Đã dùng 80% ngân sách tháng"
			body = fmt.Sprintf("Bạn đã chi tiêu %s / %s (%.0f%% ngân sách tháng %d/%d).", formatVND(spent), formatVND(target.TargetAmount), ratio*100, month, year)
		} else {
			continue
		}

		if w.hasSentToday(tag) {
			continue
		}

		payload := &dto.PushNotificationPayload{
			Title: title,
			Body:  body,
			URL:   "/planning/targets",
			Tag:   tag,
		}

		if count, err := w.notifService.SendPushToUser(ctx, userID, payload); err == nil && count > 0 {
			w.markSent(tag)
			w.logger.Info("sent budget alert", zap.String("userID", userID.String()), zap.String("tag", tag))
		}
	}
}

// ProcessDailyExpenseLoggingReminder sends an evening reminder if no transactions have been logged today
func (w *NotificationWorker) ProcessDailyExpenseLoggingReminder(ctx context.Context) {
	userIDs, err := w.pushSubRepo.GetSubscribedUserIDs(ctx)
	if err != nil {
		w.logger.Error("failed to fetch subscribed users for daily logging reminder", zap.Error(err))
		return
	}

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	endOfDay := startOfDay.AddDate(0, 0, 1).Add(-time.Nanosecond)
	todayStr := now.Format("2006-01-02")

	for _, userID := range userIDs {
		tag := fmt.Sprintf("daily-log-reminder-%s-%s", userID.String(), todayStr)
		if w.hasSentToday(tag) {
			continue
		}

		count, err := w.txRepo.CountTransactionsInRange(ctx, userID, startOfDay, endOfDay)
		if err != nil {
			continue
		}

		if count == 0 {
			payload := &dto.PushNotificationPayload{
				Title: "🌙 Nhắc nhở ghi chép chi tiêu cuối ngày",
				Body:  "Hôm nay bạn có phát sinh khoản chi tiêu nào chưa ghi chép vào Nexo không? Dành 1 phút cập nhật nhé!",
				URL:   "/transactions",
				Tag:   tag,
			}

			if sentCount, err := w.notifService.SendPushToUser(ctx, userID, payload); err == nil && sentCount > 0 {
				w.markSent(tag)
				w.logger.Info("sent daily expense logging reminder", zap.String("userID", userID.String()), zap.String("tag", tag))
			}
		}
	}
}
