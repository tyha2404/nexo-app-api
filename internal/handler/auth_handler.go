package handler

import (
	"errors"
	"net/http"

	"github.com/tyha2404/nexo-app-api/internal/constant"
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

	identifier := req.Username
	if identifier == "" {
		identifier = req.Email
	}
	if identifier == "" {
		h.errorHandler.HandleError(w, errors.New("username or email is required"), http.StatusBadRequest, "Username or email is required", "login")
		return
	}

	// Authenticate user
	user, err := h.svc.Login(r.Context(), identifier, req.Password)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Invalid username/email or password", "login")
		return
	}

	tokens, err := h.newSession(r, user)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to generate session token", "login_token_generation")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, dto.LoginResponse{
		User:          dto.NewUserResponse(user),
		TokenResponse: tokens,
	})
}

// Refresh exchanges a refresh token for a new token pair
// @Summary Refresh the access token
// @Description Exchange a refresh token for a new access token and a new refresh token. The old refresh token stops working; presenting it again revokes the whole session.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RefreshTokenRequest true "Refresh token"
// @Success 200 {object} dto.TokenResponse
// @Failure 400 {string} string "Invalid request payload"
// @Failure 401 {string} string "Invalid, expired or revoked refresh token"
// @Failure 500 {string} string "Failed to refresh session"
// @Router /auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshTokenRequest
	if err := h.validator.ValidateRequest(r, &req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "refresh")
		return
	}

	user, refreshToken, err := h.svc.RefreshSession(r.Context(), req.RefreshToken, r.UserAgent())
	if errors.Is(err, constant.ErrInvalidRefreshToken) {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Invalid or expired refresh token", "refresh")
		return
	}
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to refresh session", "refresh")
		return
	}

	token, err := util.GenerateToken(user)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to generate session token", "refresh_token_generation")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, dto.TokenResponse{
		Token:        token,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(util.AccessTokenTTL().Seconds()),
	})
}

// Logout revokes the session of a refresh token
// @Summary Logout
// @Description Revoke the session the refresh token belongs to. Unknown or already revoked tokens also succeed. The access token stays valid until it expires.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RefreshTokenRequest true "Refresh token"
// @Success 200 {object} map[string]string
// @Failure 400 {string} string "Invalid request payload"
// @Failure 500 {string} string "Failed to logout"
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req dto.RefreshTokenRequest
	if err := h.validator.ValidateRequest(r, &req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "logout")
		return
	}

	if err := h.svc.EndSession(r.Context(), req.RefreshToken); err != nil {
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to logout", "logout")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, map[string]string{"message": "Logged out"})
}

// newSession issues an access token and the first refresh token of a new
// session for user.
func (h *AuthHandler) newSession(r *http.Request, user *model.User) (dto.TokenResponse, error) {
	token, err := util.GenerateToken(user)
	if err != nil {
		return dto.TokenResponse{}, err
	}
	refreshToken, err := h.svc.StartSession(r.Context(), user.ID, r.UserAgent())
	if err != nil {
		return dto.TokenResponse{}, err
	}
	return dto.TokenResponse{
		Token:        token,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(util.AccessTokenTTL().Seconds()),
	}, nil
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
		if errors.Is(err, constant.ErrEmailAlreadyExists) {
			h.errorHandler.HandleError(w, err, http.StatusConflict, "Email already exists", "register")
		} else if errors.Is(err, constant.ErrUsernameTaken) {
			h.errorHandler.HandleError(w, err, http.StatusConflict, "Username already taken", "register")
		} else {
			h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to create user", "register")
		}
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
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized access", "whoami")
		return
	}

	h.errorHandler.HandleSuccess(w, http.StatusOK, user)
}

// ChangePassword changes the current user's password
// @Summary Change password
// @Description Change the authenticated user's password after verifying the current one. Every session (refresh token) of the user is revoked and a new token pair is returned for the caller. Accounts that sign in through Supabase Auth must change it there.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.ChangePasswordRequest true "Current and new password"
// @Success 200 {object} dto.ChangePasswordResponse
// @Failure 400 {string} string "Invalid request payload, wrong current password or unchanged password"
// @Failure 401 {string} string "Unauthorized"
// @Failure 409 {string} string "Password is managed by Supabase Auth"
// @Failure 500 {string} string "Failed to change password"
// @Router /auth/change-password [post]
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r)
	if err != nil {
		h.errorHandler.HandleError(w, err, http.StatusUnauthorized, "Unauthorized access", "change_password")
		return
	}

	var req dto.ChangePasswordRequest
	if err := h.validator.ValidateRequest(r, &req); err != nil {
		h.errorHandler.HandleValidationError(w, err, "change_password")
		return
	}

	// A wrong current password is a 400, not a 401: clients treat 401 as an
	// expired session and log the user out.
	err = h.svc.ChangePassword(r.Context(), user.ID, req.CurrentPassword, req.NewPassword)
	switch {
	case err == nil:
		h.changePasswordSuccess(w, r, user)
	case errors.Is(err, constant.ErrInvalidCredentials):
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "Current password is incorrect", "change_password")
	case errors.Is(err, constant.ErrSamePassword):
		h.errorHandler.HandleError(w, err, http.StatusBadRequest, "New password must differ from the current one", "change_password")
	case errors.Is(err, constant.ErrPasswordManagedExternally):
		h.errorHandler.HandleError(w, err, http.StatusConflict, "Password is managed by Supabase Auth", "change_password")
	default:
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Failed to change password", "change_password")
	}
}

// changePasswordSuccess starts a new session for the caller, whose old
// refresh token was revoked along with every other session. The context user
// only has the access token's claims, which is all GenerateToken needs.
func (h *AuthHandler) changePasswordSuccess(w http.ResponseWriter, r *http.Request, user *model.User) {
	tokens, err := h.newSession(r, user)
	if err != nil {
		// The password did change; the client signs in again with it.
		h.errorHandler.HandleError(w, err, http.StatusInternalServerError, "Password changed but failed to start a new session", "change_password_session")
		return
	}
	h.errorHandler.HandleSuccess(w, http.StatusOK, dto.ChangePasswordResponse{
		Message:       "Password changed",
		TokenResponse: tokens,
	})
}
