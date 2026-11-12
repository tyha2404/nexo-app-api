package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"

	"github.com/tyha2404/nexo-app-api/internal/response"
	"go.uber.org/zap"
)

// RecoverPanicMessage is the message of the JSON 500 sent after a panic.
const RecoverPanicMessage = "Có lỗi hệ thống, vui lòng thử lại sau"

// recoverWriter remembers whether the response has started, so a panic after
// the first byte (e.g. mid SSE stream) does not append a JSON body to it.
type recoverWriter struct {
	http.ResponseWriter
	started bool
}

func (rw *recoverWriter) WriteHeader(code int) {
	rw.started = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *recoverWriter) Write(b []byte) (int, error) {
	rw.started = true
	return rw.ResponseWriter.Write(b)
}

// Flush implements http.Flusher for SSE / streaming responses.
func (rw *recoverWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		rw.started = true
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker.
func (rw *recoverWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		rw.started = true
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}

// Unwrap returns the underlying ResponseWriter (http.ResponseController).
func (rw *recoverWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Recoverer turns a panic in a handler into a logged error (with stack) and a
// JSON 500 in the standard error envelope. Register it as the first
// middleware so it covers every other one. http.ErrAbortHandler is re-raised
// (net/http uses it to abort a response on purpose).
func Recoverer(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &recoverWriter{ResponseWriter: w}
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				logger.Error("panic recovered in HTTP handler",
					zap.Any("panic", rec),
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.Bool("response_started", rw.started),
					zap.Stack("stack"),
				)
				if rw.started {
					return // too late for a status code; the connection is finished as is
				}
				response.SendError(w, http.StatusInternalServerError, RecoverPanicMessage, nil)
			}()
			next.ServeHTTP(rw, r)
		})
	}
}
