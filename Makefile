# Build variables
BINARY_NAME=coinops
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
# Use git commit timestamp for reproducible builds (not build time)
COMMIT_DATE=$(shell git log -1 --format=%cI 2>/dev/null || echo "unknown")

# Ldflags to inject version information
VERSION_LDFLAGS=-X pahg-template/internal/version.Version=$(VERSION) \
	-X pahg-template/internal/version.Commit=$(COMMIT) \
	-X pahg-template/internal/version.CommitDate=$(COMMIT_DATE)

# Build the binary
.PHONY: build
build:
	@echo "Building $(BINARY_NAME) $(VERSION) ($(COMMIT))..."
	go build -ldflags "$(VERSION_LDFLAGS)" -o $(BINARY_NAME) ./cmd/coinops

# Build with optimizations (smaller binary)
.PHONY: build-release
build-release:
	@echo "Building $(BINARY_NAME) $(VERSION) ($(COMMIT)) for release..."
	CGO_ENABLED=0 go build -ldflags "-s -w $(VERSION_LDFLAGS)" -o $(BINARY_NAME) ./cmd/coinops

# Cross-compile for Linux (amd64)
.PHONY: build-linux
build-linux:
	@echo "Cross-compiling $(BINARY_NAME) $(VERSION) for linux/amd64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w $(VERSION_LDFLAGS)" -o $(BINARY_NAME)-linux-amd64 ./cmd/coinops

# Cross-compile for Linux (arm64)
.PHONY: build-linux-arm64
build-linux-arm64:
	@echo "Cross-compiling $(BINARY_NAME) $(VERSION) for linux/arm64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-s -w $(VERSION_LDFLAGS)" -o $(BINARY_NAME)-linux-arm64 ./cmd/coinops

# Cross-compile for all supported platforms
.PHONY: build-all
build-all: build-linux build-linux-arm64
	@echo "Built binaries for all platforms"

# Run the application
.PHONY: run
run: build
	./$(BINARY_NAME) serve

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME) $(BINARY_NAME)-linux-amd64 $(BINARY_NAME)-linux-arm64

# Show version information
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Commit Date: $(COMMIT_DATE)"

# Build Docker image (version info auto-detected from git)
.PHONY: docker-build
docker-build:
	@echo "Building Docker image (version auto-detected from git)..."
	docker build \
		-t $(BINARY_NAME):$(VERSION) \
		-t $(BINARY_NAME):latest \
		.

# Run tests
.PHONY: test
test:
	go test -v ./...

# Install dependencies
.PHONY: deps
deps:
	go mod download
	go mod tidy

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Run linter
.PHONY: lint
lint:
	golangci-lint run

# Default target
.DEFAULT_GOAL := build
