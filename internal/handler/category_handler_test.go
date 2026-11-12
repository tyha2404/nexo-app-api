package handler

import (
	"context"
	"errors"
	"fmt"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type stubCategoryService struct {
	service.CategoryService
	createFn        func(ctx context.Context, userID uuid.UUID, req dto.CreateCategoryRequest) (*model.Category, error)
	getFn           func(ctx context.Context, userID, id uuid.UUID) (*model.Category, error)
	listFn          func(ctx context.Context, userID uuid.UUID, categoryType string, limit, offset int) ([]model.Category, error)
	listWithTotalFn func(ctx context.Context, userID uuid.UUID, categoryType string, page, limit int) ([]model.Category, int64, error)
	updateFn        func(ctx context.Context, userID, id uuid.UUID, req dto.UpdateCategoryRequest) (*model.Category, error)
	deleteFn        func(ctx context.Context, userID, id uuid.UUID) error
}

func (s *stubCategoryService) Create(ctx context.Context, userID uuid.UUID, req dto.CreateCategoryRequest) (*model.Category, error) {
	return s.createFn(ctx, userID, req)
}
func (s *stubCategoryService) Get(ctx context.Context, userID, id uuid.UUID) (*model.Category, error) {
	return s.getFn(ctx, userID, id)
}
func (s *stubCategoryService) List(ctx context.Context, userID uuid.UUID, categoryType string, limit, offset int) ([]model.Category, error) {
	return s.listFn(ctx, userID, categoryType, limit, offset)
}
func (s *stubCategoryService) ListWithTotal(ctx context.Context, userID uuid.UUID, categoryType string, page, limit int) ([]model.Category, int64, error) {
	return s.listWithTotalFn(ctx, userID, categoryType, page, limit)
}
func (s *stubCategoryService) Update(ctx context.Context, userID, id uuid.UUID, req dto.UpdateCategoryRequest) (*model.Category, error) {
	return s.updateFn(ctx, userID, id, req)
}
func (s *stubCategoryService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.deleteFn(ctx, userID, id)
}

// categoryTestReq builds a request with an optional user and optional {id}.
func categoryTestReq(method, target, body string, userID uuid.UUID, id string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if id != "" {
		req = withURLParam(req, "id", id)
	}
	if userID != uuid.Nil {
		req = withUser(req, userID)
	}
	return req
}

func categoryAssertError(t *testing.T, rr *httptest.ResponseRecorder, status int, msg string) {
	t.Helper()
	require.Equal(t, status, rr.Code, rr.Body.String())
	body := decodeJSON(t, rr)
	assert.Equal(t, false, body["success"])
	assert.Equal(t, float64(status), body["status"])
	if msg != "" {
		assert.Equal(t, msg, body["message"])
	}
	if status >= 500 {
		_, leaked := body["error"]
		assert.False(t, leaked, "5xx must not expose error detail")
	}
}

func TestCategoryHandler_Create(t *testing.T) {
	userID := uuid.New()
	tests := []struct {
		name       string
		user       uuid.UUID
		body       string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", userID, `{"name":"Food","type":"EXPENSE","excludeFromAverageDaily":true}`, nil, http.StatusCreated, ""},
		{"unauthenticated", uuid.Nil, `{"name":"Food","type":"EXPENSE"}`, nil, http.StatusUnauthorized, "Unauthorized"},
		{"bad json", userID, `{"name":`, nil, http.StatusBadRequest, "Invalid request payload"},
		{"missing name", userID, `{"type":"EXPENSE"}`, nil, http.StatusBadRequest, "Validation failed"},
		{"invalid type", userID, `{"name":"Food","type":"GIFT"}`, nil, http.StatusBadRequest, "Validation failed"},
		{"service error", userID, `{"name":"Food","type":"INCOME"}`, errors.New("db down"), http.StatusInternalServerError, "Failed to create category"},
		{"duplicate name", userID, `{"name":"Food","type":"EXPENSE"}`, fmt.Errorf("%w: category with this name", constant.ErrAlreadyExists), http.StatusConflict, "Danh mục với tên này đã tồn tại"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *dto.CreateCategoryRequest
			var gotUser uuid.UUID
			svc := &stubCategoryService{createFn: func(ctx context.Context, uid uuid.UUID, req dto.CreateCategoryRequest) (*model.Category, error) {
				got, gotUser = &req, uid
				if tt.svcErr != nil {
					return nil, tt.svcErr
				}
				return &model.Category{ID: uuid.New(), UserID: uid, Name: req.Name, Type: model.CategoryType(req.Type)}, nil
			}}
			h := NewCategoryHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.Create(rr, categoryTestReq(http.MethodPost, "/categories", tt.body, tt.user, ""))

			if tt.wantStatus != http.StatusCreated {
				categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
			require.NotNil(t, got)
			assert.Equal(t, userID, gotUser, "category must be owned by the caller")
			assert.Equal(t, "Food", got.Name)
			assert.Equal(t, "EXPENSE", got.Type)
			require.NotNil(t, got.ExcludeFromAverageDaily)
			assert.True(t, *got.ExcludeFromAverageDaily)
			body := decodeJSON(t, rr)
			assert.Equal(t, true, body["success"])
			assert.Equal(t, float64(http.StatusCreated), body["status"])
			data := body["data"].(map[string]interface{})
			assert.Equal(t, "Food", data["name"])
		})
	}
}

func TestCategoryHandler_Get(t *testing.T) {
	userID := uuid.New()
	catID := uuid.New()
	tests := []struct {
		name       string
		user       uuid.UUID
		id         string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", userID, catID.String(), nil, http.StatusOK, ""},
		{"unauthenticated", uuid.Nil, catID.String(), nil, http.StatusUnauthorized, "Unauthorized"},
		{"invalid uuid", userID, "nope", nil, http.StatusBadRequest, "Invalid category ID"},
		{"not found", userID, catID.String(), fmt.Errorf("category %w", constant.ErrNotFound), http.StatusNotFound, "Category not found"},
		{"service error", userID, catID.String(), errors.New("db down"), http.StatusInternalServerError, "Failed to get category"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser, gotID uuid.UUID
			svc := &stubCategoryService{getFn: func(ctx context.Context, uid, id uuid.UUID) (*model.Category, error) {
				gotUser, gotID = uid, id
				if tt.svcErr != nil {
					return nil, tt.svcErr
				}
				return &model.Category{ID: id, UserID: uid, Name: "Food"}, nil
			}}
			h := NewCategoryHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.Get(rr, categoryTestReq(http.MethodGet, "/categories/x", "", tt.user, tt.id))

			if tt.wantStatus != http.StatusOK {
				categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, catID, gotID)
			body := decodeJSON(t, rr)
			assert.Equal(t, true, body["success"])
			assert.Equal(t, catID.String(), body["data"].(map[string]interface{})["id"])
		})
	}
}

func TestCategoryHandler_List(t *testing.T) {
	userID := uuid.New()

	t.Run("unauthenticated", func(t *testing.T) {
		h := NewCategoryHandler(&stubCategoryService{}, zap.NewNop())
		rr := httptest.NewRecorder()
		h.List(rr, categoryTestReq(http.MethodGet, "/categories", "", uuid.Nil, ""))
		categoryAssertError(t, rr, http.StatusUnauthorized, "Unauthorized")
	})

	t.Run("full list without pagination params", func(t *testing.T) {
		var gotUser uuid.UUID
		var gotType string
		svc := &stubCategoryService{listFn: func(ctx context.Context, uid uuid.UUID, ct string, limit, offset int) ([]model.Category, error) {
			gotUser, gotType = uid, ct
			return []model.Category{{Name: "a"}, {Name: "b"}, {Name: "c"}}, nil
		}}
		h := NewCategoryHandler(svc, zap.NewNop())
		rr := httptest.NewRecorder()
		h.List(rr, categoryTestReq(http.MethodGet, "/categories?type=EXPENSE", "", userID, ""))

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, userID, gotUser)
		assert.Equal(t, "EXPENSE", gotType)
		body := decodeJSON(t, rr)
		assert.Equal(t, true, body["success"])
		assert.Equal(t, float64(3), body["total"])
		assert.Equal(t, float64(1), body["page"])
		assert.Equal(t, float64(3), body["limit"])
		assert.Len(t, body["items"], 3)
	})

	t.Run("paginated when page or limit given", func(t *testing.T) {
		var gotUser uuid.UUID
		var gotPage, gotLimit int
		svc := &stubCategoryService{listWithTotalFn: func(ctx context.Context, uid uuid.UUID, ct string, page, limit int) ([]model.Category, int64, error) {
			gotUser, gotPage, gotLimit = uid, page, limit
			return []model.Category{{Name: "a"}}, 42, nil
		}}
		h := NewCategoryHandler(svc, zap.NewNop())
		rr := httptest.NewRecorder()
		h.List(rr, categoryTestReq(http.MethodGet, "/categories?page=2&limit=500", "", userID, ""))

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, userID, gotUser)
		assert.Equal(t, 2, gotPage)
		assert.Equal(t, MaxPageLimit, gotLimit)
		body := decodeJSON(t, rr)
		assert.Equal(t, float64(42), body["total"])
		assert.Equal(t, float64(2), body["page"])
		assert.Equal(t, float64(MaxPageLimit), body["limit"])
	})

	t.Run("service error", func(t *testing.T) {
		svc := &stubCategoryService{listFn: func(ctx context.Context, uid uuid.UUID, ct string, limit, offset int) ([]model.Category, error) {
			return nil, errors.New("db down")
		}}
		h := NewCategoryHandler(svc, zap.NewNop())
		rr := httptest.NewRecorder()
		h.List(rr, categoryTestReq(http.MethodGet, "/categories", "", userID, ""))
		categoryAssertError(t, rr, http.StatusInternalServerError, "Failed to list categories")
	})
}

func TestCategoryHandler_Update(t *testing.T) {
	userID := uuid.New()
	catID := uuid.New()
	tests := []struct {
		name       string
		user       uuid.UUID
		id         string
		body       string
		updateErr  error
		wantStatus int
		wantMsg    string
	}{
		{"success all fields", userID, catID.String(), `{"name":"Dining","type":"EXPENSE","description":"d","excludeFromAverageDaily":false}`, nil, http.StatusOK, ""},
		{"unauthenticated", uuid.Nil, catID.String(), `{"name":"x"}`, nil, http.StatusUnauthorized, "Unauthorized"},
		{"invalid uuid", userID, "bad", `{"name":"x"}`, nil, http.StatusBadRequest, "Invalid category ID"},
		{"bad json", userID, catID.String(), `{`, nil, http.StatusBadRequest, "Invalid request payload"},
		{"invalid type", userID, catID.String(), `{"type":"GIFT"}`, nil, http.StatusBadRequest, "Validation failed"},
		{"no fields", userID, catID.String(), `{}`, constant.ErrInvalidInput, http.StatusBadRequest, "No fields to update"},
		{"update not found", userID, catID.String(), `{"name":"x"}`, constant.ErrNotFound, http.StatusNotFound, "Category not found"},
		{"update error", userID, catID.String(), `{"name":"x"}`, errors.New("db"), http.StatusInternalServerError, "Failed to update category"},
		{"rename onto an existing name", userID, catID.String(), `{"name":"Food"}`, fmt.Errorf("%w: category with this name", constant.ErrAlreadyExists), http.StatusConflict, "Danh mục với tên này đã tồn tại"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser uuid.UUID
			var gotReq dto.UpdateCategoryRequest
			updateCalled := false
			svc := &stubCategoryService{
				updateFn: func(ctx context.Context, uid, id uuid.UUID, req dto.UpdateCategoryRequest) (*model.Category, error) {
					updateCalled = true
					gotUser, gotReq = uid, req
					if tt.updateErr != nil {
						return nil, tt.updateErr
					}
					return &model.Category{ID: id, UserID: uid, Name: "Dining"}, nil
				},
			}
			h := NewCategoryHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.Update(rr, categoryTestReq(http.MethodPut, "/categories/x", tt.body, tt.user, tt.id))

			if tt.wantStatus != http.StatusOK {
				categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				if (tt.wantStatus == http.StatusBadRequest && tt.updateErr == nil) || tt.wantStatus == http.StatusUnauthorized {
					assert.False(t, updateCalled, "service must not be called")
				}
				return
			}
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			assert.Equal(t, userID, gotUser)
			require.NotNil(t, gotReq.Name)
			assert.Equal(t, "Dining", *gotReq.Name)
			require.NotNil(t, gotReq.ExcludeFromAverageDaily)
			assert.False(t, *gotReq.ExcludeFromAverageDaily)
			body := decodeJSON(t, rr)
			assert.Equal(t, "Dining", body["data"].(map[string]interface{})["name"])
		})
	}
}

func TestCategoryHandler_Delete(t *testing.T) {
	userID := uuid.New()
	catID := uuid.New()
	tests := []struct {
		name       string
		user       uuid.UUID
		id         string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", userID, catID.String(), nil, http.StatusOK, ""},
		{"unauthenticated", uuid.Nil, catID.String(), nil, http.StatusUnauthorized, "Unauthorized"},
		{"invalid uuid", userID, "bad", nil, http.StatusBadRequest, "Invalid category ID"},
		{"not found", userID, catID.String(), constant.ErrNotFound, http.StatusNotFound, "Category not found"},
		{"service error", userID, catID.String(), errors.New("db"), http.StatusInternalServerError, "Failed to delete category"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser, gotID uuid.UUID
			svc := &stubCategoryService{deleteFn: func(ctx context.Context, uid, id uuid.UUID) error {
				gotUser, gotID = uid, id
				return tt.svcErr
			}}
			h := NewCategoryHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.Delete(rr, categoryTestReq(http.MethodDelete, "/categories/x", "", tt.user, tt.id))

			if tt.wantStatus != http.StatusOK {
				categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code)
			assert.JSONEq(t, `{"status":200,"success":true,"message":"Category deleted successfully","data":null}`, rr.Body.String())
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, catID, gotID)
		})
	}
}
