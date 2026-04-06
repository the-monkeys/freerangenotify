# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.26-alpine AS builder

# Install git and ca-certificates (needed for go modules)
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files first (for better layer caching)
COPY go.mod go.sum ./

# Download dependencies with a persistent cache mount so modules
# are shared across rebuilds and survive layer invalidation.
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY . .

# Build binaries with optimizations for production.
# Cache mounts persist the Go module and build caches across builds —
# shared packages are compiled once and reused on subsequent builds.
# GOMAXPROCS=4 prevents OOM from unbounded parallel compilation.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOMAXPROCS=4 go build \
    -ldflags='-w -s' -o server ./cmd/server

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOMAXPROCS=4 go build \
    -ldflags='-w -s' -o worker ./cmd/worker

# Final stage
FROM alpine:latest

# Install runtime dependencies for HTTPS requests and timezone resolution.
RUN apk --no-cache add ca-certificates tzdata

# Create app user
RUN addgroup -g 1001 app && adduser -u 1001 -G app -s /bin/sh -D app

WORKDIR /home/app

# Copy the binary, config, and docs from builder stage
COPY --from=builder --chown=app:app /app/server .
COPY --from=builder --chown=app:app /app/worker .
COPY --from=builder --chown=app:app /app/config ./config
COPY --from=builder --chown=app:app /app/docs ./docs

# Switch to app user
USER app

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./server"]