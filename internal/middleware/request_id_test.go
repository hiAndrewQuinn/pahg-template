package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestIDMiddleware_GeneratesNewID(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request ID is in context
		reqID := GetRequestID(r.Context())
		assert.NotEmpty(t, reqID)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify response header is set
	respID := rec.Header().Get("X-Request-ID")
	assert.NotEmpty(t, respID)
	// UUID format: 8-4-4-4-12 characters
	assert.Len(t, respID, 36)
}

func TestRequestIDMiddleware_PreservesClientID(t *testing.T) {
	clientID := "custom-request-id-12345"

	var contextID string
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contextID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", clientID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify the client's ID is preserved
	assert.Equal(t, clientID, rec.Header().Get("X-Request-ID"))
	assert.Equal(t, clientID, contextID)
}

func TestRequestIDMiddleware_UniqueIDs(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		id := rec.Header().Get("X-Request-ID")
		assert.False(t, ids[id], "request ID should be unique")
		ids[id] = true
	}
}

func TestRequestIDMiddleware_PassesRequestToNext(t *testing.T) {
	called := false
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/test", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called, "next handler should be called")
}

func TestGetRequestID_WithValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), RequestIDKey, "test-id-123")
	id := GetRequestID(ctx)
	assert.Equal(t, "test-id-123", id)
}

func TestGetRequestID_WithoutValue(t *testing.T) {
	ctx := context.Background()
	id := GetRequestID(ctx)
	assert.Empty(t, id)
}

func TestGetRequestID_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), RequestIDKey, 12345)
	id := GetRequestID(ctx)
	assert.Empty(t, id)
}

func TestRequestIDKey_Type(t *testing.T) {
	// Verify the key type is correct
	var key contextKey = RequestIDKey
	assert.Equal(t, contextKey("request_id"), key)
}

func TestRequestIDMiddleware_EmptyClientID(t *testing.T) {
	// Empty X-Request-ID header should be treated as no header
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should generate a new UUID (36 chars)
	respID := rec.Header().Get("X-Request-ID")
	require.NotEmpty(t, respID)
	assert.Len(t, respID, 36)
}
