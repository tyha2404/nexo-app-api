package dto

import (
	"github.com/google/uuid"
)

type CreateCostRequest struct {
	Title      string     `json:"title" example:"Office Supplies" validate:"required,min=1,max=255"`
	Amount     float64    `json:"amount" example:"250.00" validate:"required,gt=0"`
	Currency   string     `json:"currency" example:"USD" validate:"required,len=3,uppercase"`
	IncurredAt CustomTime `json:"incurredAt" example:"2024-01-15T00:00:00Z" validate:"required"`
	CategoryID uuid.UUID  `json:"categoryId" example:"550e8400-e29b-41d4-a716-446655440000" validate:"required,uuid"`
}

type UpdateCostRequest struct {
	Title      *string     `json:"title,omitempty" example:"Updated Title" validate:"omitempty,min=1,max=255"`
	Amount     *float64    `json:"amount,omitempty" example:"300.00" validate:"omitempty,gt=0"`
	Currency   *string     `json:"currency,omitempty" example:"EUR" validate:"omitempty,len=3,uppercase"`
	IncurredAt *CustomTime `json:"incurredAt,omitempty" example:"2024-01-20T00:00:00Z"`
	CategoryID *uuid.UUID  `json:"categoryId,omitempty" example:"550e8400-e29b-41d4-a716-446655440001" validate:"omitempty,uuid"`
}
