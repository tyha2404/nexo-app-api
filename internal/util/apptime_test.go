package util

import (
	"testing"
	"time"
)

func TestAppLocationDefaultsToVietnam(t *testing.T) {
	if got := AppLocation().String(); got != DefaultTimezone {
		t.Fatalf("expected %s, got %s", DefaultTimezone, got)
	}
	_, offset := Now().Zone()
	if offset != 7*3600 {
		t.Fatalf("expected UTC+7 offset, got %d", offset)
	}
}

func TestSetAppTimezone(t *testing.T) {
	t.Cleanup(func() { _ = SetAppTimezone(DefaultTimezone) })

	if err := SetAppTimezone("Not/AZone"); err == nil {
		t.Fatal("expected error for invalid zone")
	}
	if err := SetAppTimezone(""); err != nil {
		t.Fatalf("empty name should keep default: %v", err)
	}
	if err := SetAppTimezone("UTC"); err != nil {
		t.Fatal(err)
	}
	if AppLocation() != time.UTC && AppLocation().String() != "UTC" {
		t.Fatalf("expected UTC, got %s", AppLocation())
	}
}

func TestMonthRangeUsesAppTimezone(t *testing.T) {
	// 2026-10-31 20:00 UTC is already 2026-11-01 03:00 in Vietnam.
	instant := time.Date(2026, 10, 31, 20, 0, 0, 0, time.UTC)
	start, end := MonthRange(instant)
	if start.Month() != time.November || start.Day() != 1 || start.Hour() != 0 {
		t.Fatalf("unexpected start %v", start)
	}
	if end.Month() != time.December || end.Day() != 1 {
		t.Fatalf("unexpected end %v", end)
	}
	if !start.Equal(time.Date(2026, 10, 31, 17, 0, 0, 0, time.UTC)) {
		t.Fatalf("start should be 17:00 UTC on Oct 31, got %v", start.UTC())
	}

	day := StartOfDay(instant)
	if day.Day() != 1 || day.Month() != time.November {
		t.Fatalf("unexpected start of day %v", day)
	}

	s2, e2 := MonthRangeOf(2026, time.February)
	if e2.Sub(s2) != 28*24*time.Hour {
		t.Fatalf("February 2026 should be 28 days, got %v", e2.Sub(s2))
	}
}

func TestIsPastDate(t *testing.T) {
	now := Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC) // DATE column value
	if IsPastDate(today) {
		t.Fatal("a due date of today is not past")
	}
	if !IsPastDate(today.AddDate(0, 0, -1)) {
		t.Fatal("yesterday is past")
	}
	if IsPastDate(today.AddDate(0, 0, 1)) {
		t.Fatal("tomorrow is not past")
	}
	if Today() != now.Format("2006-01-02") {
		t.Fatal("Today must use the app timezone")
	}
}
