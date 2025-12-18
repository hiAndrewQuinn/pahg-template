package middleware

import (
	"crypto/subtle"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"

	"pahg-template/internal/config"
)

// IPAllowlistMiddleware restricts access to requests from allowed CIDR ranges
func IPAllowlistMiddleware(cfg *config.IPAllowlistConfig) func(http.Handler) http.Handler {
	// Parse CIDR ranges at startup
	var networks []*net.IPNet
	for _, cidr := range cfg.CIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			slog.Warn("invalid_cidr", "cidr", cidr, "error", err)
			continue
		}
		networks = append(networks, network)
	}

	slog.Info("ip_allowlist_configured", "cidr_count", len(networks), "enabled", cfg.Enabled)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := getClientIP(r)
			ip := net.ParseIP(clientIP)
			if ip == nil {
				slog.Warn("invalid_client_ip", "ip", clientIP)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Check if IP is in any allowed network
			allowed := false
			for _, network := range networks {
				if network.Contains(ip) {
					allowed = true
					break
				}
			}

			if !allowed {
				slog.Warn("ip_blocked", "ip", clientIP)
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// BasicAuthMiddleware requires HTTP Basic Authentication
// Credentials are read from BASIC_AUTH_USERNAME and BASIC_AUTH_PASSWORD env vars
func BasicAuthMiddleware(cfg *config.BasicAuthConfig) func(http.Handler) http.Handler {
	username := os.Getenv("BASIC_AUTH_USERNAME")
	password := os.Getenv("BASIC_AUTH_PASSWORD")

	if cfg.Enabled && (username == "" || password == "") {
		slog.Warn("basic_auth_enabled_but_no_credentials",
			"msg", "Basic auth is enabled but BASIC_AUTH_USERNAME or BASIC_AUTH_PASSWORD not set")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// If credentials not configured, deny all requests
			if username == "" || password == "" {
				http.Error(w, "Authentication not configured", http.StatusInternalServerError)
				return
			}

			// Get credentials from request
			reqUser, reqPass, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="CoinOps Dashboard"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Constant-time comparison to prevent timing attacks
			userMatch := subtle.ConstantTimeCompare([]byte(reqUser), []byte(username)) == 1
			passMatch := subtle.ConstantTimeCompare([]byte(reqPass), []byte(password)) == 1

			if !userMatch || !passMatch {
				slog.Warn("auth_failed", "username", reqUser)
				w.Header().Set("WWW-Authenticate", `Basic realm="CoinOps Dashboard"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP extracts the client IP from the request
// Checks X-Forwarded-For, X-Real-IP headers, then falls back to RemoteAddr
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (may contain multiple IPs)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (original client)
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr (remove port if present)
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr might not have a port
		return r.RemoteAddr
	}
	return host
}
