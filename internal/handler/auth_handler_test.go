package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func TestAuthHandler_Register(t *testing.T) {
	valid := `{"username":"new_user","email":"new@example.com","password":"password123"}`
	tests := []struct {
		name       string
		body       string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", valid, nil, http.StatusCreated, ""},
		{"bad json", `{"username":`, nil, http.StatusBadRequest, "Validation failed"},
		{"missing email", `{"username":"new_user","password":"password123"}`, nil, http.StatusBadRequest, "Validation failed"},
		{"short password", `{"username":"new_user","email":"new@example.com","password":"short"}`, nil, http.StatusBadRequest, "Validation failed"},
		{"username with spaces", `{"username":"new user","email":"new@example.com","password":"password123"}`, nil, http.StatusBadRequest, "Validation failed"},
		{"email exists", valid, constant.ErrEmailAlreadyExists, http.StatusConflict, "Email already exists"},
		{"username taken", valid, fmt.Errorf("register: %w", constant.ErrUsernameTaken), http.StatusConflict, "Username already taken"},
		{"internal error", valid, errors.New("db down"), http.StatusInternalServerError, "Failed to create user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *model.User
			mockSvc := &mockAuthService{
				registerFn: func(ctx context.Context, user *model.User) (*model.User, error) {
					got = user
					if tt.svcErr != nil {
						return nil, tt.svcErr
					}
					created := *user
					created.ID = uuid.New()
					created.Password = "$2a$hashed"
					created.Role = "user"
					return &created, nil
				},
			}
			h := NewAuthHandler(mockSvc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.Register(rr, httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(tt.body)))

			if rr.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d (%s)", tt.wantStatus, rr.Code, rr.Body.String())
			}
			body := decodeJSON(t, rr)
			if tt.wantStatus != http.StatusCreated {
				if body["success"] != false || body["message"] != tt.wantMsg {
					t.Fatalf("unexpected error envelope: %v", body)
				}
				if tt.wantStatus == http.StatusBadRequest && got != nil {
					t.Fatal("service must not be called for an invalid payload")
				}
				if _, leaked := body["error"]; leaked && tt.wantStatus >= 500 {
					t.Fatalf("5xx must not expose error detail: %v", body["error"])
				}
				return
			}
			if got == nil || got.Username != "new_user" || got.Email != "new@example.com" || got.Password != "password123" {
				t.Fatalf("unexpected user passed to service: %+v", got)
			}
			if got.Role != "" {
				t.Fatalf("register must not set a role from the request, got %q", got.Role)
			}
			if strings.Contains(rr.Body.String(), "hashed") || strings.Contains(rr.Body.String(), "password123") {
				t.Fatalf("password must not be returned: %s", rr.Body.String())
			}
			data, _ := body["data"].(map[string]interface{})
			if body["success"] != true || data["username"] != "new_user" {
				t.Fatalf("unexpected success envelope: %v", body)
			}
		})
	}
}

func TestAuthHandler_RegisterIgnoresRoleInBody(t *testing.T) {
	var got *model.User
	mockSvc := &mockAuthService{
		registerFn: func(ctx context.Context, user *model.User) (*model.User, error) {
			got = user
			return user, nil
		},
	}
	h := NewAuthHandler(mockSvc, zap.NewNop())
	rr := httptest.NewRecorder()
	body := `{"username":"sneaky","email":"s@example.com","password":"password123","role":"admin"}`
	h.Register(rr, httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(body)))

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rr.Code, rr.Body.String())
	}
	if got == nil || got.Role != "" {
		t.Fatalf("role from the request body must be ignored, got %+v", got)
	}
}

func TestAuthHandler_WhoAmI(t *testing.T) {
	h := NewAuthHandler(&mockAuthService{}, zap.NewNop())
	userID := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/auth/whoami", nil)
	req = req.WithContext(context.WithValue(req.Context(), constant.UserContextKey, model.User{ID: userID, Username: "me", Role: "user", Password: "secret-hash"}))
	rr := httptest.NewRecorder()
	h.WhoAmI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := decodeJSON(t, rr)
	data, _ := body["data"].(map[string]interface{})
	if body["success"] != true || data["id"] != userID.String() || data["username"] != "me" {
		t.Fatalf("unexpected body: %v", body)
	}
	if strings.Contains(rr.Body.String(), "secret-hash") {
		t.Fatal("password must not be serialized")
	}

	for name, r := range map[string]*http.Request{
		"no user":  httptest.NewRequest(http.MethodGet, "/auth/whoami", nil),
		"nil uuid": withUser(httptest.NewRequest(http.MethodGet, "/auth/whoami", nil), uuid.Nil),
	} {
		rr := httptest.NewRecorder()
		h.WhoAmI(rr, r)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d", name, rr.Code)
		}
	}
}
