package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/response"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type CategoryHandler struct {
	svc          service.CategoryService
	log          *zap.Logger
	errorHandler *ErrorHandler
}

func NewCategoryHandler(svc service.CategoryService, log *zap.Logger) *CategoryHandler {
	return &CategoryHandler{svc: svc, log: log, errorHandler: NewErrorHandler(log)}
}

// Create handles the creation of a new category record
// @Summary Create a new category
// @Description Create a new category
// @Tags categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param category body dto.CreateCategoryRequest true "Category object"
// @Success 201 {object} model.Category
// @Failure 400 {object} response.ErrorResponse "Invalid request payload"
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse "Failed to create category"
// @Router /categories [post]
func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateCategoryRequest
	// Get user from context
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "create_category")
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "create_category")
		return
	}

	category := &model.Category{
		Name:        req.Name,
		Type:        model.CategoryType(req.Type),
		Description: req.Description,
		UserID:      user.ID,
	}
	if req.ExcludeFromAverageDaily != nil {
		category.ExcludeFromAverageDaily = *req.ExcludeFromAverageDaily
	}

	category, err := h.svc.Create(r.Context(), category)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to create category", "create_category")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response.BaseResponse[model.Category]{
		Status:  http.StatusCreated,
		Success: true,
		Data:    *category,
	}); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// Get handles retrieving a single category by ID
// @Summary Get a category by ID
// @Description Get a category by its ID
// @Tags categories
// @Produce json
// @Security BearerAuth
// @Param id path string true "Category ID"
// @Success 200 {object} model.Category
// @Failure 400 {object} response.ErrorResponse "Invalid category ID"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Category not found (or not owned by the caller)"
// @Failure 500 {object} response.ErrorResponse "Failed to get category"
// @Router /categories/{id} [get]
func (h *CategoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "get_category")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid category ID", "get_category")
		return
	}

	category, err := h.svc.Get(r.Context(), user.ID, id)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Category not found", "get_category")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get category", "get_category")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response.BaseResponse[model.Category]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *category,
	}); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// List handles retrieving a paginated list of categories
// @Summary List categories
// @Description Get a paginated list of categories
// @Tags categories
// @Produce json
// @Security BearerAuth
// @Description Without page/limit the full list is returned; with either, results are paginated (default limit 10, max 100).
// @Param page query int false "Page number"
// @Param limit query int false "Page limit (default 10, max 100)"
// @Param type query string false "Category Type (INCOME, EXPENSE)"
// @Success 200 {array} model.Category
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse "Failed to list categories"
// @Router /categories [get]
func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		h.errorHandler.HandleError(w, constant.ErrUnauthorized, http.StatusUnauthorized, "Unauthorized", "list_categories")
		return
	}

	categoryType := r.URL.Query().Get("type")
	query := r.URL.Query()

	var categories []model.Category
	var total int64
	var page, limit int
	var err error

	// Paginate only when the client asks for it; without page/limit the full
	// list is returned (nexo-web relies on this for category pickers).
	if query.Get("page") != "" || query.Get("limit") != "" {
		page, limit = ParsePagination(r)
		categories, total, err = h.svc.ListWithTotal(r.Context(), user.ID, categoryType, page, limit)
	} else {
		// Fallback for full list fetching without pagination parameters
		categories, err = h.svc.List(r.Context(), user.ID, categoryType, 0, 0)
		total = int64(len(categories))
		page = 1
		limit = len(categories)
	}

	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to list categories", "list_categories")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response.PaginationResponse[model.Category]{
		Status:  http.StatusOK,
		Success: true,
		Items:   categories,
		Total:   int(total),
		Page:    page,
		Limit:   limit,
	}); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// Update handles updating an existing category
// @Summary Update a category
// @Description Update an existing category
// @Tags categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Category ID"
// @Param category body dto.UpdateCategoryRequest true "Fields to update"
// @Success 200 {object} model.Category
// @Failure 400 {object} response.ErrorResponse "Invalid category ID or payload"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Category not found (or not owned by the caller)"
// @Failure 500 {object} response.ErrorResponse "Failed to update category"
// @Router /categories/{id} [put]
func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "update_category")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid category ID", "update_category")
		return
	}

	var req dto.UpdateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorHandler.HandleDecodeError(w, err, "update_category")
		return
	}

	// Build updates map from non-nil fields
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.ExcludeFromAverageDaily != nil {
		updates["exclude_from_average_daily"] = *req.ExcludeFromAverageDaily
	}

	if len(updates) == 0 {
		h.errorHandler.HandleError(w, constant.ErrInvalidInput, http.StatusBadRequest, "No fields to update", "update_category")
		return
	}

	// Update specific fields
	if err := h.svc.UpdateFields(r.Context(), user.ID, id, updates); err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Category not found", "update_category")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to update category", "update_category")
		return
	}

	// Get the updated category to return in response
	updatedCategory, err := h.svc.Get(r.Context(), user.ID, id)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Category not found", "update_category")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get updated category", "update_category")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response.BaseResponse[model.Category]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *updatedCategory,
	}); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// Delete handles deleting a category by ID
// @Summary Delete a category
// @Description Delete a category by its ID
// @Tags categories
// @Security BearerAuth
// @Param id path string true "Category ID"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} response.ErrorResponse "Invalid category ID"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Category not found (or not owned by the caller)"
// @Failure 500 {object} response.ErrorResponse "Failed to delete category"
// @Router /categories/{id} [delete]
func (h *CategoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "delete_category")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid category ID", "delete_category")
		return
	}

	if err := h.svc.Delete(r.Context(), user.ID, id); err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Category not found", "delete_category")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to delete category", "delete_category")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
