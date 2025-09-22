package service

import (
	"context"

	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/repository"
)

type AuthService interface {
	Login(ctx context.Context, email string, password string) (*model.User, error)
	Register(ctx context.Context, user *model.User) (*model.User, error)
}

type authService struct {
	repo repository.UserRepo
}

func NewAuthService(repo repository.UserRepo) AuthService {
	return &authService{
		repo: repo,
	}
}

func (s *authService) Login(ctx context.Context, email string, password string) (*model.User, error) {
	// 1. Find user by email
	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		if err == constant.ErrNotFound {
			return nil, constant.ErrInvalidCredentials
		}
		return nil, err
	}

	// 2. Verify password hash
	if err := user.CheckPassword(password); err != nil {
		return nil, constant.ErrInvalidCredentials
	}

	// 3. Clear the password hash from the returned user for security
	user.Password = ""

	// Note: JWT token generation would typically happen here in the handler
	// and not in the service layer, following separation of concerns

	// 4. Return user data
	return user, nil
}

func (s *authService) Register(ctx context.Context, user *model.User) (*model.User, error) {
	// Validate user data
	if err := user.Validate(); err != nil {
		return nil, err
	}

	// Check if user with email already exists
	existingUser, err := s.repo.FindByEmail(ctx, user.Email)
	if err != nil && err != constant.ErrNotFound {
		return nil, err
	}
	if existingUser != nil {
		return nil, constant.ErrEmailAlreadyExists
	}

	// Check if username is taken
	existingUser, err = s.repo.FindByUsername(ctx, user.Username)
	if err != nil && err != constant.ErrNotFound {
		return nil, err
	}
	if existingUser != nil {
		return nil, constant.ErrUsernameTaken
	}

	// Generate password hash
	if err := user.HashPassword(); err != nil {
		return nil, err
	}

	// Save user to database
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	// Get the created user to return
	createdUser, err := s.repo.GetByID(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// Clear the password hash from the returned user for security
	createdUser.Password = ""

	return createdUser, nil
}
