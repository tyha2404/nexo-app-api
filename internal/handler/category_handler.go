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

type CategoryHandler struct {
	svc service.CategoryService
	log *zap.Logger
}

func NewCategoryHandler(svc service.CategoryService, log *zap.Logger) *CategoryHandler {
	return &CategoryHandler{svc: svc, log: log}
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
// @Failure 400 {string} string "Invalid request payload"
// @Failure 500 {string} string "Failed to create category"
// @Router /categories [post]
func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateCategoryRequest
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
		h.log.Error("failed to create category", zap.Error(err))
		http.Error(w, "Failed to create category", http.StatusInternalServerError)
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
// @Failure 400 {string} string "Invalid category ID"
// @Failure 404 {string} string "Category not found"
// @Failure 500 {string} string "Failed to get category"
// @Router /categories/{id} [get]
func (h *CategoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	category, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if err == constant.ErrNotFound {
			http.Error(w, "Category not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed to get category", zap.Error(err))
		http.Error(w, "Failed to get category", http.StatusInternalServerError)
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
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Param type query string false "Category Type (INCOME, EXPENSE)"
// @Success 200 {array} model.Category
// @Failure 500 {string} string "Failed to list categories"
// @Router /categories [get]
func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		h.log.Error("User ID not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	categoryType := r.URL.Query().Get("type")

	// If page or limit not provided, allow limit = 0 (fetch all) or handle default
	if page < 1 && limit > 0 {
		page = 1
	}

	var categories []model.Category
	var total int64
	var err error

	if page > 0 || limit > 0 {
		if page < 1 {
			page = 1
		}
		if limit < 1 {
			limit = 10
		}
		categories, total, err = h.svc.ListWithTotal(r.Context(), user.ID, categoryType, page, limit)
	} else {
		// Fallback for full list fetching without pagination parameters
		categories, err = h.svc.List(r.Context(), user.ID, categoryType, 0, 0)
		total = int64(len(categories))
		page = 1
		limit = len(categories)
	}

	if err != nil {
		h.log.Error("failed to list categories", zap.Error(err))
		http.Error(w, "Failed to list categories", http.StatusInternalServerError)
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
// @Param category body model.Category true "Category object"
// @Success 200 {object} model.Category
// @Failure 400 {string} string "Invalid category ID or payload"
// @Failure 404 {string} string "Category not found"
// @Failure 500 {string} string "Failed to update category"
// @Router /categories/{id} [put]
func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	var req dto.UpdateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed to decode request body", zap.Error(err))
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
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
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	// Update specific fields
	if err := h.svc.UpdateFields(r.Context(), id, updates); err != nil {
		if err == constant.ErrNotFound {
			http.Error(w, "Category not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed to update category", zap.Error(err))
		http.Error(w, "Failed to update category", http.StatusInternalServerError)
		return
	}

	// Get the updated category to return in response
	updatedCategory, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.log.Error("failed to get updated category", zap.Error(err))
		http.Error(w, "Failed to get updated category", http.StatusInternalServerError)
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
// @Failure 400 {string} string "Invalid category ID"
// @Failure 404 {string} string "Category not found"
// @Failure 500 {string} string "Failed to delete category"
// @Router /categories/{id} [delete]
func (h *CategoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		if err == constant.ErrNotFound {
			http.Error(w, "Category not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed to delete category", zap.Error(err))
		http.Error(w, "Failed to delete category", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
