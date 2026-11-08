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
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type stubCategoryService struct {
	service.CategoryService
	createFn        func(ctx context.Context, c *model.Category) (*model.Category, error)
	getFn           func(ctx context.Context, userID, id uuid.UUID) (*model.Category, error)
	listFn          func(ctx context.Context, userID uuid.UUID, categoryType string, limit, offset int) ([]model.Category, error)
	listWithTotalFn func(ctx context.Context, userID uuid.UUID, categoryType string, page, limit int) ([]model.Category, int64, error)
	updateFieldsFn  func(ctx context.Context, userID, id uuid.UUID, updates map[string]interface{}) error
	deleteFn        func(ctx context.Context, userID, id uuid.UUID) error
}

func (s *stubCategoryService) Create(ctx context.Context, c *model.Category) (*model.Category, error) {
	return s.createFn(ctx, c)
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
func (s *stubCategoryService) UpdateFields(ctx context.Context, userID, id uuid.UUID, updates map[string]interface{}) error {
	return s.updateFieldsFn(ctx, userID, id, updates)
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *model.Category
			svc := &stubCategoryService{createFn: func(ctx context.Context, c *model.Category) (*model.Category, error) {
				got = c
				if tt.svcErr != nil {
					return nil, tt.svcErr
				}
				c.ID = uuid.New()
				return c, nil
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
			assert.Equal(t, userID, got.UserID, "category must be owned by the caller")
			assert.Equal(t, "Food", got.Name)
			assert.Equal(t, model.CategoryType("EXPENSE"), got.Type)
			assert.True(t, got.ExcludeFromAverageDaily)
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
		name        string
		user        uuid.UUID
		id          string
		body        string
		updateErr   error
		getErr      error
		wantStatus  int
		wantMsg     string
		wantUpdates map[string]interface{}
	}{
		{"success all fields", userID, catID.String(), `{"name":"Dining","type":"EXPENSE","description":"d","excludeFromAverageDaily":false}`, nil, nil, http.StatusOK, "",
			map[string]interface{}{"name": "Dining", "type": "EXPENSE", "description": "d", "exclude_from_average_daily": false}},
		{"unauthenticated", uuid.Nil, catID.String(), `{"name":"x"}`, nil, nil, http.StatusUnauthorized, "Unauthorized", nil},
		{"invalid uuid", userID, "bad", `{"name":"x"}`, nil, nil, http.StatusBadRequest, "Invalid category ID", nil},
		{"bad json", userID, catID.String(), `{`, nil, nil, http.StatusBadRequest, "Invalid request payload", nil},
		{"invalid type", userID, catID.String(), `{"type":"GIFT"}`, nil, nil, http.StatusBadRequest, "Validation failed", nil},
		{"no fields", userID, catID.String(), `{}`, nil, nil, http.StatusBadRequest, "No fields to update", nil},
		{"update not found", userID, catID.String(), `{"name":"x"}`, constant.ErrNotFound, nil, http.StatusNotFound, "Category not found", nil},
		{"update error", userID, catID.String(), `{"name":"x"}`, errors.New("db"), nil, http.StatusInternalServerError, "Failed to update category", nil},
		{"reload not found", userID, catID.String(), `{"name":"x"}`, nil, constant.ErrNotFound, http.StatusNotFound, "Category not found", nil},
		{"reload error", userID, catID.String(), `{"name":"x"}`, nil, errors.New("db"), http.StatusInternalServerError, "Failed to get updated category", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser uuid.UUID
			var gotUpdates map[string]interface{}
			updateCalled := false
			svc := &stubCategoryService{
				updateFieldsFn: func(ctx context.Context, uid, id uuid.UUID, updates map[string]interface{}) error {
					updateCalled = true
					gotUser, gotUpdates = uid, updates
					return tt.updateErr
				},
				getFn: func(ctx context.Context, uid, id uuid.UUID) (*model.Category, error) {
					if tt.getErr != nil {
						return nil, tt.getErr
					}
					return &model.Category{ID: id, UserID: uid, Name: "Dining"}, nil
				},
			}
			h := NewCategoryHandler(svc, zap.NewNop())
			rr := httptest.NewRecorder()
			h.Update(rr, categoryTestReq(http.MethodPut, "/categories/x", tt.body, tt.user, tt.id))

			if tt.wantStatus != http.StatusOK {
				categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				if tt.wantStatus == http.StatusBadRequest || tt.wantStatus == http.StatusUnauthorized {
					assert.False(t, updateCalled, "service must not be called")
				}
				return
			}
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, tt.wantUpdates, gotUpdates)
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
		{"success", userID, catID.String(), nil, http.StatusNoContent, ""},
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

			if tt.wantStatus != http.StatusNoContent {
				categoryAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusNoContent, rr.Code)
			assert.Empty(t, rr.Body.String())
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, catID, gotID)
		})
	}
}
