package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/response"
	"go.uber.org/zap"
)

// ErrorHandler provides centralized error handling for all handlers
type ErrorHandler struct {
	logger *zap.Logger
}

// NewErrorHandler creates a new error handler instance
func NewErrorHandler(logger *zap.Logger) *ErrorHandler {
	return &ErrorHandler{
		logger: logger,
	}
}

// HandleError processes errors and sends appropriate HTTP responses
func (e *ErrorHandler) HandleError(w http.ResponseWriter, err error, operation string) {
	var statusCode int
	var message string

	// Determine status code and message based on error type
	switch {
	case errors.Is(err, constant.ErrNotFound):
		statusCode = http.StatusNotFound
		message = "Resource not found"
	case errors.Is(err, constant.ErrUnauthorized):
		statusCode = http.StatusUnauthorized
		message = "Unauthorized access"
	case errors.Is(err, constant.ErrInvalidCredentials):
		statusCode = http.StatusUnauthorized
		message = "Invalid email or password"
	case errors.Is(err, constant.ErrInvalidInput):
		statusCode = http.StatusBadRequest
		message = "Invalid input provided"
	case errors.Is(err, constant.ErrEmailAlreadyExists):
		statusCode = http.StatusConflict
		message = "Email already exists"
	case errors.Is(err, constant.ErrUsernameTaken):
		statusCode = http.StatusConflict
		message = "Username already taken"
	default:
		statusCode = http.StatusInternalServerError
		message = "Internal server error"
	}

	// Log the error with context
	e.logger.Error("operation failed",
		zap.String("operation", operation),
		zap.Error(err),
		zap.Int("status_code", statusCode),
	)

	// Send error response
	response.SendError(w, statusCode, message, err)
}

// HandleValidationError handles validation errors specifically
func (e *ErrorHandler) HandleValidationError(w http.ResponseWriter, err error, operation string) {
	e.logger.Warn("validation failed",
		zap.String("operation", operation),
		zap.Error(err),
	)

	response.SendError(w, http.StatusBadRequest, "Validation failed", err)
}

// HandleDecodeError handles JSON decoding errors
func (e *ErrorHandler) HandleDecodeError(w http.ResponseWriter, err error, operation string) {
	e.logger.Warn("failed to decode request body",
		zap.String("operation", operation),
		zap.Error(err),
	)

	response.SendError(w, http.StatusBadRequest, "Invalid request payload", err)
}

// HandleSuccess sends successful JSON responses
func (e *ErrorHandler) HandleSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(response.BaseResponse[interface{}]{
		Status:  status,
		Success: true,
		Data:    data,
	}); err != nil {
		e.logger.Error("failed to encode response", zap.Error(err))
	}
}

// HandleSuccessWithMessage sends successful JSON responses with custom message
func (e *ErrorHandler) HandleSuccessWithMessage(w http.ResponseWriter, status int, data interface{}, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(response.BaseResponse[interface{}]{
		Status:  status,
		Success: true,
		Message: message,
		Data:    data,
	}); err != nil {
		e.logger.Error("failed to encode response", zap.Error(err))
	}
}

// HandlePaginatedSuccess sends paginated JSON responses
func (e *ErrorHandler) HandlePaginatedSuccess(w http.ResponseWriter, status int, items interface{}, total int, page int, limit int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// Create a generic pagination response
	responseData := map[string]interface{}{
		"status":  status,
		"success": true,
		"items":   items,
		"total":   total,
		"page":    page,
		"limit":   limit,
	}

	if err := json.NewEncoder(w).Encode(responseData); err != nil {
		e.logger.Error("failed to encode paginated response", zap.Error(err))
	}
}
