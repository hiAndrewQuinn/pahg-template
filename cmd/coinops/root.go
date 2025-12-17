package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"pahg-template/internal/config"
)

var (
	cfgFile string
	cfg     *config.Config
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
	// Start with defaults
	cfg = config.DefaultConfig()

	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for config in current directory
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	// Environment variables
	viper.SetEnvPrefix("COINOPS")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Set defaults in Viper (lowest precedence)
	viper.SetDefault("server.port", cfg.Server.Port)
	viper.SetDefault("server.host", cfg.Server.Host)
	viper.SetDefault("logging.level", cfg.Logging.Level)
	viper.SetDefault("logging.format", cfg.Logging.Format)
	viper.SetDefault("features.avg_refresh_interval_ms", cfg.Features.AvgRefreshIntervalMs)

	// Read config file if it exists
	if err := viper.ReadInConfig(); err == nil {
		slog.Debug("using config file", "path", viper.ConfigFileUsed())
	}

	// Unmarshal into config struct
	if err := viper.Unmarshal(cfg); err != nil {
		slog.Error("failed to unmarshal config", "error", err)
	}
}

// GetConfig returns the current configuration
func GetConfig() *config.Config {
	return cfg
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
