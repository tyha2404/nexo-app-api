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

type CostService interface {
	Create(ctx context.Context, req *model.Cost) (*model.Cost, error)
	Get(ctx context.Context, id uuid.UUID) (*model.Cost, error)
	List(ctx context.Context, limit, offset int) ([]model.Cost, error)
	Update(ctx context.Context, req *model.Cost) (*model.Cost, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type costService struct {
	repo     repository.CostRepo
	validate *validator.Validate
}

func NewCostService(r repository.CostRepo) CostService {
	return &costService{repo: r, validate: validator.New()}
}

func (s *costService) Create(ctx context.Context, req *model.Cost) (*model.Cost, error) {
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

func (s *costService) Get(ctx context.Context, id uuid.UUID) (*model.Cost, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, constant.ErrNotFound
		}
		return nil, err
	}
	return c, nil
}

func (s *costService) List(ctx context.Context, limit, offset int) ([]model.Cost, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *costService) Update(ctx context.Context, req *model.Cost) (*model.Cost, error) {
	if err := s.validate.Struct(req); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *costService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
