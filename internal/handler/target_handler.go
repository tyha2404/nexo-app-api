package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/response"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"go.uber.org/zap"
)

type TargetHandler struct {
	svc          service.TargetService
	validator    *validator.Validate
	log          *zap.Logger
	errorHandler *ErrorHandler
}

func NewTargetHandler(svc service.TargetService, log *zap.Logger) *TargetHandler {
	return &TargetHandler{
		svc:          svc,
		validator:    validator.New(),
		log:          log,
		errorHandler: NewErrorHandler(log),
	}
}

func (h *TargetHandler) UpsertTarget(w http.ResponseWriter, r *http.Request) {
	var req dto.UpsertTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "upsert_target")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "upsert_target")
		return
	}

	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "upsert_target")
		return
	}

	if err := h.svc.UpsertTarget(r.Context(), user.ID, &req); err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to upsert target", "upsert_target")
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
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "get_target_summary")
		return
	}

	now := util.Now()
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
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get target summary", "get_target_summary")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.BaseResponse[dto.TargetSummaryResponse]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *summary,
	})
}
