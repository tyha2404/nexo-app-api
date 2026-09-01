package dto

import (
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

type CreateWalletRequest struct {
	Name              string           `json:"name" example:"Ví Tiền mặt" validate:"required,min=1,max=100"`
	Type              model.WalletType `json:"type" example:"CASH" validate:"required,oneof=CASH BANK E_WALLET SAVINGS CREDIT JAR"`
	Balance           float64          `json:"balance" example:"1000000"`
	Currency          string           `json:"currency" example:"VND"`
	Icon              string           `json:"icon" example:"💵"`
	JarCategory       *string          `json:"jarCategory,omitempty" example:"NEC"`
	AllocationPercent float64          `json:"allocationPercent" example:"50.0" validate:"gte=0,lte=100"`
	IsIncludedInTotal *bool            `json:"isIncludedInTotal,omitempty"`
	CreditLimit       *float64         `json:"creditLimit,omitempty" example:"30000000" validate:"omitempty,gte=0"`
	StatementDay      *int             `json:"statementDay,omitempty" example:"20" validate:"omitempty,gte=1,lte=31"`
	DueDay            *int             `json:"dueDay,omitempty" example:"5" validate:"omitempty,gte=1,lte=31"`
	StatementBalance  *float64         `json:"statementBalance,omitempty" example:"5000000" validate:"omitempty,gte=0"`
	MinimumPayment    *float64         `json:"minimumPayment,omitempty" example:"250000" validate:"omitempty,gte=0"`
	PreviousBalance   *float64         `json:"previousBalance,omitempty" example:"0" validate:"omitempty,gte=0"`
}

type UpdateWalletRequest struct {
	Name              *string           `json:"name,omitempty" example:"Ví Tiền mặt" validate:"omitempty,min=1,max=100"`
	Type              *model.WalletType `json:"type,omitempty" example:"CASH" validate:"omitempty,oneof=CASH BANK E_WALLET SAVINGS CREDIT JAR"`
	Balance           *float64          `json:"balance,omitempty" example:"1000000"`
	Currency          *string           `json:"currency,omitempty" example:"VND"`
	Icon              *string           `json:"icon,omitempty" example:"💵"`
	JarCategory       *string           `json:"jarCategory,omitempty" example:"NEC"`
	AllocationPercent *float64          `json:"allocationPercent,omitempty" example:"50.0" validate:"omitempty,gte=0,lte=100"`
	IsIncludedInTotal *bool             `json:"isIncludedInTotal,omitempty"`
	CreditLimit       *float64          `json:"creditLimit,omitempty" example:"30000000" validate:"omitempty,gte=0"`
	StatementDay      *int              `json:"statementDay,omitempty" example:"20" validate:"omitempty,gte=1,lte=31"`
	DueDay            *int              `json:"dueDay,omitempty" example:"5" validate:"omitempty,gte=1,lte=31"`
	StatementBalance  *float64          `json:"statementBalance,omitempty" example:"5000000" validate:"omitempty,gte=0"`
	MinimumPayment    *float64          `json:"minimumPayment,omitempty" example:"250000" validate:"omitempty,gte=0"`
	PreviousBalance   *float64          `json:"previousBalance,omitempty" example:"0" validate:"omitempty,gte=0"`
}

type TransferMoneyRequest struct {
	FromWalletID uuid.UUID   `json:"fromWalletId" validate:"required,uuid"`
	ToWalletID   uuid.UUID   `json:"toWalletId" validate:"required,uuid"`
	Amount       float64     `json:"amount" validate:"required,gt=0"`
	Fee          float64     `json:"fee" validate:"gte=0"`
	Note         *string     `json:"note,omitempty"`
	TransferDate *CustomTime `json:"transferDate,omitempty"`
}

type AutoAllocateRequest struct {
	SourceWalletID uuid.UUID              `json:"sourceWalletId,omitempty" validate:"omitempty,uuid"`
	IncomeAmount   float64                `json:"incomeAmount" validate:"required,gt=0"`
	Preset         model.AllocationPreset `json:"preset,omitempty" example:"50_30_20"`
}

type AllocatedWalletDetail struct {
	WalletID          uuid.UUID `json:"walletId"`
	WalletName        string    `json:"walletName"`
	AllocationPercent float64   `json:"allocationPercent"`
	AllocatedAmount   float64   `json:"allocatedAmount"`
	NewBalance        float64   `json:"newBalance"`
}

type AutoAllocateResponse struct {
	SourceWalletID *uuid.UUID              `json:"sourceWalletId,omitempty"`
	IncomeAmount   float64                 `json:"incomeAmount"`
	Allocations    []AllocatedWalletDetail `json:"allocations"`
	TotalAllocated float64                 `json:"totalAllocated"`
}

type WalletResponse struct {
	ID                uuid.UUID        `json:"id"`
	UserID            uuid.UUID        `json:"userId"`
	Name              string           `json:"name"`
	Type              model.WalletType `json:"type"`
	Balance           float64          `json:"balance"`
	Currency          string           `json:"currency"`
	Icon              string           `json:"icon"`
	JarCategory       *string          `json:"jarCategory,omitempty"`
	AllocationPercent float64          `json:"allocationPercent"`
	IsIncludedInTotal bool             `json:"isIncludedInTotal"`
	CreditLimit       *float64         `json:"creditLimit,omitempty"`
	StatementDay      *int             `json:"statementDay,omitempty"`
	DueDay            *int             `json:"dueDay,omitempty"`
	StatementBalance  *float64         `json:"statementBalance,omitempty"`
	MinimumPayment    *float64         `json:"minimumPayment,omitempty"`
	PreviousBalance   *float64         `json:"previousBalance,omitempty"`
	AvailableCredit   *float64         `json:"availableCredit,omitempty"`
	OutstandingDebt   *float64         `json:"outstandingDebt,omitempty"`
	CreatedAt         string           `json:"createdAt"`
	UpdatedAt         string           `json:"updatedAt"`
}

func ToWalletResponse(w *model.Wallet) WalletResponse {
	resp := WalletResponse{
		ID:                w.ID,
		UserID:            w.UserID,
		Name:              w.Name,
		Type:              w.Type,
		Balance:           w.Balance,
		Currency:          w.Currency,
		Icon:              w.Icon,
		JarCategory:       w.JarCategory,
		AllocationPercent: w.AllocationPercent,
		IsIncludedInTotal: w.IsIncludedInTotal,
		CreditLimit:       w.CreditLimit,
		StatementDay:      w.StatementDay,
		DueDay:            w.DueDay,
		StatementBalance:  w.StatementBalance,
		MinimumPayment:    w.MinimumPayment,
		PreviousBalance:   w.PreviousBalance,
		CreatedAt:         w.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:         w.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if w.Type == model.WalletTypeCredit {
		debt := w.Balance
		if debt < 0 {
			debt = -debt
		}
		resp.OutstandingDebt = &debt

		if w.CreditLimit != nil && *w.CreditLimit > 0 {
			avail := *w.CreditLimit + w.Balance
			if avail < 0 {
				avail = 0
			}
			resp.AvailableCredit = &avail
		}

		// Calculate default statement balance and minimum payment if not explicitly configured
		if resp.StatementBalance == nil || *resp.StatementBalance == 0 {
			if debt > 0 {
				resp.StatementBalance = &debt
			}
		}

		if resp.MinimumPayment == nil || *resp.MinimumPayment == 0 {
			if resp.StatementBalance != nil && *resp.StatementBalance > 0 {
				minPay := *resp.StatementBalance * 0.05 // 5% standard minimum payment
				if minPay < 50000 && *resp.StatementBalance >= 50000 {
					minPay = 50000
				}
				resp.MinimumPayment = &minPay
			}
		}
	}

	return resp
}

type WalletTransferResponse struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"userId"`
	FromWalletID uuid.UUID `json:"fromWalletId"`
	ToWalletID   uuid.UUID `json:"toWalletId"`
	Amount       float64   `json:"amount"`
	Fee          float64   `json:"fee"`
	Note         *string   `json:"note,omitempty"`
	TransferDate string    `json:"transferDate"`
	CreatedAt    string    `json:"createdAt"`
}

type WalletSummaryResponse struct {
	TotalBalance float64          `json:"totalBalance"`
	Wallets      []WalletResponse `json:"wallets"`
	TotalWallets int              `json:"totalWallets"`
}
