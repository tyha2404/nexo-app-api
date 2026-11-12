package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"gorm.io/gorm"
)

const (
	// DefaultPageLimit is the page size used when the client sends no (or an invalid) limit.
	DefaultPageLimit = 10
	// MaxPageLimit caps the page size a client may request. nexo-web asks for at most 50.
	MaxPageLimit = 100
)

// ParsePagination reads `page` and `limit` from the query string.
// page defaults to 1 (minimum 1); limit defaults to DefaultPageLimit and is
// clamped to [1, MaxPageLimit]. Non-numeric values fall back to the defaults.
func ParsePagination(r *http.Request) (page, limit int) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err = strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 {
		limit = DefaultPageLimit
	}
	if limit > MaxPageLimit {
		limit = MaxPageLimit
	}

	return page, limit
}

// QueryInt returns the integer query parameter key, 0 when it is missing or
// not an integer. Services apply their own defaults to 0 / out-of-range values.
func QueryInt(r *http.Request, key string) int {
	v, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return 0
	}
	return v
}

// IsNotFoundError reports whether err means the requested record does not
// exist (or does not belong to the caller, which is reported the same way).
func IsNotFoundError(err error) bool {
	return errors.Is(err, constant.ErrNotFound) || errors.Is(err, gorm.ErrRecordNotFound)
}

// GetUserFromContext extracts the authenticated user from the request context
func GetUserFromContext(r *http.Request) (*model.User, error) {
	user, ok := r.Context().Value(constant.UserContextKey).(model.User)
	if !ok || user.ID == uuid.Nil {
		return nil, constant.ErrUnauthorized
	}
	return &user, nil
}

// pathParamError reports an unparsable URL path parameter. It matches
// constant.ErrInvalidInput (errors.Is) while keeping the parser's own text, so
// the `error` detail of a 400 response is the same as before handlers shared
// ParseUUIDFromPath (e.g. "invalid UUID length: 3").
type pathParamError struct{ err error }

func (e pathParamError) Error() string   { return e.err.Error() }
func (e pathParamError) Unwrap() []error { return []error{constant.ErrInvalidInput, e.err} }

// ParseUUIDFromPath extracts a UUID from the URL path parameters. The error
// matches constant.ErrInvalidInput.
func ParseUUIDFromPath(r *http.Request, param string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		return uuid.Nil, pathParamError{err: err}
	}
	return id, nil
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
