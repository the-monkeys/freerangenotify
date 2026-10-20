# API documentation and delivery contracts

Date: 2026-10-21

Branch: `codex/whatsapp-sms-reliability`

Status: Design and implementation plan approved on 2026-10-21 for native execution of all four phases on the existing branch. Phase 1 verified locally; later phases in progress.

## Outcome and scope

Customers must be able to create an application in the dashboard, obtain its application ID and API key, register recipients, create templates, and send and inspect notifications using examples that match the running API. The same explanation must hold across public guides, the API reference, application integration snippets, contextual UI help, and SDK examples.

The user approved four phases on the existing branch, delivered in order so changes can be monitored. Preserve existing integrations. Repair confirmed defects in the documented flows instead of publishing examples around broken behavior. This work extends the completed WhatsApp/SMS work; it does not replace its routing, billing or callback design.

The accepted approach combines targeted repairs to existing flows, short task-oriented guides, and a generated API reference checked against routes and example fixtures. An editorial-only update would leave delivery defects unresolved. A replacement API or documentation framework would add migration work without helping existing integrations.

## Compatibility rules

- Preserve existing endpoint paths, channel names, request field names and response envelopes. Existing API-key authentication through `X-API-Key` and `Authorization: Bearer <app-key>` remains supported. Dashboard account JWTs remain distinct from application API keys.
- An application API key determines application scope. Do not introduce a new mandatory application-ID header. Explain the existing cases that require an app ID in a URL or body, including template creation. Preserve current environment-key selection.
- Keep legacy user identifiers readable. Recommend `external_id` for a customer's identifier, and explain the returned internal `user_id`; do not silently reinterpret or migrate existing IDs.
- Preserve normal SSE token issuance, legacy authenticated connections, named events and notification payloads. Private dashboard events keep their existing authenticated-principal rules. The integration sample must use `addEventListener('notification', ...)`.
- Notification creation accepts the existing flat request structure. Do not replace it with nested `content`, or rename `template_id` to `template`. Explain direct-content, template, bulk, batch and broadcast variants separately.
- Preserve working Meta payloads, Twilio `content_sid`, provider connections, generic webhook envelopes and signing headers. Correct vendor-facing serialization without changing customers' stored authoring models.
- Cross-app OTP access and wrong-code success are defects, not compatibility contracts. Reject them while retaining legitimate same-code verification retries and current success response shapes.
- No historical balances, tariffs, customer credentials, stored templates or production data are rewritten by these phases. Existing unrelated working changes are excluded from phase commits.

## Phase 1: Repair functional defects

### OTP ownership and state transitions

HTTP send, verify and resend must carry the authenticated application into the service. Verify and resend must check the stored request's application before checking a code, incrementing attempts or performing delivery. Return the existing not-found response for another app's request so the endpoint does not disclose its existence. Keep the HTTP JSON request bodies unchanged. Preserve existing exported service call sites through explicit scoped entry points or additive internal context rather than an incompatible method replacement.

Bind new requests to the effective environment as well as the application, and use that environment for notification creation and provider selection. Existing OTP envelopes without an environment retain application scope only for their existing remaining TTL; do not extend their expiry. This is an explicit short compatibility window, bounded by the current maximum OTP lifetime. New environment-bound requests cannot be used through another environment's key.

Make verify and resend record transitions atomic in Redis. A successful verification cannot be overwritten by a stale resend, concurrent resends cannot both pass the cooldown, and attempt counters cannot be reset by a stale record write. Use version-checked transitions or a token-fenced operation claim; a process-local mutex is insufficient. Do not hold a Redis transaction open across a provider request.

For an already-verified request, compare the supplied code in constant time before returning the previous success. A retry with the same valid code remains idempotent and consumes no additional attempt. A different code returns the existing invalid-code outcome. Expired requests remain expired.

Resend keeps its existing behavior: same request ID, a fresh code, invalidation of the previous code, reset attempt count, original expiry, and current cooldown. Store additive fields for the resolved recipient ID and original message configuration so resend preserves branding, routing and recipient preferences. For old records without those fields, retain the existing default-message resolution until their original expiry.

Apply the existing per-app/per-recipient hourly budget to both send and resend. Count a resend only after ownership, expiry and cooldown checks pass. Keep the existing hourly window and configured/default budget; correct guides that describe a different window. Make the Redis increment and initial expiry one atomic operation so an expiry failure cannot leave a permanent counter. An unavailable budget store returns a retryable infrastructure error before dispatch rather than silently bypassing abuse controls.

Persist authoritative OTP state before allowing a notification to be enqueued. For resend, record a fenced pending operation before dispatch and finalize it only for the same operation version. Distinguish definite pre-enqueue failure from an uncertain enqueue outcome. Reuse a stable internal delivery identity during recovery; never blindly resend an operation with an unknown outcome. Verification must never accept an uncommitted replacement code. Preserve enough pending state for bounded reconciliation or an explicit retryable outcome without restoring stale verified/attempt state.

Keep `otp_expires_at` enforcement before provider calls and retry/fallback attempts. Translate expected channel-admission failures, including a closed WhatsApp service window without an approved template, into a documented client error rather than a generic server failure. Do not bypass provider permissions or billing to make an example pass.

### OTP content and error descriptions

Remove raw OTP codes and full OTP request data from debug/error logging across the notification path, including body/data logging and generated template content. Avoid creating a permanent template containing each generated code; use the existing direct-content notification path with preserved template customization and admission rendering.

The verification record stores a hash, while the existing notification delivery system persists message content. The revised guides must make that distinction explicit: this scope does not promise end-to-end encryption, automatic erasure of historical notification bodies, or that no delivery record ever contains a code. Retrofitting encryption and retention across all notification storage is a separate storage migration, not a claim to add to customer documentation. Public API responses must not introduce a raw-code field.

Document the existing flat OTP outcomes separately from middleware errors. Preserve existing statuses and human-readable errors where clients may depend on them. Optional machine-readable additions must be additive. The JavaScript SDK must map only recognized string codes and handle nested middleware errors without assigning an object to its code field. After expiry, examples start a fresh send; they do not suggest resending an expired request.

### Signed files and request limits

Use one signed-download URL builder for the handler and worker. Include encoded application ID, file ID, expiry and signature, and join the configured public base URL safely. Keep existing HMAC verification, key rotation, expiry semantics and public download route. A generated worker link must successfully round-trip through the real public handler for an authorized file; missing, tampered and cross-app parameters remain denied.

Align the HTTP upload ceiling with the effective configured file limit plus bounded multipart overhead. Preserve the existing non-upload request ceiling and smaller limits elsewhere; raising file capacity must not silently allow oversized ordinary JSON bodies. The file service remains authoritative for actual file bytes, allowed MIME types and the hard upper bound. Test an upload above the old 16 MiB transport ceiling and an upload above the configured file cap. Configure no real production limits in this work.

Preserve the existing file API operations: upload, list, inspect, authenticated content, signed URL and delete. Explain logical expiry separately from physical orphan cleanup. Do not claim an expiry sweeper or per-tenant retention setting that is not implemented.

### Exit criteria

Regression tests cover cross-app/environment requests, correct and incorrect idempotent verify, concurrent verify/resend, concurrent resends, shared send/resend budget, TTL preservation, recipient/customization preservation, persistence/queue failure boundaries, and expiry before provider invocation. Signed-link tests exercise the generated URL through the public handler. Upload tests verify both transport and file-service boundaries. Existing OTP, SSE, queue and billing tests remain green.

## Phase 2: Simplify onboarding and CRUD guides

### Customer journey

Start Getting Started with the actual dashboard sequence: open Applications, choose New Application, enter the name, create it, open the app, and use Integration to copy the app ID and obtain an owner-authorized API key. Explain the one-time creation key, existing owner reveal/copy action, environment keys and the difference between a dashboard reference and a usable SDK credential. Never embed customer secrets into public docs or automatically reveal them in examples.

Use a consistent `BASE_URL` that includes `/v1`, `APP_ID` and backend-only `API_KEY` vocabulary. Provide hosted and self-hosted setup instructions without hard-coding the hosted endpoint into every generated example. Render the active deployment's API URL through trusted configuration, while public guides use explicit configurable placeholders. Request host or forwarded headers must not become an unvalidated source for generated executable code.

The first working example creates a recipient with `external_id`, reads its internal ID from the actual response envelope, creates an email template with `app_id`, sends a notification, and inspects delivery status. Explain that acceptance/queueing is different from provider delivery and recipient read state. Show representative responses and the exact response field used in the next step.

### Guide organization

Add explicit Authentication, Users and Notifications guides to the existing navigation. Retain all current documentation URLs and update their cross-links. Use short sections for purpose, prerequisites, request, response, next action and common errors.

Users cover create, read, list/pagination, update, delete, lookup/update by external ID, devices, preferences and bulk import. Keep documented CSV limits and partial-failure behavior aligned with the existing importer. Explain that deleting a user is a data operation with its current service semantics.

Templates cover create, list, read, update, delete, render, explicit version creation, version history, diff and rollback. State that normal edits update the current document in place; explicit versioning creates snapshots. Correct version routes to their actual app/name/version structure. Include channel-specific variables and required application scope.

Notifications cover send, inspect/list/filter, bulk, batch, broadcast/topic recipients, scheduling, read/archive/snooze, status updates, cancellation and retry. Explain allowed state transitions. Do not imply that a delivered message can be edited or recalled from an external provider. Document idempotency at the endpoints that implement it; do not promise exactly-once delivery for arbitrary network retries.

Review the existing Topics, Workflows, Environments, Pricing and Troubleshooting guides too. Correct fields such as `key` versus `topic_key`, `trigger_id` versus `trigger`, API key versus JWT authentication, hourly application limits, body-size limits, and rate-card-dependent pricing. Prefer live pricing/billing links over another fixed universal price list.

### UI snippets and SDKs

Correct server-generated integration snippets and their UI labels together. Keep their existing response structure and language keys. Preserve secret placeholders and owner-only credential reveal. Remove the misleading Show Token behavior from samples when the endpoint deliberately returns placeholders. Give snippets prerequisites and error/status handling; a Go sender must compile without unused imports.

Use the packages and APIs actually exported in this repository: `@freerangenotify/sdk`, its API-key-plus-options constructor and method-specific field names, and the existing Go SDK import and `New`/context-based methods. Verify React exports and hooks against their source. Do not assert that a package version is published without checking publication separately. Keep working legacy convenience methods intact.

### Exit criteria

A new customer can follow the guide from UI app creation through a complete users/templates/notifications example without guessing credentials, request structure or response paths. All old guide URLs resolve. Generated Go snippets compile, JavaScript examples type-check against the shipped SDK, and mocked browser checks cover navigation, copied examples and deployment URL selection.

## Phase 3: Correct channel, attachment and rich-message behavior

Cover every implemented channel: email, push, SMS, WhatsApp, webhook, in-app, SSE, Slack and Discord. Separate a channel from its selected provider and from provider-specific rich-content capabilities. Explain customer-owned credentials through the existing app settings/connections UI, especially custom SMTP. Failed credential validation is an error state, not an empty or connected state.

For every documented send, show recipient prerequisites, provider setup, exact payload, acceptance response and delivery-status inspection. Explain SMS phone formatting/segmentation, WhatsApp approved templates and service windows, push device registration, in-app persistence and SSE reception. Keep platform configuration separate from settings available to an individual app owner.

Document multiple attachments using the actual top-level `attachments` array, source exclusivity (`url`, `content_base64` or `file_id`), content IDs, MIME/size limits and asynchronous resolver failures. Distinguish upload errors from later provider-delivery errors. Cover a URL, uploaded file, inline email image and multi-file email with compilable SDK equivalents.

Repair the existing Resend adapter to forward inline `content_id` where requested, preserving ordinary attachments. Verify every other email adapter's advertised attachment behavior with fixture-backed tests rather than assuming uniform provider limits.

For generic outbound webhooks, preserve the current `id`/`content`/`created_at` envelope and optional `X-Webhook-Signature` hex HMAC over the exact raw request bytes. Supply a matching receiver-verification example. Generic webhooks forward attachment descriptors; they do not promise binary upload or automatic file resolution. Explain inbound workflow webhooks separately from outbound notification delivery and provider callbacks.

Use registered custom providers and `webhook_target` for Slack/Discord/Teams renderer examples. Test their attachment routing and signed links. State that Slack/Teams polls render as choices in text, and Discord's native-poll attempt has a narrowly defined fallback. No guide may promise voting controls or native WhatsApp/SMS polls that the integration does not implement.

Repair stored WhatsApp list templates by routing them to the existing interactive list path with service-window checks; a free-form list must not require a nonexistent approved-template binding. Repair approved Meta CTA and quick-reply template resolution/rendering without weakening approval, app/environment binding, account selection or billing classification. Keep raw `whatsapp_interactive`, `whatsapp_template`, `whatsapp_rich` and Twilio ContentSid paths compatible.

Correct Twilio carousel authoring serialization to the current vendor schema: card body, string media URL and valid actions. Preserve FRN's stored model and existing bindings; apply fixes to new authoring requests without automatic provider resubmission of old templates. Use the actual supported copy-code button mapping rather than silently changing it into a reply where the route supports it.

Unsupported combinations must be described and surfaced by capability-aware UI/help. Preserve existing plain-message integrations and their payloads. A newly exposed typed action must be disabled or rejected explicitly if unsupported, with a clear explanation; do not make an accepted documented action silently discard its requested content.

### Exit criteria

Test complete stored-template create/resolve/admit/render paths, not only individual render functions. Strict mock provider fixtures reject invalid carousel/button shapes. Mock webhook receivers verify envelope and signatures. Attachment tests cover email bytes/CID, custom provider uploads/links and generic descriptors. Local verification reports provider mocks separately from any live external delivery; real paid sends are not part of implementation.

## Phase 4: Synchronize references and prevent drift

Retain the existing Swagger generation approach and UI documentation renderer. Make generated handler/DTO metadata the reference source and publish the same generated specification to the API and bundled UI fallback. Correct stale annotations and add missing customer-facing routes, including SSE tokens and messaging templates. Keep existing reference URLs usable. Generated copies are updated by one reproducible command and checked for drift; do not manually maintain a second abbreviated specification.

The API reference must use the same configured API base as the UI, validate a fetched document before accepting it, and fall back predictably when a remote specification is unavailable. Render referenced request/response schemas, authentication instructions and representative JSON examples rather than displaying a blank type for `$ref` bodies. Distinguish app API keys, account JWTs, public signed links and provider-signed callbacks.

Preserve existing OpenAPI documents/URLs. Generate compatible views from shared route/schema definitions, or have their existing entry documents reference shared generated schemas with no contradictory hand-authored request models. Validate both Swagger 2 and OpenAPI 3 consumers where retained; do not serve one format while labeling it as the other. Feature-gated operations must disclose their prerequisites rather than appearing universally enabled.

Add focused checks that compare documented method/path pairs with registered routes, resolve schema references, check copied specification consistency, and execute representative example payloads through real DTOs/handlers. Maintain an explicit allowlist for intentionally undocumented internal routes. Check required fields, channel enums, response envelopes and documented failures. Avoid brittle tests that merely search for a chosen sentence.

Compile representative Go examples and type-check JavaScript/React examples. Use fake HTTP providers, disposable Redis/Elasticsearch fixtures and isolated browser configuration; never let a docs test inherit live customer credentials or production global setup. Browser checks cover public docs without login, all guide links, API-reference success/fallback, copied snippets, narrow screens and light/dark themes. Handle stale async page loads so navigation cannot display the previous guide under the new URL.

### Exit criteria

Generation is reproducible, both live and bundled references describe the same supported customer routes, examples exercise current contracts, and UI build/browser checks pass. Re-run the affected Go/SDK suites and the existing billing, broadcast, customer-connection and SSE compatibility regressions. Include concurrency verification for OTP transitions. Record skipped tests and environment limitations explicitly; a passing mock is not evidence of vendor acceptance in production.

## Phase tracking, review and rollout

Create small local commits grouped by phase with regression evidence, then mark each phase complete only after its exit criteria pass. Keep a phase checklist in the implementation plan and post concise progress updates naming completed behavior, checks and remaining work. Final integration review covers the full documentation/API diff and backward compatibility, not only the final phase.

Deployment is separate from local completion. No push, production repair, secret rotation, `.env` modification or live customer send is included in this design. Roll out compatible readers and workers before enabling new record formats. Additive fields and old envelopes must remain readable. A rollback cannot route new state through a worker unable to understand it; keep reader compatibility or drain the short-lived state before downgrading. Ordinary API restarts can interrupt SSE connections, so the release checklist includes fresh-token and reconnect canaries.

The handoff must identify phase commits, reproducible checks, intentional OTP security corrections, upload/reverse-proxy requirements, remaining provider/operator setup and the difference between local fixture verification and production delivery. Do not report the four phases complete while documentation, contract tests or an advertised supported path remain unresolved.

## Source evidence and provider references

Repository evidence: `ui/src/docs/`, `ui/src/config/docsNav.tsx`, `ui/src/pages/docs/`, `ui/src/components/apps/AppCodeSamples.tsx`, `internal/interfaces/http/routes/routes.go`, `internal/interfaces/http/dto/`, `internal/interfaces/http/handlers/`, `internal/domain/otp/models.go`, `internal/usecases/services/otp_service.go`, `internal/infrastructure/repository/otp_api_repository.go`, `internal/usecases/services/file_service.go`, `cmd/server/main.go`, `cmd/worker/main.go`, `cmd/worker/processor_delivery.go`, `internal/infrastructure/providers/`, and `sdk/`.

Provider references checked on 2026-10-21: [Twilio carousel](https://www.twilio.com/docs/content/carousel), [Twilio call-to-action](https://www.twilio.com/docs/content/twilio-call-to-action), and [Resend inline CID attachments](https://resend.com/changelog/embed-images-using-cid). Recheck version-specific Meta payload details against official documentation during the relevant implementation task.

## Design workflow checklist

- [x] Explore existing context, routes, guides, snippets and relevant service/provider boundaries.
- [x] Reflect customer intent and backward-compatibility requirements.
- [x] Present the four-phase approach and obtain scope approval.
- [x] Write the design with explicit behavior and phase acceptance criteria.
- [x] Complete the inline specification self-review for placeholders, consistency, scope and ambiguity.
- [x] Obtain review of this written specification.
- [x] Write the detailed implementation plan after specification approval.
- [ ] Obtain implementation-plan review and execution-method selection before product changes.
