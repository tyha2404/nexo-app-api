package dto

import "github.com/google/uuid"

type SummaryReport struct {
	TotalIncome     float64 `json:"totalIncome"`
	TotalExpense    float64 `json:"totalExpense"`
	TotalInvestment float64 `json:"totalInvestment"`
	RealizedPnL     float64 `json:"realizedPnL"`
}

type CategoryBreakdownItem struct {
	CategoryID   uuid.UUID `json:"categoryId"`
	CategoryName string    `json:"categoryName"`
	TotalAmount  float64   `json:"totalAmount"`
	Percentage   float64   `json:"percentage"`
}

type CategoryBreakdownReport struct {
	Items        []CategoryBreakdownItem `json:"items"`
	TotalExpense float64                 `json:"totalExpense"`
}

type MonthlyTrendItem struct {
	Month   string  `json:"month"`   // e.g. "2025-10"
	Label   string  `json:"label"`   // e.g. "T10/25"
	Expense float64 `json:"expense"` // Total expense for the month
	Income  float64 `json:"income"`  // Total income for the month
	Target  float64 `json:"target"`  // Budget / target for the month
}

type MonthlyTrendReport struct {
	AverageExpense float64            `json:"averageExpense"`
	Items          []MonthlyTrendItem `json:"items"`
}

// NetWorthPoint is net worth at the end of one month and what moved it.
// Net worth = cumulative income − expense + realized investment PnL, the same
// formula as the dashboard's "Tổng tài sản" (debts and wallet transfers move
// money between places without changing it).
type NetWorthPoint struct {
	Month       string  `json:"month" example:"2026-11"`
	Label       string  `json:"label" example:"T11/26"`
	Income      float64 `json:"income"`
	Expense     float64 `json:"expense"`
	RealizedPnL float64 `json:"realizedPnL"`
	// Change = Income − Expense + RealizedPnL for the month.
	Change   float64 `json:"change"`
	NetWorth float64 `json:"netWorth"`
}

// NetWorthReport is the month-end net worth series, oldest first, ending with
// the current month. Leading months before the user's first transaction are
// left out (at least two points are kept).
type NetWorthReport struct {
	Items []NetWorthPoint `json:"items"`
	// Opening is net worth just before the first item's month.
	Opening float64 `json:"opening"`
	// Current is the last item's NetWorth.
	Current float64 `json:"current"`
	// PeriodChange is Current − Opening.
	PeriodChange float64 `json:"periodChange"`
}

// DailyReportItem is one calendar day of the daily report.
type DailyReportItem struct {
	Date    string  `json:"date"`    // e.g. "2026-11-04"
	Income  float64 `json:"income"`  // Total income that day
	Expense float64 `json:"expense"` // Total expense that day
	// Investment is the amount invested that day (INVESTMENT of any status)
	Investment float64 `json:"investment"`
	Count      int64   `json:"count"` // Number of transactions that day (all types)
}

// DailyReport has one item per day of the month (days without transactions
// are zero), for the spending calendar.
type DailyReport struct {
	Month           string            `json:"month"` // e.g. "2026-11"
	Items           []DailyReportItem `json:"items"`
	TotalIncome     float64           `json:"totalIncome"`
	TotalExpense    float64           `json:"totalExpense"`
	TotalInvestment float64           `json:"totalInvestment"`
	MaxExpense      float64           `json:"maxExpense"` // Highest single-day expense, for color scaling
}
