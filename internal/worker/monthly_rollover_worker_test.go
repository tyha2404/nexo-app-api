package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"github.com/tyha2404/nexo-app-api/internal/worker"
	"go.uber.org/zap"
)

func TestIsEndOfMonthTransition(t *testing.T) {
	loc := util.AppLocation()

	testCases := []struct {
		name               string
		currentTime        time.Time
		expectedTransition bool
		expectedMonth      int
		expectedYear       int
	}{
		{
			name:               "23:50 on August 31 (Leads into September)",
			currentTime:        time.Date(2026, time.August, 31, 23, 50, 0, 0, loc),
			expectedTransition: true,
			expectedMonth:      9,
			expectedYear:       2026,
		},
		{
			name:               "23:50 on December 31 (Leads into January next year)",
			currentTime:        time.Date(2026, time.December, 31, 23, 50, 0, 0, loc),
			expectedTransition: true,
			expectedMonth:      1,
			expectedYear:       2027,
		},
		{
			name:               "23:50 on February 28 (Non-leap year leads into March)",
			currentTime:        time.Date(2025, time.February, 28, 23, 50, 0, 0, loc),
			expectedTransition: true,
			expectedMonth:      3,
			expectedYear:       2025,
		},
		{
			name:               "23:50 on February 29 (Leap year leads into March)",
			currentTime:        time.Date(2028, time.February, 29, 23, 50, 0, 0, loc),
			expectedTransition: true,
			expectedMonth:      3,
			expectedYear:       2028,
		},
		{
			name:               "23:50 on Mid-month (August 15)",
			currentTime:        time.Date(2026, time.August, 15, 23, 50, 0, 0, loc),
			expectedTransition: false,
			expectedMonth:      0,
			expectedYear:       0,
		},
		{
			name:               "12:00 on August 31 (Not 23:50)",
			currentTime:        time.Date(2026, time.August, 31, 12, 0, 0, 0, loc),
			expectedTransition: false,
			expectedMonth:      0,
			expectedYear:       0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isTransition, month, year := worker.IsEndOfMonthTransition(tc.currentTime)
			assert.Equal(t, tc.expectedTransition, isTransition)
			assert.Equal(t, tc.expectedMonth, month)
			assert.Equal(t, tc.expectedYear, year)
		})
	}
}

// The server may run in UTC: the transition is 23:50 in the app timezone
// (Asia/Ho_Chi_Minh, UTC+7), not 23:50 UTC.
func TestIsEndOfMonthTransition_UsesAppTimezone(t *testing.T) {
	ok, month, year := worker.IsEndOfMonthTransition(time.Date(2026, time.August, 31, 16, 50, 0, 0, time.UTC))
	assert.True(t, ok)
	assert.Equal(t, 9, month)
	assert.Equal(t, 2026, year)

	ok, _, _ = worker.IsEndOfMonthTransition(time.Date(2026, time.August, 31, 23, 50, 0, 0, time.UTC))
	assert.False(t, ok, "23:50 UTC is already 06:50 on Sept 1 in Vietnam")
}

type fakeRollover struct {
	calls chan [2]int
	err   error
}

func (f *fakeRollover) ProcessRolloverForMonth(_ context.Context, month, year int) error {
	f.calls <- [2]int{month, year}
	return f.err
}

func TestMonthlyRolloverWorker_StartSyncsCurrentMonthThenStops(t *testing.T) {
	for _, svcErr := range []error{nil, errors.New("db down")} {
		svc := &fakeRollover{calls: make(chan [2]int, 1), err: svcErr}
		w := worker.NewMonthlyRolloverWorker(svc, zap.NewNop())

		w.Start(context.Background())

		select {
		case got := <-svc.calls:
			now := util.Now()
			assert.Equal(t, [2]int{int(now.Month()), now.Year()}, got)
		case <-time.After(2 * time.Second):
			t.Fatal("startup sync did not run")
		}

		done := make(chan struct{})
		go func() { w.Stop(); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Stop did not return")
		}
	}
}
