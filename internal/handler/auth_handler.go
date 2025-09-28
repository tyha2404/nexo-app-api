package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"go.uber.org/zap"
)

type AuthHandler struct {
	svc service.AuthService
	log *zap.Logger
}

func NewAuthHandler(svc service.AuthService, log *zap.Logger) *AuthHandler {
	return &AuthHandler{svc: svc, log: log}
}

// Login handles logging in a user
// @Summary Login a user
// @Description Login a user with email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Login credentials"
// @Success 200 {object} dto.LoginResponse
// @Failure 400 {string} string "Invalid request payload"
// @Failure 401 {string} string "Invalid credentials"
// @Failure 500 {string} string "Failed to process login"
// @Router /auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed to decode request body", zap.Error(err))
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Authenticate user
	user, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if err == constant.ErrInvalidCredentials {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}
		h.log.Error("failed to process login", zap.Error(err))
		http.Error(w, "Failed to process login", http.StatusInternalServerError)
		return
	}

	// Generate JWT token
	token, err := util.GenerateToken(user)
	if err != nil {
		h.log.Error("failed to generate token", zap.Error(err))
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Prepare and send response
	response := dto.LoginResponse{
		User:  user,
		Token: token,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// Register handles the registration of a new user record
// @Summary Register a new user
// @Description Create a new user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RegisterRequest true "User registration data"
// @Success 201 {object} model.User
// @Failure 400 {string} string "Invalid request payload"
// @Failure 500 {string} string "Failed to create user"
// @Router /auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("failed to decode request body", zap.Error(err))
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	user := &model.User{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
	}

	createdUser, err := h.svc.Register(r.Context(), user)
	if err != nil {
		h.log.Error("failed to create user", zap.Error(err))
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(createdUser); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}

// WhoAmI returns the current user's information
// @Summary Get current user info
// @Description Get the currently authenticated user's information
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} model.User
// @Failure 401 {string} string "Unauthorized"
// @Router /auth/whoami [get]
func (h *AuthHandler) WhoAmI(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		h.log.Error("User ID not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(&user); err != nil {
		h.log.Error("failed to encode response", zap.Error(err))
	}
}
