package constant

// Context keys for storing values in request context
type contextKey string

const (
	// UserContextKey is the key for storing the authenticated model.User in
	// the request context
	UserContextKey contextKey = "user"
)
