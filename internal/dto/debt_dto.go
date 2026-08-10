package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

type CreateDebtRequest struct {
	Type        model.DebtType `json:"type" validate:"required,oneof=PAYABLE RECEIVABLE"`
	Title       string         `json:"title" validate:"required"`
	TotalAmount float64        `json:"totalAmount" validate:"required,gt=0"`
	StartDate   *time.Time     `json:"startDate"`
	DueDate     *time.Time     `json:"dueDate"`
	Notes       string         `json:"notes"`
}

type AddRepaymentRequest struct {
	Amount float64    `json:"amount" validate:"required,gt=0"`
	PaidAt *time.Time `json:"paidAt"`
	Notes  string     `json:"notes"`
}

type DebtSummaryResponse struct {
	TotalPayable    float64 `json:"totalPayable"`
	TotalReceivable float64 `json:"totalReceivable"`
	OverdueCount    int64   `json:"overdueCount"`
	PendingCount    int64   `json:"pendingCount"`
}

type DebtResponse struct {
	ID          uuid.UUID         `json:"id"`
	UserID      uuid.UUID         `json:"userId"`
	Type        model.DebtType    `json:"type"`
	Title       string            `json:"title"`
	TotalAmount float64           `json:"totalAmount"`
	PaidAmount  float64           `json:"paidAmount"`
	Remaining   float64           `json:"remaining"`
	StartDate   *time.Time        `json:"startDate"`
	DueDate     *time.Time        `json:"dueDate"`
	Status      model.DebtStatus  `json:"status"`
	Notes       string            `json:"notes"`
	Repayments  []model.Repayment `json:"repayments,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}
