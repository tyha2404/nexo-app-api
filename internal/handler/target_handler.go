package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/response"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type TargetHandler struct {
	svc       service.TargetService
	validator *validator.Validate
	log       *zap.Logger
}

func NewTargetHandler(svc service.TargetService, log *zap.Logger) *TargetHandler {
	return &TargetHandler{
		svc:       svc,
		validator: validator.New(),
		log:       log,
	}
}

func (h *TargetHandler) UpsertTarget(w http.ResponseWriter, r *http.Request) {
	var req dto.UpsertTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.validator.Struct(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.svc.UpsertTarget(r.Context(), user.ID, &req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response.BaseResponse[map[string]string]{
		Status:  http.StatusOK,
		Success: true,
		Message: "Target saved successfully",
	})
}

func (h *TargetHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	now := time.Now()
	month, _ := strconv.Atoi(r.URL.Query().Get("month"))
	if month <= 0 {
		month = int(now.Month())
	}

	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	if year <= 0 {
		year = now.Year()
	}

	summary, err := h.svc.GetSummary(r.Context(), user.ID, month, year)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[dto.TargetSummaryResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *summary,
	})
}
