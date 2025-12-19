package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"
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
	// Automatically load .env file if it exists
	// This allows credentials to be loaded without manual export
	if err := godotenv.Load(); err != nil {
		if !os.IsNotExist(err) {
			// Log warning but continue - .env is optional
			fmt.Fprintf(os.Stderr, "[WARN] Failed to load .env file: %v\n", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "[INFO] Loaded .env file\n")
	}

	// Setup logger based on config
	SetupLogger()

	cfg := GetConfig()

	// Log comprehensive startup diagnostics
	LogStartupDiagnostics()

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
