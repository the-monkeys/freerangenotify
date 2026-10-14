# WhatsApp and SMS Reliability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans for native execution, or superpowers:subagent-driven-development if selected. Complete tasks in order; checkboxes track verified progress.

**Goal:** Give each customer a secure, explicit WhatsApp/SMS connection and affordable, auditable delivery while preserving existing integrations.

**Architecture:** Reuse the provider manager, notification queue and credit journal. Introduce encrypted app/environment connections, immutable delivery quotes and durable authenticated callback receipts. Keep legacy adapters and tariffs readable while enabling new routes and pricing through staged flags.

**Tech Stack:** Go, Fiber, Elasticsearch CAS, Redis, React/TypeScript, existing Meta/Twilio/Vonage clients, proposed Exotel HTTP adapter, Go mock HTTP servers and Playwright.

**Spec:** `docs/superpowers/specs/2026-10-13-whatsapp-sms-reliability-design.md` (Phases 1–2 approved for local implementation).

## Global Constraints

- One local branch: `codex/whatsapp-sms-reliability`; five phases with task commits and phase checkpoints. Preserve existing working changes; do not push/deploy/send live messages or edit real `.env` values during implementation.
- Keep legacy API-key authentication, channel names, Twilio `content_sid`, Meta `whatsapp_template`, rich-template payloads and legacy config/env names readable.
- Security corrections remove secrets/master keys from ordinary reads; omitted secrets retain values and explicit clear removes them. Update dashboard clients and release notes together.
- Do not rewrite active tariffs, historical balances or queued reservation prices. New detailed pricing requires an explicitly activated immutable version.
- Unknown provider outcomes retain holds and cannot trigger blind retry, fallback or refund.
- New Exotel routes and detailed tariffs start disabled. Required encrypted writes follow deployment of all compatible readers.
- Every task follows RED test, minimal implementation, GREEN verification, scoped diff review and a local commit. No unrelated cleanup.

## Review Focus

1. Ordinary viewer reads must not disclose a master key that grants API-key-only template writes: Task 1. Previously copied keys require separate operator-directed rotation.
2. Two phone numbers in one WABA and an application beyond the first 500 records must route correctly: Task 2.
3. A callback crash after storage but before workflow delivery must resume without duplicate business actions: Task 3.
4. A queued send surviving disconnect/rotation must retain its admitted identity and price: Tasks 4 and 7.
5. An unchanged secret in a stale browser form must not overwrite a newly connected account: Tasks 1 and 5.

## Phase 1 — Security and customer isolation

**Local status:** Tasks 1–3 complete. One independent review found eight blockers; all fixed with regression tests. Phase 2 implemented locally; review pending. Phases 3–5 remain pending. No production changes or push.

**Implementation decisions:** Routing and durable receipt wiring share one task commit so signed callbacks cannot ship without recovery. Unresolved sender mappings fail closed with provider retry; no unauthenticated quarantine payload is stored. Ambiguous external workflow/webhook outcomes are quarantined rather than replayed. SSE refresh signals can repeat. Twilio receipt time cannot open CSW without provider creation time; Phase 3 needs an account-bound timestamp resolver. Details and rollout requirements: documents/production-delivery-recovery-runbook.md.

### Task 1: Safe configuration responses and consistent authorization

**Files:** Modify `internal/interfaces/http/dto/application_dto.go`, `handlers/application_handler.go`, `handlers/custom_provider_handler.go`, `handlers/whatsapp_admin_handler.go`, `middleware/dashboard_rbac.go`, `routes/routes.go` (handler/middleware paths are under `internal/interfaces/http/`); `ui/src/pages/AppDetail.tsx`, `ui/src/components/apps/AppProviders.tsx`. Create `internal/interfaces/http/dto/application_settings_response.go`, `internal/interfaces/http/handlers/channel_authorization.go`, and corresponding `_test.go` files; browser test `e2e/whatsapp-sms-settings.spec.ts`.

**Interfaces:** Produce `ToSafeApplicationSettings(settings application.Settings) SafeApplicationSettings` and a shared `AuthorizeChannelAccess(ctx context.Context, userID, appID string, manage bool) error` service under `internal/usecases/services/channel_authorization.go`. Existing settings writes retain their field names; add configured flags and explicit clear fields. Existing API-key callers remain valid; ordinary read responses no longer distribute master keys.

- [x] Write `TestChannelSettingsSecrets` asserting viewer/editor/ordinary admin reads contain no token, password, custom secret header or unrestricted API key; owner reveal requires a separate authorized action. Assert omission preserves credentials, explicit clear deletes them and stale/masked writes cannot restore secrets.
- [x] Run `go test ./internal/interfaces/http/dto ./internal/interfaces/http/handlers ./internal/interfaces/http/middleware -run 'TestChannel(SettingsSecrets|Authorization)' -count=1`; verify RED assertions, not setup errors.
- [x] Implement safe response mapping, unified tenant/app authorization, owner-only reveal, credential-preserving writes and sanitized transport errors. Update UI to use configured flags without echoing credentials. Reject unapproved secret-bearing read endpoints; document the intentional security change.
- [x] Re-run targeted tests; add browser assertions for edit-without-token and viewer permissions. Inspect captured logs/responses for fixture-secret absence.
- [x] Review/stage only Task 1 paths and commit `fix: isolate channel configuration secrets and permissions`.

### Task 2: Scoped inbox SSE and authenticated sender routing

**Files:** Modify `internal/infrastructure/sse/broadcaster.go`, `internal/usecases/services/whatsapp_service_impl.go`, `internal/interfaces/http/handlers/meta_webhook_handler.go`, `twilio_inbound_webhook_handler.go`, `twilio_content_status_handler.go`, `internal/container/container.go`, `internal/interfaces/http/routes/routes.go`. Create `internal/usecases/services/channel_sender_resolver.go`, `internal/infrastructure/repository/channel_sender_binding_repository.go`, and matching tests.

**Interfaces:** Produce `ResolveInboundSender(ctx context.Context, provider, accountID, senderID string) (*InboundSenderBinding, error)` and `PublishInboxEvent(ctx context.Context, appID, environmentID, eventType string, data map[string]interface{}) error`. `InboundSenderBinding` contains app/environment/connection identities and signing context. Legacy user-targeted SSE stays valid; unrouted inbox messages fail closed.

- [x] Write `TestChannelInboxIsolation`, `TestChannelSenderRouting` and `TestChannelWebhookSignatures`: two tenants, revoked membership, two numbers in one WABA, more than 500 apps, duplicate sender assignments, full Twilio form including `MediaUrl1`, forged forwarded host, missing secrets and mismatched AccountSid.
- [x] Run `go test ./internal/infrastructure/sse ./internal/interfaces/http/handlers ./internal/usecases/services -run 'TestChannel(InboxIsolation|SenderRouting|WebhookSignatures)' -count=1`; verify RED.
- [x] Implement explicit authorized SSE audience and complete form/canonical-URL verification. Add durable sender lookup/claims, phone-first Meta routing with WABA agreement, account-specific verification and protected content-status callbacks. Quarantine ambiguous legacy shared-sender inbound events.
- [x] Re-run tests and a disposable-ES concurrency test proving conflicting new sender claims yield one owner. Confirm a private event never enters BroadcastToAll.
- [x] Commit `fix: scope WhatsApp events and verify sender callbacks`.

### Task 3: Durable callback receipt and inbox state recovery

**Files:** Create `internal/domain/whatsapp/events.go`, `internal/infrastructure/repository/channel_event_repository.go`, `internal/usecases/services/channel_event_processor.go`, `channel_event_processor_test.go`, `tests/integration/channel_events_test.go`. Modify `internal/domain/whatsapp/models.go`, `repository.go`, `internal/infrastructure/repository/whatsapp_repository.go`, `notification_repository.go`, WhatsApp service/handlers and worker bootstrap.

**Interfaces:** Produce `AcceptVerifiedEvent(ctx context.Context, binding *InboundSenderBinding, event *whatsapp.VerifiedEvent) (*whatsapp.EventReceipt, error)` and `ProcessEventReceipt(ctx context.Context, receiptID string) error`. Receipts track durable stages and individual side-effect identities, not just a processed boolean. Notification transitions use conditional repository updates.

- [x] Write `TestChannelEventRecovery`: replay the same message twice (one row/workflow effect), crash at each durable stage, transient storage failure (no success ack), reversed/concurrent statuses, late failure after read, delayed inbound timestamp, and mark-read reducing the local unread count.
- [x] Run `go test ./internal/usecases/services ./internal/infrastructure/repository -run 'TestChannelEvent' -count=1`; verify RED.
- [x] Implement authenticated durable acceptance, idempotent stages/outbox, bounded recovery and dead-letter records. Derive CSW expiry from validated upstream time; CAS transitions and actual local read markers replace read-modify-write/status/count shortcuts. Expose auto-reply as unavailable until normal-dispatch implementation exists.
- [x] Run targeted tests and `go test ./tests/integration -run '^TestChannelEventsRealES' -count=1 -v` against disposable fixtures. Prove restart recovery and no duplicate side effects.
- [x] Commit `fix: recover WhatsApp callbacks and preserve inbox state`.

**Phase 1 checkpoint: passed locally.** Full Go short suite; 43 reliability browser cases; seven disposable ES/Redis scenarios; production UI build (existing chunk warning); Linux SSE race regressions. Credential scan and security diff reviewed. Full services/providers/handlers Linux race coverage remains a Phase 5 CI requirement. No deployment, production migration, key rotation or live sends performed.

## Phase 2 — Explicit customer connections and onboarding

### Task 4: Encrypted connections, resolution and migration

**Files:** Create `internal/domain/channelconnection/models.go`, `repository.go`, `internal/infrastructure/repository/channel_connection_repository.go`, `internal/infrastructure/secrets/connection_codec.go`, `internal/usecases/services/channel_connection_service.go`, tests and `cmd/frn/channel_connections.go`. Modify `internal/domain/application/models.go`, `internal/config/config.go`, `internal/container/container.go` and relevant index initialization.

**Interfaces:** Produce `ResolveDeliveryConnection(ctx context.Context, appID, environmentID string, channel notification.Channel) (*channelconnection.Snapshot, error)` and `UpdateConnection(ctx context.Context, id string, expectedRevision int64, patch channelconnection.Patch) (*channelconnection.Connection, error)`. Snapshot includes stable connection/revision/provider/source/sender fields. `ConnectionCodec.Seal(plaintext []byte, binding SecretBinding) (EncryptedSecret, error)` and `Open(secret EncryptedSecret, binding SecretBinding) ([]byte, error)` bind ciphertext to connection/app/environment/provider/field; define those types in `connection_codec.go`.

- [x] Write `TestChannelConnectionCompatibility`: legacy Twilio fields, BYOC without global credentials, environment-before-parent resolution, CAS conflict, ciphertext/key rotation, wrong associated data, missing key rejection, queued old-revision identity and redacted dry-run output.
- [x] Run `go test ./internal/infrastructure/secrets ./internal/usecases/services -run 'TestChannelConnection' -count=1`; verify RED.
- [x] Implement AES-256-GCM keyring, encrypted repository, legacy read adapter, immutable snapshots and explicit disconnect behavior. Add resumable dry-run-first migration with CAS/checkpoints; do not run it on customer data. Wire Task 2 sender resolver to connections without losing legacy recovery.
- [x] Run targeted tests plus `TestChannelConnectionsRealES`; verify two concurrent updates cannot overwrite each other and migration interruption leaves readable records. Keep encrypted writes disabled until compatible reader rollout.
- [x] Commit `feat: add encrypted application channel connections`.

### Task 5: Validated onboarding and truthful environment/template UI

**Files:** Modify WhatsApp admin/template/rich-template handlers/services, `custom_provider_handler.go`, `ui/src/components/whatsapp/WhatsAppConnect.tsx`, `ui/src/pages/AppDetail.tsx`, `ui/src/components/apps/AppEnvironments.tsx`, `AppProviders.tsx`, `ui/src/pages/whatsapp/*Templates.tsx`, `ui/src/services/api.ts`. Create `internal/interfaces/http/handlers/channel_connection_handler.go`, handler tests and `e2e/whatsapp-sms-connections.spec.ts`.

**Interfaces:** Add JWT/app-authorized `GET/PUT /v1/apps/:id/channel-connections/:channel` and `POST /v1/apps/:id/channel-connections/:channel/validate`, with optional environment/revision fields. Validation returns readiness facts and never sends a paid message. Add explicit rich draft update/submit operations and provider-template ownership checks.

- [x] Write `TestChannelConnectionOnboarding`: manual Meta without SDK keys, selected phone absent from WABA, empty phone list, forbidden tenant/viewer access, app-load failure without system fallback, shared-account template mutation denied, stale revision and imported-provider links/actions.
- [x] Run handler tests matching `TestChannelConnectionOnboarding`; verify RED. Add browser scenarios tagged `WhatsApp SMS connections` for connect-then-unrelated-save, disconnect-then-save, selected environment persistence and saved-versus-submitted draft messages.
- [x] Implement full phone selection/ownership validation and readiness; refresh parent configuration and patch changed fields. Scope template bindings/account permissions, draft submission and imported resources. Present explicit load/permission errors and separate local removal from vendor deletion.
- [x] Run targeted Go tests and `npx playwright test --config ui/playwright.channels.config.ts --workers=1`; create this config using the existing reliability config's fixture/server pattern, with grep `WhatsApp SMS connections` and separate output directory. Confirm legacy settings requests still work.
- [x] Commit `feat: validate customer messaging connections and environments`.

**Phase 2 checkpoint:** local fixture validation proves two independent Meta customers need no global sender credentials, and scoped connection resolution selects the environment override before the application default.

**Local implementation status:** Tasks 4–5 implemented. Go suite, production-schema Elasticsearch CAS/recovery, UI build and 47 browser cases verified; independent review pending. Encrypted writers remain disabled until Phase 3 and complete fleet reader rollout.

## Phase 3 — Shared dispatch, pricing and delivery accounting

### Task 6: Deterministic outbound routing and timeout boundaries

**Files:** Modify `cmd/worker/main.go`, `processor.go`, `internal/infrastructure/providers/manager.go`, `registry.go`, `twilio_provider.go`, `whatsapp_provider.go`, `meta_whatsapp_provider.go`, `vonage_provider.go`. Create `internal/usecases/services/channel_dispatch_service.go`, worker/provider regression tests.

**Interfaces:** Produce `PrepareChannelDelivery(ctx context.Context, notif *notification.Notification, usr *user.User) (*channelconnection.PreparedDelivery, error)` and `DispatchPrepared(ctx context.Context, delivery *channelconnection.PreparedDelivery) (*providers.Result, error)`. Resolve identity before financial admission; store safe snapshot metadata and retrieve secrets privately. Preserve Provider interface and current notification payloads.

- [ ] Write `TestChannelDispatchRouting`: Twilio BYOC plus Content SID uses customer account, Meta template stays Meta, SMS Twilio/Vonage selection independent of map order, missing app sender never inherits a foreign sender, app load/disconnect fail closed, timeout/cancellation and lost acceptance response cause no fallback.
- [ ] Run `go test ./cmd/worker ./internal/infrastructure/providers ./internal/usecases/services -run 'TestChannelDispatch' -count=1`; verify RED.
- [ ] Implement request-scoped BYOC adapters without global bootstrap requirements, explicit provider/source selection and contextual HTTP deadlines. Apply the Task 3 CSW check to all free-form WhatsApp sends. Remove synthetic success IDs for malformed acceptance responses; preserve unknown-outcome quarantine.
- [ ] Re-run targeted tests with mock servers that count accepted requests. Confirm legacy empty provider defaults to Twilio and configured explicit choices never drift across accounts.
- [ ] Commit `fix: bind messaging delivery to the admitted customer account`.

### Task 7: Immutable quotes with BYOC, category and segment dimensions

**Files:** Create `internal/domain/billing/channel_quote.go`, `internal/usecases/services/channel_quote_service.go` and tests. Modify `internal/domain/billing/credits.go`, `rates.go`, `internal/usecases/services/ratecard_service.go`, `credit_service.go`, `credit_service_journal.go`, `internal/infrastructure/billingrepo/es_rate_card_repo.go`, worker admission and billing DTO/UI; create `tests/integration/channel_quote_settlement_test.go`.

**Interfaces:** Produce `QuoteChannelDelivery(ctx context.Context, prepared *channelconnection.PreparedDelivery) (*billing.ChannelQuote, error)` and `ReserveQuotedDelivery(ctx context.Context, tenantID, appID, notificationID string, quote *billing.ChannelQuote) (*billing.CreditReservation, error)`. Quote contains immutable rate-card version, pricing model, connection identity, credit amount, unit count, source, country/category and estimated vendor cost currency. Add detailed rate dimensions while preserving legacy flat cards and existing `Reserve` callers through their compatibility adapter.

- [ ] Write `TestChannelQuote`: unchanged old card costs; BYOC fee from an explicitly activated detailed card; GSM 160/161 and UCS-2 70/71 boundaries, extension characters, emoji and exact rendered body; destination/category differences; spoofed client cost/free flags rejected; quote survives rate refresh and queued account changes.
- [ ] Run `go test ./internal/domain/billing ./internal/usecases/services -run 'TestChannelQuote' -count=1`; verify RED.
- [ ] Implement server-derived quote inputs, segment counting, immutable tariff dimensions, admitted snapshots and atomic exactly-once settlement. Separate vendor delivery costs/late failures from FRN processing charges. Preserve current tariffs and old reservations; no automatic new rate activation or retrospective billing.
- [ ] Run targeted tests and `TestChannelQuoteSettlementRealES` with concurrent admission/replay/release. Check wallet, ledger and usage agree and a late callback cannot charge or refund twice. Document provider rate verification/activation prerequisites without inventing prices.
- [ ] Commit `feat: quote messaging costs by connection and billable units`.

### Task 8: Inbox replies and phone OTP through shared dispatch

**Files:** Modify `internal/usecases/services/whatsapp_service_impl.go`, `auth_service_impl.go`, `otp_service.go`, `internal/interfaces/http/handlers/whatsapp_conversation_handler.go`, `auth_handler.go`, `internal/container/container.go`, `ui/src/pages/whatsapp/WhatsAppConversations.tsx`. Create `internal/usecases/services/phone_otp_dispatch.go`, tests and browser reply cases.

**Interfaces:** Preserve `whatsapp.Service.Reply` and existing HTTP request fields; internally enqueue through notification/quote admission and add notification/accepted status fields to the response. Produce `DispatchPhoneVerificationOTP(ctx context.Context, userID, phone, code string) error` using a platform authentication connection/budget rather than a customer's wallet.

- [ ] Write `TestChannelInboxReplyAccounting` and `TestChannelPhoneOTPDispatch`: Meta/Twilio replies produce one outbound record/provider ID/charge, expired CSW requires template, replay does not resend, plus-phone encoded correctly, missing configuration returns failure, code never logged, expiry/attempt limits unchanged, user/destination/IP budgets reject excess.
- [ ] Run targeted service/handler tests for these names; verify RED.
- [ ] Replace direct Meta inbox HTTP calls and raw-env/manual-form OTP code with shared dispatch. Enforce spec defaults (3/user/10min, 3/destination/10min, 10/IP/10min), destination allowlist and operator spend budget; Redis budget errors fail closed. Keep phone verification independent of the system-send verified-phone gate.
- [ ] Run targeted tests and browser queued-reply/failure cases. Assert notification history, conversation history, usage and wallet consistency; API failure cannot claim an OTP was sent.
- [ ] Commit `fix: account for inbox replies and validate OTP dispatch`.

**Phase 3 checkpoint:** legacy Twilio templates, Meta templates, SMS, inbox replies and OTP all retain identity and use the intended accounting path. New prices remain inactive.

## Phase 4 — Domestic SMS and operational controls

### Task 9: Exotel adapter, DLT readiness and sender budgets

**Files:** Create `internal/infrastructure/providers/exotel_provider.go`, `exotel_provider_test.go`, `internal/usecases/services/channel_delivery_budget.go` and tests. Modify connection models/service/handler, config/bootstrap, provider factory registration and UI SMS fields; add integration/browser cases.

**Interfaces:** Exotel implements the existing Provider interface with app-scoped encrypted credentials, approved sender, DLT entity/template IDs and per-connection throughput. `AcquireDeliveryBudget(ctx context.Context, snapshot *channelconnection.Snapshot, units int64) error` uses atomic Redis limits separate from HTTP quotas. Define detailed request fields from the verified provider API before implementation, not legacy draft documentation.

- [ ] Verify current Exotel official API/auth/callback requirements and account-specific Indian routes; record dated references and confirm mock-only development. Write `TestExotelDelivery` for valid/invalid DLT binding, approved-template variable mismatch, auth/429/network/unknown response, provider acceptance and signed callback correlation.
- [ ] Run `go test ./internal/infrastructure/providers -run '^TestExotel' -count=1`; verify RED. Add a mock two-connection budget test showing a busy sender cannot exhaust another sender's allowance.
- [ ] Implement Exotel, safe UI fields/readiness and atomic sender/unit/spend budgets. Additive provider choice preserves Twilio defaults; domestic route starts disabled. Validate rendered text against approved DLT template; do not route internationally as an implicit workaround.
- [ ] Run Exotel/unit/integration/browser tests with mocks/disposable Redis. Simulate a 6,000-recipient queue at allowed throughput, measure preparation/processing and report provider-imposed limits; no zero-time delivery promise. Require a verified commercial quote before live enablement.
- [ ] Commit `feat: add domestic SMS connections and delivery budgets`.

**Phase 4 checkpoint:** demonstrate domestic SMS configuration/estimates and queue behavior with fixtures; provider approval and actual prices remain deployment prerequisites.

## Phase 5 — Environment simplification, cleanup and rollout evidence

### Task 10: Runtime public configuration and compatible environment aliases

**Files:** Modify `internal/config/config.go`, `config/config.yaml`, `config/config.prod.yaml`, container/routes, `ui/src/components/whatsapp/WhatsAppConnect.tsx`, `ui/src/services/api.ts`, `ui/Dockerfile.prod`, `.github/workflows/ci.yml`; inspect `docker-compose.yml` without overwriting pre-existing edits. Create `internal/interfaces/http/handlers/channel_capabilities_handler.go`, config tests and `config/messaging.env.example`.

**Interfaces:** `GET /v1/channel-capabilities` returns only available modes, public Meta App/signup IDs, Graph version and safe feature/readiness information; per-app facts require authorization. `FREERANGE_MESSAGING_META_ENABLED` controls both delivery/onboarding when explicitly set; when absent, preserve independent legacy flags, including outbound-only configurations. Shared Twilio account defaults permit channel account/sender overrides.

- [ ] Write `TestChannelEnvironmentCompatibility`: old env names, new shared account, distinct channel overrides, conflicting Meta secrets, false flags with stale credentials, daily-cap toggle, runtime UI without build keys and capability response containing no secrets.
- [ ] Run `go test ./internal/config ./internal/interfaces/http/handlers -run 'TestChannel(EnvironmentCompatibility|Capabilities)' -count=1`; verify RED.
- [ ] Implement runtime public config, canonical version/secret/flag resolution and effective daily-cap configuration. Retain old public SDK app/config vars as temporary fallback; remove fixed customer business/WABA/phone defaults. Document actual provider retry/queue policy and remove ineffective options only with compatibility handling. Create placeholder-only env example; do not edit real `.env` files.
- [ ] Run config/browser tests and `npm --prefix ui run build`. Verify a production-image-style frontend gets onboarding config after build without injecting customer identities or secrets.
- [ ] Commit `refactor: simplify messaging configuration with legacy compatibility`.

### Task 11: Verified cleanup, whole-branch review and rollout runbook

**Files:** Remove only confirmed unwired `internal/infrastructure/providers/whatsapp_self_hosted_provider.go`, `internal/usecases/services/whatsapp_self_hosted_service.go`, `internal/interfaces/http/handlers/whatsapp_self_hosted_handler.go`; update `go.mod/go.sum` only if dependencies have no remaining callers. Mark `documents/hybrid-sms-implementation-plan.md` superseded by this plan. Extend `documents/production-delivery-recovery-runbook.md`; update this plan's progress.

**Interfaces:** No new public API. Recovery readers continue to support created connections/event receipts/reservations when new admissions/features are rolled back. Operator commands are dry-run by default with explicit apply scopes and no secret output.

- [ ] Confirm constructor/import/reference searches and build inventory before deletion. Run the accumulated regression suite, fresh whole-branch security/correctness review and staged secret scan; inspect every failure rather than declaring earlier evidence sufficient.
- [ ] Remove only verified unused files/dependencies; preserve remaining providers, docs, user edits and review evidence. Record unavoidable read-response security changes and all operational prerequisites.
- [ ] Run `go test -short ./...`, relevant disposable ES/Redis integration cases, `npm --prefix ui run build` and both channel/reliability Playwright suites. Run `go test -race` on services/providers/handlers/worker in supported Linux CI; record Windows toolchain limitations rather than treating them as coverage.
- [ ] Document reader-first deploy, explicit scoped migration, sender/signing validation, connection canaries, detailed tariff activation, rate/spend alerts, dead-letter recovery, key rotation and rollback. Live test sends require explicit recipient/account authorization; historical data/balance repair requires a separate audit.
- [ ] Commit `chore: remove unused WhatsApp code and document messaging rollout`; open a branch diff for local review. Do not push or deploy as an implicit final step.

## Completion criteria

- No cross-tenant inbox delivery or ordinary secret-bearing response.
- Existing Twilio customers retain working credentials, payloads and admitted prices; new customer accounts work without global provider credentials.
- Meta sender readiness is validated, environment switching is real and settings cannot restore disconnected credentials.
- Duplicate callbacks and crash recovery do not duplicate storage, workflows, provider sends or charges.
- Inbox replies, OTP and SMS use the intended shared dispatch/accounting/budget paths; status/read counts reflect actual state.
- Domestic SMS is tested but remains gated until vendor/commercial prerequisites are met.
- Environment examples are simplified with tested legacy aliases; actual secrets and production state are untouched.
- All five phase checkpoints, review findings and verification limitations are recorded before claiming code completion.

## Execution handoff

Recommended execution: native implementation in this chat, one local branch, phase updates and a fresh whole-branch reviewer. Tasks share connection/quote/event interfaces heavily, so sequential ownership reduces integration risk. The user reviews this proposed design and plan before implementation; commercial/provider enablement is a later operational decision.
