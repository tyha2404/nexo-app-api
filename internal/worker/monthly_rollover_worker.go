package worker

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/tyha2404/nexo-app-api/internal/service"
)

type MonthlyRolloverWorker struct {
	cron    *cron.Cron
	service service.RolloverService
	logger  *zap.Logger
}

func NewMonthlyRolloverWorker(service service.RolloverService, logger *zap.Logger) *MonthlyRolloverWorker {
	return &MonthlyRolloverWorker{
		cron:    cron.New(),
		service: service,
		logger:  logger,
	}
}

// IsEndOfMonthTransition checks if the given timestamp is at 23:50 on the last day of the month.
func IsEndOfMonthTransition(t time.Time) (bool, int, int) {
	if t.Hour() != 23 || t.Minute() != 50 {
		return false, 0, 0
	}

	nextMonthTime := t.Add(15 * time.Minute)
	if nextMonthTime.Month() != t.Month() {
		return true, int(nextMonthTime.Month()), nextMonthTime.Year()
	}

	return false, 0, 0
}

// Start registers the cron schedule and triggers initial startup sync
func (w *MonthlyRolloverWorker) Start(ctx context.Context) {
	// 1. Startup Backfill for current month
	go func() {
		w.logger.Info("running startup sync for monthly targets and budgets...")
		now := time.Now()
		if err := w.service.ProcessRolloverForMonth(ctx, int(now.Month()), now.Year()); err != nil {
			w.logger.Error("startup rollover sync encountered error", zap.Error(err))
		} else {
			w.logger.Info("startup rollover sync completed successfully")
		}
	}()

	// 2. Schedule daily check at 23:50
	_, err := w.cron.AddFunc("50 23 * * *", func() {
		now := time.Now()
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
