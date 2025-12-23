package dto

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type CustomTime struct {
	time.Time
}

const ctLayout = time.RFC3339

func (ct *CustomTime) UnmarshalJSON(b []byte) error {
	s := string(b)
	// Remove quotes if present
	if len(s) > 0 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "" || s == "null" {
		ct.Time = time.Time{}
		return nil
	}
	t, err := time.Parse(ctLayout, s)
	if err != nil {
		return fmt.Errorf("invalid time format, expected RFC3339: %v", err)
	}
	ct.Time = t
	return nil
}

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

type CostResponse struct {
	ID           string  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	UserID       string  `json:"userId" example:"550e8400-e29b-41d4-a716-446655440001"`
	Title        string  `json:"title" example:"Office Supplies"`
	Amount       float64 `json:"amount" example:"250.00"`
	Currency     string  `json:"currency" example:"USD"`
	IncurredAt   string  `json:"incurredAt" example:"2024-01-15T00:00:00Z"`
	CategoryID   string  `json:"categoryId" example:"550e8400-e29b-41d4-a716-446655440000"`
	CategoryName string  `json:"categoryName,omitempty" example:"Office"`
	CreatedAt    string  `json:"createdAt" example:"2024-01-15T00:00:00Z"`
	UpdatedAt    string  `json:"updatedAt" example:"2024-01-15T00:00:00Z"`
	DeletedAt    *string `json:"deletedAt,omitempty" example:"2024-01-20T00:00:00Z"`
}
