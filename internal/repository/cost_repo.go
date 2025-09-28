package repository

import (
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type CostRepo interface {
	BaseRepo[model.Cost]
}

type costRepo struct {
	*GormBaseRepo[model.Cost, uuid.UUID]
}

func NewCostRepo(db *gorm.DB) CostRepo {
	return &costRepo{
		GormBaseRepo: NewGormBaseRepo[model.Cost, uuid.UUID](db),
	}
}
