package worker_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tyha2404/nexo-app-api/internal/worker"
)

func TestIsEndOfMonthTransition(t *testing.T) {
	loc := time.UTC

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
