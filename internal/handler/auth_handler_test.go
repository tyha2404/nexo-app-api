package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"go.uber.org/zap"
)

type mockAuthService struct {
	loginFn    func(ctx context.Context, email string, password string) (*model.User, error)
	registerFn func(ctx context.Context, user *model.User) (*model.User, error)
	changePwFn func(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error
	startFn    func(ctx context.Context, userID uuid.UUID, userAgent string) (string, error)
	refreshFn  func(ctx context.Context, refreshToken, userAgent string) (*model.User, string, error)
	endFn      func(ctx context.Context, refreshToken string) error
}

func (m *mockAuthService) StartSession(ctx context.Context, userID uuid.UUID, userAgent string) (string, error) {
	if m.startFn != nil {
		return m.startFn(ctx, userID, userAgent)
	}
	return "refresh-" + userID.String(), nil
}

func (m *mockAuthService) RefreshSession(ctx context.Context, refreshToken, userAgent string) (*model.User, string, error) {
	if m.refreshFn != nil {
		return m.refreshFn(ctx, refreshToken, userAgent)
	}
	return nil, "", constant.ErrInvalidRefreshToken
}

func (m *mockAuthService) EndSession(ctx context.Context, refreshToken string) error {
	if m.endFn != nil {
		return m.endFn(ctx, refreshToken)
	}
	return nil
}

func (m *mockAuthService) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	if m.changePwFn != nil {
		return m.changePwFn(ctx, userID, currentPassword, newPassword)
	}
	return nil
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
	data, _ := response["data"].(map[string]interface{})
	if data["token"] == "" || data["token"] == nil {
		t.Errorf("expected an access token, got %v", data["token"])
	}
	if rt, _ := data["refreshToken"].(string); !strings.HasPrefix(rt, "refresh-") {
		t.Errorf("expected the session's refresh token, got %v", data["refreshToken"])
	}
	if data["expiresIn"] != float64(util.AccessTokenTTL().Seconds()) {
		t.Errorf("expected expiresIn %v, got %v", util.AccessTokenTTL().Seconds(), data["expiresIn"])
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

func TestAuthHandler_ChangePassword(t *testing.T) {
	userID := uuid.New()
	tests := []struct {
		name       string
		withUser   bool
		body       dto.ChangePasswordRequest
		svcErr     error
		wantStatus int
	}{
		{"success", true, dto.ChangePasswordRequest{CurrentPassword: "oldpassword", NewPassword: "newpassword123"}, nil, http.StatusOK},
		{"unauthenticated", false, dto.ChangePasswordRequest{CurrentPassword: "oldpassword", NewPassword: "newpassword123"}, nil, http.StatusUnauthorized},
		{"new password too short", true, dto.ChangePasswordRequest{CurrentPassword: "oldpassword", NewPassword: "short"}, nil, http.StatusBadRequest},
		{"wrong current password", true, dto.ChangePasswordRequest{CurrentPassword: "wrong", NewPassword: "newpassword123"}, constant.ErrInvalidCredentials, http.StatusBadRequest},
		{"same password", true, dto.ChangePasswordRequest{CurrentPassword: "newpassword123", NewPassword: "newpassword123"}, constant.ErrSamePassword, http.StatusBadRequest},
		{"supabase account", true, dto.ChangePasswordRequest{CurrentPassword: "oldpassword", NewPassword: "newpassword123"}, constant.ErrPasswordManagedExternally, http.StatusConflict},
		{"internal error", true, dto.ChangePasswordRequest{CurrentPassword: "oldpassword", NewPassword: "newpassword123"}, errors.New("db down"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUserID uuid.UUID
			mockSvc := &mockAuthService{
				changePwFn: func(ctx context.Context, id uuid.UUID, currentPassword, newPassword string) error {
					gotUserID = id
					return tt.svcErr
				},
			}
			handler := NewAuthHandler(mockSvc, zap.NewNop())

			reqBody, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/auth/change-password", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")
			if tt.withUser {
				req = req.WithContext(context.WithValue(req.Context(), constant.UserContextKey, model.User{ID: userID}))
			}
			rr := httptest.NewRecorder()

			handler.ChangePassword(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("expected %d, got %d (%s)", tt.wantStatus, rr.Code, rr.Body.String())
			}
			if tt.wantStatus == http.StatusOK {
				if gotUserID != userID {
					t.Errorf("expected service to be called for user %s, got %s", userID, gotUserID)
				}
				if !strings.Contains(rr.Body.String(), `"refreshToken":"refresh-`) {
					t.Errorf("expected a new session in the response, got %s", rr.Body.String())
				}
			}
		})
	}
}

func TestAuthHandler_Refresh(t *testing.T) {
	util.InitJWT(&config.Config{JwtSecret: "test_secret_for_handler_testing_at_least_32_chars"})
	user := &model.User{ID: uuid.New(), Username: "refresher", Email: "r@example.com", Role: "user"}

	tests := []struct {
		name       string
		body       string
		svcErr     error
		wantStatus int
	}{
		{"success", `{"refreshToken":"good"}`, nil, http.StatusOK},
		{"missing token", `{}`, nil, http.StatusBadRequest},
		{"invalid token", `{"refreshToken":"bad"}`, constant.ErrInvalidRefreshToken, http.StatusUnauthorized},
		{"internal error", `{"refreshToken":"good"}`, errors.New("db down"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUA string
			mockSvc := &mockAuthService{
				refreshFn: func(ctx context.Context, refreshToken, userAgent string) (*model.User, string, error) {
					gotUA = userAgent
					if tt.svcErr != nil {
						return nil, "", tt.svcErr
					}
					return user, "rotated", nil
				},
			}
			h := NewAuthHandler(mockSvc, zap.NewNop())
			req := httptest.NewRequest("POST", "/auth/refresh", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "test-agent")
			rr := httptest.NewRecorder()

			h.Refresh(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d (%s)", tt.wantStatus, rr.Code, rr.Body.String())
			}
			if tt.wantStatus != http.StatusOK {
				return
			}
			if gotUA != "test-agent" {
				t.Errorf("expected user agent to reach the service, got %q", gotUA)
			}
			var resp struct {
				Data dto.TokenResponse `json:"data"`
			}
			_ = json.Unmarshal(rr.Body.Bytes(), &resp)
			if resp.Data.RefreshToken != "rotated" {
				t.Errorf("expected rotated refresh token, got %q", resp.Data.RefreshToken)
			}
			claims, err := util.ValidateToken(resp.Data.Token)
			if err != nil || claims.ID != user.ID {
				t.Errorf("expected a valid access token for the user, got %v / %v", claims, err)
			}
		})
	}
}

func TestAuthHandler_Logout(t *testing.T) {
	var ended string
	mockSvc := &mockAuthService{
		endFn: func(ctx context.Context, refreshToken string) error {
			ended = refreshToken
			return nil
		},
	}
	h := NewAuthHandler(mockSvc, zap.NewNop())

	req := httptest.NewRequest("POST", "/auth/logout", strings.NewReader(`{"refreshToken":"bye"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Logout(rr, req)
	if rr.Code != http.StatusOK || ended != "bye" {
		t.Errorf("expected 200 and the session ended, got %d / %q", rr.Code, ended)
	}

	req = httptest.NewRequest("POST", "/auth/logout", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	h.Logout(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 without a refresh token, got %d", rr.Code)
	}
}
