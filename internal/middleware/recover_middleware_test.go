package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestRecoverer_PanicBecomesJSON500(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	h := Recoverer(zap.New(core))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "https://app.example") // set by an earlier middleware
		var m map[string]int
		m["boom"]++ // nil map write
	}))
	rr := httptest.NewRecorder()
	require.NotPanics(t, func() { h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)) })

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	assert.Equal(t, "https://app.example", rr.Header().Get("Access-Control-Allow-Origin"), "CORS headers already set are kept")
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, false, body["success"])
	assert.Equal(t, RecoverPanicMessage, body["message"])
	_, leaked := body["error"]
	assert.False(t, leaked, "the panic value is not sent to the client")

	require.Equal(t, 1, logs.Len())
	entry := logs.All()[0]
	assert.Contains(t, entry.ContextMap()["stack"], "recover_middleware_test", "the stack is logged")
	assert.Equal(t, "/api/v1/x", entry.ContextMap()["path"])
}

func TestRecoverer_AfterResponseStartedOnlyLogs(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	h := Recoverer(zap.New(core))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: hi\n\n"))
		w.(http.Flusher).Flush()
		panic(errors.New("mid stream"))
	}))
	rr := httptest.NewRecorder()
	require.NotPanics(t, func() { h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/chat/stream", nil)) })

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "data: hi\n\n", rr.Body.String(), "no JSON appended to a started stream")
	assert.Equal(t, 1, logs.Len())
}

func TestRecoverer_ReRaisesErrAbortHandler(t *testing.T) {
	h := Recoverer(zap.NewNop())(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	})
}

func TestRecoverer_PassesThrough(t *testing.T) {
	h := Recoverer(zap.NewNop())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusTeapot, rr.Code)
}
