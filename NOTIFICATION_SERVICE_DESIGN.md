# FreeRangeNotify: Universal Notification Service

## Table of Contents
1. [Overview](#overview)
2. [Problem Statement](#problem-statement)
3. [Solution Architecture](#solution-architecture)
4. [System Design](#system-design)
5. [Database Schema (Elasticsearch)](#database-schema-elasticsearch)
6. [API Design](#api-design)
7. [Service Components](#service-components)
8. [Scalability & Performance](#scalability--performance)
9. [Security & Authentication](#security--authentication)
10. [Deployment Strategy](#deployment-strategy)
11. [Integration Guide](#integration-guide)
12. [Implementation Roadmap](#implementation-roadmap)

## Overview

FreeRangeNotify is a generic, pluggable notification service built in Go that provides real-time, multi-channel notification capabilities for any application. It supports push notifications, email, SMS, in-app notifications, and webhooks while maintaining high scalability and reliability.

## Problem Statement

Modern applications require sophisticated notification systems that can:
- Handle multiple notification channels (push, email, SMS, in-app)
- Support real-time delivery with high throughput
- Provide reliable delivery guarantees
- Scale horizontally to handle millions of users
- Offer easy integration for any application
- Support complex notification rules and preferences
- Maintain audit trails and analytics
- Handle notification templates and personalization

## Solution Architecture

### High-Level Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Client Apps   │    │   Admin Panel   │    │  Webhook APIs   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │   API Gateway   │
                    └─────────────────┘
                                 │
         ┌───────────────────────┼───────────────────────┐
         │                       │                       │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Notification    │    │ User Management │    │   Analytics     │
│    Service      │    │    Service      │    │   Service       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │  Message Queue  │
                    │   (Redis/Kafka) │
                    └─────────────────┘
                                 │
         ┌───────────────────────┼───────────────────────┐
         │                       │                       │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  Push Provider  │    │  Email Provider │    │   SMS Provider  │
│   (FCM/APNS)    │    │   (SendGrid)    │    │   (Twilio)      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                 │
                    ┌─────────────────┐
                    │  Elasticsearch  │
                    │    Cluster      │
                    └─────────────────┘
```

### Core Components

1. **API Gateway**: Entry point for all client requests
2. **Notification Service**: Core notification processing engine
3. **User Management Service**: Handles user preferences and device management
4. **Analytics Service**: Tracks delivery metrics and user engagement
5. **Message Queue**: Ensures reliable message processing
6. **Provider Services**: Integrates with external notification providers
7. **Elasticsearch**: Primary data store and search engine

## System Design

### Key Design Principles

1. **Plugin Architecture**: Modular design allowing easy addition of new notification channels
2. **Event-Driven**: Asynchronous processing with event sourcing
3. **Multi-Tenant**: Support for multiple applications with isolated data
4. **Fault Tolerant**: Graceful handling of failures with retry mechanisms
5. **Horizontally Scalable**: Stateless services that can scale independently

### Data Flow

```
Application → API Gateway → Notification Service → Queue → Provider → User
     ↓
Elasticsearch ← Analytics Service ← Delivery Status ← Provider Response
```

## Database Schema (Elasticsearch)

### Index Structure

#### 1. Applications Index (`applications`)
```json
{
  "mappings": {
    "properties": {
      "app_id": {"type": "keyword"},
      "app_name": {"type": "text"},
      "api_key": {"type": "keyword"},
      "webhook_url": {"type": "keyword"},
      "settings": {
        "type": "object",
        "properties": {
          "rate_limit": {"type": "integer"},
          "retry_attempts": {"type": "integer"},
          "default_template": {"type": "keyword"}
        }
      },
      "created_at": {"type": "date"},
      "updated_at": {"type": "date"}
    }
  }
}
```

#### 2. Users Index (`users`)
```json
{
  "mappings": {
    "properties": {
      "user_id": {"type": "keyword"},
      "app_id": {"type": "keyword"},
      "external_user_id": {"type": "keyword"},
      "email": {"type": "keyword"},
      "phone": {"type": "keyword"},
      "timezone": {"type": "keyword"},
      "language": {"type": "keyword"},
      "preferences": {
        "type": "object",
        "properties": {
          "email_enabled": {"type": "boolean"},
          "push_enabled": {"type": "boolean"},
          "sms_enabled": {"type": "boolean"},
          "quiet_hours": {
            "type": "object",
            "properties": {
              "start": {"type": "keyword"},
              "end": {"type": "keyword"}
            }
          }
        }
      },
      "devices": {
        "type": "nested",
        "properties": {
          "device_id": {"type": "keyword"},
          "platform": {"type": "keyword"},
          "token": {"type": "keyword"},
          "active": {"type": "boolean"}
        }
      },
      "created_at": {"type": "date"},
      "updated_at": {"type": "date"}
    }
  }
}
```

#### 3. Notifications Index (`notifications`)
```json
{
  "mappings": {
    "properties": {
      "notification_id": {"type": "keyword"},
      "app_id": {"type": "keyword"},
      "user_id": {"type": "keyword"},
      "template_id": {"type": "keyword"},
      "channel": {"type": "keyword"},
      "priority": {"type": "keyword"},
      "status": {"type": "keyword"},
      "content": {
        "type": "object",
        "properties": {
          "title": {"type": "text"},
          "body": {"type": "text"},
          "data": {"type": "object", "enabled": false}
        }
      },
      "scheduled_at": {"type": "date"},
      "sent_at": {"type": "date"},
      "delivered_at": {"type": "date"},
      "read_at": {"type": "date"},
      "error_message": {"type": "text"},
      "retry_count": {"type": "integer"},
      "created_at": {"type": "date"}
    }
  }
}
```

#### 4. Templates Index (`templates`)
```json
{
  "mappings": {
    "properties": {
      "template_id": {"type": "keyword"},
      "app_id": {"type": "keyword"},
      "name": {"type": "text"},
      "channel": {"type": "keyword"},
      "subject": {"type": "text"},
      "body": {"type": "text"},
      "variables": {"type": "keyword"},
      "created_at": {"type": "date"},
      "updated_at": {"type": "date"}
    }
  }
}
```

#### 5. Analytics Index (`analytics`)
```json
{
  "mappings": {
    "properties": {
      "event_id": {"type": "keyword"},
      "app_id": {"type": "keyword"},
      "notification_id": {"type": "keyword"},
      "user_id": {"type": "keyword"},
      "event_type": {"type": "keyword"},
      "channel": {"type": "keyword"},
      "timestamp": {"type": "date"},
      "metadata": {"type": "object", "enabled": false}
    }
  }
}
```

## API Design

### Authentication
All APIs use API Key authentication with rate limiting.

### Core Endpoints

#### 1. Application Management
```
POST   /v1/apps                    # Register new application
GET    /v1/apps/{app_id}           # Get application details
PUT    /v1/apps/{app_id}           # Update application settings
DELETE /v1/apps/{app_id}           # Delete application
```

#### 2. User Management
```
POST   /v1/users                   # Create/update user
GET    /v1/users/{user_id}         # Get user details
PUT    /v1/users/{user_id}         # Update user preferences
DELETE /v1/users/{user_id}         # Delete user
POST   /v1/users/{user_id}/devices # Register device token
```

#### 3. Notification APIs
```
POST   /v1/notifications           # Send notification
GET    /v1/notifications           # List notifications
GET    /v1/notifications/{id}      # Get notification details
PUT    /v1/notifications/{id}      # Update notification status
DELETE /v1/notifications/{id}      # Cancel scheduled notification
```

#### 4. Template Management
```
POST   /v1/templates               # Create template
GET    /v1/templates               # List templates
PUT    /v1/templates/{id}          # Update template
DELETE /v1/templates/{id}          # Delete template
```

#### 5. Analytics
```
GET    /v1/analytics/summary       # Get delivery summary
GET    /v1/analytics/events        # Get event timeline
GET    /v1/analytics/metrics       # Get performance metrics
```

### Sample API Requests

#### Send Notification
```json
POST /v1/notifications
{
  "users": ["user1", "user2"],
  "channels": ["push", "email"],
  "template_id": "welcome_template",
  "data": {
    "user_name": "John Doe",
    "action_url": "https://app.com/welcome"
  },
  "priority": "high",
  "scheduled_at": "2025-10-24T10:00:00Z"
}
```

#### Register User
```json
POST /v1/users
{
  "external_user_id": "app_user_123",
  "email": "user@example.com",
  "phone": "+1234567890",
  "timezone": "America/New_York",
  "preferences": {
    "email_enabled": true,
    "push_enabled": true,
    "quiet_hours": {
      "start": "22:00",
      "end": "08:00"
    }
  }
}
```

## Service Components

### 1. Core Services

#### Notification Service
- **Responsibility**: Process and route notifications
- **Features**: Template processing, channel selection, scheduling
- **Scaling**: Horizontal scaling with queue-based processing

#### User Service
- **Responsibility**: Manage user data and preferences
- **Features**: Device token management, preference handling
- **Scaling**: Read replicas for high-read workloads

#### Analytics Service
- **Responsibility**: Track and analyze notification metrics
- **Features**: Real-time dashboards, delivery tracking
- **Scaling**: Event streaming with time-series aggregation

### 2. Provider Integrations

#### Push Notifications
- **FCM (Firebase Cloud Messaging)**: Android notifications
- **APNS (Apple Push Notification Service)**: iOS notifications
- **Web Push**: Browser notifications

#### Email Providers
- **SendGrid**: Primary email provider
- **Amazon SES**: Backup email provider
- **Custom SMTP**: Enterprise integration

#### SMS Providers
- **Twilio**: Primary SMS provider
- **Amazon SNS**: Backup SMS provider

### 3. Supporting Services

#### Rate Limiter
- Per-application rate limiting
- Sliding window algorithm
- Redis-based implementation

#### Template Engine
- Mustache/Handlebars template processing
- Multi-language support
- Dynamic content insertion

#### Retry Manager
- Exponential backoff retry logic
- Dead letter queue handling
- Circuit breaker pattern

## Scalability & Performance

### Performance Targets
- **Throughput**: 100k+ notifications per second
- **Latency**: < 100ms API response time
- **Availability**: 99.9% uptime SLA
- **Durability**: Zero message loss guarantee

### Scaling Strategy

#### Horizontal Scaling
```
Load Balancer → API Gateway Cluster → Service Cluster → Provider Pool
                      ↓
                Queue Cluster → Elasticsearch Cluster
```

#### Caching Strategy
- **Redis**: Session data, rate limiting, temporary storage
- **Application Cache**: Template caching, user preferences
- **CDN**: Static assets, webhook responses

#### Database Optimization
- **Index Strategy**: Optimized for query patterns
- **Sharding**: Time-based and tenant-based sharding
- **Retention**: Automated data lifecycle management

## Security & Authentication

### API Security
- **API Key Authentication**: Per-application API keys
- **Rate Limiting**: Prevent abuse and ensure fair usage
- **Input Validation**: Comprehensive request validation
- **HTTPS Only**: All communication over TLS

### Data Security
- **Encryption**: Data encryption at rest and in transit
- **PII Protection**: Tokenization of sensitive data
- **Access Control**: Role-based access control
- **Audit Logging**: Complete audit trail

### Privacy Compliance
- **GDPR Compliance**: Data portability and deletion
- **Data Minimization**: Store only necessary data
- **Consent Management**: User consent tracking
- **Data Anonymization**: Analytics data anonymization

## Deployment Strategy

### Infrastructure
```yaml
# Kubernetes Deployment Structure
├── ingress/
│   ├── api-gateway-ingress.yaml
│   └── ssl-certificates.yaml
├── services/
│   ├── notification-service/
│   ├── user-service/
│   ├── analytics-service/
│   └── provider-services/
├── databases/
│   ├── elasticsearch-cluster.yaml
│   ├── redis-cluster.yaml
│   └── kafka-cluster.yaml
└── monitoring/
    ├── prometheus.yaml
    ├── grafana.yaml
    └── alertmanager.yaml
```

### Environment Configuration
- **Development**: Single-node Elasticsearch, local Redis
- **Staging**: Multi-node setup with production-like data
- **Production**: Full cluster with high availability

### CI/CD Pipeline
1. **Code Commit** → GitHub/GitLab
2. **Build** → Docker images with multi-stage builds
3. **Test** → Unit tests, integration tests, load tests
4. **Deploy** → Blue-green deployment with canary releases
5. **Monitor** → Health checks and performance monitoring

## Integration Guide

### Quick Start Integration

#### 1. Register Application
```bash
curl -X POST https://api.freerangenotify.com/v1/apps \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "app_name": "My Application",
    "webhook_url": "https://myapp.com/notifications/webhook"
  }'
```

#### 2. SDK Integration (Go)
```go
package main

import (
    "github.com/freerangenotify/go-sdk"
)

func main() {
    client := notify.NewClient("YOUR_API_KEY")
    
    // Register user
    user := &notify.User{
        ExternalUserID: "user123",
        Email: "user@example.com",
        Preferences: notify.Preferences{
            EmailEnabled: true,
            PushEnabled: true,
        },
    }
    client.Users.Create(user)
    
    // Send notification
    notification := &notify.Notification{
        Users: []string{"user123"},
        Channels: []string{"push", "email"},
        TemplateID: "welcome",
        Data: map[string]interface{}{
            "user_name": "John Doe",
        },
    }
    client.Notifications.Send(notification)
}
```

#### 3. Webhook Handling
```go
func handleWebhook(w http.ResponseWriter, r *http.Request) {
    var event notify.WebhookEvent
    if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
        http.Error(w, "Invalid JSON", 400)
        return
    }
    
    switch event.Type {
    case "notification.delivered":
        // Handle delivery confirmation
    case "notification.clicked":
        // Handle user engagement
    case "notification.failed":
        // Handle delivery failure
    }
    
    w.WriteHeader(200)
}
```

### Language SDKs
- **Go**: Native SDK with full feature support
- **Node.js**: JavaScript/TypeScript SDK
- **Python**: Python SDK with async support
- **Java**: Java SDK for enterprise applications
- **PHP**: PHP SDK for web applications

## Implementation Roadmap

### Phase 1: Core Foundation (Weeks 1-4)

#### Week 1: Project Setup ✅
- [x] Initialize Go project with modules
  ```bash
  go mod init github.com/the-monkeys/freerangenotify
  ```
- [x] Setup project structure with clean architecture
- [x] Configure Docker and Docker Compose
- [x] Setup Elasticsearch with Docker
- [x] Create basic configuration management
- [x] Initialize Git repository with proper .gitignore

#### Week 2: Database Foundation ✅
- [x] Setup Elasticsearch client connection
- [x] Create index templates for all entities
- [x] Implement base repository pattern
- [x] Create migration scripts for indices
- [x] Setup connection pooling and health checks
- [x] Implement basic CRUD operations

#### Week 3: Core Services Architecture ✅
- [x] Implement dependency injection container
- [x] Create base service interfaces
- [x] Setup HTTP server with Fiber framework (faster than Gin)
- [x] Implement middleware (logging, CORS, auth)
- [x] Create error handling system
- [x] Setup configuration with Viper

#### Week 4: User Management & Basic API ✅
- [x] Implement User service with full CRUD
- [x] Create Application management service
- [x] Build REST API endpoints for users (18 endpoints total)
- [x] Implement API key authentication with Bearer tokens
- [x] Add input validation with struct tags
- [x] Create Swagger/OpenAPI 3.0 documentation
- [x] Integrate Swagger UI for interactive API testing
- [x] Fix Elasticsearch keyword field queries (api_key.keyword, app_id.keyword)
- [x] Fix UpdateSettings bug (enable_webhooks, enable_analytics)
- [x] Add /metrics endpoint for Prometheus
- [x] Configure Docker to include docs/ directory
- [x] Create comprehensive integration tests (27 tests, 100% pass)

### Phase 2: Multi-Channel Support (Weeks 5-8)

#### Week 5: Notification Core Engine
**Day 1 (Dec 1, 2025)** ✅
- [x] Created notification domain models with complete entity structure
- [x] Implemented Channel enum (push, email, sms, webhook, in_app)
- [x] Implemented Priority enum (low, normal, high, critical)
- [x] Implemented Status enum (pending, queued, processing, sent, delivered, read, failed, cancelled)
- [x] Added validation methods for all enums (Valid(), String(), IsFinal())
- [x] Created helper methods (CanRetry(), IsScheduled())
- [x] Defined Repository interface with 7 core methods
- [x] Defined Service interface for business logic
- [x] Created comprehensive unit tests (12 test functions, 55 subtests, 100% pass)

**Day 2-7 (Completed)** ✅
- [x] Design notification processing pipeline
- [x] Implement Elasticsearch repository with optimized mappings
- [x] Implement NotificationService interface
- [x] Create channel abstraction layer
- [x] Build notification queuing system with Redis
- [x] Implement priority-based processing
- [x] Add notification status tracking
- [x] Prometheus metrics integration
- [x] End-to-end integration tests

#### Week 6: Push Notification Provider ✅
- [x] Integrate Firebase Cloud Messaging (FCM)
- [x] Implement APNS for iOS notifications
- [x] Create device token management
- [x] Build push notification templates
- [x] Add platform-specific payload handling
- [x] Implement delivery status tracking
- [x] Provider manager for routing

#### Week 7: Email & SMS Providers ✅
- [x] Integrate SendGrid for email delivery
- [x] Implement Twilio for SMS notifications
- [x] Create provider abstraction interface
- [x] Build email template system with HTML/text
- [x] Add attachment support for emails (prepared)
- [x] Implement SMS template with variables
- [x] All providers integrated in worker

#### Week 8: Template Management System ✅
- [x] Design template storage in Elasticsearch
- [x] Template domain models with versioning & localization
- [x] Implement template CRUD operations
- [x] Create template rendering engine (Go text/template)
- [x] Build template validation system
- [x] Template HTTP API endpoints (8 endpoints)
- [x] Template integration tests (12 test scenarios)

### Phase 3: Advanced Features (Weeks 9-12)

#### Week 9: Queue System Implementation
- [x] Setup Redis for queue management
- [x] Implement job queue with priorities
- [x] Create worker pool for parallel processing
- [x] Add job retry mechanisms with exponential backoff
- [x] Implement dead letter queue handling
- [x] Build queue monitoring and metrics

#### Week 10: Scheduled Notifications
- [x] Design cron-based scheduling system
- [x] Implement time zone handling
- [ ] Create recurring notification patterns
- [x] Build scheduling validation
- [x] Add schedule modification capabilities
- [ ] Implement bulk scheduling operations

#### Week 11: User Preferences & Rules
- [x] Implement user preference management
- [x] Create quiet hours functionality
- [x] Build notification frequency controls
- [x] Add channel preference per notification type
- [x] Implement do-not-disturb settings
- [ ] Create preference inheritance system

#### Week 12: Rate Limiting & Error Handling
- [x] Implement sliding window rate limiting
- [x] Create circuit breaker pattern for providers
- [x] Build comprehensive error categorization
- [x] Add automatic retry with jitter
- [x] Implement graceful degradation
- [x] Create health check endpoints

### Phase 4: Scalability & Monitoring (Weeks 13-16)

#### Week 13: Horizontal Scaling
- [ ] Implement stateless service design
- [ ] Create load balancer configuration
- [ ] Setup database connection pooling
- [ ] Implement distributed locking with Redis
- [ ] Add service discovery mechanism
- [ ] Create auto-scaling policies

#### Week 14: Performance Optimization
- [ ] Implement caching strategies (Redis)
- [ ] Optimize Elasticsearch queries
- [ ] Add connection pooling for HTTP clients
- [ ] Implement bulk operations for efficiency
- [ ] Create performance benchmarking suite
- [ ] Optimize memory usage and GC

#### Week 15: Monitoring & Observability
- [ ] Setup Prometheus metrics collection
- [ ] Implement structured logging with Zap
- [ ] Create Grafana dashboards
- [ ] Add distributed tracing with Jaeger
- [ ] Implement alerting rules
- [ ] Create SLA monitoring

#### Week 16: Analytics & Reporting
- [ ] Build real-time analytics pipeline
- [ ] Implement delivery rate tracking
- [ ] Create engagement metrics collection
- [ ] Build analytics API endpoints
- [ ] Add data visualization components
- [ ] Implement data retention policies

### Phase 5: Production Ready (Weeks 17-20)

#### Week 17: Security Hardening
- [ ] Implement HTTPS/TLS everywhere
- [ ] Add input sanitization and validation
- [ ] Create API rate limiting per client
- [ ] Implement audit logging
- [ ] Add secrets management (Vault/K8s secrets)
- [ ] Conduct security vulnerability scanning

#### Week 18: Webhook System & Compliance
- [ ] Build webhook delivery system
- [ ] Implement webhook retry mechanisms
- [ ] Add GDPR compliance features
- [ ] Create data export/import functionality
- [ ] Implement data anonymization
- [ ] Add consent management system

#### Week 19: SDK Development & Documentation
- [ ] Create Go SDK with full feature support
- [ ] Build JavaScript/Node.js SDK
- [ ] Implement Python SDK
- [ ] Create comprehensive API documentation
- [ ] Build integration examples
- [ ] Add SDK testing and CI/CD

#### Week 20: Production Deployment
- [ ] Create Kubernetes deployment manifests
- [ ] Setup production Elasticsearch cluster
- [ ] Implement backup and disaster recovery
- [ ] Create deployment automation (GitOps)
- [ ] Conduct load testing and optimization
- [ ] Setup production monitoring and alerting

### Phase 6: Extended Provider Support (Week 21+)

#### SMTP Provider (Gmail Integration)
- [x] Implement generic SMTP provider in `smtp_provider.go`
- [x] Support SASL authentication (Plain, Login, CRAM-MD5)
- [x] Configure specialized Gmail support (App Passwords)
- [x] Add HTML and text body support with attachments

#### Webhook Provider
- [x] Implement generic Webhook provider in `webhook_provider.go`
- [x] Secure POST requests with signature verification (HMAC)
- [x] Configurable timeouts and retry policies
- [x] Support custom headers and payload structures

## Detailed Implementation Steps

### Step 1: Project Structure Setup

Create the following directory structure:
```
FreeRangeNotify/
├── cmd/
│   ├── server/
│   │   └── main.go
│   ├── worker/
│   │   └── main.go
│   └── migrate/
│       └── main.go
├── internal/
│   ├── config/
│   ├── domain/
│   │   ├── user/
│   │   ├── notification/
│   │   ├── template/
│   │   └── analytics/
│   ├── infrastructure/
│   │   ├── database/
│   │   ├── queue/
│   │   ├── providers/
│   │   └── cache/
│   ├── interfaces/
│   │   ├── http/
│   │   ├── grpc/
│   │   └── webhook/
│   └── usecases/
├── pkg/
│   ├── logger/
│   ├── validator/
│   ├── errors/
│   └── utils/
├── api/
│   ├── openapi/
│   └── proto/
├── deployments/
│   ├── docker/
│   ├── kubernetes/
│   └── terraform/
├── scripts/
├── docs/
└── tests/
    ├── integration/
    └── load/
```

### Step 2: Essential Go Dependencies

Add these dependencies to your `go.mod`:
```go
require (
    github.com/gofiber/fiber/v2 v2.52.9
    github.com/elastic/go-elasticsearch/v8 v8.11.0
    github.com/go-redis/redis/v8 v8.11.5
    github.com/spf13/viper v1.17.0
    github.com/spf13/cobra v1.7.0
    go.uber.org/zap v1.26.0
    github.com/golang-jwt/jwt/v5 v5.0.0
    github.com/google/uuid v1.4.0
    github.com/stretchr/testify v1.8.4
    github.com/prometheus/client_golang v1.17.0
    github.com/opentracing/opentracing-go v1.2.0
    github.com/segmentio/kafka-go v0.4.47
)
```

### Step 3: Core Domain Models

Start with these essential domain models:

```go
// internal/domain/notification/models.go
type Notification struct {
    ID           string                 `json:"id"`
    AppID        string                 `json:"app_id"`
    UserID       string                 `json:"user_id"`
    TemplateID   string                 `json:"template_id"`
    Channel      Channel                `json:"channel"`
    Priority     Priority               `json:"priority"`
    Status       Status                 `json:"status"`
    Content      Content                `json:"content"`
    Metadata     map[string]interface{} `json:"metadata"`
    ScheduledAt  *time.Time             `json:"scheduled_at"`
    SentAt       *time.Time             `json:"sent_at"`
    DeliveredAt  *time.Time             `json:"delivered_at"`
    CreatedAt    time.Time              `json:"created_at"`
}

type Channel string
const (
    ChannelPush  Channel = "push"
    ChannelEmail Channel = "email"
    ChannelSMS   Channel = "sms"
    ChannelWebhook Channel = "webhook"
)

type Priority string
const (
    PriorityLow    Priority = "low"
    PriorityNormal Priority = "normal" 
    PriorityHigh   Priority = "high"
    PriorityCritical Priority = "critical"
)
```

### Step 4: Repository Interface Pattern

```go
// internal/domain/notification/repository.go
type Repository interface {
    Create(ctx context.Context, notification *Notification) error
    GetByID(ctx context.Context, id string) (*Notification, error)
    Update(ctx context.Context, notification *Notification) error
    List(ctx context.Context, filter Filter) ([]*Notification, error)
    Delete(ctx context.Context, id string) error
}

// internal/infrastructure/database/elasticsearch/notification_repository.go
type NotificationRepository struct {
    client *elasticsearch.Client
    index  string
}

func (r *NotificationRepository) Create(ctx context.Context, notification *Notification) error {
    // Implementation with Elasticsearch
}
```

### Step 5: Service Layer Implementation

```go
// internal/usecases/notification_service.go
type NotificationService struct {
    repo     notification.Repository
    queue    queue.Queue
    providers map[Channel]Provider
    logger   logger.Logger
}

func (s *NotificationService) Send(ctx context.Context, req SendRequest) error {
    // 1. Validate request
    // 2. Create notification record
    // 3. Enqueue for processing
    // 4. Return immediately (async processing)
}

func (s *NotificationService) Process(ctx context.Context, notification *Notification) error {
    // 1. Get user preferences
    // 2. Select appropriate provider
    // 3. Send notification
    // 4. Update status
    // 5. Track analytics
}
```

### Step 6: Provider Interface Implementation

```go
// internal/infrastructure/providers/provider.go
type Provider interface {
    Send(ctx context.Context, notification *Notification) (*Result, error)
    GetName() string
    IsHealthy(ctx context.Context) bool
}

// internal/infrastructure/providers/fcm/provider.go
type FCMProvider struct {
    client *messaging.Client
    config Config
}

func (p *FCMProvider) Send(ctx context.Context, notification *Notification) (*Result, error) {
    // FCM implementation
}
```

### Step 7: HTTP API Implementation

```go
// internal/interfaces/http/notification_handler.go
type NotificationHandler struct {
    service usecases.NotificationService
}

func (h *NotificationHandler) Send(c *fiber.Ctx) error {
    var req SendNotificationRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": err.Error()})
    }
    
    result, err := h.service.Send(c.Context(), req)
    if err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }
    
    return c.JSON(result)
}
```

### Step 8: Configuration Management

```go
// internal/config/config.go
type Config struct {
    Server      ServerConfig      `mapstructure:"server"`
    Database    DatabaseConfig    `mapstructure:"database"`
    Redis       RedisConfig       `mapstructure:"redis"`
    Providers   ProvidersConfig   `mapstructure:"providers"`
    Queue       QueueConfig       `mapstructure:"queue"`
    Monitoring  MonitoringConfig  `mapstructure:"monitoring"`
}

func Load() (*Config, error) {
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath("./config")
    viper.AddConfigPath(".")
    
    if err := viper.ReadInConfig(); err != nil {
        return nil, err
    }
    
    var config Config
    if err := viper.Unmarshal(&config); err != nil {
        return nil, err
    }
    
    return &config, nil
}
```

### Step 9: Testing Strategy

```go
// tests/integration/notification_test.go
func TestNotificationEndToEnd(t *testing.T) {
    // Setup test environment
    testDB := setupTestElasticsearch(t)
    testRedis := setupTestRedis(t)
    
    // Create test server
    server := setupTestServer(testDB, testRedis)
    
    // Test notification sending
    resp := sendTestNotification(t, server)
    assert.Equal(t, 200, resp.StatusCode)
    
    // Verify notification was queued
    verifyNotificationQueued(t, testRedis)
    
    // Process queue and verify delivery
    processQueue(t, server)
    verifyNotificationDelivered(t, testDB)
}
```

### Step 10: Docker Setup

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o server ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .
COPY --from=builder /app/config ./config

EXPOSE 8080
CMD ["./server"]
```

```yaml
# docker-compose.yml
version: '3.8'
services:
  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.11.0
    environment:
      - discovery.type=single-node
      - xpack.security.enabled=false
    ports:
      - "9200:9200"
    
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
      
  notification-service:
    build: .
    ports:
      - "8080:8080"
    depends_on:
      - elasticsearch
      - redis
    environment:
      - ELASTICSEARCH_URL=http://elasticsearch:9200
      - REDIS_URL=redis://redis:6379
```

## Conclusion

FreeRangeNotify is designed to be the universal notification service that any application can integrate with minimal effort. By leveraging Elasticsearch for storage and search capabilities, Go for high-performance service implementation, and a microservices architecture for scalability, this service can handle the notification needs of applications ranging from small startups to large enterprises.

The modular design ensures that new notification channels can be easily added, while the comprehensive API and SDK support make integration straightforward for developers using any technology stack.