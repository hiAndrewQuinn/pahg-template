package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"

	"pahg-template/internal/coingecko"
)

var (
	listUsername string
	listPassword string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured coins with current prices",
	Long: `Fetches and displays all configured coins with their current prices
and 24-hour change. Useful for debugging configuration issues.

Output is TSV format suitable for piping to other tools.

Requires authentication via --username and --password flags.`,
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVarP(&listUsername, "username", "u", "", "Username for authentication (required)")
	listCmd.Flags().StringVarP(&listPassword, "password", "p", "", "Password for authentication (required)")
	listCmd.MarkFlagRequired("username")
	listCmd.MarkFlagRequired("password")
}

func runList(cmd *cobra.Command, args []string) error {
	// Load .env file for credentials
	if err := godotenv.Load(); err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[WARN] Failed to load .env file: %v\n", err)
		}
	}

	// Verify credentials
	if err := verifyCredentials(listUsername, listPassword); err != nil {
		return err
	}

	cfg := GetConfig()

	fmt.Fprintf(os.Stderr, "Config source: %s\n", GetConfigSource())
	fmt.Fprintf(os.Stderr, "Coins configured: %d\n", len(cfg.Coins))
	fmt.Fprintf(os.Stderr, "Timestamp: %s\n\n", time.Now().UTC().Format(time.RFC3339))

	if len(cfg.Coins) == 0 {
		fmt.Fprintln(os.Stderr, "ERROR: No coins configured!")
		return nil
	}

	// Print all configured coins first (config validation)
	fmt.Fprintln(os.Stderr, "Configured coins:")
	for i, c := range cfg.Coins {
		fmt.Fprintf(os.Stderr, "  %3d. %s (%s)\n", i+1, c.ID, c.DisplayName)
	}
	fmt.Fprintln(os.Stderr)

	// Create CoinGecko service with our coins
	service := coingecko.NewService(cfg.Coins)

	// Fetch prices
	fmt.Fprintf(os.Stderr, "Fetching prices for %d coins from CoinGecko...\n", len(cfg.Coins))
	coins, err := service.GetPrices()
	if err != nil {
		return fmt.Errorf("failed to fetch prices: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Received %d price entries\n\n", len(coins))

	// Output as TSV
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tDISPLAY_NAME\tUSD\t24H_CHANGE")

	for _, coin := range coins {
		fmt.Fprintf(w, "%s\t%s\t%.2f\t%.2f%%\n",
			coin.ID,
			coin.DisplayName,
			coin.Price,
			coin.Change24h,
		)
	}

	w.Flush()
	return nil
}

// verifyCredentials checks username and password against stored credentials
func verifyCredentials(username, password string) error {
	envUsername := os.Getenv("BASIC_AUTH_USERNAME")
	envPasswordHash := os.Getenv("BASIC_AUTH_PASSWORD_HASH")

	if envUsername == "" || envPasswordHash == "" {
		return fmt.Errorf("authentication not configured: run 'coinops genenv' first")
	}

	// Check username
	if username != envUsername {
		return fmt.Errorf("authentication failed: invalid credentials")
	}

	// Verify password against bcrypt hash
	if err := bcrypt.CompareHashAndPassword([]byte(envPasswordHash), []byte(password)); err != nil {
		return fmt.Errorf("authentication failed: invalid credentials")
	}

	return nil
}
