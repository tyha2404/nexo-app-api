package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/response"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

// sendAuthError writes the standard error envelope
// {status, success: false, message, error} with msg as both message and
// error (error carried msg alone before; clients only rely on the status).
func sendAuthError(w http.ResponseWriter, status int, msg string) {
	response.SendError(w, status, msg, errors.New(msg))
}

// AuthMiddleware is a middleware that verifies JWT tokens
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			sendAuthError(w, http.StatusUnauthorized, "Authorization header is required")
			return
		}

		// Extract the token from the header
		// Format: "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			sendAuthError(w, http.StatusUnauthorized, "Authorization header format must be 'Bearer {token}'")
			return
		}

		tokenString := parts[1]

		// Validate the token
		claims, err := util.ValidateToken(tokenString)
		if err != nil {
			sendAuthError(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		// Add user information to the request context
		// Create a new user struct without the password
		user := model.User{
			ID:       claims.ID,
			Email:    claims.Email,
			Username: claims.Username,
			Role:     claims.Role,
		}

		// Store the user in context
		ctx := r.Context()
		ctx = context.WithValue(ctx, constant.UserContextKey, user)

		// Call the next handler with the new context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AdminOnly is a middleware that ensures the user has admin role
func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(constant.UserContextKey).(model.User)
		if !ok {
			sendAuthError(w, http.StatusUnauthorized, "User not found in context")
			return
		}

		if user.Role != model.RoleAdmin {
			sendAuthError(w, http.StatusForbidden, "Admin access required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RoleLookup returns a user's current role from the database.
type RoleLookup func(ctx context.Context, userID uuid.UUID) (string, error)

// RequireCurrentRole re-checks the caller's role in the database. The role in
// a token is a snapshot from login, so without this a demoted or deleted
// admin keeps admin access until the token expires. Use it after AdminOnly.
func RequireCurrentRole(role string, lookup RoleLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value(constant.UserContextKey).(model.User)
			if !ok {
				sendAuthError(w, http.StatusUnauthorized, "User not found in context")
				return
			}
			current, err := lookup(r.Context(), user.ID)
			if err != nil || current != role {
				sendAuthError(w, http.StatusForbidden, "Admin access required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
