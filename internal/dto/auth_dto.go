package dto

import "github.com/tyha2404/nexo-app-api/internal/model"

type LoginRequest struct {
	Email    string `json:"email" example:"john@example.com" validate:"required,email"`
	Password string `json:"password" example:"password123" validate:"required,min=8"`
}

type LoginResponse struct {
	User  *model.User `json:"user"`
	Token string      `json:"token"`
}

type RegisterRequest struct {
	Username string `json:"username" example:"johndoe" validate:"required,min=3,max=50,alphanum"`
	Email    string `json:"email" example:"john@example.com" validate:"required,email"`
	Password string `json:"password" example:"password123" validate:"required,min=8"`
}
