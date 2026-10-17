package middleware

import (
	"encoding/json"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tyha2404/nexo-app-api/internal/constant"
	"github.com/tyha2404/nexo-app-api/internal/model"
	"github.com/tyha2404/nexo-app-api/internal/response"
)

// RateLimiter is an in-memory, per-key token bucket. Each key gets `burst`
// tokens that refill continuously at `limit` tokens per `window`. State is
// per process, which matches the single PM2 instance this API runs as.
type RateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*tokenBucket
	ratePerS  float64
	burst     float64
	keyFn     func(*http.Request) string
	now       func() time.Time
	lastSweep time.Time
	message   string
}

type tokenBucket struct {
	tokens   float64
	lastSeen time.Time
}

// rateLimitSweepInterval is how often idle (already full) buckets are dropped.
const rateLimitSweepInterval = 5 * time.Minute

// NewRateLimiter allows `limit` requests per `window` per key, with bursts of
// up to `limit`. keyFn picks the key (see ClientIPKey and UserOrIPKey).
func NewRateLimiter(limit int, window time.Duration, keyFn func(*http.Request) string, message string) *RateLimiter {
	if limit < 1 {
		limit = 1
	}
	if message == "" {
		message = "Too many requests, please try again later"
	}
	return &RateLimiter{
		buckets:  make(map[string]*tokenBucket),
		ratePerS: float64(limit) / window.Seconds(),
		burst:    float64(limit),
		keyFn:    keyFn,
		now:      time.Now,
		message:  message,
	}
}

// allow takes one token for key. When none is left it returns false and how
// long until the next token is available.
func (l *RateLimiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if now.Sub(l.lastSweep) > rateLimitSweepInterval {
		l.sweep(now)
	}

	b, ok := l.buckets[key]
	if !ok {
		b = &tokenBucket{tokens: l.burst, lastSeen: now}
		l.buckets[key] = b
	} else {
		elapsed := now.Sub(b.lastSeen).Seconds()
		b.tokens = math.Min(l.burst, b.tokens+elapsed*l.ratePerS)
		b.lastSeen = now
	}

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := time.Duration((1 - b.tokens) / l.ratePerS * float64(time.Second))
	return false, wait
}

// sweep drops buckets that have been idle long enough to be full again.
func (l *RateLimiter) sweep(now time.Time) {
	fullAfter := time.Duration(l.burst / l.ratePerS * float64(time.Second))
	for k, b := range l.buckets {
		if now.Sub(b.lastSeen) > fullAfter {
			delete(l.buckets, k)
		}
	}
	l.lastSweep = now
}

// Middleware rejects requests over the limit with 429 and a Retry-After header.
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, wait := l.allow(l.keyFn(r))
		if !ok {
			secs := int(math.Ceil(wait.Seconds()))
			if secs < 1 {
				secs = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(response.ErrorResponse{
				Status:  http.StatusTooManyRequests,
				Success: false,
				Message: l.message,
				Error:   "rate limit exceeded",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClientIP returns the caller's IP. X-Forwarded-For / X-Real-IP are trusted
// only when the TCP peer is a loopback address, i.e. the request came through
// nexo-web's server.cjs proxy on the same host (it sets X-Forwarded-For to
// the browser's socket address). Anyone else could spoof those headers.
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer == nil || !peer.IsLoopback() {
		return host
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// The right-most entry was added by our own proxy.
		parts := strings.Split(xff, ",")
		if ip := net.ParseIP(strings.TrimSpace(parts[len(parts)-1])); ip != nil {
			return ip.String()
		}
	}
	if xr := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); xr != nil {
		return xr.String()
	}
	return host
}

// ClientIPKey keys the limiter by client IP.
func ClientIPKey(r *http.Request) string {
	return "ip:" + ClientIP(r)
}

// UserOrIPKey keys the limiter by authenticated user ID (set by
// AuthMiddleware), falling back to client IP.
func UserOrIPKey(r *http.Request) string {
	if user, ok := r.Context().Value(constant.UserContextKey).(model.User); ok && user.ID != uuid.Nil {
		return "user:" + user.ID.String()
	}
	return ClientIPKey(r)
}
