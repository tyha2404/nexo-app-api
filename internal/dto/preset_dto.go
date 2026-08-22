package dto

import (
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

type CreatePresetRequest struct {
	CategoryID  uuid.UUID `json:"categoryId" binding:"required"`
	Name        string    `json:"name" binding:"required"`
	Amount      float64   `json:"amount" binding:"required"`
	Type        string    `json:"type" binding:"required"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	SortOrder   int       `json:"sortOrder"`
}

type UpdatePresetRequest struct {
	CategoryID  *uuid.UUID `json:"categoryId"`
	Name        *string    `json:"name"`
	Amount      *float64   `json:"amount"`
	Type        *string    `json:"type"`
	Description *string    `json:"description"`
	Icon        *string    `json:"icon"`
	SortOrder   *int       `json:"sortOrder"`
}

type PresetResponse struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"userId"`
	CategoryID   uuid.UUID `json:"categoryId"`
	CategoryName string    `json:"categoryName"`
	Name         string    `json:"name"`
	Amount       float64   `json:"amount"`
	Type         string    `json:"type"`
	Description  string    `json:"description"`
	Icon         string    `json:"icon"`
	SortOrder    int       `json:"sortOrder"`
}

func ToPresetResponse(p *model.Preset) *PresetResponse {
	var catName string
	if p.Category.Name != "" {
		catName = p.Category.Name
	}
	return &PresetResponse{
		ID:           p.ID,
		UserID:       p.UserID,
		CategoryID:   p.CategoryID,
		CategoryName: catName,
		Name:         p.Name,
		Amount:       p.Amount,
		Type:         string(p.Type),
		Description:  p.Description,
		Icon:         p.Icon,
		SortOrder:    p.SortOrder,
	}
}
