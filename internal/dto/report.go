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
