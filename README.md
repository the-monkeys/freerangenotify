# FreeRangeNotify - Universal Notification Service

A generic, pluggable notification service built in Go that provides real-time, multi-channel notification capabilities for any application.

## Project Structure

```
FreeRangeNotify/
├── cmd/                    # Application entry points
│   ├── server/            # HTTP server main
│   ├── worker/            # Background worker main
│   └── migrate/           # Database migration tool
├── internal/              # Private application code
│   ├── config/           # Configuration management
│   ├── domain/           # Business logic and entities
│   │   ├── user/         # User domain
│   │   ├── notification/ # Notification domain
│   │   ├── template/     # Template domain
│   │   └── analytics/    # Analytics domain
│   ├── infrastructure/   # External dependencies
│   │   ├── database/     # Database implementations
│   │   ├── queue/        # Queue implementations
│   │   ├── providers/    # External service providers
│   │   └── cache/        # Cache implementations
│   ├── interfaces/       # Interface adapters
│   │   ├── http/         # HTTP handlers
│   │   ├── grpc/         # gRPC handlers
│   │   └── webhook/      # Webhook handlers
│   └── usecases/         # Application business logic
├── pkg/                  # Public packages
│   ├── logger/          # Logging utilities
│   ├── validator/       # Validation utilities
│   ├── errors/          # Error handling
│   └── utils/           # Common utilities
├── api/                 # API definitions
│   ├── openapi/         # OpenAPI/Swagger specs
│   └── proto/           # Protocol buffer definitions
├── deployments/         # Deployment configurations
│   ├── docker/          # Docker files
│   ├── kubernetes/      # Kubernetes manifests
│   └── terraform/       # Infrastructure as code
├── scripts/             # Build and utility scripts
├── docs/                # Documentation
├── tests/              # Test files
│   ├── integration/    # Integration tests
│   └── load/           # Load tests
└── config/             # Configuration files
```

## Getting Started

### Prerequisites
- Go 1.21+
- Docker and Docker Compose
- Elasticsearch 8.x
- Redis 7.x

### Development Setup
1. Clone the repository
2. Install dependencies: `go mod tidy`
3. Start services: `docker-compose up -d`
4. Run the server: `go run cmd/server/main.go`

## Features
- Multi-channel notifications (Push, Email, SMS, Webhook)
- Real-time delivery with high throughput
- Template management system
- User preference handling
- Analytics and reporting
- Horizontal scalability
- Production-ready monitoring

## Architecture
Built using Clean Architecture principles with:
- Domain-driven design
- Repository pattern
- Dependency injection
- Event-driven processing
- Microservices architecture