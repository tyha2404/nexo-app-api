package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type CategoryService interface {
	Create(ctx context.Context, category *model.Category) (*model.Category, error)
	Get(ctx context.Context, id uuid.UUID) (*model.Category, error)
	List(ctx context.Context, userID uuid.UUID, categoryType string, limit, offset int) ([]model.Category, error)
	Update(ctx context.Context, category *model.Category) (*model.Category, error)
	UpdateFields(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type categoryService struct {
	repo repository.CategoryRepo
}

func NewCategoryService(repo repository.CategoryRepo) CategoryService {
	return &categoryService{
		repo: repo,
	}
}

func (s *categoryService) Create(ctx context.Context, category *model.Category) (*model.Category, error) {
	if err := s.repo.Create(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

func (s *categoryService) Get(ctx context.Context, id uuid.UUID) (*model.Category, error) {
	category, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return category, nil
}

func (s *categoryService) List(ctx context.Context, userID uuid.UUID, categoryType string, limit, offset int) ([]model.Category, error) {
	return s.repo.List(ctx, userID, categoryType, limit, offset)
}

func (s *categoryService) Update(ctx context.Context, category *model.Category) (*model.Category, error) {
	if err := s.repo.Update(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

func (s *categoryService) UpdateFields(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	return s.repo.UpdateFields(ctx, id, updates)
}

func (s *categoryService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
