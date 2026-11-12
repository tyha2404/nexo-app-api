package worker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type panickingRollover struct{}

func (panickingRollover) ProcessRolloverForMonth(context.Context, int, int) error {
	panic("rollover exploded")
}

func waitForLog(t *testing.T, logs *observer.ObservedLogs, msg string) observer.LoggedEntry {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if found := logs.FilterMessage(msg).All(); len(found) > 0 {
			return found[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("log %q not written; got %v", msg, logs.All())
	return observer.LoggedEntry{}
}

// A panic in the startup sync goroutine is logged instead of crashing the
// process (an unrecovered goroutine panic would abort this test binary).
func TestStartupSyncPanicIsRecovered(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	w := NewMonthlyRolloverWorker(panickingRollover{}, zap.New(core))
	w.Start(context.Background())
	defer w.Stop()

	entry := waitForLog(t, logs, "panic recovered in background task")
	assert.Equal(t, "startup rollover sync", entry.ContextMap()["task"])
	assert.Contains(t, entry.ContextMap()["stack"], "recoverPanic")
}

// Scheduled jobs run through cron.Recover with the zap adapter.
func TestCronJobPanicIsRecovered(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	w := NewMonthlyRolloverWorker(panickingRollover{}, zap.New(core))
	_, err := w.cron.AddFunc("@every 1s", func() { panic("job exploded") })
	require.NoError(t, err)
	w.cron.Start()
	defer w.Stop()

	entry := waitForLog(t, logs, "cron: panic")
	assert.Contains(t, entry.ContextMap()["error"], "job exploded")
}
