package handler

import (
	"net/http"

	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type CostHandler struct {
	svc          service.CostService
	log          *zap.Logger
	errorHandler *ErrorHandler
	validator    *Validator
}

func NewCostHandler(svc service.CostService, log *zap.Logger) *CostHandler {
	return &CostHandler{
		svc:          svc,
		log:          log,
		errorHandler: NewErrorHandler(log),
		validator:    NewValidator(),
	}
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
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, "cost_create")
		return
	}

	if err := h.validator.ValidateRequest(r, &req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "cost_create")
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

	createdCost, err := h.svc.Create(r.Context(), cost)
	if err != nil {
		h.errorHandler.HandleError(w, err, "cost_create")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusCreated, *createdCost)
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
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, "cost_get")
		return
	}

	id, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, "cost_get")
		return
	}

	cost, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.errorHandler.HandleError(w, err, "cost_get")
		return
	}

	// Verify user has access to this cost
	if cost.UserID != user.ID {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, "cost_get")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, *cost)
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
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, "cost_list")
		return
	}

	limit := ParseQueryIntWithValidation(r, "limit", 10, 1)
	offset := ParseQueryIntWithValidation(r, "offset", 0, 0)

	filters := BuildFilterMap(r, []string{"startDate", "endDate"})

	costs, err := h.svc.ListWithCategory(r.Context(), user.ID, limit, offset, filters)
	if err != nil {
		h.errorHandler.HandleError(w, err, "cost_list")
		return
	}

	h.errorHandler.HandlePaginatedSuccess(w, http.StatusOK, costs, len(costs), offset, limit)
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
	id, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, "cost_update")
		return
	}

	var req model.Cost
	if err := h.validator.ValidatePartial(r, &req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "cost_update")
		return
	}
	req.ID = id

	updatedCost, err := h.svc.Update(r.Context(), &req)
	if err != nil {
		h.errorHandler.HandleError(w, err, "cost_update")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, *updatedCost)
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
	id, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, "cost_delete")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.errorHandler.HandleError(w, err, "cost_delete")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
