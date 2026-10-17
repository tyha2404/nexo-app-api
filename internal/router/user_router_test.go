package router

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/handler"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"go.uber.org/zap"
)

type stubUserService struct{ listCalled bool }

func (s *stubUserService) Create(ctx context.Context, req dto.CreateUserRequest) (*model.User, error) {
	return &model.User{ID: uuid.New(), Username: req.Username, Email: req.Email, Role: "user"}, nil
}
func (s *stubUserService) Get(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return &model.User{ID: id}, nil
}
func (s *stubUserService) List(ctx context.Context, limit, offset int) ([]model.User, error) {
	s.listCalled = true
	return []model.User{}, nil
}
func (s *stubUserService) Update(ctx context.Context, id uuid.UUID, req dto.UpdateUserRequest) (*model.User, error) {
	return &model.User{ID: id}, nil
}
func (s *stubUserService) Delete(ctx context.Context, id uuid.UUID) error { return nil }

func tokenFor(t *testing.T, role string) string {
	t.Helper()
	tok, err := util.GenerateToken(&model.User{ID: uuid.New(), Username: "u", Email: "u@example.com", Role: role})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestUserRoutes_RequireAdmin(t *testing.T) {
	util.InitJWT(&config.Config{JwtSecret: "test_secret_for_router_testing_at_least_32_chars"})

	svc := &stubUserService{}
	r := chi.NewRouter()
	NewUserRouter(handler.NewUserHandler(svc, zap.NewNop()), zap.NewNop()).RegisterRoutes(r)

	id := uuid.New().String()
	routes := []struct{ method, path string }{
		{http.MethodGet, "/users"},
		{http.MethodPost, "/users"},
		{http.MethodGet, "/users/" + id},
		{http.MethodPut, "/users/" + id},
		{http.MethodDelete, "/users/" + id},
	}

	for _, rt := range routes {
		// No token -> 401
		req := httptest.NewRequest(rt.method, rt.path, bytes.NewBufferString(`{}`))
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token: expected 401, got %d", rt.method, rt.path, rr.Code)
		}

		// Regular user -> 403
		req = httptest.NewRequest(rt.method, rt.path, bytes.NewBufferString(`{}`))
		req.Header.Set("Authorization", "Bearer "+tokenFor(t, "user"))
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("%s %s as user: expected 403, got %d", rt.method, rt.path, rr.Code)
		}
	}

	// Admin -> allowed
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(t, "admin"))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !svc.listCalled {
		t.Fatalf("admin list: expected 200 and service call, got %d (called=%v)", rr.Code, svc.listCalled)
	}
}

func TestAuthRoutes_LoginIsRateLimited(t *testing.T) {
	ar := NewAuthRouter(handler.NewAuthHandler(nil, zap.NewNop()), zap.NewNop())
	r := chi.NewRouter()
	ar.RegisterRoutes(r)

	var last int
	for i := 0; i <= AuthLimitPerMinute; i++ {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{}`))
		req.RemoteAddr = "203.0.113.50:1234"
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		last = rr.Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after %d attempts, got %d", AuthLimitPerMinute, last)
	}
}
