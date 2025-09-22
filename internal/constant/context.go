package constant

// Context keys for storing values in request context
type contextKey string

const (
	// userContextKey is the key for storing user information in context
	UserContextKey contextKey = "user"
)
