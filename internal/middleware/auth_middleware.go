package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/util"
)

// AuthMiddleware is a middleware that verifies JWT tokens
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			render.Status(r, http.StatusUnauthorized)
			render.JSON(w, r, map[string]string{"error": "Authorization header is required"})
			return
		}

		// Extract the token from the header
		// Format: "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			render.Status(r, http.StatusUnauthorized)
			render.JSON(w, r, map[string]string{"error": "Authorization header format must be 'Bearer {token}'"})
			return
		}

		tokenString := parts[1]

		// Validate the token
		claims, err := util.ValidateToken(tokenString)
		if err != nil {
			render.Status(r, http.StatusUnauthorized)
			render.JSON(w, r, map[string]string{"error": "Invalid or expired token"})
			return
		}

		// Add user information to the request context
		// Create a new user struct without the password
		user := model.User{
			ID:       claims.ID,
			Email:    claims.Email,
			Username: claims.Username,
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
		// In a real application, you would check the user's role from the token or database
		// For now, this is a placeholder that always allows access
		// Replace this with your actual admin check logic

		// Example:
		// userID := r.Context().Value(constant.UserIDKey).(string)
		// if !isUserAdmin(userID) {
		//     render.Status(r, http.StatusForbidden)
		//     render.JSON(w, r, map[string]string{"error": "Admin access required"})
		//     return
		// }

		next.ServeHTTP(w, r)
	})
}
