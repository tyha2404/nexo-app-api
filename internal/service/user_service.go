package service

import (
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type UserService interface {
	BaseService[model.User]
}

type userService struct {
	*BaseServiceImpl[model.User, uuid.UUID]
}

func NewUserService(repo repository.UserRepo) UserService {
	return &userService{
		BaseServiceImpl: NewBaseService[model.User, uuid.UUID](repo),
	}
}
