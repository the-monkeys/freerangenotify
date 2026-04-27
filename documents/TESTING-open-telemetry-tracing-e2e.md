# E2E test guide: OpenTelemetry tracing → Tempo → Grafana

**Repo:** `FreeRangeNotify`  
**Goal:** Verify end-to-end traces from **API + worker** are exported via **OTLP** → **otel-collector** → **Tempo**, then visible in **Grafana**.

---

## Prereqs

- Docker Desktop running
- Your base stack can start via `docker compose -f docker-compose.yml up -d`
- The optional observability stack files exist:
  - `docker-compose.otel.yml`
  - `config/otel-collector.yaml`
  - `config/tempo.yaml`
  - `config/grafana-datasources.yaml`

---

## 1) Start Tempo + Grafana + Collector

From repo root:

```bash
docker compose -f docker-compose.yml -f docker-compose.otel.yml up -d tempo otel-collector grafana
```

Sanity checks:

```bash
curl.exe -s -o NUL -w "%{http_code}\n" http://127.0.0.1:3200/ready
curl.exe -s -o NUL -w "%{http_code}\n" http://127.0.0.1:3001/login
```

- Expect Tempo ready: `200`
- Expect Grafana login page: `200`

Grafana UI:
- **URL:** `http://127.0.0.1:3001`
- **Default login:** `admin` / `admin` (unless overridden via env in compose)

Tempo datasource is provisioned automatically by `config/grafana-datasources.yaml`:
- **Connections → Data sources** → should show **Tempo** (default)

---

## 2) Enable tracing for API + worker

Tracing is **opt-in**. Set these env vars for BOTH:
- `notification-service` (API)
- `notification-worker` (worker)

Minimum set:

```text
FREERANGE_OTEL_ENABLED=true
FREERANGE_OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317
FREERANGE_OTEL_SAMPLE_RATIO=1
FREERANGE_OTEL_ENV=development
```

Notes:
- Use `FREERANGE_OTEL_SAMPLE_RATIO=1` for testing so you don’t miss traces.
- For production, lower this (e.g. `0.05` or `0.01`) once you confirm everything works.

Restart the API + worker containers after setting env:

```bash
docker compose -f docker-compose.yml up -d --force-recreate notification-service notification-worker
```

---

## 3) Generate traces (API)

Hit a simple endpoint:

```bash
curl.exe -s -o NUL -w "%{http_code}\n" http://127.0.0.1:8080/v1/health
```

Expected:
- HTTP `200`
- A trace should appear in Tempo for service **`freerange-notification-service`**

---

## 4) Generate traces that also drive worker activity (recommended)

The API health check only proves server spans.
To prove worker spans too, enqueue a notification using the API.

### Option A (preferred): use the existing container test script

If `scripts/container-api-test.sh` exists, run it from inside the API container (bypasses host networking quirks):

```bash
docker exec -it freerange-notification-service sh -lc "ls -la /app/scripts && sh /app/scripts/container-api-test.sh"
```

Expected outcomes:
- It logs in, creates user (or reuses), and sends a quick notification
- Worker processes the queued notification shortly after

### Option B: manual quick-send (if you already have a token + appId)

1) Login:

```bash
curl.exe -s -X POST http://127.0.0.1:8080/v1/auth/login ^
  -H "Content-Type: application/json" ^
  -d "{\"email\":\"<admin_email>\",\"password\":\"<admin_password>\"}"
```

2) Use token to send:

```bash
curl.exe -s -X POST http://127.0.0.1:8080/v1/notifications/quick-send ^
  -H "Authorization: Bearer <token>" ^
  -H "Content-Type: application/json" ^
  -d "{\"app_id\":\"<app_id>\",\"channel\":\"email\",\"to\":\"someone@example.com\",\"title\":\"trace test\",\"message\":\"hello\"}"
```

Expected:
- API enqueues
- Worker processes and attempts provider send (even if provider fails, you still get spans)

---

## 5) Find traces in Grafana (Tempo)

In Grafana:

1) **Explore**
2) Select datasource **Tempo**
3) Use one of:

### Search by service name

- **Service name**: `freerange-notification-service`
- Run query for “last 15 minutes”

Then click a trace → confirm you see spans named like:
- `HTTP GET /v1/health` (or similar; exact naming depends on middleware)

### Search by worker service name

- **Service name**: `freerange-notification-worker`
- Time range: last 15 minutes

Then click a trace → confirm you see:
- `worker.processNotification`
- Attributes like `notification.id`, `app.id`, `channel`

---

## 6) Common problems + fixes

### No traces in Grafana at all

- Confirm Tempo is ready:
  - `http://127.0.0.1:3200/ready` returns `200`
- Confirm Grafana is reachable:
  - `http://127.0.0.1:3001/login` returns `200`
- Confirm the Tempo datasource exists:
  - Grafana → **Connections → Data sources** → Tempo present

### API/worker started but still no traces

- Confirm env is set inside containers:

```bash
docker exec -it freerange-notification-service sh -lc "env | grep FREERANGE_OTEL"
docker exec -it freerange-notification-worker sh -lc "env | grep FREERANGE_OTEL"
```

- Confirm the collector is listening:

```bash
docker logs --tail 50 freerange-otel-collector
```

You should see OTLP receivers on `0.0.0.0:4317` and `0.0.0.0:4318`.

### Collector can’t export to Tempo

Collector log can show `connect: connection refused` briefly while Tempo boots.
If it persists:

- Ensure Tempo OTLP is bound to `0.0.0.0` (not localhost):
  - `config/tempo.yaml` should set:
    - `grpc.endpoint: 0.0.0.0:4317`
    - `http.endpoint: 0.0.0.0:4318`

Restart the observability stack:

```bash
docker compose -f docker-compose.yml -f docker-compose.otel.yml up -d --force-recreate tempo otel-collector grafana
```

### Worker traces missing but API traces present

- Ensure the worker also has:
  - `FREERANGE_OTEL_ENABLED=true`
  - `FREERANGE_OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317`
  - `FREERANGE_OTEL_SAMPLE_RATIO=1`
- Generate a notification that actually hits the queue and is processed.

---

## 7) What “success” looks like

- Grafana Explore shows traces for **both**:
  - `freerange-notification-service`
  - `freerange-notification-worker`
- Clicking a trace shows:
  - API span(s) for HTTP route(s)
  - Worker span(s) for processing
- Even if a provider send fails, you still see the worker span with an error status.

