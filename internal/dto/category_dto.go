package dto

type CreateCategoryRequest struct {
	Name        string  `json:"name" example:"Food" validate:"required,min=1,max=50"`
	Description *string `json:"description" example:"Food and groceries" validate:"omitempty,max=500"`
}

type UpdateCategoryRequest struct {
	Name        *string `json:"name,omitempty" example:"Food & Dining" validate:"omitempty,min=1,max=50"`
	Description *string `json:"description,omitempty" example:"Updated description" validate:"omitempty,max=500"`
}

type CategoryResponse struct {
	ID          string  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	UserID      string  `json:"userId" example:"550e8400-e29b-41d4-a716-446655440001"`
	Name        string  `json:"name" example:"Food"`
	Description *string `json:"description,omitempty" example:"Food and groceries"`
	CreatedAt   string  `json:"createdAt" example:"2024-01-01T00:00:00Z"`
	UpdatedAt   string  `json:"updatedAt" example:"2024-01-01T00:00:00Z"`
	DeletedAt   *string `json:"deletedAt,omitempty" example:"2024-01-01T00:00:00Z"`
}
