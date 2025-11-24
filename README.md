# FreeRangeNotify - Universal Notification Service

A generic, pluggable notification service built in Go that provides real-time, multi-channel notification capabilities for any application.

## Quick Links
- 📖 **[API Documentation](http://localhost:8080/swagger/index.html)** - Interactive Swagger UI
- 📋 **[OpenAPI Spec](./docs/openapi/swagger.yaml)** - Full API specification
- 🏗️ **[Architecture Design](./NOTIFICATION_SERVICE_DESIGN.md)** - System design document
- 📚 **[Development Guide](./.copilot-instructions.md)** - Complete project guide

## Getting Started

### Prerequisites
- Go 1.24+
- Docker and Docker Compose
- Elasticsearch 8.11.0
- Redis 7.x

### Development Setup
```bash
# 1. Clone the repository
git clone https://github.com/the-monkeys/freerangenotify.git
cd freerangenotify

# 2. Install dependencies
go mod tidy

# 3. Start services
docker-compose up -d

# 4. Wait for services to be healthy (check logs)
docker-compose logs -f notification-service

# 5. Access the API
curl http://localhost:8080/health

# 6. View API documentation
open http://localhost:8080/swagger/index.html
```

## API Endpoints

### System
- `GET /health` - Service health check
- `GET /metrics` - Prometheus metrics
- `GET /swagger/*` - API documentation

### Applications (Public)
- `POST /v1/apps` - Create application (get API key)
- `GET /v1/apps` - List applications
- `PUT /v1/apps/{id}/settings` - Update settings

### Users (Protected - requires API key)
- `POST /v1/users` - Create user
- `GET /v1/users` - List users
- `POST /v1/users/{id}/devices` - Add device
- `PUT /v1/users/{id}/preferences` - Update preferences

See [Swagger UI](http://localhost:8080/swagger/index.html) for complete documentation.

## Example Usage

```bash
# 1. Create an application
curl -X POST http://localhost:8080/v1/apps \
  -H "Content-Type: application/json" \
  -d '{"app_name":"MyApp","description":"My application"}'

# Response includes api_key: frn_xxxxx

# 2. Create a user
curl -X POST http://localhost:8080/v1/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer frn_xxxxx" \
  -d '{"external_user_id":"user123","email":"user@example.com","name":"John Doe"}'

# 3. Add device token
curl -X POST http://localhost:8080/v1/users/{user_id}/devices \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer frn_xxxxx" \
  -d '{"token":"device_token","platform":"ios","device_name":"iPhone 15"}'
```

## Features
✅ **Implemented**
- Multi-tenant application management
- User management with preferences
- Device registration (iOS/Android/Web)
- API key authentication
- Swagger/OpenAPI documentation
- Docker development environment
- Elasticsearch data storage
- Prometheus metrics

🚧 **In Progress**
- Multi-channel notifications (Push, Email, SMS)
- Template management system
- Scheduled notifications
- Webhook callbacks

📋 **Planned**
- Analytics and reporting
- Rate limiting per application
- Retry mechanisms
- Production deployment guides

## Architecture
Built using Clean Architecture principles with:
- **Framework**: Go 1.24 + Fiber v2.52.9
- **Database**: Elasticsearch 8.11.0
- **Cache/Queue**: Redis 7.x
- **Monitoring**: Prometheus + Grafana
- **Documentation**: Swagger/OpenAPI 3.0

For detailed architecture, see **[NOTIFICATION_SERVICE_DESIGN.md](./NOTIFICATION_SERVICE_DESIGN.md)**

For development guidelines, see **[.copilot-instructions.md](./.copilot-instructions.md)**