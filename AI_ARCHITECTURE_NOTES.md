## FreeRangeNotify – Architecture Notes (AI)

Last updated: 2026-03-08

### High-level overview

- **Product**: FreeRangeNotify – multi-channel notification platform (email, SMS, push, webhooks, SSE, etc.) with workflows, digests, topics, RBAC and audit logging.
- **Backend**: Go 1.24 (Fiber) service plus a worker process.
- **Storage**: Elasticsearch as the primary data store (notifications, users, templates, auth, audit, etc.).
- **Infra services**: Redis for queues, throttling, presence/unread counters, and worker coordination.
- **Frontend**: Vite + React 19 + TypeScript dashboard (`ui/`) for admins + documentation hub.
- **SDKs**: Separate JS/TS SDK and React bell component under `sdk/js` and `sdk/react`.

### Runtime topology (docker-compose)

- **Elasticsearch**
  - Single-node cluster (`freerange-cluster`) exposed on `9200/9300`.
  - Security disabled for development; data volume `elasticsearch_data`.
- **Redis**
  - `redis:7-alpine` on `6379` with append-only enabled, volume `redis_data`.
- **notification-service (Go HTTP API)**
  - Built from repo root Dockerfile as `freerange-notify:latest`.
  - Exposes API on `8080` mapped to `${FREERANGE_SERVER_PORT:-8080}`.
  - Depends on healthy Elasticsearch; configured with:
    - `FREERANGE_DATABASE_URLS` → Elasticsearch URL(s).
    - `FREERANGE_REDIS_HOST` / `FREERANGE_REDIS_PORT`.
    - `FREERANGE_SERVER_PUBLIC_URL` for generating links.
- **notification-worker (Go worker)**
  - Same image, command `./worker`.
  - Uses Redis + Elasticsearch and processes queued notification/workflow jobs.
- **ui (React dashboard)**
  - Vite dev server running on `3000`, hot reloaded via `./ui` bind mount.
  - Proxies API calls via `API_PROXY_TARGET=http://notification-service:8080`, browser talks to `/v1` relative paths.

`docker-compose.dev.yml` overrides:
- Runs server/worker in **builder** stage with `go run ./cmd/server` / `./cmd/worker` and mounts the source tree for live code changes.
- Lowers Elasticsearch JVM memory for local dev.

### Configuration (`config/config.yaml`)

- **App / server**
  - Name/version, environment, debug/log level.
  - Fiber server host/port and timeouts; `write_timeout=0` to support long-lived SSE.
- **Database**
  - List of Elasticsearch URLs, credentials, index prefix (`freerange_dev`).
- **Redis / queue**
  - Connection settings, pool sizing, retry configuration.
  - Queue type is `"redis"` with worker count/concurrency and retry policy.
- **Providers**
  - Config blocks for `fcm`, `apns`, `sendgrid`, `smtp`, `twilio`, plus Phase 3 webhooks: `slack`, `discord`, `whatsapp`.
  - Most fields are environment-driven (`${FREERANGE_PROVIDERS_*}`).
- **Monitoring**
  - Prometheus-style metrics toggle, port, path, and namespace.
- **Security**
  - JWT secret + access/refresh TTLs.
  - API key header name (`X-API-Key`), global rate limiting, and CORS policy (localhost UI + API).
- **Features (feature flags)**
  - `workflow_enabled`, `digest_enabled`, `sse_hmac_enforced`, `topics_enabled`, `throttle_enabled`, `audit_enabled`, `rbac_enabled`,
    `template_versioning_enabled`, `snooze_enabled`, `multi_environment_enabled`.

These flags gate subsystems in the container and routing layers.

### Backend structure (Go)

- **Entrypoints (`cmd/`)**
  - `cmd/server/main.go`:
    - Loads config, validates it, builds `container.Container`.
    - Initializes Elasticsearch via `DatabaseManager.Initialize`.
    - Spins up Fiber app with:
      - Custom error handler (`middleware.ErrorHandler`).
      - `recover`, `requestid`, HTTP logger, and permissive CORS.
    - Exposes:
      - `/health`, `/database/stats`, `/version`, `/api/v1/status`.
      - Swagger UI under `/swagger/*` and static OpenAPI at `/openapi`.
      - Stubbed `/metrics` (Prometheus integration TODO).
    - Delegates core API registration to `routes.SetupRoutes(app, c)`.
  - `cmd/worker/main.go` (not read yet but implied from Docker):
    - Worker process that pulls from Redis-backed queues and invokes providers to deliver notifications/digests/workflows.
  - Other commands:
    - `cmd/migrate`: migration/index management against Elasticsearch.
    - `cmd/receiver` + `cmd/sse_receiver`: helper tools around webhooks / SSE testing.

- **Dependency injection container (`internal/container/container.go`)**
  - Central object that wires:
    - **Config + logger**.
    - **Database**: `DatabaseManager` (Elasticsearch client, index templates, repositories).
    - **Redis**: Redis client, Redis-backed queue (`queue.NewRedisQueue`).
    - **Metrics**: `NotificationMetrics`.
    - **Limiter**: Redis rate limiter for per-user/channel controls.
    - **Repositories** from `DatabaseManager.GetRepositories()`:
      - Applications, users, notifications, templates, workflows, digests, topics, audit logs, environments, auth, etc.
    - **SSE**:
      - `sse.Broadcaster` backed by notification repository + Redis; supports subscriber presence and admin activity feeds.
    - **Services** (`internal/usecases` + `internal/usecases/services`):
      - `ApplicationService`, `UserService`, `NotificationService`, `TemplateService`, `PresenceService`.
      - Workflow engine (`WorkflowService`) behind `workflow_enabled`.
      - Digest engine (`DigestService`) behind `digest_enabled`.
      - Topics (`TopicService`) behind `topics_enabled`.
      - Audit logging (`AuditService`) behind `audit_enabled`.
      - RBAC team management (`TeamService`) behind `rbac_enabled`.
      - Multi-environment management (`EnvironmentService`) behind `multi_environment_enabled`.
      - Auth service (`AuthService`) for dashboard admins.
    - **JWT**:
      - `jwt.Manager` built from security config (access/refresh TTLs).
    - **OIDC/SSO**:
      - Optional OIDC provider, OAuth2 config, and ID token verifier if `cfg.OIDC.Enabled`.
    - **Handlers** (`internal/interfaces/http/handlers`):
      - Users, applications, notifications, templates, presence, admin, health, SSE, auth, quick-send, playground, analytics, workflows, digests, topics, audit, team, environments, custom providers.
  - Applies feature flags:
    - If a flag is off, associated service/handler is not created and routes are skipped.
    - SSE HMAC enforcement toggled via `SSEHandler.SetHMACEnforced(true)`.

- **HTTP routing (`internal/interfaces/http/routes/routes.go`)**
  - Root API group at `/v1`.
  - **Audit middleware**:
    - If `AuditService` is present, wraps all `/v1` routes via `AuditMiddleware` to capture state-changing requests.
  - **Public routes**:
    - `/v1/auth/*`: register, login, refresh, forgot/reset password.
    - `/v1/auth/sso/*`: OIDC login/callback when OIDC is configured.
    - `/v1/health`: health check (`HealthHandler.Check`).
    - `/v1/sse`: client SSE connection (`SSEHandler.Connect`) for subscriber notifications.
    - `/v1/playground/:id`: public webhook playground receiver + fetch.
  - **Protected routes (API key auth)**:
    - API key middleware: `APIKeyAuth` uses `ApplicationService` (and optionally `EnvironmentService`) to authenticate requests via `X-API-Key` and optional environment context.
    - `/v1/users`: CRUD + bulk create, devices, preferences, subscriber hash for SSE HMAC.
    - `/v1/sse/tokens`: short-lived SSE token issuance endpoint.
    - `/v1/presence`: check-in endpoint.
    - `/v1/quick-send`: simplified notification endpoint that wraps the full notification engine.
    - `/v1/notifications`: send, bulk, broadcast, batch, unread count/list, mark read/read-all, bulk archive, snooze/unsnooze, cancel/cancel batch, retry, get/update.
    - `/v1/templates`: library, clone, CRUD, render, rollback, diff, test-send (via SMTP provider), controls, versioning.
    - `/v1/workflows` (if enabled): CRUD, trigger, executions, cancel.
    - `/v1/digest-rules` (if enabled): CRUD for digest rules.
    - `/v1/topics` (if enabled): topic CRUD + subscriber management.
  - **Admin routes (`/v1/admin` + `/v1/apps`)**
    - JWT-based admin auth middleware (`JWTAuth`) built on `AuthService`.
    - `/v1/admin`:
      - `/me`, `/logout`, `/change-password`.
      - Queue management: stats, DLQ, replay.
      - Provider health, playground creation (webhook + SSE), SSE test messages.
      - Analytics summary, activity feed SSE.
      - Audit routes (if enabled) with RBAC permission checks.
    - `/v1/apps`:
      - JWT-protected management of applications (create, list, get, update, delete).
      - API key regeneration, app settings CRUD.
      - Custom provider management (Phase 3).
      - Multi-environment management (Phase 6): create/list/promote/get/delete environments per app.
      - RBAC enforcement via `RequirePermission` middleware on destructive settings and delete, reading `app_id` from route params.

- **RBAC middleware (`rbac_middleware.go`)**
  - `RequirePermission(perm, membershipRepo, appRepo, logger)`:
    - Reads `app_id` and `user_id` from `Fiber` locals.
    - If no `user_id` (pure API key) → treat as app owner and allow.
    - If app owner (AdminUserID) → allow and set role to `owner`.
    - Otherwise, loads membership and checks `HasPermission(role, perm)`; denies with `Forbidden` on failure, sets `role` local on success.

- **Domain models**
  - `internal/domain/user/models.go`:
    - Users scoped by `AppID` and optional `EnvironmentID`.
    - Multiple channels: email, phone (SMS), webhook URLs, Slack/Discord/WhatsApp endpoints.
    - Rich `Preferences`: per-channel toggles, quiet hours, DND, category-level settings, throttle configs, daily limits.
    - `Device` model for push tokens / multi-device tracking.
    - Repository + service interfaces for CRUD, devices, preferences, bulk ops.
  - `internal/domain/auth/models.go`:
    - Dashboard admin users, password reset tokens, JWT refresh tokens.
    - DTOs for login/register/forgot/reset/change password.
    - `AuthResponse` wrapping user + token pair.
    - RBAC model:
      - Roles: owner/admin/editor/viewer.
      - Permissions: manage app/members/templates, send notifications, view logs/audit.
      - RolePermission map + `HasPermission` helper.
      - `AppMembership` model and `MembershipRepository` + `TeamService` interface.

- **Use cases/services**
  - `authService`:
    - Handles register/login/refresh/logout/password flows + SSO.
    - Generates/validates JWT via `jwt.Manager`, stores refresh tokens in ES.
    - Uses SMTP env vars directly to send password reset emails.
    - Claims pending memberships on register/login/SSO via `MembershipRepository.ClaimByEmail`.
    - Guards against duplicate users by email (race-safe create).
  - `teamService`:
    - Invitation flow: invites by email + role, resolves to user ID if the user already exists.
    - **Sends invitation email via SMTP** after creating the membership (best-effort, non-blocking).
      - Two variants: **existing user** gets "Log in to accept" CTA; **new user** gets "Create your account" CTA.
      - Email includes the app name, assigned role, and a human-readable role description.
      - When a new user registers/logs in, `ClaimByEmail` converts their email-placeholder membership to a real user ID.
    - Prevents duplicate memberships, protects the last owner from demotion/removal.
    - Lists memberships and fetches membership for a (app,user) pair.
  - Notification, workflow, digest, topics, environment, analytics, etc. live in `internal/usecases` and `internal/usecases/services`; they orchestrate repositories, queues, metrics, and providers.

### HTTP handlers – example: applications

- `ApplicationHandler` (`internal/interfaces/http/handlers/application_handler.go`):
  - Uses `ApplicationService` + `MembershipRepository` + `application.Repository` + validator + logger.
  - **`authorizeAppAccess(c, appID, userID)`** — central helper that resolves access and role:
    - Returns `(app, RoleOwner, nil)` if user is the app's `AdminUserID`.
    - Returns `(app, membership.Role, nil)` if user has an `AppMembership` (RBAC enabled).
    - Returns `Forbidden` error otherwise.
    - Stashes resolved `role` in `c.Locals("role")` for downstream use.
  - Endpoints with role-based guards:
    - `Create`: any authenticated admin; new app's `AdminUserID` is set to caller.
    - `GetByID`: any team member (viewer+). Non-owners see masked API key.
    - `List`: shows owned apps + apps where user has a membership.
    - `Update`: admin or owner only.
    - `Delete`: owner only.
    - `RegenerateAPIKey`: owner only.
    - `UpdateSettings`: admin or owner only.
    - `GetSettings`: any team member (viewer+).
  - API key masking: owners see full key; all other roles see `***` + last 8 chars.

### Frontend architecture (`ui/`)

- **Stack**
  - Vite + React 19 + TypeScript, Tailwind 4, Radix UI, `sonner` toasts.
  - React Router v7, context-based auth and theming.

- **Entry + root app**
  - `main.tsx` mounts `<App />` into `#root`.
  - `App.tsx`:
    - Wraps everything in `Router`, `ThemeProvider`, `AuthProvider`, `ErrorBoundary`.
    - Uses React Router routes with lazy-loaded pages and a global `PageLoader` fallback.
    - Renders global `CommandPalette` and `Toaster` outside of route `Suspense`.

- **Routing**
  - `/`: marketing/landing page.
  - Auth routes (`AuthLayout`):
    - `/login`, `/register`, `/forgot-password`, `/reset-password`.
  - SSO:
    - `/auth/callback` handles OIDC redirect.
  - Protected dashboard (`ProtectedRoute` + `DashboardLayout`):
    - `/apps`, `/apps/:id`, `/apps/:id/templates/library`.
    - `/dashboard`: overview with metrics and quick tests.
    - `/workflows`, `/workflows/new`, `/workflows/:id`, `/workflows/executions`.
    - `/digest-rules`, `/topics`, `/audit`.
  - Documentation hub:
    - `/docs` → `DocsLayout` with nested routes.
    - `/docs/getting-started`, other slugs via `DocsPage`; `/docs/api` via `ApiReferencePage`.

- **Key shared pieces**
  - `contexts/AuthContext`:
    - Holds current admin user and JWT; wraps Axios or fetch-based API calls with token injection and refresh handling (not fully inspected yet).
  - `contexts/ThemeContext`:
    - Dark/light theme switching and persistence.
  - `components/ui/*`:
    - Tailwind + Radix primitives (button, card, dialog, dropdown, table, tabs, slide-panel, etc.).
  - `hooks/use-api-query.ts`:
    - Small data-fetching hook:
      - `useApiQuery(fetcher, deps, { enabled })` → `{ data, loading, error, refetch }`.
      - Normalizes Axios error shapes via `err.response.data.error/message`.
  - `lib/utils.ts`:
    - `cn` for class merging.
    - `timeAgo`, `formatDuration`.
    - `extractErrorMessage` for Axios error handling.

- **Example complex page – workflow builder**
  - `pages/workflows/WorkflowBuilder.tsx`:
    - Supports **create** and **edit** modes (`/workflows/new` vs `/workflows/:id`).
    - Requires selecting an application first:
      - Stores `last_app_id` and `last_api_key` in `localStorage`.
      - Fetches applications via `applicationsAPI.list()`, sets `apiKey` for subsequent workflow API calls.
    - Manages workflow metadata (name, description, trigger ID, status).
    - Maintains an ordered list of steps (`BuilderStep`), displayed via `WorkflowStepCard` and edited in a slide-in panel (`SlidePanel` + `WorkflowStepEditor`).
    - Save flow:
      - Validates required fields and at least one step.
      - For create, calls `workflowsAPI.create(apiKey, payload)` then navigates to `/workflows/:id`.
      - For edit, calls `workflowsAPI.update(apiKey, id, payload)` and can also activate the workflow.
    - Test trigger:
      - Collapsible card when workflow is `active`.
      - Uses `ResourcePicker<User>` to pick a test user via `usersAPI.list(apiKey, 1, 100)`.
      - Free-form JSON payload editor (`JsonEditor`) parsed into `payload`.
      - Triggers backend via `workflowsAPI.trigger(apiKey, { trigger_id, user_id, payload })` and surfaces errors via `extractErrorMessage`.

### SDKs and auxiliary projects

- `sdk/js`:
  - `@freerangenotify/sdk` – JS/TS SDK for interacting with the API (notifications, workflows, SSE, etc.).
- `sdk/react`:
  - `@freerangenotify/react` – React notification bell component for SSE streams.
- `test-sse-client`:
  - Next.js app used as a playground/testing client for SSE-based notification delivery and UI flows.

### Security & access control highlights

- **Public vs protected APIs**
  - Public `/v1/auth/*`, `/v1/health`, `/v1/sse`, `/v1/playground/*`.
  - Protected `/v1/*` via:
    - API key auth for tenant-facing endpoints (users, notifications, templates, workflows, etc.).
    - JWT admin auth for dashboard-facing endpoints (`/v1/admin`, `/v1/apps`, analytics, queues, audit, team).
- **RBAC**
  - Role/permission system scoped per-app; implemented via `AppMembership`, `MembershipRepository`, `TeamService`.
  - **Handler-level enforcement** via `authorizeAppAccess()` in `ApplicationHandler` — resolves ownership or membership and returns the user's role. Each endpoint applies role-based guards (viewer+ for reads, admin+ for writes, owner-only for destructive ops).
  - **Middleware-level enforcement** via `RequirePermission` for audit log routes (`PermViewAudit`) and team management routes (`PermManageMembers`).
  - Feature flag controlled (`rbac_enabled`); when disabled, `membershipRepo` is nil and only app owners have access.
  - API key visibility is masked for non-owner roles.
- **JWT**
  - Access/refresh token pair, refresh tokens persisted to ES and revocable per token or per user.
  - Password reset and change flows revoke existing tokens for safety.
- **SSE**
  - Subscriber hash + short-lived token endpoint + optional HMAC enforcement control who can receive live streams.

### What to focus on next (for improving prod experience)

- **Performance & UX**
  - End-to-end latency: queues + providers + SSE updates across workflows.
  - Dashboard responsiveness on large datasets (notifications, audit logs, workflows).
  - Error surfaces in UI using `useApiQuery` + `extractErrorMessage` (ensure backend error shapes are consistent).
- **Security hardening**
  - Production CORS + JWT secret management.
  - OIDC/SSO flows and RBAC guardrails, especially around app ownership and team roles.
  - API key handling, regeneration flows, and UI hints (where keys are stored/visible).
- **Reliability**
  - Queue retry strategies and DLQ handling in worker.
  - Elasticsearch index mappings/templates for long-term stability and migrations.
  - Provider timeouts/retries and circuit breaker behavior.

