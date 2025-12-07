package handler

import (
	"net/http"

	"github.com/tyha2404/nexo-app-api/internal/dto"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/service"
	"github.com/tyha2404/nexo-app-api/internal/util"
	"go.uber.org/zap"
)

type AuthHandler struct {
	svc          service.AuthService
	log          *zap.Logger
	errorHandler *ErrorHandler
	validator    *Validator
}

func NewAuthHandler(svc service.AuthService, log *zap.Logger) *AuthHandler {
	return &AuthHandler{
		svc:          svc,
		log:          log,
		errorHandler: NewErrorHandler(log),
		validator:    NewValidator(),
	}
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
	if err := h.validator.ValidateRequest(r, &req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "login")
		return
	}

	// Authenticate user
	user, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		h.errorHandler.HandleError(w, err, "login")
		return
	}

	// Generate JWT token
	token, err := util.GenerateToken(user)
	if err != nil {
		h.errorHandler.HandleError(w, err, "login_token_generation")
		return
	}

	// Prepare and send loginResponse
	loginResponse := dto.LoginResponse{
		User:  user,
		Token: token,
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, loginResponse)
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
	if err := h.validator.ValidateRequest(r, &req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "register")
		return
	}

	user := &model.User{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
	}

	createdUser, err := h.svc.Register(r.Context(), user)
	if err != nil {
		h.errorHandler.HandleError(w, err, "register")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusCreated, *createdUser)
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
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, "whoami")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, user)
}
