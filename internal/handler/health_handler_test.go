package handler

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// healthTestDriver is a database/sql driver whose connections only answer Ping,
// so the health check can be exercised without a real database.
type healthTestDriver struct{ pingErr error }

func (d healthTestDriver) Open(string) (driver.Conn, error) { return healthTestConn(d), nil }

type healthTestConn struct{ pingErr error }

func (c healthTestConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (c healthTestConn) Close() error                        { return nil }
func (c healthTestConn) Begin() (driver.Tx, error)           { return nil, errors.New("not supported") }

// Ping makes healthTestConn a driver.Pinger, which sql.DB.Ping calls.
func (c healthTestConn) Ping(context.Context) error { return c.pingErr }

func init() {
	sql.Register("handlertest-health-ok", healthTestDriver{})
	sql.Register("handlertest-health-down", healthTestDriver{pingErr: errors.New("connection refused")})
}

func healthTestGorm(t *testing.T, driverName string) *gorm.DB {
	t.Helper()
	sqlDB, err := sql.Open(driverName, "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	require.NoError(t, err)
	return db
}

func TestHealthHandler_Health(t *testing.T) {
	tests := []struct {
		name       string
		db         func(t *testing.T) *gorm.DB
		wantStatus int
		wantState  string
		wantDB     string
	}{
		{"database healthy", func(t *testing.T) *gorm.DB { return healthTestGorm(t, "handlertest-health-ok") }, http.StatusOK, "healthy", "healthy"},
		{"ping fails", func(t *testing.T) *gorm.DB { return healthTestGorm(t, "handlertest-health-down") }, http.StatusServiceUnavailable, "unhealthy", "ping failed"},
		{"no sql.DB behind gorm", func(t *testing.T) *gorm.DB { return &gorm.DB{Config: &gorm.Config{}} }, http.StatusServiceUnavailable, "unhealthy", "connection failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHealthHandler(tt.db(t), zap.NewNop())
			rr := httptest.NewRecorder()

			h.Health(rr, httptest.NewRequest(http.MethodGet, "/health", nil))

			require.Equal(t, tt.wantStatus, rr.Code)
			assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
			var got HealthStatus
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
			assert.Equal(t, tt.wantState, got.Status)
			assert.Equal(t, tt.wantDB, got.Services["database"])
			assert.False(t, got.Timestamp.IsZero())
		})
	}
}

func TestHealthHandler_ReadyAndLive(t *testing.T) {
	h := NewHealthHandler(nil, zap.NewNop())

	rr := httptest.NewRecorder()
	h.Ready(rr, httptest.NewRequest(http.MethodGet, "/ready", nil))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"status":"ready"}`, rr.Body.String())

	rr = httptest.NewRecorder()
	h.Live(rr, httptest.NewRequest(http.MethodGet, "/live", nil))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"status":"alive"}`, rr.Body.String())
}
