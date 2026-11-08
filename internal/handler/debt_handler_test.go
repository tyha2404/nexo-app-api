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

// debtTestReq builds a request with an optional JSON body, an optional
// authenticated user (nil = anonymous) and an optional chi {id} URL param.
// Shared by the debt, credit-card statement, budget, target and preset tests.
func debtTestReq(method, target, body string, user *uuid.UUID, id string) *http.Request {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	if id != "" {
		r = withURLParam(r, "id", id)
	}
	if user != nil {
		r = withUser(r, *user)
	}
	return r
}

// debtAssertError checks the standard error envelope. 5xx responses must not
// leak the underlying error text.
func debtAssertError(t *testing.T, rr *httptest.ResponseRecorder, wantStatus int, wantMsg string) {
	t.Helper()
	require.Equal(t, wantStatus, rr.Code, "body: %s", rr.Body.String())
	body := decodeJSON(t, rr)
	assert.Equal(t, false, body["success"])
	assert.Equal(t, float64(wantStatus), body["status"])
	if wantMsg != "" {
		assert.Equal(t, wantMsg, body["message"])
	}
	if wantStatus >= http.StatusInternalServerError {
		_, leaked := body["error"]
		assert.False(t, leaked, "5xx must not expose error detail: %v", body["error"])
	}
}

// debtAssertSuccess checks a {status, success:true, data} envelope and returns it.
func debtAssertSuccess(t *testing.T, rr *httptest.ResponseRecorder, wantStatus int) map[string]interface{} {
	t.Helper()
	require.Equal(t, wantStatus, rr.Code, "body: %s", rr.Body.String())
	body := decodeJSON(t, rr)
	assert.Equal(t, true, body["success"])
	assert.Equal(t, float64(wantStatus), body["status"])
	return body
}

type stubDebtService struct {
	service.DebtService
	calls     int
	gotUser   uuid.UUID
	gotDebtID uuid.UUID
	gotType   model.DebtType
	gotStatus model.DebtStatus
	gotCreate dto.CreateDebtRequest
	gotRepay  dto.AddRepaymentRequest
	err       error
}

func (s *stubDebtService) CreateDebt(ctx context.Context, userID uuid.UUID, req dto.CreateDebtRequest) (*dto.DebtResponse, error) {
	s.calls++
	s.gotUser, s.gotCreate = userID, req
	if s.err != nil {
		return nil, s.err
	}
	return &dto.DebtResponse{ID: uuid.New(), UserID: userID, Type: req.Type, Title: req.Title, TotalAmount: req.TotalAmount, Remaining: req.TotalAmount}, nil
}

func (s *stubDebtService) GetDebts(ctx context.Context, userID uuid.UUID, debtType model.DebtType, status model.DebtStatus) ([]dto.DebtResponse, error) {
	s.calls++
	s.gotUser, s.gotType, s.gotStatus = userID, debtType, status
	if s.err != nil {
		return nil, s.err
	}
	return []dto.DebtResponse{{ID: uuid.New(), UserID: userID, Title: "A"}, {ID: uuid.New(), UserID: userID, Title: "B"}}, nil
}

func (s *stubDebtService) GetDebtSummary(ctx context.Context, userID uuid.UUID) (*dto.DebtSummaryResponse, error) {
	s.calls++
	s.gotUser = userID
	if s.err != nil {
		return nil, s.err
	}
	return &dto.DebtSummaryResponse{TotalPayable: 300, TotalReceivable: 100, OverdueCount: 1, PendingCount: 2}, nil
}

func (s *stubDebtService) AddRepayment(ctx context.Context, userID uuid.UUID, debtID uuid.UUID, req dto.AddRepaymentRequest) (*dto.DebtResponse, error) {
	s.calls++
	s.gotUser, s.gotDebtID, s.gotRepay = userID, debtID, req
	if s.err != nil {
		return nil, s.err
	}
	return &dto.DebtResponse{ID: debtID, UserID: userID, PaidAmount: req.Amount}, nil
}

func (s *stubDebtService) DeleteDebt(ctx context.Context, userID uuid.UUID, debtID uuid.UUID) error {
	s.calls++
	s.gotUser, s.gotDebtID = userID, debtID
	return s.err
}

func TestDebtHandler_CreateDebt(t *testing.T) {
	userID := uuid.New()
	valid := `{"type":"PAYABLE","title":"Vay mua xe","totalAmount":1500000,"notes":"n"}`
	tests := []struct {
		name       string
		anonymous  bool
		body       string
		svcErr     error
		wantStatus int
		wantMsg    string
		wantCalled bool
	}{
		{"success", false, valid, nil, http.StatusCreated, "Debt created successfully", true},
		{"unauthenticated", true, valid, nil, http.StatusUnauthorized, "Unauthorized", false},
		{"malformed json", false, `{"type":`, nil, http.StatusBadRequest, "Invalid request payload", false},
		{"invalid type", false, `{"type":"LOAN","title":"x","totalAmount":10}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"missing title", false, `{"type":"RECEIVABLE","totalAmount":10}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"non-positive amount", false, `{"type":"PAYABLE","title":"x","totalAmount":0}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"wallet not found", false, valid, fmt.Errorf("wallet: %w", gorm.ErrRecordNotFound), http.StatusNotFound, "Wallet not found", true},
		{"wallet not owned", false, valid, constant.ErrNotFound, http.StatusNotFound, "Wallet not found", true},
		{"internal error", false, valid, errors.New("pq: boom"), http.StatusInternalServerError, "Failed to create debt", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubDebtService{err: tt.svcErr}
			h := NewDebtHandler(svc, zap.NewNop())
			var user *uuid.UUID
			if !tt.anonymous {
				user = &userID
			}
			rr := httptest.NewRecorder()
			h.CreateDebt(rr, debtTestReq(http.MethodPost, "/debts", tt.body, user, ""))

			assert.Equal(t, tt.wantCalled, svc.calls == 1, "service call count %d", svc.calls)
			if tt.wantStatus != http.StatusCreated {
				debtAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			body := debtAssertSuccess(t, rr, http.StatusCreated)
			assert.Equal(t, tt.wantMsg, body["message"])
			assert.Equal(t, userID, svc.gotUser, "debt must be created for the caller")
			assert.Equal(t, "Vay mua xe", svc.gotCreate.Title)
			assert.Equal(t, model.DebtTypePayable, svc.gotCreate.Type)
			data := body["data"].(map[string]interface{})
			assert.Equal(t, "Vay mua xe", data["title"])
			assert.Equal(t, userID.String(), data["userId"])
		})
	}
}

func TestDebtHandler_GetDebts(t *testing.T) {
	userID := uuid.New()

	t.Run("passes filters and user", func(t *testing.T) {
		svc := &stubDebtService{}
		h := NewDebtHandler(svc, zap.NewNop())
		rr := httptest.NewRecorder()
		h.GetDebts(rr, debtTestReq(http.MethodGet, "/debts?type=RECEIVABLE&status=OVERDUE", "", &userID, ""))

		body := debtAssertSuccess(t, rr, http.StatusOK)
		assert.Equal(t, userID, svc.gotUser)
		assert.Equal(t, model.DebtType("RECEIVABLE"), svc.gotType)
		assert.Equal(t, model.DebtStatus("OVERDUE"), svc.gotStatus)
		assert.Len(t, body["data"], 2)
	})

	t.Run("no filters", func(t *testing.T) {
		svc := &stubDebtService{}
		h := NewDebtHandler(svc, zap.NewNop())
		rr := httptest.NewRecorder()
		h.GetDebts(rr, debtTestReq(http.MethodGet, "/debts", "", &userID, ""))
		debtAssertSuccess(t, rr, http.StatusOK)
		assert.Empty(t, svc.gotType)
		assert.Empty(t, svc.gotStatus)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		svc := &stubDebtService{}
		rr := httptest.NewRecorder()
		NewDebtHandler(svc, zap.NewNop()).GetDebts(rr, debtTestReq(http.MethodGet, "/debts", "", nil, ""))
		debtAssertError(t, rr, http.StatusUnauthorized, "Unauthorized")
		assert.Zero(t, svc.calls)
	})

	t.Run("service error", func(t *testing.T) {
		svc := &stubDebtService{err: errors.New("db down")}
		rr := httptest.NewRecorder()
		NewDebtHandler(svc, zap.NewNop()).GetDebts(rr, debtTestReq(http.MethodGet, "/debts", "", &userID, ""))
		debtAssertError(t, rr, http.StatusInternalServerError, "Failed to get debts")
	})
}

func TestDebtHandler_GetSummary(t *testing.T) {
	userID := uuid.New()

	svc := &stubDebtService{}
	rr := httptest.NewRecorder()
	NewDebtHandler(svc, zap.NewNop()).GetSummary(rr, debtTestReq(http.MethodGet, "/debts/summary", "", &userID, ""))
	body := debtAssertSuccess(t, rr, http.StatusOK)
	assert.Equal(t, userID, svc.gotUser)
	data := body["data"].(map[string]interface{})
	assert.Equal(t, float64(300), data["totalPayable"])
	assert.Equal(t, float64(1), data["overdueCount"])

	svc = &stubDebtService{}
	rr = httptest.NewRecorder()
	NewDebtHandler(svc, zap.NewNop()).GetSummary(rr, debtTestReq(http.MethodGet, "/debts/summary", "", nil, ""))
	debtAssertError(t, rr, http.StatusUnauthorized, "Unauthorized")
	assert.Zero(t, svc.calls)

	svc = &stubDebtService{err: errors.New("db down")}
	rr = httptest.NewRecorder()
	NewDebtHandler(svc, zap.NewNop()).GetSummary(rr, debtTestReq(http.MethodGet, "/debts/summary", "", &userID, ""))
	debtAssertError(t, rr, http.StatusInternalServerError, "Failed to get debt summary")
}

func TestDebtHandler_AddRepayment(t *testing.T) {
	userID := uuid.New()
	debtID := uuid.New()
	valid := `{"amount":250000,"notes":"tra lan 1"}`
	tests := []struct {
		name       string
		anonymous  bool
		id         string
		body       string
		svcErr     error
		wantStatus int
		wantMsg    string
		wantCalled bool
	}{
		{"success", false, debtID.String(), valid, nil, http.StatusOK, "Repayment recorded successfully", true},
		{"unauthenticated", true, debtID.String(), valid, nil, http.StatusUnauthorized, "Unauthorized", false},
		{"invalid id", false, "abc", valid, nil, http.StatusBadRequest, "Invalid debt ID", false},
		{"malformed json", false, debtID.String(), `not json`, nil, http.StatusBadRequest, "Invalid request payload", false},
		{"zero amount", false, debtID.String(), `{"amount":0}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"negative amount", false, debtID.String(), `{"amount":-5}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"exceeds outstanding", false, debtID.String(), valid, service.ErrInvalidRepayment, http.StatusBadRequest, "Invalid repayment", true},
		{"debt completed", false, debtID.String(), valid, service.ErrDebtAlreadyCompleted, http.StatusBadRequest, "Invalid repayment", true},
		{"debt not found", false, debtID.String(), valid, service.ErrDebtNotFound, http.StatusNotFound, "Debt not found", true},
		{"wallet not found", false, debtID.String(), valid, fmt.Errorf("wallet %w", constant.ErrNotFound), http.StatusNotFound, "Wallet not found", true},
		{"internal error", false, debtID.String(), valid, errors.New("tx aborted"), http.StatusInternalServerError, "Failed to add repayment", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubDebtService{err: tt.svcErr}
			var user *uuid.UUID
			if !tt.anonymous {
				user = &userID
			}
			rr := httptest.NewRecorder()
			NewDebtHandler(svc, zap.NewNop()).AddRepayment(rr, debtTestReq(http.MethodPost, "/debts/"+tt.id+"/repayments", tt.body, user, tt.id))

			assert.Equal(t, tt.wantCalled, svc.calls == 1, "service call count %d", svc.calls)
			if tt.wantStatus != http.StatusOK {
				debtAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			body := debtAssertSuccess(t, rr, http.StatusOK)
			assert.Equal(t, tt.wantMsg, body["message"])
			assert.Equal(t, userID, svc.gotUser)
			assert.Equal(t, debtID, svc.gotDebtID)
			assert.Equal(t, 250000.0, svc.gotRepay.Amount)
			assert.Equal(t, float64(250000), body["data"].(map[string]interface{})["paidAmount"])
		})
	}
}

func TestDebtHandler_DeleteDebt(t *testing.T) {
	userID := uuid.New()
	debtID := uuid.New()
	tests := []struct {
		name       string
		anonymous  bool
		id         string
		svcErr     error
		wantStatus int
		wantMsg    string
		wantCalled bool
	}{
		{"success", false, debtID.String(), nil, http.StatusOK, "Debt deleted successfully", true},
		{"unauthenticated", true, debtID.String(), nil, http.StatusUnauthorized, "Unauthorized", false},
		{"invalid id", false, "123", nil, http.StatusBadRequest, "Invalid debt ID", false},
		{"not found", false, debtID.String(), service.ErrDebtNotFound, http.StatusNotFound, "Debt not found", true},
		{"internal error", false, debtID.String(), errors.New("boom"), http.StatusInternalServerError, "Failed to delete debt", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubDebtService{err: tt.svcErr}
			var user *uuid.UUID
			if !tt.anonymous {
				user = &userID
			}
			rr := httptest.NewRecorder()
			NewDebtHandler(svc, zap.NewNop()).DeleteDebt(rr, debtTestReq(http.MethodDelete, "/debts/"+tt.id, "", user, tt.id))

			assert.Equal(t, tt.wantCalled, svc.calls == 1, "service call count %d", svc.calls)
			if tt.wantStatus != http.StatusOK {
				debtAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			body := debtAssertSuccess(t, rr, http.StatusOK)
			assert.Equal(t, tt.wantMsg, body["message"])
			assert.Equal(t, userID, svc.gotUser)
			assert.Equal(t, debtID, svc.gotDebtID)
		})
	}
}
