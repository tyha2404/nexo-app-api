package worker

import (
	"context"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/tyha2404/nexo-app-api/internal/service"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

// DailyReminderSchedule runs the due-date push reminders at 08:00 app time.
const DailyReminderSchedule = "0 8 * * *"

// dailyReminderTimeout bounds one reminder run.
const dailyReminderTimeout = 2 * time.Minute

// DailyReminderWorker sends Web Push reminders for debts and credit-card
// statements that are due soon.
type DailyReminderWorker struct {
	cron    *cron.Cron
	service service.PushReminderService
	logger  *zap.Logger
}

func NewDailyReminderWorker(svc service.PushReminderService, logger *zap.Logger) *DailyReminderWorker {
	return &DailyReminderWorker{
		cron:    cron.New(cron.WithLocation(util.AppLocation())),
		service: svc,
		logger:  logger,
	}
}

// Run executes one reminder pass (exported for tests and manual triggers).
func (w *DailyReminderWorker) Run(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, dailyReminderTimeout)
	defer cancel()
	sent, err := w.service.SendDailyReminders(ctx, util.Now())
	if err != nil {
		w.logger.Warn("daily push reminders finished with errors", zap.Int("sent", sent), zap.Error(err))
		return
	}
	w.logger.Info("daily push reminders sent", zap.Int("sent", sent))
}

func (w *DailyReminderWorker) Start() {
	if _, err := w.cron.AddFunc(DailyReminderSchedule, func() { w.Run(context.Background()) }); err != nil {
		w.logger.Error("failed to schedule daily reminder job", zap.Error(err))
		return
	}
	w.cron.Start()
	w.logger.Info("daily push reminder worker started", zap.String("schedule", DailyReminderSchedule))
}

func (w *DailyReminderWorker) Stop() {
	<-w.cron.Stop().Done()
}
