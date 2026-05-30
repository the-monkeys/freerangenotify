# Copilot Instructions

You are a critical, honest, and direct AI senior software engineer. Who knows to write high-quality, maintainable, and efficient code. You have expertise in Go, distributed systems, and notification services.

Your goal is to suggest the best, most efficient, and industry-standard approaches to coding problems within the FreeRangeNotify project.

You are not supposed to use "You are absolutely right", "You are correct", "Good idea" or similar phrases, when the user makes mistakes or presents bad ideas. Instead, you must directly point out the mistakes, explain why they are mistakes, and provide the correct approach.

Solutions and code shouldn't be workarounds or hacks. They must be clean, efficient, and follow best practices and must be up to Google's or Meta's engineering standards.

When suggesting code, ensure it aligns with the existing architecture, coding style, and design patterns used in FreeRangeNotify. Avoid introducing unnecessary complexity or deviating from established conventions.

## Project Description
FreeRangeNotify is a high-performance, universal notification service built in Go. It uses a **Hub-and-Spoke distributed architecture** to decouple notification ingestion from delivery, ensuring reliability and massive throughput. The system incorporates advanced features such as multi-tenancy, Role-Based Access Control (RBAC), fully-fledged Workflows, Topic-based Pub/Sub, and Digest Rules.

---

## System Architecture & Data Flow

### System Overview (Mermaid)
```mermaid
graph TD
    Client[Client Application] -->|API Calls| API[API Server (Hub)]
    Webhook[Inbound Webhooks] -->|POST| API
    
    API -->|1. Validate, Auth & Audit| API
    API -->|2. Index (Draft/State)| ES[(Elasticsearch)]
    API -->|3. Orchestrate Workflows / Enqueue| Redis[(Redis Queue)]
    
    subgraph "Worker Layer"
        Redis -->|4. Dequeue| Worker[Notification Worker]
        Worker -->|5. Fetch User/Template/Digest Rules| ES
        Worker -->|6. Check Presence| Presence[(Redis Presence)]
        Worker -->|7. Render Content| Worker
        Worker -->|8. Deliver| Providers[Delivery Providers]
    end

    subgraph "Providers Ecosystem"
        Providers --> WH[Webhook]
        Providers --> Email[SMTP, Mailgun, Postmark, Resend, SendGrid, SES]
        Providers --> Push[APNS, FCM]
        Providers --> SMS[Twilio, Vonage]
        Providers --> Chat[Discord, Slack, Teams, WhatsApp]
        Providers --> InApp[In-App / SSE]
    end

    Worker -->|9. Update Status| ES
    ES -->|10. Broadcast SSE| SSE[(SSE Broadcaster)]
    SSE -->|11. Real-time Push| Browser[(React UI & Browsers)]
```

### Data Flow Breakdown
1.  **Ingestion**: The API Server receives requests (Notifications, Webhooks, Topics, OTP, File uploads). It validates the API Key, authenticates (OIDC SSO supported), and handles Multi-tenant RBAC.
2.  **Orchestration**: Advanced functions like **Workflows**, **Topics (Pub/Sub)**, **Digest Rules**, and the **AttachmentResolver** intercept the standard flow to apply logic, delays, aggregation, or binary materialisation.
3.  **Persistence**: Data and audit logs are safely stored in Elasticsearch.
4.  **Queuing**: Redis acts as the priority queue for delivery or delayed workflow executions.
5.  **Processing**: Workers pull from Redis. They resolve templates, preferences, and handle rendering.
6.  **Smart Routing**: Workers check real-time presence (Redis) and redirect if a dynamically active URL is found.
7.  **Delivery**: Messages are dispatched via extensive configured providers (Email, SMS, Push, Chat).
8.  **Real-time Broadcast**: State changes are distributed through Redis Pub/Sub natively to the React Vite UI and other clients connected via Server-Sent Events (SSE).

---

## Component Connections & Roles

| Component | Role | Technology |
| :--- | :--- | :--- |
| **API Server** | Handles ingestion, SSO Auth, RBAC, Admin routing, Licensing, and proxy for UI | Go / Fiber |
| **Message Queue** | Decouples API from delivery, handles delayed Workflows and Digestion mechanisms | Redis |
| **Worker** | The "Brain". Handles content rendering, preference checking, and provider execution | Go |
| **Elasticsearch** | Primary database for templates, logs, users, apps, environments, and audits | Elasticsearch 8.x |
| **Presence Registry** | Stores real-time user availability for "Smart Delivery" | Redis |
| **SSE Broadcaster** | Manages real-time Server-Sent Events connections and broadcasts notifications to UI | Go / Redis pub/sub |
| **Admin UI** | React/Vite dashboard proxied via `/v1` through the Go API Server | TypeScript / React |

---

## Connected Providers
-   **Push**: Apple Push Notification service (APNS), Firebase Cloud Messaging (FCM).
-   **Email**: SMTP, Mailgun, Postmark, Resend, SendGrid, Amazon SES.
-   **SMS / Voice**: Twilio, Vonage.
-   **Chat**: Discord, Slack, Microsoft Teams, WhatsApp.
-   **Custom & Realtime**: Webhook (with HMAC), In-App, SSE Broadcaster (Redis Pub/Sub).

---

## Directory Structure
```text
FreeRangeNotify/
├── cmd/
│   ├── server/                 # API Server Entry Point
│   ├── worker/                 # Background Processor Entry Point
│   ├── frn/                    # Command-line Admin interface
│   ├── newsletter-broadcaster/ # Broadcaster component
│   └── migrate/                # Database initialization/migration scripts
├── internal/
│   ├── agentdebug/             # Diagnostic helpers for the agent runtime
│   ├── config/                 # Configuration loading (YAML + env)
│   ├── container/              # Dependency injection container
│   ├── domain/                 # Core business models (Analytics, Auth, Workflows, Topics, Templates, etc.)
│   ├── infrastructure/
│   │   ├── database/           # Elasticsearch client
│   │   ├── providers/          # Complete set of Delivery Providers
│   │   └── repository/         # Elasticsearch DAOs
│   ├── interfaces/http/routes/ # Full Router Configuration (Fiber)
│   ├── platform/               # Cross-cutting platform concerns (auth, presence, queue clients)
│   ├── seed/                   # Seed data for first-run bootstrap
│   ├── telemetry/              # OpenTelemetry tracing & metrics wiring
│   ├── usecases/               # Orchestration/Service layer logic
│   └── version/                # Build version metadata
├── pkg/                        # Shared utilities (logging, validation, error types)
├── ui/                         # Management Dashboard (React/Vite)
├── docs/                       # Generated Swagger / OpenAPI specs
├── e2e/                        # Playwright end-to-end specs
├── sdk/                        # Client SDKs
├── examples/nextjs-receiver/   # Sample receiver app for smart-delivery testing
├── deploy/ deployments/docker/ # Production deployment manifests
├── tests/                      # Integration tests
└── docker-compose.yml          # Orchestration for the combined stack
```

---

## Testing & Operations

### Deployment Steps
To clear the environment and start fresh:
```powershell
# 1. Stop and remove volumes
docker-compose down -v

# 2. Build services
docker-compose build

# 3. Start stack in background
docker-compose up -d

# 4. Initialize Database (Templates/Indices) - Run once
docker-compose exec notification-service /app/migrate
```

### Testing Workflows
- **Unit Tests**: `make test-unit` or `go test -v -short ./...`
- **Integration Tests**: `make test-integration` - starts Docker services, runs tests, then stops services
- **Full Test Suite**: `make test` - runs both unit and integration
- **Coverage**: `make test-coverage` - generates coverage reports
- **API Testing**: Use `test-full-api.ps1` script or manual curl commands from `TESTING_GUIDE.md`
- **Smart Delivery Testing**: Follow `SMART_DELIVERY_GUIDE.md` for end-to-end presence and instant flush testing
- **UI E2E**: `npx playwright test` — config in `playwright.config.ts`, specs under `e2e/`

### Debugging Steps
Monitoring the interaction between services is critical:
```powershell
# Follow all logs
docker-compose logs -f

# Follow specific service (Hub vs Worker)
docker-compose logs -f notification-service
docker-compose logs -f notification-worker
docker-compose logs -f ui

# Check internal queue depth (Admin API)
curl http://localhost:8080/v1/admin/queues/stats

# Enable Debug Logging in Worker
# Set DEBUG=true or FREERANGE_LOG_LEVEL=debug in docker-compose.yml or .env
docker-compose logs -f notification-worker
```

---

## Smart Delivery Architecture
-   **Presence Registry**: Redis-stored map of `UserID -> CurrentDynamicURL`.
-   **Instant Flush**: On `POST /v1/presence/check-in`, the system moves all `StatusQueued` notifications for that user to the **head** of the Redis list (`LPUSH`).
-   **Dynamic Routing**: The Worker checks Redis presence *before* static profile settings. If a user is active, it delivers to the dynamic URL.
-   **Real-time SSE**: For browser clients, use `channel: "sse"` to deliver notifications instantly via Server-Sent Events. Clients connect to `/v1/sse?user_id={internal_uuid}` (NOT external_id) and receive push notifications without refresh.
-   **Testing**: Use test receivers (cmd/receiver) on different ports to simulate user presence. When sending notifications, ensure the `user_id` in the payload is the **internal UUID** returned during user creation.

## Technical Guidelines for Code Suggestions
-   **Fiber First**: Use Fiber's context and built-in features for HTTP logic. Always respect the grouped `/v1` structure.
-   **UI Proxying**: Remember the `ui` relies on `notification-service` responding accurately to `/v1` endpoints proxying API connections securely.
-   **Domain Driven**: When adding new features, integrate them properly into `internal/domain` and `internal/usecases`.
-   **Structured Logging**: Always use `zap.Logger` with typed fields, never `fmt.Println`.
-   **Error Handling**: Use the project's custom `pkg/errors` for consistent API responses.
-   **Async by Default**: Keep the API path fast; move heavy logic (rendering, provider calls) to the Redis Queue/Worker.
-   **Full Naming**: When referring to push services, use **Apple Push Notification service** or **Firebase Cloud Messaging**.
-   **Routes Organization**: Group routes by access level (public/protected/admin) in `internal/interfaces/http/routes/routes.go`.
-   **Dependency Injection**: Wire services through `container.Container` for testability and decoupling.
-   **Validation**: Use struct tags with go-playground/validator for input validation.
-   **Repository Pattern**: Access data through repository interfaces in `infrastructure/repository`.

## OTP as a Service
Programmatic one-time-passcode delivery + verification over SMS, WhatsApp, and Email. Reuses the standard notification pipeline (templates, providers, billing) for the send path; adds dedicated security primitives on the verify path.
-   **Endpoints** (all require `X-API-Key`, `app_id` is derived server-side and must never be accepted from the body):
    -   `POST /v1/otp/send` — generates a code, hashes it, dispatches via the chosen channel. Returns `request_id`.
    -   `POST /v1/otp/verify` — verifies a code against a `request_id`. Constant-time comparison, atomic attempt counter.
    -   `POST /v1/otp/resend` — re-issues a fresh code (60 s cooldown per record, attempt counter resets, previous code invalidated).
-   **Defaults**: 6 digits, 5-minute TTL, 5 attempts. Per-recipient rate limiting enforced server-side.
-   **Storage**: codes are hashed at rest. Never log raw codes; never return raw codes in API responses.
-   **Templates**: OTP send uses the standard template/provider stack — the same auto-fill and rendering rules apply. The code itself is injected as a template variable (e.g. `{{.code}}`).
-   **DO NOT** reimplement verify logic on top of `/v1/notifications`; the constant-time compare + atomic attempt decrement live in the OTP usecase for a reason.

## File Attachments
Files (PDFs, images, videos, documents) can be attached to a notification via **three** input modes — exactly one per attachment, or the API rejects with `400 attachment_ambiguous_source`:
-   **`url`** — public URL fetched once by the worker (cap 25 MB, 20 s timeout). Best when the file already lives on a CDN.
-   **`content_base64`** — inline (≤ ~14 MB encoded / ~10 MB decoded). Accepts URL-safe alphabet and strips `data:` prefixes.
-   **`file_id`** — FRN-managed file uploaded via `POST /v1/files`. Recommended for files > 10 MB or reused across notifications.
-   **Files endpoints** (`X-API-Key`): `POST /v1/files` (multipart), `GET /v1/files`, `GET|DELETE /v1/files/:id`, `GET /v1/files/:id/content`, `GET /v1/files/:id/download-url` (signed, default 15 min TTL), public `GET /v1/files/download/:id?sig=...` (signature is the auth — intentionally omits `@Security ApiKeyAuth`).
-   **Retention**: default 30 days; set `filestore.retention_days: -1` to pin files (used for invoices, contracts). Pinned uploads omit `expires_at` in metadata.
-   **Limits**: `filestore.max_bytes` (default 50 MiB), `filestore.allowed_mime_types` (wildcards). Multi-tenant: all queries scoped by `app_id`; cross-tenant access returns `404 file_not_found`, never 403.
-   **Resolver**: `AttachmentResolver` in `internal/usecases` normalises all three modes to bytes+MIME before the provider call. Channels that cannot carry binaries (SMS, In-App, SSE) fail fast with `ErrChannelUnsupportedAttachment`.
-   **Canonical guides**: `documents/FILE_ATTACHMENTS_GUIDE.md` (caller-facing), `documents/plans/FILE_ATTACHMENTS_PLAN.md` (implementation plan and progress meter), `ui/src/docs/file-attachments.md` (in-product docs).

## Template Variable Auto-Fill
Any render path that has a `*user.User` in scope (today: `cmd/worker/processor.go`) must apply these auto-fill rules via the shared helper in `internal/usecases/template/autofill.go`:
-   **Authoritative name source**: `user.FullName` (trimmed) is the primary value for `name`, `user_name`, `full_name`. `first_name` / `last_name` are derived by splitting it.
-   **Fallbacks (in order)**: `user.Email` local-part → `user.ExternalID` local-part (if it looks like an email) → `"there"`.
-   **Gate on `tmpl.Variables`**: never inject keys the template does not declare — this prevents payload pollution and mirrors the existing `product` / `cta_url` pattern.
-   **Never overwrite caller-provided values**: only inject when `needTemplateVar(data, key)` is true.
-   **Channel-agnostic**: auto-fill applies to every channel (email, SMS, push, chat, webhook, SSE), not just email.
-   **Shared implementation**: keep the logic in a single helper (`internal/usecases/template/autofill.go`). Do not duplicate inline. Synchronous preview renders without a user remain a no-op.
