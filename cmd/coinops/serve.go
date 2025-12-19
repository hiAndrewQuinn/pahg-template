package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"

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

	// If basic auth is enabled but no credentials are set, generate them
	if cfg.Security.BasicAuth.Enabled {
		if err := ensureCredentials(); err != nil {
			return fmt.Errorf("failed to setup credentials: %w", err)
		}
	}

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

// ensureCredentials checks if credentials are set, and generates them if not.
// This is useful for Docker containers where no .env file is present.
func ensureCredentials() error {
	username := os.Getenv("BASIC_AUTH_USERNAME")
	passwordHash := os.Getenv("BASIC_AUTH_PASSWORD_HASH")

	// If both are set, we're good
	if username != "" && passwordHash != "" {
		slog.Info("credentials_loaded", "username", username)
		return nil
	}

	// Generate new credentials
	newUsername, err := generateSecureString(12)
	if err != nil {
		return fmt.Errorf("failed to generate username: %w", err)
	}

	newPassword, err := generateSecureString(24)
	if err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Set environment variables for this process
	os.Setenv("BASIC_AUTH_USERNAME", newUsername)
	os.Setenv("BASIC_AUTH_PASSWORD_HASH", string(newHash))

	// Print credentials prominently
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "=================================================================")
	fmt.Fprintln(os.Stderr, "  AUTO-GENERATED CREDENTIALS (no .env file or env vars found)")
	fmt.Fprintln(os.Stderr, "=================================================================")
	fmt.Fprintf(os.Stderr, "  Username: %s\n", newUsername)
	fmt.Fprintf(os.Stderr, "  Password: %s\n", newPassword)
	fmt.Fprintln(os.Stderr, "=================================================================")
	fmt.Fprintln(os.Stderr, "  These credentials are valid for THIS SESSION ONLY.")
	fmt.Fprintln(os.Stderr, "  For persistent credentials, run: coinops genenv")
	fmt.Fprintln(os.Stderr, "  Or pass via: docker run -e BASIC_AUTH_USERNAME=... -e BASIC_AUTH_PASSWORD_HASH=...")
	fmt.Fprintln(os.Stderr, "=================================================================")
	fmt.Fprintln(os.Stderr, "")

	slog.Info("credentials_generated", "username", newUsername)
	return nil
}

// Note: generateSecureString is defined in genenv.go and shared across the package
