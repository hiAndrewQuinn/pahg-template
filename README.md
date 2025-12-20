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
- **Authentication** - Session-based login with bcrypt password hashing and auto-generated credentials

## Stack

| Component | Version | Purpose |
|-----------|---------|---------|
| Go | 1.25+ | Backend, CLI (Cobra), config (Viper), logging (slog) |
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

## Docker

Build and run with Docker using the included distroless Dockerfile:

```bash
# Build the image
docker build -t coinops .

# Run in foreground (see logs)
docker run -p 3000:3000 coinops

# Run detached (background)
docker run -d -p 3000:3000 --name coinops coinops
```

Open http://localhost:3000 in your browser.

### Docker Commands

```bash
# View logs (if detached)
docker logs coinops

# Follow logs live
docker logs -f coinops

# Stop the container
docker stop coinops

# Remove the container
docker rm coinops
```

### Docker Configuration

Override settings via environment variables:

```bash
# Change the port
docker run -p 8080:8080 -e COINOPS_SERVER_PORT=8080 coinops

# Enable debug logging
docker run -p 3000:3000 -e COINOPS_LOGGING_LEVEL=debug coinops

# Use a custom config file
docker run -p 3000:3000 -v $(pwd)/my-config.yaml:/config.yaml coinops
```

The image uses `gcr.io/distroless/static-debian12:nonroot` as the runtime base - a minimal ~2MB image with no shell, running as non-root by default.

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

## Authentication

CoinOps includes a complete authentication system with bcrypt password hashing and session management.

### Quick Setup

```bash
# Generate credentials (creates .env file with bcrypt-hashed password)
./coinops genenv

# Start server (auto-loads .env)
./coinops serve
```

The `genenv` command displays the plaintext password **once** - save it securely. Only the bcrypt hash is stored in `.env`.

### Docker

When running in Docker without credentials, the server auto-generates temporary credentials:

```bash
docker run -p 3000:3000 coinops
```

Output:
```
=================================================================
  AUTO-GENERATED CREDENTIALS (no .env file or env vars found)
=================================================================
  Username: abc123XYZ789
  Password: someSecurePassword24chars
=================================================================
  These credentials are valid for THIS SESSION ONLY.
=================================================================
```

For persistent Docker credentials:

```bash
# Option 1: Generate locally and pass .env file
./coinops genenv
docker run -p 3000:3000 --env-file .env coinops

# Option 2: Pass environment variables directly
docker run -p 3000:3000 \
  -e BASIC_AUTH_USERNAME=myuser \
  -e BASIC_AUTH_PASSWORD_HASH='$2a$10$...' \
  coinops
```

### Configuration

Enable/disable authentication in `config.yaml`:

```yaml
security:
  basic_auth:
    enabled: true  # Set to false to disable authentication
```

### Security Features

- **Bcrypt hashing** - Passwords hashed with cost factor 10
- **Session cookies** - HttpOnly, SameSite=Lax, Secure on HTTPS
- **24-hour sessions** - Auto-expiry with background cleanup
- **Constant-time comparison** - Built into bcrypt verification
- **Login page** - Clean UI with Pico.css and Alpine.js at `/login`
- **Logout** - Session destruction at `/logout`

## Project Structure

```
.
├── cmd/
│   └── coinops/
│       ├── main.go           # Entrypoint
│       ├── root.go           # Root command & Viper config setup
│       ├── serve.go          # 'serve' command implementation
│       ├── genenv.go         # 'genenv' credential generator
│       └── list.go           # 'list' command (authenticated)
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
│   ├── session/
│   │   └── store.go          # In-memory session management
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
│           ├── login.html
│           └── partials/
│               ├── ticker.html
│               ├── report-success.html
│               └── notifications.html
├── config.yaml               # Default configuration
├── Dockerfile                # Multi-stage build with Go 1.25
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
  genenv      Generate .env file with secure credentials
  help        Help about any command
  list        List all configured coins with current prices
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

### Genenv Command

```
Generate a .env file with bcrypt-hashed credentials for authentication.

Usage:
  coinops genenv [flags]

Flags:
  -f, --force           Overwrite existing .env file
  -o, --output string   Output path for .env file (default ".env")
  -h, --help            help for genenv
```

### List Command

```
List all configured coins with current prices (requires authentication).

Usage:
  coinops list [flags]

Flags:
  -u, --username string   Username for authentication (required)
  -p, --password string   Password for authentication (required)
  -h, --help              help for list
```

## Testing

The project includes comprehensive unit tests using Go's standard testing package with [testify](https://github.com/stretchr/testify) for assertions and [clockwork](https://github.com/jonboulle/clockwork) for time mocking.

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -cover ./...

# Run tests for a specific package
go test -v ./internal/server/...

# Run tests with coverage profile and view in browser
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Skip slow tests (e.g., the 3-second report generation)
go test -short ./...
```

### Test Coverage

All internal packages maintain 80%+ test coverage:

| Package | Coverage |
|---------|----------|
| `internal/version` | 100% |
| `internal/config` | 100% |
| `internal/middleware` | 100% |
| `internal/notifications` | 100% |
| `internal/session` | 93.5% |
| `internal/coingecko` | 93.9% |
| `internal/server` | 83.7% |
| `internal/math` | 81.8% |

### Test Structure

Tests are located alongside the code they test in `*_test.go` files:

```
internal/
├── config/
│   ├── config.go
│   └── config_test.go
├── coingecko/
│   ├── service.go
│   └── service_test.go
├── server/
│   ├── server.go
│   ├── server_test.go
│   ├── interfaces.go      # Interfaces for dependency injection
│   └── mocks_test.go      # Mock implementations for testing
...
```
