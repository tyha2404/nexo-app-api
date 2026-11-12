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
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type stubPresetService struct {
	service.PresetService
	calls     int
	gotUser   uuid.UUID
	gotID     uuid.UUID
	gotCreate dto.CreatePresetRequest
	gotUpdate dto.UpdatePresetRequest
	err       error
}

func (s *stubPresetService) CreatePreset(ctx context.Context, userID uuid.UUID, req dto.CreatePresetRequest) (*dto.PresetResponse, error) {
	s.calls++
	s.gotUser, s.gotCreate = userID, req
	if s.err != nil {
		return nil, s.err
	}
	return &dto.PresetResponse{ID: uuid.New(), UserID: userID, CategoryID: req.CategoryID, Name: req.Name, Amount: req.Amount, Type: req.Type}, nil
}

func (s *stubPresetService) GetPreset(ctx context.Context, userID, id uuid.UUID) (*dto.PresetResponse, error) {
	s.calls++
	s.gotUser, s.gotID = userID, id
	if s.err != nil {
		return nil, s.err
	}
	return &dto.PresetResponse{ID: id, UserID: userID, Name: "Cafe"}, nil
}

func (s *stubPresetService) ListPresets(ctx context.Context, userID uuid.UUID) ([]dto.PresetResponse, error) {
	s.calls++
	s.gotUser = userID
	if s.err != nil {
		return nil, s.err
	}
	return []dto.PresetResponse{{ID: uuid.New(), UserID: userID, Name: "Cafe"}, {ID: uuid.New(), UserID: userID, Name: "Grab"}}, nil
}

func (s *stubPresetService) UpdatePreset(ctx context.Context, userID, id uuid.UUID, req dto.UpdatePresetRequest) (*dto.PresetResponse, error) {
	s.calls++
	s.gotUser, s.gotID, s.gotUpdate = userID, id, req
	if s.err != nil {
		return nil, s.err
	}
	name := "unchanged"
	if req.Name != nil {
		name = *req.Name
	}
	return &dto.PresetResponse{ID: id, UserID: userID, Name: name}, nil
}

func (s *stubPresetService) DeletePreset(ctx context.Context, userID, id uuid.UUID) error {
	s.calls++
	s.gotUser, s.gotID = userID, id
	return s.err
}

func TestPresetHandler_Create(t *testing.T) {
	userID := uuid.New()
	catID := uuid.New()
	mk := func(extra string) string {
		return `{"categoryId":"` + catID.String() + `",` + extra + `}`
	}
	valid := mk(`"name":"Cafe sang","amount":35000,"type":"EXPENSE","icon":"coffee"`)
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
		{"malformed json", false, `{"name":}`, nil, http.StatusBadRequest, "Invalid request payload", false},
		{"missing category", false, `{"name":"x","amount":1,"type":"EXPENSE"}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"bad type", false, mk(`"name":"x","amount":1,"type":"TRANSFER"`), nil, http.StatusBadRequest, "Validation failed", false},
		{"name too long", false, mk(`"name":"` + strings.Repeat("a", 101) + `","amount":1,"type":"INCOME"`), nil, http.StatusBadRequest, "Validation failed", false},
		{"amount too large", false, mk(`"name":"x","amount":10000000000,"type":"INCOME"`), nil, http.StatusBadRequest, "Validation failed", false},
		{"zero amount", false, mk(`"name":"x","amount":0,"type":"INCOME"`), nil, http.StatusBadRequest, "Validation failed", false},
		{"category not found", false, valid, gorm.ErrRecordNotFound, http.StatusNotFound, "Category not found", true},
		{"internal error", false, valid, errors.New("boom"), http.StatusInternalServerError, "Failed to create preset", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubPresetService{err: tt.svcErr}
			var user *uuid.UUID
			if !tt.anonymous {
				user = &userID
			}
			rr := httptest.NewRecorder()
			NewPresetHandler(svc, zap.NewNop()).Create(rr, debtTestReq(http.MethodPost, "/presets", tt.body, user, ""))

			assert.Equal(t, tt.wantCalled, svc.calls == 1, "service call count %d", svc.calls)
			if tt.wantStatus != http.StatusCreated {
				debtAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			body := debtAssertSuccess(t, rr, http.StatusCreated)
			assert.Equal(t, userID, svc.gotUser)
			assert.Equal(t, catID, svc.gotCreate.CategoryID)
			data := body["data"].(map[string]interface{})
			assert.Equal(t, "Cafe sang", data["name"])
			assert.Equal(t, "EXPENSE", data["type"])
		})
	}
}

func TestPresetHandler_Get(t *testing.T) {
	userID := uuid.New()
	id := uuid.New()
	tests := []struct {
		name       string
		anonymous  bool
		id         string
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", false, id.String(), nil, http.StatusOK, ""},
		{"unauthenticated", true, id.String(), nil, http.StatusUnauthorized, "Unauthorized"},
		{"invalid id", false, "nope", nil, http.StatusBadRequest, "Invalid ID format"},
		{"not found", false, id.String(), constant.ErrNotFound, http.StatusNotFound, "Preset not found"},
		{"internal error", false, id.String(), errors.New("boom"), http.StatusInternalServerError, "Failed to get preset"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubPresetService{err: tt.svcErr}
			var user *uuid.UUID
			if !tt.anonymous {
				user = &userID
			}
			rr := httptest.NewRecorder()
			NewPresetHandler(svc, zap.NewNop()).Get(rr, debtTestReq(http.MethodGet, "/presets/"+tt.id, "", user, tt.id))
			if tt.wantStatus != http.StatusOK {
				debtAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			body := debtAssertSuccess(t, rr, http.StatusOK)
			assert.Equal(t, userID, svc.gotUser)
			assert.Equal(t, id, svc.gotID)
			assert.Equal(t, id.String(), body["data"].(map[string]interface{})["id"])
		})
	}
}

func TestPresetHandler_List(t *testing.T) {
	userID := uuid.New()

	svc := &stubPresetService{}
	rr := httptest.NewRecorder()
	NewPresetHandler(svc, zap.NewNop()).List(rr, debtTestReq(http.MethodGet, "/presets", "", &userID, ""))
	body := debtAssertSuccess(t, rr, http.StatusOK)
	assert.Equal(t, userID, svc.gotUser)
	assert.Len(t, body["data"], 2)

	svc = &stubPresetService{}
	rr = httptest.NewRecorder()
	NewPresetHandler(svc, zap.NewNop()).List(rr, debtTestReq(http.MethodGet, "/presets", "", nil, ""))
	debtAssertError(t, rr, http.StatusUnauthorized, "Unauthorized")
	assert.Zero(t, svc.calls)

	svc = &stubPresetService{err: errors.New("db down")}
	rr = httptest.NewRecorder()
	NewPresetHandler(svc, zap.NewNop()).List(rr, debtTestReq(http.MethodGet, "/presets", "", &userID, ""))
	debtAssertError(t, rr, http.StatusInternalServerError, "Failed to list presets")
}

func TestPresetHandler_Update(t *testing.T) {
	userID := uuid.New()
	id := uuid.New()
	valid := `{"name":"Tra sua","amount":45000}`
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
		{"invalid id", false, "1", valid, nil, http.StatusBadRequest, "Invalid ID format", false},
		{"malformed json", false, id.String(), `{"name":1`, nil, http.StatusBadRequest, "Invalid request payload", false},
		{"empty name", false, id.String(), `{"name":""}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"bad type", false, id.String(), `{"type":"OTHER"}`, nil, http.StatusBadRequest, "Validation failed", false},
		{"not found", false, id.String(), valid, fmt.Errorf("preset %w", constant.ErrNotFound), http.StatusNotFound, "Preset or category not found", true},
		{"internal error", false, id.String(), valid, errors.New("boom"), http.StatusInternalServerError, "Failed to update preset", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubPresetService{err: tt.svcErr}
			var user *uuid.UUID
			if !tt.anonymous {
				user = &userID
			}
			rr := httptest.NewRecorder()
			NewPresetHandler(svc, zap.NewNop()).Update(rr, debtTestReq(http.MethodPut, "/presets/"+tt.id, tt.body, user, tt.id))

			assert.Equal(t, tt.wantCalled, svc.calls == 1, "service call count %d", svc.calls)
			if tt.wantStatus != http.StatusOK {
				debtAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			body := debtAssertSuccess(t, rr, http.StatusOK)
			assert.Equal(t, userID, svc.gotUser)
			assert.Equal(t, id, svc.gotID)
			require.NotNil(t, svc.gotUpdate.Amount)
			assert.Equal(t, 45000.0, *svc.gotUpdate.Amount)
			assert.Nil(t, svc.gotUpdate.Type, "absent fields must stay nil (partial update)")
			assert.Equal(t, "Tra sua", body["data"].(map[string]interface{})["name"])
		})
	}
}

func TestPresetHandler_Delete(t *testing.T) {
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
		{"not found", false, id.String(), gorm.ErrRecordNotFound, http.StatusNotFound, "Preset not found", true},
		{"internal error", false, id.String(), errors.New("boom"), http.StatusInternalServerError, "Failed to delete preset", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubPresetService{err: tt.svcErr}
			var user *uuid.UUID
			if !tt.anonymous {
				user = &userID
			}
			rr := httptest.NewRecorder()
			NewPresetHandler(svc, zap.NewNop()).Delete(rr, debtTestReq(http.MethodDelete, "/presets/"+tt.id, "", user, tt.id))
			assert.Equal(t, tt.wantCalled, svc.calls == 1, "service call count %d", svc.calls)
			debtAssertError(t, rr, tt.wantStatus, tt.wantMsg)
		})
	}

	t.Run("success", func(t *testing.T) {
		svc := &stubPresetService{}
		rr := httptest.NewRecorder()
		NewPresetHandler(svc, zap.NewNop()).Delete(rr, debtTestReq(http.MethodDelete, "/presets/"+id.String(), "", &userID, id.String()))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.JSONEq(t, `{"status":200,"success":true,"message":"Preset deleted successfully","data":null}`, rr.Body.String())
		assert.Equal(t, userID, svc.gotUser)
		assert.Equal(t, id, svc.gotID)
	})
}
