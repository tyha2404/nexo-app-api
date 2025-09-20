package repository

import (
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

type UserRepo interface {
	BaseRepo[model.User]
}

type userRepo struct {
	*GormBaseRepo[model.User, uuid.UUID]
}

func NewUserRepo(db *gorm.DB) UserRepo {
	return &userRepo{
		GormBaseRepo: NewGormBaseRepo[model.User, uuid.UUID](db),
	}
}
