package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type CostRepo interface {
	Create(ctx context.Context, cost *model.Cost) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Cost, error)
	List(ctx context.Context, limit, offset int) ([]model.Cost, error)
	Update(ctx context.Context, cost *model.Cost) error
	UpdateFields(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error
	Delete(ctx context.Context, id uuid.UUID) error
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

func (r *costRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Cost, error) {
	var cost model.Cost
	err := r.db.WithContext(ctx).First(&cost, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &cost, nil
}

func (r *costRepo) List(ctx context.Context, limit, offset int) ([]model.Cost, error) {
	var costs []model.Cost
	err := r.db.WithContext(ctx).Limit(limit).Offset(offset).Find(&costs).Error
	return costs, err
}

func (r *costRepo) Update(ctx context.Context, cost *model.Cost) error {
	return r.db.WithContext(ctx).Save(cost).Error
}

func (r *costRepo) UpdateFields(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.Cost{}).Where("id = ?", id).Updates(updates).Error
}

func (r *costRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Cost{}, "id = ?", id).Error
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
