package handler

import (
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func errorHandlerTestObserved() (*ErrorHandler, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.DebugLevel)
	return NewErrorHandler(zap.New(core)), logs
}

func TestErrorHandler_HandleError_LogsAndHidesServerDetail(t *testing.T) {
	eh, logs := errorHandlerTestObserved()
	rr := httptest.NewRecorder()

	eh.HandleError(rr, errors.New("pq: secret table"), http.StatusInternalServerError, "Failed", "op_name")

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	body := decodeJSON(t, rr)
	assert.Equal(t, "Failed", body["message"])
	_, leaked := body["error"]
	assert.False(t, leaked)

	entries := logs.FilterMessage("operation failed").All()
	require.Len(t, entries, 1)
	ctx := entries[0].ContextMap()
	assert.Equal(t, "op_name", ctx["operation"])
	assert.Equal(t, int64(http.StatusInternalServerError), ctx["status_code"])
	assert.Equal(t, "pq: secret table", ctx["error"], "the cause must still be logged")
}

func TestErrorHandler_HandleValidationAndDecodeError(t *testing.T) {
	eh, logs := errorHandlerTestObserved()

	rr := httptest.NewRecorder()
	eh.HandleValidationError(rr, errors.New("Name is required"), "create_x")
	require.Equal(t, http.StatusBadRequest, rr.Code)
	body := decodeJSON(t, rr)
	assert.Equal(t, "Validation failed", body["message"])
	assert.Equal(t, "Name is required", body["error"])

	rr = httptest.NewRecorder()
	eh.HandleDecodeError(rr, errors.New("unexpected EOF"), "create_x")
	require.Equal(t, http.StatusBadRequest, rr.Code)
	body = decodeJSON(t, rr)
	assert.Equal(t, "Invalid request payload", body["message"])
	assert.Equal(t, "unexpected EOF", body["error"])

	assert.Equal(t, 1, logs.FilterMessage("validation failed").Len())
	assert.Equal(t, 1, logs.FilterMessage("failed to decode request body").Len())
	for _, e := range logs.All() {
		assert.Equal(t, zapcore.WarnLevel, e.Level, "client errors are warnings, not errors")
	}
}

func TestErrorHandler_HandleSuccess(t *testing.T) {
	eh, _ := errorHandlerTestObserved()
	rr := httptest.NewRecorder()

	eh.HandleSuccess(rr, http.StatusCreated, map[string]string{"id": "1"})

	require.Equal(t, http.StatusCreated, rr.Code)
	body := decodeJSON(t, rr)
	assert.Equal(t, true, body["success"])
	assert.Equal(t, float64(http.StatusCreated), body["status"])
	assert.Equal(t, map[string]interface{}{"id": "1"}, body["data"])
}

func TestErrorHandler_HandleSuccessWithMessage(t *testing.T) {
	eh, _ := errorHandlerTestObserved()
	rr := httptest.NewRecorder()

	eh.HandleSuccessWithMessage(rr, http.StatusOK, []int{1, 2}, "Đã lưu")

	require.Equal(t, http.StatusOK, rr.Code)
	body := decodeJSON(t, rr)
	assert.Equal(t, true, body["success"])
	assert.Equal(t, "Đã lưu", body["message"])
	assert.Equal(t, []interface{}{float64(1), float64(2)}, body["data"])
}

func TestErrorHandler_HandlePaginatedSuccess(t *testing.T) {
	eh, _ := errorHandlerTestObserved()
	rr := httptest.NewRecorder()

	eh.HandlePaginatedSuccess(rr, http.StatusOK, []string{"a", "b"}, 12, 2, 2)

	require.Equal(t, http.StatusOK, rr.Code)
	body := decodeJSON(t, rr)
	assert.Equal(t, map[string]interface{}{
		"status":  float64(200),
		"success": true,
		"items":   []interface{}{"a", "b"},
		"total":   float64(12),
		"page":    float64(2),
		"limit":   float64(2),
	}, body)
}

func TestErrorHandler_EncodeFailureIsLogged(t *testing.T) {
	eh, logs := errorHandlerTestObserved()

	eh.HandleSuccess(httptest.NewRecorder(), http.StatusOK, math.NaN())
	eh.HandleSuccessWithMessage(httptest.NewRecorder(), http.StatusOK, math.Inf(1), "x")
	eh.HandlePaginatedSuccess(httptest.NewRecorder(), http.StatusOK, []float64{math.NaN()}, 1, 1, 1)

	assert.Equal(t, 2, logs.FilterMessage("failed to encode response").Len())
	assert.Equal(t, 1, logs.FilterMessage("failed to encode paginated response").Len())
}
