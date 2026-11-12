package handler

import (
	"errors"
	"net/http"

	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"go.uber.org/zap"
)

type CategoryHandler struct {
	svc          service.CategoryService
	errorHandler *ErrorHandler
}

func NewCategoryHandler(svc service.CategoryService, log *zap.Logger) *CategoryHandler {
	return &CategoryHandler{svc: svc, errorHandler: NewErrorHandler(log)}
}

// Create handles the creation of a new category record
// @Summary Create a new category
// @Description Create a new category
// @Tags categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param category body dto.CreateCategoryRequest true "Category object"
// @Success 201 {object} CategoryEnvelope
// @Failure 400 {object} response.ErrorResponse "Invalid request payload"
// @Failure 401 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse "A category with this name already exists"
// @Failure 500 {object} response.ErrorResponse "Failed to create category"
// @Router /categories [post]
func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateCategoryRequest
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "create_category")
		return
	}

	if !h.errorHandler.decodeAndValidate(w, r, &req, "create_category") {
		return
	}

	category, err := h.svc.Create(r.Context(), user.ID, req)
	if err != nil {
		if errors.Is(err, constant.ErrAlreadyExists) {
			h.errorHandler.HandleError(w, err, http.StatusConflict, "Danh mục với tên này đã tồn tại", "create_category")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to create category", "create_category")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusCreated, *category)
}

// Get handles retrieving a single category by ID
// @Summary Get a category by ID
// @Description Get a category by its ID
// @Tags categories
// @Produce json
// @Security BearerAuth
// @Param id path string true "Category ID"
// @Success 200 {object} CategoryEnvelope
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

	id, err := ParseUUIDFromPath(r, "id")
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

	h.errorHandler.HandleSuccess(w, http.StatusOK, *category)
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
// @Success 200 {object} CategoryPage
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse "Failed to list categories"
// @Router /categories [get]
func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "list_categories")
		return
	}

	categoryType := r.URL.Query().Get("type")
	query := r.URL.Query()

	var categories []model.Category
	var total int64
	var page, limit int

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

	h.errorHandler.HandlePaginated(w, http.StatusOK, categories, int(total), page, limit, nil)
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
// @Success 200 {object} CategoryEnvelope
// @Failure 400 {object} response.ErrorResponse "Invalid category ID or payload"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "Category not found (or not owned by the caller)"
// @Failure 409 {object} response.ErrorResponse "A category with this name already exists"
// @Failure 500 {object} response.ErrorResponse "Failed to update category"
// @Router /categories/{id} [put]
func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized", "update_category")
		return
	}

	id, err := ParseUUIDFromPath(r, "id")
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid category ID", "update_category")
		return
	}

	var req dto.UpdateCategoryRequest
	if !h.errorHandler.decodeAndValidate(w, r, &req, "update_category") {
		return
	}

	updatedCategory, err := h.svc.Update(r.Context(), user.ID, id, req)
	if err != nil {
		if errors.Is(err, constant.ErrInvalidInput) {
			h.errorHandler.HandleError(w, constant.ErrInvalidInput, http.StatusBadRequest, "No fields to update", "update_category")
			return
		}
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "Category not found", "update_category")
			return
		}
		if errors.Is(err, constant.ErrAlreadyExists) {
			h.errorHandler.HandleError(w, err, http.StatusConflict, "Danh mục với tên này đã tồn tại", "update_category")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to update category", "update_category")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, *updatedCategory)
}

// Delete handles deleting a category by ID
// @Summary Delete a category
// @Description Delete a category by its ID
// @Tags categories
// @Security BearerAuth
// @Param id path string true "Category ID"
// @Produce json
// @Success 200 {object} MessageResponse
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

	id, err := ParseUUIDFromPath(r, "id")
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

	h.errorHandler.HandleSuccessWithMessage(w, http.StatusOK, nil, "Category deleted successfully")
}
