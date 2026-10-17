package service

import (
	"testing"
	"time"

	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

func TestBudgetSpendWindowUsesAppTimezone(t *testing.T) {
	monthly := &model.Budget{PeriodType: "monthly"}

	// 2026-09-30T20:00Z is already Oct 1 03:00 in Vietnam -> October window.
	start, end := budgetSpendWindow(monthly, time.Date(2026, time.September, 30, 20, 0, 0, 0, time.UTC))
	if start.Format("2006-01-02") != "2026-10-01" || end.Format("2006-01-02") != "2026-10-31" {
		t.Fatalf("got %s..%s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
	if start.Location() != util.AppLocation() {
		t.Fatal("window must be in the app timezone")
	}

	// A DATE value (UTC midnight) stays in its own month.
	start, end = budgetSpendWindow(monthly, time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC))
	if start.Format("2006-01-02") != "2026-09-01" || end.Format("2006-01-02") != "2026-09-30" {
		t.Fatalf("got %s..%s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}

	start, end = budgetSpendWindow(&model.Budget{PeriodType: "yearly"}, time.Date(2026, time.December, 31, 18, 0, 0, 0, time.UTC))
	if start.Format("2006-01-02") != "2027-01-01" || end.Format("2006-01-02") != "2027-12-31" {
		t.Fatalf("yearly got %s..%s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
}
