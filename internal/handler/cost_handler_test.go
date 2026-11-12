package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

type stubCostService struct {
	service.CostService
	createFn func(ctx context.Context, userID uuid.UUID, req dto.CreateCostRequest) (*model.Cost, error)
	getFn    func(ctx context.Context, userID, id uuid.UUID) (*model.Cost, error)
	updateFn func(ctx context.Context, userID, id uuid.UUID, req dto.UpdateCostRequest) (*model.Cost, error)
	deleteFn func(ctx context.Context, userID, id uuid.UUID) error
	listFn   func(ctx context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Cost, error)
}

func (s *stubCostService) Create(ctx context.Context, userID uuid.UUID, req dto.CreateCostRequest) (*model.Cost, error) {
	return s.createFn(ctx, userID, req)
}
func (s *stubCostService) Get(ctx context.Context, userID, id uuid.UUID) (*model.Cost, error) {
	return s.getFn(ctx, userID, id)
}
func (s *stubCostService) Update(ctx context.Context, userID, id uuid.UUID, req dto.UpdateCostRequest) (*model.Cost, error) {
	return s.updateFn(ctx, userID, id, req)
}
func (s *stubCostService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.deleteFn(ctx, userID, id)
}
func (s *stubCostService) ListWithCategory(ctx context.Context, userID uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Cost, error) {
	return s.listFn(ctx, userID, limit, offset, filters)
}

func TestCostHandler_Create(t *testing.T) {
	userID := uuid.New()
	catID := uuid.New()
	valid := `{"title":"Cafe","amount":25000,"currency":"VND","incurredAt":"2026-11-01","categoryId":"` + catID.String() + `"}`
	tests := []struct {
		name       string
		user       uuid.UUID
		body       string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", userID, valid, nil, http.StatusCreated, ""},
		{"unauthenticated", uuid.Nil, valid, nil, http.StatusUnauthorized, "Unauthorized access"},
		{"bad json", userID, `{"title":`, nil, http.StatusBadRequest, "Validation failed"},
		{"bad date", userID, `{"title":"Cafe","amount":1,"currency":"VND","incurredAt":"01/11/2026","categoryId":"` + catID.String() + `"}`, nil, http.StatusBadRequest, "Validation failed"},
		{"zero amount", userID, `{"title":"Cafe","amount":0,"currency":"VND","incurredAt":"2026-11-01","categoryId":"` + catID.String() + `"}`, nil, http.StatusBadRequest, "Validation failed"},
		{"lowercase currency", userID, `{"title":"Cafe","amount":1,"currency":"vnd","incurredAt":"2026-11-01","categoryId":"` + catID.String() + `"}`, nil, http.StatusBadRequest, "Validation failed"},
		{"category not found", userID, valid, gorm.ErrRecordNotFound, http.StatusNotFound, "Category not found"},
		{"service error", userID, valid, errors.New("db"), http.StatusInternalServerError, "Failed to create cost"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *model.Cost
			svc := &stubCostService{createFn: func(ctx context.Context, uid uuid.UUID, req dto.CreateCostRequest) (*model.Cost, error) {
				got = &model.Cost{UserID: uid, CategoryID: req.CategoryID, Amount: req.Amount, Title: req.Title, IncurredAt: req.IncurredAt.Time}
				if tt.svcErr != nil {
					return nil, tt.svcErr
				}
				return got, nil
			}}
			h := NewCostHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.Create(rr, categoryTestReq(http.MethodPost, "/costs", tt.body, tt.user, ""))

			if tt.wantStatus != http.StatusCreated {
				categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
			require.NotNil(t, got)
			assert.Equal(t, userID, got.UserID)
			assert.Equal(t, catID, got.CategoryID)
			assert.Equal(t, 25000.0, got.Amount)
			assert.True(t, got.IncurredAt.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)))
			body := decodeJSON(t, rr)
			assert.Equal(t, true, body["success"])
			assert.Equal(t, float64(http.StatusCreated), body["status"])
		})
	}
}

func TestCostHandler_Get(t *testing.T) {
	userID := uuid.New()
	costID := uuid.New()
	tests := []struct {
		name       string
		user       uuid.UUID
		id         string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", userID, costID.String(), nil, http.StatusOK, ""},
		{"unauthenticated", uuid.Nil, costID.String(), nil, http.StatusUnauthorized, "Unauthorized access"},
		{"invalid uuid", userID, "x", nil, http.StatusBadRequest, "Invalid cost ID"},
		{"not found", userID, costID.String(), constant.ErrNotFound, http.StatusNotFound, "Cost not found"},
		{"service error", userID, costID.String(), errors.New("db"), http.StatusInternalServerError, "Failed to get cost"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser uuid.UUID
			svc := &stubCostService{getFn: func(ctx context.Context, uid, id uuid.UUID) (*model.Cost, error) {
				gotUser = uid
				if tt.svcErr != nil {
					return nil, tt.svcErr
				}
				return &model.Cost{ID: id, UserID: uid, Title: "Cafe"}, nil
			}}
			h := NewCostHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.Get(rr, categoryTestReq(http.MethodGet, "/costs/x", "", tt.user, tt.id))

			if tt.wantStatus != http.StatusOK {
				categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, userID, gotUser)
			body := decodeJSON(t, rr)
			assert.Equal(t, true, body["success"])
			assert.Equal(t, costID.String(), body["data"].(map[string]interface{})["id"])
		})
	}
}

func TestCostHandler_List(t *testing.T) {
	userID := uuid.New()
	tests := []struct {
		name        string
		query       string
		wantLimit   int
		wantOffset  int
		wantFilters map[string]interface{}
	}{
		{"defaults", "", 10, 0, map[string]interface{}{}},
		{"explicit", "?limit=20&offset=40&startDate=2026-01-01&endDate=2026-01-31&ignored=1", 20, 40,
			map[string]interface{}{"startDate": "2026-01-01", "endDate": "2026-01-31"}},
		{"limit capped", "?limit=1000", MaxPageLimit, 0, map[string]interface{}{}},
		{"invalid values fall back", "?limit=0&offset=-3", 10, 0, map[string]interface{}{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser uuid.UUID
			var gotLimit, gotOffset int
			var gotFilters map[string]interface{}
			svc := &stubCostService{listFn: func(ctx context.Context, uid uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Cost, error) {
				gotUser, gotLimit, gotOffset, gotFilters = uid, limit, offset, filters
				return []model.Cost{{Title: "a"}, {Title: "b"}}, nil
			}}
			h := NewCostHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.List(rr, categoryTestReq(http.MethodGet, "/costs"+tt.query, "", userID, ""))

			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, tt.wantLimit, gotLimit)
			assert.Equal(t, tt.wantOffset, gotOffset)
			assert.Equal(t, tt.wantFilters, gotFilters)
			body := decodeJSON(t, rr)
			assert.Equal(t, true, body["success"])
			assert.Equal(t, float64(2), body["total"])
			assert.Equal(t, float64(tt.wantLimit), body["limit"])
			assert.Len(t, body["items"], 2)
		})
	}

	t.Run("unauthenticated", func(t *testing.T) {
		h := NewCostHandler(&stubCostService{}, zap.NewNop())
		rr := httptest.NewRecorder()
		h.List(rr, categoryTestReq(http.MethodGet, "/costs", "", uuid.Nil, ""))
		categoryAssertError(t, rr, http.StatusUnauthorized, "Unauthorized access")
	})

	t.Run("service error", func(t *testing.T) {
		svc := &stubCostService{listFn: func(ctx context.Context, uid uuid.UUID, limit, offset int, filters map[string]interface{}) ([]model.Cost, error) {
			return nil, errors.New("db")
		}}
		h := NewCostHandler(svc, zap.NewNop())
		rr := httptest.NewRecorder()
		h.List(rr, categoryTestReq(http.MethodGet, "/costs", "", userID, ""))
		categoryAssertError(t, rr, http.StatusInternalServerError, "Failed to list costs")
	})
}

func TestCostHandler_Update(t *testing.T) {
	userID := uuid.New()
	costID := uuid.New()
	tests := []struct {
		name       string
		user       uuid.UUID
		id         string
		body       string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", userID, costID.String(), `{"title":"Tea","amount":5}`, nil, http.StatusOK, ""},
		{"unauthenticated", uuid.Nil, costID.String(), `{"title":"Tea"}`, nil, http.StatusUnauthorized, "Unauthorized access"},
		{"invalid uuid", userID, "x", `{"title":"Tea"}`, nil, http.StatusBadRequest, "Invalid cost ID"},
		{"bad json", userID, costID.String(), `{`, nil, http.StatusBadRequest, "Validation failed"},
		{"negative amount", userID, costID.String(), `{"amount":-1}`, nil, http.StatusBadRequest, "Validation failed"},
		{"no fields", userID, costID.String(), `{}`, constant.ErrInvalidInput, http.StatusBadRequest, "No fields to update"},
		{"not found", userID, costID.String(), `{"title":"Tea"}`, constant.ErrNotFound, http.StatusNotFound, "Cost or category not found"},
		{"service error", userID, costID.String(), `{"title":"Tea"}`, errors.New("db"), http.StatusInternalServerError, "Failed to update cost"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser uuid.UUID
			var gotReq dto.UpdateCostRequest
			svc := &stubCostService{updateFn: func(ctx context.Context, uid, id uuid.UUID, req dto.UpdateCostRequest) (*model.Cost, error) {
				gotUser, gotReq = uid, req
				if tt.svcErr != nil {
					return nil, tt.svcErr
				}
				return &model.Cost{ID: id, UserID: uid, Title: *req.Title}, nil
			}}
			h := NewCostHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.Update(rr, categoryTestReq(http.MethodPut, "/costs/x", tt.body, tt.user, tt.id))

			if tt.wantStatus != http.StatusOK {
				categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			assert.Equal(t, userID, gotUser)
			require.NotNil(t, gotReq.Title)
			assert.Equal(t, "Tea", *gotReq.Title)
			body := decodeJSON(t, rr)
			assert.Equal(t, "Tea", body["data"].(map[string]interface{})["title"])
		})
	}
}

func TestCostHandler_Delete(t *testing.T) {
	userID := uuid.New()
	costID := uuid.New()
	tests := []struct {
		name       string
		user       uuid.UUID
		id         string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", userID, costID.String(), nil, http.StatusNoContent, ""},
		{"unauthenticated", uuid.Nil, costID.String(), nil, http.StatusUnauthorized, "Unauthorized access"},
		{"invalid uuid", userID, "x", nil, http.StatusBadRequest, "Invalid cost ID"},
		{"not found", userID, costID.String(), gorm.ErrRecordNotFound, http.StatusNotFound, "Cost not found"},
		{"service error", userID, costID.String(), errors.New("db"), http.StatusInternalServerError, "Failed to delete cost"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser, gotID uuid.UUID
			svc := &stubCostService{deleteFn: func(ctx context.Context, uid, id uuid.UUID) error {
				gotUser, gotID = uid, id
				return tt.svcErr
			}}
			h := NewCostHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.Delete(rr, categoryTestReq(http.MethodDelete, "/costs/x", "", tt.user, tt.id))

			if tt.wantStatus != http.StatusNoContent {
				categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusNoContent, rr.Code)
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, costID, gotID)
		})
	}
}
