package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"pahg-template/internal/config"
)

func TestIPAllowlistMiddleware_Disabled(t *testing.T) {
	cfg := &config.IPAllowlistConfig{
		Enabled: false,
		CIDRs:   []string{"127.0.0.0/8"},
	}

	called := false
	handler := IPAllowlistMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "8.8.8.8:12345" // Google DNS - not in allowlist
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called, "should call next handler when disabled")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPAllowlistMiddleware_AllowsLocalhost(t *testing.T) {
	cfg := &config.IPAllowlistConfig{
		Enabled: true,
		CIDRs:   []string{"127.0.0.0/8"},
	}

	called := false
	handler := IPAllowlistMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPAllowlistMiddleware_BlocksUnauthorized(t *testing.T) {
	cfg := &config.IPAllowlistConfig{
		Enabled: true,
		CIDRs:   []string{"127.0.0.0/8"},
	}

	called := false
	handler := IPAllowlistMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.False(t, called, "should not call next handler for blocked IP")
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestIPAllowlistMiddleware_AllowsPrivateRanges(t *testing.T) {
	cfg := &config.IPAllowlistConfig{
		Enabled: true,
		CIDRs: []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
		},
	}

	testCases := []struct {
		ip      string
		allowed bool
	}{
		{"10.0.0.1:12345", true},
		{"10.255.255.255:12345", true},
		{"172.16.0.1:12345", true},
		{"172.31.255.255:12345", true},
		{"192.168.1.1:12345", true},
		{"192.168.100.50:12345", true},
		{"8.8.8.8:12345", false},
		{"1.1.1.1:12345", false},
	}

	for _, tc := range testCases {
		t.Run(tc.ip, func(t *testing.T) {
			called := false
			handler := IPAllowlistMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tc.ip
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tc.allowed, called, "IP %s should be allowed=%v", tc.ip, tc.allowed)
		})
	}
}

func TestIPAllowlistMiddleware_IPv6(t *testing.T) {
	cfg := &config.IPAllowlistConfig{
		Enabled: true,
		CIDRs:   []string{"::1/128", "fc00::/7"},
	}

	testCases := []struct {
		ip      string
		allowed bool
	}{
		{"[::1]:12345", true},
		{"[fc00::1]:12345", true},
		{"[2001:4860:4860::8888]:12345", false}, // Google DNS IPv6
	}

	for _, tc := range testCases {
		t.Run(tc.ip, func(t *testing.T) {
			called := false
			handler := IPAllowlistMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tc.ip
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tc.allowed, called, "IP %s should be allowed=%v", tc.ip, tc.allowed)
		})
	}
}

func TestIPAllowlistMiddleware_XForwardedFor(t *testing.T) {
	cfg := &config.IPAllowlistConfig{
		Enabled: true,
		CIDRs:   []string{"10.0.0.0/8"},
	}

	called := false
	handler := IPAllowlistMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "8.8.8.8:12345"               // Would be blocked
	req.Header.Set("X-Forwarded-For", "10.0.0.50") // But X-Forwarded-For is allowed
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPAllowlistMiddleware_XForwardedForMultiple(t *testing.T) {
	cfg := &config.IPAllowlistConfig{
		Enabled: true,
		CIDRs:   []string{"10.0.0.0/8"},
	}

	called := false
	handler := IPAllowlistMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	// Multiple IPs - first one is the original client
	req.Header.Set("X-Forwarded-For", "10.0.0.50, 172.16.0.1, 8.8.8.8")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPAllowlistMiddleware_XRealIP(t *testing.T) {
	cfg := &config.IPAllowlistConfig{
		Enabled: true,
		CIDRs:   []string{"10.0.0.0/8"},
	}

	called := false
	handler := IPAllowlistMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	req.Header.Set("X-Real-IP", "10.0.0.50")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestIPAllowlistMiddleware_InvalidCIDR(t *testing.T) {
	cfg := &config.IPAllowlistConfig{
		Enabled: true,
		CIDRs:   []string{"invalid-cidr", "127.0.0.0/8"},
	}

	called := false
	handler := IPAllowlistMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Valid CIDR should still work
	assert.True(t, called)
}

func TestIPAllowlistMiddleware_InvalidClientIP(t *testing.T) {
	cfg := &config.IPAllowlistConfig{
		Enabled: true,
		CIDRs:   []string{"127.0.0.0/8"},
	}

	called := false
	handler := IPAllowlistMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "invalid-ip"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// Basic Auth Tests

func TestBasicAuthMiddleware_Disabled(t *testing.T) {
	cfg := &config.BasicAuthConfig{
		Enabled: false,
	}

	called := false
	handler := BasicAuthMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBasicAuthMiddleware_ValidCredentials(t *testing.T) {
	// Set environment variables for this test
	os.Setenv("BASIC_AUTH_USERNAME", "testuser")
	os.Setenv("BASIC_AUTH_PASSWORD", "testpass")
	defer func() {
		os.Unsetenv("BASIC_AUTH_USERNAME")
		os.Unsetenv("BASIC_AUTH_PASSWORD")
	}()

	cfg := &config.BasicAuthConfig{
		Enabled: true,
	}

	called := false
	handler := BasicAuthMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.SetBasicAuth("testuser", "testpass")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBasicAuthMiddleware_InvalidUsername(t *testing.T) {
	os.Setenv("BASIC_AUTH_USERNAME", "testuser")
	os.Setenv("BASIC_AUTH_PASSWORD", "testpass")
	defer func() {
		os.Unsetenv("BASIC_AUTH_USERNAME")
		os.Unsetenv("BASIC_AUTH_PASSWORD")
	}()

	cfg := &config.BasicAuthConfig{
		Enabled: true,
	}

	called := false
	handler := BasicAuthMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.SetBasicAuth("wronguser", "testpass")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), "Basic realm")
}

func TestBasicAuthMiddleware_InvalidPassword(t *testing.T) {
	os.Setenv("BASIC_AUTH_USERNAME", "testuser")
	os.Setenv("BASIC_AUTH_PASSWORD", "testpass")
	defer func() {
		os.Unsetenv("BASIC_AUTH_USERNAME")
		os.Unsetenv("BASIC_AUTH_PASSWORD")
	}()

	cfg := &config.BasicAuthConfig{
		Enabled: true,
	}

	called := false
	handler := BasicAuthMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.SetBasicAuth("testuser", "wrongpass")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestBasicAuthMiddleware_NoCredentialsProvided(t *testing.T) {
	os.Setenv("BASIC_AUTH_USERNAME", "testuser")
	os.Setenv("BASIC_AUTH_PASSWORD", "testpass")
	defer func() {
		os.Unsetenv("BASIC_AUTH_USERNAME")
		os.Unsetenv("BASIC_AUTH_PASSWORD")
	}()

	cfg := &config.BasicAuthConfig{
		Enabled: true,
	}

	called := false
	handler := BasicAuthMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	// No basic auth set
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), "Basic realm")
}

func TestBasicAuthMiddleware_NoEnvCredentials(t *testing.T) {
	// Ensure env vars are not set
	os.Unsetenv("BASIC_AUTH_USERNAME")
	os.Unsetenv("BASIC_AUTH_PASSWORD")

	cfg := &config.BasicAuthConfig{
		Enabled: true,
	}

	called := false
	handler := BasicAuthMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.SetBasicAuth("any", "credentials")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// getClientIP Tests

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	ip := getClientIP(req)

	assert.Equal(t, "192.168.1.100", ip)
}

func TestGetClientIP_RemoteAddrNoPort(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100"

	ip := getClientIP(req)

	assert.Equal(t, "192.168.1.100", ip)
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	req.Header.Set("X-Forwarded-For", "10.0.0.50")

	ip := getClientIP(req)

	assert.Equal(t, "10.0.0.50", ip)
}

func TestGetClientIP_XForwardedForMultiple(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	req.Header.Set("X-Forwarded-For", "10.0.0.50, 172.16.0.1, 8.8.8.8")

	ip := getClientIP(req)

	assert.Equal(t, "10.0.0.50", ip)
}

func TestGetClientIP_XForwardedForWithSpaces(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	req.Header.Set("X-Forwarded-For", "  10.0.0.50  ")

	ip := getClientIP(req)

	assert.Equal(t, "10.0.0.50", ip)
}

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	req.Header.Set("X-Real-IP", "10.0.0.50")

	ip := getClientIP(req)

	assert.Equal(t, "10.0.0.50", ip)
}

func TestGetClientIP_XForwardedForPrecedence(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	req.Header.Set("X-Forwarded-For", "10.0.0.50")
	req.Header.Set("X-Real-IP", "172.16.0.1")

	ip := getClientIP(req)

	// X-Forwarded-For should take precedence
	assert.Equal(t, "10.0.0.50", ip)
}

func TestGetClientIP_IPv6(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "[::1]:12345"

	ip := getClientIP(req)

	assert.Equal(t, "::1", ip)
}
