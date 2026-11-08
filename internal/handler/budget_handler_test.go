package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
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

type mockBudgetService struct {
	getFn  func(ctx context.Context, userID, id uuid.UUID) (*dto.BudgetResponse, error)
	listFn func(ctx context.Context, userID uuid.UUID, page, limit int) ([]dto.BudgetResponse, int64, error)
}

func (m *mockBudgetService) CreateBudget(ctx context.Context, userID uuid.UUID, req dto.CreateBudgetRequest) (*dto.BudgetResponse, error) {
	return nil, nil
}

func (m *mockBudgetService) GetBudget(ctx context.Context, userID, id uuid.UUID) (*dto.BudgetResponse, error) {
	return m.getFn(ctx, userID, id)
}

func (m *mockBudgetService) ListBudgets(ctx context.Context, userID uuid.UUID, page, limit int) ([]dto.BudgetResponse, int64, error) {
	return m.listFn(ctx, userID, page, limit)
}

func (m *mockBudgetService) UpdateBudget(ctx context.Context, userID, id uuid.UUID, req dto.UpdateBudgetRequest) (*dto.BudgetResponse, error) {
	return nil, nil
}

func (m *mockBudgetService) DeleteBudget(ctx context.Context, userID, id uuid.UUID) error {
	return nil
}

// newAuthedRequest builds a request carrying an authenticated user and an optional chi {id} param.
func newAuthedRequest(method, target, id string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	ctx := context.WithValue(req.Context(), constant.UserContextKey, model.User{ID: uuid.New()})
	if id != "" {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", id)
		ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	}
	return req.WithContext(ctx)
}

func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v (%q)", err, rr.Body.String())
	}
	return body
}

func TestBudgetHandler_Get_StatusMapping(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"wrapped ErrNotFound", uuid.NewString(), fmt.Errorf("budget %w", constant.ErrNotFound), http.StatusNotFound, "Budget not found"},
		{"gorm record not found", uuid.NewString(), gorm.ErrRecordNotFound, http.StatusNotFound, "Budget not found"},
		{"unexpected error", uuid.NewString(), errors.New("pq: connection refused"), http.StatusInternalServerError, "Failed to get budget"},
		{"invalid uuid", "not-a-uuid", nil, http.StatusBadRequest, "Invalid ID format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockBudgetService{
				getFn: func(ctx context.Context, userID, id uuid.UUID) (*dto.BudgetResponse, error) {
					return nil, tt.svcErr
				},
			}
			h := NewBudgetHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()

			h.Get(rr, newAuthedRequest(http.MethodGet, "/budgets/"+tt.id, tt.id))

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rr.Code, tt.wantStatus, rr.Body.String())
			}
			body := decodeJSON(t, rr)
			if body["success"] != false || body["status"] != float64(tt.wantStatus) {
				t.Fatalf("unexpected envelope: %v", body)
			}
			if body["message"] != tt.wantMsg {
				t.Fatalf("message = %v, want %q", body["message"], tt.wantMsg)
			}
			if tt.wantStatus >= 500 {
				if _, ok := body["error"]; ok {
					t.Fatalf("5xx must not expose error detail, got %v", body["error"])
				}
			}
		})
	}
}

func TestBudgetHandler_Get_MissingUserReturns401(t *testing.T) {
	h := NewBudgetHandler(&mockBudgetService{}, zap.NewNop())
	id := uuid.NewString()
	req := httptest.NewRequest(http.MethodGet, "/budgets/"+id, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	decodeJSON(t, rr)
}

func TestBudgetHandler_List_UsesParsePagination(t *testing.T) {
	var gotPage, gotLimit int
	svc := &mockBudgetService{
		listFn: func(ctx context.Context, userID uuid.UUID, page, limit int) ([]dto.BudgetResponse, int64, error) {
			gotPage, gotLimit = page, limit
			return []dto.BudgetResponse{}, 0, nil
		},
	}
	h := NewBudgetHandler(svc, zap.NewNop())
	rr := httptest.NewRecorder()

	h.List(rr, newAuthedRequest(http.MethodGet, "/budgets?page=0&limit=5000", ""))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if gotPage != 1 || gotLimit != MaxPageLimit {
		t.Fatalf("page/limit = %d/%d, want 1/%d", gotPage, gotLimit, MaxPageLimit)
	}
}

type stubBudgetService2 struct {
	service.BudgetService
	calls     int
	gotUser   uuid.UUID
	gotID     uuid.UUID
	gotPage   int
	gotLimit  int
	gotCreate dto.CreateBudgetRequest
	gotUpdate dto.UpdateBudgetRequest
	err       error
}

func (s *stubBudgetService2) CreateBudget(ctx context.Context, userID uuid.UUID, req dto.CreateBudgetRequest) (*dto.BudgetResponse, error) {
	s.calls++
	s.gotUser, s.gotCreate = userID, req
	if s.err != nil {
		return nil, s.err
	}
	return &dto.BudgetResponse{ID: uuid.New(), UserID: userID, CategoryID: req.CategoryID, Amount: req.Amount, PeriodType: req.PeriodType}, nil
}

func (s *stubBudgetService2) ListBudgets(ctx context.Context, userID uuid.UUID, page, limit int) ([]dto.BudgetResponse, int64, error) {
	s.calls++
	s.gotUser, s.gotPage, s.gotLimit = userID, page, limit
	if s.err != nil {
		return nil, 0, s.err
	}
	return []dto.BudgetResponse{{ID: uuid.New(), UserID: userID}, {ID: uuid.New(), UserID: userID}}, 42, nil
}

func (s *stubBudgetService2) GetBudget(ctx context.Context, userID, id uuid.UUID) (*dto.BudgetResponse, error) {
	s.calls++
	s.gotUser, s.gotID = userID, id
	if s.err != nil {
		return nil, s.err
	}
	return &dto.BudgetResponse{ID: id, UserID: userID, Amount: 10}, nil
}

func (s *stubBudgetService2) UpdateBudget(ctx context.Context, userID, id uuid.UUID, req dto.UpdateBudgetRequest) (*dto.BudgetResponse, error) {
	s.calls++
	s.gotUser, s.gotID, s.gotUpdate = userID, id, req
	if s.err != nil {
		return nil, s.err
	}
	return &dto.BudgetResponse{ID: id, UserID: userID, Amount: *req.Amount}, nil
}

func (s *stubBudgetService2) DeleteBudget(ctx context.Context, userID, id uuid.UUID) error {
	s.calls++
	s.gotUser, s.gotID = userID, id
	return s.err
}

func TestBudgetHandler_Create(t *testing.T) {
	userID := uuid.New()
	catID := uuid.New()
	valid := `{"categoryId":"` + catID.String() + `","amount":3000000,"periodType":"monthly","periodStart":"2026-11-01T00:00:00Z"}`
	tests := []struct {
		name       string
		anonymous  bool
		body       string
		svcErr     error
		wantStatus int
		wantMsg    string
		wantCalled bool
	}{
		{"success", false, valid, nil, http.StatusCreated, "", true},
		{"unauthenticated", true, valid, nil, http.StatusUnauthorized, "Unauthorized", false},
		{"malformed json", false, `{"amount":`, nil, http.StatusBadRequest, "Invalid request payload", false},
		{"bad period type", false, `{"categoryId":"` + catID.String() + `","amount":1,"periodType":"weekly","periodStart":"2026-11-01T00:00:00Z"}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"zero amount", false, `{"categoryId":"` + catID.String() + `","amount":0,"periodType":"monthly","periodStart":"2026-11-01T00:00:00Z"}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"missing category", false, `{"amount":1,"periodType":"monthly","periodStart":"2026-11-01T00:00:00Z"}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"category not found", false, valid, constant.ErrNotFound, http.StatusNotFound, "Category not found", true},
		{"duplicate", false, valid, fmt.Errorf("budget: %w", constant.ErrAlreadyExists), http.StatusConflict, "Ngân sách cho danh mục và kỳ này đã tồn tại", true},
		{"internal error", false, valid, errors.New("boom"), http.StatusInternalServerError, "Failed to create budget", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubBudgetService2{err: tt.svcErr}
			var user *uuid.UUID
			if !tt.anonymous {
				user = &userID
			}
			rr := httptest.NewRecorder()
			NewBudgetHandler(svc, zap.NewNop()).Create(rr, debtTestReq(http.MethodPost, "/budgets", tt.body, user, ""))

			assert.Equal(t, tt.wantCalled, svc.calls == 1, "service call count %d", svc.calls)
			if tt.wantStatus != http.StatusCreated {
				debtAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			body := debtAssertSuccess(t, rr, http.StatusCreated)
			assert.Equal(t, userID, svc.gotUser)
			assert.Equal(t, catID, svc.gotCreate.CategoryID)
			assert.Equal(t, "monthly", svc.gotCreate.PeriodType)
			data := body["data"].(map[string]interface{})
			assert.Equal(t, float64(3000000), data["amount"])
			assert.Equal(t, userID.String(), data["userId"])
		})
	}
}

func TestBudgetHandler_Get_Success(t *testing.T) {
	userID := uuid.New()
	id := uuid.New()
	svc := &stubBudgetService2{}
	rr := httptest.NewRecorder()
	NewBudgetHandler(svc, zap.NewNop()).Get(rr, debtTestReq(http.MethodGet, "/budgets/"+id.String(), "", &userID, id.String()))

	body := debtAssertSuccess(t, rr, http.StatusOK)
	assert.Equal(t, userID, svc.gotUser)
	assert.Equal(t, id, svc.gotID)
	assert.Equal(t, id.String(), body["data"].(map[string]interface{})["id"])
}

func TestBudgetHandler_List_Envelope(t *testing.T) {
	userID := uuid.New()

	svc := &stubBudgetService2{}
	rr := httptest.NewRecorder()
	NewBudgetHandler(svc, zap.NewNop()).List(rr, debtTestReq(http.MethodGet, "/budgets?page=3&limit=20", "", &userID, ""))
	body := debtAssertSuccess(t, rr, http.StatusOK)
	assert.Equal(t, userID, svc.gotUser)
	assert.Equal(t, 3, svc.gotPage)
	assert.Equal(t, 20, svc.gotLimit)
	assert.Len(t, body["items"], 2)
	assert.Equal(t, float64(42), body["total"])
	assert.Equal(t, float64(3), body["page"])
	assert.Equal(t, float64(20), body["limit"])

	svc = &stubBudgetService2{}
	rr = httptest.NewRecorder()
	NewBudgetHandler(svc, zap.NewNop()).List(rr, debtTestReq(http.MethodGet, "/budgets", "", nil, ""))
	debtAssertError(t, rr, http.StatusUnauthorized, "Unauthorized")
	assert.Zero(t, svc.calls)

	svc = &stubBudgetService2{err: errors.New("db down")}
	rr = httptest.NewRecorder()
	NewBudgetHandler(svc, zap.NewNop()).List(rr, debtTestReq(http.MethodGet, "/budgets", "", &userID, ""))
	debtAssertError(t, rr, http.StatusInternalServerError, "Failed to list budgets")
}

func TestBudgetHandler_Update(t *testing.T) {
	userID := uuid.New()
	id := uuid.New()
	valid := `{"amount":500000,"periodType":"yearly"}`
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
		{"success", false, id.String(), valid, nil, http.StatusOK, "", true},
		{"unauthenticated", true, id.String(), valid, nil, http.StatusUnauthorized, "Unauthorized", false},
		{"invalid id", false, "nope", valid, nil, http.StatusBadRequest, "Invalid ID format", false},
		{"malformed json", false, id.String(), `}`, nil, http.StatusBadRequest, "Invalid request payload", false},
		{"negative amount", false, id.String(), `{"amount":-1}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"bad period type", false, id.String(), `{"periodType":"daily"}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"not found", false, id.String(), valid, gorm.ErrRecordNotFound, http.StatusNotFound, "Budget not found", true},
		{"duplicate", false, id.String(), valid, constant.ErrAlreadyExists, http.StatusConflict, "Ngân sách cho danh mục và kỳ này đã tồn tại", true},
		{"internal error", false, id.String(), valid, errors.New("boom"), http.StatusInternalServerError, "Failed to update budget", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubBudgetService2{err: tt.svcErr}
			var user *uuid.UUID
			if !tt.anonymous {
				user = &userID
			}
			rr := httptest.NewRecorder()
			NewBudgetHandler(svc, zap.NewNop()).Update(rr, debtTestReq(http.MethodPut, "/budgets/"+tt.id, tt.body, user, tt.id))

			assert.Equal(t, tt.wantCalled, svc.calls == 1, "service call count %d", svc.calls)
			if tt.wantStatus != http.StatusOK {
				debtAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			body := debtAssertSuccess(t, rr, http.StatusOK)
			assert.Equal(t, userID, svc.gotUser)
			assert.Equal(t, id, svc.gotID)
			assert.Equal(t, float64(500000), body["data"].(map[string]interface{})["amount"])
		})
	}
}

func TestBudgetHandler_Delete(t *testing.T) {
	userID := uuid.New()
	id := uuid.New()
	tests := []struct {
		name       string
		anonymous  bool
		id         string
		svcErr     error
		wantStatus int
		wantMsg    string
		wantCalled bool
	}{
		{"unauthenticated", true, id.String(), nil, http.StatusUnauthorized, "Unauthorized", false},
		{"invalid id", false, "x", nil, http.StatusBadRequest, "Invalid ID format", false},
		{"not found", false, id.String(), fmt.Errorf("budget %w", constant.ErrNotFound), http.StatusNotFound, "Budget not found", true},
		{"internal error", false, id.String(), errors.New("boom"), http.StatusInternalServerError, "Failed to delete budget", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubBudgetService2{err: tt.svcErr}
			var user *uuid.UUID
			if !tt.anonymous {
				user = &userID
			}
			rr := httptest.NewRecorder()
			NewBudgetHandler(svc, zap.NewNop()).Delete(rr, debtTestReq(http.MethodDelete, "/budgets/"+tt.id, "", user, tt.id))
			assert.Equal(t, tt.wantCalled, svc.calls == 1, "service call count %d", svc.calls)
			debtAssertError(t, rr, tt.wantStatus, tt.wantMsg)
		})
	}

	t.Run("success", func(t *testing.T) {
		svc := &stubBudgetService2{}
		rr := httptest.NewRecorder()
		NewBudgetHandler(svc, zap.NewNop()).Delete(rr, debtTestReq(http.MethodDelete, "/budgets/"+id.String(), "", &userID, id.String()))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		body := decodeJSON(t, rr)
		assert.Equal(t, "Budget deleted successfully", body["message"])
		assert.Equal(t, userID, svc.gotUser)
		assert.Equal(t, id, svc.gotID)
	})
}
