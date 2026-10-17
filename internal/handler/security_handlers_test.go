package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

func withUser(r *http.Request, id uuid.UUID) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), constant.UserContextKey, model.User{ID: id, Role: "user"}))
}

func withURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// ---- users ----

type captureUserService struct {
	service.UserService
	gotID  uuid.UUID
	gotReq dto.UpdateUserRequest
	called bool
}

func (c *captureUserService) Update(ctx context.Context, id uuid.UUID, req dto.UpdateUserRequest) (*model.User, error) {
	c.called, c.gotID, c.gotReq = true, id, req
	return &model.User{ID: id, Username: *req.Username, Role: "user"}, nil
}

func TestUserHandler_UpdateIgnoresRoleAndPassword(t *testing.T) {
	svc := &captureUserService{}
	h := NewUserHandler(svc, zap.NewNop())
	id := uuid.New()

	body := `{"username":"newname","role":"admin","password":"hunter22","Password":"x"}`
	req := httptest.NewRequest(http.MethodPut, "/users/"+id.String(), strings.NewReader(body))
	req = withURLParam(req, "id", id.String())
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !svc.called || svc.gotID != id || svc.gotReq.Username == nil || *svc.gotReq.Username != "newname" {
		t.Fatalf("unexpected service call: %+v", svc)
	}
	if strings.Contains(rr.Body.String(), `"role":"admin"`) {
		t.Fatal("role must not be changeable through PUT /users/{id}")
	}
}

func TestUserHandler_UpdateRequiresAField(t *testing.T) {
	svc := &captureUserService{}
	h := NewUserHandler(svc, zap.NewNop())
	id := uuid.New()

	req := httptest.NewRequest(http.MethodPut, "/users/"+id.String(), strings.NewReader(`{"role":"admin"}`))
	req = withURLParam(req, "id", id.String())
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusBadRequest || svc.called {
		t.Fatalf("expected 400 without service call, got %d (called=%v)", rr.Code, svc.called)
	}
}

// ---- categories ----

type scopedCategoryService struct {
	service.CategoryService
	owner uuid.UUID
}

func (s *scopedCategoryService) Get(ctx context.Context, userID, id uuid.UUID) (*model.Category, error) {
	if userID != s.owner {
		return nil, constant.ErrNotFound
	}
	return &model.Category{ID: id, UserID: userID}, nil
}
func (s *scopedCategoryService) UpdateFields(ctx context.Context, userID, id uuid.UUID, updates map[string]interface{}) error {
	if userID != s.owner {
		return constant.ErrNotFound
	}
	return nil
}
func (s *scopedCategoryService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	if userID != s.owner {
		return constant.ErrNotFound
	}
	return nil
}

func TestCategoryHandler_OtherUsersCategoryIs404(t *testing.T) {
	owner, attacker := uuid.New(), uuid.New()
	h := NewCategoryHandler(&scopedCategoryService{owner: owner}, zap.NewNop())
	catID := uuid.New().String()

	cases := []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
		req  *http.Request
	}{
		{"get", h.Get, httptest.NewRequest(http.MethodGet, "/categories/"+catID, nil)},
		{"update", h.Update, httptest.NewRequest(http.MethodPut, "/categories/"+catID, strings.NewReader(`{"name":"x"}`))},
		{"delete", h.Delete, httptest.NewRequest(http.MethodDelete, "/categories/"+catID, nil)},
	}
	for _, tc := range cases {
		req := withURLParam(withUser(tc.req, attacker), "id", catID)
		req = withUser(req, attacker)
		rr := httptest.NewRecorder()
		tc.call(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("%s: expected 404, got %d", tc.name, rr.Code)
		}
	}

	// Owner succeeds.
	req := withUser(withURLParam(httptest.NewRequest(http.MethodGet, "/categories/"+catID, nil), "id", catID), owner)
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("owner get: expected 200, got %d", rr.Code)
	}

	// Missing auth context.
	rr = httptest.NewRecorder()
	h.Get(rr, withURLParam(httptest.NewRequest(http.MethodGet, "/categories/"+catID, nil), "id", catID))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("no user: expected 401, got %d", rr.Code)
	}
}

// ---- costs ----

type captureCostService struct {
	service.CostService
	owner  uuid.UUID
	gotReq dto.UpdateCostRequest
}

func (c *captureCostService) Update(ctx context.Context, userID, id uuid.UUID, req dto.UpdateCostRequest) (*model.Cost, error) {
	if userID != c.owner {
		return nil, constant.ErrNotFound
	}
	c.gotReq = req
	return &model.Cost{ID: id, UserID: userID}, nil
}

func TestCostHandler_UpdateScopedAndIgnoresUserID(t *testing.T) {
	owner, attacker := uuid.New(), uuid.New()
	svc := &captureCostService{owner: owner}
	h := NewCostHandler(svc, zap.NewNop())
	id := uuid.New().String()

	body := `{"title":"Cafe","userId":"` + attacker.String() + `"}`
	req := withUser(withURLParam(httptest.NewRequest(http.MethodPut, "/costs/"+id, bytes.NewBufferString(body)), "id", id), owner)
	rr := httptest.NewRecorder()
	h.Update(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("owner update: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), owner.String()) {
		t.Fatal("cost must stay owned by the caller")
	}

	req = withUser(withURLParam(httptest.NewRequest(http.MethodPut, "/costs/"+id, bytes.NewBufferString(`{"title":"x"}`)), "id", id), attacker)
	rr = httptest.NewRecorder()
	h.Update(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("attacker update: expected 404, got %d", rr.Code)
	}
}

// ---- chat ----

type noopChatService struct {
	service.ChatService
	streamed bool
}

func (n *noopChatService) ProcessMessageStream(ctx context.Context, userID uuid.UUID, req dto.SendMessageRequest, eventChan chan<- dto.ChatStreamEvent) error {
	n.streamed = true
	close(eventChan)
	return nil
}

func TestChatHandler_RejectsTooLongMessage(t *testing.T) {
	svc := &noopChatService{}
	h := NewChatHandler(svc, zap.NewNop())

	long := strings.Repeat("á", dto.MaxChatMessageLength+1)
	req := withUser(httptest.NewRequest(http.MethodPost, "/chat/stream", strings.NewReader(`{"message":"`+long+`"}`)), uuid.New())
	rr := httptest.NewRecorder()
	h.StreamMessage(rr, req)
	if rr.Code != http.StatusBadRequest || svc.streamed {
		t.Fatalf("expected 400 without streaming, got %d (streamed=%v)", rr.Code, svc.streamed)
	}

	ok := strings.Repeat("á", dto.MaxChatMessageLength)
	req = withUser(httptest.NewRequest(http.MethodPost, "/chat/stream", strings.NewReader(`{"message":"`+ok+`"}`)), uuid.New())
	rr = httptest.NewRecorder()
	h.StreamMessage(rr, req)
	if rr.Code != http.StatusOK || !svc.streamed {
		t.Fatalf("expected stream for max-length message, got %d", rr.Code)
	}
}
