# FreeRangeNotify - Developer Reference Document

## Project Overview
FreeRangeNotify is a high-performance, multi-tenant notification service built in Go (Fiber v2) with a React (Vite/TypeScript) frontend. It supports multi-channel delivery (Push, Email, SMS, Webhook, SSE, Slack, Discord, WhatsApp) and follows a Clean Architecture/DDD structure.

## Tech Stack
- **Backend**: Go 1.24, Fiber v2
- **Database**: Elasticsearch 8.11.0 (Primary storage for notifications, users, apps, templates)
- **Cache/Queue**: Redis 7.x (Queueing, rate limiting, SSE pub/sub, activity feed)
- **Frontend**: React 19, Vite, TypeScript, Radix UI, Tailwind CSS
- **Monitoring**: Prometheus (Metrics), Zap (Logging)
- **Infrastructure**: Docker, Docker Compose

## Architecture & Core Components

### 1. Backend Structure (`internal/`)
- **`domain/`**: Business entities and repository/service interfaces.
  - `application`: Multi-tenant app management, API keys, settings.
  - `notification`: Core notification entity, channels, priorities, statuses.
  - `user`: Subscriber management, preferences, device tokens.
  - `workflow`: Multi-step delivery pipelines (Phase 1).
  - `template`: Handlebars-like template management with versioning.
- **`usecases/`**: Application logic implementations.
  - `NotificationService`: Handles validation, quota checks, and enqueuing.
- **`infrastructure/`**: Implementation of external interfaces.
  - `repository/`: Elasticsearch-backed repositories.
  - `queue/`: Redis-backed reliable queue.
  - `providers/`: Delivery channel implementations (SMTP, FCM, APNS, etc.).
  - `orchestrator/`: Engines for Workflows and Digests.
- **`interfaces/http/`**: Fiber handlers, routes, and DTOs.
  - `handlers/`: Individual controllers for each resource.
  - `middleware/`: Auth (JWT/API Key), RBAC, Error Handling, Auditing.

### 2. Worker Process (`cmd/worker/`)
The worker is responsible for processing the notification queue:
1. Dequeues `NotificationQueueItem`.
2. Fetches full `Notification` from Elasticsearch.
3. Applies template rendering.
4. Checks for Digests, Throttling, and User Preferences.
5. Routes to the appropriate `Provider` via `ProviderManager`.
6. Handles retries with exponential backoff.
7. Publishes real-time status updates to Redis.

### 3. Frontend (`ui/`)
- Uses `react-router-dom` for navigation.
- Dashboard for managing Apps, Templates, Users, Workflows, and Audit Logs.
- Real-time activity feed via SSE.
- Public documentation hub and API reference.

## Feature Phases
The project is organized into several phases, many of which are controlled by feature flags in `config/config.yaml`:
- **Phase 1**: Workflows, Digests, SSE HMAC.
- **Phase 2**: Topics, Subscriber Throttling, Audit Logs, RBAC.
- **Phase 3**: Custom Providers, Slack/Discord/WhatsApp integrations.
- **Phase 4**: Template Versioning, Snooze/Archive.
- **Phase 6**: Multi-Environment support.

## Critical Paths

### Notification Delivery Flow
1. `POST /v1/notifications` → `NotificationHandler` → `NotificationService.Send`.
2. `NotificationService`: Resolve user/template → Check limits/DND → Save to ES → Enqueue to Redis.
3. `NotificationWorker`: Dequeue → Fetch from ES → Render Template → `ProviderManager.Send` → Update Status.
4. Real-time: Status update → Redis Pub/Sub → `SSEBroadcaster` → Browser.

### Authentication
- **User/Admin**: JWT-based (stored in cookies/localStorage).
- **API (from customer apps)**: API Key (`X-API-Key` or `Bearer` token).
- **SSO**: OIDC integration (Monkeys Identity).

## Deployment & Development
- **Local**: `docker-compose up -d` starts ES, Redis, Backend, Worker, and UI.
- **Configuration**: Managed via `config/config.yaml` and environment variables (prefixed with `FREERANGE_`).
- **Migrations**: `cmd/migrate` handles Elasticsearch index templates and initial setup.

## Current State & Known Issues
- Customer complaints in production suggest a need for improved smoothness, security, and ease of use.
- Features like Analytics and Rate Limiting are marked as "Planned" or "In Progress" in some docs.
- The UI contains many components for advanced features (Workflows, Digests) that may need polish.

## Developer Tips
- Always check feature flags in `config.yaml` when debugging new modules.
- Elasticsearch is used for both transactional data and analytics; ensure index templates are up to date.
- Use the `playground` in the UI to test SSE and Webhooks.
