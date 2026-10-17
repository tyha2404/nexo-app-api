package handler

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/tyha2404/nexo-app-api/internal/constant"
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
