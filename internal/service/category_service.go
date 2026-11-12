package service

import (
	"context"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type CategoryService interface {
	Create(ctx context.Context, userID uuid.UUID, req dto.CreateCategoryRequest) (*model.Category, error)
	// Get, Update and Delete are scoped to userID and return
	// constant.ErrNotFound when the category belongs to someone else.
	Get(ctx context.Context, userID, id uuid.UUID) (*model.Category, error)
	List(ctx context.Context, userID uuid.UUID, categoryType string, limit, offset int) ([]model.Category, error)
	ListWithTotal(ctx context.Context, userID uuid.UUID, categoryType string, page, limit int) ([]model.Category, int64, error)
	// Update applies the fields present in req and returns the updated
	// category; constant.ErrInvalidInput when req has no field.
	Update(ctx context.Context, userID, id uuid.UUID, req dto.UpdateCategoryRequest) (*model.Category, error)
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

func (s *categoryService) Create(ctx context.Context, userID uuid.UUID, req dto.CreateCategoryRequest) (*model.Category, error) {
	category := &model.Category{
		Name:        req.Name,
		Type:        model.CategoryType(req.Type),
		Description: req.Description,
		UserID:      userID,
	}
	if req.ExcludeFromAverageDaily != nil {
		category.ExcludeFromAverageDaily = *req.ExcludeFromAverageDaily
	}
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

func (s *categoryService) Update(ctx context.Context, userID, id uuid.UUID, req dto.UpdateCategoryRequest) (*model.Category, error) {
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.ExcludeFromAverageDaily != nil {
		updates["exclude_from_average_daily"] = *req.ExcludeFromAverageDaily
	}
	if len(updates) == 0 {
		return nil, constant.ErrInvalidInput
	}
	if err := s.repo.UpdateFieldsForUser(ctx, userID, id, updates); err != nil {
		return nil, err
	}
	return s.repo.GetByIDForUser(ctx, userID, id)
}

func (s *categoryService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.DeleteForUser(ctx, userID, id)
}
