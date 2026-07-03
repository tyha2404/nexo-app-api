package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

type CreateBudgetRequest struct {
	CategoryID  uuid.UUID `json:"categoryId" validate:"required"`
	Amount      float64   `json:"amount" validate:"required,gt=0"`
	PeriodType  string    `json:"periodType" validate:"required,oneof=monthly yearly"`
	PeriodStart time.Time `json:"periodStart" validate:"required"`
}

type UpdateBudgetRequest struct {
	Amount     *float64 `json:"amount" validate:"omitempty,gt=0"`
	PeriodType *string  `json:"periodType" validate:"omitempty,oneof=monthly yearly"`
}

type BudgetResponse struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"userId"`
	CategoryID   uuid.UUID `json:"categoryId"`
	CategoryName string    `json:"categoryName"`
	Amount       float64   `json:"amount"`
	PeriodType   string    `json:"periodType"`
	PeriodStart  string    `json:"periodStart"`
	CreatedAt    string    `json:"createdAt"`
}

func ToBudgetResponse(b *model.Budget) *BudgetResponse {
	var catName string
	if b.Category.Name != "" {
		catName = b.Category.Name
	}
	return &BudgetResponse{
		ID:           b.ID,
		UserID:       b.UserID,
		CategoryID:   b.CategoryID,
		CategoryName: catName,
		Amount:       b.Amount,
		PeriodType:   b.PeriodType,
		PeriodStart:  b.PeriodStart.Format("2006-01-02"),
		CreatedAt:    b.CreatedAt.Format(time.RFC3339),
	}
}
