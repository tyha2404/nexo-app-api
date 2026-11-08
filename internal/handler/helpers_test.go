package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

func TestParsePagination(t *testing.T) {
	tests := []struct {
		query     string
		wantPage  int
		wantLimit int
	}{
		{"", 1, DefaultPageLimit},
		{"?page=3&limit=20", 3, 20},
		{"?page=0&limit=0", 1, DefaultPageLimit},
		{"?page=-2&limit=-5", 1, DefaultPageLimit},
		{"?page=abc&limit=xyz", 1, DefaultPageLimit},
		{"?limit=50", 1, 50},
		{"?limit=100", 1, 100},
		{"?limit=101", 1, MaxPageLimit},
		{"?page=2&limit=100000", 2, MaxPageLimit},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/items"+tt.query, nil)
			page, limit := ParsePagination(r)
			if page != tt.wantPage || limit != tt.wantLimit {
				t.Fatalf("ParsePagination(%q) = (%d, %d), want (%d, %d)", tt.query, page, limit, tt.wantPage, tt.wantLimit)
			}
		})
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"constant.ErrNotFound", constant.ErrNotFound, true},
		{"wrapped ErrNotFound", fmt.Errorf("budget %w", constant.ErrNotFound), true},
		{"gorm.ErrRecordNotFound", gorm.ErrRecordNotFound, true},
		{"wrapped gorm", fmt.Errorf("source wallet not found: %w", gorm.ErrRecordNotFound), true},
		{"other", errors.New("boom"), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFoundError(tt.err); got != tt.want {
				t.Fatalf("IsNotFoundError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestParseQueryInt(t *testing.T) {
	tests := []struct {
		query string
		want  int
	}{
		{"", 7},
		{"?n=3", 3},
		{"?n=0", 0},
		{"?n=-1", 7},
		{"?n=abc", 7},
		{"?n=2.5", 7},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/x"+tt.query, nil)
			if got := ParseQueryInt(r, "n", 7); got != tt.want {
				t.Fatalf("ParseQueryInt(%q) = %d, want %d", tt.query, got, tt.want)
			}
		})
	}
}

func TestParseQueryIntWithValidation(t *testing.T) {
	tests := []struct {
		query string
		min   int
		want  int
	}{
		{"", 1, 12},
		{"?n=5", 1, 5},
		{"?n=1", 1, 1},
		{"?n=0", 1, 12},
		{"?n=-3", -5, -3},
		{"?n=-6", -5, 12},
		{"?n=x", 1, 12},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_min%d", tt.query, tt.min), func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/x"+tt.query, nil)
			if got := ParseQueryIntWithValidation(r, "n", 12, tt.min); got != tt.want {
				t.Fatalf("ParseQueryIntWithValidation(%q, min=%d) = %d, want %d", tt.query, tt.min, got, tt.want)
			}
		})
	}
}

func TestBuildFilterMap(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x?type=EXPENSE&keyword=&userId=evil&tag=a&tag=b", nil)
	got := BuildFilterMap(r, []string{"type", "keyword", "tag", "status"})
	want := map[string]interface{}{"type": "EXPENSE", "tag": "a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildFilterMap = %v, want %v (only listed, non-empty keys; first value wins)", got, want)
	}
	if empty := BuildFilterMap(r, nil); len(empty) != 0 {
		t.Fatalf("no keys must give an empty map, got %v", empty)
	}
}

func TestValidateRequired(t *testing.T) {
	if err := ValidateRequired(map[string]string{"a": "1", "b": "2"}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := ValidateRequired(nil); err != nil {
		t.Fatalf("expected nil for no fields, got %v", err)
	}
	if err := ValidateRequired(map[string]string{"a": "1", "b": ""}); !errors.Is(err, constant.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetUserFromContext(t *testing.T) {
	id := uuid.New()
	tests := []struct {
		name    string
		ctxVal  interface{}
		wantErr bool
	}{
		{"valid user", model.User{ID: id}, false},
		{"missing", nil, true},
		{"nil id", model.User{}, true},
		{"pointer instead of value", &model.User{ID: id}, true},
		{"wrong type", "user", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.ctxVal != nil {
				r = r.WithContext(context.WithValue(r.Context(), constant.UserContextKey, tt.ctxVal))
			}
			u, err := GetUserFromContext(r)
			if tt.wantErr {
				if !errors.Is(err, constant.ErrUnauthorized) || u != nil {
					t.Fatalf("expected ErrUnauthorized, got %v / %v", u, err)
				}
				if IsAuthenticated(r) {
					t.Fatal("IsAuthenticated must be false")
				}
				return
			}
			if err != nil || u.ID != id || !IsAuthenticated(r) {
				t.Fatalf("expected user %s, got %v / %v", id, u, err)
			}
		})
	}
}

func TestParseUUIDFromPath(t *testing.T) {
	id := uuid.New()
	r := withURLParam(httptest.NewRequest(http.MethodGet, "/", nil), "walletId", id.String())
	got, err := ParseUUIDFromPath(r, "walletId")
	if err != nil || got != id {
		t.Fatalf("got %v / %v, want %s", got, err, id)
	}

	r = withURLParam(httptest.NewRequest(http.MethodGet, "/", nil), "walletId", "not-a-uuid")
	if got, err := ParseUUIDFromPath(r, "walletId"); !errors.Is(err, constant.ErrInvalidInput) || got != uuid.Nil {
		t.Fatalf("expected ErrInvalidInput, got %v / %v", got, err)
	}
	if _, err := ParseUUIDFromPath(httptest.NewRequest(http.MethodGet, "/", nil), "walletId"); !errors.Is(err, constant.ErrInvalidInput) {
		t.Fatalf("missing param: expected ErrInvalidInput, got %v", err)
	}
}

func TestRequireAuthentication(t *testing.T) {
	called := false
	next := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}
	h := RequireAuthentication(next)

	rr := httptest.NewRecorder()
	h(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusUnauthorized || called {
		t.Fatalf("anonymous: expected 401 without calling next, got %d (called=%v)", rr.Code, called)
	}
	body := decodeJSON(t, rr)
	if body["message"] != "Unauthorized" || body["success"] != false {
		t.Fatalf("unexpected envelope: %v", body)
	}

	rr = httptest.NewRecorder()
	h(rr, withUser(httptest.NewRequest(http.MethodGet, "/", nil), uuid.New()))
	if rr.Code != http.StatusNoContent || !called {
		t.Fatalf("authenticated: expected next to run, got %d (called=%v)", rr.Code, called)
	}
}

func TestSetCommonHeadersAndDecodeJSONBody(t *testing.T) {
	rr := httptest.NewRecorder()
	SetCommonHeaders(rr)
	for k, v := range map[string]string{
		"Content-Type":           "application/json",
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
	} {
		if got := rr.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}

	var dst struct {
		A int `json:"a"`
	}
	if err := DecodeJSONBody(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":4}`)), &dst); err != nil || dst.A != 4 {
		t.Fatalf("DecodeJSONBody = %v / %+v", err, dst)
	}
	if err := DecodeJSONBody(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":"x"}`)), &dst); err == nil {
		t.Fatal("expected a type error")
	}
}
