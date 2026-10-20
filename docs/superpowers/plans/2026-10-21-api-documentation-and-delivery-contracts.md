# API Documentation and Delivery Contracts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every customer-facing API guide and integration example usable against the existing API, repairing the documented OTP, file and rich-message defects without breaking valid integrations.

**Architecture:** Keep existing routes, providers, notification queues, credit accounting and UI documentation renderer. Add scoped OTP context and atomic request transitions, repair existing delivery adapters, then align curated guides and generated Swagger/OpenAPI views with executable contract fixtures. Work in four ordered phases with local commits and phase verification.

**Tech Stack:** Go 1.26.1, Fiber v2, Redis v8 client, Elasticsearch, existing provider adapters, React/TypeScript, Playwright, swag v1.16.6 and YAML v3. Add pinned kin-openapi v0.151.0 only for specification conversion/validation in Phase 4.

**Spec:** `docs/superpowers/specs/2026-10-21-api-documentation-and-delivery-contracts-design.md` (written design approved by the user on 2026-10-21).

## Global Constraints

- One branch: `codex/whatsapp-sms-reliability`; use the existing checkout as explicitly requested. Starting product baseline is `2c0093b`. Preserve unrelated edits in `cmd/frn/admin.go`, `cmd/frn/config.go`, `docker-compose.yml` and untracked user files.
- Preserve existing endpoint paths, channel names, request field names and response envelopes. Existing API-key authentication through `X-API-Key` and `Authorization: Bearer <app-key>` remains supported. Dashboard account JWTs remain distinct from application API keys.
- An application API key determines application scope. Do not introduce a new mandatory application-ID header. Explain the existing cases that require an app ID in a URL or body, including template creation. Preserve current environment-key selection.
- Keep legacy user identifiers readable. Recommend `external_id` for a customer's identifier, and explain the returned internal `user_id`; do not silently reinterpret or migrate existing IDs.
- Preserve normal SSE token issuance, legacy authenticated connections, named events and notification payloads. Private dashboard events keep their existing authenticated-principal rules. The integration sample must use `addEventListener('notification', ...)`.
- Notification creation accepts the existing flat request structure. Do not replace it with nested `content`, or rename `template_id` to `template`. Explain direct-content, template, bulk, batch and broadcast variants separately.
- Preserve working Meta payloads, Twilio `content_sid`, provider connections, generic webhook envelopes and signing headers. Correct vendor-facing serialization without changing customers' stored authoring models.
- Cross-app OTP access and wrong-code success are defects, not compatibility contracts. Reject them while retaining legitimate same-code verification retries and current success response shapes.
- No historical balances, tariffs, customer credentials, stored templates or production data are rewritten by these phases. Existing unrelated working changes are excluded from phase commits.
- No push, deployment, production mutation, real `.env` changes, credential rotation or paid/live sends. External HTTP calls in tests go only to test servers. New dependencies are pinned, and no package publication is implied by a local SDK build.
- Each behavior repair follows a failing regression, the smallest implementation, passing targeted checks and a scoped commit. Documentation prose alone needs no mirror tests; example contracts and executable snippets do. Never stage the entire checkout.

## Review Focus

1. A legacy OTP record without an environment remains usable within its original app and TTL, while new records reject another environment: Tasks 1–2.
2. A queue accepts an OTP and loses its acknowledgement while verify/resend race: no duplicate recovery enqueue, stale verification reset or acceptance of an uncommitted code: Task 3.
3. A signed file URL contains a deployment path prefix/reserved app-ID characters, and a file exceeds 16 MiB while ordinary JSON exceeds 16 MiB: Task 5.
4. A docs page changes while a previous load is slow, or the remote specification returns HTML/invalid JSON/cyclic references: Task 14.
5. A stored WhatsApp list/CTA/quick reply crosses an open/closed service window or uses a historical approved binding: Tasks 9–10.

## File and Interface Map

- `internal/domain/otp/scope.go`, `state.go`: additive scope, record revision and delivery-operation contracts; keep existing `otp.Service` and `otp.Repository` method sets intact.
- `internal/infrastructure/repository/otp_api_repository.go`, `otp_api_atomic.go`: existing record access plus CAS and atomic budget operations. Do not change the separate account-registration OTP repository.
- `internal/usecases/services/otp_service.go`, `otp_state.go`, `otp_delivery.go`: scope checks, atomic verification and fenced send/resend orchestration.
- `internal/domain/notification/dispatch.go`, `internal/usecases/notification_service.go`, `cmd/worker/processor_otp.go`: internal dispatch identity/outcome and worker authorization; no new customer request fields.
- `internal/infrastructure/filestore/download_url.go`, `internal/interfaces/http/middleware/body_limit.go`: shared URL assembly and transport/admission limits.
- `ui/src/docs/`, `ui/src/config/docsNav.tsx`, `internal/interfaces/http/handlers/application_code_samples.go`: curated guides, discoverability and generated language snippets.
- `examples/docs/api-cases.json`, `tests/contracts/`, `examples/docs/typescript/`, `examples/docs/go/`: safe executable examples and contract checks, shared by later reference generation.
- `internal/usecases/services/whatsapp_rich_*`, `internal/infrastructure/providers/`, existing channel UI components: provider repairs and honest capability explanations.
- `cmd/docsgen/`, `internal/documentation/`, `docs/`, `ui/public/swagger.json`: one generation command and compatible Swagger 2/OpenAPI 3 outputs.
- `e2e/docs.config.ts`, `e2e/docs/`: isolated public-docs tests with no production global setup or stored customer login.

## Shared Execution and Verification Rules

Run commands from the repository root unless a task says otherwise. Commands below are planned checks, not claims that they have already passed. Use PowerShell on this host; shell commands on separate lines are separate steps. Keep `GOMAXPROCS=4` and `go test -p 2` locally. Do not run a bare `go test ./...` because dependency trees can exist under UI folders.

After plan approval, provision dependencies from their existing lockfiles with `npm ci`, `npm --prefix ui ci`, `npm --prefix sdk/js ci` and `npm --prefix sdk/react ci`; do not install or modify dependencies during planning. Task 13 alone adds its pinned converter with `go get github.com/getkin/kin-openapi@v0.151.0`. Cache the pinned generator/converter before offline verification. No Go/Node toolchain floor change is required.

For Redis integration tests, create an owned disposable `redis:7-alpine` container bound only to `127.0.0.1:16390`; expose it through `FRN_OTP_REDIS_ADDR=127.0.0.1:16390`. The new tests refuse another address and use unique `frn:otp:api:test:<uuid>:` prefixes; no `FLUSHALL`. Assert readiness before running them and remove only the verified owned container. Existing billing/channel fixture tests keep their own documented opt-in variables and ports; do not repoint them at a production store.

For every task: inspect the scoped diff, run `git diff --check`, stage only its enumerated files, inspect `git diff --cached --name-only`, and commit with the stated message. Do not mark a phase complete when a required test was skipped. Record command/result, skipped checks and commit in the phase evidence section at the end of this plan.

---

## Phase 1 — Functional Repairs

### Task 1: Scope OTP operations and correct idempotent verification

**Files:** Create `internal/domain/otp/scope.go`, `internal/usecases/services/otp_scope_test.go`; modify `internal/domain/otp/models.go`, `internal/usecases/services/otp_service.go`, `internal/interfaces/http/handlers/otp_handler.go`, `internal/interfaces/http/handlers/otp_handler_test.go`.

**Interfaces:** Add `type Scope struct { AppID, EnvironmentID string }`, `WithScope(context.Context, Scope) context.Context`, `ScopeFromContext(context.Context) (Scope, bool)`. Add optional `EnvironmentID string` to `otp.Request` and `otp.SendInput`. Keep `Send`, `Verify` and `Resend` signatures unchanged. HTTP handlers always attach non-empty authenticated scope; trusted pre-existing internal service callers may omit it. `Send` uses the existing `channelconnection.WithEnvironment` context when creating delivery.

- [x] **1. Write regressions:** `TestOTPScopedOperations` asserts `require.ErrorIs(t, err, otp.ErrNotFound)` for another app/environment and unchanged attempt count, delivery calls and record bytes; `TestOTPVerifiedRetryChecksCode` asserts same code returns prior timestamp with no extra attempt, different code returns `otp.ErrInvalidCode`. Add handler tests asserting authenticated scope reaches all three service calls, including environment-scoped send.
- [x] **2. Run RED:** `go test -p 2 ./internal/domain/otp ./internal/usecases/services ./internal/interfaces/http/handlers -run 'TestOTP(Scoped|VerifiedRetry|HandlerScope)' -count=1`; expect assertion failures against current behavior.
- [x] **3. Implement scope propagation/checks before mutation and constant-time comparison before prior-success return.** New records store effective environment (`default` when scoped default); legacy missing-environment records accept only the matching app and retain their exact expiry. Keep HTTP bodies, success envelopes and unscoped internal call compilation unchanged.
- [x] **4. Run GREEN:** rerun the command and existing OTP tests with `-run 'OTP'`. A legacy missing-environment fixture must pass within its app, fail in another app and preserve `ExpiresAt`; an expired one must remain rejected.
- [x] **5. Commit:** `fix: scope OTP requests and validate repeated verification`.

### Task 2: Add atomic OTP storage and recipient budgets

**Files:** Create `internal/domain/otp/state.go`, `internal/infrastructure/repository/otp_api_atomic.go`, `internal/infrastructure/repository/otp_api_atomic_test.go`; modify `internal/domain/otp/models.go`, `internal/infrastructure/repository/otp_api_repository.go`, `internal/usecases/services/otp_service_test.go`.

**Interfaces:** Add `Revision uint64` to `otp.Request`, treating absent revision as zero. Add `AtomicRepository interface { Repository; CompareAndSwap(context.Context, *Request, *Request) error }` and sentinels `ErrConflict`, `ErrStateUnavailable`. CAS checks request/app/environment identity, expected revision and original expiry; each successful write increments revision once. Keep the exported legacy repository method set and constructor return type unchanged; the Redis implementation also implements `AtomicRepository`.

- [x] **1. Write `TestOTPAtomicStore` with real disposable Redis:** two writers from revision zero produce `require.Equal(t, 1, successes)`; stale writes return `ErrConflict`; deleted/expired keys are never recreated; `ExpiresAt` remains identical and Redis TTL never increases. Preserve unknown JSON fields during additive transitions. Add `TestOTPRecipientBudgetAtomicExpiry`: concurrent calls return distinct counts and every nonempty counter has positive TTL.
- [x] **2. Run RED:** set the fixture variable as described above, then `go test -p 2 ./internal/infrastructure/repository -run 'TestOTP(AtomicStore|RecipientBudget)' -count=1`; initial failure must be missing CAS behavior, not an unavailable Redis connection.
- [x] **3. Implement bounded WATCH/MULTI CAS (maximum eight conflicts) and a Lua increment/expiry operation for the existing 3,600-second budget.** Compute TTL from the original absolute expiry, never the current `Update` fallback of 60 seconds. Handle old zero-revision records without extending them. Extend the existing mutex-protected fake with the same CAS semantics and deep copies.
- [x] **4. Run GREEN:** rerun repository tests with `-count=5`; prove receipt-loss retries cannot increment revision twice from one expected version. Existing repository readers and internal registration OTP tests must still pass.
- [x] **5. Commit:** `fix: add atomic OTP state and expiring recipient budgets`.

### Task 3: Fence OTP dispatch, resend and worker recovery

**Files:** Create `internal/usecases/services/otp_state.go`, `internal/usecases/services/otp_delivery.go`, `internal/usecases/services/otp_state_test.go`, `internal/usecases/services/otp_dispatch_test.go`, `internal/domain/notification/dispatch.go`, `internal/usecases/notification_dispatch_test.go`, `cmd/worker/processor_otp.go`, `cmd/worker/processor_otp_test.go`; modify `internal/domain/otp/state.go`, `internal/domain/otp/models.go`, `internal/usecases/services/otp_service.go`, `internal/usecases/notification_service.go`, `internal/container/container.go`, `cmd/worker/processor.go`, `cmd/worker/main.go` and existing OTP test doubles as required by additive capabilities.

**Interfaces:** `otp.Request` adds `ResolvedUserID`, `TemplateBody`, `TemplateData`, and `Delivery *DeliveryOperation`. Define `DeliveryOperation` with `ID`, `NotificationID`, `State` (`pending`, `accepted`, `uncertain`), replacement `CodeHash`/`Salt`, and `StartedAt`. Define `otp.DeliveryAuthorizer.AuthorizeDelivery(ctx context.Context, scope Scope, requestID, operationID, notificationID string) error`; `OTPService` implements it. Add `notification.WithDispatchIdentity(ctx context.Context, id string) context.Context`, `DispatchIdentityFromContext(context.Context) (string, bool)` and `DispatchError{NotificationID string; MayBeEnqueued bool; Cause error}` with `Error`/`Unwrap`; only trusted service code supplies identity. Operation metadata uses `otp_request_id`, `otp_operation_id` and existing `otp_expires_at`, bound to `NotificationID` and authenticated scope. Add worker `SetOTPDeliveryAuthorizer(otp.DeliveryAuthorizer)`.

- [x] **1. Write deterministic barriers/fault tests:** `TestOTPDispatchStateBeforeEnqueue`, `TestOTPResendConcurrentClaim`, `TestOTPVerifyCannotBeResetByResend`, `TestOTPEnqueueAckLoss`, `TestOTPResendPreservesRecipientAndTemplate`. Assert no dispatch on failed initial persistence, one dispatch for concurrent resend, exact original expiry, no committed new hash before acceptance, and `require.Equal(t, 1, enqueueCalls)` after recovery. Inject a queue that enqueues then returns an error, and a Redis CAS that commits then loses acknowledgement. Include a recipient-preference suppression returning `(nil, nil)` from notification send: OTP must neither panic nor report successful enqueue; abort only the matching claim and return a client admission failure.
- [x] **2. Run RED:** `go test -p 2 ./internal/usecases/... ./cmd/worker -run 'TestOTP(Dispatch|ResendConcurrent|VerifyCannot|EnqueueAck|ResendPreserves|Worker)' -count=1`; assert failures at the state/side-effect boundary.
- [x] **3. Implement service transitions using Task 2 CAS.** Persist initial authority and pending operation before enqueue; allocate one deterministic notification UUID per request/operation. Resend reserves its operation before generating/delivering the new code, preserves original recipient/config, and applies the same five-per-hour budget after ownership/cooldown checks. Budget-store failure fails before delivery. Add `otp.ErrOperationPending = errors.New("otp: delivery operation pending")` in `internal/domain/otp/state.go`; while pending, verify/resend return this retryable error (HTTP 409). Exhausted CAS/store availability maps to HTTP 503 without changing existing outcomes.
- [x] **4. Implement outcome fencing in notification send and worker.** Definitive pre-enqueue failure aborts only the matching pending operation and preserves the previous committed code; failed initial sends cannot verify. Unknown enqueue outcomes keep the operation, return retryable error and never enqueue again automatically. For trusted OTP dispatch, do not mark an uncertain enqueue definitively failed. Actual worker dequeue proves queue acceptance: matching authorizer CAS commits the pending hash/attempt reset before any provider call; accepted retries require the same operation/notification identity. Stale, expired or unmatched operations cannot send or alter the wallet. Legacy notifications without operation metadata retain the existing deadline/retry path. Wire the authorizer in worker bootstrap; a new-format operation without it fails closed.
- [x] **5. Run GREEN:** rerun the targeted suites, real-Redis `TestOTPAtomicStore`, and worker expiry/billing regressions. Assert duplicate queue deliveries do not bypass the existing delivery claim, a verified record stays verified, old code fails after successful replacement, and delivery never occurs after `otp_expires_at`. Keep uncertain operations bounded by original expiry; do not add blind recovery retries or claim exactly-once provider effects.
- [x] **6. Commit:** `fix: fence OTP send and resend across queue failures`.

### Task 4: Remove OTP template leakage and align error handling

**Files:** Create `internal/usecases/services/otp_content_test.go`, `internal/usecases/notification_logging_test.go`, `internal/domain/notification/admission.go`, `sdk/js/tests/otp-errors.test.cjs`, `sdk/js/scripts/test.mjs`; modify `internal/usecases/services/otp_service.go`, `internal/usecases/notification_service.go`, `internal/interfaces/http/handlers/notification_handler.go`, `internal/interfaces/http/handlers/otp_handler.go`, `internal/usecases/services/channel_dispatch_service.go`, `sdk/js/src/otp.ts`, `sdk/js/package.json`.

**Interfaces:** Keep `dispatchNotification` internal and dispatch existing direct-content `notification.SendRequest{Title, Body, Data}` without a permanent per-code template. Add a typed `notification.AdmissionError` with `Error`/`Unwrap` for expected channel rejections; the OTP handler maps it to HTTP 400. Keep `templateCreator`/constructor call compatibility for existing callers. The SDK retains `OTPError` and exported code union; map only recognized primitive strings, leaving unknown/middleware errors as `FreeRangeNotifyError`.

- [x] **1. Write `TestOTPContentUsesDirectSendAndSafeLogs`:** the template spy has zero creates, the provider receives the generated code/custom body, the response contains no raw code, and Zap observer output does not contain that code or full body/data on success/failure. Include malformed raw HTTP body logging and typed WhatsApp payload customization. `TestOTPAdmissionErrorIsClientError` asserts 400 without exposing provider credentials. SDK Node tests mock `fetch` for valid flat OTP errors, nested 401 middleware errors, unknown strings and invalid JSON.
- [x] **2. Run RED:** `go test -p 2 ./internal/usecases/... ./internal/interfaces/http/handlers -run 'TestOTP(Content|Admission)|TestNotificationSensitiveLogging' -count=1`; add SDK `test` script `node scripts/test.mjs`, then run `npm --prefix sdk/js test`. The script compiles SDK source to a temporary CommonJS directory using its pinned TypeScript compiler, writes a temporary `type: commonjs` package marker and runs `tests/*.test.cjs` against it via `FRN_SDK_TEST_OUTPUT`. This avoids changing the shipped ESM format or relying on Node's handling of extensionless ESM imports; always remove only the owned temporary directory.
- [x] **3. Implement direct-content dispatch, retain generated code precedence/customization, remove full content/body/data logging from affected paths and introduce typed admission errors at their source.** No API raw-code field, new encryption promise or historical data deletion. Preserve existing flat verify status/error values and opaque infrastructure failures.
- [x] **4. Run GREEN:** rerun the targeted checks plus `go test -p 2 ./sdk/go/... -run OTP -count=1`; assert successful SMS/email/approved WhatsApp OTP requests keep their current response shape and expiry metadata.
- [x] **5. Commit:** `fix: keep OTP content out of logs and preserve error contracts`.

### Task 5: Repair signed file URLs and upload boundaries

**Files:** Create `internal/infrastructure/filestore/download_url.go`, `internal/infrastructure/filestore/download_url_test.go`, `internal/interfaces/http/middleware/body_limit.go`, `internal/interfaces/http/middleware/body_limit_test.go`, `internal/interfaces/http/handlers/file_contract_test.go`; modify `internal/interfaces/http/handlers/file_handler.go`, `cmd/worker/main.go`, `cmd/server/main.go`.

**Interfaces:** Add `BuildSignedDownloadURL(baseURL, appID, fileID string, exp int64, sig string) string`, using escaped path segments/query parameters and a normalized deployment prefix. Validate configured absolute public URL at startup; empty base produces a relative handler link. Add `UploadTransportLimit(maxFileBytes int64) int` and `NonUploadBodyLimit(limit int) fiber.Handler`. Effective file cap is the existing configured/default file-service maximum (hard cap 50 MiB), transport overhead allowance is 1 MiB, ordinary JSON remains capped at 16 MiB. The global transport ceiling is `max(16 MiB, effectiveFileCap + 1 MiB)` so a small configured file cap cannot reduce the existing JSON ceiling.

- [x] **1. Write `TestFileGeneratedURLRoundTrip` using actual handler/signer and a temporary file store:** a prefixed base URL and app ID with reserved query characters round-trip; tampering/missing app/expiry/signature fail. `TestUploadBodyBoundaries` posts a valid 17 MiB multipart file successfully under a 50 MiB service cap, rejects a file over the configured cap and rejects ordinary JSON over 16 MiB. With a 1 MiB file cap, assert transport still admits existing 15 MiB JSON while file admission still rejects oversized uploads.
- [x] **2. Run RED:** `go test -p 2 ./internal/infrastructure/filestore ./internal/interfaces/http/middleware ./internal/interfaces/http/handlers -run 'TestFileGenerated|TestUploadBody' -count=1`.
- [x] **3. Use the shared builder in both handler and worker.** Set Fiber's transport ceiling from the effective file cap plus allowance; exempt only authenticated `POST /v1/files` (and its accepted trailing-slash form) with multipart content from the ordinary request-size admission guard. Retain existing media endpoint limits, file-byte validation, ownership, signatures and expiry.
- [x] **4. Run GREEN:** rerun targeted tests and existing file-service/signed-URL tests; verify no tenant retention or expiry-deletion behavior was invented. Append actual Phase 1 checks/commits below; mark the phase complete only with all required checks executed.
- [x] **5. Commit:** `fix: align file links and upload request limits`.

## Phase 2 — Onboarding and CRUD Documentation

### Task 6: Build one executable onboarding path

**Files:** Create `ui/src/docs/authentication.md`, `ui/src/docs/users.md`, `examples/docs/api-cases.json`, `tests/contracts/examples_test.go`, `tests/contracts/examples.go`, `tests/contracts/router_test.go`; modify `ui/src/docs/getting-started.md`, `ui/src/config/docsNav.tsx`.

**Interfaces:** Define fixture `APICase{ID, Method, Path, Auth, Guide string; Headers, Capture map[string]string; Request json.RawMessage; Status int; Response json.RawMessage}` and `loadCases(t *testing.T) []APICase`, `newExampleRouter(t *testing.T) *fiber.App` as test helpers. Run scenario cases in order: `Capture` maps a variable name to its response path (for example `USER_ID` to `data.user_id`); validate returned ID shape, then substitute `${USER_ID}` in subsequent paths/bodies and expected responses. This exercises generated real identifiers without hard-coding random UUIDs. Use `https://notify.example.test/v1`, `YOUR_API_KEY` and safe example recipients. Mark request/response/curl blocks with `<!-- api-example:<id>:request -->`, `:response` and `:curl`; compare parsed JSON and the curl method/path/auth/body to the same case without comparing prose.

- [x] **1. Add cases `create-app`, `create-user`, `get-user`, `update-user`, `delete-user`, `create-template`, `send-template`, `get-notification`.** `TestDocumentationOnboardingContracts` asserts the real handler statuses/envelopes, `app_name` field, `data.user_id`, template response `id`, notification response `notification_id`, and exact copied JSON. Use fake repositories/queue, actual service validation and real handlers. Add both API-key header formats to auth integration fixtures; keep JWT-only app creation separate.
- [x] **2. Run RED:** `go test -p 2 ./tests/contracts -run 'TestDocumentationOnboarding' -count=1`; expect current examples to disagree, not a missing production service.
- [x] **3. Rewrite Getting Started around the actual UI labels and owner-authorized key reveal; add Authentication and Users navigation/guides.** Explain one-time/revealed keys, dashboard references, backend secrets, app/environment scope, `external_id` versus existing internal IDs, full user CRUD, pagination, preferences, devices and bulk import. Keep all existing slugs and legacy identifiers. Document actual CSV limits/partial failures from the importer; do not conflate subscriber ID fields with automatic migration.
- [x] **4. Run GREEN:** contract tests pass; manually review the guide as a new customer and ensure each output supplies the next request's identifier. The examples show acceptance versus delivery without promising instant external delivery.
- [x] **5. Commit:** `docs: add verified app setup authentication and user guides`.

### Task 7: Document templates, notification lifecycle and adjacent guides

**Files:** Create `ui/src/docs/notifications.md`; modify `ui/src/docs/templates.md`, `ui/src/docs/topics.md`, `ui/src/docs/workflows.md`, `ui/src/docs/environments.md`, `ui/src/docs/pricing.md`, `ui/src/docs/troubleshooting.md`, `ui/src/config/docsNav.tsx`, `examples/docs/api-cases.json`, `tests/contracts/examples_test.go` and `tests/contracts/router_test.go`.

**Interfaces:** Consume `APICase` and `newExampleRouter` from Task 6. Add marked cases for render, explicit version creation/history/rollback, topic creation/subscription, workflow create/trigger, notification bulk/batch/broadcast, scheduling, read/archive/snooze, status/cancel/retry and environment create/promotion. A case names its real credential type and actual response envelope; a documented feature-gated operation names its prerequisite.

- [x] **1. Add `TestDocumentationLifecycleContracts`:** verify actual method/path/body/response for the added cases, `key` and `trigger_id`, current version routes, and in-place template update followed by explicit snapshot. Assert a delivered notification is never described/tested as a mutable provider message. Test only idempotency the endpoint actually supports.
- [x] **2. Run RED:** `go test -p 2 ./tests/contracts -run 'TestDocumentationLifecycle' -count=1`; current JWT workflow and version examples must disagree with runtime contracts.
- [x] **3. Write the missing lifecycle guide and correct adjacent guides.** Provide prerequisites, request, response and next action; describe supported lifecycle transitions rather than invented generic notification CRUD. Correct application limits to requests/hour, distinguish queue/provider limits and link active pricing/rate cards rather than repeating a universal fixed tariff. Explain available, reserved and remaining credits and current recharge semantics without changing billing code/data.
- [x] **4. Run GREEN:** all contract cases pass and all existing documentation links remain resolvable. Review current import/broadcast/retry behavior before making performance or exactly-once claims.
- [x] **5. Commit:** `docs: align template notification and workflow lifecycle examples`.

### Task 8: Repair generated snippets and SDK examples

**Files:** Create `internal/interfaces/http/handlers/application_code_samples.go`, `internal/interfaces/http/handlers/application_code_samples_test.go`, `examples/docs/go/main.go`, `examples/docs/typescript/client.ts`, `examples/docs/typescript/react.tsx`, `examples/docs/typescript/tsconfig.json`; modify `internal/interfaces/http/handlers/application_handler.go`, `internal/container/container.go`, `ui/src/components/apps/AppCodeSamples.tsx`, `ui/src/docs/sdk.md`, `ui/src/docs/sse.md`, `ui/src/docs/in-app.md`, `ui/src/docs/monkeys-integration.md`.

**Interfaces:** Move only `GetCodeSamples` and its helpers into the focused handler file; preserve its existing route, nested JSON and language keys. Add `SetCodeSampleBaseURL(raw string) error` on `ApplicationHandler`, called with trusted `cfg.Server.PublicURL`; normalize one `/v1`, reject unsupported schemes/query/fragment and use `YOUR_BASE_URL/v1` when unset. Quote inserted values with each language's string encoder, never request/forwarded host headers. TypeScript fixtures map `@freerangenotify/sdk` to repository `sdk/js/src/index.ts` and React to `sdk/react/src/index.tsx`.

- [x] **1. Write `TestIntegrationCodeSamples`:** request each language/channel, assert placeholder key remains, configured base/prefix round-trips, header-injection host is ignored, and browser SSE registers the named `notification` event. `TestIntegrationGoSamplesCompile` compiles generated Go in `t.TempDir()` with no execution/network. Compile documentation Go/TypeScript/React fixtures against actual exports.
- [x] **2. Run RED:** `go test -p 2 ./internal/interfaces/http/handlers -run 'TestIntegration(Code|Go)' -count=1`; `node ui/node_modules/typescript/bin/tsc --noEmit -p examples/docs/typescript/tsconfig.json`; `go test ./examples/docs/go`.
- [x] **3. Fix snippets, language escaping, unused Go imports, error/status checks and labels; remove the ineffective sample Show Token toggle.** Update SDK examples to `@freerangenotify/sdk`, `new FreeRangeNotify(apiKey, options)`, standard-send snake_case versus quick-send camelCase, Go `New`/context methods and actual React props/hooks. Preserve exported legacy APIs. Explain that the existing React provider requires an application key and is not a token-only public embedding API; recommend backend-issued SSE tokens/BFF requests for public browsers instead of embedding a master key or inventing unsupported props.
- [x] **4. Run GREEN:** all three commands pass; review the self-hosted and hosted examples, owner-only key reveal and links. Record Phase 2 results below.
- [x] **5. Commit:** `fix: align integration snippets and SDK documentation`.

## Phase 3 — Channels and Rich Content

### Task 9: Complete stored Meta list, CTA and quick-reply delivery

**Files:** Create `internal/usecases/services/whatsapp_rich_delivery_contract_test.go`; modify `internal/usecases/services/whatsapp_rich_template_service.go`, `internal/usecases/services/channel_dispatch_service.go`, `internal/infrastructure/providers/meta_whatsapp_provider.go`, `internal/infrastructure/providers/whatsapp_payload_kind.go`, existing rich-template/provider tests, and `ui/src/pages/whatsapp/WhatsAppRichTemplates.tsx` only for corresponding capability/error display.

**Interfaces:** Preserve `ResolveSendPayload(ctx context.Context, appID, notificationID string, payload whatsapp.SendPayload) (map[string]interface{}, error)` and existing public authoring models. Keep its resolved-map return shape; the admission wrapper translates resolved `kind: list` to the working `data.whatsapp_interactive` input and approved CTA/quick reply to `data.whatsapp_template`/components usable by the existing Meta renderer. Keep `providers.WhatsAppPayloadKind` output `text` for window-governed interactive lists and `template` for approved templates.

- [x] **1. Add `TestStoredWhatsAppRichDeliveryContract` over create/load/resolve/admit/render:** a stored list works in an open window without a fictional Meta binding, fails closed outside it; approved CTA/quick reply retain template identity/components and work outside the window; draft/rejected/unbound and cross-app/environment templates cannot dispatch. Compare the historical raw payload path and assert identical route/quote identity for equivalent content.
- [x] **2. Run RED:** `go test -p 2 ./internal/usecases/services ./internal/infrastructure/providers -run 'TestStoredWhatsAppRich|TestWhatsAppPayload' -count=1`.
- [x] **3. Implement translation into the working payload paths before classification; retain approval, connection ownership, destination integrity, provider selection and billing checks.** Add explicit rendering where required rather than returning a fallback kind that silently loses buttons. Update the UI capability/error state for unsupported combinations.
- [x] **4. Run GREEN:** targeted and existing raw/rich/CSW/quote tests pass. Check configured-version payload details against official Meta docs before updating strict mock fixtures; a mock pass is not a live approval or send.
- [x] **5. Commit:** `fix: complete stored Meta interactive and button delivery`.

### Task 10: Correct Twilio carousel and copy-code authoring

**Files:** Create `internal/usecases/services/whatsapp_rich_twilio_payload_test.go`; modify `internal/usecases/services/whatsapp_rich_twilio_payload.go`, `internal/usecases/services/whatsapp_rich_template_service_test.go` and relevant draft tests.

**Interfaces:** Keep `twilioContentPayload(*whatsapp.RichTemplate) map[string]interface{}`, `twilioCarouselType(*whatsapp.RichTemplate) map[string]interface{}` and FRN's stored model. New authoring emits carousel `body`, per-card `body`, string `media`, supported ordered actions and provider limits. Coupon buttons use `COPY_CODE` on the supported call-to-action route instead of silent quick-reply degradation.

- [x] **1. Add `TestTwilioRichAuthoringSchema`:** strict mock HTTP server rejects array media/missing card body/invalid action shapes and asserts actual serialized HTTP JSON, not only the helper map. Assert historical ContentSid sends and authoring-model JSON remain unchanged, and edits do not auto-resubmit existing bindings.
- [x] **2. Run RED:** `go test -p 2 ./internal/usecases/services -run 'TestTwilioRichAuthoringSchema' -count=1`.
- [x] **3. Correct serialization and validation against the official Twilio carousel/call-to-action contracts cited in the design.** Keep runtime values/variables and customer account routing; introduce no automatic vendor mutation/migration.
- [x] **4. Run GREEN:** the strict fixture and existing Twilio content/draft/snapshot tests pass. Record tested provider contract version/date.
- [x] **5. Commit:** `fix: correct Twilio rich content authoring payloads`.

### Task 11: Preserve inline images and verify email attachment adapters

**Files:** Modify `internal/infrastructure/providers/resend_provider.go`, `internal/infrastructure/providers/email_attachments_test.go`; create `internal/infrastructure/providers/email_attachment_contract_test.go`.

**Interfaces:** Keep `Provider.Send`; Resend JSON attachment objects add `content_id` only for inline attachments with a valid content ID. Ordinary attachments remain unchanged. Consume the existing attachment resolver and provider fixture transports.

- [x] **1. Replace `TestResendProvider_Send_InlineDegradesToRegular` with `TestResendProvider_Send_InlineEmitsContentID`:** captured JSON has `content_id: logo-image` paired with HTML `cid:logo-image`; ordinary file attachments omit it. Add byte/source/error fixtures for SMTP, SES, SendGrid, Mailgun, Postmark and Resend using URL/base64/file-ID sources and multiple attachments; assert each advertised CID behavior at its actual wire boundary.
- [x] **2. Run RED:** `go test -p 2 ./internal/infrastructure/providers -run 'Test.*(Inline|AttachmentContract)' -count=1`.
- [x] **3. Forward Resend content ID and make only reproducible adapter corrections found by those fixtures.** Keep provider-specific size limits and close resolver streams. Unsupported combinations must have accurate capability descriptions rather than a fabricated success.
- [x] **4. Run GREEN:** targeted and existing attachment tests pass; no remote provider or paid send is used.
- [x] **5. Commit:** `fix: preserve inline content IDs in email attachments`.

### Task 12: Publish accurate channel, file, webhook and poll recipes

**Files:** Create `ui/src/docs/webhooks.md`, `examples/docs/typescript/webhook-receiver.ts`, `tests/contracts/rich_examples_test.go`; modify `ui/src/docs/channels.md`, `ui/src/docs/file-attachments.md`, `ui/src/docs/otp.md`, `ui/src/docs/sse.md`, `ui/src/docs/in-app.md`, `ui/src/docs/sdk.md`, `ui/src/config/docsNav.tsx`, `examples/docs/api-cases.json`, `internal/infrastructure/providers/custom_provider_test.go`, `internal/infrastructure/providers/render/rich_renderer_test.go`, and corresponding inline help in `ui/src/pages/AppDetail.tsx`, `ui/src/components/AppFiles.tsx`, `ui/src/components/AppNotifications.tsx` and `ui/src/pages/whatsapp/WhatsAppRichTemplates.tsx`.

**Interfaces:** Extend Task 6 cases with top-level attachment arrays, inline CID, OTP email/SMS/approved WhatsApp, generic webhook and registered custom-provider rich payloads. Receiver verifies `X-Webhook-Signature` as hex HMAC-SHA256 of exact raw body bytes; generic envelope remains `id`, nested `content`, `created_at`. `webhook_target` selects registered custom renderers; it does not change generic webhook payloads.

- [ ] **1. Add `TestRichDocumentationContracts`:** exact fixture requests pass the real admission/provider path; missing/expired files produce the documented upload/admission/async outcome; generic attachments remain descriptors; custom Slack/Teams file IDs use Task 5 links. Test tampered webhook bytes/signature and missing optional secret. Assert Slack/Teams poll text has no voting-control promise and Discord fallback occurs only on the recognized rejection; raw WhatsApp/SMS generic poll support is not invented.
- [ ] **2. Run RED:** `go test -p 2 ./tests/contracts ./internal/infrastructure/providers/... -run 'TestRichDocumentation|Test.*Poll|Test.*WebhookSignature' -count=1`; type-check the receiver fixture with Task 8's TypeScript command.
- [ ] **3. Write channel-specific prerequisites, owner setup, request, acceptance response and status check.** Cover custom SMTP, push devices, SMS phones/segments, Meta/Twilio approval/windows, in-app/SSE, Slack/Discord/custom Teams and inbound versus outbound webhooks. Describe URL/base64/file-ID source exclusivity and multiple attachments; WhatsApp uses its actual media/template fields. Explain logical expiry, global retention and async failures accurately. OTP recipes state five/hour budget, fresh-code resend/original expiry, new send after expiry, actual error shapes and hash-record versus delivery-content privacy.
- [ ] **4. Run GREEN:** contract/provider/receiver checks pass; verify existing routes/guides and each UI label/help link. Record Phase 3 results below.
- [ ] **5. Commit:** `docs: publish verified channel OTP file and webhook recipes`.

## Phase 4 — Reference Generation and Regression Protection

### Task 13: Generate compatible references from one source

**Files:** Create `cmd/docsgen/main.go`, `internal/documentation/generate.go`, `internal/documentation/generate_test.go`, `tests/contracts/routes_test.go`, `examples/docs/internal-routes.json`; modify `Makefile`, `go.mod`, `go.sum`, relevant handler/DTO annotations, `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`, `docs/openapi/swagger.yaml`, `docs/openapi/otp.yaml`, `docs/openapi/whatsapp-rich.yaml`, `ui/public/swagger.json`, `cmd/server/main.go` and `internal/interfaces/http/routes/routes.go` for the reference endpoint only.

**Interfaces:** Add `Generate(root string, check bool) error` and CLI `go run ./cmd/docsgen [-check]`. Invoke pinned `go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go ...` in isolated output, validate Swagger 2, copy byte-identical UI fallback, convert with `github.com/getkin/kin-openapi/openapi2conv` v0.151.0 and validate OpenAPI 3. Preserve OpenAPI entry documents by generating filtered OTP/rich views with shared schemas. Publish the generated JSON at `/v1/swagger/doc.json`; retain existing `/swagger/*`, `/openapi/*` and bundled `/swagger.json` use. No request data selects arbitrary files.

- [ ] **1. Write `TestGeneratedSpecifications`:** generated-copy equality, resolvable references, multipart/binary requests, conditional template/direct-content fields, both key-header schemes, JWT/public-link/callback auth and valid converted documents. `TestDocumentedRoutesMatchRouter` registers actual Fiber routes with a synthetic all-features container and compares canonical method/path pairs. Explicit internal-route exclusions include individual paths/methods and reasons; no catch-all omission. Cover enabled/disabled feature prerequisites and references containing recursive schema types.
- [ ] **2. Run RED:** `go test -p 2 ./internal/documentation ./tests/contracts -run 'TestGenerated|TestDocumentedRoutes' -count=1`; current abbreviated UI copy and missing SSE/messaging routes must fail. Acquire only pinned tooling dependencies for implementation; never use an unpinned latest converter.
- [ ] **3. Correct annotations/DTO schema metadata to existing behavior, including optional/conditional send fields and all customer channels.** Generate the full shared documents and filtered compatible views using deterministic ordering and no timestamps; shared source includes descriptions/examples from contract fixtures. Make `make swagger` call the Go CLI; `-check` compares temporary generated output and performs no source writes. Register the JSON reference endpoint outside customer rate/auth middleware so public docs remain usable.
- [ ] **4. Run GREEN:** `go run ./cmd/docsgen`, `go run ./cmd/docsgen -check`, targeted tests, and `git diff --check`. A second generation produces no differences. Validate both old reference entry URLs and all new example/schema references; an OpenAPI 3 file must declare OpenAPI 3 rather than disguise Swagger 2.
- [ ] **5. Commit:** `docs: generate consistent API references and contract coverage`.

### Task 14: Make the UI reference and guide loading reliable

**Files:** Create `ui/src/pages/docs/apiSpec.ts`, `e2e/docs.config.ts`, `e2e/mocked.config.ts`, `e2e/docs/navigation.spec.ts`, `e2e/docs/reference.spec.ts`, `e2e/docs/snippets.spec.ts`; modify `ui/src/pages/docs/ApiReferencePage.tsx`, `ui/src/pages/docs/DocsPage.tsx`, `ui/vite.config.ts` and `package.json`.

**Interfaces:** Add `loadApiSpec(url: string, fallbackURL: string, signal?: AbortSignal): Promise<SwaggerSpec>`, `resolveSchema(spec: SwaggerSpec, ref: string): SwaggerSchema | undefined` and `schemaExample(spec: SwaggerSpec, schema: SwaggerSchema): unknown` in `apiSpec.ts`. Define/export the shared spec/schema types there; recursive expansion is bounded to depth four with a visited-reference set. Use existing `buildApiUrl('/swagger/doc.json')`, honor configured deployment API base and validate `swagger === '2.0'` plus a paths object before accepting a response. Render request/response schema properties, required fields, auth guidance, examples and feature prerequisites; build no authenticated API-execution console.

- [ ] **1. Add Playwright tests with route fixtures:** public guides load logged out, every nav/cross-link resolves, copied samples contain placeholders, remote spec uses configured API base, fallback works for HTTP errors/HTML/invalid JSON, cyclic references stay bounded and delayed old guide loads never replace the new slug. Check light/dark themes and a 390 px viewport. Test the app Integration copy against the corrected named-event snippet with a synthetic owner session.
- [ ] **2. Run RED:** `npx playwright test -c e2e/docs.config.ts`. Configuration must set `globalSetup: undefined`, `storageState: undefined`, no `.env` loading, route-mocked `/v1/**`, isolated output directory and a locally started preview server. Any unexpected external URL fails the test. Do not reuse production global setup. Add `FRN_DOCS_TEST=1` handling to Vite configuration: skip explicit `loadEnv`, set `envDir: false` and use only fixture API-base overrides in this test mode; ordinary builds preserve current behavior.
- [ ] **3. Implement shared spec loading/validation/ref expansion and render schema/example details.** Cancel/ignore stale fetches and Markdown imports on navigation/unmount. Retain all existing public slugs, syntax highlighting and app navigation. Add root `test:docs` script pointing at the isolated config. The separate `e2e/mocked.config.ts` has the same clean setup and explicitly selects only self-contained route-mocked `subscriber-import.spec.ts`, `exotel-connections.spec.ts`, `whatsapp-runtime-config.spec.ts` and `whatsapp-sms-*.spec.ts`; it must not import saved-state/global-setup fixtures.
- [ ] **4. Run GREEN:** `npm --prefix ui run build`; `npx playwright test -c e2e/docs.config.ts`. Use explicit test command configuration for a second non-default deployment API-base build; avoid editing real UI environment files. Verify copy actions and error states in the browser, not only generated HTML strings.
- [ ] **5. Commit:** `fix: keep UI API reference and guide navigation accurate`.

### Task 15: Integrate all phases and record release evidence

**Files:** Create `scripts/verify-docs.mjs`; modify `package.json`, `.github/workflows/ci.yml`, this plan and `documents/production-delivery-recovery-runbook.md` with a new documentation/API-contract section. Reuse earlier tests and fixtures; add targeted missing regressions to their owning files if integration finds defects.

**Interfaces:** `node scripts/verify-docs.mjs` runs `docsgen -check`, contract tests, Go example compile and SDK fixture type-check as child processes from repo root, returns nonzero on any failure and passes an explicit environment allowlist (toolchain/cache/OS/temp variables and designated fixture variables only). Do not inherit customer tokens, provider credentials or `.env` configuration into documentation checks. CI runs that command and isolated docs browser tests without requiring the existing gated customer-login E2E workflow.

- [ ] **1. Add a required CI check invoking the reproducible docs verifier and isolated browser config.** Record Go/Node/tool versions; use lockfiles. Assert generation and tests do not mutate the real environment or require customer credentials. Include a deliberately invalid contract/spec fixture in test-only temporary output and assert the verifier fails.
- [ ] **2. Run affected checks and the final Go suite:** `go test -p 2 -short -count=1 ./cmd/... ./internal/... ./pkg/... ./sdk/go/... ./tests/... ./examples/docs/go`; enable owned disposable Redis/ES integration fixtures separately, and run existing credit lifecycle, broadcast receipt loss, import and channel-connection regressions. On Linux/CGO-supported infrastructure run `go test -race -p 2 ./internal/usecases/services ./internal/infrastructure/repository ./internal/infrastructure/sse ./internal/interfaces/http/handlers ./cmd/worker`; do not report a Windows non-race run as equivalent.
- [ ] **3. Run documentation/UI verification:** `node scripts/verify-docs.mjs`, `npm --prefix sdk/js test`, `npm --prefix ui run build`, `npx playwright test -c e2e/docs.config.ts`, `npx playwright test -c e2e/mocked.config.ts`. Exercise the intentional legacy-signup configuration case in a dedicated fixture build so its default skip is not mistaken for coverage. Record each count/result and any blocked fixture/race check explicitly; do not replace provider integration claims with mock success. Existing saved-state E2E suites requiring a real backend are separate and must not be reported as executed by these mock-only commands.
- [ ] **4. Obtain a fresh whole-change review of commits since `2c0093b`, with the approved spec and five Review Focus conditions.** Fix findings through a failing regression and rerun affected checks. Scan the scoped diff for credentials and accidental unrelated changes; record justified remaining limitations. Native execution uses one fresh final reviewer; subagent execution also follows its task review gates.
- [ ] **5. Document rollout/rollback and handoff:** deploy compatible workers before the new OTP writer, upgrade all APIs, retain readers through the maximum 900-second legacy OTP TTL and any pending state, and never roll back onto a writer that can overwrite new state. Include SSE fresh-token/reconnect canaries, signed-file public URL/reverse-proxy limits, operator provider setup, fixture versus live verification and phase commit IDs. Update this plan's phase checkboxes only with completed evidence.
- [ ] **6. Commit:** `test: enforce documentation contracts and record phased verification`. Clean up only owned fixtures/artifacts after checking identity; leave unrelated user files intact. Report all four phase results and pending production/operator actions without pushing or deploying.

## Phase Evidence and Progress

- [x] **Phase 1 (Tasks 1–5):** complete locally; commits `9e59347` through `99d9dee`; full Go/Redis/worker/file/upload checks passed.
- [x] **Phase 2 (Tasks 6–8):** complete locally; commits `5e52c28`, `c2a54a5` and the snippet/SDK commit containing this record. 32-case contracts, generated Go compilation, Go/TS/React SDK examples, SDK suites and UI build passed.
- [ ] **Phase 3 (Tasks 9–12):** pending implementation; record stored Meta, Twilio schema, email attachment and webhook/poll checks here.
- [ ] **Phase 4 (Tasks 13–15):** pending implementation; record generation repeatability, route/schema coverage, docs browser/build, final regressions, review and credential scan here.

## Planning Self-Review and Handoff

- [x] Confirm every approved spec section is covered by a task and phase exit check.
- [x] Check all cross-task signatures/types, source paths and test command selectors.
- [x] Confirm each Review Focus condition has explicit assertions in its owning task.
- [x] Confirm plan steps name concrete behavior without prescribing unnecessary function bodies.
- [ ] Present this plan for review and preserve the user's selected execution method before product implementation.

Recommended execution: native execution in this session, with local phase commits and one fresh whole-change reviewer. OTP service/repository/worker tasks and the later documentation fixtures/reference generator share interfaces, so one implementer can integrate them with less coordination overhead. Subagent-driven execution remains available if the user prefers a fresh implementer/reviewer per task.

Dependency reference checked on 2026-10-21: [kin-openapi v0.151.0 source module](https://github.com/getkin/kin-openapi/blob/v0.151.0/go.mod) declares Go 1.25 and [openapi2conv](https://pkg.go.dev/github.com/getkin/kin-openapi/openapi2conv) supplies the Swagger/OpenAPI conversion. The existing Go 1.26.1 floor is sufficient; the dependency is for generation/validation, not a new HTTP runtime framework.

### Phase 1 execution evidence (2026-10-21)

Commits: `9e59347`, `406ce80`, `e472494`, `8eecdf5`, `99d9dee`. Passed `go test -p 2 -short -count=1 ./cmd/... ./internal/... ./pkg/... ./sdk/go/... ./tests/...` with isolated Redis at `127.0.0.1:16390`; billing, SSE, worker, OTP, file downloads and multipart boundaries included. JavaScript SDK 5/5 passed. No production calls or data writes.
