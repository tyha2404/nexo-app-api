package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

type mockUserRepo struct {
	usersByEmail    map[string]*model.User
	usersByUsername map[string]*model.User
	usersByID       map[uuid.UUID]*model.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		usersByEmail:    make(map[string]*model.User),
		usersByUsername: make(map[string]*model.User),
		usersByID:       make(map[uuid.UUID]*model.User),
	}
}

func (m *mockUserRepo) Create(ctx context.Context, user *model.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	m.usersByEmail[user.Email] = user
	m.usersByUsername[user.Username] = user
	m.usersByID[user.ID] = user
	return nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	u, ok := m.usersByID[id]
	if !ok {
		return nil, constant.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) List(ctx context.Context, limit, offset int) ([]model.User, error) {
	var list []model.User
	for _, u := range m.usersByID {
		list = append(list, *u)
	}
	return list, nil
}

func (m *mockUserRepo) Update(ctx context.Context, user *model.User) error {
	m.usersByID[user.ID] = user
	return nil
}

func (m *mockUserRepo) UpdateFields(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	return nil
}

func (m *mockUserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	delete(m.usersByID, id)
	return nil
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	u, ok := m.usersByEmail[email]
	if !ok {
		return nil, constant.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	u, ok := m.usersByUsername[username]
	if !ok {
		return nil, constant.ErrNotFound
	}
	return u, nil
}

func TestAuthService_Register(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewAuthService(repo)

	userToReg := &model.User{
		Username: "newuser",
		Email:    "new@example.com",
		Password: "password123",
	}

	created, err := svc.Register(context.Background(), userToReg)
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	if created.Username != userToReg.Username {
		t.Errorf("expected username %s, got %s", userToReg.Username, created.Username)
	}
	if created.Password != "" {
		t.Error("expected password to be cleared in response, but it was not")
	}

	// Try registering with same email again
	_, err = svc.Register(context.Background(), userToReg)
	if !errors.Is(err, constant.ErrEmailAlreadyExists) {
		t.Errorf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestAuthService_Login(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewAuthService(repo)

	user := &model.User{
		Username: "loginuser",
		Email:    "login@example.com",
		Password: "correctpassword",
	}
	_ = user.HashPassword()
	_ = repo.Create(context.Background(), user)

	// Restore plaintext password for comparison check in Login
	// Restore is not needed because GORM hashes it in model helper but in mock we need correct hash stored.
	// Hash is correct in database, so now we try login:

	// 1. Success login
	logged, err := svc.Login(context.Background(), "login@example.com", "correctpassword")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}
	if logged.Email != user.Email {
		t.Errorf("expected email %s, got %s", user.Email, logged.Email)
	}

	// 2. Failure login - incorrect password
	_, err = svc.Login(context.Background(), "login@example.com", "wrongpassword")
	if !errors.Is(err, constant.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}
