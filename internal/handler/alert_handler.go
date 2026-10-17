package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/response"
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.PaginationResponse[dto.AlertResponse]{
		Status:  http.StatusOK,
		Success: true,
		Items:   items,
		Total:   int(total),
		Page:    page,
		Limit:   limit,
	})
}

func (h *AlertHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Alert deleted successfully"})
}
