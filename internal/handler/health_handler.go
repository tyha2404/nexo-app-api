package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"gorm.io/gorm"
)

// healthPingTimeout bounds the database ping so a hung connection cannot
// stall the health check.
const healthPingTimeout = 2 * time.Second

type HealthHandler struct {
	db *gorm.DB
}

type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]string `json:"services"`
}

func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	status := HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Services:  make(map[string]string),
	}

	// Check database
	sqlDB, err := h.db.DB()
	if err != nil {
		status.Status = "unhealthy"
		status.Services["database"] = "connection failed"
	} else if err := pingWithTimeout(r.Context(), sqlDB.PingContext); err != nil {
		status.Status = "unhealthy"
		status.Services["database"] = "ping failed"
	} else {
		status.Services["database"] = "healthy"
	}

	statusCode := http.StatusOK
	if status.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(status)
}

// pingWithTimeout runs ping with a context that expires after healthPingTimeout.
func pingWithTimeout(parent context.Context, ping func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(parent, healthPingTimeout)
	defer cancel()
	return ping(ctx)
}

// Ready is a static readiness probe: it answers "ready" whenever the process
// can serve HTTP and does not check dependencies (use /health for the
// database check).
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	// Liveness probe - check if application is alive
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}
