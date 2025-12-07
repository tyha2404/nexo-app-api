package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
)

// GetUserFromContext extracts the authenticated user from the request context
func GetUserFromContext(r *http.Request) (*model.User, error) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		return nil, constant.ErrUnauthorized
	}
	return &user, nil
}

// ParseUUIDFromPath extracts a UUID from the URL path parameters
func ParseUUIDFromPath(r *http.Request, param string) (uuid.UUID, error) {
	idStr := chi.URLParam(r, param)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, constant.ErrInvalidInput
	}
	return id, nil
}

// ParseQueryInt extracts an integer from query parameters with a default value
func ParseQueryInt(r *http.Request, key string, defaultValue int) int {
	valueStr := r.URL.Query().Get(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil || value < 0 {
		return defaultValue
	}

	return value
}

// ParseQueryIntWithValidation extracts an integer from query parameters with validation
func ParseQueryIntWithValidation(r *http.Request, key string, defaultValue int, minValue int) int {
	valueStr := r.URL.Query().Get(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil || value < minValue {
		return defaultValue
	}

	return value
}

// DecodeJSONBody decodes JSON request body into the provided struct
func DecodeJSONBody(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// ValidateRequired checks if required fields are present in the request
func ValidateRequired(fields map[string]string) error {
	for _, value := range fields {
		if value == "" {
			return constant.ErrInvalidInput
		}
	}
	return nil
}

// BuildFilterMap creates a filter map from query parameters
func BuildFilterMap(r *http.Request, filterKeys []string) map[string]interface{} {
	filters := make(map[string]interface{})
	query := r.URL.Query()

	for _, key := range filterKeys {
		if value := query.Get(key); value != "" {
			filters[key] = value
		}
	}

	return filters
}

// SetCommonHeaders sets common HTTP headers for responses
func SetCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
}

// GetClientIP extracts the client IP from the request
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// IsAuthenticated checks if the user is authenticated
func IsAuthenticated(r *http.Request) bool {
	_, err := GetUserFromContext(r)
	return err == nil
}

// RequireAuthentication middleware helper
func RequireAuthentication(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
