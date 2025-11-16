package service

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"gorm.io/gorm"
)

// BaseServiceImpl is a base implementation of BaseService
type BaseServiceImpl[T any, ID any] struct {
	repo     interface{}
	validate *validator.Validate
}

// NewBaseService creates a new base service
func NewBaseService[T any, ID any](repo interface{}) *BaseServiceImpl[T, ID] {
	return &BaseServiceImpl[T, ID]{
		repo:     repo,
		validate: validator.New(),
	}
}

// Create creates a new entity
func (s *BaseServiceImpl[T, ID]) Create(ctx context.Context, req *T) (*T, error) {
	if err := s.validate.Struct(req); err != nil {
		return nil, err
	}

	repo, ok := s.repo.(interface {
		Create(context.Context, *T) error
	})
	if !ok {
		return nil, fmt.Errorf("repository does not implement Create method")
	}

	if err := repo.Create(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

// Get retrieves an entity by ID
func (s *BaseServiceImpl[T, ID]) Get(ctx context.Context, id ID) (*T, error) {
	repo, ok := s.repo.(interface {
		GetByID(context.Context, ID) (*T, error)
	})
	if !ok {
		return nil, fmt.Errorf("repository does not implement GetByID method")
	}

	entity, err := repo.GetByID(ctx, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, constant.ErrNotFound
		}
		return nil, err
	}
	return entity, nil
}

// List retrieves a paginated list of entities
func (s *BaseServiceImpl[T, ID]) List(ctx context.Context, limit, offset int) ([]T, error) {
	repo, ok := s.repo.(interface {
		List(context.Context, int, int) ([]T, error)
	})
	if !ok {
		return nil, fmt.Errorf("repository does not implement List method")
	}

	return repo.List(ctx, limit, offset)
}

// Update updates an existing entity
func (s *BaseServiceImpl[T, ID]) Update(ctx context.Context, req *T) (*T, error) {
	if err := s.validate.Struct(req); err != nil {
		return nil, err
	}

	repo, ok := s.repo.(interface {
		Update(context.Context, *T) error
	})
	if !ok {
		return nil, fmt.Errorf("repository does not implement Update method")
	}

	if err := repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

// Delete removes an entity by ID
func (s *BaseServiceImpl[T, ID]) Delete(ctx context.Context, id ID) error {
	repo, ok := s.repo.(interface {
		Delete(context.Context, ID) error
	})
	if !ok {
		return fmt.Errorf("repository does not implement Delete method")
	}

	return repo.Delete(ctx, id)
}
