package handler

import (
	"errors"
	"net/http"

	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type BudgetHandler struct {
	svc          service.BudgetService
	errorHandler *ErrorHandler
}

func NewBudgetHandler(svc service.BudgetService, log *zap.Logger) *BudgetHandler {
	return &BudgetHandler{
		svc:          svc,
		errorHandler: NewErrorHandler(log),
	}
}

// Create handles POST /budgets
// @Summary Create a budget
// @Description Creates a budget for one of the caller's categories and period.
// @Tags Budgets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateBudgetRequest true "Budget"
// @Success 201 {object} BudgetEnvelope
// @Failure 400 {object} response.ErrorResponse "Invalid request payload / Validation failed"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Category not found"
// @Failure 409 {object} response.ErrorResponse "A budget for this category and period already exists"
// @Failure 500 {object} response.ErrorResponse "Failed to create budget"
// @Router /budgets [post]
func (h *BudgetHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateBudgetRequest
	if !h.errorHandler.decodeAndValidate(w, r, &req, "create_budget") {
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "create_budget")
		return
	}
	userID := user.ID
	res, err := h.svc.CreateBudget(r.Context(), userID, req)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Category not found", "create_budget")
			return
		}
		if errors.Is(err, constant.ErrAlreadyExists) {
			h.errorHandler.HandleError(w, err, http.StatusConflict, "Ngân sách cho danh mục và kỳ này đã tồn tại", "create_budget")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to create budget", "create_budget")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusCreated, *res)
}

// Get handles GET /budgets/{id}
// @Summary Get a budget
// @Tags Budgets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Budget ID"
// @Success 200 {object} BudgetEnvelope
// @Failure 400 {object} response.ErrorResponse "Invalid ID format"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Budget not found"
// @Failure 500 {object} response.ErrorResponse "Failed to get budget"
// @Router /budgets/{id} [get]
func (h *BudgetHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid ID format", "get_budget")
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "get_budget")
		return
	}
	userID := user.ID
	res, err := h.svc.GetBudget(r.Context(), userID, id)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Budget not found", "get_budget")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get budget", "get_budget")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, *res)
}

// List handles GET /budgets
// @Summary List budgets
// @Tags Budgets
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page (default 1)"
// @Param limit query int false "Page size (default 10, max 100)"
// @Success 200 {object} BudgetPage
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 500 {object} response.ErrorResponse "Failed to list budgets"
// @Router /budgets [get]
func (h *BudgetHandler) List(w http.ResponseWriter, r *http.Request) {
	page, limit := ParsePagination(r)

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "list_budgets")
		return
	}
	userID := user.ID
	items, total, err := h.svc.ListBudgets(r.Context(), userID, page, limit)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to list budgets", "list_budgets")
		return
	}

	h.errorHandler.HandlePaginated(w, http.StatusOK, items, int(total), page, limit, nil)
}

// Update handles PUT /budgets/{id}
// @Summary Update a budget
// @Tags Budgets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Budget ID"
// @Param request body dto.UpdateBudgetRequest true "Fields to update"
// @Success 200 {object} BudgetEnvelope
// @Failure 400 {object} response.ErrorResponse "Invalid ID format / Invalid request payload / Validation failed"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Budget not found"
// @Failure 409 {object} response.ErrorResponse "A budget for this category and period already exists"
// @Failure 500 {object} response.ErrorResponse "Failed to update budget"
// @Router /budgets/{id} [put]
func (h *BudgetHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid ID format", "update_budget")
		return
	}

	var req dto.UpdateBudgetRequest
	if !h.errorHandler.decodeAndValidate(w, r, &req, "update_budget") {
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "update_budget")
		return
	}
	userID := user.ID
	res, err := h.svc.UpdateBudget(r.Context(), userID, id, req)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Budget not found", "update_budget")
			return
		}
		if errors.Is(err, constant.ErrAlreadyExists) {
			h.errorHandler.HandleError(w, err, http.StatusConflict, "Ngân sách cho danh mục và kỳ này đã tồn tại", "update_budget")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to update budget", "update_budget")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, *res)
}

// Delete handles DELETE /budgets/{id}
// @Summary Delete a budget
// @Tags Budgets
// @Produce json
// @Security BearerAuth
// @Param id path string true "Budget ID"
// @Success 200 {object} MessageResponse
// @Failure 400 {object} response.ErrorResponse "Invalid ID format"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Budget not found"
// @Failure 500 {object} response.ErrorResponse "Failed to delete budget"
// @Router /budgets/{id} [delete]
func (h *BudgetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid ID format", "delete_budget")
		return
	}

	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "delete_budget")
		return
	}
	userID := user.ID
	if err := h.svc.DeleteBudget(r.Context(), userID, id); err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Budget not found", "delete_budget")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to delete budget", "delete_budget")
		return
	}

	h.errorHandler.HandleSuccessWithMessage(w, http.StatusOK, nil, "Budget deleted successfully")
}
