package middleware

import "net/http"

// DefaultMaxBodyBytes caps JSON request bodies API-wide. No endpoint takes
// uploads, so 1 MB is far above any legitimate payload.
const DefaultMaxBodyBytes int64 = 1 << 20

// MaxBodyBytes limits how much of the request body handlers can read. Reads
// past the limit fail, so the JSON decoder returns an error (400) instead of
// buffering an arbitrarily large body in memory.
func MaxBodyBytes(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}
