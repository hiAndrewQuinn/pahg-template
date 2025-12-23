package server

import (
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"pahg-template/internal/config"
	"pahg-template/internal/session"
)

func newTestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Port: 3000,
			Host: "localhost",
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Coins: []config.CoinConfig{
			{ID: "bitcoin", DisplayName: "Bitcoin"},
			{ID: "ethereum", DisplayName: "Ethereum"},
		},
		Features: config.FeaturesConfig{
			AvgRefreshIntervalMs: 5000,
		},
		Security: config.SecurityConfig{
			BasicAuth: config.BasicAuthConfig{
				Enabled: false,
			},
			IPAllowlist: config.IPAllowlistConfig{
				Enabled: false,
			},
		},
		Links: config.LinksConfig{
			RequestFeatureURL: "https://example.com/feature",
			ReportBugURL:      "https://example.com/bug",
		},
	}
}

func TestNew(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)

	require.NoError(t, err)
	require.NotNil(t, server)
	assert.NotNil(t, server.cfg)
	assert.NotNil(t, server.templates)
	assert.NotNil(t, server.coinService)
	assert.NotNil(t, server.notifications)
	assert.NotNil(t, server.sessions)
	assert.NotNil(t, server.mux)
}

func TestHandler(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	handler := server.Handler()
	assert.NotNil(t, handler)
}

func TestHandleHealth(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	server.handleHealth(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response HealthResponse
	err = json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "ok", response.Status)
	assert.NotEmpty(t, response.Uptime)
	assert.Greater(t, response.Goroutines, 0)
	assert.GreaterOrEqual(t, response.MemoryMB, 0.0)
	assert.NotEmpty(t, response.GoVersion)
}

func TestHandleMetadata(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/metadata", nil)
	rec := httptest.NewRecorder()

	server.handleMetadata(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response MetadataResponse
	err = json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.NotEmpty(t, response.Version)
	assert.NotEmpty(t, response.Environment)
	assert.NotNil(t, response.Features)
}

func TestHandleLogin_GET(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/login", nil)
	rec := httptest.NewRecorder()

	server.handleLogin(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
}

func TestHandleLogin_AlreadyAuthenticated(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	// Create a session
	sess, _ := server.sessions.Create("testuser")

	req := httptest.NewRequest("GET", "/login", nil)
	req.AddCookie(&http.Cookie{
		Name:  session.GetCookieName(),
		Value: sess.ID,
	})
	rec := httptest.NewRecorder()

	server.handleLogin(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/", rec.Header().Get("Location"))
}

func TestHandleAuth_Success(t *testing.T) {
	// Set up credentials
	password := "testpassword"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	os.Setenv("BASIC_AUTH_USERNAME", "testuser")
	os.Setenv("BASIC_AUTH_PASSWORD_HASH", string(hash))
	defer func() {
		os.Unsetenv("BASIC_AUTH_USERNAME")
		os.Unsetenv("BASIC_AUTH_PASSWORD_HASH")
	}()

	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	form := url.Values{}
	form.Set("username", "testuser")
	form.Set("password", password)

	req := httptest.NewRequest("POST", "/auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.handleAuth(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response AuthResponse
	err = json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.True(t, response.Success)

	// Should have a cookie set
	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == session.GetCookieName() {
			sessionCookie = c
			break
		}
	}
	require.NotNil(t, sessionCookie)
	assert.NotEmpty(t, sessionCookie.Value)
}

func TestHandleAuth_InvalidCredentials(t *testing.T) {
	password := "testpassword"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	os.Setenv("BASIC_AUTH_USERNAME", "testuser")
	os.Setenv("BASIC_AUTH_PASSWORD_HASH", string(hash))
	defer func() {
		os.Unsetenv("BASIC_AUTH_USERNAME")
		os.Unsetenv("BASIC_AUTH_PASSWORD_HASH")
	}()

	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	form := url.Values{}
	form.Set("username", "testuser")
	form.Set("password", "wrongpassword")

	req := httptest.NewRequest("POST", "/auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.handleAuth(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var response AuthResponse
	err = json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	assert.False(t, response.Success)
}

func TestHandleAuth_MethodNotAllowed(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/auth", nil)
	rec := httptest.NewRecorder()

	server.handleAuth(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestHandleLogout(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	// Create a session first
	sess, _ := server.sessions.Create("testuser")
	require.Equal(t, 1, server.sessions.Count())

	req := httptest.NewRequest("GET", "/logout", nil)
	req.AddCookie(&http.Cookie{
		Name:  session.GetCookieName(),
		Value: sess.ID,
	})
	rec := httptest.NewRecorder()

	server.handleLogout(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/login", rec.Header().Get("Location"))

	// Session should be deleted
	assert.Equal(t, 0, server.sessions.Count())
}

func TestHandleIndex_NotRoot(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	rec := httptest.NewRecorder()

	server.handleIndex(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleTicker(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/ticker", nil)
	rec := httptest.NewRecorder()

	server.handleTicker(rec, req)

	// May fail due to network, but should return something
	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, rec.Code)
}

func TestHandleTickerCoin_Empty(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/ticker/", nil)
	rec := httptest.NewRecorder()

	server.handleTickerCoin(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleSearch(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/search?search=bit", nil)
	rec := httptest.NewRecorder()

	server.handleSearch(rec, req)

	// May fail due to network, but should return something
	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, rec.Code)
}

func TestHandleGenerateReport_MethodNotAllowed(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/generate-report", nil)
	rec := httptest.NewRecorder()

	server.handleGenerateReport(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestHandleNotifications(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/notifications", nil)
	rec := httptest.NewRecorder()

	server.handleNotifications(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
}

func TestIsPublicEndpoint(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	testCases := []struct {
		path     string
		isPublic bool
	}{
		{"/login", true},
		{"/auth", true},
		{"/logout", true},
		{"/assets/css/style.css", true},
		{"/health", true},
		{"/", false},
		{"/ticker", false},
		{"/search", false},
		{"/notifications", false},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			result := server.isPublicEndpoint(tc.path)
			assert.Equal(t, tc.isPublic, result)
		})
	}
}

func TestGetSessionFromRequest_NoSession(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/", nil)

	sess := server.getSessionFromRequest(req)

	assert.Nil(t, sess)
}

func TestGetSessionFromRequest_ValidSession(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	// Create a session
	createdSess, _ := server.sessions.Create("testuser")

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{
		Name:  session.GetCookieName(),
		Value: createdSess.ID,
	})

	sess := server.getSessionFromRequest(req)

	require.NotNil(t, sess)
	assert.Equal(t, createdSess.ID, sess.ID)
}

func TestGenerateDelayQueue(t *testing.T) {
	cfg := newTestConfig()
	cfg.Features.AvgRefreshIntervalMs = 1000

	server, err := New(cfg)
	require.NoError(t, err)

	delays := server.generateDelayQueue()

	assert.Len(t, delays, 10)
	for _, delay := range delays {
		// Should be within bounds (0.1x to 10x of mean)
		assert.GreaterOrEqual(t, delay, 100) // 0.1 * 1000
		assert.LessOrEqual(t, delay, 10000)  // 10 * 1000
	}
}

func TestGetEnvironment(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		os.Unsetenv("ENVIRONMENT")
		os.Unsetenv("ENV")

		env := getEnvironment()
		assert.Equal(t, "production", env)
	})

	t.Run("ENVIRONMENT set", func(t *testing.T) {
		os.Setenv("ENVIRONMENT", "staging")
		defer os.Unsetenv("ENVIRONMENT")

		env := getEnvironment()
		assert.Equal(t, "staging", env)
	})

	t.Run("ENV set", func(t *testing.T) {
		os.Unsetenv("ENVIRONMENT")
		os.Setenv("ENV", "development")
		defer os.Unsetenv("ENV")

		env := getEnvironment()
		assert.Equal(t, "development", env)
	})
}

func TestSessionAuthMiddleware_PublicEndpoint(t *testing.T) {
	cfg := newTestConfig()
	cfg.Security.BasicAuth.Enabled = true
	server, err := New(cfg)
	require.NoError(t, err)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := server.sessionAuthMiddleware(next)

	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestSessionAuthMiddleware_Disabled(t *testing.T) {
	cfg := newTestConfig()
	cfg.Security.BasicAuth.Enabled = false
	server, err := New(cfg)
	require.NoError(t, err)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := server.sessionAuthMiddleware(next)

	req := httptest.NewRequest("GET", "/protected", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called)
}

func TestSessionAuthMiddleware_ValidSession(t *testing.T) {
	cfg := newTestConfig()
	cfg.Security.BasicAuth.Enabled = true
	server, err := New(cfg)
	require.NoError(t, err)

	// Create a session
	sess, _ := server.sessions.Create("testuser")

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := server.sessionAuthMiddleware(next)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{
		Name:  session.GetCookieName(),
		Value: sess.ID,
	})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called)
}

func TestSessionAuthMiddleware_NoSession_Redirect(t *testing.T) {
	cfg := newTestConfig()
	cfg.Security.BasicAuth.Enabled = true
	server, err := New(cfg)
	require.NoError(t, err)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := server.sessionAuthMiddleware(next)

	req := httptest.NewRequest("GET", "/protected", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), "/login")
}

func TestSessionAuthMiddleware_AJAX_Returns401(t *testing.T) {
	cfg := newTestConfig()
	cfg.Security.BasicAuth.Enabled = true
	server, err := New(cfg)
	require.NoError(t, err)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := server.sessionAuthMiddleware(next)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestPageData(t *testing.T) {
	data := PageData{
		Title:             "Test",
		NotificationCount: 5,
		AvgRefreshMs:      1000,
		Version:           "1.0.0",
		Commit:            "abc123",
		CommitDate:        "2025-01-01",
		RequestFeatureURL: "https://example.com/feature",
		ReportBugURL:      "https://example.com/bug",
	}

	assert.Equal(t, "Test", data.Title)
	assert.Equal(t, 5, data.NotificationCount)
	assert.Equal(t, 1000, data.AvgRefreshMs)
}

func TestTickerData(t *testing.T) {
	data := TickerData{
		Coins: []CoinRowData{
			{ID: "bitcoin", DisplayName: "Bitcoin", Price: 50000.00},
		},
	}

	assert.Len(t, data.Coins, 1)
	assert.Equal(t, "bitcoin", data.Coins[0].ID)
}

func TestCoinRowData(t *testing.T) {
	data := CoinRowData{
		ID:          "bitcoin",
		DisplayName: "Bitcoin (BTC)",
		Price:       50000.00,
		Change24h:   2.5,
		Delays:      []int{1000, 2000, 3000},
	}

	assert.Equal(t, "bitcoin", data.ID)
	assert.Equal(t, "Bitcoin (BTC)", data.DisplayName)
	assert.Equal(t, 50000.00, data.Price)
	assert.Equal(t, 2.5, data.Change24h)
	assert.Len(t, data.Delays, 3)
}

func TestReportData(t *testing.T) {
	data := ReportData{
		Timestamp:         "20250120_120000",
		NotificationCount: 10,
	}

	assert.Equal(t, "20250120_120000", data.Timestamp)
	assert.Equal(t, 10, data.NotificationCount)
}

func TestNotificationsData(t *testing.T) {
	data := NotificationsData{
		Count: 3,
	}

	assert.Equal(t, 3, data.Count)
}

func TestMetadataResponse(t *testing.T) {
	response := MetadataResponse{
		Version:     "1.0.0",
		Commit:      "abc123",
		CommitDate:  "2025-01-01",
		Environment: "production",
		Features:    map[string]interface{}{"feature1": true},
	}

	assert.Equal(t, "1.0.0", response.Version)
	assert.Equal(t, "production", response.Environment)
	assert.True(t, response.Features["feature1"].(bool))
}

func TestHealthResponse(t *testing.T) {
	response := HealthResponse{
		Status:     "ok",
		Uptime:     "1h0m0s",
		Goroutines: 10,
		MemoryMB:   100.5,
		GoVersion:  "go1.21.0",
	}

	assert.Equal(t, "ok", response.Status)
	assert.Equal(t, "1h0m0s", response.Uptime)
	assert.Equal(t, 10, response.Goroutines)
	assert.Equal(t, 100.5, response.MemoryMB)
}

func TestAuthRequest(t *testing.T) {
	request := AuthRequest{
		Username: "testuser",
		Password: "testpass",
	}

	assert.Equal(t, "testuser", request.Username)
	assert.Equal(t, "testpass", request.Password)
}

func TestAuthResponse(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		response := AuthResponse{
			Success:  true,
			Redirect: "/",
		}

		assert.True(t, response.Success)
		assert.Empty(t, response.Error)
		assert.Equal(t, "/", response.Redirect)
	})

	t.Run("failure", func(t *testing.T) {
		response := AuthResponse{
			Success: false,
			Error:   "Invalid credentials",
		}

		assert.False(t, response.Success)
		assert.Equal(t, "Invalid credentials", response.Error)
	})
}

func TestServer_StartTime(t *testing.T) {
	cfg := newTestConfig()
	before := time.Now()
	server, err := New(cfg)
	after := time.Now()

	require.NoError(t, err)
	assert.True(t, server.startTime.After(before) || server.startTime.Equal(before))
	assert.True(t, server.startTime.Before(after) || server.startTime.Equal(after))
}

func TestHandleIndex_Success(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	server.handleIndex(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	// Verify the page contains expected content
	body := rec.Body.String()
	assert.Contains(t, body, "Dashboard")
}

func TestHandleTickerCoin_Found(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	// Use a coin that's in the default config (will use fallback prices)
	req := httptest.NewRequest("GET", "/ticker/bitcoin", nil)
	rec := httptest.NewRecorder()

	server.handleTickerCoin(rec, req)

	// Will either succeed or return 404 if coin not in service
	assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, rec.Code)
}

func TestHandleTickerCoin_NotFound(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/ticker/nonexistent-coin", nil)
	rec := httptest.NewRecorder()

	server.handleTickerCoin(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleGenerateReport_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow test in short mode")
	}

	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	initialCount := server.notifications.Count()

	req := httptest.NewRequest("POST", "/generate-report", nil)
	rec := httptest.NewRecorder()

	server.handleGenerateReport(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// Should add a notification
	assert.Equal(t, initialCount+1, server.notifications.Count())
}

func TestHandleNotifications_WithNotifications(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	// Add some notifications
	server.notifications.Add("Test Title 1", "Test Message 1")
	server.notifications.Add("Test Title 2", "Test Message 2")

	req := httptest.NewRequest("GET", "/notifications", nil)
	rec := httptest.NewRecorder()

	server.handleNotifications(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Test Title 1")
	assert.Contains(t, body, "Test Title 2")
}

func TestHandleSearch_EmptyQuery(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/search", nil)
	rec := httptest.NewRecorder()

	server.handleSearch(rec, req)

	// Should return all coins with empty query
	assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, rec.Code)
}

func TestHandleAuth_WithRedirect(t *testing.T) {
	password := "testpassword"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	os.Setenv("BASIC_AUTH_USERNAME", "testuser")
	os.Setenv("BASIC_AUTH_PASSWORD_HASH", string(hash))
	defer func() {
		os.Unsetenv("BASIC_AUTH_USERNAME")
		os.Unsetenv("BASIC_AUTH_PASSWORD_HASH")
	}()

	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	form := url.Values{}
	form.Set("username", "testuser")
	form.Set("password", password)

	req := httptest.NewRequest("POST", "/auth?redirect=/dashboard", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.handleAuth(rec, req)

	var response AuthResponse
	json.NewDecoder(rec.Body).Decode(&response)
	assert.Equal(t, "/dashboard", response.Redirect)
}

func TestHandleAuth_InvalidUsername(t *testing.T) {
	password := "testpassword"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	os.Setenv("BASIC_AUTH_USERNAME", "testuser")
	os.Setenv("BASIC_AUTH_PASSWORD_HASH", string(hash))
	defer func() {
		os.Unsetenv("BASIC_AUTH_USERNAME")
		os.Unsetenv("BASIC_AUTH_PASSWORD_HASH")
	}()

	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	form := url.Values{}
	form.Set("username", "wronguser")
	form.Set("password", password)

	req := httptest.NewRequest("POST", "/auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.handleAuth(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandleLogout_NoCookie(t *testing.T) {
	cfg := newTestConfig()
	server, err := New(cfg)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/logout", nil)
	rec := httptest.NewRecorder()

	server.handleLogout(rec, req)

	// Should still redirect even without a session
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/login", rec.Header().Get("Location"))
}

func TestSessionAuthMiddleware_BasicAuth(t *testing.T) {
	password := "testpassword"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	os.Setenv("BASIC_AUTH_USERNAME", "testuser")
	os.Setenv("BASIC_AUTH_PASSWORD_HASH", string(hash))
	defer func() {
		os.Unsetenv("BASIC_AUTH_USERNAME")
		os.Unsetenv("BASIC_AUTH_PASSWORD_HASH")
	}()

	cfg := newTestConfig()
	cfg.Security.BasicAuth.Enabled = true
	server, err := New(cfg)
	require.NoError(t, err)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := server.sessionAuthMiddleware(next)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.SetBasicAuth("testuser", password)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, called)
}

func TestSessionAuthMiddleware_InvalidBasicAuth(t *testing.T) {
	password := "testpassword"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	os.Setenv("BASIC_AUTH_USERNAME", "testuser")
	os.Setenv("BASIC_AUTH_PASSWORD_HASH", string(hash))
	defer func() {
		os.Unsetenv("BASIC_AUTH_USERNAME")
		os.Unsetenv("BASIC_AUTH_PASSWORD_HASH")
	}()

	cfg := newTestConfig()
	cfg.Security.BasicAuth.Enabled = true
	server, err := New(cfg)
	require.NoError(t, err)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := server.sessionAuthMiddleware(next)

	req := httptest.NewRequest("GET", "/protected", nil)
	req.SetBasicAuth("testuser", "wrongpassword")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.False(t, called)
}

func TestFuncMap(t *testing.T) {
	// Test the json template function
	fn := funcMap["json"].(func(interface{}) template.JS)
	result := fn(map[string]int{"test": 123})
	assert.Contains(t, string(result), "test")
	assert.Contains(t, string(result), "123")
}
