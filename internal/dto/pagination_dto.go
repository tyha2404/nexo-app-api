package dto

// PaginationRequest represents common pagination parameters for list endpoints
type PaginationRequest struct {
	Page     int    `json:"page" example:"1" validate:"required,min=1"`
	PageSize int    `json:"pageSize" example:"20" validate:"required,min=1,max=100"`
	SortBy   string `json:"sortBy" example:"createdAt" validate:"omitempty"`
	SortDir  string `json:"sortDir" example:"desc" validate:"omitempty,oneof=asc desc"`
}

// PaginationResponse represents the pagination metadata in list responses
type PaginationResponse struct {
	Page       int `json:"page" example:"1"`
	PageSize   int `json:"pageSize" example:"20"`
	TotalPages int `json:"totalPages" example:"5"`
	TotalCount int `json:"totalCount" example:"100"`
}

// ListTransactionRequest represents the request parameters for listing transactions
type ListTransactionRequest struct {
	PaginationRequest
	Type       *string `json:"type,omitempty" example:"EXPENSE" validate:"omitempty,oneof=INCOME EXPENSE"`
	CategoryID *string `json:"categoryId,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	StartDate  *string `json:"startDate,omitempty" example:"2024-01-01"`
	EndDate    *string `json:"endDate,omitempty" example:"2024-12-31"`
	MinAmount  *string `json:"minAmount,omitempty" example:"0"`
	MaxAmount  *string `json:"maxAmount,omitempty" example:"1000"`
}

// ListTransactionResponse represents the response for listing transactions
type ListTransactionResponse struct {
	Data       []TransactionResponse `json:"data"`
	Pagination PaginationResponse    `json:"pagination"`
}

// ListCategoryRequest represents the request parameters for listing categories
type ListCategoryRequest struct {
	PaginationRequest
	Name *string `json:"name,omitempty" example:"Food"`
}

// ListCategoryResponse represents the response for listing categories
type ListCategoryResponse struct {
	Data       []CategoryResponse `json:"data"`
	Pagination PaginationResponse `json:"pagination"`
}

// ListCostRequest represents the request parameters for listing costs
type ListCostRequest struct {
	PaginationRequest
	Currency   *string `json:"currency,omitempty" example:"USD"`
	CategoryID *string `json:"categoryId,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	StartDate  *string `json:"startDate,omitempty" example:"2024-01-01"`
	EndDate    *string `json:"endDate,omitempty" example:"2024-12-31"`
	MinAmount  *string `json:"minAmount,omitempty" example:"0"`
	MaxAmount  *string `json:"maxAmount,omitempty" example:"1000"`
}

// ListCostResponse represents the response for listing costs
type ListCostResponse struct {
	Data       []CostResponse     `json:"data"`
	Pagination PaginationResponse `json:"pagination"`
}
