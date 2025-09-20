package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
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
// @Param category body model.Category true "Category object"
// @Success 201 {object} model.Category
// @Failure 400 {string} string "Invalid request payload"
// @Failure 500 {string} string "Failed to create category"
// @Router /categories [post]
func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.Category
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed to decode request body", zap.Error(err))
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	category, err := h.svc.Create(r.Context(), &req)
	if err != nil {
		h.log.Error("failed to create category", zap.Error(err))
		http.Error(w, "Failed to create category", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(category); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// Get handles retrieving a single category by ID
// @Summary Get a category by ID
// @Description Get a category by its ID
// @Tags categories
// @Produce json
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
	if err := json.NewEncoder(w).Encode(category); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// List handles retrieving a paginated list of categories
// @Summary List categories
// @Description Get a paginated list of categories
// @Tags categories
// @Produce json
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Success 200 {array} model.Category
// @Failure 500 {string} string "Failed to list categories"
// @Router /categories [get]
func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

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

	categories, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		h.log.Error("failed to list categories", zap.Error(err))
		http.Error(w, "Failed to list categories", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(categories); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// Update handles updating an existing category
// @Summary Update a category
// @Description Update an existing category
// @Tags categories
// @Accept json
// @Produce json
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

	var req model.Category
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed to decode request body", zap.Error(err))
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	req.ID = id

	updatedCategory, err := h.svc.Update(r.Context(), &req)
	if err != nil {
		if err == constant.ErrNotFound {
			http.Error(w, "Category not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed to update category", zap.Error(err))
		http.Error(w, "Failed to update category", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(updatedCategory); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// Delete handles deleting a category by ID
// @Summary Delete a category
// @Description Delete a category by its ID
// @Tags categories
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
