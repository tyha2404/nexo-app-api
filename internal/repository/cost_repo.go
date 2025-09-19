package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type CostRepo interface {
	Create(ctx context.Context, cost *model.Cost) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Cost, error)
	List(ctx context.Context, limit, offset int) ([]model.Cost, error)
	Update(ctx context.Context, cost *model.Cost) error
	Delete(ctx context.Context, id uuid.UUID) error
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
	var c model.Cost
	if err := r.db.WithContext(ctx).First(&c, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *costRepo) List(ctx context.Context, limit, offset int) ([]model.Cost, error) {
	var costs []model.Cost
	if err := r.db.WithContext(ctx).Limit(limit).Offset(offset).Find(&costs).Error; err != nil {
		return nil, err
	}
	return costs, nil
}

func (r *costRepo) Update(ctx context.Context, cost *model.Cost) error {
	return r.db.WithContext(ctx).Save(cost).Error
}

func (r *costRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Cost{}, "id = ?", id).Error
}
