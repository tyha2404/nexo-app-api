package dto

import (
	"time"

	"github.com/tyha2404/nexo-app-api/internal/model"
)

type LoginRequest struct {
	Username string `json:"username" example:"johndoe" validate:"max=255"`
	Email    string `json:"email" example:"john@example.com" validate:"max=255"`
	Password string `json:"password" example:"password123" validate:"required,min=6,max=128"`
}

type LoginResponse struct {
	User UserResponse `json:"user"`
	TokenResponse
}

// TokenResponse is a session's token pair. Token is the short-lived access
// token (Authorization: Bearer); RefreshToken renews it via POST
// /auth/refresh and is replaced on every refresh.
type TokenResponse struct {
	Token        string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	RefreshToken string `json:"refreshToken" example:"q0bIu5YBhM3n0uC8Yk3V3o7mVq9b2x1kQ9wJp6cTz4E"`
	// ExpiresIn is the access token lifetime in seconds.
	ExpiresIn int64 `json:"expiresIn" example:"900"`
}

// RefreshTokenRequest is the body of POST /auth/refresh and /auth/logout.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" example:"q0bIu5YBhM3n0uC8Yk3V3o7mVq9b2x1kQ9wJp6cTz4E" validate:"required,max=128"`
}

// ChangePasswordResponse carries a fresh session: changing the password
// signs out every other session, including the caller's old refresh token.
type ChangePasswordResponse struct {
	Message string `json:"message" example:"Password changed"`
	TokenResponse
}

type RegisterRequest struct {
	Username string `json:"username" example:"johndoe" validate:"required,min=3,max=50,username"`
	Email    string `json:"email" example:"john@example.com" validate:"required,email,max=255"`
	Password string `json:"password" example:"password123" validate:"required,min=8,max=128"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" example:"password123" validate:"required,max=128"`
	NewPassword     string `json:"newPassword" example:"newpassword456" validate:"required,min=8,max=128"`
}

type UserResponse struct {
	ID        string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Username  string `json:"username" example:"johndoe"`
	Email     string `json:"email" example:"john@example.com"`
	CreatedAt string `json:"createdAt" example:"2024-01-01T00:00:00Z"`
	UpdatedAt string `json:"updatedAt" example:"2024-01-01T00:00:00Z"`
}

// FromUser converts a model.User to dto.UserResponse
func (ur *UserResponse) FromUser(user *model.User) {
	if user == nil {
		return
	}
	ur.ID = user.ID.String()
	ur.Username = user.Username
	ur.Email = user.Email
	ur.CreatedAt = user.CreatedAt.Format(time.RFC3339)
	ur.UpdatedAt = user.UpdatedAt.Format(time.RFC3339)
}

// NewUserResponse creates a new UserResponse from a model.User
func NewUserResponse(user *model.User) UserResponse {
	var ur UserResponse
	ur.FromUser(user)
	return ur
}

// UpdateUserRequest is the only payload accepted by PUT /users/{id}.
// Role and password are deliberately absent: they can never be changed
// through this endpoint (unknown JSON fields such as "role" are ignored).
type UpdateUserRequest struct {
	Username *string `json:"username,omitempty" example:"newusername" validate:"omitempty,min=3,max=50,username"`
	Email    *string `json:"email,omitempty" example:"newemail@example.com" validate:"omitempty,email,max=255"`
}

// CreateUserRequest is the payload for the admin-only POST /users endpoint.
// New users always get the default "user" role.
type CreateUserRequest struct {
	Username string `json:"username" example:"johndoe" validate:"required,min=3,max=50,username"`
	Email    string `json:"email" example:"john@example.com" validate:"required,email,max=255"`
	Password string `json:"password" example:"password123" validate:"required,min=8,max=128"`
}
