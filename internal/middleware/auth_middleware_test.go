package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/config"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	handlerToTest := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Next handler should not be called when Authorization header is missing")
	}))

	req := httptest.NewRequest("GET", "/any-endpoint", nil)
	rr := httptest.NewRecorder()

	handlerToTest.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	cfg := &config.Config{
		JwtSecret: "test_secret_key_at_least_32_characters_long",
	}
	util.InitJWT(cfg)

	user := &model.User{
		ID:       uuid.New(),
		Username: "john_doe",
		Email:    "john@example.com",
		Role:     "user",
	}

	token, _ := util.GenerateToken(user)

	nextHandlerCalled := false
	handlerToTest := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		ctxUser, ok := r.Context().Value(constant.UserContextKey).(model.User)
		if !ok {
			t.Fatal("User not found in request context")
		}
		if ctxUser.ID != user.ID {
			t.Errorf("expected user ID %v, got %v", user.ID, ctxUser.ID)
		}
		if ctxUser.Role != user.Role {
			t.Errorf("expected user role %s, got %s", user.Role, ctxUser.Role)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/any-endpoint", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handlerToTest.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if !nextHandlerCalled {
		t.Fatal("expected next handler to be called")
	}
}

func TestAdminOnly_Middleware(t *testing.T) {
	handlerToTest := AdminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Case 1: Non-admin user
	nonAdminUser := model.User{
		ID:   uuid.New(),
		Role: "user",
	}
	req := httptest.NewRequest("GET", "/admin-only", nil)
	ctx := context.WithValue(req.Context(), constant.UserContextKey, nonAdminUser)
	rr := httptest.NewRecorder()

	handlerToTest.ServeHTTP(rr, req.WithContext(ctx))

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for non-admin, got %d", rr.Code)
	}

	// Case 2: Admin user
	adminUser := model.User{
		ID:   uuid.New(),
		Role: "admin",
	}
	reqAdmin := httptest.NewRequest("GET", "/admin-only", nil)
	ctxAdmin := context.WithValue(reqAdmin.Context(), constant.UserContextKey, adminUser)
	rrAdmin := httptest.NewRecorder()

	handlerToTest.ServeHTTP(rrAdmin, reqAdmin.WithContext(ctxAdmin))

	if rrAdmin.Code != http.StatusOK {
		t.Errorf("expected 200 OK for admin, got %d", rrAdmin.Code)
	}
}

func TestRequireCurrentRole_UsesDatabaseRole(t *testing.T) {
	dbRoles := map[uuid.UUID]string{}
	lookup := func(_ context.Context, id uuid.UUID) (string, error) {
		role, ok := dbRoles[id]
		if !ok {
			return "", constant.ErrNotFound
		}
		return role, nil
	}
	handlerToTest := RequireCurrentRole("admin", lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	serve := func(id uuid.UUID) int {
		req := httptest.NewRequest("GET", "/admin-only", nil)
		// The token still says admin in every case.
		ctx := context.WithValue(req.Context(), constant.UserContextKey, model.User{ID: id, Role: "admin"})
		rr := httptest.NewRecorder()
		handlerToTest.ServeHTTP(rr, req.WithContext(ctx))
		return rr.Code
	}

	admin, demoted, deleted := uuid.New(), uuid.New(), uuid.New()
	dbRoles[admin] = "admin"
	dbRoles[demoted] = "user"

	if code := serve(admin); code != http.StatusOK {
		t.Errorf("current admin: expected 200, got %d", code)
	}
	if code := serve(demoted); code != http.StatusForbidden {
		t.Errorf("demoted admin: expected 403, got %d", code)
	}
	if code := serve(deleted); code != http.StatusForbidden {
		t.Errorf("deleted admin: expected 403, got %d", code)
	}
}
