package worker

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/tyha2404/nexo-app-api/internal/service"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

type MonthlyRolloverWorker struct {
	cron    *cron.Cron
	service service.RolloverService
	logger  *zap.Logger
}

func NewMonthlyRolloverWorker(service service.RolloverService, logger *zap.Logger) *MonthlyRolloverWorker {
	cronLog := cronLogger{logger: logger}
	return &MonthlyRolloverWorker{
		// Run the schedule in the business timezone, not the server's (often UTC).
		// cron.Recover logs a panicking job (with stack) instead of crashing
		// the whole API process.
		cron: cron.New(
			cron.WithLocation(util.AppLocation()),
			cron.WithLogger(cronLog),
			cron.WithChain(cron.Recover(cronLog)),
		),
		service: service,
		logger:  logger,
	}
}

// cronLogger adapts zap to cron.Logger. cron's routine Info messages
// (schedule, wake, run) go to Debug; errors and recovered panics to Error.
type cronLogger struct {
	logger *zap.Logger
}

func (l cronLogger) Info(msg string, keysAndValues ...interface{}) {
	l.logger.Sugar().Debugw("cron: "+msg, keysAndValues...)
}

func (l cronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	l.logger.Sugar().Errorw("cron: "+msg, append([]interface{}{"error", err}, keysAndValues...)...)
}

// recoverPanic logs a panic of a background task (with stack) so it does not
// crash the API process. Use it as `defer w.recoverPanic("...")`.
func (w *MonthlyRolloverWorker) recoverPanic(task string) {
	if rec := recover(); rec != nil {
		w.logger.Error("panic recovered in background task",
			zap.String("task", task),
			zap.Any("panic", rec),
			zap.Stack("stack"),
		)
	}
}

// IsEndOfMonthTransition checks if the given timestamp is at 23:50 on the last
// day of the month in the app timezone.
func IsEndOfMonthTransition(t time.Time) (bool, int, int) {
	t = t.In(util.AppLocation())
	if t.Hour() != 23 || t.Minute() != 50 {
		return false, 0, 0
	}

	_, nextMonthStart := util.MonthRange(t)
	if !t.Add(15 * time.Minute).Before(nextMonthStart) {
		return true, int(nextMonthStart.Month()), nextMonthStart.Year()
	}

	return false, 0, 0
}

// Start registers the cron schedule and triggers initial startup sync
func (w *MonthlyRolloverWorker) Start(ctx context.Context) {
	// 1. Startup Backfill for current month
	go func() {
		defer w.recoverPanic("startup rollover sync")
		w.logger.Info("running startup sync for monthly targets and budgets...")
		now := util.Now()
		if err := w.service.ProcessRolloverForMonth(ctx, int(now.Month()), now.Year()); err != nil {
			w.logger.Error("startup rollover sync encountered error", zap.Error(err))
		} else {
			w.logger.Info("startup rollover sync completed successfully")
		}
	}()

	// 2. Schedule daily check at 23:50
	_, err := w.cron.AddFunc("50 23 * * *", func() {
		now := util.Now()
		isTransition, targetMonth, targetYear := IsEndOfMonthTransition(now)
		if isTransition {
			w.logger.Info("end-of-month detected (23:50), triggering next month rollover",
				zap.Int("target_month", targetMonth),
				zap.Int("target_year", targetYear),
			)
			if err := w.service.ProcessRolloverForMonth(context.Background(), targetMonth, targetYear); err != nil {
				w.logger.Error("failed to process scheduled monthly rollover", zap.Error(err))
			}
		}
	})

	if err != nil {
		w.logger.Error("failed to schedule monthly rollover cron job", zap.Error(err))
		return
	}

	w.cron.Start()
	w.logger.Info("monthly rollover worker started with schedule: 50 23 * * *")
}

// Stop gracefully shuts down the cron runner
func (w *MonthlyRolloverWorker) Stop() {
	w.logger.Info("stopping monthly rollover worker...")
	ctx := w.cron.Stop()
	<-ctx.Done()
	w.logger.Info("monthly rollover worker stopped")
}
