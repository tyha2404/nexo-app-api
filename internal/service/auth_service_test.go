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
	lastUpdates     map[string]interface{}
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
	u, ok := m.usersByID[id]
	if !ok {
		return constant.ErrNotFound
	}
	m.lastUpdates = updates
	if v, ok := updates["username"].(string); ok {
		u.Username = v
	}
	if v, ok := updates["email"].(string); ok {
		u.Email = v
	}
	if v, ok := updates["password"].(string); ok {
		u.Password = v
	}
	return nil
}

func (m *mockUserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := m.usersByID[id]; !ok {
		return constant.ErrNotFound
	}
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

	// 1. Success login with email
	logged, err := svc.Login(context.Background(), "login@example.com", "correctpassword")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}
	if logged.Email != user.Email {
		t.Errorf("expected email %s, got %s", user.Email, logged.Email)
	}

	// 1.1 Success login with username
	user2 := &model.User{
		Username: "loginuser2",
		Email:    "login2@example.com",
		Password: "correctpassword",
	}
	_ = user2.HashPassword()
	_ = repo.Create(context.Background(), user2)

	loggedUser, err := svc.Login(context.Background(), "loginuser2", "correctpassword")
	if err != nil {
		t.Fatalf("failed to login by username: %v", err)
	}
	if loggedUser.Username != user2.Username {
		t.Errorf("expected username %s, got %s", user2.Username, loggedUser.Username)
	}

	// 2. Failure login - incorrect password
	_, err = svc.Login(context.Background(), "login@example.com", "wrongpassword")
	if !errors.Is(err, constant.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_ChangePassword(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewAuthService(repo)
	ctx := context.Background()

	user := &model.User{Username: "pwuser", Email: "pw@example.com", Password: "oldpassword"}
	_ = user.HashPassword()
	_ = repo.Create(ctx, user)

	// Wrong current password
	err := svc.ChangePassword(ctx, user.ID, "wrongpassword", "newpassword123")
	if !errors.Is(err, constant.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}

	// Same password
	err = svc.ChangePassword(ctx, user.ID, "oldpassword", "oldpassword")
	if !errors.Is(err, constant.ErrSamePassword) {
		t.Errorf("expected ErrSamePassword, got %v", err)
	}

	// Success: new password logs in, old one no longer does
	if err := svc.ChangePassword(ctx, user.ID, "oldpassword", "newpassword123"); err != nil {
		t.Fatalf("failed to change password: %v", err)
	}
	if _, ok := repo.lastUpdates["password"]; !ok || len(repo.lastUpdates) != 1 {
		t.Errorf("expected only the password column to be updated, got %v", repo.lastUpdates)
	}
	if _, err := svc.Login(ctx, "pw@example.com", "newpassword123"); err != nil {
		t.Errorf("expected login with new password to succeed, got %v", err)
	}
	if _, err := svc.Login(ctx, "pw@example.com", "oldpassword"); !errors.Is(err, constant.ErrInvalidCredentials) {
		t.Errorf("expected old password to be rejected, got %v", err)
	}

	// Supabase-synced user (empty local hash)
	sbUser := &model.User{Username: "sbuser", Email: "sb@example.com"}
	_ = repo.Create(ctx, sbUser)
	err = svc.ChangePassword(ctx, sbUser.ID, "anything", "newpassword123")
	if !errors.Is(err, constant.ErrPasswordManagedExternally) {
		t.Errorf("expected ErrPasswordManagedExternally, got %v", err)
	}

	// Unknown user
	err = svc.ChangePassword(ctx, uuid.New(), "oldpassword", "newpassword123")
	if !errors.Is(err, constant.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
