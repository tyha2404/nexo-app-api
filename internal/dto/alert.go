package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

type AlertResponse struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"userId"`
	BudgetID    uuid.UUID `json:"budgetId"`
	AlertType   string    `json:"alertType"`
	TriggeredAt string    `json:"triggeredAt"`
	Message     string    `json:"message"`
	CreatedAt   string    `json:"createdAt"`
}

func ToAlertResponse(a *model.Alert) *AlertResponse {
	return &AlertResponse{
		ID:          a.ID,
		UserID:      a.UserID,
		BudgetID:    a.BudgetID,
		AlertType:   a.AlertType,
		TriggeredAt: a.TriggeredAt.Format(time.RFC3339),
		Message:     a.Message,
		CreatedAt:   a.CreatedAt.Format(time.RFC3339),
	}
}
