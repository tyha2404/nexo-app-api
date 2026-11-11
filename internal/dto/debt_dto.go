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
	StartDate   *CustomTime    `json:"startDate" example:"2026-11-11"`
	DueDate     *CustomTime    `json:"dueDate" example:"2026-12-31"`
	Notes       string         `json:"notes"`
	WalletID    *uuid.UUID     `json:"walletId,omitempty"`
}

// UpdateDebtRequest replaces a debt's editable fields (full replacement: a
// null dueDate / walletId clears it). The type cannot change. totalAmount must
// not be below the amount already repaid; the wallet effect of the debt's
// creation is moved/resized to match the new wallet and amount.
type UpdateDebtRequest struct {
	Title       string      `json:"title" validate:"required"`
	TotalAmount float64     `json:"totalAmount" validate:"required,gt=0"`
	StartDate   *CustomTime `json:"startDate" example:"2026-11-11"`
	DueDate     *CustomTime `json:"dueDate" example:"2026-12-31"`
	Notes       string      `json:"notes"`
	WalletID    *uuid.UUID  `json:"walletId,omitempty"`
}

type AddRepaymentRequest struct {
	Amount   float64    `json:"amount" validate:"required,gt=0"`
	PaidAt   *time.Time `json:"paidAt"`
	Notes    string     `json:"notes"`
	WalletID *uuid.UUID `json:"walletId,omitempty"`
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
	StartDate   *string           `json:"startDate" example:"2026-11-11"` // YYYY-MM-DD
	DueDate     *string           `json:"dueDate" example:"2026-12-31"`   // YYYY-MM-DD
	Status      model.DebtStatus  `json:"status"`
	Notes       string            `json:"notes"`
	WalletID    *uuid.UUID        `json:"walletId,omitempty"`
	Repayments  []model.Repayment `json:"repayments,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}
