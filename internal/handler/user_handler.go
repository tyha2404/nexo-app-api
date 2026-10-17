package handler

import (
	"encoding/json"
	"errors"
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

// UserHandler serves the admin-only /users endpoints (AuthMiddleware +
// AdminOnly are applied in router/user_router.go).
type UserHandler struct {
	svc          service.UserService
	log          *zap.Logger
	errorHandler *ErrorHandler
	validator    *Validator
}

func NewUserHandler(svc service.UserService, log *zap.Logger) *UserHandler {
	return &UserHandler{svc: svc, log: log, errorHandler: NewErrorHandler(log), validator: NewValidator()}
}

// writeUserConflict maps duplicate username/email errors to 409. It returns
// false when err is not a conflict.
func (h *UserHandler) writeUserConflict(w http.ResponseWriter, err error, operation string) bool {
	switch {
	case errors.Is(err, constant.ErrEmailAlreadyExists):
		h.errorHandler.HandleError(w, err, http.StatusConflict, "Email already exists", operation)
		return true
	case errors.Is(err, constant.ErrUsernameTaken):
		h.errorHandler.HandleError(w, err, http.StatusConflict, "Username already taken", operation)
		return true
	}
	return false
}

// Create handles the creation of a new user record
// @Summary Create a new user (admin only)
// @Description Create a new user with the default "user" role. Requires an admin token.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user body dto.CreateUserRequest true "User object"
// @Success 201 {object} model.User
// @Failure 400 {object} response.ErrorResponse "Invalid request payload"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin access required"
// @Failure 409 {object} response.ErrorResponse "Email or username already exists"
// @Failure 500 {object} response.ErrorResponse "Failed to create user"
// @Router /users [post]
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateUserRequest
	if err := h.validator.ValidateRequest(r, &req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "create_user")
		return
	}

	user, err := h.svc.Create(r.Context(), req)
	if err != nil {
		if h.writeUserConflict(w, err, "create_user") {
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to create user", "create_user")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response.BaseResponse[model.User]{
		Status:  http.StatusCreated,
		Success: true,
		Data:    *user,
	}); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// Get handles retrieving a single user by ID
// @Summary Get a user by ID (admin only)
// @Description Get a user by its ID. Requires an admin token.
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} model.User
// @Failure 400 {object} response.ErrorResponse "Invalid user ID"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin access required"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 500 {object} response.ErrorResponse "Failed to get user"
// @Router /users/{id} [get]
func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid user ID", "get_user")
		return
	}

	user, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "User not found", "get_user")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to get user", "get_user")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response.BaseResponse[model.User]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *user,
	}); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// List handles retrieving a paginated list of users
// @Summary List users (admin only)
// @Description Get a paginated list of users. Requires an admin token.
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param limit query int false "Page limit (default 10, max 100)"
// @Param offset query int false "Offset (legacy, overrides page)"
// @Success 200 {array} model.User
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin access required"
// @Failure 500 {object} response.ErrorResponse "Failed to list users"
// @Router /users [get]
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	page, limit := ParsePagination(r)
	offset := (page - 1) * limit

	// Legacy clients may still send an explicit offset instead of page.
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
			page = offset/limit + 1
		}
	}

	users, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to list users", "list_users")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response.PaginationResponse[model.User]{
		Status:  http.StatusOK,
		Success: true,
		Items:   users,
		Total:   len(users),
		Page:    page,
		Limit:   limit,
	}); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// Update handles updating an existing user
// @Summary Update a user (admin only)
// @Description Update a user's username and/or email. Role and password cannot be changed here; any other fields in the body are ignored.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Param user body dto.UpdateUserRequest true "Fields to update"
// @Success 200 {object} model.User
// @Failure 400 {object} response.ErrorResponse "Invalid user ID or payload"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin access required"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 409 {object} response.ErrorResponse "Email or username already exists"
// @Failure 500 {object} response.ErrorResponse "Failed to update user"
// @Router /users/{id} [put]
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	userID, err := uuid.Parse(idStr)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid user ID", "update_user")
		return
	}

	var req dto.UpdateUserRequest
	if err := h.validator.ValidateRequest(r, &req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "update_user")
		return
	}
	if req.Username == nil && req.Email == nil {
		h.errorHandler.HandleError(w, constant.ErrInvalidInput, http.StatusBadRequest, "No fields to update", "update_user")
		return
	}

	updatedUser, err := h.svc.Update(r.Context(), userID, req)
	if err != nil {
		if h.writeUserConflict(w, err, "update_user") {
			return
		}
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "User not found", "update_user")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to update user", "update_user")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response.BaseResponse[model.User]{
		Status:  http.StatusOK,
		Success: true,
		Data:    *updatedUser,
	}); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// Delete handles deleting a user by ID
// @Summary Delete a user (admin only)
// @Description Delete a user by its ID. Requires an admin token.
// @Tags users
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} response.ErrorResponse "Invalid user ID"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 403 {object} response.ErrorResponse "Admin access required"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 500 {object} response.ErrorResponse "Failed to delete user"
// @Router /users/{id} [delete]
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Invalid user ID", "delete_user")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		if IsNotFoundError(err) {
			h.errorHandler.HandleError(w, err, http.StatusNotFound, "User not found", "delete_user")
			return
		}
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to delete user", "delete_user")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
