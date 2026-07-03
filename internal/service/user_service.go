package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type UserService interface {
	Create(ctx context.Context, user *model.User) (*model.User, error)
	Get(ctx context.Context, id uuid.UUID) (*model.User, error)
	List(ctx context.Context, limit, offset int) ([]model.User, error)
	Update(ctx context.Context, user *model.User) (*model.User, error)
	UpdateFields(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type userService struct {
	repo repository.UserRepo
}

func NewUserService(repo repository.UserRepo) UserService {
	return &userService{
		repo: repo,
	}
}

func (s *userService) Create(ctx context.Context, user *model.User) (*model.User, error) {
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *userService) Get(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *userService) List(ctx context.Context, limit, offset int) ([]model.User, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *userService) Update(ctx context.Context, user *model.User) (*model.User, error) {
	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *userService) UpdateFields(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	return s.repo.UpdateFields(ctx, id, updates)
}

func (s *userService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
