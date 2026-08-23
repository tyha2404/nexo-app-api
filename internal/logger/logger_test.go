package logger_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tyha2404/nexo-app-api/internal/logger"
)

func TestNewLogger(t *testing.T) {
	t.Run("creates dev logger with console encoder", func(t *testing.T) {
		logg, err := logger.New("info", "dev")
		assert.NoError(t, err)
		assert.NotNil(t, logg)
	})

	t.Run("creates prod logger with json encoder", func(t *testing.T) {
		logg, err := logger.New("info", "prod")
		assert.NoError(t, err)
		assert.NotNil(t, logg)
	})

	t.Run("creates default logger when env not provided", func(t *testing.T) {
		logg, err := logger.New("info")
		assert.NoError(t, err)
		assert.NotNil(t, logg)
	})
}

func TestColorHelpers(t *testing.T) {
	assert.Contains(t, logger.ColorMethod(http.MethodGet), "GET")
	assert.Contains(t, logger.ColorMethod(http.MethodPost), "POST")
	assert.Contains(t, logger.ColorMethod(http.MethodPut), "PUT")
	assert.Contains(t, logger.ColorMethod(http.MethodPatch), "PATCH")
	assert.Contains(t, logger.ColorMethod(http.MethodDelete), "DELETE")

	assert.Contains(t, logger.ColorStatus(200), "200 OK")
	assert.Contains(t, logger.ColorStatus(201), "201 CREATED")
	assert.Contains(t, logger.ColorStatus(400), "400 BAD REQ")
	assert.Contains(t, logger.ColorStatus(401), "401 UNAUTH")
	assert.Contains(t, logger.ColorStatus(404), "404 NOT FOUND")
	assert.Contains(t, logger.ColorStatus(500), "500 ERR")

	assert.NotEmpty(t, logger.ColorDuration(5*time.Millisecond))
	assert.NotEmpty(t, logger.ColorDuration(25*time.Millisecond))
	assert.NotEmpty(t, logger.ColorDuration(100*time.Millisecond))
	assert.NotEmpty(t, logger.ColorDuration(300*time.Millisecond))

	urlStr := logger.ColorURL("/api/v1/transactions", "page=1&limit=10")
	assert.Contains(t, urlStr, "/api/v1")
	assert.Contains(t, urlStr, "/transactions")
	assert.Contains(t, urlStr, "page=1&limit=10")

	assert.Contains(t, logger.ColorBytes(500), "500 B")
	assert.Contains(t, logger.ColorBytes(2048), "2.0 KB")

	sqlHighlighted := logger.HighlightSQL("SELECT id, name FROM users WHERE id = 1 ORDER BY id DESC")
	assert.Contains(t, sqlHighlighted, "SELECT")
	assert.Contains(t, sqlHighlighted, "FROM")
	assert.Contains(t, sqlHighlighted, "WHERE")
	assert.Contains(t, sqlHighlighted, "ORDER BY")
}
