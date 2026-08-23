package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/tyha2404/nexo-app-api/internal/logger"
	"go.uber.org/zap"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)
	return n, err
}

// Flush implements http.Flusher interface for SSE / streaming responses
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker interface for WebSockets / connection hijacking
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}

// Unwrap returns the underlying ResponseWriter (Go 1.20+ ResponseController compatibility)
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// LoggingMiddleware logs request information with colorful terminal badges and structured logs
func LoggingMiddleware(logg *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Call the next handler
			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			// Visual fine-grained colorful console format:
			// [HTTP] 15:04:05.123 | [ 200 OK ] |  12.45ms |   1.2 KB | 127.0.0.1:54321 | [ GET ] /api/v1/transactions?page=1
			fmt.Printf("%s[HTTP]%s %s%s%s | %s | %s | %s | %-24s | %s %s\n",
				logger.Cyan, logger.Reset,
				logger.HiBlack, time.Now().Format("15:04:05.000"), logger.Reset,
				logger.ColorStatus(rw.statusCode),
				logger.ColorDuration(duration),
				logger.ColorBytes(rw.bytesWritten),
				logger.ColorIP(r.RemoteAddr),
				logger.ColorMethod(r.Method),
				logger.ColorURL(r.URL.Path, r.URL.RawQuery),
			)

			// Record server errors to Zap structured log
			if rw.statusCode >= 500 {
				logg.Error("HTTP Server Error",
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.String("query", r.URL.RawQuery),
					zap.Int("status", rw.statusCode),
					zap.Duration("duration", duration),
					zap.String("remote_addr", r.RemoteAddr),
					zap.Int64("bytes", rw.bytesWritten),
				)
			}
		})
	}
}
