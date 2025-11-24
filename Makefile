# Makefile for FreeRangeNotify

.PHONY: help build run test test-integration test-unit clean docker-up docker-down docker-logs

# Default target
help:
	@echo "Available targets:"
	@echo "  build              - Build the application binary"
	@echo "  run                - Run the application locally"
	@echo "  test               - Run all tests (unit + integration)"
	@echo "  test-unit          - Run unit tests only"
	@echo "  test-integration   - Run integration tests"
	@echo "  test-coverage      - Run tests with coverage report"
	@echo "  docker-build       - Build Docker image"
	@echo "  docker-up          - Start all services with Docker Compose"
	@echo "  docker-down        - Stop all services"
	@echo "  docker-logs        - View logs from all services"
	@echo "  docker-clean       - Stop services and remove volumes"
	@echo "  clean              - Clean build artifacts"
	@echo "  lint               - Run linters"
	@echo "  fmt                - Format code"

# Build the application
build:
	@echo "Building application..."
	go build -o bin/server ./cmd/server

# Run the application locally
run:
	@echo "Running application..."
	go run ./cmd/server

# Run all tests
test: test-unit test-integration

# Run unit tests only
test-unit:
	@echo "Running unit tests..."
	go test -v -short ./...

# Run integration tests
test-integration:
	@echo "Starting Docker services..."
	docker-compose up -d
	@echo "Waiting for services to be ready..."
	timeout /t 15 /nobreak
	@echo "Running integration tests..."
	go test -v ./tests/integration/...
	@echo "Stopping Docker services..."
	docker-compose down

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker-compose build

# Start all services
docker-up:
	@echo "Starting all services..."
	docker-compose up -d
	@echo "Services started. Check status with 'make docker-logs'"

# Stop all services
docker-down:
	@echo "Stopping all services..."
	docker-compose down

# View logs
docker-logs:
	docker-compose logs -f

# Clean up everything including volumes
docker-clean:
	@echo "Cleaning up Docker resources..."
	docker-compose down -v
	@echo "Cleaned up successfully"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	if exist bin rmdir /s /q bin
	if exist coverage.out del coverage.out
	if exist coverage.html del coverage.html
	@echo "Cleaned up successfully"

# Run linters
lint:
	@echo "Running linters..."
	golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

# Install development dependencies
install-dev:
	@echo "Installing development dependencies..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "Development dependencies installed"

# Run services in development mode
dev:
	@echo "Starting services in development mode..."
	docker-compose up

# Quick test - run a subset of tests for fast feedback
test-quick:
	@echo "Running quick tests..."
	go test -v -short -timeout 30s ./internal/...

# Generate mocks (when needed)
generate:
	@echo "Generating mocks and code..."
	go generate ./...

# Database migration (when implemented)
migrate-up:
	@echo "Running database migrations..."
	# Add migration command here when implemented

migrate-down:
	@echo "Rolling back database migrations..."
	# Add migration rollback command here when implemented
