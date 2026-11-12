package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type stubTargetService struct {
	service.TargetService
	calls     int
	gotUser   uuid.UUID
	gotReq    *dto.UpsertTargetRequest
	gotMonth  int
	gotYear   int
	upsertErr error
	sumErr    error
}

func (s *stubTargetService) UpsertTarget(ctx context.Context, userID uuid.UUID, req *dto.UpsertTargetRequest) error {
	s.calls++
	s.gotUser, s.gotReq = userID, req
	return s.upsertErr
}

func (s *stubTargetService) GetSummary(ctx context.Context, userID uuid.UUID, month, year int) (*dto.TargetSummaryResponse, error) {
	s.calls++
	s.gotUser, s.gotMonth, s.gotYear = userID, month, year
	if s.sumErr != nil {
		return nil, s.sumErr
	}
	return &dto.TargetSummaryResponse{Month: month, Year: year, Expense: dto.ExpenseSummary{TargetAmount: 1000, SpentAmount: 400, RemainingAmount: 600}}, nil
}

func TestTargetHandler_UpsertTarget(t *testing.T) {
	userID := uuid.New()
	valid := `{"targetType":"EXPENSE","targetAmount":8000000,"month":11,"year":2026}`
	tests := []struct {
		name       string
		anonymous  bool
		body       string
		svcErr     error
		wantStatus int
		wantMsg    string
		wantCalled bool
	}{
		{"success", false, valid, nil, http.StatusOK, "Target saved successfully", true},
		{"investment target", false, `{"targetType":"INVESTMENT","targetAmount":1,"month":1,"year":2000}`, nil, http.StatusOK, "Target saved successfully", true},
		{"unauthenticated", true, valid, nil, http.StatusUnauthorized, "Unauthorized", false},
		{"malformed json", false, `{"targetType"`, nil, http.StatusBadRequest, "Invalid request payload", false},
		{"bad target type", false, `{"targetType":"INCOME","targetAmount":1,"month":1,"year":2026}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"month 13", false, `{"targetType":"EXPENSE","targetAmount":1,"month":13,"year":2026}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"year too far", false, `{"targetType":"EXPENSE","targetAmount":1,"month":1,"year":2101}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"amount too large", false, `{"targetType":"EXPENSE","targetAmount":10000000000,"month":1,"year":2026}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"zero amount", false, `{"targetType":"EXPENSE","targetAmount":0,"month":1,"year":2026}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"internal error", false, valid, errors.New("boom"), http.StatusInternalServerError, "Failed to upsert target", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubTargetService{upsertErr: tt.svcErr}
			var user *uuid.UUID
			if !tt.anonymous {
				user = &userID
			}
			rr := httptest.NewRecorder()
			NewTargetHandler(svc, zap.NewNop()).UpsertTarget(rr, debtTestReq(http.MethodPost, "/targets", tt.body, user, ""))

			assert.Equal(t, tt.wantCalled, svc.calls == 1, "service call count %d", svc.calls)
			if tt.wantStatus != http.StatusOK {
				debtAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			body := debtAssertSuccess(t, rr, http.StatusOK)
			assert.Equal(t, tt.wantMsg, body["message"])
			assert.Equal(t, userID, svc.gotUser)
		})
	}
}

func TestTargetHandler_GetSummary(t *testing.T) {
	userID := uuid.New()
	tests := []struct {
		name      string
		query     string
		wantMonth int
		wantYear  int
	}{
		// The handler passes raw values (0 when missing/invalid);
		// targetService.GetSummary applies the defaults.
		{"explicit month and year", "?month=2&year=2025", 2, 2025},
		{"missing", "", 0, 0},
		{"invalid values", "?month=abc&year=-3", 0, -3},
		{"zero month", "?month=0&year=2024", 0, 2024},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubTargetService{}
			rr := httptest.NewRecorder()
			NewTargetHandler(svc, zap.NewNop()).GetSummary(rr, debtTestReq(http.MethodGet, "/targets/summary"+tt.query, "", &userID, ""))

			body := debtAssertSuccess(t, rr, http.StatusOK)
			assert.Equal(t, userID, svc.gotUser)
			assert.Equal(t, tt.wantMonth, svc.gotMonth)
			assert.Equal(t, tt.wantYear, svc.gotYear)
			data := body["data"].(map[string]interface{})
			assert.Equal(t, float64(tt.wantMonth), data["month"])
			assert.Equal(t, float64(600), data["expense"].(map[string]interface{})["remainingAmount"])
		})
	}

	t.Run("unauthenticated", func(t *testing.T) {
		svc := &stubTargetService{}
		rr := httptest.NewRecorder()
		NewTargetHandler(svc, zap.NewNop()).GetSummary(rr, debtTestReq(http.MethodGet, "/targets/summary", "", nil, ""))
		debtAssertError(t, rr, http.StatusUnauthorized, "Unauthorized")
		assert.Zero(t, svc.calls)
	})

	t.Run("service error", func(t *testing.T) {
		svc := &stubTargetService{sumErr: errors.New("db down")}
		rr := httptest.NewRecorder()
		NewTargetHandler(svc, zap.NewNop()).GetSummary(rr, debtTestReq(http.MethodGet, "/targets/summary", "", &userID, ""))
		debtAssertError(t, rr, http.StatusInternalServerError, "Failed to get target summary")
	})
}
