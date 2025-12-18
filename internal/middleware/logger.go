package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware logs every HTTP request with structured logging
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Get client IP
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = forwarded
		}

		// Get request ID from context
		reqID := GetRequestID(r.Context())

		// Log request start
		slog.Info("request_started",
			"request_id", reqID,
			"method", r.Method,
			"path", r.URL.Path,
			"ip", ip,
			"user_agent", r.UserAgent(),
		)

		// Wrap response writer to capture status
		wrapped := newResponseWriter(w)

		// Process request
		next.ServeHTTP(wrapped, r)

		// Log request completion
		duration := time.Since(start)
		slog.Info("request_completed",
			"request_id", reqID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", float64(duration.Microseconds())/1000.0,
		)
	})
}
