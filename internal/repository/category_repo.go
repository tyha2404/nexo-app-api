package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type CategoryRepo interface {
	Create(ctx context.Context, category *model.Category) error
	// GetByIDForUser returns constant.ErrNotFound when the category does not
	// exist or belongs to another user.
	GetByIDForUser(ctx context.Context, userID, id uuid.UUID) (*model.Category, error)
	List(ctx context.Context, userID uuid.UUID, categoryType string, limit, offset int) ([]model.Category, error)
	ListWithTotal(ctx context.Context, userID uuid.UUID, categoryType string, limit, offset int) ([]model.Category, int64, error)
	// UpdateFieldsForUser / DeleteForUser only touch the caller's own row and
	// return constant.ErrNotFound when nothing matched.
	UpdateFieldsForUser(ctx context.Context, userID, id uuid.UUID, updates map[string]interface{}) error
	DeleteForUser(ctx context.Context, userID, id uuid.UUID) error
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

func (r *categoryRepo) GetByIDForUser(ctx context.Context, userID, id uuid.UUID) (*model.Category, error) {
	var category model.Category
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&category).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constant.ErrNotFound
		}
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

func (r *categoryRepo) ListWithTotal(ctx context.Context, userID uuid.UUID, categoryType string, limit, offset int) ([]model.Category, int64, error) {
	var categories []model.Category
	var total int64
	query := r.db.WithContext(ctx).Model(&model.Category{}).Where("user_id = ?", userID)
	if categoryType != "" {
		query = query.Where("type = ?", categoryType)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	err := query.Find(&categories).Error
	return categories, total, err
}

func (r *categoryRepo) UpdateFieldsForUser(ctx context.Context, userID, id uuid.UUID, updates map[string]interface{}) error {
	result := r.db.WithContext(ctx).Model(&model.Category{}).
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

func (r *categoryRepo) DeleteForUser(ctx context.Context, userID, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&model.Category{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return constant.ErrNotFound
	}
	return nil
}
