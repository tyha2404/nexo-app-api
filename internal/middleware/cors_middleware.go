package middleware

import (
	"net/http"
	"strings"
)

const (
	corsAllowMethods = "GET, POST, PUT, DELETE, OPTIONS, PATCH"
	corsAllowHeaders = "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization"
)

// NewCORSMiddleware returns a CORS middleware that only reflects origins in
// allowedOrigins (exact match, scheme + host + port, no trailing slash). A
// literal "*" entry allows any origin; use it only for local experiments.
//
// Requests without an Origin header (same-origin calls through the nexo-web
// proxy, curl, server-to-server) are passed through untouched.
func NewCORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowAll := false
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		if o == "*" {
			allowAll = true
			continue
		}
		if o != "" {
			allowed[o] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				w.Header().Add("Vary", "Origin")
				_, ok := allowed[origin]
				if ok || allowAll {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Methods", corsAllowMethods)
					w.Header().Set("Access-Control-Allow-Headers", corsAllowHeaders)
					w.Header().Set("Access-Control-Max-Age", "600")
				}
			}

			// Answer preflights here. Without the allow headers above the
			// browser rejects a disallowed origin's actual request.
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
