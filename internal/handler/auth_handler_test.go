package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"go.uber.org/zap"
)

type mockAuthService struct {
	loginFn    func(ctx context.Context, email string, password string) (*model.User, error)
	registerFn func(ctx context.Context, user *model.User) (*model.User, error)
}

func (m *mockAuthService) Login(ctx context.Context, email string, password string) (*model.User, error) {
	if m.loginFn != nil {
		return m.loginFn(ctx, email, password)
	}
	return nil, nil
}

func (m *mockAuthService) Register(ctx context.Context, user *model.User) (*model.User, error) {
	if m.registerFn != nil {
		return m.registerFn(ctx, user)
	}
	return nil, nil
}

func TestAuthHandler_Login_Success(t *testing.T) {
	// Initialize JWT
	cfg := &config.Config{
		JwtSecret: "test_secret_for_handler_testing_at_least_32_chars",
	}
	util.InitJWT(cfg)

	mockSvc := &mockAuthService{
		loginFn: func(ctx context.Context, email string, password string) (*model.User, error) {
			return &model.User{
				ID:       uuid.New(),
				Username: "testuser",
				Email:    email,
				Role:     "user",
			}, nil
		},
	}

	logger := zap.NewNop()
	handler := NewAuthHandler(mockSvc, logger)

	reqBody, _ := json.Marshal(dto.LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
	})

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Login(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rr.Code)
	}

	var response map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &response)

	if response["success"] != true {
		t.Errorf("expected success to be true, got %v", response["success"])
	}
}

func TestAuthHandler_Login_Failure(t *testing.T) {
	mockSvc := &mockAuthService{
		loginFn: func(ctx context.Context, email string, password string) (*model.User, error) {
			return nil, errors.New("invalid credentials")
		},
	}

	logger := zap.NewNop()
	handler := NewAuthHandler(mockSvc, logger)

	reqBody, _ := json.Marshal(dto.LoginRequest{
		Email:    "test@example.com",
		Password: "wrongpassword",
	})

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Login(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rr.Code)
	}
}
