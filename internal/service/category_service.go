package service

import (
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type CategoryService interface {
	BaseService[model.Category]
}

type categoryService struct {
	*BaseServiceImpl[model.Category, uuid.UUID]
}

func NewCategoryService(repo repository.CategoryRepo) CategoryService {
	return &categoryService{
		BaseServiceImpl: NewBaseService[model.Category, uuid.UUID](repo),
	}
}
