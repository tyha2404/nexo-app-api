package handler

import (
	"encoding/json"
	"net/http"

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
func (e *ErrorHandler) HandleError(w http.ResponseWriter, err error, statusCode int, message string, operation string) {
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

// paginatedEnvelope is response.PaginationResponse without the type
// parameter, so HandlePaginated can take any item slice. Keep the field order
// and tags identical to response.PaginationResponse: clients (and the
// envelope shape tests) depend on the exact JSON.
type paginatedEnvelope struct {
	Status  int         `json:"status"`
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Items   interface{} `json:"items"`
	Total   int         `json:"total"`
	Page    int         `json:"page"`
	Limit   int         `json:"limit"`
	Summary interface{} `json:"summary,omitempty"`
}

// HandlePaginated sends the standard paginated envelope
// {status, success, message, items, total, page, limit[, summary]}.
// Pass a nil summary to leave the field out. A typed nil (e.g. a nil
// *dto.TransactionSummaryDTO) is a non-nil interface and is sent as
// "summary": null, as before.
func (e *ErrorHandler) HandlePaginated(w http.ResponseWriter, status int, items interface{}, total, page, limit int, summary interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(paginatedEnvelope{
		Status:  status,
		Success: true,
		Items:   items,
		Total:   total,
		Page:    page,
		Limit:   limit,
		Summary: summary,
	}); err != nil {
		e.logger.Error("failed to encode paginated response", zap.Error(err))
	}
}

// decodeAndValidate decodes the JSON request body into dst and validates it.
// A decode error is answered with 400 "Invalid request payload" and a
// validation error with 400 "Validation failed" (friendly field messages in
// `error`). It reports whether the handler may continue.
func (e *ErrorHandler) decodeAndValidate(w http.ResponseWriter, r *http.Request, dst interface{}, operation string) bool {
	if err := DecodeJSONBody(r, dst); err != nil {
		e.HandleDecodeError(w, err, operation)
		return false
	}
	if err := requestValidator.ValidateStruct(dst); err != nil {
		e.HandleValidationError(w, err, operation)
		return false
	}
	return true
}

// HandlePaginatedSuccess sends the legacy paginated map (sorted keys, no
// message field) still used by the /costs list.
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
