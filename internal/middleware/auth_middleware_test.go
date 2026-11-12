package middleware

import (
	"context"
	"fmt"
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

// Auth errors use the standard error envelope (response.SendError) with the
// same status codes as before.
func TestAuthErrors_UseStandardEnvelope(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	admin := func(r *http.Request) *http.Request {
		return r.WithContext(context.WithValue(r.Context(), constant.UserContextKey, model.User{ID: uuid.New(), Role: "user"}))
	}
	tests := []struct {
		name    string
		handler http.Handler
		req     func() *http.Request
		status  int
		msg     string
	}{
		{"missing header", AuthMiddleware(next), func() *http.Request { return httptest.NewRequest("GET", "/x", nil) },
			http.StatusUnauthorized, "Authorization header is required"},
		{"bad format", AuthMiddleware(next), func() *http.Request {
			r := httptest.NewRequest("GET", "/x", nil)
			r.Header.Set("Authorization", "Token abc")
			return r
		}, http.StatusUnauthorized, "Authorization header format must be 'Bearer {token}'"},
		{"invalid token", AuthMiddleware(next), func() *http.Request {
			r := httptest.NewRequest("GET", "/x", nil)
			r.Header.Set("Authorization", "Bearer not-a-jwt")
			return r
		}, http.StatusUnauthorized, "Invalid or expired token"},
		{"no user", AdminOnly(next), func() *http.Request { return httptest.NewRequest("GET", "/x", nil) },
			http.StatusUnauthorized, "User not found in context"},
		{"not admin", AdminOnly(next), func() *http.Request { return admin(httptest.NewRequest("GET", "/x", nil)) },
			http.StatusForbidden, "Admin access required"},
		{"role re-check", RequireCurrentRole(model.RoleAdmin, func(context.Context, uuid.UUID) (string, error) { return "user", nil })(next),
			func() *http.Request { return admin(httptest.NewRequest("GET", "/x", nil)) }, http.StatusForbidden, "Admin access required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			tt.handler.ServeHTTP(rr, tt.req())
			if rr.Code != tt.status {
				t.Fatalf("status = %d, want %d", rr.Code, tt.status)
			}
			if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type = %q", ct)
			}
			want := fmt.Sprintf(`{"status":%d,"success":false,"message":%q,"error":%q}`+"\n", tt.status, tt.msg, tt.msg)
			if got := rr.Body.String(); got != want {
				t.Fatalf("body = %s, want %s", got, want)
			}
		})
	}
}
