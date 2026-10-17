package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
}

func TestRateLimiter_BlocksAfterLimitAndRefills(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := NewRateLimiter(3, time.Minute, ClientIPKey, "")
	l.now = func() time.Time { return now }
	h := l.Middleware(okHandler())

	do := func(remote string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.RemoteAddr = remote
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	for i := 0; i < 3; i++ {
		if rr := do("203.0.113.5:1234"); rr.Code != http.StatusOK {
			t.Fatalf("request %d should pass, got %d", i, rr.Code)
		}
	}
	rr := do("203.0.113.5:1234")
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}

	// A different IP has its own bucket.
	if rr := do("203.0.113.6:1234"); rr.Code != http.StatusOK {
		t.Fatalf("other IP should pass, got %d", rr.Code)
	}

	// 20s later one token (3/min) has refilled.
	now = now.Add(20 * time.Second)
	if rr := do("203.0.113.5:1234"); rr.Code != http.StatusOK {
		t.Fatalf("expected refill after 20s, got %d", rr.Code)
	}
	if rr := do("203.0.113.5:1234"); rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 again, got %d", rr.Code)
	}
}

func TestRateLimiter_SweepRemovesIdleBuckets(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	l := NewRateLimiter(10, time.Minute, ClientIPKey, "")
	l.now = func() time.Time { return now }
	l.allow("a")
	now = now.Add(10 * time.Minute)
	l.allow("b")
	if _, ok := l.buckets["a"]; ok {
		t.Fatal("idle bucket should have been swept")
	}
}

func TestClientIP_TrustsForwardedOnlyFromLoopback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:5000"
	req.Header.Set("X-Forwarded-For", "198.51.100.7")
	if got := ClientIP(req); got != "198.51.100.7" {
		t.Fatalf("expected forwarded IP from local proxy, got %s", got)
	}

	req.Header.Set("X-Forwarded-For", "1.1.1.1, 198.51.100.8")
	if got := ClientIP(req); got != "198.51.100.8" {
		t.Fatalf("expected right-most forwarded IP, got %s", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.9:5000"
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	if got := ClientIP(req); got != "203.0.113.9" {
		t.Fatalf("must ignore X-Forwarded-For from non-loopback peer, got %s", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "[::1]:5000"
	if got := ClientIP(req); got != "::1" {
		t.Fatalf("expected loopback peer without header, got %s", got)
	}
}

func TestUserOrIPKey(t *testing.T) {
	id := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/chat/stream", nil)
	req.RemoteAddr = "203.0.113.1:1"
	if got := UserOrIPKey(req); got != "ip:203.0.113.1" {
		t.Fatalf("expected IP key, got %s", got)
	}
	req = req.WithContext(context.WithValue(req.Context(), constant.UserContextKey, model.User{ID: id}))
	if got := UserOrIPKey(req); got != "user:"+id.String() {
		t.Fatalf("expected user key, got %s", got)
	}
}
