package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

func strp(s string) *string { return &s }

func TestUserService_UpdateOnlyTouchesWhitelistedFields(t *testing.T) {
	repo := newMockUserRepo()
	existing := &model.User{ID: uuid.New(), Username: "alice", Email: "alice@example.com", Password: "$2a$hash", Role: "user"}
	_ = repo.Create(context.Background(), existing)
	svc := NewUserService(repo)

	updated, err := svc.Update(context.Background(), existing.ID, dto.UpdateUserRequest{Username: strp("alice2")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Username != "alice2" {
		t.Fatalf("username not updated: %q", updated.Username)
	}
	for key := range repo.lastUpdates {
		if key != "username" && key != "email" {
			t.Fatalf("update must only include whitelisted columns, got %q", key)
		}
	}
	stored := repo.usersByID[existing.ID]
	if stored.Password != "$2a$hash" || stored.Role != "user" {
		t.Fatalf("password/role must be untouched, got password=%q role=%q", stored.Password, stored.Role)
	}
}

func TestUserService_UpdateConflictsAndNotFound(t *testing.T) {
	repo := newMockUserRepo()
	a := &model.User{ID: uuid.New(), Username: "alice", Email: "a@example.com"}
	b := &model.User{ID: uuid.New(), Username: "bob", Email: "b@example.com"}
	_ = repo.Create(context.Background(), a)
	_ = repo.Create(context.Background(), b)
	svc := NewUserService(repo)

	if _, err := svc.Update(context.Background(), a.ID, dto.UpdateUserRequest{Username: strp("bob")}); !errors.Is(err, constant.ErrUsernameTaken) {
		t.Fatalf("expected ErrUsernameTaken, got %v", err)
	}
	if _, err := svc.Update(context.Background(), a.ID, dto.UpdateUserRequest{Email: strp("b@example.com")}); !errors.Is(err, constant.ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
	}
	if _, err := svc.Update(context.Background(), uuid.New(), dto.UpdateUserRequest{Username: strp("carol")}); !errors.Is(err, constant.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if _, err := svc.Update(context.Background(), a.ID, dto.UpdateUserRequest{}); !errors.Is(err, constant.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty update, got %v", err)
	}
}

func TestUserService_CreateHashesPasswordAndForcesUserRole(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewUserService(repo)

	u, err := svc.Create(context.Background(), dto.CreateUserRequest{Username: "dave", Email: "d@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Password != "" {
		t.Fatal("returned user must not carry the password hash")
	}
	stored := repo.usersByID[u.ID]
	if stored.Role != "user" {
		t.Fatalf("expected role user, got %q", stored.Role)
	}
	if err := (&model.User{Password: stored.Password}).CheckPassword("password123"); err != nil {
		t.Fatal("password must be stored as a bcrypt hash")
	}
}
