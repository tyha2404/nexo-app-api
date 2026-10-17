package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type CategoryService interface {
	Create(ctx context.Context, category *model.Category) (*model.Category, error)
	// Get, UpdateFields and Delete are scoped to userID and return
	// constant.ErrNotFound when the category belongs to someone else.
	Get(ctx context.Context, userID, id uuid.UUID) (*model.Category, error)
	List(ctx context.Context, userID uuid.UUID, categoryType string, limit, offset int) ([]model.Category, error)
	ListWithTotal(ctx context.Context, userID uuid.UUID, categoryType string, page, limit int) ([]model.Category, int64, error)
	UpdateFields(ctx context.Context, userID, id uuid.UUID, updates map[string]interface{}) error
	Delete(ctx context.Context, userID, id uuid.UUID) error
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

func (s *categoryService) Get(ctx context.Context, userID, id uuid.UUID) (*model.Category, error) {
	category, err := s.repo.GetByIDForUser(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return category, nil
}

func (s *categoryService) List(ctx context.Context, userID uuid.UUID, categoryType string, limit, offset int) ([]model.Category, error) {
	return s.repo.List(ctx, userID, categoryType, limit, offset)
}

func (s *categoryService) ListWithTotal(ctx context.Context, userID uuid.UUID, categoryType string, page, limit int) ([]model.Category, int64, error) {
	offset := 0
	if page > 1 && limit > 0 {
		offset = (page - 1) * limit
	}
	return s.repo.ListWithTotal(ctx, userID, categoryType, limit, offset)
}

func (s *categoryService) UpdateFields(ctx context.Context, userID, id uuid.UUID, updates map[string]interface{}) error {
	return s.repo.UpdateFieldsForUser(ctx, userID, id, updates)
}

func (s *categoryService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.DeleteForUser(ctx, userID, id)
}
