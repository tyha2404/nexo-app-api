package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/response"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type CostHandler struct {
	svc service.CostService
	log *zap.Logger
}

func NewCostHandler(svc service.CostService, log *zap.Logger) *CostHandler {
	return &CostHandler{svc: svc, log: log}
}

// Create handles the creation of a new cost record
// @Summary Create a new cost
// @Description Create a new cost
// @Tags costs
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param cost body dto.CreateCostRequest true "Cost object"
// @Success 201 {object} model.Cost
// @Failure 400 {string} string "Invalid request payload"
// @Failure 500 {string} string "Failed to create cost"
// @Router /costs [post]
func (h *CostHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateCostRequest
	// Get user from context
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		h.log.Error("User ID not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed to decode request body", zap.Error(err))
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	cost := &model.Cost{
		Title:      req.Title,
		Amount:     req.Amount,
		Currency:   req.Currency,
		IncurredAt: req.IncurredAt.Time,
		CategoryID: req.CategoryID,
		UserID:     user.ID,
	}

	cost, err := h.svc.Create(r.Context(), cost)
	if err != nil {
		h.log.Error("failed to create cost", zap.Error(err))
		http.Error(w, "Failed to create cost", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response.BaseResponse[model.Cost]{
		Status:  http.StatusCreated,
		Success: true,
		Data:    *cost,
	}); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// Get handles retrieving a single cost by ID
// @Summary Get a cost by ID
// @Description Get a cost by its ID
// @Tags costs
// @Produce json
// @Security BearerAuth
// @Param id path string true "Cost ID"
// @Success 200 {object} model.Cost
// @Failure 400 {string} string "Invalid cost ID"
// @Failure 404 {string} string "Cost not found"
// @Failure 500 {string} string "Failed to get cost"
// @Router /costs/{id} [get]
func (h *CostHandler) Get(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		h.log.Error("User ID not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid cost ID", http.StatusBadRequest)
		return
	}

	cost, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if err == constant.ErrNotFound {
			http.Error(w, "Cost not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed to get cost", zap.Error(err))
		http.Error(w, "Failed to get cost", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response.BaseResponse[model.Cost]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *cost,
	}); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// List handles retrieving a paginated list of costs
// @Summary List costs
// @Description Get a paginated list of costs
// @Tags costs
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Param startDate query string false "Start date filter (YYYY-MM-DD)"
// @Param endDate query string false "End date filter (YYYY-MM-DD)"
// @Success 200 {array} model.Cost
// @Failure 500 {string} string "Failed to list costs"
// @Router /costs [get]
func (h *CostHandler) List(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		h.log.Error("User ID not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	startDateStr := r.URL.Query().Get("startDate")
	endDateStr := r.URL.Query().Get("endDate")

	limit := 10
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	costs, err := h.svc.ListWithCategory(r.Context(), user.ID, limit, offset, map[string]interface{}{
		"startDate": startDateStr,
		"endDate":   endDateStr,
	})
	if err != nil {
		h.log.Error("failed to list costs", zap.Error(err))
		http.Error(w, "Failed to list costs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response.PaginationResponse[model.Cost]{
		Status:  http.StatusOK,
		Success: true,
		Items:   costs,
		Total:   len(costs),
		Page:    offset,
		Limit:   limit,
	}); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// Update handles updating an existing cost
// @Summary Update a cost
// @Description Update an existing cost
// @Tags costs
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Cost ID"
// @Param cost body model.Cost true "Cost object"
// @Success 200 {object} model.Cost
// @Failure 400 {string} string "Invalid cost ID or payload"
// @Failure 404 {string} string "Cost not found"
// @Failure 500 {string} string "Failed to update cost"
// @Router /costs/{id} [put]
func (h *CostHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid cost ID", http.StatusBadRequest)
		return
	}

	var req model.Cost
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed to decode request body", zap.Error(err))
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	req.ID = id

	updatedCost, err := h.svc.Update(r.Context(), &req)
	if err != nil {
		if err == constant.ErrNotFound {
			http.Error(w, "Cost not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed to update cost", zap.Error(err))
		http.Error(w, "Failed to update cost", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response.BaseResponse[model.Cost]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *updatedCost,
	}); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// Delete handles deleting a cost by ID
// @Summary Delete a cost
// @Description Delete a cost by its ID
// @Tags costs
// @Security BearerAuth
// @Param id path string true "Cost ID"
// @Success 204 {string} string "No Content"
// @Failure 400 {string} string "Invalid cost ID"
// @Failure 404 {string} string "Cost not found"
// @Failure 500 {string} string "Failed to delete cost"
// @Router /costs/{id} [delete]
func (h *CostHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid cost ID", http.StatusBadRequest)
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		if err == constant.ErrNotFound {
			http.Error(w, "Cost not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed to delete cost", zap.Error(err))
		http.Error(w, "Failed to delete cost", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
