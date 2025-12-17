package config

// Config holds all application configuration
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Coins    []CoinConfig   `mapstructure:"coins"`
	Features FeaturesConfig `mapstructure:"features"`
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
	}
}
