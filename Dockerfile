# Build stage
FROM golang:1.23-alpine AS builder

# Install git for version detection
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files first for better caching
COPY config.yaml go.mod go.sum ./
RUN go mod download

# Copy source code (including .git for version detection)
COPY . .

# Build static binary with version information
# Version info is computed from git if available, otherwise uses defaults
# Uses git commit timestamp (not build time) for reproducible builds
RUN VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev") && \
    COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") && \
    COMMIT_DATE=$(git log -1 --format=%cI 2>/dev/null || echo "unknown") && \
    echo "Building version=${VERSION} commit=${COMMIT} commit_date=${COMMIT_DATE}" && \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w \
    -X pahg-template/internal/version.Version=${VERSION} \
    -X pahg-template/internal/version.Commit=${COMMIT} \
    -X pahg-template/internal/version.CommitDate=${COMMIT_DATE}" \
    -o /coinops ./cmd/coinops

# Runtime stage - distroless static image for Go binaries
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy with proper ownership for nonroot user (UID 65532)
COPY --from=builder --chown=nonroot:nonroot /coinops /app/coinops
COPY --from=builder --chown=nonroot:nonroot /app/config.yaml /app/config.yaml

EXPOSE 3000

ENTRYPOINT ["/app/coinops"]
CMD ["serve", "--host", "0.0.0.0", "--config", "/app/config.yaml"]
