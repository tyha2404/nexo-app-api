package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"gorm.io/gorm"
)

// BaseServiceImpl is a base implementation of BaseService
type BaseServiceImpl[T any] struct {
	repo Repository[T]
}

// NewBaseService creates a new base service
func NewBaseService[T any](repo Repository[T]) *BaseServiceImpl[T] {
	return &BaseServiceImpl[T]{
		repo: repo,
	}
}

// Create creates a new entity
func (s *BaseServiceImpl[T]) Create(ctx context.Context, req *T) (*T, error) {
	if err := s.repo.Create(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

// Get retrieves an entity by ID
func (s *BaseServiceImpl[T]) Get(ctx context.Context, id uuid.UUID) (*T, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, constant.ErrNotFound
		}
		return nil, err
	}
	return entity, nil
}

// List retrieves a paginated list of entities
func (s *BaseServiceImpl[T]) List(ctx context.Context, limit, offset int) ([]T, error) {
	return s.repo.List(ctx, limit, offset)
}

// Update updates an existing entity
func (s *BaseServiceImpl[T]) Update(ctx context.Context, req *T) (*T, error) {
	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

// UpdateFields updates specific fields of an existing entity
func (s *BaseServiceImpl[T]) UpdateFields(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	return s.repo.UpdateFields(ctx, id, updates)
}

// Delete removes an entity by ID
func (s *BaseServiceImpl[T]) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
