package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

// UserService backs the admin-only /users endpoints.
type UserService interface {
	Create(ctx context.Context, req dto.CreateUserRequest) (*model.User, error)
	Get(ctx context.Context, id uuid.UUID) (*model.User, error)
	List(ctx context.Context, limit, offset int) ([]model.User, error)
	// Update applies only the whitelisted fields of req (username, email).
	// Role and password can never be changed through it.
	Update(ctx context.Context, id uuid.UUID, req dto.UpdateUserRequest) (*model.User, error)
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

func (s *userService) Create(ctx context.Context, req dto.CreateUserRequest) (*model.User, error) {
	email := strings.TrimSpace(req.Email)
	username := strings.TrimSpace(req.Username)

	if existing, err := s.repo.FindByEmail(ctx, email); err != nil && err != constant.ErrNotFound {
		return nil, err
	} else if existing != nil {
		return nil, constant.ErrEmailAlreadyExists
	}
	if existing, err := s.repo.FindByUsername(ctx, username); err != nil && err != constant.ErrNotFound {
		return nil, err
	} else if existing != nil {
		return nil, constant.ErrUsernameTaken
	}

	user := &model.User{
		Username: username,
		Email:    email,
		Password: req.Password,
		Role:     "user",
	}
	if err := user.HashPassword(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	out := *user
	out.Password = ""
	return &out, nil
}

func (s *userService) Get(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *userService) List(ctx context.Context, limit, offset int) ([]model.User, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *userService) Update(ctx context.Context, id uuid.UUID, req dto.UpdateUserRequest) (*model.User, error) {
	updates := make(map[string]interface{})
	if req.Username != nil {
		username := strings.TrimSpace(*req.Username)
		existing, err := s.repo.FindByUsername(ctx, username)
		if err != nil && err != constant.ErrNotFound {
			return nil, err
		}
		if existing != nil && existing.ID != id {
			return nil, constant.ErrUsernameTaken
		}
		updates["username"] = username
	}
	if req.Email != nil {
		email := strings.TrimSpace(*req.Email)
		existing, err := s.repo.FindByEmail(ctx, email)
		if err != nil && err != constant.ErrNotFound {
			return nil, err
		}
		if existing != nil && existing.ID != id {
			return nil, constant.ErrEmailAlreadyExists
		}
		updates["email"] = email
	}
	if len(updates) == 0 {
		return nil, constant.ErrInvalidInput
	}

	if err := s.repo.UpdateFields(ctx, id, updates); err != nil {
		return nil, err
	}
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	out := *user
	out.Password = ""
	return &out, nil
}

func (s *userService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
