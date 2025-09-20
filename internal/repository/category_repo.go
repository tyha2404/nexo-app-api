package repository

import (
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type CategoryRepo interface {
	BaseRepo[model.Category]
}

type categoryRepo struct {
	*GormBaseRepo[model.Category, uuid.UUID]
}

func NewCategoryRepo(db *gorm.DB) CategoryRepo {
	return &categoryRepo{
		GormBaseRepo: NewGormBaseRepo[model.Category, uuid.UUID](db),
	}
}
