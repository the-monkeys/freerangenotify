# WhatsApp and SMS customer connections and delivery design

Status: Phases 1–3 approved for local implementation. Phases 1–3 implemented and verified locally, including review fixes. Phases 4–5 remain pending. No production changes.

## Purpose and scope

Enable affordable, reliable WhatsApp and SMS for customer applications. Each application must clearly identify its provider, sender, credential owner, environment and charges. Preserve existing Twilio integrations, notification payloads and historical billing records while fixing credential exposure and customer isolation.

Use direct Meta Cloud API as the preferred option for newly connected WhatsApp customers. Keep Twilio WhatsApp and SMS, and the existing Vonage SMS adapter. Implement Exotel as the first proposed domestic Indian SMS adapter; keep it disabled until its account, destination, DLT and pricing requirements are verified. MSG91 and other domestic adapters are future extensions, rather than simultaneous implementations.

Recipients can use their ordinary phone/WhatsApp accounts. Programmable senders still need the applicable business/provider onboarding; ordinary personal WhatsApp automation is not a supported production route. Small senders should be able to bring their own eligible account without changing the platform's global credentials.

## Compatibility and rollout constraints

- Implement in one local branch, proposed name `codex/whatsapp-sms-reliability`, with reviewable commits in five phases. Preserve pre-existing working changes. Do not push, deploy, send live messages or edit real `.env` values as part of implementation without a specific rollout instruction.
- Keep existing API-key server integrations, channel names `whatsapp` and `sms`, Twilio `content_sid`, raw Meta `whatsapp_template`, and FRN rich-template payloads.
- Keep existing legacy configuration field names and environment variables readable. Explicit new fields override legacy aliases. Conflicting aliases for the same purpose report a configuration conflict without logging values; deliberately separate accounts/signing contexts remain supported.
- Security corrections intentionally remove secrets and unrestricted master keys from ordinary read responses. Do not preserve viewer access to credentials. Update dashboard clients in the same release; document this change for owners whose integrations read settings.
- Omitted secrets retain existing values. Never treat masked display strings as credentials. Use an explicit clear operation. Restrict key reveal/rotation to authorized owners; keep existing SDK authentication working. Previously exposed keys need separate operator-directed rotation; hiding new responses does not revoke already copied keys.
- Keep existing pricing versions and queued reservation amounts intact. New detailed pricing requires an explicitly activated immutable rate-card version; do not rewrite current rates automatically.
- New provider availability and readiness must be discoverable. Explicit missing, disconnected, forbidden or invalid connections fail clearly; do not silently send from another customer's/platform account.
- Existing wallet reservation, idempotency and unknown-outcome safeguards remain authoritative. An ambiguous vendor timeout must not trigger an automatic resend, cross-provider fallback or refund.
- Every phase must have regression evidence and a reviewable diff. No blanket deletion of documents, data, keys or provider implementations.

## Connection model

Add a `channel_connections` repository with optimistic concurrency. Each record has `id`, `app_id`, `environment_id`, `channel`, `provider`, `credential_source` (`system` or `byoc`), sender/account identifiers, encrypted credential material, readiness, revision and timestamps. Legacy parent-app settings remain a read adapter until migrated. A new environment override resolves before the parent/default connection.

Use explicit readiness states: `unconfigured`, `pending`, `ready`, `disconnected`, `invalid`. Saving a token is not proof of readiness. Readiness covers provider permissions, chosen sender ownership, applicable number registration, callback subscription and account billing readiness where the provider exposes it. It must not send a paid test message automatically.

Claim inbound sender identity durably by provider/account/sender. A WABA can contain multiple phone numbers; phone identity is primary and its WABA/account must agree. Reject conflicting new inbound assignments. Existing shared platform outbound senders can continue to work, but ambiguous inbound events are quarantined rather than guessed into an app/environment. Do not scan only the newest 500 applications.

Credentials are encrypted using AES-256-GCM with a versioned keyring, supplied through `FREERANGE_CONNECTION_ENCRYPTION_KEYS`. Associated data binds ciphertext to connection/app/environment/provider/field. Provide decrypt-only old keys during rotation. New writes require encryption once the feature is enabled; legacy plaintext reads are restricted to server-side compatibility adapters. Migration is dry-run first, CAS-protected and resumable, with counts/checkpoints but no secret values in output. Enable encrypted writes only after all API/worker instances support reading them.

## Public configuration and onboarding

Add safe JWT/app-authorized connection management and public capability configuration. Safe reads expose readiness, provider, sender summaries, source, revision and configured flags; never tokens or arbitrary secret headers. Use the same app/tenant role resolver across Settings, Providers, templates and Meta management.

Manual Meta connection is available without Facebook SDK configuration. Fetch all permitted numbers and validate the selected phone ID against the selected WABA. Embedded Signup uses the actual returned customer assets, not fixed frontend business/WABA/phone defaults. Separate legacy manual/customer-owned Meta app signing from platform-app Embedded Signup signing: each callback must have an identified signing context.

Refresh parent settings after connect/disconnect. Save only changed fields with revision checks. Environment selection persists and participates in requests/cache keys. Disable unsupported UI actions based on capabilities, and distinguish failed loads from empty state.

Provider templates are scoped to the selected connection/account and environment. Shared account management needs explicit permission and a local ownership/binding map. Database lookup failures never select a system account. Rich drafts support editing and explicit submission; local removal and provider deletion are separate actions. Imported custom providers are filtered by individual resource links and expose origin/allowed actions.

## Security and inbound events

WhatsApp inbox SSE carries an explicit authorized app/environment audience. The broadcaster must not globally broadcast an inbox event lacking that audience. Existing user-targeted notification SSE remains compatible. Revoked members cannot receive further inbox events.

Verify callbacks before processing using the connection's signing context. Twilio signatures cover the full submitted form and the configured canonical public callback URL; do not trust arbitrary forwarded host headers. Verify AccountSid/sender ownership. Meta signatures use the originating app secret. Missing signing configuration fails closed. Protect template approval callbacks as well as inbound/delivery callbacks.

Persist a durable verified event receipt before acknowledging. Receipt identity includes provider, connection and upstream event/message identity. Duplicate callbacks do not duplicate conversation records, workflow triggers or forwarding. A crash resumes unfinished work; event receipt alone must not be mistaken for completion of all side effects. Return retryable failure if durable acceptance fails. Bound retries and provide a dead-letter/operator recovery path.

Customer-service windows use the greatest validated upstream inbound timestamp, not processing time. Old callbacks cannot reopen an expired window. All free-form WhatsApp sends, including worker sends, use this rule; outside the window require an approved template. Sending permission and fee eligibility are separate concepts.

Bind delivery callbacks to the original connection and notification. CAS status transitions cannot regress `read` to `delivered` or allow a stale failure to overwrite success. Track vendor timestamps/reasons separately. Inbox unread counts use actual local read state. Auto-reply is either implemented through the normal dispatch path with loop protection or explicitly reported unavailable; do not pretend a log-only implementation works.

## Outbound dispatch and billing

Resolve one immutable delivery snapshot before credit admission: connection ID/revision, provider, credential source, sender, environment, payload kind and immutable price quote. Store references/safe facts on notification metadata, never plaintext credentials. Queued sends keep their admitted account and price; disconnect/rotation or missing credentials must not change the account silently.

Instantiate app-owned adapters without needing global credentials. Twilio Content SID routing preserves Twilio BYOC credentials. Meta-only data cannot silently become a Twilio text message. Explicit cross-provider fallback requires compatible payloads, an authorized account and proof that the first attempt was not accepted.

Inbox replies create normal outbound notifications, retain provider IDs, record outbound conversation entries and use reservation/idempotency/usage machinery. Keep the existing reply request and response fields compatible, adding notification/status fields to report accepted/queued honestly. Support the selected Twilio or Meta connection, rather than a separate hardcoded Meta-only sender.

Phone-verification OTP uses a shared dispatch abstraction with a platform authentication connection and its own budget, rather than a customer wallet. Preserve existing verification endpoints. Use proper form encoding, contextual timeouts, explicit dispatch failure, existing OTP expiry/attempt controls and Redis abuse limits. Never log OTPs. Defaults: three dispatches per user per ten minutes, three per destination per ten minutes, ten per IP per ten minutes; operator-configured destination allowlist and platform budget apply. Redis budget failure rejects a new paid dispatch.

Detailed quotes distinguish platform-owned vendor costs from BYOC platform fees. SMS quotes use exact rendered text, destination and GSM-7/UCS-2 segment count, including GSM extension characters and surrogate pairs. Standard boundaries: GSM 160 single/153 concatenated septets; UCS-2 70 single/67 concatenated UTF-16 units. Respect provider-specific exceptions. Use server-derived WhatsApp category/window/entry-point facts and effective country/category rates, not client-supplied prices or free flags.

FRN processing charges settle once at provider acceptance; vendor actual delivered-message costs and late failures are tracked separately. Unknown outcomes retain holds for reconciliation. Show the distinction between estimated vendor costs and FRN charges; do not advertise free delivery when only a provider allowance is free. Existing flat quotes continue until a new detailed tariff is activated. New tariff publication needs verified dated provider/account rates, taxes/currency assumptions and BYOC fee policy.

## Domestic SMS and configuration cleanup

Exotel connections include account credentials, sender/header, DLT entity ID and approved template bindings. Validate the rendered template's permitted variables before enqueue. Indian domestic routes require appropriate entity/header/template registration; global Twilio/Vonage routes remain available under their own destination rules. Provider readiness, segment caps, per-sender throughput and spend budgets are separate from the application's HTTP request quota.

Add `FREERANGE_MESSAGING_META_ENABLED` as the simple new switch for both Meta delivery and onboarding. When it is absent, preserve the independent legacy delivery and onboarding flags exactly; a historically outbound-only deployment must not acquire new public routes implicitly. Keep one Meta app secret for a shared signing/OAuth context; preserve explicitly different legacy contexts where configured. Optional platform sender credentials remain separate from customer connections. One optional shared Twilio account plus channel sender overrides removes duplicated account credentials without preventing separate accounts.

Serve non-secret Meta App ID, signup configuration ID, API version and available connection modes from the backend at runtime. Remove frontend customer business/WABA/phone defaults after the runtime path works. Keep old public app/config variables as temporary fallback. Replace scattered hardcoded Graph versions with the configured version. Remove ignored retry options only after mapping their meaning to the queue policy; never introduce blind provider retries.

Remove the three unwired self-hosted WhatsApp files and unused dependencies only after reference/build verification. They currently have no wired API/worker delivery path and their paired-device isolation is unsafe. Mark the old hybrid SMS plan superseded rather than deleting unrelated documentation. `.env` examples contain placeholders only; actual `.env` edits and secret rotation belong to the deployment procedure.

## Acceptance and deployment

Mock-provider, real disposable ES/Redis, browser and race/CI coverage must prove tenant isolation, legacy payload/env compatibility, absent-global-credential BYOC, duplicate/crash-safe callbacks, settings update safety, selected-phone validation, GSM/UCS-2 boundaries and quote/settlement consistency. Never run mock tests against real provider accounts.

Deploy API/worker readers first, then encrypted writes/migration, then individual provider/price flags. Canary two separate customer accounts and two distinct phone numbers in one WABA, plus staging/production separation and a legacy Twilio customer. Abort new enablement on cross-tenant routing, credential leaks, duplicate provider acceptance, unexplained wallet divergence or callback loss. Rollback feature flags must not disable readers/recovery of already-created connection/event/reservation records. Historical repairs are separate audited operations.

## Commercial assumptions to confirm before enablement

India is the first domestic SMS market, not a restriction on existing international sends. Exotel account terms and lower-volume prices need a quote; no published high-volume rate is promised to a 6,000-recipient customer. Confirm direct Meta business/app approval, access permissions, account billing and current rate cards. New BYOC discounts and destination/category prices require a published immutable tariff. Code completion does not imply provider approval or production enablement.
