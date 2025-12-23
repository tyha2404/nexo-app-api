package dto

import (
	"time"

	"github.com/google/uuid"
)

type CreateTransactionRequest struct {
	CategoryID      uuid.UUID `json:"categoryId" example:"550e8400-e29b-41d4-a716-446655440000" validate:"required,uuid"`
	Amount          float64   `json:"amount" example:"100.50" validate:"required,gt=0"`
	Type            string    `json:"type" example:"EXPENSE" validate:"required,oneof=INCOME EXPENSE"`
	Description     *string   `json:"description" example:"Grocery shopping" validate:"omitempty,max=500"`
	TransactionDate time.Time `json:"transactionDate" example:"2024-01-15T00:00:00Z" validate:"required"`
}

type UpdateTransactionRequest struct {
	CategoryID      *uuid.UUID `json:"categoryId,omitempty" example:"550e8400-e29b-41d4-a716-446655440000" validate:"omitempty,uuid"`
	Amount          *float64   `json:"amount,omitempty" example:"150.00" validate:"omitempty,gt=0"`
	Type            *string    `json:"type,omitempty" example:"INCOME" validate:"omitempty,oneof=INCOME EXPENSE"`
	Description     *string    `json:"description,omitempty" example:"Updated description" validate:"omitempty,max=500"`
	TransactionDate *time.Time `json:"transactionDate,omitempty" example:"2024-01-20T00:00:00Z"`
}

type TransactionResponse struct {
	ID              uuid.UUID  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	UserID          uuid.UUID  `json:"userId" example:"550e8400-e29b-41d4-a716-446655440001"`
	CategoryID      uuid.UUID  `json:"categoryId" example:"550e8400-e29b-41d4-a716-446655440000"`
	CategoryName    string     `json:"categoryName,omitempty" example:"Food"`
	Amount          float64    `json:"amount" example:"100.50"`
	Type            string     `json:"type" example:"EXPENSE"`
	Description     *string    `json:"description,omitempty" example:"Grocery shopping"`
	TransactionDate string     `json:"transactionDate" example:"2024-01-15T00:00:00Z"`
	CreatedAt       string     `json:"createdAt" example:"2024-01-15T00:00:00Z"`
	UpdatedAt       string     `json:"updatedAt" example:"2024-01-15T00:00:00Z"`
	DeletedAt       *string    `json:"deletedAt,omitempty" example:"2024-01-20T00:00:00Z"`
}
