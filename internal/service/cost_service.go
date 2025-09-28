package service

import (
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type CostService interface {
	BaseService[model.Cost]
}

type costService struct {
	*BaseServiceImpl[model.Cost, uuid.UUID]
}

func NewCostService(repo repository.CostRepo) CostService {
	return &costService{
		BaseServiceImpl: NewBaseService[model.Cost, uuid.UUID](repo),
	}
}
