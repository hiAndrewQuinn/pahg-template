package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"pahg-template/internal/config"
)

var (
	cfgFile    string
	cfg        *config.Config
	configUsed string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "coinops",
	Short: "CoinOps Dashboard - A production-grade internal dashboard",
	Long: `CoinOps Dashboard is a production-grade internal dashboard using the PAHG stack
(Pico, Alpine, HTMX, Go). It features advanced configuration management,
structured observability, and complex frontend-backend timing synchronization.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	defaults := config.DefaultConfig()

	// DIAGNOSTIC: Log what config flag was passed
	fmt.Fprintf(os.Stderr, "[DIAG] Config flag: %q\n", cfgFile)
	fmt.Fprintf(os.Stderr, "[DIAG] CWD: %s\n", mustGetCwd())

	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)

		// DIAGNOSTIC: Check if the file actually exists
		if info, err := os.Stat(cfgFile); err != nil {
			fmt.Fprintf(os.Stderr, "[DIAG] Config file stat ERROR: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "[DIAG] Config file exists: size=%d, mode=%s\n", info.Size(), info.Mode())

			// Try to read first 200 bytes to verify content
			if data, err := os.ReadFile(cfgFile); err != nil {
				fmt.Fprintf(os.Stderr, "[DIAG] Config file read ERROR: %v\n", err)
			} else {
				preview := string(data)
				if len(preview) > 500 {
					preview = preview[:500] + "..."
				}
				fmt.Fprintf(os.Stderr, "[DIAG] Config file preview:\n%s\n", preview)

				// Count coins in raw file
				coinCount := strings.Count(string(data), "- id:")
				fmt.Fprintf(os.Stderr, "[DIAG] Raw file contains %d coin entries\n", coinCount)
			}
		}
	} else {
		// Search for config in current directory
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
		fmt.Fprintf(os.Stderr, "[DIAG] No config flag, searching in CWD for config.yaml\n")
	}

	// Environment variables
	viper.SetEnvPrefix("COINOPS")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Set defaults in Viper (lowest precedence) - including coins
	viper.SetDefault("server.port", defaults.Server.Port)
	viper.SetDefault("server.host", defaults.Server.Host)
	viper.SetDefault("logging.level", defaults.Logging.Level)
	viper.SetDefault("logging.format", defaults.Logging.Format)
	viper.SetDefault("features.avg_refresh_interval_ms", defaults.Features.AvgRefreshIntervalMs)
	viper.SetDefault("coins", defaults.Coins)
	viper.SetDefault("links.request_feature_url", defaults.Links.RequestFeatureURL)
	viper.SetDefault("links.report_bug_url", defaults.Links.ReportBugURL)

	// Read config file if it exists
	configUsed = "defaults-only"
	if err := viper.ReadInConfig(); err == nil {
		configUsed = viper.ConfigFileUsed()
		fmt.Fprintf(os.Stderr, "[DIAG] Viper successfully read config: %s\n", configUsed)

		// Check what viper thinks it has for coins
		viperCoins := viper.Get("coins")
		fmt.Fprintf(os.Stderr, "[DIAG] Viper coins type: %T\n", viperCoins)
		if coins, ok := viperCoins.([]interface{}); ok {
			fmt.Fprintf(os.Stderr, "[DIAG] Viper has %d coins in memory\n", len(coins))
		}
	} else {
		fmt.Fprintf(os.Stderr, "[DIAG] Viper ReadInConfig ERROR: %v\n", err)
		configUsed = fmt.Sprintf("defaults-only (error: %v)", err)
	}

	// Unmarshal into a FRESH config struct (not pre-populated)
	cfg = &config.Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[DIAG] Viper Unmarshal ERROR: %v\n", err)
		cfg = defaults // fallback
	}

	// Validate mandatory configuration fields
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] Configuration validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "[DIAG] Final config has %d coins\n", len(cfg.Coins))
	if len(cfg.Coins) > 0 {
		fmt.Fprintf(os.Stderr, "[DIAG] First coin: %s (%s)\n", cfg.Coins[0].ID, cfg.Coins[0].DisplayName)
	}
	if len(cfg.Coins) > 5 {
		fmt.Fprintf(os.Stderr, "[DIAG] Sixth coin: %s (%s)\n", cfg.Coins[5].ID, cfg.Coins[5].DisplayName)
	}
}

// GetConfig returns the current configuration
func GetConfig() *config.Config {
	return cfg
}

// GetConfigSource returns where the config was loaded from
func GetConfigSource() string {
	return configUsed
}

// LogStartupDiagnostics logs detailed startup information as JSON
func LogStartupDiagnostics() {
	coinIDs := make([]string, len(cfg.Coins))
	for i, c := range cfg.Coins {
		coinIDs[i] = c.ID
	}
	coinsJSON, _ := json.Marshal(coinIDs)

	slog.Info("startup_diagnostics",
		"config_source", configUsed,
		"config_flag", cfgFile,
		"cwd", mustGetCwd(),
		"coins_count", len(cfg.Coins),
		"coins", string(coinsJSON),
		"server_port", cfg.Server.Port,
		"server_host", cfg.Server.Host,
		"log_level", cfg.Logging.Level,
		"avg_refresh_ms", cfg.Features.AvgRefreshIntervalMs,
	)
}

func mustGetCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return cwd
}

// SetupLogger configures the global slog logger based on config
func SetupLogger() {
	var handler slog.Handler

	level := slog.LevelInfo
	switch strings.ToLower(cfg.Logging.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}

	if strings.ToLower(cfg.Logging.Format) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}
