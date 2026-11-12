package handler

import (
	"net/http"

	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type AlertHandler struct {
	svc          service.AlertService
	errorHandler *ErrorHandler
}

func NewAlertHandler(svc service.AlertService, log *zap.Logger) *AlertHandler {
	return &AlertHandler{svc: svc, errorHandler: NewErrorHandler(log)}
}

// List handles GET /alerts
// @Summary List budget alerts
// @Description Paginated budget alerts of the caller, newest first.
// @Tags Alerts
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page (default 1)"
// @Param limit query int false "Page size (default 10, max 100)"
// @Success 200 {object} AlertPage
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Failed to list alerts"
// @Router /alerts [get]
func (h *AlertHandler) List(w http.ResponseWriter, r *http.Request) {
	page, limit := ParsePagination(r)

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "list_alerts")
		return
	}
	userID := user.ID
	items, total, err := h.svc.ListAlerts(r.Context(), userID, page, limit)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to list alerts", "list_alerts")
		return
	}

	h.errorHandler.HandlePaginated(w, http.StatusOK, items, int(total), page, limit, nil)
}

// Delete handles DELETE /alerts/{id}
// @Summary Delete a budget alert
// @Tags Alerts
// @Produce json
// @Security BearerAuth
// @Param id path string true "Alert ID"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} response.ErrorResponse "Invalid ID format"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Alert not found"
// @Failure 500 {object} response.ErrorResponse "Failed to delete alert"
// @Router /alerts/{id} [delete]
func (h *AlertHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid ID format", "delete_alert")
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "delete_alert")
		return
	}
	userID := user.ID
	if err := h.svc.DeleteAlert(r.Context(), userID, id); err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Alert not found", "delete_alert")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to delete alert", "delete_alert")
		return
	}

	h.errorHandler.HandleSuccessWithMessage(w, http.StatusOK, nil, "Alert deleted successfully")
}
