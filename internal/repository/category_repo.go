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
	List(ctx context.Context, userID uuid.UUID, categoryType string, limit, offset int) ([]model.Category, error)
	Update(ctx context.Context, category *model.Category) error
	UpdateFields(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error
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
	var category model.Category
	err := r.db.WithContext(ctx).First(&category, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *categoryRepo) List(ctx context.Context, userID uuid.UUID, categoryType string, limit, offset int) ([]model.Category, error) {
	var categories []model.Category
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if categoryType != "" {
		query = query.Where("type = ?", categoryType)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	err := query.Find(&categories).Error
	return categories, err
}

func (r *categoryRepo) Update(ctx context.Context, category *model.Category) error {
	return r.db.WithContext(ctx).Save(category).Error
}

func (r *categoryRepo) UpdateFields(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.Category{}).Where("id = ?", id).Updates(updates).Error
}

func (r *categoryRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Category{}, "id = ?", id).Error
}
