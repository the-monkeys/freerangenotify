# App Business Billing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Native execution is already selected by the user. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete app-scoped product/subscriber billing with each business's own Razorpay account, both collection modes, recoverable accounting, and responsive business/customer experiences without losing existing data or integrations.

**Architecture:** Retain `/v1/biz/*`, its Elasticsearch records, and the existing service facade. Add a small, ordered financial-operation journal with versioned projections, encrypted merchant connections, and durable gateway/notification processing. Activate the upgraded lifecycle per app; never dual-write through old and new financial paths.

**Tech Stack:** Go 1.26.1, Fiber 2, Elasticsearch 8.18.0 fixture (matching the committed Compose image), Redis 7, existing Razorpay Go SDK, React 19, TypeScript, Vite, and the locked Playwright version on Ubuntu 24.04 for browser CI. No new database, crypto implementation, or runtime browser/PDF service.

**Spec:** `docs/superpowers/specs/2026-11-11-business-billing-end-to-end-design.md` (user approved 2026-11-11).

**Execution:** All six phases, native implementation, one branch: `codex/business-billing-end-to-end`. Base `94acfe1`; design commit `dcff935`. This document is the authoritative local plan, decisions, feedback and handoff record. Read the current-state section first; dated execution entries below are historical evidence, not the current task status.

## Current state and handoff — updated 2026-11-12

- **Scope:** Business billing inside an application for the business's own customers. Gateway-free invoicing and recording received cash/direct-UPI/bank payments remain available. A business choosing online collection uses its own Razorpay account; connecting a gateway is optional for manual billing. Support invoice/payment-link and customer-authorized automatic collection, selected per subscription. Notification delivery credits and platform subscriptions remain separate financial systems.
- **Completed implementation:** Phases 1 and 2 (Tasks 1-11) are complete locally on `codex/business-billing-end-to-end`. Phase 2 adds optional encrypted per-business Razorpay connections, scoped checkout, durable signed callbacks and real refundable-money recovery. **Latest instruction: implement Phase 3, verify it, then implement Phase 4; stop after Phase 4 for the user’s joint smoke test.** No push, PR or production deployment was performed. The broader billing UI changes remain Phase 4.
- **Phase 3 progress:** Tasks 12–14 are complete locally. Task 15 durable receipt/reminder delivery is next. Phase 3 is not complete and Phase 4 has not started.
- **Phase 1 verification evidence:** 115 top-level billing tests across seven packages; broader short suite across 63 packages; nine legacy HTTP end-to-end scenarios; six live lifecycle HTTP boundary checks; four fixture-safety tests; generated API check and UI production build all passed. This is local evidence, not live-provider or production verification.
- **Phase 2 verification evidence:** 149 billing regressions across nine packages passed with zero skipped cases; the final terminal-refund correction was followed by 17 refund/provider/HTTP regressions passing with zero skips. The broader short suite passed (36 tested packages plus 32 packages compiled without tests), as did generated-reference consistency, documentation contracts, TypeScript examples, webhook examples, seven Node safety checks, UI type checking and the Vite production build. Native main-Compose backend/UI builds and additive setup passed. One cluster/five services are running with zero restart counts and telemetry disabled. Read-only API checks verified merchant readiness/status routes and rejection of an unsigned callback with 401. The smoke subscription/invoice/payment documents match their pre-rebuild snapshot exactly; the UI serves the updated guide and API reference. Payment/refund execution used local fixtures only; a real Razorpay test-mode canary remains unperformed.
- **Guided setup/lifecycle smoke:** Created Smoke Test Product, Smoke Test Monthly (USD 10/month), and Smoke Test Customer in the local test application LocalOnly-Billing. Created a quantity-one subscription with no trial, generated its first invoice, and verified pause/resume restored the subscription to active without creating an extra invoice or payment.
- **Saved financial result:** At the end of the guided test there was exactly one active subscription and one open invoice; subtotal USD 10.00, test tax USD 1.80, total/due USD 11.80, actual paid USD 0.00, and zero payment records. The current UI incorrectly renders USD 11.80 under Amount Paid; this is a confirmed display mapping bug, not a collected payment. Actual payment collection/manual payment entry has not been exercised through this UI. The legacy HTTP payment flow was covered by automated checks. Before the Phase 2 rebuild, the existing scheduler had marked the immediately-due unpaid invoice overdue and its subscription past due (2026-11-11 22:30 UTC). The one invoice remains USD 11.80 due, USD 0.00 paid, with no payment records. Preserve these current records and statuses; do not reset them to make a demo appear paid or active.
- **Deferred user findings:** (1) INR-first selectable default currency, preserving alternate currencies and existing historical amounts; (2) app-scoped searchable customer picker for large customer lists; (3) correct Amount Paid mapping. These belong to Phase 4 Tasks 16/17. Currency is also tracked in [GitHub issue 130](https://github.com/the-monkeys/freerangenotify/issues/130). Real-time FX conversion and conversion charges remain separate future design. The user explicitly requested recording these findings and continuing the plan rather than implementing them during the smoke test. A started two-line paid-column edit was reverted; no running UI update or money change was made for it.
- **Additional observation for later triage:** Leaving the customer phone number empty resulted in the list displaying only the selected country code. Preserve this observation alongside the three user-prioritized findings; do not silently treat it as fixed.
- **Compatibility and rollout:** Existing applications remain on their legacy lifecycle. New collection activation defaults off. Migration requires explicit review, currency decisions and drained-writer readiness. No historical money, invoice identifiers, balances, SSE contracts or existing integrations may be silently rewritten.
- **Local resources:** At most one FreeRangeNotify Compose cluster at a time, including test fixtures. Use the main `docker-compose.yml` with the existing private local override; never use the telemetry Compose file locally. Telemetry is disabled. Preserve local volumes and all production data; leave other projects' clusters alone. The current local UI is `http://127.0.0.1:18762` and API is `http://127.0.0.1:18761`; runtime configuration and synthetic credentials stay in private scratch, never in this document or commits.
- **Permissions:** Native execution and local commits on the same branch are approved. The user permits optional pushing of this branch, but explicitly forbids opening a pull request until instructed. A proposed Saturday merge is a planning target, not authorization to merge, deploy, make live charges, send external notifications or migrate production. Report publication status accurately.
- **Milestones:** Proposed PR review Saturday, 2026-11-14; business presentation Sunday, 2026-11-15. Readiness depends on the implemented and verified demonstration path, not the calendar alone. Phase 4 owns responsive billing UI; Phase 6 owns broader regression and release evidence. The wider nontechnical-user application restructuring is deferred until the six phases finish.
- **Continuation:** Use this document as the single source of truth for task order, approvals and deferred scope. Private `.superpowers/sdd/2026-11-11-business-billing-end-to-end/` files contain supporting logs, ownership manifests and transient runtime state; they are evidence, not an alternate plan. Do not load production `.env`, start another cluster, create a PR or expand scope merely to recover context.

## Global Constraints

- Preserve existing routes, authentication, RBAC, identifiers, response envelopes, amount units, and status meanings. Add optional fields and endpoints; do not rename legacy fields.
- Missing subscription collection mode means `invoice`; existing records never acquire a payment mandate or automatic charges through migration.
- Existing `frn_biz_*` records and indices are retained. Do not reindex into empty replacements, delete history, regenerate invoice numbers, or recalculate issued historical totals.
- Preserve stored currencies and minor units. Never sum currencies or silently convert them.
- Each business choosing online collection connects its own Razorpay account; platform notification-credit purchases and workspace licensing are outside this project. Do not require merchant onboarding for issuing an invoice, recording authorized manual cash/direct-UPI/bank receipts, or sending the corresponding receipt.
- Upgrade the billing lifecycle per app through an explicit, recorded readiness/cutover step. Apps not yet upgraded retain their legacy API behavior.
- Initial automatic collection supports fixed-amount INR plans with cycles and terms supported by the connected merchant's Razorpay Subscriptions capability. Unsupported combinations are rejected before authorization.
- Automatic collection freezes its invoice total and excludes manual collection/adjustment until the debit is captured or definitively stopped. Verified unexpected money is recorded and reconciled, not discarded.
- No negative customer balance or implicit reopening of another invoice to fund a refund. Spent overpayment credit restricts refundability.
- At 360, 390, 768, and 1280 CSS pixels, primary actions and forms must work without page-wide horizontal scrolling. Test light/dark modes, keyboard focus, and permissions.
- Reuse existing infrastructure; no new database. Production remains untouched unless a later rollout is authorized.
- Keep at most one owned FreeRangeNotify Compose project running, reuse it for local tests and UI, and never start local telemetry services. Preserve volumes and leave other projects' resources untouched.
- No pull request may be opened until the user explicitly instructs it. Optional branch pushing is permitted, subject to secret checks and deployment-trigger inspection; it does not authorize merging or production deployment.
- Do not touch the user's existing changes in `cmd/frn/admin.go`, `cmd/frn/config.go`, or `docker-compose.yml`, or unrelated untracked files. Do not stage whole directories or run destructive cleanup.
- No live payment, production backup/migration, deployment, GitHub issue comment/closure, or external notification is authorized by this plan. Provider test-mode canaries require supplied test configuration; local fixtures are not real-provider evidence.
- Each task follows a failing-test → minimal implementation → passing-test cycle, except baseline characterization which must pass on the old behavior. Commit only that task's explicit files. Stop and investigate failures rather than weakening assertions.
- Every task adding/changing a public route or documented DTO includes its annotations and generated reference updates in the same commit, not only in Phase 5. Its allowed companion files are `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`, `ui/public/swagger.json`, `docs/openapi/swagger.yaml`, `docs/openapi/otp.yaml`, `docs/openapi/whatsapp-rich.yaml`, and affected `tests/contracts/routes_test.go`/router fixtures. Run `go run ./cmd/docsgen` followed by `go run ./cmd/docsgen -check`; never hand-edit generated payloads. Task 22 adds full recipes and consolidates coverage.

## Review Focus

1. **RF1 — spoofed replay identity:** changing the event-ID header on the same signed body cannot settle twice; reusing an event ID with different bytes cannot suppress a legitimate payment. Task 10 tests both.
2. **RF2 — uncertain external effects:** a gateway succeeds, then the response/persistence is lost during key rotation. Recovery must find the original result, not repeat the order/refund/mandate. Tasks 9 and 11 test this.
3. **RF3 — live cutover races:** a legacy request starts before app activation or a merchant-slot switch; it cannot commit through a stale writer or settle against the new merchant. Tasks 6 and 8 test fencing and immutable bindings.
4. **RF4 — credit provenance under concurrency:** refund and credit spend race against the same overpayment. The accepted total never exceeds available source funds and creates no hidden debt. Tasks 5 and 11 test this.
5. **RF5 — portal expiry mid-payment:** a customer link expires after checkout begins. A verified gateway capture still settles once; the expired link grants no fresh access. Tasks 18 and 19 test both sides.

## Files and shared decisions

Keep `BizLifecycleService` and its constructor/callers compatible; optional dependencies can be installed by explicit setters. Move only cohesive responsibilities into `bizbilling_*` collaborators. Domain types remain in `internal/domain/bizbilling`; persistence remains in `internal/infrastructure/bizbillingrepo`. Do not create a second billing engine/package tree.

The following named contracts are defined by the task that owns their file. Later tasks consume them unchanged:

- `Scope{AppID, UserID, Currency string}` identifies a customer/currency financial stream. `GatewayMode` is `test|live`. `GatewayRef{ConnectionID, AccountID string; Mode GatewayMode; CredentialRevision int64}` is immutable on a transaction.
- `LifecycleState{AppID, PrimaryCurrency string; LifecycleVersion int; Status string; Epoch int64; MutationPermits map[string]MutationPermit; MerchantSlots map[GatewayMode]string}`. Versions: 1 legacy, 2 journal. States: `legacy|preparing|ready|activating|active`. Missing state reads as legacy. `MutationPermit{AppID,WriterID,OperationID string; Epoch int64; StartedAt,ExpiresAt time.Time}` fences every financial writer. Derive active-writer counts from registered permits; do not maintain an unrelated counter. Expired legacy permits require source/operation recovery review before clearing, not automatic permission to activate.
- `Command{Scope Scope; Kind CommandKind; IdempotencyKey string; Payload json.RawMessage}` uses a closed set of explicitly decoded financial command kinds. No arbitrary JSON patch/expression execution. `OperationResult{OperationID string; Status string; ResourceID string; Sequence int64}`; statuses `pending|applying|completed|rejected`. An accepted journal fact is immutable; rejection is possible only before that fact exists.
- `JournalEvent{ID, OperationID, IntentHash string; Scope Scope; Sequence int64; Kind CommandKind; Payload json.RawMessage; Notifications []NotificationIntent; OccurredAt time.Time}`. Its typed payload carries exact money effects and resource IDs; it is not a generic workflow DSL.
- New indices only: `frn_biz_lifecycle_states`, `frn_biz_operations`, `frn_biz_streams`, `frn_biz_journal`, `frn_biz_balances`, `frn_biz_gateway_connections`, `frn_biz_gateway_events`, `frn_biz_notification_outbox`, and `frn_biz_migration_reviews`. Add mappings; preserve existing index contents/mappings.
- Stream heads hold committed sequence, active operation, lease generation/owner, and migration fences. No unbounded list of past operation IDs in a customer document. Per-resource checkpoints and immutable journal entries provide replay evidence.
- Safe projection reads must honor the active stream operation. Before final commit, its tentative projections are used only by the owning operation/reconciler; ordinary invoice/balance/statement APIs serve the last completed projection revision or a specific reconciliation-pending response. A competing request must not spend a partially projected balance or display a refund as completed. Historical/imported v1 records retain their normal reads until explicit cutover.
- New requests may send `Idempotency-Key`. Gateway effects and scheduled operations always derive deterministic keys. For legacy manual APIs without a key, separate submissions remain distinct intentional actions; give each accepted request an operation ID and make interrupted/uncertain requests resumable through that ID. The new UI retains the same key across retries. Do not pretend identical requests prove the user meant only one payment.
- `internal/domain/bizbilling/errors.go` owns typed/sentinel errors referenced below (`ErrIdempotencyConflict`, `ErrCurrencyMismatch`, `ErrAutomaticCollectionInProgress`, `ErrActiveLegacyWriter`, reconciliation-required, stale-permit, and busy/conflict errors). HTTP mapping preserves existing envelopes. `testutil.ErrUnownedFixture` is test-only and belongs to the fixture guard, not the business domain.
- `GET /v1/biz/operations/:id` exposes only the authenticated app's sanitized progress. Incomplete synchronous operations retain the existing error envelope plus additive `operation_id`, `operation_status`, and `retryable`; use 409 for competing/changed intents and 503 for unavailable or uncertain completion. Success retains existing envelopes/status codes and only follows required projections.
- Merchant management: `GET|PUT|DELETE /v1/biz/gateway-connections/:mode`; saving authenticates through a read-only API and deleting disconnects the active slot while retaining historical credentials; callback `POST /v1/webhooks/biz/razorpay/:connection_id`. These are separate from old connector callbacks and platform `/v1/billing/*` routes.
- Upgrade management: `GET /v1/biz/lifecycle`, `POST /v1/biz/lifecycle/prepare`, `POST /v1/biz/lifecycle/activate`; review `GET /v1/biz/lifecycle/reviews`, `POST /v1/biz/lifecycle/reviews/:id/resolve`. Existing `manage_billing` write authorization applies; app scope always comes from auth, never a body override.
- Add `collection_mode?: invoice|automatic`, `gateway_mode?: test|live`, and `change_timing?: immediate|next_period`. Missing mode stays invoice. Legacy manual change-plan without timing keeps immediate proration; new UI explicitly selects next period. Automatic currency/pricing/cycle incompatibility is a validation error, never fallback.
- Customer links stay under the existing `/v1/portal/:token` API and `/portal/:token` React paths. New v2 tokens are stored by digest, last 30 days by default (matching legacy TTL), and support revocation. Keep read-only legacy token lookup until its original expiry.
- Print-ready React invoice/receipt views supply an explicitly named **Print / Save as PDF** action using the browser print facility. Headless Playwright verifies actual PDF output in tests; do not deploy Chromium or add a server PDF library. Do not promise a server-generated PDF endpoint.
- `BizBillingConfig.NewCollectionEnabled` binds `FREERANGE_BIZ_BILLING_NEW_COLLECTION_ENABLED` and defaults false for v2 initiation/activation. It does not change unupgraded legacy API behavior. Disabling it blocks new checkout/mandate initiation and new FRN-originated automatic collection; callbacks, financial recording, and recovery remain enabled. **It does not stop Razorpay from collecting an existing authorized subscription**: stopping that requires a provider-confirmed pause/cancel. Runbooks and UI must state this explicitly.

## Fixture and command conventions

Task 1 creates `scripts/biz-billing-fixtures.mjs`. Commands below use:

`node scripts/biz-billing-fixtures.mjs run [--stack] -- <command> <args>`

The runner creates its own uniquely named/labelled Compose project using `tests/fixtures/biz-billing.compose.yml`; ES/Redis host ports are random and loopback-only. `--stack` builds and launches server/worker/migrate binaries from a temporary working directory containing an explicit fixture config. It never changes the user's Compose file or reads the workspace `.env`. Child environment is an allowlist; provider credentials, `NODE_OPTIONS`, arbitrary proxy settings, and user `VITE_*` values are excluded. Gateway calls use injected local HTTP fixtures, never real Razorpay keys.

The manifest `FRN_BIZ_FIXTURE_MANIFEST` records container IDs, owner labels, project name, addresses, and run ID. Tests verify that identity before any cleanup. `FRN_BIZ_REQUIRED=1` means missing billing flag/auth/fixtures fails, rather than skips. Existing optional integration entrypoints remain optional outside that mode. Remove only the runner's own verified containers/volumes/processes and temporary files. No production-like default URLs are accepted for required tests.

Go unit commands below use explicit package paths to avoid traversing `node_modules`. Run broader suites once per completed phase, and again only after a relevant change/failure. Assertion snippets specify acceptance checks after arranging the named scenario; task-owned fixtures must exercise real service/repository/handler paths, not duplicate implementation logic.

## Phase 1 — Compatibility and financial foundation

### Task 1: Characterize legacy contracts and introduce a disabled lifecycle gate

**Files:** Create `tests/fixtures/biz-billing/legacy-contracts.json`, `tests/fixtures/biz-billing.compose.yml`, `scripts/biz-billing-fixtures.mjs`, `scripts/biz-billing-fixtures.test.mjs`, `internal/testutil/bizbilling_fixture.go`, `internal/domain/bizbilling/scope.go`, `internal/domain/bizbilling/lifecycle_state.go`, `internal/domain/bizbilling/errors.go`, `internal/infrastructure/bizbillingrepo/lifecycle_state_repo.go`, `internal/usecases/services/bizbilling_lifecycle_gate.go`, `internal/usecases/services/bizbilling_lifecycle_gate_test.go`, `internal/interfaces/http/handlers/biz_billing_compatibility_test.go`, `internal/infrastructure/database/biz_billing_index_templates_v2.go`, `internal/config/biz_billing_test.go`; modify `tests/integration/biz_billing_e2e_test.go`, `internal/container/container.go`, `internal/config/config.go`, `config/config.yaml`, `internal/infrastructure/database/index_manager.go`, `cmd/migrate/main.go`.

**Interfaces:** Produce shared `Scope`, `GatewayMode`, and `GatewayRef` in `scope.go`, then `LifecycleGate.State(ctx, appID) (LifecycleState,error)`, `AcquireMutation(ctx, appID) (MutationPermit,error)`, `CheckMutation(ctx, MutationPermit) error`, and `ReleaseMutation(ctx, MutationPermit) error`. Permits register in durable app state and expire only into a recovery/drain check, not permission to overwrite an unknown operation. Initial wiring selects legacy behavior; activation is implemented in Task 6.

- [x] **1. Tests:** Capture legacy create/list/get, manual payment, immediate change-plan, error envelopes, invoice units/statuses, portal links, API-key/JWT/RBAC, feature-off routes, and platform-billing/SSE noninterference. Characterization must pass before modifications. Then add failing `TestBizLifecycleMissingStateUsesLegacy` and `TestBizRequiredFixtureRejectsUnownedEndpoint`:
  ```go
  require.Equal(t, 1, state.LifecycleVersion)
  require.Equal(t, "legacy", state.Status)
  require.ErrorIs(t, unownedFixtureError, testutil.ErrUnownedFixture)
  ```
- [x] **2. Red:** `node --test scripts/biz-billing-fixtures.test.mjs`; `go test -short ./internal/interfaces/http/handlers ./internal/usecases/services -run 'TestBiz(Legacy|Lifecycle)' -count=1`. New gate/safety cases must fail; legacy fixture differences must be understood before proceeding.
- [x] **3. Implement:** Add the runner/Go fixture guard and disabled gate with the signatures above; generate fixture JWT/encryption secrets at runtime, redact output, and preserve all existing constructor behavior. Add the default-false v2 collection flag and only the lifecycle-state index initially; Task 4 extends its mapping file. Wrap legacy as well as v2 financial facade entrypoints in durable mutation permits, carried in context, so preparation can drain all writers; check the permit before persistence and release only after writes finish. Required-fixture guards must run before suite setup/cleanup. Do not activate v2 apps or change legacy API defaults.
- [x] **4. Green:** Repeat focused tests, then run the legacy `TestBizBillingE2ESuite` through the owned `--stack` runner. Verify expected cases execute and feature/RBAC setup does not skip.
- [x] **5. Commit:** Stage only listed files; `git commit -m "test(biz-billing): pin legacy contracts and isolated fixture safety"`.

### Task 2: Checked money, explicit zero tax, and stable calendar anchors

**Files:** Create `internal/domain/bizbilling/money.go`, `money_test.go`, `calendar.go`, `calendar_test.go`, `tax_test.go`, `proration_test.go`; modify `tax.go`, `proration.go`, `invoice.go`, `credit_note.go`, `config.go` in that same domain directory.

**Interfaces:** Produce `CheckedAdd(a,b int64) (int64,error)`, `CheckedMul(a,b int64) (int64,error)`, `ProrateMinor(amount,numerator,denominator int64) (int64,error)`, `NextAnchoredPeriod(start time.Time, cycle BillingCycle, cycleDays, anchorDay int) (time.Time,error)`, and `ComputeInvoiceTotalsChecked(items []InvoiceLineItem,cfg TaxConfig,state string,overrides []LineTaxOverride) (InvoiceTotals,error)` with `LineTaxOverride{Rate *int; Exempt bool}`. Keep current public helpers as legacy wrappers; new journal writers use checked variants. Optional override fields distinguish explicit zero from omitted/legacy zero.

- [x] **1. Tests:** `TestBizMoneyOverflow`, `TestBizExplicitZeroTaxPreservesLegacyDefault`, `TestBizAnchoredCalendar`, `TestBizProrationTruncatesMinorUnits`; cover maximum integers, negatives, January 31 → February 28 → March 31, leap-day years, and partial periods.
  ```go
  require.Error(t, overflowErr)
  require.Zero(t, explicitZero.TaxPaisa)
  require.Equal(t, int64(1800), legacyDefault.TaxPaisa) // 10,000 taxable paisa
  require.Equal(t, "2027-03-31", march.Format("2006-01-02"))
  ```
- [x] **2. Red:** `go test ./internal/domain/bizbilling -run 'TestBiz(Money|Explicit|Anchored|Proration)' -count=1`.
- [x] **3. Implement:** Preserve truncation toward minor units using checked integer/rational arithmetic; validate rather than clamp invalid new inputs. Preserve issued snapshots and legacy defaults. Retain the original anniversary across short months; provider-confirmed periods take precedence for automatic collection.
- [x] **4. Green:** Repeat command; include table/fuzz seeds for boundary amounts and date arithmetic. No historical fixture changes.
- [x] **5. Commit:** Stage listed domain files; `git commit -m "fix(biz-billing): validate money and preserve billing anchors"`.

### Task 3: Conditional creation and pure optimistic-concurrency mutations

**Files:** Modify `internal/infrastructure/bizbillingrepo/store.go`, `lifecycle_repo.go`; create `versioned_store.go`, `versioned_store_test.go` in that directory; modify `internal/usecases/services/bizbilling_lifecycle_service.go` only to remove external effects from retryable mutation closures.

**Interfaces:** Produce `Version{SeqNo,PrimaryTerm int64}`, `Store[T].GetVersioned(ctx,id) (*T,Version,error)`, `CreateIfAbsent(ctx,id,doc *T) (bool,error)`, and `ReplaceVersioned(ctx,id,doc *T,expected Version) error`. Existing `Create/Update` signatures remain; immutable IDs use `op_type=create`. CAS retries are capped at 8, return conflict on exhaustion, and do no external work inside mutation callbacks.

- [x] **1. Tests:** `TestBizCreateDoesNotOverwrite`, `TestBizCASNoLostUpdate`, `TestBizCASPreservesUnknownFields`, `TestBizRetryDoesNotRepeatGatewayEffect`. Assert two concurrent increments survive, mismatched term/sequence produces 409, unknown optional source fields survive known-field edits, and gateway calls remain one outside CAS retries.
  ```go
  require.Equal(t, int64(2), storedCounter)
  require.Equal(t, "retained", raw["future_field"])
  require.Equal(t, 1, gatewayCalls)
  ```
- [x] **2. Red:** `node scripts/biz-billing-fixtures.mjs run -- go test ./internal/infrastructure/bizbillingrepo -run 'TestBiz(Create|CAS|Retry)' -count=1`.
- [x] **3. Implement:** Use actual ES sequence/term guards; retain unknown source keys while applying owned-field changes, including intentional removals. A blind full-document replace is not an OCC fix. Keep generic `BaseRepository` behavior outside this module unchanged.
- [x] **4. Green:** Repeat against real ES; kill one response/force contention in fixtures and verify bounded retry behavior.
- [x] **5. Commit:** Stage only listed files; `git commit -m "fix(biz-billing): fence concurrent document mutations"`.

Evidence: owned ES regression suite 9/9 PASS (13.554s), gateway retry regression PASS; full short domain/repository/service/handler suites PASS. Unknown write response tested through a disconnected HTTP transport; only confirmed version conflicts are retried.

### Task 4: Ordered journal and resumable projections

**Files:** Create `internal/domain/bizbilling/operation.go`, `internal/domain/bizbilling/balance.go`, `internal/domain/bizbilling/notification_intent.go`, `internal/infrastructure/bizbillingrepo/operation_repo.go`, `journal_repo.go`, `stream_repo.go`, `internal/usecases/services/bizbilling_operation_service.go`, `bizbilling_projection_service.go`, `bizbilling_operation_test.go`; modify `internal/infrastructure/database/biz_billing_index_templates_v2.go`, `internal/infrastructure/database/index_manager.go`, `cmd/migrate/main.go`, `internal/container/container.go`.

**Interfaces:** Produce `BizOperationService.Apply(ctx, Command) (OperationResult,error)`, `Get(ctx,appID,id string) (OperationResult,error)`, `Resume(ctx,appID,id string) (OperationResult,error)`, `Reconcile(ctx context.Context, limit int) (int,error)`; repository `Begin(ctx,Command) (*Operation,bool,error)`, `Claim(ctx,Scope,operationID string) (*StreamClaim,error)`, `Append(ctx,StreamClaim,JournalEvent) error`, and `Complete(ctx,StreamClaim) error`. `Operation` stores intent hash, immutable command, state, resource/effect references, and sanitized progress; `StreamClaim` carries scope, sequence, operation ID, owner, and fencing generation. Define `CustomerBalance{Scope Scope; AvailablePaisa,ReservedPaisa,LastSequence int64; UnresolvedLegacy bool}`, `BalanceAdjustment{DeltaPaisa int64; Reason string}`, and `NotificationIntent` from Task 15's contract now, to avoid forward type dependencies. Implement opening-balance/explicit balance-adjustment cases using private `buildFinancialEvent(ctx,Command,StreamClaim) (JournalEvent,error)` and `applyFinancialProjection(ctx,StreamClaim,JournalEvent) error`; Task 5 extends this closed dispatcher with remaining real commands, not a new financial-service hierarchy or test-only kinds.

- [x] **1. Tests:** `TestBizOperationReplay`, `TestBizProjectionCrashRecovery`, `TestBizProjectionReadDoesNotExposePartialCommit`, `TestBizLeaseCannotSkipAcceptedOperation`, `TestBizIdempotencyIntentConflict`. Inject failure after journal creation and after each resource projection; restart using persisted ES state. While interrupted, ordinary reads must return their last completed revision or reconciliation-pending error, never a newly spendable partial balance.
  ```go
  require.Equal(t, first.OperationID, replay.OperationID)
  require.Equal(t, int64(1), journalFacts)
  require.Equal(t, int64(400), balance.AvailablePaisa)
  require.ErrorIs(t, changedIntentErr, ErrIdempotencyConflict)
  ```
- [x] **2. Red:** Run the owned runner with `go test ./internal/usecases/services -run 'TestBiz(Operation|Projection|Lease|Idempotency)' -count=1`.
- [x] **3. Implement the concrete journal protocol:** conditionally persist immutable intent; claim the customer/currency stream by CAS; reserve sequence `committed+1`; validate against completed prior projections; CAS-freeze the computed event payload in the operation; conditionally append that exact immutable fact; apply each typed projection with its checkpoint in the same CAS as its money update; then complete and release the head. A second worker reads the already-frozen event, never recomputes prices/tax after lease transfer. Frozen/prepared facts cannot be discarded to let a different operation overtake them. The journal fact itself durably carries notification intents; Task 15 materializes outbox rows idempotently. Gate normal reads on completed projection revisions as specified above, including during a crash. Lease expiry transfers recovery of the **same** active operation, never permits the next operation to skip it. A validation failure before freezing can release/reject; once frozen, recovery must append/project or record an explicit compensation. External gateway waits occur outside this stream lock; money reservations are short, journaled commands. Reconciliation scans stable pages of 100, max 8 CAS attempts per update, with 30-second renewable worker leases. No generic saga/patch interpreter.
- [x] **4. Green:** Repeat, including parallel different invoices on one customer/currency, independent customers, and a lease transfer immediately before/after event freezing. A competing worker must reuse identical frozen bytes despite changed mutable configuration. Assert no duplicated financial effects and no unbounded per-customer dedupe arrays. Required projections must complete before synchronous success.
- [x] **5. Commit:** Stage listed files; `git commit -m "feat(biz-billing): journal financial operations with crash recovery"`.

Evidence: persisted ES replay, four response-loss boundaries, partial-read gate, lease transfer/frozen-byte reuse, rejection, first credit, schema fencing, bounded recovery/fairness PASS; repository refuses aborting frozen events and committing missing journal/projection. Final repeatable service suite55.367s; repository invariant suite0.634s.

### Task 5: Route every money writer through the protected path

**Files:** Create `internal/usecases/services/bizbilling_financial_commands_test.go`; move the private financial dispatcher/projection functions from Task 4 into cohesive `bizbilling_financial_commands.go` as they grow, preserving their signatures; modify `bizbilling_lifecycle_service.go`, `bizbilling_credit_service.go`, `bizbilling_coupon_service.go`, `bizbilling_estimate_service.go`, `bizbilling_usage_service.go`, `bizbilling_analytics_service.go` in services; modify domain `balance.go`, `invoice.go`, `credit_note.go`, `payment.go`; add handler tests in `internal/interfaces/http/handlers/biz_billing_operations_test.go` and handler file `biz_billing_handler_operations.go`; modify existing lifecycle/sales handlers and routes.

**Interfaces:** Consume Tasks 1–4 and extend their `buildFinancialEvent`/`applyFinancialProjection` functions. Produce `BizOperationService.Balance(ctx,Scope) (CustomerBalance,error)` using Task 4's model. Existing facade methods call `BizOperationService.Apply`; there is no extra Execute→Apply→Execute cycle. Typed command payloads cover invoice issue/amendment, manual payment/TDS, coupon reservation/release, late fee, write-off, void, credit note, retainer, balance adjustment/application/reversal, and refund reservation/settlement. `CreditLot{ID,SourcePaymentID,Currency string; OriginalPaisa,AvailablePaisa,ReservedPaisa int64}` records provenance. Keep facade methods and legacy DTOs; optional currency/idempotency flow is additive.

- [x] **1. Tests:** `TestBizAllMoneyWritersUseJournal`, `TestBizCurrencylessCreditRequest`, `TestBizCreditSpendRefundRace` (RF4), `TestBizCrossCurrencyCreditRejected`, `TestBizInvoiceAccountingInvariant`. Include indirect estimate conversion, usage billing, and coupon limits shared across customers; a global coupon redemption cap needs its own CAS reservation and recovery, not a customer lock alone.
  ```go
  require.Equal(t, "INR", defaulted.Scope.Currency)
  require.Equal(t, int64(0), doubleSpentPaisa)
  require.Equal(t, int64(1000), refundableAfterSpentExcess)
  require.ErrorIs(t, mismatchedCurrencyErr, ErrCurrencyMismatch)
  ```
- [x] **2. Red:** Owned runner: `go test ./internal/usecases/services ./internal/interfaces/http/handlers -run 'TestBiz(AllMoney|Currencyless|CreditSpend|CrossCurrency|InvoiceAccounting)' -count=1`.
- [x] **3. Implement:** Protect all active-v2 entrypoints, including scalar user-balance mirrors and generic-store writes. Retain legacy behavior for non-upgraded apps. Invoice paid fact is not reversed by a cash refund; expose refunds separately. Frozen/automatic-owned invoices reject competing initiated payments/adjustments, while verified unexpected captures remain recordable. Preserve existing manual APIs without pretending no-key requests are deduplicable.
- [x] **4. Green:** Repeat; verify balances/statement/payment projections match journal facts after restart, concurrency, and a failure between projections. Existing financial response snapshots still match.
- [x] **5. Commit:** Stage listed files; `git commit -m "fix(biz-billing): protect balances and all monetary mutations"`.

### Task 6: Non-destructive inventory, review, and per-app cutover

**Files:** Create `cmd/migrate/biz_billing_v2.go`, `internal/usecases/services/bizbilling_migration_service.go`, `bizbilling_migration_test.go`, `internal/interfaces/http/handlers/biz_billing_handler_upgrade.go`, `internal/infrastructure/bizbillingrepo/migration_review_repo.go`; modify the lifecycle gate, `cmd/migrate/main.go`, container wiring, and routes.

**Interfaces:** Produce `Prepare(ctx,appID string,options MigrationOptions) (*MigrationReport,error)`, `Resolve(ctx,appID,reviewID string,decision LegacyResolution) error`, `Activate(ctx,appID,reportID string) error`. `MigrationOptions{DryRun bool; ExpectedReportID string}` defaults to dry run at both CLI/HTTP boundaries. `MigrationReport` includes source revisions/hash, resource counts, unresolved currency/ownership, missing references, legacy gateway bindings, checkpoint, and readiness. `LegacyResolution` carries actor, reason, assigned currency/reference, and expected source revision. CLI `migrate biz-billing-v2 --app-id ID` defaults to dry run; `--apply --report-id ID` is explicit. Dry-run reports use a deterministic content digest and are returned without persistence; explicit preparation persists a reviewed report before resolutions/activation.

- [x] **1. Tests:** `TestBizMigrationDryRunNoWrites`, `TestBizMigrationResumePreservesSources`, `TestBizAmbiguousCurrencyNeedsReview`, `TestBizCutoverFencesOldWriter` (RF3). Pause a real legacy mutation holding its permit while activation begins; neither a timed-out lease nor stale report permits activation over that unresolved writer.
  ```go
  require.Equal(t, before.SourceHashes, after.SourceHashes)
  require.False(t, report.Ready) // mixed/untraceable credit currency
  require.ErrorIs(t, activationErr, ErrActiveLegacyWriter)
  require.Equal(t, 2, activated.LifecycleVersion)
  ```
- [x] **2. Red:** Owned runner: `go test ./internal/usecases/services -run 'TestBiz(Migration|Ambiguous|Cutover)' -count=1`.
- [x] **3. Implement:** Before opening-balance capture, acquire the app migration fence, stop admission of new app money writes, drain registered writers and accepted operations, and snapshot/check source revisions. All new instances must register permits for legacy as well as v2 writes; deployment must first drain binaries unaware of this protocol. Dry run only reads; reviewed apply writes additive opening facts/checkpoints, preserving original documents. Stale source/review hashes invalidate activation. Release back to legacy on aborted preparation, or switch atomically to v2 after complete projection/readiness checks. Primary-currency selection never relabels old credit; ambiguous money remains unavailable pending owner review. Re-running uses the same opening-event identities.
- [x] **4. Green:** Repeat with crashes at every migration stage, post-report source changes, old server-wide orders, and duplicate apply. Verify original totals/numbers/history unchanged and one active writer version per app. Do not run against production.
- [x] **5. Commit:** Stage listed files; `git commit -m "feat(biz-billing): add reviewed resumable app upgrades"`.

### Task 7: Fix audit date filtering (#67)

**Files:** Modify `internal/domain/audit/models.go`, `internal/interfaces/http/handlers/audit_handler.go`, `internal/infrastructure/repository/audit_repository.go`; create `internal/interfaces/http/handlers/audit_date_filter_test.go`, `internal/infrastructure/repository/audit_date_filter_test.go`.

**Interfaces:** Add `FromDate,ToDate *time.Time` and `ToDateExclusive bool` to `audit.Filter`; add local `parseAuditDateRange(from,to string) (*time.Time,*time.Time,bool,error)`. Date-only bounds use UTC; date-only end is next-day exclusive; RFC3339 timestamps retain exact instants. Both count/list use the identical range.

- [x] **1. Tests:** `TestAuditDateBounds`, `TestAuditDateFilteringKeepsTenantScope`: same-day date range includes 23:59:59.999, excludes next midnight, handles offsets, rejects malformed/reversed bounds with 400, preserves count/pagination and app ownership.
  ```go
  require.Equal(t, "2026-11-12T00:00:00Z", end.Format(time.RFC3339))
  require.True(t, exclusive)
  require.Equal(t, int64(2), filteredCount)
  ```
- [x] **2. Red:** `go test -short ./internal/interfaces/http/handlers ./internal/infrastructure/repository -run 'TestAuditDate' -count=1`.
- [x] **3. Implement:** Parse explicitly and add ES `created_at` range alongside existing app filters. Do not silently ignore invalid input or change other filters.
- [x] **4. Green:** Repeat; verify actual ES range/count once through the owned runner.
- [x] **5. Commit:** Stage listed files; `git commit -m "fix(audit): apply date range filters consistently"`.

Evidence: parser/HTTP validation and tenant-scoped List/Count tests PASS; owned ES same-day boundary/count/pagination PASS (6.67s); generated API references updated and docsgen -check PASS. Legacy page count envelope is retained.

**Phase 1 checkpoint:** Run focused domain/repository/service/handler suites and legacy compatibility fixtures. Report commits, executed test counts, unresolved migration cases, and the fact v2 activation remains off. No production writes or implicit rollout.

Phase 1 evidence (2026-11-12): 115 top-level billing checks across seven packages passed against owned local data; the broader short suite covered 63 packages including API route/contracts and SSE. Nine legacy HTTP end-to-end scenarios and six live lifecycle boundary checks passed. API generation/check, UI type-check/production build, and four fixture-safety tests passed. A fresh review's four important findings were reproduced and fixed in one verified pass. Native execution used checkpoint `7d8d895` plus one atomic final Task 5/6 commit because their remaining financial and migration wiring shares contracts.

The local UI/API/worker use one main Compose project, with telemetry disabled and synthetic credentials; other projects and production data were untouched. Existing apps remain legacy and new activation defaults off. Ambiguous historical money, active legacy writers, unfinished operations, legacy gateway bindings and unsupported active subscriptions block cutover. Operator currency review and drained-writer recovery are explicit audited actions. Merchant collection, recurring renewals, delivery outbox, the full billing UI overhaul and release rehearsals remain assigned to later phases. The guided user smoke test must finish before Phase 2.


## Phase 2 — Each business's Razorpay account

### Task 8: Encrypted merchant onboarding and immutable account binding

**Manual-mode compatibility requirement:** Razorpay is optional. Characterize `TestBizManualBillingDoesNotRequireMerchant` alongside the merchant tests: no merchant credentials, provider calls or gateway readiness requirement for manual invoice/cash/direct-UPI recording. Treat an already-passing manual path as baseline evidence, not as a missing-feature RED. Only online checkout/automatic collection require their supported merchant prerequisites. A manually entered UPI reference is staff-recorded evidence, not provider-verified settlement.

**Files:** Create `internal/domain/bizbilling/gateway.go`, `internal/infrastructure/bizbillingrepo/gateway_connection_repo.go`, `internal/usecases/services/bizbilling_gateway_connection_service.go`, `bizbilling_gateway_connection_test.go`, `internal/interfaces/http/handlers/biz_billing_handler_gateway.go`; modify lifecycle state slots, container/config bindings, index mappings, and routes. Reuse `internal/infrastructure/secrets/connection_codec.go` unchanged unless a narrowly tested compatibility extension is necessary.

**Interfaces:** `MerchantConnection{ID,AppID,AccountID string; Mode GatewayMode; Revision int64; Readiness string; Secrets map[string]channelconnection.EncryptedSecret}`; plaintext belongs only in write-only `MerchantPatch`/in-memory credentials. Produce `Save(ctx,appID string,mode GatewayMode,expectedRevision int64,patch MerchantPatch) (MerchantSnapshot,error)`, `Snapshot(ctx,appID string,mode GatewayMode) (MerchantSnapshot,error)`, `Resolve(ctx,GatewayRef) (MerchantCredentials,error)`, `Disconnect(ctx,appID string,mode GatewayMode,expectedRevision int64) error`.

- [x] **1. Tests:** `TestBizMerchantEncryptionAndRBAC`, `TestBizMerchantSlotSwitchKeepsOldOrder` (RF3), `TestBizMerchantRotationKeepsCallbackSecret`. Assert wrong-app AAD fails; reads/logs omit keys; test/live cannot cross; stale revision conflicts; old connection/account remains resolvable after slot replacement.
  ```go
  require.NotContains(t, responseBody, fixtureSecret)
  require.Equal(t, original.AccountID, oldOrderCredentials.AccountID)
  require.Error(t, crossAppDecryptErr)
  ```
- [x] **2. Red:** Owned runner: `go test ./internal/usecases/services ./internal/interfaces/http/handlers -run 'TestBizMerchant' -count=1`.
- [x] **3. Implement:** Use existing AES-GCM binding with connection/app/mode/provider/field; reuse the configured encryption keyring, never the platform Razorpay keys. Changing merchant identity creates a new immutable connection; slot switching does not overwrite the old account. Retain referenced connection/secret revisions and block destructive deletion while in-flight effects exist. Validate credentials with a documented read-only API and pin the expected merchant account ID; show API authentication and signed-callback account verification as distinct readiness facts. A key prefix alone proves neither account ownership nor recurring capability. Do not assume a partner-only account API is available to ordinary merchants. Gate live automatic readiness on verified linkage/capabilities, and verify payment/order ownership using the pinned credentials on every financial settlement. Missing keyring fails onboarding closed.
- [x] **4. Green:** Repeat including unavailable retained key, previous webhook secret, disconnected slot with a late old capture, and permission denial.
- [x] **5. Commit:** Stage listed files; `git commit -m "feat(biz-billing): connect encrypted per-app Razorpay merchants"`.

Task 8 verification: owned local merchant/lifecycle regression batch passed (services 9.612s, provider 2.192s, handlers 0.231s, repository 0.210s; container/migrate compile passed). Includes real lost-write acknowledgments, expired metadata-fence recovery, unavailable retained keys and gateway-free cash/UPI/bank recording. API authentication alone leaves account verification and automatic readiness false. No live provider canary performed.

### Task 9: App-scoped orders and verified checkout

**Files:** Create `internal/infrastructure/payment/biz_razorpay.go`, `biz_razorpay_test.go`, `internal/usecases/services/bizbilling_checkout_service.go`, `bizbilling_checkout_test.go`; modify `internal/interfaces/http/handlers/biz_portal_handler.go`, container wiring, and domain gateway contracts. Keep platform `razorpay.go` behavior independent.

**Interfaces:** `GatewayFactory.For(ctx,GatewayRef) (BizGateway,error)`; `BizGateway.CreateOrder(ctx,OrderIntent) (GatewayOrder,error)`, `FindOrder(ctx,OrderIntent) (*GatewayOrder,error)`, `FetchPayment(ctx,paymentID string) (CapturedPayment,error)`, `VerifyCheckout(orderID,paymentID,signature string) error`. `OrderIntent` pins operation/invoice/account/mode/currency/amount/receipt; `CapturedPayment` carries provider IDs/account/captured status/amount/currency. `BizCheckoutService.Start(ctx,appID,userID,invoiceID string,mode GatewayMode,key string) (CheckoutDescriptor,error)` and `Verify(ctx,appID,userID,invoiceID string,proof CheckoutProof) (OperationResult,error)`. Descriptor adds public key ID, amount/currency, order ID, merchant display name, operation ID; no private credential.

- [x] **1. Tests:** `TestBizCheckoutMerchantIsolation`, `TestBizCheckoutVerifyServerFacts`, `TestBizOrderUnknownOutcomeDoesNotRecreate` (RF2). Cover paid/void/automatic-locked invoices, tampered signature, authorized-but-uncaptured payments, amount/currency mismatch, partial manual balances, and lost order response.
  ```go
  require.Equal(t, invoice.AmountDuePaisa, descriptor.AmountPaisa)
  require.Equal(t, 1, remoteOrderCreateCalls)
  require.NotEqual(t, "paid", uncapturedInvoice.Status)
  ```
- [x] **2. Red:** `go test ./internal/infrastructure/payment ./internal/usecases/services -run 'TestBiz(Checkout|Order)' -count=1`; real ES variants through runner.
- [x] **3. Implement:** Persist external-effect intent before API call; use bounded 10-second request timeout and cancelable HTTP transport. Reconcile exact stored account/receipt/amount/currency on unknown outcomes; if lookup is ambiguous/unavailable, leave pending instead of issuing another order. Only expose supported verified facts. Add `POST /v1/portal/:token/invoices/:id/verify-payment`; preserve existing `/pay` fields while extending its descriptor. New v2 orders never fall back to platform credentials.
- [x] **4. Green:** Strict `httptest` wire tests plus owned repository/real-handler flow; demonstrate one remote order after crash/replay and zero false paid states.
- [x] **5. Commit:** Stage listed files; `git commit -m "feat(biz-billing): add scoped invoice checkout and capture verification"`.

Task 9 verification: final owned batch passed (services 67.764s, provider 1.003s, real portal HTTP handlers 2.757s; container/domain/migrate compile passed). Verified wrong account/money/status/proof rejection, one capture on replay, lost remote/persisted response recovery, test/live invoice separation, legacy-mode precondition and a new remainder attempt only after earlier capture settles. Retained attempt chains and key bindings preserve old order identities. Provider canary remains unperformed.

### Task 10: Signed durable gateway inbox and replay-safe settlement

**Files:** Create `internal/infrastructure/bizbillingrepo/gateway_event_repo.go`, `internal/usecases/services/bizbilling_gateway_event_service.go`, `bizbilling_gateway_event_test.go`, `internal/interfaces/http/handlers/biz_billing_gateway_callback.go`, `biz_billing_gateway_callback_test.go`; modify routes, container wiring, and existing legacy `BizGatewayRecorder` adapter only where needed to preserve its bound historical orders.

**Interfaces:** `Accept(ctx,connectionID,eventID string,raw []byte,signature string) (GatewayEventReceipt,error)`, `Process(ctx,eventID string) (OperationResult,error)`, `Reconcile(ctx,limit int) (int,error)`. Receipt stores raw-body digest, encrypted/minimized event fields, verified account/mode/secret revision, and processing state. Internal lookup IDs must be namespaced by connection, not the untrusted header alone.

- [x] **1. Tests:** `TestBizGatewayReplayIdentity` (RF1), `TestBizGatewayOutOfOrder`, `TestBizGatewayWrongAccountOrMoney`, `TestBizInboxPersistBeforeAck`. Same signed body/new header settles once; same header/different body is quarantined and reported; concurrent captured/order-paid events share one financial effect; unknown invoice remains visible for reconciliation.
  ```go
  require.Equal(t, int64(1), settlementFacts)
  require.Equal(t, 503, persistFailedHTTPStatus)
  require.Equal(t, "unmatched", orphanReceipt.Status)
  ```
- [x] **2. Red:** Owned runner: `go test ./internal/usecases/services ./internal/interfaces/http/handlers -run 'TestBiz(Gateway|Inbox)' -count=1`.
- [x] **3. Implement:** Verify raw HMAC with retained valid secret revisions, then persist verified receipt before 2xx. Deduplicate independently by underlying merchant payment/refund/period identity; changing unsigned event headers cannot bypass accounting dedupe. No local settlement from browser proof or unverified metadata. New merchant callbacks do not reuse the notification-only connector parser, and no event can settle a platform credit purchase accidentally.
- [x] **4. Green:** Repeat with reordered captured/failed events, ES outage before/after inbox acceptance, rotated credentials, real stored legacy order routing, and restart recovery. Never swallow errors with an unpersisted success acknowledgment.
- [x] **5. Commit:** Stage listed files; `git commit -m "feat(biz-billing): reconcile signed gateway events durably"`.

Task 10 verification: signed inbox, replay/header collision, out-of-order and concurrent capture, unmatched/restart recovery, retained-secret/API rotation, app-scoped status reads and storage-before-ack HTTP tests passed. Final focused services 34.311s, handlers 2.313s, provider 2.284s; repository/container/migrate compile passed. Full checkout/callback regression also passed (services 95.031s), including real-wire browser rotation. Stored legacy-order callback and platform order/credit idempotency contracts passed separately (2.624s). Original data/bindings retained; no provider canary or production change.

### Task 11: Real refunds, provenance reservations, and uncertain outcomes

**Additional verified safety requirement:** a real local regression showed test-mode overpayment entering the live customer wallet. Keep sandbox money in separate optional `Scope.mode=test` streams, exclude it from default real-money reports and primary customer balance mirrors, and prohibit cross-mode spending/refunds. Missing mode remains live with byte-identical existing stream IDs; do not rewrite production balances. Gateway mode is pinned by the business invoice. Test document numbering must not consume the live sequence. Phase 3 notification outbox must suppress real receipt/reminder delivery for sandbox money by default. This corrects test/live isolation within Phase 2; it does not expand into a new payment provider or broader UI restructuring.

**Files:** Create `internal/usecases/services/bizbilling_refund_service.go`, `bizbilling_refund_test.go`; extend `internal/infrastructure/payment/biz_razorpay.go`, its tests, domain gateway/payment contracts, and the existing refund handler facade.

**Interfaces:** Add `BizGateway.Refund(ctx,RefundIntent) (GatewayRefund,error)`, `FindRefund(ctx,RefundIntent) (*GatewayRefund,error)`; produce `BizRefundService.Request(ctx,appID,paymentID string,req RefundPaymentRequest,key string) (OperationResult,error)` and `Confirm(ctx,GatewayRefund) (OperationResult,error)`. `RefundIntent` pins account/payment/currency/amount/reservation/operation; offline recording is an explicit authorized command, not a successful online refund.

- [x] **1. Tests:** `TestBizRefundOnlyAfterProviderConfirmation`, `TestBizRefundSpentCreditCap` (RF4), `TestBizRefundUnknownOutcomeAfterRotation` (RF2), `TestBizRefundDoesNotReopenInvoice`. Include a 1,200 payment/1,000 invoice/200 excess: spent excess limits refund to 1,000; unspent excess may be reversed with the refund; simultaneous spend/refund cannot both consume it.
  ```go
  require.Equal(t, int64(1000), refundableWithSpentExcess)
  require.Equal(t, 1, providerRefundCalls)
  require.Equal(t, "paid", originalInvoice.Status)
  require.Equal(t, "pending", uncertain.Status)
  ```
- [x] **2. Red:** Owned runner: `go test ./internal/usecases/services ./internal/infrastructure/payment -run 'TestBizRefund' -count=1`.
- [x] **3. Implement:** Journal reservation before external call; reserve both cash refund budget and relevant source credit lots. On confirmation journal the refund/reversal and release holds exactly once; on definitive failure release reservations; on uncertainty retain holds and reconcile without blind retry. Validate provider refund state/amount/currency/account. Legacy provenance ambiguity requires explicit review, not assumed refundability.
- [x] **4. Green:** Repeat with partial refunds, duplicate callbacks, multiple refund requests, provider timeouts after success, disconnected merchants, and attempted refund beyond unreserved funds. Verify no debt or double credit.
- [x] **5. Commit:** Stage listed files; `git commit -m "fix(biz-billing): reconcile real refunds without double credit"`.

**Phase 2 checkpoint:** Demonstrate two apps using distinct fixture merchants, test/live separation, successful capture/refund recovery, and preserved platform/legacy gateway contracts. Report local gateway fixtures separately from any real Razorpay test-mode canary; lack of credentials never becomes a claim of live-provider verification.

**Native Phase 2 checkpoint — 2026-11-12:** Tasks 8-10 are committed as `f49bb93`, `f1ca44a`, and `1c4029f`; the Task 11 commit containing this entry completes the phase. The final refund run covered 17 top-level regressions (services 74.360 s, handlers 3.227 s, provider 2.024 s), including pending versus processed/failed outcomes, partial/multiple refunds, spent excess, spend/refund races, disconnect/rotation, signed duplicate callbacks, restart and lost durable-write responses. A documented provider `refunded` status exposed a second-request edge case; an already fully refunded local payment now rejects new unattempted work rather than leaving a new effect permanently pending. Original uncertain attempts retain their holds and never receive a blind second provider POST.

Sandbox wallet/stream isolation and distinct test numbering passed actual local regressions; existing live scope serialization/IDs remain unchanged. Only additive gateway metadata mappings were applied, with no index deletion, recreation or financial backfill. Native fixture job metadata was archived with original snapshots retained; financial records and volumes were preserved. The existing migration CLI summary calls the three metadata mapping updates “created”; accurate action/count wording is a minor Phase 5 operator-visibility follow-up, not evidence of index recreation.

Local verification is available at `http://127.0.0.1:18762`. The user’s smoke invoice remains USD 11.80 due, USD 0.00 paid, with no payments; its existing overdue/past-due lifecycle statuses were preserved. UI currency selection, customer search and Amount Paid display correction remain assigned to Phase 4. Actual receipt delivery and subscription collection modes remain Phase 3. No real merchant credentials were used, no production data was accessed, and no push, PR or deployment was performed. **Wait for the user before starting Task 12.**

## Phase 3 — Both subscription modes and invoice lifecycle

### Task 12: Complete manual invoice flow and immutable issued snapshots

**Explicit gateway-free journey:** An authorized business without Razorpay can issue an invoice, record cash or a direct-UPI/bank amount and reference, handle partial/full settlement, and obtain a receipt. Preserve the distinction between staff-confirmed external money and provider-verified capture. Gateway unavailability must not prevent manual invoice issue or receipt recording. Task 15 owns reliable receipt delivery, independently of the completed money fact; Task 17 exposes the manual controls without requiring gateway onboarding. Protect automatically owned invoices under the existing collection-owner rules.

**Files:** Create `internal/usecases/services/bizbilling_invoice_service.go`, `bizbilling_invoice_test.go`; modify domain `invoice.go`, `subscription.go`, `config.go`; modify lifecycle facade, financial commands, plan/product services, and `internal/interfaces/http/handlers/biz_billing_handler_lifecycle.go`.

**Interfaces:** Produce `BizInvoiceService.Issue(ctx,appID,invoiceID,key string) (*BizInvoice,error)`, `Preview(ctx,appID string,req CreateInvoiceRequest) (InvoicePreview,error)` and `Document(ctx,appID,userID,invoiceID string) (BillingDocument,error)`. `BillingDocument` contains immutable issued issuer/customer identity, line items, currency, rounding/tax totals, invoice number, timestamps, and applied payment/refund references. Product/plan edits cannot mutate an issued snapshot. Subscriber references must belong to the app.

- [x] **1. Tests:** `TestBizManualInvoiceEndToEnd`, `TestBizIssuedSnapshotSurvivesProfileEdit`, `TestBizPreviewEqualsIssue`, `TestBizDraftAndReferencedEntityValidation`. Cover zero/partial/full payment, coupon/credit/retainer application, void/write-off, and archived products/plans with existing subscriptions.
  ```go
  require.Equal(t, preview.TotalPaisa, issued.TotalPaisa)
  require.Equal(t, originalIssuer, document.Issuer)
  require.Equal(t, "partially_paid", partiallyPaid.Status)
  ```
- [x] **2. Red:** Owned runner: `go test ./internal/usecases/services ./internal/interfaces/http/handlers -run 'TestBiz(ManualInvoice|Issued|Preview|Draft)' -count=1`.
- [x] **3. Implement:** Keep old paths/envelopes, checked totals, currency-aware adjustments, stable numbering, and one issue event. Separate gateway order preparation from invoice issue CAS; a gateway outage leaves a valid issued invoice with visible checkout-pending state, not a fake payment link or duplicate number. Snapshot data only at issue; preserve historical issued totals as-is.
- [x] **4. Green:** Repeat; validate statements/analytics by currency and advanced estimate/contract/retainer integrations touched by this refactor.
- [x] **5. Commit:** Stage listed files; `git commit -m "feat(biz-billing): complete recoverable invoice and document lifecycle"`.

**Task 12 local checkpoint:** Invoice preview, issue snapshots, staff-confirmed cash/UPI references and scoped document payment/credit/refund history are implemented. Thirty-seven selected top-level billing regressions passed across four packages, including schema-2 completed/frozen/accepted recovery. New facts use schema 3 to prevent older workers discarding snapshots; historical schema-2 facts remain intact. Archived-catalog renewal coverage is tracked with Task 14, where the period engine is implemented. Receipt delivery remains Task 15 and dashboard controls remain Phase 4.

### Task 13: Explicit mandate authorization and collection-mode transitions

**Files:** Create `internal/usecases/services/bizbilling_recurring_service.go`, `bizbilling_recurring_test.go`; extend domain `gateway.go`, `subscription.go`, adapter `biz_razorpay.go`, lifecycle facade/handlers, container, and routes.

**Interfaces:** Add `BizGateway.CreateSubscription(ctx,RecurringIntent) (GatewaySubscription,error)`, `FindSubscription(ctx,RecurringIntent) (*GatewaySubscription,error)`, `FetchSubscription(ctx,id string) (GatewaySubscription,error)`, `ChangeSubscription(ctx,id string,action SubscriptionAction) (GatewaySubscription,error)`. Produce `BizRecurringService.Authorize(ctx,appID,userID,subscriptionID string,mode GatewayMode,key string) (AuthorizationDescriptor,error)` and `ChangeCollection(ctx,appID,subscriptionID string,mode CollectionMode,key string) (OperationResult,error)`. Add `POST /biz/subscriptions/:id/collection-mode` and customer `POST /portal/:token/subscriptions/:id/authorize`.

- [x] **1. Tests:** `TestBizAutomaticNeedsExplicitAuthorization`, `TestBizCollectionModeTransition`, `TestBizUnsupportedAutomaticPlanRejected`, `TestBizAutomaticInvoiceExcludesManualPayment`. Confirm no mandate is created for legacy/missing mode; authorized is not captured; changing to automatic does not charge arrears; changing away waits for remote cancellation of future debits.
  ```go
  require.Zero(t, legacyMandateCreateCalls)
  require.Equal(t, "invoice", legacy.CollectionMode)
  require.ErrorIs(t, parallelManualPaymentErr, ErrAutomaticCollectionInProgress)
  ```
- [x] **2. Red:** Owned runner: `go test ./internal/usecases/services ./internal/infrastructure/payment -run 'TestBiz(Automatic|Collection|Unsupported)' -count=1`.
- [x] **3. Implement:** Verify current official Razorpay request/signature/state contracts against primary docs and strict HTTP fixtures before coding their mappings. Persist authorization/remote references; recurring charge total includes the frozen invoice's actual tax/discount/setup/add-on effects. If these cannot be represented by the supported fixed plan, reject automatic setup with an explanation. Provider plans are versioned; never edit a shared remote plan to change old subscriptions. Native lifecycle transitions remain pending during unknown outcomes. Hosted authorization keeps card/mandate secrets outside FRN.
- [x] **4. Green:** Repeat with unsupported currencies/cycles/pricing, lost create response, remote pause/cancel failures, late captures, and merchant slot/key changes. A customer choosing invoice mode must still have a complete working path.
- [x] **5. Commit:** Stage listed files; `git commit -m "feat(biz-billing): authorize automatic collection per subscription"`.

**Task 13 local checkpoint:** Optional recurring-provider capability, fixed-price mandate preparation, customer signature verification, signed subscription callbacks, account/capability readiness, cancellation fencing and restart recovery are implemented. Sixty-nine selected regressions passed; all five business-provider contract tests also pass, including pause/resume wire requests and provider failures. New-subscription initial periods, due-period synchronization and native pause/resume lifecycle coordination are verified with Task 14; no Phase 3 completion or live-provider claim is made here.

### Task 14: Idempotent renewals, provider periods, and scheduler catch-up

**Files:** Create `internal/usecases/services/bizbilling_period_service.go`, `bizbilling_period_test.go`; modify scheduler, lifecycle facade, recurring/event services, domain subscription fields, and repository stable-page queries.

**Interfaces:** Produce `EnsurePeriod(ctx,appID,subscriptionID string,start,end time.Time) (OperationResult,error)` and `RunDue(ctx,limit int) (int,error)`; derive period ID from app/subscription/confirmed period-start, not worker time. Store pending change timing and original anniversary. Provide PIT/`search_after` iteration ordered by due time, mapped `id` keyword, and PIT `_shard_doc` tie-breaker (never sort on ES `_id`); recheck each record's current due state before mutation. Page size 100, renewable 30-second claims, no single-page cap on a run. Expired PIT restarts the idempotent scan safely.

**Shared acceptance dependencies:** Verify archived catalog entries do not break existing subscription renewals (from Task 12). Model a new automatic subscription as awaiting its initial authorized provider period; do not give away the first cycle by generating a manual first invoice and scheduling the first debit a month later. Preserve future-only starts and no-arrears charging when an existing invoice subscription changes to automatic. Native automatic pause/resume changes must wait for confirmed provider outcomes; adapter contracts are already covered by Task 13.

- [x] **1. Tests:** `TestBizRenewalCrashDoesNotLoseInvoice`, `TestBizMultipleSchedulersOnePeriod`, `TestBizMoreThan100DueSubscriptions`, `TestBizProviderInvoiceMapsToSamePeriod`, `TestBizLegacyChangePlanTiming`. Run 250 due subscriptions and two schedulers; crash between invoice creation and period advancement, including trial conversion and zero-value invoices.
  ```go
  require.Equal(t, 250, completedPeriods)
  require.Equal(t, int64(1), invoiceCountForPeriod)
  require.Equal(t, providerPeriodStart, internalPeriodStart)
  ```
- [x] **2. Red:** Owned runner: `go test ./internal/usecases/services -run 'TestBiz(Renewal|MultipleSchedulers|MoreThan|ProviderInvoice|LegacyChange)' -count=1`.
- [x] **3. Implement:** Journal/resume period creation and progression rather than update-then-create. Automatic subscriptions mirror provider-generated invoices/captures into the same period, without issuing a second debit. Manual subscriptions issue their own invoice. Catch up bounded pages until no due work remains or context cancels; cancellation resumes next run. Never infer paid status from renewal. Preserve old immediate manual proration; explicit new next-period changes snapshot future prices. Handle trial/past-due/pause/cancel/non-renewing states through the same fenced lifecycle.
- [x] **4. Green:** Repeat including leap years, month-end anchors, delayed events, restart, provider/internal invoice ordering, and incompatible auto plan changes. Verify no period skipped or charged twice.
- [x] **5. Commit:** Stage listed files; `git commit -m "fix(biz-billing): recover renewals and deduplicate billing periods"`.

Task 14 verification: durable period, interrupted projection, competing schedulers and 250-subscription pagination passed. Final affected regression batch: 49 top-level tests, 0 failures, 0 skips; services300.754s, domain2.550s, provider3.048s, HTTP10.209s, repository/container compilation passed. Tests protect initial automatic billing, confirmed-stop cash/UPI collection, unstarted cancellation, native terminal outcomes/pause eligibility, frozen trial pricing, debt status, list read fencing, reviewed migration and historical schemas2/3. Earlier expanded migration/invoice checks passed; no live debit or production change.

### Task 15: Durable invoice, receipt, and reminder delivery

**Files:** Create `internal/infrastructure/bizbillingrepo/notification_outbox_repo.go`, `internal/usecases/services/bizbilling_outbox_service.go`, `bizbilling_outbox_test.go`; extend `internal/domain/bizbilling/notification_intent.go` from Task 4, and modify notifier, scheduler, operation/event services, config DTOs, container, and worker wiring.

**Interfaces:** `NotificationIntent{ID,AppID,UserID,TemplateID,EventID,InvoiceID,StepID,Channel string; Payload map[string]string}`; `DispatchDue(ctx,limit int) (int,error)` and `RefreshDelivery(ctx,limit int) (int,error)`. Intent ID uses app/event/template/step/channel; progress `pending|accepted|delivered|failed|unknown`. Add `GET /biz/notification-deliveries` and authorized retry endpoint via a small handler; return sanitized failure codes.

- [ ] **1. Tests:** `TestBizNotificationFailureDoesNotUndoPayment`, `TestBizSameDayReminderChannels`, `TestBizOutboxAdmissionRecovery`, `TestBizAcceptedIsNotDelivered`. Include credit rejection, absent channel readiness, duplicate worker, restart after enqueue response loss, and retry exhaustion.
  ```go
  require.Equal(t, "paid", invoice.Status)
  require.Len(t, sameDayEmailAndWhatsAppIntents, 2)
  require.Equal(t, "accepted", enqueuedOnly.Status)
  ```
- [ ] **2. Red:** Owned runner: `go test ./internal/usecases/services -run 'TestBiz(Notification|SameDay|Outbox|Accepted)' -count=1`.
- [ ] **3. Implement:** Persist intent as part of accepted operation progress; separate payment completion from notification delivery. Immutable journal payloads contain document/link references, not raw bearer URLs. Where retryable delivery must retain a link, use the existing secret codec for its sensitive outbox envelope and resolve it only at dispatch. The ordinary notification engine may still persist rendered delivery content containing the link under its existing retention policy: disclose and test that boundary instead of claiming all copies are hashed. Use existing admission/queue dedupe and app channel configuration; do not modify generic API semantics to fit billing. Retry known admission failures with bounded backoff; query deterministic notification identity after uncertainty before resending. Keep unknown provider outcomes visible. Reminder keys include step/channel; legacy already-sent reminder markers are imported conservatively to avoid resending history.
- [ ] **4. Green:** Repeat; verify retrying a receipt never recharges/refunds a customer and no false delivered state appears in the business API.
- [ ] **5. Commit:** Stage listed files; `git commit -m "feat(biz-billing): persist and recover receipt and reminder delivery"`.

**Phase 3 checkpoint:** Execute one-time, manual recurring, and authorized automatic flows through real handlers/stores with strict local gateway fixtures. Report scheduler/concurrency/replay results, notification failure recovery, and unsupported automatic combinations. No real debit or automatic migration of existing subscribers.

## Phase 4 — Responsive business UI and customer portal

### Task 16: Guided app setup, typed API contracts, and focused navigation

**Guided smoke-test feedback, assigned to this Phase 4 task:** Offer an application-level default-currency selector, initially INR for new setups, with explicit alternate supported currencies. Persist the choice and use it in catalog/add-on forms and displays instead of hardcoded USD. Preserve existing configured defaults and historical plan/invoice/payment/balance currencies and amounts. Track the requested behavior in [the deferred currency issue](https://github.com/the-monkeys/freerangenotify/issues/130). Real-time exchange-rate conversion and conversion charges require a separate future design; do not introduce them implicitly or recalculate historical money when the default changes.

**Files:** Create `ui/src/components/biz-billing/BizBillingSetup.tsx`, `BizBillingSettings.tsx`, `BizBillingCatalog.tsx`, `useBizBillingResource.ts`, `ui/src/services/bizBilling.ts`, `ui/src/types/bizBilling.ts`, `e2e/biz-billing.config.ts`, `e2e/biz-billing/setup-catalog.spec.ts`; modify `AppBizBilling.tsx`, `biz-billing/shared.tsx`, `ui/src/services/api.ts`, `ui/src/types/index.ts`, app-detail/sidebar navigation as required. Reuse `scripts/docs-preview.mjs` for clean preview; do not change its existing fixture defaults.

**Interfaces:** Move billing methods into `createBizBillingAPI(client: AxiosInstance,authHeaders:(apiKey:string)=>Record<string,string>)` in `services/bizBilling.ts`; initialize it once from `services/api.ts` with the existing client/header helper, preserving the `bizBillingAPI` export. The extracted module must not import `api.ts`, avoiding a circular dependency or a second auth/refresh implementation. New hook `useBizBillingResource<T>(key:string,load:(signal:AbortSignal)=>Promise<T>)` returns data/loading/error/retry and discards stale app responses. New setup forms consume lifecycle/gateway/config contracts; no secret in caches/persistent browser storage. Browser config uses isolated preview, no production storage state/global setup.

- [ ] **1. Tests:** `setup-catalog.spec.ts`: switch app while a request is pending; prove data cannot cross apps; gateway errors are not rendered as empty success; secret fields never echo; viewer cannot edit; product/plan CRUD, pagination, typed tax/numbering forms and readiness work at all four widths.
  ```ts
  await expect(page.getByRole('alert')).toContainText('Could not load');
  await expect(page.getByLabel('Razorpay key secret')).toHaveValue('');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  ```
- [ ] **2. Red:** `npx playwright test -c e2e/biz-billing.config.ts setup-catalog.spec.ts`; target new setup/real error flow must fail.
- [ ] **3. Implement:** Split existing component by behavior, keep its existing app entrypoint and advanced panels. Replace raw JSON with typed forms, app-aware loading/error states and explicit upgrade readiness/review. Load/paginate only active view. Server validation remains authoritative; confirmation for activation shows unresolved balances and immutable old-account references. Use existing responsive/accessibility primitives.
- [ ] **4. Green:** Repeat with light/dark, keyboard navigation, disconnected merchants, stale revisions, secret rotation, and 503 retry without losing form data. `npm --prefix ui run build` passes.
- [ ] **5. Commit:** Stage listed files; `git commit -m "feat(ui): guide app billing setup and responsive catalog management"`.

### Task 17: Subscription, invoice, payment, and reconciliation workflows

**Guided smoke-test feedback, assigned to this Phase 4 task:** Replace manual customer-ID entry with an application-scoped searchable subscriber picker showing recognizable name/email, suitable for 500-1000+ customers without preloading the entire customer base. Reuse the existing subscriber search API and preserve the internal `user_id` request contract. Add a regression for the discovered invoice-table mapping: Amount Paid must use the saved paid amount, not the invoice total. Verify unpaid, partially paid, fully paid, credited and refunded cases against saved amounts; the local smoke invoice had USD 11.80 total/due and USD 0 paid while the current table incorrectly displayed USD 11.80 paid. These findings are deferred to the assigned UI phase at the user's request; do not interrupt the Phase 1 smoke test with implementation.

**Files:** Create `ui/src/components/biz-billing/BizBillingOverview.tsx`, `BizBillingSubscriptions.tsx`, `BizBillingInvoices.tsx`, `BizBillingPayments.tsx`, `BizBillingDeliveries.tsx`, `e2e/biz-billing/business-workflows.spec.ts`; modify the app billing shell/shared types/API from Task 16 and existing advanced panels only where needed for currency or operation status.

**Interfaces:** UI mutation helpers retain one idempotency key per submitted action and return/resume `operation_id`; no automatic retry with a new key. Use existing subscriber search API; do not add a separate billing customer entity. Expose legacy-compatible immediate and explicit next-period change timing honestly.

- [ ] **1. Tests:** `business-workflows.spec.ts`: select subscriber by search; create invoice/subscription with either mode; show unsupported auto combination before authorization; preview/issue/pay/refund; recover interrupted operation; preserve form on conflict; show ambiguous legacy credits and blocked spent-credit refunds. Ensure reports are separated by currency and advanced features remain reachable.
  ```ts
  await expect(page.getByRole('status')).toContainText('Payment pending');
  await expect(page.getByRole('button', {name:'Record payment'})).toBeDisabled(); // auto-owned invoice
  expect(new Set(submittedIdempotencyKeys).size).toBe(1);
  ```
- [ ] **2. Red:** `npx playwright test -c e2e/biz-billing.config.ts business-workflows.spec.ts`.
- [ ] **3. Implement:** Focused forms/detail views, currency/minor-unit conversion without floating money drift, payment/refund confirmations, scoped retry/status polling with cancellation, and honest delivery states. Use responsive cards or contained secondary-table scrolling. Do not hide API errors or expose infrastructure jargon as ordinary customer instructions.
- [ ] **4. Green:** Repeat all four widths, permissions, empty/loading/error states, network interruption after successful mutation, and app navigation mid-request; build UI.
- [ ] **5. Commit:** Stage listed files; `git commit -m "feat(ui): complete subscriber invoice and payment workflows"`.

### Task 18: Secure portal tokens and customer-scoped action APIs

**Files:** Modify `internal/usecases/services/bizbilling_portal_service.go`, `internal/interfaces/http/handlers/biz_portal_handler.go`, portal-token model in `internal/domain/bizbilling/connector.go`, `cmd/server/main.go`, and routes; create `internal/interfaces/http/middleware/biz_portal_privacy.go`, `biz_portal_privacy_test.go`, `internal/usecases/services/bizbilling_portal_test.go`, `internal/interfaces/http/handlers/biz_portal_security_test.go`.

**Interfaces:** Preserve `CreateToken/Resolve/PortalLink` calls; add `Revoke(ctx,appID,tokenID string) error` and `ResolveScope(ctx,token string) (PortalScope,error)` where `PortalScope{AppID,UserID,TokenID string; ExpiresAt time.Time}`. Lookup uses the v2 token digest and permitted scope, never a plaintext token index key. If a stable existing `payment_link` response/retry requires re-rendering a newly issued link, retain its bearer value only in an AES-GCM envelope using the existing codec, with token/app/customer/purpose binding; do not persist plaintext on a v2 invoice or journal. Do not rotate/mint a new link on each portal GET, which would allow uncontrolled extension of access. Add customer subscription list/detail, operation status, invoice document and receipt endpoints beneath `/portal/:token`, checking the same customer/app each time. Server callback processing uses merchant scope, never portal-token validity.

- [ ] **1. Tests:** `TestBizPortalCrossCustomerAndApp`, `TestBizPortalLegacyExpiry`, `TestBizPortalExpiredAfterCheckoutCapture` (RF5), `TestBizPortalSecretsNeverLogged`. Try guessed invoice/payment/operation IDs, revoked/expired links, and existing raw legacy tokens.
  ```go
  require.Equal(t, 404, otherCustomerInvoiceStatus)
  require.Equal(t, 401, expiredPortalStatus)
  require.Equal(t, "paid", capturedAfterExpiry.Status)
  require.NotContains(t, logs, rawPortalToken)
  ```
- [ ] **2. Red:** Owned runner: `go test ./internal/usecases/services ./internal/interfaces/http/handlers -run 'TestBizPortal' -count=1`.
- [ ] **3. Implement:** Constant-meaning digest lookup for new tokens and bounded legacy lookup until original expiry; revocation and rate-limited access. Return no private gateway keys. Add `Referrer-Policy: no-referrer` and `Cache-Control: no-store`; install privacy controls before the global logger/OTel middleware in `cmd/server/main.go`. Log a templated route for portal requests and omit raw token-bearing URL attributes (skip automatic portal spans if this middleware cannot sanitize them safely). Test actual logger/trace output, not just handler log lines. Querying an operation must also verify customer ownership, not just app.
- [ ] **4. Green:** Repeat through real handlers; prove link expiry does not suppress a valid callback or grant renewed portal access. Preserve feature-disabled behavior.
- [ ] **5. Commit:** Stage listed files; `git commit -m "fix(biz-billing): protect customer portal scopes and legacy links"`.

### Task 19: Mobile customer checkout, authorization, invoices and PDF output

**Files:** Create `ui/src/pages/portal/BillingPortal.tsx`, `PortalInvoice.tsx`, `PortalSubscription.tsx`, `BillingDocument.tsx`, `ui/src/hooks/useBizRazorpayCheckout.ts`, `ui/src/services/bizPortal.ts`, `ui/src/pages/portal/print.css`, `e2e/biz-billing/customer-portal.spec.ts`; modify `ui/src/App.tsx`, `ui/nginx.conf`, and shared billing types. Do not change platform `useRazorpayCheckout.ts` to implement app billing.

**Interfaces:** Public `/portal/:token`, `/portal/:token/invoices/:invoiceId`, `/portal/:token/subscriptions/:subscriptionId`, and `/portal/:token/invoices/:invoiceId/print` routes. Hook handles descriptor-based order checkout or mandate authorization and then queries server operation state; SDK load is shared/deduplicated and errors are recoverable. `BillingDocument` renders the immutable document from Task 12, never current mutable business/user settings.

- [ ] **1. Tests:** `customer-portal.spec.ts`: invoice/full/partial payment, mandate consent, canceled checkout, delayed verified capture, refund receipt, expired/revoked legacy/new links, reload after success, and viewport/accessibility checks. RF5 expires the link after opening checkout, then verifies payment via the fixture callback while fresh portal access fails.
  ```ts
  await expect(page.getByText('Awaiting payment confirmation')).toBeVisible();
  expect(pdfBytes.subarray(0,4).toString()).toBe('%PDF');
  await expect(page.getByRole('button', {name:'Manage merchant credentials'})).toHaveCount(0);
  ```
- [ ] **2. Red:** `npx playwright test -c e2e/biz-billing.config.ts customer-portal.spec.ts`.
- [ ] **3. Implement:** Public customer-only navigation, real pending/failure states, invoice/receipt download through clearly labelled Print / Save as PDF, and brand-consistent accessible mobile layouts. `bizPortal.ts` uses the configured API base with a token-scoped public transport (`credentials: omit`), no dashboard Authorization/X-API-Key headers, and no dashboard 401 login-redirect interceptor. Configure Nginx's initial `/portal/` document response with no-referrer/no-store and sanitized or disabled raw-path access logging, so privacy does not depend on React loading first. No dashboard login, analytics leakage, token in localStorage, or dashboard API token in public code. Deterministic SDK stubs are CI evidence; actual test-mode SDK/callback acceptance is a separately recorded canary.
- [ ] **4. Green:** Repeat four widths, light/dark, keyboard/focus recovery, SDK load failure, malicious text in issued documents (React escapes), and generated A4 PDF layout with complete totals/identities. UI build passes.
- [ ] **5. Commit:** Stage listed files; `git commit -m "feat(ui): ship mobile billing portal and invoice payment flows"`.

**Phase 4 checkpoint:** Demonstrate the full business-to-customer journey with local gateway fixtures, all viewport checks, role isolation, old links, and actual PDF output. Report screenshots/test counts and remaining real-provider setup; do not call mocked payment acceptance a production success.

## Phase 5 — Related delivery issues and documentation

### Task 20: Verify WhatsApp broadcast and billing reminders (#87)

**Files:** Create `tests/integration/biz_billing_whatsapp_test.go`, `e2e/biz-billing/whatsapp-reminders.spec.ts`; extend existing `cmd/worker/channel_reply_integration_test.go`, `internal/usecases/services/channel_connection_routing_test.go`, and `e2e/whatsapp-runtime-config.spec.ts` only for uncovered regressions. Potential production fixes are restricted to demonstrated defects in `internal/usecases/notification_service.go`, `internal/usecases/services/channel_connection_service.go`, `internal/infrastructure/providers/meta_whatsapp_provider.go`, `whatsapp_provider.go`, and their exact existing tests; no speculative fixes in unrelated layers.

**Interfaces:** Consume existing channel admission/connection/broadcast/provider contracts and Task 15 outbox. No new WhatsApp API or alternate billing notifier. Merchant credentials must never be confused with notification-channel credentials.

- [ ] **1. Tests:** `TestBizWhatsAppBroadcastAndReminderScope`: two apps, approved Meta templates versus Twilio Content templates, invalid recipient/config, correct connection selection and billing link/body variables. Browser test starts from actual broadcast UI. Preserve SSE event/token contracts and custom SMTP send tests.
  ```go
  require.Equal(t, appAConnectionID, sent.ConnectionID)
  require.Equal(t, 1, acceptedNotificationsForIntent)
  require.Zero(t, sendsUsingAppBCredentials)
  ```
- [ ] **2. Red/characterize:** Run focused owned fixture integration and browser spec. If old report already passes, record it as verified locally and add only meaningful regression coverage; do not manufacture a code change or claim a vendor canary.
- [ ] **3. Implement:** For a reproduced failure, trace admission → prepared notification → queue → provider and fix its owning existing layer. Never bypass template approval/window/recipient rules to force success. Reuse billing outbox progress for errors.
- [ ] **4. Green:** Run the targeted reproducer plus impacted existing channel/SSE/custom SMTP regressions once. Record provider acceptance separately and leave issue closure unauthorized.
- [ ] **5. Commit:** Stage only actual test/fix files; `git commit -m "test(delivery): verify scoped WhatsApp broadcast and billing reminders"` (use a fix title if a defect is repaired).

### Task 21: Measure and repair demonstrated worker bottlenecks (#93)

**Files:** Create `cmd/worker/biz_billing_benchmark_test.go`; inspect/modify only profile-proven bottlenecks in existing `cmd/worker/main.go`, `processor_delivery.go`, `internal/infrastructure/queue/redis_queue.go`. Record results in this plan's execution evidence section, not another speculative report.

**Interfaces:** Add `BenchmarkBizBillingFanout6000` and `TestBizWorkerConfiguredConcurrency`. Exercise existing pool/queue/provider admission limits; fixture provider has a declared fixed latency and no external sends. Capture admission, queue wait p50/p95, provider latency, throughput, peak concurrency, and persisted terminal outcomes.

- [ ] **1. Baseline/test:** 6,000 distinct notifications with deterministic fake provider; hold provider calls to verify a configured pool above 10 actually permits that concurrency without bypassing per-app/provider limits. Assert every accepted ID is accounted for once; latency claims are limited to fixture conditions.
  ```go
  require.Equal(t, 6000, len(uniqueTerminalIDs))
  require.Greater(t, peakConcurrent, 10) // fixture configured for 32; provider allows 32
  require.LessOrEqual(t, peakConcurrent, 32)
  ```
- [ ] **2. Measure:** Owned runner: `go test ./cmd/worker -run TestBizWorkerConfiguredConcurrency -count=1`; then `go test ./cmd/worker -run '^$' -bench BenchmarkBizBillingFanout6000 -benchtime=1x -benchmem -cpuprofile <owned-temp-path>`. Store baseline parameters/results.
- [ ] **3. Implement only if demonstrated:** Fix polling/pool/connection/repeated-IO bottlenecks located by that profile; preserve fairness, ordered financial events, rate limits, and lease recovery. Do not just raise all concurrency or remove backpressure. If no bottleneck is demonstrated, ship tests/evidence with no performance production patch.
- [ ] **4. Verify:** Repeat the same fixture/hardware parameters; report measured before/after and integrity assertions. No “instant email” or unmeasured provider/inbox SLA claim.
- [ ] **5. Commit:** Stage actual listed files; `git commit -m "perf(delivery): verify and improve measured billing queue throughput"` (use test prefix if no runtime change).

### Task 22: Accurate billing/API and telemetry documentation (#114)

**Files:** Create `ui/src/docs/business-billing.md`, `business-billing-payments.md`, `customer-billing-portal.md`, `observability.md`, `tests/contracts/biz_billing_guide_cases_test.go`; modify `ui/src/config/docsNav.tsx`, new/existing business handler annotations, `examples/docs/api-cases.json`, `e2e/docs/reference.spec.ts`, and generated Swagger/OpenAPI outputs through existing `cmd/docsgen` only. Existing `tests/contracts/examples_test.go` helpers supply executable guide-case arrangements; telemetry claims are checked against `internal/telemetry/telemetry.go`.

**Interfaces:** Public examples show app creation, backend API-key use, product/plan/subscriber/invoice/payment flow, both subscription modes, merchant setup, webhook verification, portal links, refund/credit limits, and recovery operation IDs. Document actual feature/capability prerequisites and minor units. Explain collected telemetry/redaction/retention/configuration; no private token examples.

- [ ] **1. Tests:** `TestBizBillingGuideContracts` plus docs browser checks: every documented request uses actual DTOs/routes/envelopes, examples handle asynchronous operation status and legacy defaults, docs display on mobile, telemetry text matches configured instrumentation fields.
  ```go
  require.Equal(t, documentedStatus, actualHandlerStatus)
  require.NotContains(t, publicSpec, "key_secret_example_value")
  ```
- [ ] **2. Red:** `go test ./tests/contracts -run TestBizBillingGuideContracts -count=1`; add failing browser/route cases for newly documented endpoints.
- [ ] **3. Implement:** Write executable examples and regenerate with `go run ./cmd/docsgen`; preserve existing API doc URLs and UI spec fallback. Document actual provider sampling/request attributes and portal redaction verified in Task 18, not aspirational privacy claims. Cross-link relevant operational alerts and setup.
- [ ] **4. Green:** `node scripts/verify-docs.mjs`; `npx playwright test -c e2e/docs.config.ts`; verify generated copies, all handler/route coverage and examples. Revalidate existing docs/SDK checks if their shared registries changed.
- [ ] **5. Commit:** Stage explicit docs/annotation/fixture/generated files only; `git commit -m "docs(biz-billing): publish verified merchant and subscriber workflows"`.

**Phase 5 checkpoint:** Report #67/#87/#93/#114 evidence by issue, with unresolved provider canaries clearly separate. Keep #122 and the remaining template/SDK/retention issues outside this release as the approved design specifies. Do not close issues or send external messages.

## Phase 6 — CI, data recovery rehearsal, and release readiness

### Task 23: Mandatory business-billing CI with real stores and mobile checks

**Files:** Modify `.github/workflows/ci.yml`, `tests/integration/biz_billing_e2e_test.go`, fixture runner/tests, browser config; create `tests/integration/biz_billing_v2_e2e_test.go`, `scripts/verify-biz-billing.mjs`, `scripts/verify-biz-billing.test.mjs`.

**Interfaces:** `node scripts/verify-biz-billing.mjs` orchestrates explicit package tests through owned fixtures, full billing browser checks and existing docs/SDK compatibility. It fails on missing flags/credentials/fixture manifest or unexpected skipped required cases. Default CI uses local gateway fixtures, not repository payment secrets. Dedicated job `business-billing` on Ubuntu 24.04 uses Go 1.26.1/Node 24, billing enabled, CGO race checks, and the committed dependency locks.

- [ ] **1. Tests:** `TestBizBillingRequiredSuiteCannotSkip`, verifier environment/exit-code tests, and v2 real HTTP/ES/Redis E2E for app setup → prepare/activate → subscriber → invoice/manual/automatic payment → refund → statement/outbox → portal. Include feature-off and legacy apps simultaneously.
  ```go
  require.Zero(t, skippedRequiredBillingCases)
  require.Equal(t, before.LegacyIDs, after.LegacyIDs)
  require.Equal(t, "completed", finalOperation.Status)
  ```
- [ ] **2. Red:** `node --test scripts/verify-biz-billing.test.mjs`; run required suite with intentionally absent billing flag and confirm nonzero exit instead of skip.
- [ ] **3. Implement:** Wire mandatory isolated job; upload sanitized test/coverage/browser artifacts on failure, never secrets/tokenized URLs/raw gateway bodies. Keep CI/build checks independent from deployment. Linux `go test -race -p 2` covers new domain/repo/services/handlers and affected worker code. Test script and container ownership before cleanup.
- [ ] **4. Green:** `node scripts/verify-biz-billing.mjs`; existing verifier, SDK and mocked browser jobs; UI production build. Require all required billing tests to execute, not merely a green job with feature skips.
- [ ] **5. Commit:** Stage listed files; `git commit -m "ci(biz-billing): require real-store and mobile contract checks"`.

### Task 24: Backup/restore and migration rehearsal on owned fixtures

**Files:** Create `tests/integration/biz_billing_migration_rehearsal_test.go`, `scripts/rehearse-biz-billing-migration.mjs`, `documents/business-billing-rollout-runbook.md`; modify `tests/fixtures/biz-billing.compose.yml` to provide an owned snapshot volume and explicit ES `path.repo`, and modify migration service/CLI only if a rehearsal exposes a defect.

**Interfaces:** `node scripts/rehearse-biz-billing-migration.mjs` operates only on a verified owned fixture. It snapshots/restores fixture data, computes per-ID source checksums/currency totals, simulates reader-first/app-fenced upgrade, interruptions, restart, rollback of new admissions, and continued reconciliation. Real production procedures are documented but not executed.

- [ ] **1. Tests:** `TestBizMigrationRehearsalPreservesHistory` loads old/new-schema and ambiguous fixtures with unknown fields; attempts snapshot restore, repeat migration, stale report, failed currency assignment, legacy order late callback, and disabling new collection while accepted operations drain.
  ```go
  require.Equal(t, sourceChecksums, restoredChecksums)
  require.Equal(t, originalInvoiceNumbers, migratedInvoiceNumbers)
  require.Equal(t, int64(1), openingFactsPerAssignedBalance)
  require.Equal(t, "completed", drainingOperation.Status)
  ```
- [ ] **2. Red:** Run the fixture rehearsal before migration implementation is complete; it must expose missing restore/resume guarantees, not access external endpoints.
- [ ] **3. Implement:** Add only proven repairs. Runbook requires backup inventory/restore verification, reader-first deployment, draining old unaware writers, app review/cutover, merchant/account/primary-currency confirmation, and post-cutover reconciliation. Configure `FREERANGE_BIZ_BILLING_NEW_COLLECTION_ENABLED=false` to stop new FRN-initiated collection only; callbacks/recovery remain enabled. Existing Razorpay mandates continue until a separately confirmed provider pause/cancel: test and document this distinction. Do not disable the entire business module or revert to old writers while accepted v2 operations exist. No destructive down migration or rollback of financial history.
- [ ] **4. Green:** Run full rehearsal twice on distinct fresh fixtures; compare counts, IDs, numbers, source hashes, journal/currency invariants and migration checkpoints. Report simulated evidence separately from any unperformed production backup.
- [ ] **5. Commit:** Stage listed files; `git commit -m "test(biz-billing): rehearse data-preserving upgrade and recovery"`.

### Task 25: Whole-branch review, final verification, and handoff

**Files:** Update this plan's execution evidence and rollout runbook with exact commands/commits/results. Fix only confirmed review/CI defects in their owning files with focused regression tests; preserve unrelated local changes.

**Interfaces:** Consume all completed tasks. Native implementation is followed by one fresh whole-branch reviewer using `superpowers:requesting-code-review`; do not switch to per-task subagent implementation. Review app/account scoping, all money writers, external uncertainty, cutover leases, raw-body callbacks, portal token handling, refund provenance, and mobile/legacy behavior.

- [ ] **1. Verify:** Run `go test -p 2 -short -count=1 ./cmd/... ./internal/... ./pkg/... ./sdk/go/... ./tests/... ./examples/docs/go`, mandatory billing verifier and migration rehearsal, Linux billing/affected messaging race suite, `node scripts/verify-docs.mjs`, SDK tests, UI build, docs/mocked/billing browser suites. Use isolated env; no production `.env` or default external test URLs.
- [ ] **2. Review:** Obtain one fresh branch review against `94acfe1`. Reproduce significant findings, add failing regressions, repair, and rerun relevant checks. Do not equate reviewer silence with a provider canary or broad correctness proof.
- [ ] **3. Audit:** Check every spec requirement maps to a completed task; scan the entire outgoing commit range and committed tree with a checksum-verified secret scanner; inspect staged paths and generated contracts. Remove only this task's verified scratch/fixture artifacts; keep the user's files.
- [ ] **4. Remote CI if published:** The user explicitly permitted optional pushing of this branch on 2026-11-12, while forbidding a PR until separately instructed. Before any push, inspect workflow deployment triggers and outgoing files for secrets; push only this exact branch, inspect CI, and fix failures without deploying. If kept local, report that status and pending remote checks explicitly. No PR, force push, merge, production release or live billing migration is authorized by this step.
- [ ] **5. Handoff:** Commit evidence/runbook updates only. Report phase completion, test counts, branch/commits, review repairs, migration results, compatibility evidence, and remaining real-provider/production rollout prerequisites. Zero hidden required skips; no unsupported claim of zero future failures.

## Phase-wise reporting and execution ledger

### Presentation target and deferred application restructuring

User steering on 2026-11-12: aim to review a coherent billing pull request on Saturday, 2026-11-14, ahead of the business presentation on Sunday, 2026-11-15. This is a planning target, not authorization to push, merge, deploy, migrate production or represent unverified payment behavior as working. Demo readiness requires the demonstrated path to pass its functional, monetary-state, permission and compatibility checks. Distinguish a local/mock or provider-test demonstration from verified live collection. Keep the current six-phase scope focused; report any missing provider access or incomplete demo requirement explicitly.

Responsive billing UI verification belongs to Phase 4 at the existing four viewport widths, including keyboard, permission, loading/error and currency-state checks. Phase 6 must also exercise the critical existing application journeys across Templates, Subscribers, Notifications, notification-credit billing and business Billing Setup; identify coverage gaps rather than calling the whole application verified from a billing-only browser suite.

After the approved six phases are complete, revisit a broader restructuring of those screens and their navigation on the same branch. Record and review that later design before implementing it. Aim for a coherent journey while retaining the distinct meanings and balances of notification delivery credits and business subscriber invoicing/payments. Preserve existing API routes, response/authentication contracts, SSE integrations and production data. Keep focused, reviewable commits and relevant regression checks even on one large branch; file count is not a target, and unrelated cleanup or broad renaming is not part of the current scope.

Additional future usability requirement: support nontechnical businesses end to end, with a doctor sending patients reminders for appointments today or tomorrow as an example persona, not a separate healthcare-only product. Later design should explore task-oriented guided flows, understandable customer/template/channel/time selection, previews and clear delivery/payment outcomes, while keeping technical identifiers and provider setup out of ordinary daily tasks. Cover notifications and both billing areas coherently, with advanced administration still accessible. Use synthetic examples for demonstrations. A successful single-recipient notification does not establish bulk performance; retain realistic delivery/capacity verification as a requirement. This is deferred design feedback and must not expand or interrupt the currently approved six-phase work.

### Sunday demonstration checklist

- [ ] Use one owned local/test Compose cluster and synthetic customer data. Label local/mock versus real-provider test mode accurately; no live charges or production changes.
- [ ] Log in, open the test application, and explain the distinction between notification delivery credits and billing the business's own customers.
- [ ] Show the product and monthly plan with the verified price/currency. The current smoke setup uses USD; do not claim INR selection or FX is implemented until its tests pass.
- [ ] Create/select the test customer, attach the plan with the intended quantity/trial, and show the saved subscription and period.
- [ ] Verify invoice subtotal, tax, total, due and actual paid amount against saved data. An unpaid invoice has zero paid; fix or explicitly disclose the current Amount Paid display bug.
- [ ] Demonstrate a provider-test payment and receipt only after gateway, settlement and UI steps are implemented and verified. Otherwise finish this portion at the clearly unpaid invoice; never imply collection occurred.
- [ ] Verify an existing notification reaches a synthetic/test recipient, and distinguish queued/admitted from delivered. Verify pause/resume does not duplicate an invoice or payment.
- [ ] Rehearse the demonstrated flow on the intended desktop and mobile widths, check permissions/errors, and retain the tested commit/results and known limitations for the presentation.

After each phase, send a concise update containing its delivered behavior, commits, verification evidence, and pending risks. The latest user instruction authorizes native execution of Phases 3 and 4 in sequence: verify Phase 3 before starting Phase 4, then stop for the user’s joint smoke test. Do not push, open a PR or deploy to production. All six phases remain on this branch when the user resumes the work.

- [x] Phase 1 implementation: Tasks 1–7; automated verification and guided setup/lifecycle check performed. Three known UI findings are assigned to Phase 4 at the user's request; payment collection through the UI remains unverified. Phase 2 is now complete locally.
- [x] Phase 2: Tasks 8–11; local merchant, checkout, callback and refund verification complete. Complete locally; the user has now authorized Phase 3, then Phase 4. No real-provider canary or production deployment claimed.
- [ ] Phase 3: Tasks 12–15; evidence pending implementation.
- [ ] Phase 4: Tasks 16–19; evidence pending implementation.
- [ ] Phase 5: Tasks 20–22; evidence pending implementation.
- [ ] Phase 6: Tasks 23–25; evidence pending implementation.

At execution, append exact test totals, commands, commit SHAs, performance parameters/results, review decisions, and unavailable external checks here. Do not check off a phase on compilation alone.

## Plan self-review

Before handoff, verify specification coverage, exact file existence/ownership, type/signature consistency, all five Review Focus regressions, legacy/manual timing semantics, every competing money writer, safe fixture/cleanup boundaries, and proportionality. This is a local plan review, not another subagent dispatch or an assertion that the implementation passes.

Self-review completed 2026-11-11:

- Scope/compatibility/data-preservation requirements map to Tasks 1, 5, 6, 18, and 24; calculations and all money writers to Tasks 2–5 and 11; merchant/callback/uncertainty to Tasks 8–11; lifecycle/collection/notification guarantees to Tasks 12–15; mobile business/customer/PDF experiences to Tasks 16–19; bounded issue work/docs to Tasks 7 and 20–22; CI/recovery/review to Tasks 23–25.
- All five Review Focus cases have explicit named regressions in their owning tasks. Additional coverage includes partially projected reads, old in-flight writers, expiry of a link during payment, spent-credit refunds, legacy omitted timing/currency, and month-end/trial catch-up.
- Corrected repository-specific paths: docs navigation is `ui/src/config/docsNav.tsx`; portal token model is `internal/domain/bizbilling/connector.go`; channel connection routing is in services, not a nonexistent orchestrator channel manager. Shared types/notification intent now precede consumers; no redundant financial Execute/Apply service cycle or UI transport import cycle.
- Clarified dry-run reporting versus explicit preparation, default-false collection configuration, provider mandates continuing after local admission stops, frozen event payloads during recovery, and digest lookup versus encrypted/rendered delivery-link copies. Existing API/reference tests must remain current throughout all phases.
- Plan is 25 independently testable tasks in six phases, with focused assertions, red/green commands, scoped commits, and one final independent branch review under native execution. It contains decisions and acceptance checks, not full implementation bodies.

Initial review status (historical): the detailed plan was approved before native implementation. Consult the current-state section for execution progress; production actions remain unauthorized.

Execution 2026-11-11: Task 1 compatibility/gate foundation verified: Node fixture safety 4/4; focused Go domain/config/service/handler/fixture/CLI tests pass; real ES/Redis legacy billing E2E 9/9, zero skips (101.855s). Owned fixture containers and volume removed. Task 2 independent math implementation is in progress; no v2 app activation or production change.

Task 2 evidence: checked arithmetic/tax/calendar tests RED→GREEN; legacy zero-rate default retained. Five-second independent big-integer-oracle fuzz run passed 164,884 executions. No historical periods/totals or public DTOs changed; strict overrides are integrated into protected writers in Task 5 before advertising them.
