package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type stubUserService struct {
	service.UserService
	createFn func(ctx context.Context, req dto.CreateUserRequest) (*model.User, error)
	getFn    func(ctx context.Context, id uuid.UUID) (*model.User, error)
	listFn   func(ctx context.Context, limit, offset int) ([]model.User, error)
	updateFn func(ctx context.Context, id uuid.UUID, req dto.UpdateUserRequest) (*model.User, error)
	deleteFn func(ctx context.Context, id uuid.UUID) error
}

func (s *stubUserService) Create(ctx context.Context, req dto.CreateUserRequest) (*model.User, error) {
	return s.createFn(ctx, req)
}
func (s *stubUserService) Get(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return s.getFn(ctx, id)
}
func (s *stubUserService) List(ctx context.Context, limit, offset int) ([]model.User, error) {
	return s.listFn(ctx, limit, offset)
}
func (s *stubUserService) Update(ctx context.Context, id uuid.UUID, req dto.UpdateUserRequest) (*model.User, error) {
	return s.updateFn(ctx, id, req)
}
func (s *stubUserService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.deleteFn(ctx, id)
}

func TestUserHandler_WriteUserConflict(t *testing.T) {
	h := NewUserHandler(&stubUserService{}, zap.NewNop())
	tests := []struct {
		name       string
		err        error
		wantOK     bool
		wantStatus int
		wantMsg    string
	}{
		{"email", constant.ErrEmailAlreadyExists, true, http.StatusConflict, "Email already exists"},
		{"wrapped email", fmt.Errorf("create: %w", constant.ErrEmailAlreadyExists), true, http.StatusConflict, "Email already exists"},
		{"username", constant.ErrUsernameTaken, true, http.StatusConflict, "Username already taken"},
		{"other", errors.New("db"), false, 0, ""},
		{"not found", constant.ErrNotFound, false, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			ok := h.writeUserConflict(rr, tt.err, "op")
			assert.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				assert.Empty(t, rr.Body.String(), "nothing must be written when not a conflict")
				return
			}
			categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
		})
	}
}

func TestUserHandler_Create(t *testing.T) {
	valid := `{"username":"john_doe","email":"john@example.com","password":"password123"}`
	tests := []struct {
		name       string
		body       string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", valid, nil, http.StatusCreated, ""},
		{"bad json", `{"username":`, nil, http.StatusBadRequest, "Validation failed"},
		{"bad email", `{"username":"john","email":"nope","password":"password123"}`, nil, http.StatusBadRequest, "Validation failed"},
		{"short password", `{"username":"john","email":"j@e.com","password":"short"}`, nil, http.StatusBadRequest, "Validation failed"},
		{"bad username chars", `{"username":"john doe!","email":"j@e.com","password":"password123"}`, nil, http.StatusBadRequest, "Validation failed"},
		{"email conflict", valid, constant.ErrEmailAlreadyExists, http.StatusConflict, "Email already exists"},
		{"username conflict", valid, constant.ErrUsernameTaken, http.StatusConflict, "Username already taken"},
		{"service error", valid, errors.New("db"), http.StatusInternalServerError, "Failed to create user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			var gotReq dto.CreateUserRequest
			svc := &stubUserService{createFn: func(ctx context.Context, req dto.CreateUserRequest) (*model.User, error) {
				called, gotReq = true, req
				if tt.svcErr != nil {
					return nil, tt.svcErr
				}
				return &model.User{ID: uuid.New(), Username: req.Username, Email: req.Email, Password: "hashed-secret", Role: "user"}, nil
			}}
			h := NewUserHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.Create(rr, httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(tt.body)))

			if tt.wantStatus != http.StatusCreated {
				categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				if tt.wantStatus == http.StatusBadRequest {
					assert.False(t, called)
				}
				return
			}
			require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
			assert.Equal(t, "john_doe", gotReq.Username)
			assert.NotContains(t, rr.Body.String(), "hashed-secret", "password hash must never be serialized")
			body := decodeJSON(t, rr)
			assert.Equal(t, true, body["success"])
			assert.Equal(t, "john_doe", body["data"].(map[string]interface{})["username"])
		})
	}
}

func TestUserHandler_Get(t *testing.T) {
	id := uuid.New()
	tests := []struct {
		name       string
		id         string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", id.String(), nil, http.StatusOK, ""},
		{"invalid uuid", "x", nil, http.StatusBadRequest, "Invalid user ID"},
		{"not found", id.String(), gorm.ErrRecordNotFound, http.StatusNotFound, "User not found"},
		{"service error", id.String(), errors.New("db"), http.StatusInternalServerError, "Failed to get user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotID uuid.UUID
			svc := &stubUserService{getFn: func(ctx context.Context, uid uuid.UUID) (*model.User, error) {
				gotID = uid
				if tt.svcErr != nil {
					return nil, tt.svcErr
				}
				return &model.User{ID: uid, Username: "bob", Password: "hashed-secret"}, nil
			}}
			h := NewUserHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.Get(rr, withURLParam(httptest.NewRequest(http.MethodGet, "/users/x", nil), "id", tt.id))

			if tt.wantStatus != http.StatusOK {
				categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, id, gotID)
			assert.NotContains(t, rr.Body.String(), "hashed-secret")
			assert.Equal(t, id.String(), decodeJSON(t, rr)["data"].(map[string]interface{})["id"])
		})
	}
}

func TestUserHandler_List(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantLimit  int
		wantOffset int
		wantPage   int
	}{
		{"defaults", "", DefaultPageLimit, 0, 1},
		{"page 3", "?page=3&limit=20", 20, 40, 3},
		{"legacy offset overrides page", "?page=5&limit=10&offset=25", 10, 25, 3},
		{"negative offset ignored", "?page=2&limit=10&offset=-1", 10, 10, 2},
		{"limit capped", "?limit=9999", MaxPageLimit, 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotLimit, gotOffset int
			svc := &stubUserService{listFn: func(ctx context.Context, limit, offset int) ([]model.User, error) {
				gotLimit, gotOffset = limit, offset
				return []model.User{{Username: "a"}, {Username: "b"}}, nil
			}}
			h := NewUserHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.List(rr, httptest.NewRequest(http.MethodGet, "/users"+tt.query, nil))

			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, tt.wantLimit, gotLimit)
			assert.Equal(t, tt.wantOffset, gotOffset)
			body := decodeJSON(t, rr)
			assert.Equal(t, float64(tt.wantPage), body["page"])
			assert.Equal(t, float64(tt.wantLimit), body["limit"])
			assert.Equal(t, float64(2), body["total"])
		})
	}

	t.Run("service error", func(t *testing.T) {
		svc := &stubUserService{listFn: func(ctx context.Context, limit, offset int) ([]model.User, error) {
			return nil, errors.New("db")
		}}
		h := NewUserHandler(svc, zap.NewNop())
		rr := httptest.NewRecorder()
		h.List(rr, httptest.NewRequest(http.MethodGet, "/users", nil))
		categoryAssertError(t, rr, http.StatusInternalServerError, "Failed to list users")
	})
}

func TestUserHandler_UpdateErrors(t *testing.T) {
	id := uuid.New()
	tests := []struct {
		name       string
		id         string
		body       string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"invalid uuid", "x", `{"username":"newname"}`, nil, http.StatusBadRequest, "Invalid user ID"},
		{"bad json", id.String(), `{`, nil, http.StatusBadRequest, "Validation failed"},
		{"bad email", id.String(), `{"email":"nope"}`, nil, http.StatusBadRequest, "Validation failed"},
		{"email conflict", id.String(), `{"email":"a@b.com"}`, constant.ErrEmailAlreadyExists, http.StatusConflict, "Email already exists"},
		{"username conflict", id.String(), `{"username":"taken"}`, constant.ErrUsernameTaken, http.StatusConflict, "Username already taken"},
		{"not found", id.String(), `{"username":"newname"}`, constant.ErrNotFound, http.StatusNotFound, "User not found"},
		{"service error", id.String(), `{"username":"newname"}`, errors.New("db"), http.StatusInternalServerError, "Failed to update user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubUserService{updateFn: func(ctx context.Context, uid uuid.UUID, req dto.UpdateUserRequest) (*model.User, error) {
				return nil, tt.svcErr
			}}
			h := NewUserHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.Update(rr, withURLParam(httptest.NewRequest(http.MethodPut, "/users/x", strings.NewReader(tt.body)), "id", tt.id))
			categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
		})
	}
}

func TestUserHandler_Delete(t *testing.T) {
	id := uuid.New()
	tests := []struct {
		name       string
		id         string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", id.String(), nil, http.StatusNoContent, ""},
		{"invalid uuid", "x", nil, http.StatusBadRequest, "Invalid user ID"},
		{"not found", id.String(), constant.ErrNotFound, http.StatusNotFound, "User not found"},
		{"service error", id.String(), errors.New("db"), http.StatusInternalServerError, "Failed to delete user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotID uuid.UUID
			svc := &stubUserService{deleteFn: func(ctx context.Context, uid uuid.UUID) error {
				gotID = uid
				return tt.svcErr
			}}
			h := NewUserHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.Delete(rr, withURLParam(httptest.NewRequest(http.MethodDelete, "/users/x", nil), "id", tt.id))

			if tt.wantStatus != http.StatusNoContent {
				categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusNoContent, rr.Code)
			assert.Equal(t, id, gotID)
		})
	}
}
