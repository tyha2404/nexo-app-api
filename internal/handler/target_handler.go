package handler

import (
	"errors"
	"net/http"

	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type TargetHandler struct {
	svc          service.TargetService
	errorHandler *ErrorHandler
}

func NewTargetHandler(svc service.TargetService, log *zap.Logger) *TargetHandler {
	return &TargetHandler{
		svc:          svc,
		errorHandler: NewErrorHandler(log),
	}
}

// UpsertTarget handles POST /targets/monthly
// @Summary Set a monthly financial target
// @Description Creates or replaces the caller's EXPENSE (spending limit) or INVESTMENT target of a month.
// @Tags Targets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.UpsertTargetRequest true "Target"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} response.ErrorResponse "Invalid request payload / Validation failed / invalid target type, amount or month"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Failed to upsert target"
// @Router /targets/monthly [post]
func (h *TargetHandler) UpsertTarget(w http.ResponseWriter, r *http.Request) {
	var req dto.UpsertTargetRequest
	if !h.errorHandler.decodeAndValidate(w, r, &req, "upsert_target") {
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "upsert_target")
		return
	}

	if err := h.svc.UpsertTarget(r.Context(), user.ID, &req); err != nil {
		if errors.Is(err, constant.ErrInvalidInput) {
			h.errorHandler.HandleError(w, err, http.StatusBadRequest, err.Error(), "upsert_target")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to upsert target", "upsert_target")
		return
	}

	h.errorHandler.HandleSuccessWithMessage(w, http.StatusOK, nil, "Target saved successfully")
}

// GetSummary handles GET /targets/summary
// @Summary Get the monthly target summary
// @Description Spending vs. EXPENSE target and invested amount vs. INVESTMENT target of a month.
// @Tags Targets
// @Produce json
// @Security BearerAuth
// @Param month query int false "Month 1-12 (default: current month in the app timezone)"
// @Param year query int false "Year (default: current year)"
// @Success 200 {object} TargetSummaryEnvelope
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Failed to get target summary"
// @Router /targets/summary [get]
func (h *TargetHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "get_target_summary")
		return
	}

	// The service defaults a missing month/year to the current one.
	summary, err := h.svc.GetSummary(r.Context(), user.ID, QueryInt(r, "month"), QueryInt(r, "year"))
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get target summary", "get_target_summary")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, *summary)
}
