package service

import (
	"context"
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
	"gorm.io/gorm"
)

type CategoryService interface {
	Create(ctx context.Context, req *model.Category) (*model.Category, error)
	Get(ctx context.Context, id uuid.UUID) (*model.Category, error)
	List(ctx context.Context, limit, offset int) ([]model.Category, error)
	Update(ctx context.Context, req *model.Category) (*model.Category, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type categoryService struct {
	repo     repository.CategoryRepo
	validate *validator.Validate
}

func NewCategoryService(r repository.CategoryRepo) CategoryService {
	return &categoryService{repo: r, validate: validator.New()}
}

func (s *categoryService) Create(ctx context.Context, req *model.Category) (*model.Category, error) {
	if err := s.validate.Struct(req); err != nil {
		return nil, err
	}
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}
	if err := s.repo.Create(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *categoryService) Get(ctx context.Context, id uuid.UUID) (*model.Category, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constant.ErrNotFound
		}
		return nil, err
	}
	return c, nil
}

func (s *categoryService) List(ctx context.Context, limit, offset int) ([]model.Category, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *categoryService) Update(ctx context.Context, req *model.Category) (*model.Category, error) {
	if err := s.validate.Struct(req); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *categoryService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
