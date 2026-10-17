package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func corsTestHandler(origins []string) http.Handler {
	return NewCORSMiddleware(origins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

func TestCORS_AllowedOriginIsReflected(t *testing.T) {
	h := corsTestHandler([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("expected origin to be reflected, got %q", got)
	}
	if rr.Header().Get("Vary") != "Origin" {
		t.Fatalf("expected Vary: Origin")
	}
}

func TestCORS_DisallowedOriginGetsNoHeaders(t *testing.T) {
	h := corsTestHandler([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "https://evil.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no ACAO header, got %q", got)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("non-preflight request should still reach handler, got %d", rr.Code)
	}
}

func TestCORS_Preflight(t *testing.T) {
	h := corsTestHandler([]string{"http://localhost:5173"})
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/transactions", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for preflight, got %d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatal("expected allow-methods header")
	}
}

func TestCORS_NoOriginPassesThrough(t *testing.T) {
	h := corsTestHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("unexpected response: %d %q", rr.Code, rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_Wildcard(t *testing.T) {
	h := corsTestHandler([]string{"*"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://anything.test")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Header().Get("Access-Control-Allow-Origin") != "http://anything.test" {
		t.Fatal("wildcard should allow any origin")
	}
}
