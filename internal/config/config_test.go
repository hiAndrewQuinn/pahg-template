package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	require.NotNil(t, cfg)

	t.Run("server defaults", func(t *testing.T) {
		assert.Equal(t, 3000, cfg.Server.Port)
		assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	})

	t.Run("logging defaults", func(t *testing.T) {
		assert.Equal(t, "info", cfg.Logging.Level)
		assert.Equal(t, "json", cfg.Logging.Format)
	})

	t.Run("coins defaults", func(t *testing.T) {
		assert.Len(t, cfg.Coins, 5, "should have 5 default coins")

		expectedCoins := []struct {
			id          string
			displayName string
		}{
			{"bitcoin", "Bitcoin"},
			{"ethereum", "Ethereum"},
			{"dogecoin", "Doge"},
			{"solana", "Solana"},
			{"cardano", "Cardano"},
		}

		for i, expected := range expectedCoins {
			assert.Equal(t, expected.id, cfg.Coins[i].ID)
			assert.Equal(t, expected.displayName, cfg.Coins[i].DisplayName)
		}
	})

	t.Run("features defaults", func(t *testing.T) {
		assert.Equal(t, 5000, cfg.Features.AvgRefreshIntervalMs)
	})

	t.Run("security defaults", func(t *testing.T) {
		assert.False(t, cfg.Security.BasicAuth.Enabled)
		assert.False(t, cfg.Security.IPAllowlist.Enabled)

		// Should have default CIDR ranges
		expectedCIDRs := []string{
			"127.0.0.0/8",
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"::1/128",
			"fc00::/7",
			"fe80::/10",
		}
		assert.Equal(t, expectedCIDRs, cfg.Security.IPAllowlist.CIDRs)
	})

	t.Run("links defaults", func(t *testing.T) {
		assert.Contains(t, cfg.Links.RequestFeatureURL, "github.com")
		assert.Contains(t, cfg.Links.RequestFeatureURL, "enhancement")
		assert.Contains(t, cfg.Links.ReportBugURL, "github.com")
		assert.Contains(t, cfg.Links.ReportBugURL, "bug")
	})
}

func TestConfig_Validate(t *testing.T) {
	t.Run("valid config passes", func(t *testing.T) {
		cfg := DefaultConfig()
		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("empty request feature URL fails", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Links.RequestFeatureURL = ""

		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "request_feature_url is required")
	})

	t.Run("empty report bug URL fails", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Links.ReportBugURL = ""

		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "report_bug_url is required")
	})

	t.Run("both URLs empty fails on first one", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Links.RequestFeatureURL = ""
		cfg.Links.ReportBugURL = ""

		err := cfg.Validate()
		assert.Error(t, err)
		// Should fail on the first check (request_feature_url)
		assert.Contains(t, err.Error(), "request_feature_url is required")
	})

	t.Run("custom URLs pass validation", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Links.RequestFeatureURL = "https://example.com/feature"
		cfg.Links.ReportBugURL = "https://example.com/bug"

		err := cfg.Validate()
		assert.NoError(t, err)
	})
}

func TestCoinConfig(t *testing.T) {
	coin := CoinConfig{
		ID:          "bitcoin",
		DisplayName: "Bitcoin (BTC)",
	}

	assert.Equal(t, "bitcoin", coin.ID)
	assert.Equal(t, "Bitcoin (BTC)", coin.DisplayName)
}

func TestServerConfig(t *testing.T) {
	server := ServerConfig{
		Port: 8080,
		Host: "localhost",
	}

	assert.Equal(t, 8080, server.Port)
	assert.Equal(t, "localhost", server.Host)
}

func TestLoggingConfig(t *testing.T) {
	logging := LoggingConfig{
		Level:  "debug",
		Format: "text",
	}

	assert.Equal(t, "debug", logging.Level)
	assert.Equal(t, "text", logging.Format)
}

func TestFeaturesConfig(t *testing.T) {
	features := FeaturesConfig{
		AvgRefreshIntervalMs: 10000,
	}

	assert.Equal(t, 10000, features.AvgRefreshIntervalMs)
}

func TestSecurityConfig(t *testing.T) {
	security := SecurityConfig{
		BasicAuth: BasicAuthConfig{
			Enabled: true,
		},
		IPAllowlist: IPAllowlistConfig{
			Enabled: true,
			CIDRs:   []string{"192.168.1.0/24"},
		},
	}

	assert.True(t, security.BasicAuth.Enabled)
	assert.True(t, security.IPAllowlist.Enabled)
	assert.Equal(t, []string{"192.168.1.0/24"}, security.IPAllowlist.CIDRs)
}

func TestLinksConfig(t *testing.T) {
	links := LinksConfig{
		RequestFeatureURL: "https://example.com/feature",
		ReportBugURL:      "https://example.com/bug",
	}

	assert.Equal(t, "https://example.com/feature", links.RequestFeatureURL)
	assert.Equal(t, "https://example.com/bug", links.ReportBugURL)
}
