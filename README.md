# PAHG Template - Pico.css, Alpine.js, HTMX, Go

A production-grade internal dashboard template demonstrating the PAHG stack with enterprise features.

## Features

- **Live Crypto Ticker** - Real-time prices from CoinGecko API with Poisson-distributed refresh intervals
- **Visual Countdown Timer** - SVG donut animation showing time until next refresh
- **Active Search** - Debounced search filtering with `hx-trigger="keyup changed delay:500ms"`
- **Loading States** - Simulated slow operations with HTMX indicators
- **3-Way Dark Mode** - Light/Dark/Auto with OS preference detection and localStorage persistence
- **Notification Center** - Thread-safe in-memory store with modal history view
- **OOB Swaps** - Notification counter updates via `hx-swap-oob`
- **Structured Logging** - JSON output with slog middleware for HTTP tracing
- **Flexible Configuration** - Cobra/Viper with config file, environment variables, and flags

## Stack

| Component | Version | Purpose |
|-----------|---------|---------|
| Go | 1.21+ | Backend, CLI (Cobra), config (Viper), logging (slog) |
| HTMX | 2.0.4 | HTML over the wire |
| Alpine.js | 3.14.7 | Client-side reactivity |
| Pico.css | 2.0.6 | Classless CSS framework |

All frontend dependencies are vendored in `/internal/server/assets/` for immutability and offline use.

## Quick Start

```bash
# Build
go build -o coinops ./cmd/coinops

# Run with defaults (port 3000)
./coinops serve

# Or specify options
./coinops serve --port 8080
COINOPS_SERVER_PORT=9090 ./coinops serve
```

Open http://localhost:3000 in your browser.

## Configuration

CoinOps uses a hierarchical configuration system via Viper. Values are read in this order of precedence (highest to lowest):

1. **Command-line flags**: `./coinops serve --port 9090`
2. **Environment variables**: `COINOPS_SERVER_PORT=9090` (prefix: `COINOPS_`)
3. **Config file**: `config.yaml` in the current directory
4. **Defaults**: Hardcoded in the application

### Config File

Create a `config.yaml` in the same directory as the binary:

```yaml
server:
  port: 3000
  host: "0.0.0.0"

logging:
  level: "info"    # debug, info, warn, error
  format: "json"   # json, text

coins:
  - id: "bitcoin"
    display_name: "Bitcoin"
  - id: "ethereum"
    display_name: "Ethereum"
  - id: "dogecoin"
    display_name: "Doge"

features:
  avg_refresh_interval_ms: 5000  # Lambda for Poisson distribution
```

### Environment Variables

All config keys can be set via environment variables with the `COINOPS_` prefix:

```bash
# Server settings
COINOPS_SERVER_PORT=8080
COINOPS_SERVER_HOST=127.0.0.1

# Logging settings
COINOPS_LOGGING_LEVEL=debug
COINOPS_LOGGING_FORMAT=text

# Feature settings
COINOPS_FEATURES_AVG_REFRESH_INTERVAL_MS=3000
```

### Example: Override Config via Environment

```bash
# Use config.yaml but override the port
COINOPS_SERVER_PORT=5000 ./coinops serve

# Use debug logging in development
COINOPS_LOGGING_LEVEL=debug COINOPS_LOGGING_FORMAT=text ./coinops serve
```

## Project Structure

```
.
├── cmd/
│   └── coinops/
│       ├── main.go           # Entrypoint
│       ├── root.go           # Root command & Viper config setup
│       └── serve.go          # 'serve' command implementation
├── internal/
│   ├── config/
│   │   └── config.go         # Configuration structs
│   ├── coingecko/
│   │   └── service.go        # CoinGecko API client with caching
│   ├── math/
│   │   └── poisson.go        # Poisson/exponential delay generator
│   ├── middleware/
│   │   └── logger.go         # slog HTTP logging middleware
│   ├── notifications/
│   │   └── store.go          # Thread-safe notification store
│   └── server/
│       ├── server.go         # HTTP handlers and routing
│       ├── assets/           # Embedded static files
│       │   ├── js/
│       │   │   ├── htmx.min.js
│       │   │   └── alpine.min.js
│       │   └── css/
│       │       └── pico.min.css
│       └── templates/        # Embedded HTML templates
│           ├── layout.html
│           ├── index.html
│           └── partials/
│               ├── ticker.html
│               ├── report-success.html
│               └── notifications.html
├── config.yaml               # Default configuration
└── go.mod
```

## Demo Scenarios

1. **Poisson Refresh**: Watch the SVG donut countdown - each refresh interval is randomly distributed around 5s
2. **Search**: Type "doge" to filter the coin list (debounced 500ms)
3. **Loading states**: Click "Generate Compliance Report" - shows spinner for 3s
4. **OOB swap**: After report generation, notification counter updates automatically
5. **Notifications**: Click "Notifications" in nav to see history with timestamps
6. **Dark Mode**: Click Settings to toggle between Light/Dark/Auto themes (persists to localStorage)

## Structured Logging

All HTTP requests are logged in JSON format with timing information:

```json
{"time":"2024-01-15T10:30:00Z","level":"INFO","msg":"request_started","method":"GET","path":"/ticker","ip":"127.0.0.1:54321"}
{"time":"2024-01-15T10:30:00Z","level":"INFO","msg":"prices_updated","count":5}
{"time":"2024-01-15T10:30:00Z","level":"INFO","msg":"request_completed","method":"GET","path":"/ticker","status":200,"duration_ms":45.2}
```

Enable debug logging for more verbose output:

```bash
COINOPS_LOGGING_LEVEL=debug ./coinops serve
```

## Poisson Timing

The ticker uses exponential distribution (the inter-arrival time for Poisson processes) to create natural-feeling random refresh intervals:

```go
// Time = -ln(U) * mean, where U is uniform random [0,1)
delay := int(-math.Log(rand.Float64()) * targetMean)
```

This creates variance around the mean refresh interval, making the dashboard feel more dynamic.

## Portability

The compiled binary embeds all templates and static assets via `go:embed`. Deploy anywhere with:

```bash
# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o coinops-linux ./cmd/coinops

# Cross-compile for macOS
GOOS=darwin GOARCH=arm64 go build -o coinops-macos ./cmd/coinops

# Cross-compile for Windows
GOOS=windows GOARCH=amd64 go build -o coinops.exe ./cmd/coinops
```

## CLI Reference

```
CoinOps Dashboard - A production-grade internal dashboard

Usage:
  coinops [command]

Available Commands:
  help        Help about any command
  serve       Start the CoinOps dashboard server

Flags:
      --config string   config file (default is ./config.yaml)
  -h, --help            help for coinops

Use "coinops [command] --help" for more information about a command.
```

### Serve Command

```
Start the HTTP server that serves the CoinOps dashboard application.

Usage:
  coinops serve [flags]

Flags:
  -H, --host string   Server host (default from config)
  -p, --port int      Server port (default from config)
  -h, --help          help for serve

Global Flags:
      --config string   config file (default is ./config.yaml)
```
