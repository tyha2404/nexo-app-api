package dto

type LoginRequest struct {
	Email    string `json:"email" example:"john@example.com" validate:"required,email,max=255"`
	Password string `json:"password" example:"password123" validate:"required,min=8,max=128"`
}

type LoginResponse struct {
	User  UserResponse `json:"user"`
	Token string       `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

type RegisterRequest struct {
	Username string `json:"username" example:"johndoe" validate:"required,min=3,max=50,alphanum"`
	Email    string `json:"email" example:"john@example.com" validate:"required,email,max=255"`
	Password string `json:"password" example:"password123" validate:"required,min=8,max=128"`
}

type UserResponse struct {
	ID        string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Username  string `json:"username" example:"johndoe"`
	Email     string `json:"email" example:"john@example.com"`
	CreatedAt string `json:"createdAt" example:"2024-01-01T00:00:00Z"`
	UpdatedAt string `json:"updatedAt" example:"2024-01-01T00:00:00Z"`
}

type UpdateUserRequest struct {
	Username *string `json:"username,omitempty" example:"newusername" validate:"omitempty,min=3,max=50,alphanum"`
	Email    *string `json:"email,omitempty" example:"newemail@example.com" validate:"omitempty,email,max=255"`
}
