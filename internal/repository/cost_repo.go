package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type CostRepo interface {
	Create(ctx context.Context, cost *model.Cost) error
	// GetByIDForUser, UpdateFieldsForUser and DeleteForUser are scoped to the
	// owner and return constant.ErrNotFound when the cost is missing or
	// belongs to someone else.
	GetByIDForUser(ctx context.Context, userID, id uuid.UUID) (*model.Cost, error)
	UpdateFieldsForUser(ctx context.Context, userID, id uuid.UUID, updates map[string]interface{}) error
	DeleteForUser(ctx context.Context, userID, id uuid.UUID) error
	ListWithCategory(ctx context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Cost, error)
}

type costRepo struct {
	db *gorm.DB
}

func NewCostRepo(db *gorm.DB) CostRepo {
	return &costRepo{db: db}
}

func (r *costRepo) Create(ctx context.Context, cost *model.Cost) error {
	return r.db.WithContext(ctx).Create(cost).Error
}

func (r *costRepo) GetByIDForUser(ctx context.Context, userID, id uuid.UUID) (*model.Cost, error) {
	var cost model.Cost
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&cost).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constant.ErrNotFound
		}
		return nil, err
	}
	return &cost, nil
}

func (r *costRepo) UpdateFieldsForUser(ctx context.Context, userID, id uuid.UUID, updates map[string]interface{}) error {
	result := r.db.WithContext(ctx).Model(&model.Cost{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return constant.ErrNotFound
	}
	return nil
}

func (r *costRepo) DeleteForUser(ctx context.Context, userID, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.Cost{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return constant.ErrNotFound
	}
	return nil
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
