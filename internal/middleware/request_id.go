package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

// RequestIDMiddleware adds a unique request ID to each request
// If the client sends an X-Request-ID header, it uses that value
// Otherwise, it generates a new UUID
// The request ID is added to both the response header and the request context
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if client sent a request ID
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			// Generate a new UUID
			id = uuid.New().String()
		}

		// Add to response headers
		w.Header().Set("X-Request-ID", id)

		// Add to context for use by other middleware and handlers
		ctx := context.WithValue(r.Context(), RequestIDKey, id)

		// Continue with the updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID extracts the request ID from the context
// Returns an empty string if not found
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}
