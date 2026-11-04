package dto

import (
	"github.com/google/uuid"
)

type CreateTransactionRequest struct {
	CategoryID      uuid.UUID  `json:"categoryId" example:"550e8400-e29b-41d4-a716-446655440000" validate:"required,uuid"`
	WalletID        *uuid.UUID `json:"walletId,omitempty" example:"550e8400-e29b-41d4-a716-446655440000" validate:"omitempty,uuid"`
	Amount          float64    `json:"amount" example:"100.50" validate:"required,gt=0"`
	Type            string     `json:"type" example:"EXPENSE" validate:"required,oneof=INCOME EXPENSE INVESTMENT"`
	Status          *string    `json:"status,omitempty" example:"HOLDING" validate:"omitempty,oneof=HOLDING SOLD MATURED CANCELLED"`
	RealizedPnL     *float64   `json:"realizedPnl,omitempty" example:"50.00"`
	Description     *string    `json:"description" example:"Grocery shopping" validate:"omitempty,max=500"`
	ReceiptURL      *string    `json:"receiptUrl,omitempty" validate:"omitempty,max=1024,len=0|http_url" example:"https://supabase.co/storage/v1/object/public/receipts/user/img.jpg"`
	Tags            []string   `json:"tags,omitempty" example:"Du lịch Đà Lạt" validate:"omitempty,max=10,dive,max=50"`
	TransactionDate CustomTime `json:"transactionDate" example:"2024-01-15T00:00:00Z" validate:"required"`
}

type UpdateTransactionRequest struct {
	CategoryID  *uuid.UUID `json:"categoryId,omitempty" example:"550e8400-e29b-41d4-a716-446655440000" validate:"omitempty,uuid"`
	WalletID    *uuid.UUID `json:"walletId,omitempty" example:"550e8400-e29b-41d4-a716-446655440000" validate:"omitempty,uuid"`
	Amount      *float64   `json:"amount,omitempty" example:"150.00" validate:"omitempty,gt=0"`
	Type        *string    `json:"type,omitempty" example:"INCOME" validate:"omitempty,oneof=INCOME EXPENSE INVESTMENT"`
	Status      *string    `json:"status,omitempty" example:"HOLDING" validate:"omitempty,oneof=HOLDING SOLD MATURED CANCELLED"`
	RealizedPnL *float64   `json:"realizedPnl,omitempty" example:"50.00"`
	Description *string    `json:"description,omitempty" example:"Updated description" validate:"omitempty,max=500"`
	ReceiptURL  *string    `json:"receiptUrl,omitempty" validate:"omitempty,max=1024,len=0|http_url" example:"https://supabase.co/storage/v1/object/public/receipts/user/img.jpg"`
	// Tags replaces the whole tag list when present ([] clears it); omitted keeps it.
	Tags            *[]string   `json:"tags,omitempty" example:"Du lịch Đà Lạt" validate:"omitempty,max=10,dive,max=50"`
	TransactionDate *CustomTime `json:"transactionDate,omitempty" example:"2024-01-20T00:00:00Z"`
}

type TransactionResponse struct {
	ID              uuid.UUID  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	UserID          uuid.UUID  `json:"userId" example:"550e8400-e29b-41d4-a716-446655440001"`
	CategoryID      uuid.UUID  `json:"categoryId" example:"550e8400-e29b-41d4-a716-446655440000"`
	CategoryName    string     `json:"categoryName,omitempty" example:"Food"`
	WalletID        *uuid.UUID `json:"walletId,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Amount          float64    `json:"amount" example:"100.50"`
	Type            string     `json:"type" example:"EXPENSE"`
	Status          *string    `json:"status,omitempty" example:"HOLDING"`
	RealizedPnL     *float64   `json:"realizedPnl,omitempty" example:"50.00"`
	Description     *string    `json:"description,omitempty" example:"Grocery shopping"`
	ReceiptURL      *string    `json:"receiptUrl,omitempty" example:"https://supabase.co/storage/v1/object/public/receipts/user/img.jpg"`
	Tags            []string   `json:"tags" example:"Du lịch Đà Lạt"`
	TransactionDate string     `json:"transactionDate" example:"2024-01-15T00:00:00Z"`
	CreatedAt       string     `json:"createdAt" example:"2024-01-15T00:00:00Z"`
	UpdatedAt       string     `json:"updatedAt" example:"2024-01-15T00:00:00Z"`
	DeletedAt       *string    `json:"deletedAt,omitempty" example:"2024-01-20T00:00:00Z"`
}

type TransactionSummaryDTO struct {
	SumAmount           float64 `json:"sumAmount"`
	SumAmountForAverage float64 `json:"sumAmountForAverage"`
	Total               int64   `json:"total"`
	HoldingAmount       float64 `json:"holdingAmount"`
	HoldingCount        int64   `json:"holdingCount"`
	RealizedPnL         float64 `json:"realizedPnl"`
}

// TagSummary is one tag the user has used, with its all-time totals.
type TagSummary struct {
	Name         string  `json:"name" example:"Du lịch Đà Lạt"`
	Count        int64   `json:"count" example:"12"`
	TotalExpense float64 `json:"totalExpense" example:"8500000"`
	TotalIncome  float64 `json:"totalIncome" example:"0"`
	LastUsed     string  `json:"lastUsed" example:"2026-11-04"` // latest transaction date with this tag
}
