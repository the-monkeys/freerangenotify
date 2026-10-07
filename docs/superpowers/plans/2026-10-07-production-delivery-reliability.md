# Production Delivery Reliability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Fix custom SMTP delivery, misleading provider/credit errors, and reservation recovery gaps while preserving existing customer integrations.

**Architecture:** Three separate patches: provider correctness, credit diagnostics/UI, then durable lifecycle hardening. Keep public contracts and pricing stable; infrastructure/provider-account recovery is a separate verified operation.

**Tech Stack:** Go, existing Elasticsearch v8 and Redis v8 clients, Fiber, React/TypeScript, Docker Compose. No new database/provider SDK.

**Spec:** docs/superpowers/specs/2026-10-07-production-delivery-reliability-design.md

**Status:** Implemented and locally verified on codex/production-delivery-reliability; prepared for staged review. No production operations or commits have been performed.

## Global constraints

- Preserve settings.email_config names and existing system-default/incomplete-config behavior.
- Preserve asynchronous acceptance, response envelopes, notification statuses, legacy billing, daily-cap policy, current channel prices, and balances.
- Preserve Provider.IsHealthy(context.Context) bool and errors.Is(err, ErrInsufficientCredits).
- Preserve error_message; new diagnostic metadata is optional.
- No automatic historical/OTP replay, retroactive debit, or reservation reset.
- Keep unrelated local changes untouched; stage only relevant files.
- No production writes/restarts during investigation.

## Review focus

- Different apps' SMTP settings stay isolated under concurrent sends and fallback.
- Customer SMTP works without system SMTP or a default email provider.
- Unknown/temporary errors do not accidentally become terminal authentication failures.
- Provider acceptance plus ledger failure causes neither a second send nor a second debit.
- Restart, Redis expiry/outage, and competing reapers preserve unrelated holds.

## Investigation and test method

Four desired-behavior probes already fail against unchanged product code: wrong SMTP health endpoint, 535 retry/classification, repeated commit after ledger failure, duplicate release from separate service instances. They use temporary Go overlays; promote them into permanent tests during their owning tasks.

Focused existing service/handler/worker/provider tests passed. They do not cover real ES script behavior, Redis TTL loss, inter-store crash windows, or competing terminal transitions. Real ES/Redis tests are required for patch 3.

## Patch 1: custom SMTP correctness and actionable errors

### Task 1: isolate application SMTP resolution, health and send

**Files**
- Modify internal/infrastructure/providers/smtp_provider.go, manager.go, provider.go.
- Update cmd/worker/main.go and processor.go only if needed for explicit-provider resolution.
- Extend internal/infrastructure/providers/smtp_provider_test.go.
- Create internal/infrastructure/providers/manager_smtp_test.go.

**Interfaces**
- Consume existing EmailConfigKey, application.EmailConfig, provider factory, and manager APIs.
- Add optional HealthChecker interface with CheckHealth(ctx context.Context) error; SMTP implements it and preserves IsHealthy as a bool wrapper.
- Add private SMTPProvider.resolveConfig(ctx context.Context) (SMTPConfig, string), returning config and existing credential-source value.
- Resolve explicit application SMTP before channel default selection. Instantiate a request-scoped factory adapter when system SMTP is absent; never register it as the channel default.

- [x] Write failing local-server tests: unavailable system/healthy customer; healthy system/unavailable customer; absent system/default provider; two apps using distinct endpoints; omitted port defaults to 587; unchanged system-default/incomplete-config paths.
- [x] Add deadline/cancellation, STARTTLS submission, implicit TLS 465, and configured nonstandard-port fixtures. Keep existing MIME/attachment/sender behavior and supported unauthenticated SMTP covered.
- [x] Run go test ./internal/infrastructure/providers -run 'TestSMTPProvider|TestManager.*SMTP' -count=1 and verify expected failures.
- [x] Implement request-local resolution and correct endpoint probing with bounded, cancelable network operations; no shared last-error/config cache.
- [x] Rerun focused tests and go test ./internal/infrastructure/providers -count=1. Verify default-provider ordering and configured fallback paths.
- [x] Stage/review this patch. Commit only when requested.

### Task 2: retain error category/cause and retry safely

**Files**
- Create internal/infrastructure/providers/delivery_error.go and delivery_error_test.go.
- Modify manager.go, smtp_provider.go, cmd/worker/retry_classify.go and processor.go.
- Create cmd/worker/retry_classify_test.go; extend processor_test.go and provider tests.

**Interfaces**
- Add DeliveryError with provider/type/stage/credential-source/retryable fields and Unwrap() error.
- Retain provider signatures and existing ErrorType values; wrap unsuccessful Result at the manager boundary and preserve error chains through worker/fallback paths.
- Optional metadata: failure_code, failure_stage, provider, credential_source, retryable.

- [x] Test SMTP 535/Twilio 20003: terminal delivery attempt, no burn, reservation released, category retained after wrapping. DNS SERVFAIL/timeouts/SMTP 4xx/HTTP 429 or 5xx remain retryable. Unknown errors retain current retry policy.
- [x] Test failed Result with nil Go error does not count as circuit success; BYOC auth/config failures do not open shared system breakers or affect a second app. Exercise explicit fallback chains.
- [x] Test history sanitization and backward-compatible error_message. Do not persist credentials, tokens, or raw sensitive upstream bodies.
- [x] Observe failures, then implement typed errors, auth short-circuiting, cancellation-aware waits, and optional metadata.
- [x] Run go test ./internal/infrastructure/providers ./cmd/worker -count=1 and review attempt limits/hold cleanup.
- [x] Stage/review independently from billing.

## Patch 2: authoritative credit diagnostics and UI

### Task 3: distinguish contention, exhausted funds and daily caps

**Files**
- Modify internal/domain/billing/credits.go, internal/infrastructure/billingrepo/subscription_credit_balance_repo.go, internal/usecases/services/credit_service.go, cmd/worker/processor.go.
- Add a dedicated structured script-response operation in internal/infrastructure/repository/subscription_repository.go; preserve generic ScriptUpdate.
- Extend credit_service_reservation_test.go, credit_service_legacy_test.go, and processor_test.go.
- Create billingrepo/subscription_credit_balance_repo_test.go and an ES concurrency fixture under tests/integration/.

**Interfaces**
- Add CreditUnavailableError carrying reason/cost/remaining/reserved/available/card version; Error() retains "insufficient credits", Unwrap() retains ErrInsufficientCredits.
- Structured reserve outcome comes from the atomic response with source retrieval; rejected capacity uses a no-op outcome. Never map storage/conflict errors to insufficient funds or log a pre-read as the rejection snapshot.
- Reasons: insufficient_credits, credits_temporarily_reserved, existing daily_cap_exceeded.

- [x] Test remaining=1500, SMS cost=800, pending hold=800: rejection observes reserved=800/available=700. Following release a bounded retry succeeds; following sibling commit remaining=700 becomes terminal exhaustion.
- [x] Test exactly sufficient/zero funds, channel aliases, card refresh between cost/version reads, bounded 409 exhaustion, malformed/database errors, and no fabricated depletion state.
- [x] Test legacy quotas, free daily caps, billing absent, and wired service/enforcement disabled. Guard nil reservations in worker metadata handling.
- [x] Observe failures; implement structured observations, truthful WARN/metadata, bounded queue retry only for contention. Honor notification/OTP validity windows. Undo only the failed attempt's daily-cap token.
- [x] Run go test ./internal/usecases/services ./cmd/worker ./internal/infrastructure/billingrepo -count=1, then real ES concurrency tests.
- [x] Stage/review with no rate-card or balance changes.

### Task 4: display costs and historical failure context

**Files**
- Modify ui/src/components/AppNotifications.tsx, Sidebar.tsx, ui/src/types/index.ts.
- Reuse billingAPI.getRates/getUsage from ui/src/services/api.ts.
- Add browser cases to the existing e2e suite.

- [x] Write scenarios: SMS cost 800, wallet 1500; one recipient estimates 800; two known recipients estimate 1600; unknown broadcast recipient count shows only per-recipient cost.
- [x] Test old API responses without new fields, old notifications with only error_message, billing-disabled/legacy modes, and rates-fetch failure. Estimates stay advisory; no send acceptance contract changes.
- [x] Test DNS/auth/contention/depletion/daily-cap messaging. A later current balance is not the earlier attempt's snapshot.
- [x] Implement optional diagnostic rendering with old-data fallbacks, cost estimates from active rates, and sent_at under Sent At. Unsent failures show no delivery time; expose created_at separately.
- [x] Run npm --prefix ui run build and targeted Playwright tests; visually verify one custom SMTP and one two-recipient SMS case.
- [x] Stage/review separately; optional fields support either compatible backend version.

## Patch 3: durable lifecycle and ledger recovery

### Task 5: idempotent reservation transitions across failures/restarts

**Files**
- Modify billing/credits.go, license/models.go, billingrepo/subscription_credit_balance_repo.go, billingrepo/es_credit_ledger_repo.go, repository/subscription_repository.go, database/index_templates.go.
- Modify services/credit_service.go, credit_reservation_store.go, credit_reservation_store_redis.go, and worker reaper wiring.
- Create tests/integration/credit_reservation_lifecycle_test.go; extend service regressions.

**Interfaces/invariants**
- Reservation-aware reserve/commit/release operations return durable transition outcome and exact operation balance, bound to reservation ID and original subscription ID.
- Receipt and delta change in one ES update. Add source-only credit_reservation_journal with enabled:false object mapping, installed before journal writes.
- Redis becomes an index/cache. Deterministic transition-type/reservation-ID ledger rows repair without applying the balance delta again.
- Reservation/reaper APIs surface storage errors; absence is distinct from failure.

- [x] Promote probe: ledger failure after burn + retry leaves 99 from 100, not 98. Promote probe: duplicate release preserves another one-credit hold.
- [x] Test duplicate/competing commit/release, crashes before/after each ES/Redis/ledger write, Redis outage and more-than-24-hour expiry, missed reaper, renewal, and successful delivery plus accounting outage. Assert one terminal delta and no accounting-induced resend.
- [x] Audit and test every full subscription writer: payment, renewal, ops, bootstrap. Stale updates must not overwrite counters/journal.
- [x] Observe failures; implement authoritative receipts and deterministic ledger repair; paginate reaping from authoritative pending receipts. Retain terminal receipts initially, expose document-growth metrics, and defer pruning until delayed-replay safety is proven.
- [ ] Run unit/race tests on supported CI and real ES/Redis fault scenarios. A Redis lock or deterministic ledger ID alone does not establish crash safety.
- [x] Prepare paused-worker drain/import and canary flag. Do not mix old/new lifecycle writers per tenant. Quarantine unmatched old counters; preserve balances. Review ops reset so it cannot clear active journal-backed holds.
- [x] Stage/review separately. Rollback stops new admissions, preserves recovery, drains journal-backed reservations, and only then restores old writers.

## Operations and controlled rollout


### Task 6: DNS/provider credentials/tracing recovery runbook

**Files**
- Create documents/production-delivery-recovery-runbook.md.
- Review targeted changes in docker-compose.otel.yml, config/otel-collector.yaml, prod/deploy.sh.
- DNS override is deployment-specific and opt-in; keep unrelated local port edits out.

- [ ] Recheck DNS for the application's actual SMTP host inside the worker and other external/internal names. Verify TCP/TLS without printing credentials or sending unauthorized test messages.
- [ ] Identify system versus BYOC credentials for HelixCare's failing SMTP/Twilio rows; correct the owning account through the authorized operator path. Code cannot activate an inactive account or infer a correct password.
- [ ] Validate a chosen DNS remedy with docker compose config; preserve internal discovery; recreate only services whose DNS configuration changed. No static SMTP IPs/public-resolver defaults.
- [ ] Restore OTel independently with restart policies, correct merged/standalone network behavior, production tags and bounded exporters. Validate each supported Compose combination.
- [ ] Canary patch 1 using an authorized recipient for a custom-SMTP app, a system-default app, and configured fallback. Verify delivered sender, retry category, and released holds.
- [ ] Canary patch 2 with the 1500/800 fixture. Canary patch 3 only after fault tests/cutover review.
- [ ] Observe queue depth, reserved-credit age, delivery errors, ledger repair backlog and document size before broader rollout.
- [ ] Retry only explicitly selected, still-valid business notifications; no blanket DLQ replay or expired OTP replay.

## Review and execution handoff

The approved implementation was completed locally and independently reviewed across providers, worker handling, UI, operations, and billing. Review the staged changes and the recovery runbook before an authorized rollout. Staging does not perform a commit, deployment, account correction, migration, or replay.

For each patch, watch new tests fail, implement, then rerun focused tests and relevant suites. Before calling the final branch ready, run required project checks and report unavailable integration environments accurately. Preserve async acceptance with handler contract tests and old-data UI fixtures.

## Local acceptance evidence (2026-10-07)

- Final repository short suite passed: go test -short ./... -count=1.
- Real disposable Elasticsearch 8.18 and Redis 7 fixtures: 20 billing scenarios and one notification-history scenario passed, plus four lost-response subcases (47.396s). Coverage includes competing terminal transitions, original-subscription binding, Redis loss/expiry, pagination, immutable ledger repair, per-attempt delivery evidence, old/new allocation contracts, captured-payment drain/retry, and keyed renewal replay before/after adoption.
- Full affected provider/worker/service/handler suites passed. SMTP STARTTLS and port 465 fixtures both ran. Worker tests confirm one provider invocation despite commit, ledger, or acknowledgement faults; expired OTP cleanup uses independent bounded contexts.
- UI build passed; 33 browser cases passed with local HTTP fixtures. Visual checks covered two-recipient SMS cost and custom-SMTP history.
- All 16 deployment checks passed; the pinned collector 0.130.0 binary validated the configuration.
- Independent source reviews have no remaining reported findings. Whitespace checks passed.

The full live-stack suite failed because its existing localhost:9200 Elasticsearch and notification-service fixtures were unavailable. It is not claimed green. Local race checks are unavailable with CGO_ENABLED=0 and no C compiler; supported CI remains required. Production DNS/account credentials, actual delivery/tracing canaries, and coordinated cutover remain unrun operator gates.

The journal admissions flag defaults false. Its durable mode marker protects adopted subscriptions even before their first reservation and during rollback. Confirmed failed attempts retain receipt evidence; successful status and its delivery reservation binding persist atomically. Unknown outcomes stay quarantined. Keyed renewal intent is optional; unkeyed clients retain existing semantics and uncertain manual retries require reconciliation. No automatic legacy importer/reset, retroactive debit, historical replay, or receipt pruning was added.
