package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type CostRepo interface {
	BaseRepo[model.Cost]
	ListWithCategory(ctx context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Cost, error)
}

type costRepo struct {
	*GormBaseRepo[model.Cost, uuid.UUID]
}

func NewCostRepo(db *gorm.DB) CostRepo {
	return &costRepo{
		GormBaseRepo: NewGormBaseRepo[model.Cost, uuid.UUID](db),
	}
}

func (r *costRepo) ListWithCategory(
	ctx context.Context,
	userID uuid.UUID,
	limit, offset int,
	filters map[string]interface{},
) ([]model.Cost, error) {
	query := r.db.WithContext(ctx)

	// Filter by startDate
	if s, ok := filters["startDate"].(string); ok && s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			query = query.Where("incurred_at >= ?", t)
		}
	}

	// Filter by endDate (add +1 day to include full endDate)
	if e, ok := filters["endDate"].(string); ok && e != "" {
		if t, err := time.Parse("2006-01-02", e); err == nil {
			query = query.Where("incurred_at < ?", t.Add(24*time.Hour))
		}
	}

	var costs []model.Cost
	err := query.
		Preload("Category").
		Where("user_id = ?", userID).
		Order("incurred_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&costs).Error

	return costs, err
}
