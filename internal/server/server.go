package server

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"pahg-template/internal/coingecko"
	"pahg-template/internal/config"
	pmath "pahg-template/internal/math"
	"pahg-template/internal/middleware"
	"pahg-template/internal/notifications"
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
	mux           *http.ServeMux
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
		mux:           http.NewServeMux(),
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
}

// Handler returns the HTTP handler with middleware applied
func (s *Server) Handler() http.Handler {
	// Chain middleware: RequestID -> Logging
	// RequestID must run first so the logger can access it from context
	return middleware.RequestIDMiddleware(
		middleware.LoggingMiddleware(s.mux),
	)
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
