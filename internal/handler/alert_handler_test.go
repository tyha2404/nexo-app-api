package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type stubAlertService struct {
	service.AlertService
	listFn   func(ctx context.Context, userID uuid.UUID, page, limit int) ([]dto.AlertResponse, int64, error)
	deleteFn func(ctx context.Context, userID, id uuid.UUID) error
	calls    int
}

func (s *stubAlertService) ListAlerts(ctx context.Context, userID uuid.UUID, page, limit int) ([]dto.AlertResponse, int64, error) {
	s.calls++
	return s.listFn(ctx, userID, page, limit)
}

func (s *stubAlertService) DeleteAlert(ctx context.Context, userID, id uuid.UUID) error {
	s.calls++
	return s.deleteFn(ctx, userID, id)
}

func TestAlertHandler_List(t *testing.T) {
	userID := uuid.New()

	t.Run("success", func(t *testing.T) {
		var gotUser uuid.UUID
		var gotPage, gotLimit int
		svc := &stubAlertService{listFn: func(ctx context.Context, uid uuid.UUID, page, limit int) ([]dto.AlertResponse, int64, error) {
			gotUser, gotPage, gotLimit = uid, page, limit
			return []dto.AlertResponse{{ID: uuid.New(), UserID: uid, AlertType: "BUDGET_EXCEEDED"}}, 7, nil
		}}
		rr := httptest.NewRecorder()
		NewAlertHandler(svc, zap.NewNop()).List(rr, txnTestReq(http.MethodGet, "/alerts?page=2&limit=5", "", userID, ""))

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, userID, gotUser)
		assert.Equal(t, 2, gotPage)
		assert.Equal(t, 5, gotLimit)
		body := decodeJSON(t, rr)
		assert.Equal(t, true, body["success"])
		assert.Equal(t, float64(7), body["total"])
		assert.Equal(t, float64(2), body["page"])
		assert.Equal(t, float64(5), body["limit"])
		require.Len(t, body["items"], 1)
		assert.Equal(t, "BUDGET_EXCEEDED", body["items"].([]interface{})[0].(map[string]interface{})["alertType"])
	})

	t.Run("unauthenticated", func(t *testing.T) {
		svc := &stubAlertService{}
		rr := httptest.NewRecorder()
		NewAlertHandler(svc, zap.NewNop()).List(rr, txnTestReq(http.MethodGet, "/alerts", "", uuid.Nil, ""))
		txnAssertError(t, rr, http.StatusUnauthorized, "Unauthorized")
		assert.Zero(t, svc.calls)
	})

	t.Run("service error", func(t *testing.T) {
		svc := &stubAlertService{listFn: func(ctx context.Context, uid uuid.UUID, page, limit int) ([]dto.AlertResponse, int64, error) {
			return nil, 0, errors.New("boom")
		}}
		rr := httptest.NewRecorder()
		NewAlertHandler(svc, zap.NewNop()).List(rr, txnTestReq(http.MethodGet, "/alerts", "", userID, ""))
		txnAssertError(t, rr, http.StatusInternalServerError, "Failed to list alerts")
	})
}

func TestAlertHandler_Delete(t *testing.T) {
	userID, alertID := uuid.New(), uuid.New()
	tests := []struct {
		name       string
		id         string
		user       uuid.UUID
		svcErr     error
		wantStatus int
		wantMsg    string
	}{
		{"success", alertID.String(), userID, nil, http.StatusOK, ""},
		{"invalid uuid", "x", userID, nil, http.StatusBadRequest, "Invalid ID format"},
		{"unauthenticated", alertID.String(), uuid.Nil, nil, http.StatusUnauthorized, "Unauthorized"},
		{"not found", alertID.String(), userID, constant.ErrNotFound, http.StatusNotFound, "Alert not found"},
		{"unexpected", alertID.String(), userID, errors.New("boom"), http.StatusInternalServerError, "Failed to delete alert"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUser, gotID uuid.UUID
			svc := &stubAlertService{deleteFn: func(ctx context.Context, uid, id uuid.UUID) error {
				gotUser, gotID = uid, id
				return tt.svcErr
			}}
			rr := httptest.NewRecorder()
			NewAlertHandler(svc, zap.NewNop()).Delete(rr, txnTestReq(http.MethodDelete, "/alerts/"+tt.id, "", tt.user, tt.id))

			if tt.wantStatus != http.StatusOK {
				txnAssertError(t, rr, tt.wantStatus, tt.wantMsg)
				return
			}
			require.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, userID, gotUser)
			assert.Equal(t, alertID, gotID)
			assert.JSONEq(t, `{"message":"Alert deleted successfully"}`, rr.Body.String())
		})
	}
}
