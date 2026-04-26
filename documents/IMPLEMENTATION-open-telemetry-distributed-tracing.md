# Implementation plan: OpenTelemetry distributed tracing (minimal compute)

**Repo:** `FreeRangeNotify`  
**Audience:** Dave + future AI handoff  
**Status:** Plan only (no OTel code added yet)

## Motivation / current status

- We need **distributed tracing for the entire system** (all APIs + background processing), so production debugging is no longer “grep logs and guess.”
- A recent motivating example: **Prod WhatsApp** failures showed up in worker logs as Twilio **21656** (invalid `ContentVariables`). This plan is *not* WhatsApp-specific; it’s to make issues like that easy to diagnose across the platform.

## Goals

- **Coverage (system-wide):**
  - **All HTTP APIs** served by `freerange-notification-service` (every request gets a server span).
  - **All background work** in `freerange-notification-worker` (dequeue → process → provider calls).
  - **External calls** as child spans (Twilio, Meta, SMTP/SendGrid, Discord/Slack/Teams webhooks, etc.).
  - **Optional later**: Elasticsearch/Redis client spans if needed (can be deferred to reduce overhead).
- **Correlation:** Logs and traces correlate via `trace_id` / `span_id`.
- **Low overhead:** Keep compute and operational complexity minimal (sampling, traces-only pipeline).

## Non-goals (for safety + cost)

- Do **not** capture full request/response bodies in tracing by default (PII + secrets risk).
- Do **not** require frontend (Vercel) tracing for Phase 1.
- Do **not** build a large observability stack (no Kubernetes required).

## Span naming + attributes (conventions)

These conventions make traces searchable and consistent across features.

- **HTTP server spans (API):** `HTTP {method} {route}` (e.g. `HTTP POST /v1/notifications/bulk`)
  - Attributes: `http.method`, `http.route`, `http.status_code`, duration
  - Domain attrs (non-PII): `app.id`, `user.id` (UUIDs), `channel`
- **Worker spans:** `worker.processNotification`
  - Attributes: `notification.id`, `app.id`, `channel`, `priority`
- **Provider spans:** `provider.send {provider}` (e.g. `provider.send twilio.whatsapp`)
  - Attributes: `provider`, `provider.kind`, upstream status/error code (no bodies)

## Proposed architecture (minimal compute)

### Phase 1 (fastest value): API tracing + local trace UI

- Instrument Go Fiber API with OpenTelemetry SDK.
- Export traces via OTLP (gRPC) to a local **OpenTelemetry Collector** container.
- Collector exports to a single trace backend:
  - **Recommended for minimal compute:** **Jaeger all-in-one** (1 container + built-in UI).

**Data flow:** Browser/Client → API (span) → Collector → Jaeger UI

### Phase 2 (complete server-side story): worker tracing + correlation

- Instrument worker to create spans for:
  - dequeue/process notification
  - provider selection + send attempt
  - external calls (Twilio/Meta/etc.)
- Propagate trace context from API to worker:
  - Store `traceparent` (W3C Trace Context) in queued payload / notification metadata.
  - Worker extracts and continues the trace (preferred), else starts a new trace and logs correlation keys.

**Data flow:** Client → API span → enqueue with `traceparent` → worker continues span → Collector → Jaeger

### Phase 3 (optional): Vercel frontend correlation

- Add lightweight request correlation (prefer `X-Request-Id`) or OTel Web tracing later.
- Only if needed; Phase 1+2 typically solves production backend visibility.

## High-level design (HLD)

### Components

- **`notification-service` (Go/Fiber)**:
  - HTTP server spans, route + status + latency attributes.
  - Add `trace_id` to zap logs (correlation).

- **`notification-worker` (Go)**:
  - Spans for `processNotification` and provider sends.
  - Add `trace_id` to logs.

- **`otel-collector` (Docker container)**:
  - Receives OTLP from API/worker.
  - Exports to Jaeger.

- **`jaeger` (Docker container)**:
  - Trace storage + UI.

### Minimal configuration knobs (via `.env` / env vars)

All default to **off** so prod can enable safely.

- `FREERANGE_OTEL_ENABLED=false|true`
- `FREERANGE_OTEL_SERVICE_NAME=freerange-notification-service` (service) / `freerange-notification-worker` (worker)
- `FREERANGE_OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317`
- `FREERANGE_OTEL_SAMPLE_RATIO=0.1` (10% sampling, adjust down if needed)
- `FREERANGE_OTEL_ENV=production|staging|dev` (resource attribute)

## Detailed implementation steps (what to change in this repo)

### Step A — add a telemetry bootstrap package (shared)

Create `internal/telemetry/telemetry.go`:

- If `FREERANGE_OTEL_ENABLED != "true"` → return no-op shutdown.
- Otherwise:
  - Create OTLP trace exporter (`otlptracegrpc`) to `FREERANGE_OTEL_EXPORTER_OTLP_ENDPOINT`.
  - Create `sdktrace.TracerProvider` with:
    - sampling: parent-based ratio (from `FREERANGE_OTEL_SAMPLE_RATIO`)
    - resource attrs: `service.name`, `service.version`, `deployment.environment`
  - Set global propagator: `TraceContext` + `Baggage`.
  - Return shutdown func that flushes provider on SIGTERM.

**Design note:** keep this package dependency-local and small; no metrics/log exporters initially.

### Step B — instrument Fiber (API)

In `cmd/server/main.go`:

- Initialize telemetry early (after config load, before app starts).
- Add Fiber middleware:
  - Use `github.com/gofiber/contrib/otelfiber/v2` *or* a tiny custom middleware that:
    - starts a span per request
    - records status + route
    - injects/extracts W3C headers
- Ensure shutdown calls telemetry flush.

### Step C — instrument worker

In `cmd/worker/main.go`:

- Initialize telemetry with `service.name=freerange-notification-worker`.
- Ensure shutdown flush.

In `cmd/worker/processor.go`:

- Add spans around:
  - `processNotification`
  - `sendNotification`
  - provider Send calls (Twilio/Meta)
- Include attributes:
  - `notification.id`, `app.id`, `channel`, `provider`

### Step D — propagate trace context API → worker (Phase 2)

Preferred minimal approach:

- When API enqueues work, attach `traceparent` into the queued message (or notification doc metadata).
- When worker dequeues, extract it and set parent context before starting spans.

If the queue payload format is hard to change, fallback:

- Worker starts a new trace but logs `notification_id` and `trace_id` so you can still join the dots.

### Step E — add collector + jaeger to Docker Compose (minimal)

Add a new optional compose file (recommended):

- `docker-compose.otel.yml`:
  - `otel-collector` (exposes 4317 internally, optionally 4318)
  - `jaeger` (UI on 16686)

Add new config file:

- `config/otel-collector.yaml`:
  - receiver: `otlp` (grpc)
  - exporter: `jaeger` (or `otlp` to jaeger depending on image)
  - pipeline: traces only

Reason: keep base compose stable; ops can enable by:

```bash
docker compose -f docker-compose.yml -f docker-compose.otel.yml up -d
```

### Step F — ops runbook (prod)

- Enable:
  - `FREERANGE_OTEL_ENABLED=true`
  - `FREERANGE_OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317`
  - `FREERANGE_OTEL_SAMPLE_RATIO=0.05` initially
- Restart compose.
- Access Jaeger UI internally at `http://<server>:16686`.

## Acceptance criteria

- With OTel enabled:
  - A request like `GET /v1/health` appears in Jaeger with route and status.
  - Worker spans appear for notification processing.
  - Logs contain `trace_id` so a single trace can be found from a log line.

## Security / compliance notes

- Never export secrets (Authorization headers, SMTP passwords, webhook URLs) as span attributes.
- Never export message bodies or user phone numbers without explicit redaction.
- Sampling defaults low; increase only when debugging.

## Handover notes (for another AI)

- Fiber server entry: `cmd/server/main.go`
- Worker entry: `cmd/worker/main.go`
- Config system: Viper env prefix `FREERANGE_` with dot→underscore mapping (`internal/config/config.go`)
- Existing logs show the WhatsApp template error is Twilio 21656 (not phone verification).
- Keep implementation minimal: traces only, Collector + Jaeger, sampling, no payload capture.

