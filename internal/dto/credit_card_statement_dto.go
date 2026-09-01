package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

type CreateStatementRequest struct {
	WalletID         uuid.UUID `json:"walletId" validate:"required,uuid"`
	StatementMonth   int       `json:"statementMonth" example:"8" validate:"required,gte=1,lte=12"`
	StatementYear    int       `json:"statementYear" example:"2026" validate:"required,gte=2000"`
	StatementDate    string    `json:"statementDate" example:"2026-08-20" validate:"required"`
	DueDate          string    `json:"dueDate" example:"2026-09-05" validate:"required"`
	StatementBalance float64   `json:"statementBalance" example:"5000000" validate:"required,gte=0"`
	MinimumPayment   float64   `json:"minimumPayment" example:"250000" validate:"gte=0"`
	PreviousBalance  float64   `json:"previousBalance" example:"0" validate:"gte=0"`
	PaidAmount       float64   `json:"paidAmount" example:"0" validate:"gte=0"`
	Note             *string   `json:"note,omitempty" example:"Sao kê kỳ tháng 08/2026"`
}

type UpdateStatementRequest struct {
	StatementMonth   *int                   `json:"statementMonth,omitempty" example:"8" validate:"omitempty,gte=1,lte=12"`
	StatementYear    *int                   `json:"statementYear,omitempty" example:"2026" validate:"omitempty,gte=2000"`
	StatementDate    *string                `json:"statementDate,omitempty" example:"2026-08-20"`
	DueDate          *string                `json:"dueDate,omitempty" example:"2026-09-05"`
	StatementBalance *float64               `json:"statementBalance,omitempty" example:"5000000" validate:"omitempty,gte=0"`
	MinimumPayment   *float64               `json:"minimumPayment,omitempty" example:"250000" validate:"omitempty,gte=0"`
	PreviousBalance  *float64               `json:"previousBalance,omitempty" example:"0" validate:"omitempty,gte=0"`
	PaidAmount       *float64               `json:"paidAmount,omitempty" example:"1000000" validate:"omitempty,gte=0"`
	Status           *model.StatementStatus `json:"status,omitempty" example:"PARTIALLY_PAID"`
	Note             *string                `json:"note,omitempty" example:"Đã thanh toán một phần"`
}

type PayStatementRequest struct {
	Amount         float64    `json:"amount" example:"5000000" validate:"required,gt=0"`
	SourceWalletID *uuid.UUID `json:"sourceWalletId,omitempty" validate:"omitempty,uuid"`
	PaymentDate    *string    `json:"paymentDate,omitempty" example:"2026-09-01"`
	Note           *string    `json:"note,omitempty" example:"Thanh toán sao kê tháng 8"`
}

type StatementResponse struct {
	ID               uuid.UUID             `json:"id"`
	UserID           uuid.UUID             `json:"userId"`
	WalletID         uuid.UUID             `json:"walletId"`
	WalletName       string                `json:"walletName,omitempty"`
	WalletIcon       string                `json:"walletIcon,omitempty"`
	StatementMonth   int                   `json:"statementMonth"`
	StatementYear    int                   `json:"statementYear"`
	StatementDate    string                `json:"statementDate"`
	DueDate          string                `json:"dueDate"`
	StatementBalance float64               `json:"statementBalance"`
	MinimumPayment   float64               `json:"minimumPayment"`
	PreviousBalance  float64               `json:"previousBalance"`
	PaidAmount       float64               `json:"paidAmount"`
	RemainingAmount  float64               `json:"remainingAmount"`
	Status           model.StatementStatus `json:"status"`
	Note             string                `json:"note,omitempty"`
	CreatedAt        string                `json:"createdAt"`
	UpdatedAt        string                `json:"updatedAt"`
}

type StatementListResponse struct {
	Items          []StatementResponse `json:"items"`
	Total          int64               `json:"total"`
	TotalBalance   float64             `json:"totalBalance"`
	TotalPaid      float64             `json:"totalPaid"`
	TotalRemaining float64             `json:"totalRemaining"`
}

func ToStatementResponse(s *model.CreditCardStatement) StatementResponse {
	remaining := s.StatementBalance - s.PaidAmount
	if remaining < 0 {
		remaining = 0
	}

	status := s.Status
	now := time.Now()
	// Evaluate status dynamically if unpaid/partially paid and overdue
	if status == model.StatementStatusUnpaid || status == model.StatementStatusPartiallyPaid {
		if s.PaidAmount >= s.StatementBalance && s.StatementBalance > 0 {
			status = model.StatementStatusPaid
		} else if now.After(s.DueDate) && remaining > 0 {
			status = model.StatementStatusOverdue
		} else if s.PaidAmount > 0 {
			status = model.StatementStatusPartiallyPaid
		} else {
			status = model.StatementStatusUnpaid
		}
	}

	resp := StatementResponse{
		ID:               s.ID,
		UserID:           s.UserID,
		WalletID:         s.WalletID,
		StatementMonth:   s.StatementMonth,
		StatementYear:    s.StatementYear,
		StatementDate:    s.StatementDate.Format("2006-01-02"),
		DueDate:          s.DueDate.Format("2006-01-02"),
		StatementBalance: s.StatementBalance,
		MinimumPayment:   s.MinimumPayment,
		PreviousBalance:  s.PreviousBalance,
		PaidAmount:       s.PaidAmount,
		RemainingAmount:  remaining,
		Status:           status,
		Note:             s.Note,
		CreatedAt:        s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:        s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if s.Wallet != nil {
		resp.WalletName = s.Wallet.Name
		resp.WalletIcon = s.Wallet.Icon
	}

	return resp
}
