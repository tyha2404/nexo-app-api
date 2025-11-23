package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type CostService interface {
	BaseService[model.Cost]
	ListWithCategory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Cost, error)
}

type costService struct {
	*BaseServiceImpl[model.Cost, uuid.UUID]
	repo repository.CostRepo
}

func NewCostService(repo repository.CostRepo) CostService {
	return &costService{
		BaseServiceImpl: NewBaseService[model.Cost, uuid.UUID](repo),
		repo:            repo,
	}
}

func (s *costService) ListWithCategory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Cost, error) {
	return s.repo.ListWithCategory(ctx, userID, limit, offset)
}
