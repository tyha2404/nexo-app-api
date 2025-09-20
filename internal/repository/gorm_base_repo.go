package repository

import (
	"context"

	"gorm.io/gorm"
)

// GormBaseRepo is a base GORM implementation of BaseRepo
type GormBaseRepo[T any, ID any] struct {
	db *gorm.DB
}

// NewGormBaseRepo creates a new base GORM repository
func NewGormBaseRepo[T any, ID any](db *gorm.DB) *GormBaseRepo[T, ID] {
	return &GormBaseRepo[T, ID]{db: db}
}

// Create implements BaseRepo.Create
func (r *GormBaseRepo[T, ID]) Create(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

// GetByID implements BaseRepo.GetByID
func (r *GormBaseRepo[T, ID]) GetByID(ctx context.Context, id ID) (*T, error) {
	var entity T
	err := r.db.WithContext(ctx).First(&entity, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

// List implements BaseRepo.List
func (r *GormBaseRepo[T, ID]) List(ctx context.Context, limit, offset int) ([]T, error) {
	var entities []T
	err := r.db.WithContext(ctx).Limit(limit).Offset(offset).Find(&entities).Error
	if err != nil {
		return nil, err
	}
	return entities, nil
}

// Update implements BaseRepo.Update
func (r *GormBaseRepo[T, ID]) Update(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Save(entity).Error
}

// Delete implements BaseRepo.Delete
func (r *GormBaseRepo[T, ID]) Delete(ctx context.Context, id ID) error {
	var entity T
	return r.db.WithContext(ctx).Delete(&entity, "id = ?", id).Error
}
