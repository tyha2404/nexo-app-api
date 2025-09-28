package response

type BaseResponse[T any] struct {
	Status  int    `json:"status"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type PaginationResponse[T any] struct {
	Status  int    `json:"status"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Items   []T    `json:"items"`
	Total   int    `json:"total"`
	Page    int    `json:"page"`
	Limit   int    `json:"limit"`
}
