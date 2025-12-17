package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"pahg-template/internal/server"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the CoinOps dashboard server",
	Long:  `Start the HTTP server that serves the CoinOps dashboard application.`,
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Local flags for serve command
	serveCmd.Flags().IntP("port", "p", 0, "Server port (default from config)")
	serveCmd.Flags().StringP("host", "H", "", "Server host (default from config)")

	// Bind flags to viper
	viper.BindPFlag("server.port", serveCmd.Flags().Lookup("port"))
	viper.BindPFlag("server.host", serveCmd.Flags().Lookup("host"))
}

func runServe(cmd *cobra.Command, args []string) error {
	// Setup logger based on config
	SetupLogger()

	cfg := GetConfig()

	slog.Info("configuration_loaded",
		"port", cfg.Server.Port,
		"host", cfg.Server.Host,
		"log_level", cfg.Logging.Level,
		"log_format", cfg.Logging.Format,
		"coins_count", len(cfg.Coins),
		"avg_refresh_ms", cfg.Features.AvgRefreshIntervalMs,
	)

	// Create server
	srv, err := server.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	slog.Info("server_starting",
		"address", addr,
		"url", fmt.Sprintf("http://localhost:%d", cfg.Server.Port),
	)

	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}
