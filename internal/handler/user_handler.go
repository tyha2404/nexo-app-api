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

type UserHandler struct {
	svc service.UserService
	log *zap.Logger
}

func NewUserHandler(svc service.UserService, log *zap.Logger) *UserHandler {
	return &UserHandler{svc: svc, log: log}
}

// Create handles the creation of a new user record
// @Summary Create a new user
// @Description Create a new user
// @Tags users
// @Accept json
// @Produce json
// @Param user body model.User true "User object"
// @Success 201 {object} model.User
// @Failure 400 {string} string "Invalid request payload"
// @Failure 500 {string} string "Failed to create user"
// @Router /users [post]
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.User
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed to decode request body", zap.Error(err))
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	user, err := h.svc.Create(r.Context(), &req)
	if err != nil {
		h.log.Error("failed to create user", zap.Error(err))
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(user); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// Get handles retrieving a single user by ID
// @Summary Get a user by ID
// @Description Get a user by its ID
// @Tags users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} model.User
// @Failure 400 {string} string "Invalid user ID"
// @Failure 404 {string} string "User not found"
// @Failure 500 {string} string "Failed to get user"
// @Router /users/{id} [get]
func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	user, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if err == constant.ErrNotFound {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed to get user", zap.Error(err))
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(user); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// List handles retrieving a paginated list of users
// @Summary List users
// @Description Get a paginated list of users
// @Tags users
// @Produce json
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Success 200 {array} model.User
// @Failure 500 {string} string "Failed to list users"
// @Router /users [get]
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
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

	users, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		h.log.Error("failed to list users", zap.Error(err))
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(users); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// Update handles updating an existing user
// @Summary Update a user
// @Description Update an existing user
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param user body model.User true "User object"
// @Success 200 {object} model.User
// @Failure 400 {string} string "Invalid user ID or payload"
// @Failure 404 {string} string "User not found"
// @Failure 500 {string} string "Failed to update user"
// @Router /users/{id} [put]
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	userID, err := uuid.Parse(idStr)
	if err != nil {
		h.log.Error("invalid user ID format", zap.Error(err), zap.String("id", idStr))
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var req model.User
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed to decode request body", zap.Error(err))
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	req.ID = userID

	updatedUser, err := h.svc.Update(r.Context(), &req)
	if err != nil {
		if err == constant.ErrNotFound {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed to update user", zap.Error(err))
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(updatedUser); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// Delete handles deleting a user by ID
// @Summary Delete a user
// @Description Delete a user by its ID
// @Tags users
// @Param id path string true "User ID"
// @Success 204 {string} string "No Content"
// @Failure 400 {string} string "Invalid user ID"
// @Failure 404 {string} string "User not found"
// @Failure 500 {string} string "Failed to delete user"
// @Router /users/{id} [delete]
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		if err == constant.ErrNotFound {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		h.log.Error("failed to delete user", zap.Error(err))
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RegisterRoutes registers all user-related routes to the router
func (h *UserHandler) RegisterRoutes(r chi.Router) {
	r.Route("/users", func(r chi.Router) {
		r.Post("/", h.Create)
		r.Get("/", h.List)
		r.Get("/{id}", h.Get)
		r.Put("/{id}", h.Update)
		r.Delete("/{id}", h.Delete)
	})
}
