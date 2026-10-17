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
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
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
