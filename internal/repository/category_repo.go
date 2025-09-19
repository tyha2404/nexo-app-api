package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type CategoryRepo interface {
	Create(ctx context.Context, category *model.Category) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Category, error)
	List(ctx context.Context, limit, offset int) ([]model.Category, error)
	Update(ctx context.Context, category *model.Category) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type categoryRepo struct {
	db *gorm.DB
}

func NewCategoryRepo(db *gorm.DB) CategoryRepo {
	return &categoryRepo{db: db}
}

func (r *categoryRepo) Create(ctx context.Context, category *model.Category) error {
	return r.db.WithContext(ctx).Create(category).Error
}

func (r *categoryRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Category, error) {
	var c model.Category
	if err := r.db.WithContext(ctx).First(&c, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *categoryRepo) List(ctx context.Context, limit, offset int) ([]model.Category, error) {
	var categories []model.Category
	if err := r.db.WithContext(ctx).Limit(limit).Offset(offset).Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

func (r *categoryRepo) Update(ctx context.Context, category *model.Category) error {
	return r.db.WithContext(ctx).Save(category).Error
}

func (r *categoryRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Category{}, "id = ?", id).Error
}
