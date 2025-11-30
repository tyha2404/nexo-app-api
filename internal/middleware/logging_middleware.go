package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// LoggingMiddleware logs request information including query strings
func LoggingMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Log the request details including query string
			logger.Sugar().Infow("Incoming request",
				"method", r.Method,
				"path", r.URL.Path,
				"query_string", r.URL.RawQuery,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)

			// Call the next handler
			next.ServeHTTP(w, r)

			// Log the completion time
			duration := time.Since(start)
			logger.Sugar().Infow("Request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"query_string", r.URL.RawQuery,
				"duration_ms", duration.Milliseconds(),
			)
		})
	}
}
