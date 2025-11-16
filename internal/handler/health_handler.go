package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db  *gorm.DB
	log *zap.Logger
}

type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Services  map[string]string `json:"services"`
}

func NewHealthHandler(db *gorm.DB, log *zap.Logger) *HealthHandler {
	return &HealthHandler{db: db, log: log}
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
	} else if err := sqlDB.Ping(); err != nil {
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

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	// Readiness probe - check if application is ready to serve traffic
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	// Liveness probe - check if application is alive
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}
