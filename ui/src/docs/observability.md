# Monitoring and privacy

## Local development

Use the standard Compose project for local work. Telemetry is optional; leave `FREERANGE_OTEL_ENABLED=false` unless you intentionally run a collector. Running the billing UI does not require the telemetry Compose file.

## Tracing configuration

Tracing is enabled only when `FREERANGE_OTEL_ENABLED=true`. The application exports OTLP over gRPC to `FREERANGE_OTEL_EXPORTER_OTLP_ENDPOINT`, defaulting to `otel-collector:4317`. Export calls have a five-second timeout. The configured transport is insecure gRPC; keep it on a trusted internal network or provide a separately secured transport path.

`FREERANGE_OTEL_SAMPLE_RATIO` accepts a value from 0 to 1 and defaults to 0.1. Sampling is parent-based, so an upstream sampled parent can affect whether a request is traced. Set `FREERANGE_OTEL_ENV=production` for production; otherwise the application environment is used. Resources include service name, service version and deployment environment.

Request and worker instrumentation can include route, method, status, timing and application, notification or channel identifiers. Do not treat sampled traces as a complete financial ledger. Financial operations, immutable events and saved payment records are the source for reconciliation.

## Customer links and secrets

Portal requests are protected before normal request logging and automatic tracing. Raw bearer URLs must not appear in exported spans or request logs. Customer portal responses and initial pages use no-store and no-referrer policies. New portal credentials are stored as digests; delivery-link copies needed for notification delivery are encrypted.

Do not log payment account secrets, raw callback bodies, Authorization headers or private portal links. Do not enable a detailed debug exporter in production. Redaction in this application is not a promise that an external reverse proxy, browser plugin or custom provider logger removes the same data; configure those separately.

## Operational checks

Monitor pending and unknown financial operations, failed receipt admission, delivery requiring review, growing queue age, dead-letter depth and collector availability. A completed payment with a failed message is still a completed payment. A queued message is not a delivered receipt.

Retention is controlled by the deployed stores and observability backend configuration. The application tracing initializer does not set a universal retention period. Restrict access and configure retention for logs, traces, backups and financial records according to your business requirements.

See [payment recovery](/docs/business-billing-payments) before retrying an uncertain charge or refund. Rollout and data recovery procedures are maintained in the repository's business billing rollout runbook.
