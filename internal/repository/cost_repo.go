package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type CostRepo interface {
	BaseRepo[model.Cost]
	ListWithCategory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Cost, error)
}

type costRepo struct {
	*GormBaseRepo[model.Cost, uuid.UUID]
}

func NewCostRepo(db *gorm.DB) CostRepo {
	return &costRepo{
		GormBaseRepo: NewGormBaseRepo[model.Cost, uuid.UUID](db),
	}
}

func (r *costRepo) ListWithCategory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Cost, error) {
	var costs []model.Cost
	err := r.db.WithContext(ctx).
		Preload("Category").
		Where("user_id = ?", userID).
		Order("incurred_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&costs).Error

	return costs, err
}
