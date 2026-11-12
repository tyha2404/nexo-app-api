package middleware_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyha2404/nexo-app-api/internal/logger"
	"github.com/tyha2404/nexo-app-api/internal/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestLoggingMiddleware(t *testing.T) {
	logg, err := logger.New("info", "dev")
	assert.NoError(t, err)

	mw := middleware.LoggingMiddleware(logg, false)

	t.Run("records 200 OK request", func(t *testing.T) {
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok response"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "ok response", rr.Body.String())
	})

	t.Run("records 404 Not Found request with query params", func(t *testing.T) {
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/unknown?page=1&limit=10", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("records 500 Internal Error request", func(t *testing.T) {
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("server error"))
		}))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("supports http.Flusher for SSE streaming", func(t *testing.T) {
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			flusher, ok := w.(http.Flusher)
			assert.True(t, ok, "ResponseWriter must implement http.Flusher")
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "data: test-chunk\n\n")
			flusher.Flush()
		}))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/stream", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "data: test-chunk")
	})
}

func TestLoggingMiddleware_ProductionLogsWithoutQueryOrColors(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	mw := middleware.LoggingMiddleware(zap.New(core), true)

	for _, status := range []int{http.StatusOK, http.StatusInternalServerError} {
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions?keyword=secret", nil)
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}

	entries := logs.All()
	require.Len(t, entries, 2)
	assert.Equal(t, "HTTP request", entries[0].Message)
	assert.Equal(t, zap.InfoLevel, entries[0].Level)
	assert.Equal(t, "HTTP Server Error", entries[1].Message)
	assert.Equal(t, zap.ErrorLevel, entries[1].Level)
	for _, e := range entries {
		fields := e.ContextMap()
		assert.Equal(t, "/api/v1/transactions", fields["path"])
		assert.NotContains(t, fields, "query")
		for _, v := range fields {
			assert.NotContains(t, fmt.Sprint(v), "secret")
			assert.NotContains(t, fmt.Sprint(v), "\x1b[")
		}
	}
}
