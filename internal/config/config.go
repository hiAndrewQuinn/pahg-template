package config

import "fmt"

// Config holds all application configuration
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Coins    []CoinConfig   `mapstructure:"coins"`
	Features FeaturesConfig `mapstructure:"features"`
	Security SecurityConfig `mapstructure:"security"`
	Links    LinksConfig    `mapstructure:"links"`
}

// LinksConfig holds the mandatory feedback link URLs
type LinksConfig struct {
	RequestFeatureURL string `mapstructure:"request_feature_url"`
	ReportBugURL      string `mapstructure:"report_bug_url"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Host string `mapstructure:"host"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// CoinConfig holds cryptocurrency display settings
type CoinConfig struct {
	ID          string `mapstructure:"id"`
	DisplayName string `mapstructure:"display_name"`
}

// FeaturesConfig holds feature flags and settings
type FeaturesConfig struct {
	AvgRefreshIntervalMs int `mapstructure:"avg_refresh_interval_ms"`
}

// SecurityConfig holds security-related settings
// Credentials are loaded from environment variables, not config file
type SecurityConfig struct {
	BasicAuth   BasicAuthConfig   `mapstructure:"basic_auth"`
	IPAllowlist IPAllowlistConfig `mapstructure:"ip_allowlist"`
}

// BasicAuthConfig controls HTTP Basic Authentication
// Username/password come from BASIC_AUTH_USERNAME and BASIC_AUTH_PASSWORD env vars
type BasicAuthConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// IPAllowlistConfig controls IP-based access restrictions
type IPAllowlistConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	CIDRs   []string `mapstructure:"cidrs"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 3000,
			Host: "0.0.0.0",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Coins: []CoinConfig{
			{ID: "bitcoin", DisplayName: "Bitcoin"},
			{ID: "ethereum", DisplayName: "Ethereum"},
			{ID: "dogecoin", DisplayName: "Doge"},
			{ID: "solana", DisplayName: "Solana"},
			{ID: "cardano", DisplayName: "Cardano"},
		},
		Features: FeaturesConfig{
			AvgRefreshIntervalMs: 5000,
		},
		Security: SecurityConfig{
			BasicAuth: BasicAuthConfig{
				Enabled: false,
			},
			IPAllowlist: IPAllowlistConfig{
				Enabled: false,
				CIDRs: []string{
					// IPv4 private ranges
					"127.0.0.0/8",    // Loopback
					"10.0.0.0/8",     // Class A private
					"172.16.0.0/12",  // Class B private
					"192.168.0.0/16", // Class C private
					// IPv6 private ranges
					"::1/128",   // Loopback
					"fc00::/7",  // Unique local addresses
					"fe80::/10", // Link-local addresses
				},
			},
		},
		Links: LinksConfig{
			RequestFeatureURL: "https://github.com/hiAndrewQuinn/pahg-template/issues/new?labels=enhancement&title=%5BFeature%5D+",
			ReportBugURL:      "https://github.com/hiAndrewQuinn/pahg-template/issues/new?labels=bug&title=%5BBug%5D+",
		},
	}
}

// Validate checks that all mandatory configuration fields are set
func (c *Config) Validate() error {
	if c.Links.RequestFeatureURL == "" {
		return fmt.Errorf("links.request_feature_url is required")
	}
	if c.Links.ReportBugURL == "" {
		return fmt.Errorf("links.report_bug_url is required")
	}
	return nil
}
