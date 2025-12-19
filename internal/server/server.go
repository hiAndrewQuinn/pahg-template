package server

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"pahg-template/internal/coingecko"
	"pahg-template/internal/config"
	pmath "pahg-template/internal/math"
	"pahg-template/internal/middleware"
	"pahg-template/internal/notifications"
	"pahg-template/internal/session"
	"pahg-template/internal/version"
)

//go:embed templates/*.html templates/partials/*.html
var templatesFS embed.FS

//go:embed assets/*
var assetsFS embed.FS

// Server holds all dependencies for the HTTP server
type Server struct {
	cfg           *config.Config
	templates     *template.Template
	coinService   *coingecko.Service
	notifications *notifications.Store
	sessions      *session.Store
	mux           *http.ServeMux
	startTime     time.Time
}

// Template functions
var funcMap = template.FuncMap{
	"json": func(v interface{}) template.JS {
		b, _ := json.Marshal(v)
		return template.JS(b)
	},
}

// New creates a new server instance
func New(cfg *config.Config) (*Server, error) {
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templatesFS, "templates/*.html", "templates/partials/*.html")
	if err != nil {
		return nil, err
	}

	s := &Server{
		cfg:           cfg,
		templates:     tmpl,
		coinService:   coingecko.NewService(cfg.Coins),
		notifications: notifications.NewStore(),
		sessions:      session.NewStore(),
		mux:           http.NewServeMux(),
		startTime:     time.Now(),
	}

	s.setupRoutes()
	return s, nil
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	assetsSubFS, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		slog.Error("failed to create assets sub-filesystem", "error", err)
	} else {
		s.mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsSubFS))))
	}

	// Auth endpoints (no auth required)
	s.mux.HandleFunc("/login", s.handleLogin)
	s.mux.HandleFunc("/auth", s.handleAuth)
	s.mux.HandleFunc("/logout", s.handleLogout)

	// Pages
	s.mux.HandleFunc("/", s.handleIndex)

	// HTMX endpoints
	s.mux.HandleFunc("/ticker", s.handleTicker)
	s.mux.HandleFunc("/ticker/", s.handleTickerCoin) // Per-coin endpoint: /ticker/{coinId}
	s.mux.HandleFunc("/search", s.handleSearch)
	s.mux.HandleFunc("/generate-report", s.handleGenerateReport)
	s.mux.HandleFunc("/notifications", s.handleNotifications)

	// API endpoints
	s.mux.HandleFunc("/metadata", s.handleMetadata)
	s.mux.HandleFunc("/health", s.handleHealth)
}

// Handler returns the HTTP handler with middleware applied
func (s *Server) Handler() http.Handler {
	// Chain middleware from outermost to innermost:
	// 1. RequestID - adds unique ID to every request
	// 2. Logging - logs all requests with timing
	// 3. IPAllowlist - restricts by IP (if enabled)
	// 4. SessionAuth - requires authentication via session or Basic Auth (if enabled)
	// 5. mux - actual route handling
	var handler http.Handler = s.mux

	// Apply SessionAuth (innermost security layer)
	handler = s.sessionAuthMiddleware(handler)

	// Apply IP Allowlist (checked before auth)
	handler = middleware.IPAllowlistMiddleware(&s.cfg.Security.IPAllowlist)(handler)

	// Apply logging
	handler = middleware.LoggingMiddleware(handler)

	// Apply RequestID (outermost - runs first)
	handler = middleware.RequestIDMiddleware(handler)

	return handler
}

// PageData holds common data for page rendering
type PageData struct {
	Title             string
	NotificationCount int
	AvgRefreshMs      int
	Version           string
	Commit            string
	CommitDate        string
}

// TickerData holds data for the full ticker table (initial load)
type TickerData struct {
	Coins []CoinRowData
}

// CoinRowData holds data for a single coin row with its delay queue
type CoinRowData struct {
	ID          string
	DisplayName string
	Price       float64
	Change24h   float64
	Delays      []int // Queue of 10 delays in milliseconds
}

// ReportData holds data for report success template
type ReportData struct {
	Timestamp         string
	NotificationCount int
}

// NotificationsData holds data for notifications modal
type NotificationsData struct {
	Notifications []notifications.Notification
	Count         int
}

// generateDelayQueue creates a queue of 10 Poisson-distributed delays
func (s *Server) generateDelayQueue() []int {
	delays := make([]int, 10)
	for i := range delays {
		delays[i] = pmath.GetPoissonDelay(float64(s.cfg.Features.AvgRefreshIntervalMs))
	}
	return delays
}

// handleIndex renders the main dashboard page
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	versionInfo := version.Get()
	data := PageData{
		Title:             "Dashboard",
		NotificationCount: s.notifications.Count(),
		AvgRefreshMs:      s.cfg.Features.AvgRefreshIntervalMs,
		Version:           versionInfo.Version,
		Commit:            versionInfo.Commit,
		CommitDate:        versionInfo.CommitDate,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("template_error", "template", "layout.html", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleTicker returns the full crypto price table (initial load)
func (s *Server) handleTicker(w http.ResponseWriter, r *http.Request) {
	coins, err := s.coinService.GetPrices()
	if err != nil {
		http.Error(w, "Failed to fetch prices", http.StatusInternalServerError)
		return
	}

	coinData := make([]CoinRowData, len(coins))
	for i, c := range coins {
		coinData[i] = CoinRowData{
			ID:          c.ID,
			DisplayName: c.DisplayName,
			Price:       c.Price,
			Change24h:   c.Change24h,
			Delays:      s.generateDelayQueue(),
		}
	}

	data := TickerData{Coins: coinData}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "ticker.html", data); err != nil {
		slog.Error("template_error", "template", "ticker.html", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleTickerCoin returns a single coin row (async refresh per coin)
func (s *Server) handleTickerCoin(w http.ResponseWriter, r *http.Request) {
	// Extract coin ID from path: /ticker/{coinId}
	coinID := strings.TrimPrefix(r.URL.Path, "/ticker/")
	if coinID == "" {
		http.NotFound(w, r)
		return
	}

	coin, err := s.coinService.GetCoin(coinID)
	if err != nil {
		http.Error(w, "Coin not found", http.StatusNotFound)
		return
	}

	data := CoinRowData{
		ID:          coin.ID,
		DisplayName: coin.DisplayName,
		Price:       coin.Price,
		Change24h:   coin.Change24h,
		Delays:      s.generateDelayQueue(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "ticker_row.html", data); err != nil {
		slog.Error("template_error", "template", "ticker_row.html", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleSearch filters coins by search query
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("search")

	coins, err := s.coinService.SearchCoins(query)
	if err != nil {
		http.Error(w, "Failed to search", http.StatusInternalServerError)
		return
	}

	coinData := make([]CoinRowData, len(coins))
	for i, c := range coins {
		coinData[i] = CoinRowData{
			ID:          c.ID,
			DisplayName: c.DisplayName,
			Price:       c.Price,
			Change24h:   c.Change24h,
			Delays:      s.generateDelayQueue(),
		}
	}

	data := TickerData{Coins: coinData}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "ticker.html", data); err != nil {
		slog.Error("template_error", "template", "ticker.html", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleGenerateReport simulates a slow admin operation
func (s *Server) handleGenerateReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Simulate slow backend operation (3 seconds)
	time.Sleep(3 * time.Second)

	timestamp := time.Now().Format("20060102_150405")
	s.notifications.Add("Report Ready", "Compliance report "+timestamp+" generated successfully")

	data := ReportData{
		Timestamp:         timestamp,
		NotificationCount: s.notifications.Count(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "report-success.html", data); err != nil {
		slog.Error("template_error", "template", "report-success.html", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleNotifications returns the notifications list
func (s *Server) handleNotifications(w http.ResponseWriter, r *http.Request) {
	data := NotificationsData{
		Notifications: s.notifications.GetAll(),
		Count:         s.notifications.Count(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "notifications.html", data); err != nil {
		slog.Error("template_error", "template", "notifications.html", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// MetadataResponse holds the metadata endpoint response for stale tab detection
type MetadataResponse struct {
	Version     string                 `json:"version"`
	Commit      string                 `json:"commit"`
	CommitDate  string                 `json:"commit_date"`
	Environment string                 `json:"environment"`
	Features    map[string]interface{} `json:"features"`
}

// handleMetadata returns version, environment, and feature flags as JSON
// Used for stale tab detection - clients poll this to detect server updates
func (s *Server) handleMetadata(w http.ResponseWriter, r *http.Request) {
	versionInfo := version.Get()

	// Get environment from env var, default to "production"
	environment := getEnvironment()

	// Build features map from config
	features := map[string]interface{}{
		"avg_refresh_interval_ms": s.cfg.Features.AvgRefreshIntervalMs,
	}

	response := MetadataResponse{
		Version:     versionInfo.Version,
		Commit:      versionInfo.Commit,
		CommitDate:  versionInfo.CommitDate,
		Environment: environment,
		Features:    features,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("json_encode_error", "endpoint", "/metadata", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// getEnvironment returns the current environment name
func getEnvironment() string {
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = os.Getenv("ENV")
	}
	if env == "" {
		env = "production"
	}
	return env
}

// HealthResponse holds the health endpoint response for observability
type HealthResponse struct {
	Status     string  `json:"status"`
	Uptime     string  `json:"uptime"`
	Goroutines int     `json:"goroutines"`
	MemoryMB   float64 `json:"memory_mb"`
	GoVersion  string  `json:"go_version"`
}

// handleHealth returns runtime stats for monitoring and Kubernetes probes
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	uptime := time.Since(s.startTime)

	response := HealthResponse{
		Status:     "ok",
		Uptime:     uptime.Round(time.Second).String(),
		Goroutines: runtime.NumGoroutine(),
		MemoryMB:   float64(memStats.Alloc) / 1024 / 1024,
		GoVersion:  runtime.Version(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("json_encode_error", "endpoint", "/health", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleLogin serves the login page
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// If user is already authenticated, redirect to home
	if sess := s.getSessionFromRequest(r); sess != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "login.html", nil); err != nil {
		slog.Error("template_error", "template", "login.html", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AuthRequest holds login request data
type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse holds login response data
type AuthResponse struct {
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
	Redirect string `json:"redirect,omitempty"`
}

// handleAuth validates credentials and creates a session
func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Error:   "Invalid request",
		})
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	// Get credentials from environment
	envUsername := os.Getenv("BASIC_AUTH_USERNAME")
	envPasswordHash := os.Getenv("BASIC_AUTH_PASSWORD_HASH")

	// Validate credentials
	// First check username (simple equality is fine for username)
	if username != envUsername {
		slog.Warn("login_failed", "username", username, "ip", r.RemoteAddr, "reason", "invalid_username")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Error:   "Invalid username or password",
		})
		return
	}

	// Then verify password hash using bcrypt (constant-time comparison built-in)
	if err := bcrypt.CompareHashAndPassword([]byte(envPasswordHash), []byte(password)); err != nil {
		slog.Warn("login_failed", "username", username, "ip", r.RemoteAddr, "reason", "invalid_password")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Error:   "Invalid username or password",
		})
		return
	}

	// Create session
	sess, err := s.sessions.Create(username)
	if err != nil {
		slog.Error("session_create_error", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Error:   "Failed to create session",
		})
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     session.GetCookieName(),
		Value:    sess.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil, // Only send over HTTPS if available
		SameSite: http.SameSiteLaxMode,
		Expires:  sess.ExpiresAt,
	})

	slog.Info("login_success", "username", username, "session_id", sess.ID, "ip", r.RemoteAddr)

	// Get redirect target from query param or default to /
	redirect := r.URL.Query().Get("redirect")
	if redirect == "" {
		redirect = "/"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Success:  true,
		Redirect: redirect,
	})
}

// handleLogout destroys the session and redirects to login
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Get session from cookie
	cookie, err := r.Cookie(session.GetCookieName())
	if err == nil {
		// Delete session
		s.sessions.Delete(cookie.Value)
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     session.GetCookieName(),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1, // Delete cookie
	})

	slog.Info("logout", "ip", r.RemoteAddr)

	// Redirect to login page
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// sessionAuthMiddleware checks for valid session or Basic Auth
func (s *Server) sessionAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for public endpoints
		if s.isPublicEndpoint(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Skip if auth is disabled
		if !s.cfg.Security.BasicAuth.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Check for valid session first
		if sess := s.getSessionFromRequest(r); sess != nil {
			next.ServeHTTP(w, r)
			return
		}

		// Check for HTTP Basic Auth as fallback
		envUsername := os.Getenv("BASIC_AUTH_USERNAME")
		envPasswordHash := os.Getenv("BASIC_AUTH_PASSWORD_HASH")

		if envUsername != "" && envPasswordHash != "" {
			reqUser, reqPass, ok := r.BasicAuth()
			if ok && reqUser == envUsername {
				// Verify password using bcrypt
				if err := bcrypt.CompareHashAndPassword([]byte(envPasswordHash), []byte(reqPass)); err == nil {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		// No valid authentication - redirect to login
		slog.Warn("auth_required", "path", r.URL.Path, "ip", r.RemoteAddr)

		// For AJAX requests, return 401
		if r.Header.Get("X-Requested-With") == "XMLHttpRequest" || r.Header.Get("HX-Request") == "true" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// For regular requests, redirect to login with return URL
		loginURL := "/login?redirect=" + r.URL.Path
		http.Redirect(w, r, loginURL, http.StatusSeeOther)
	})
}

// isPublicEndpoint returns true if the path doesn't require authentication
func (s *Server) isPublicEndpoint(path string) bool {
	publicPaths := []string{
		"/login",
		"/auth",
		"/logout",
		"/assets/",
		"/health",
	}

	for _, publicPath := range publicPaths {
		if strings.HasPrefix(path, publicPath) {
			return true
		}
	}

	return false
}

// getSessionFromRequest retrieves the session from the request cookie
func (s *Server) getSessionFromRequest(r *http.Request) *session.Session {
	cookie, err := r.Cookie(session.GetCookieName())
	if err != nil {
		return nil
	}

	return s.sessions.Get(cookie.Value)
}
