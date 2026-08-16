package dto

import "github.com/google/uuid"

type SummaryReport struct {
	TotalIncome     float64 `json:"totalIncome"`
	TotalExpense    float64 `json:"totalExpense"`
	TotalInvestment float64 `json:"totalInvestment"`
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
