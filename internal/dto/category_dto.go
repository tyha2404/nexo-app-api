package dto

type CreateCategoryRequest struct {
	Name                    string  `json:"name" example:"Food" validate:"required,min=1,max=50"`
	Type                    string  `json:"type" example:"EXPENSE" validate:"required,oneof=INCOME EXPENSE INVESTMENT"`
	Description             *string `json:"description" example:"Food and groceries" validate:"omitempty,max=500"`
	ExcludeFromAverageDaily *bool   `json:"excludeFromAverageDaily,omitempty" example:"false"`
}

type UpdateCategoryRequest struct {
	Name                    *string `json:"name,omitempty" example:"Food & Dining" validate:"omitempty,min=1,max=50"`
	Type                    *string `json:"type,omitempty" example:"EXPENSE" validate:"omitempty,oneof=INCOME EXPENSE INVESTMENT"`
	Description             *string `json:"description,omitempty" example:"Updated description" validate:"omitempty,max=500"`
	ExcludeFromAverageDaily *bool   `json:"excludeFromAverageDaily,omitempty" example:"false"`
}
