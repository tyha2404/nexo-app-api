package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type CostService interface {
	Create(ctx context.Context, cost *model.Cost) (*model.Cost, error)
	Get(ctx context.Context, id uuid.UUID) (*model.Cost, error)
	List(ctx context.Context, limit, offset int) ([]model.Cost, error)
	Update(ctx context.Context, cost *model.Cost) (*model.Cost, error)
	UpdateFields(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListWithCategory(ctx context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Cost, error)
}

type costService struct {
	repo repository.CostRepo
}

func NewCostService(repo repository.CostRepo) CostService {
	return &costService{
		repo: repo,
	}
}

func (s *costService) Create(ctx context.Context, cost *model.Cost) (*model.Cost, error) {
	if err := s.repo.Create(ctx, cost); err != nil {
		return nil, err
	}
	return cost, nil
}

func (s *costService) Get(ctx context.Context, id uuid.UUID) (*model.Cost, error) {
	cost, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return cost, nil
}

func (s *costService) List(ctx context.Context, limit, offset int) ([]model.Cost, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *costService) Update(ctx context.Context, cost *model.Cost) (*model.Cost, error) {
	if err := s.repo.Update(ctx, cost); err != nil {
		return nil, err
	}
	return cost, nil
}

func (s *costService) UpdateFields(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	return s.repo.UpdateFields(ctx, id, updates)
}

func (s *costService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *costService) ListWithCategory(ctx context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Cost, error) {
	return s.repo.ListWithCategory(ctx, userID, limit, offset, filters)
}
