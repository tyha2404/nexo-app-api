package service

import (
	"errors"
	"testing"

	"github.com/tyha2404/nexo-app-api/internal/constant"
)

func TestValidateTarget(t *testing.T) {
	cases := []struct {
		name       string
		targetType string
		amount     float64
		month      int
		year       int
		ok         bool
	}{
		{"valid expense", "EXPENSE", 5_000_000, 10, 2026, true},
		{"valid investment", "INVESTMENT", 1, 1, 2000, true},
		{"unknown type", "SAVINGS", 100, 10, 2026, false},
		{"lowercase type", "expense", 100, 10, 2026, false},
		{"zero amount", "EXPENSE", 0, 10, 2026, false},
		{"amount overflows column", "EXPENSE", 1e11, 10, 2026, false},
		{"month 13", "EXPENSE", 100, 13, 2026, false},
		{"month 0", "EXPENSE", 100, 0, 2026, false},
		{"year too far", "EXPENSE", 100, 1, 9999, false},
	}
	for _, tc := range cases {
		err := validateTarget(tc.targetType, tc.amount, tc.month, tc.year)
		if tc.ok && err != nil {
			t.Errorf("%s: unexpected error %v", tc.name, err)
		}
		if !tc.ok && !errors.Is(err, constant.ErrInvalidInput) {
			t.Errorf("%s: expected ErrInvalidInput, got %v", tc.name, err)
		}
	}
}

func TestToolErrorMessage_HidesInternalErrors(t *testing.T) {
	if got := toolErrorMessage(errors.New(`pq: duplicate key value violates unique constraint "idx_x"`)); got == "" ||
		got == `pq: duplicate key value violates unique constraint "idx_x"` {
		t.Fatalf("internal error leaked: %q", got)
	}
	if got := toolErrorMessage(constant.ErrInsufficientBalance); got != constant.ErrInsufficientBalance.Error() {
		t.Fatalf("business error should pass through, got %q", got)
	}
}
