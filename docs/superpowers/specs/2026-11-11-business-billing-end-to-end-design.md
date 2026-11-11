# App Business Billing: End-to-End Design

Date: 2026-11-11. Base: `main` at `94acfe1`. Branch: `codex/business-billing-end-to-end`.

Status: proposed design, awaiting user approval after builder review. This is not authorization to deploy or modify production data.

## Intent and agreed decisions

Complete the billing module **inside an application**: businesses sell their products and bill their own subscribers. Platform notification-credit purchases and workspace licensing are outside this project.

The user selected:

- Complete and harden the existing billing module, preserving existing integrations and production records.
- Each business connects its **own Razorpay account**; payments go to that account.
- Each subscription can use invoice/payment-link collection or automatic recurring payments after customer authorization.
- Deliver both business and customer experiences, responsive on mobile.
- Keep code focused, modular, extensible, and free of unnecessary additions.

No software can guarantee zero production failures. Acceptance requires demonstrated correctness under concurrency, duplicate callbacks, interrupted writes, and gateway outages, plus a recoverable rollout. Passing tests is not a claim that provider outages or every future defect are impossible.

## Existing implementation and confirmed gaps

- `/v1/biz/*` already provides products, plans, subscriptions, invoices, payments, credits, estimates, contracts, usage, expenses, analytics, and connectors. Preserve these resources and their IDs.
- `internal/container/container.go` injects the server-wide `PaymentProvider` into `BizLifecycleService`; it is not a per-app merchant connection.
- `bizbillingrepo/store.go` and `lifecycle_repo.go` use unguarded read/mutate/replace operations. `RecordPayment` writes a payment and then updates an invoice separately. These writes can race or be interrupted.
- `RenewSubscription` advances the period before generating its invoice. A failure between those operations can lose a billable period.
- `RefundPayment` changes local records without initiating or confirming a gateway refund.
- `RecordGatewayPayment` currently settles the current amount due using an order/payment identifier, without passing the captured amount/currency into settlement.
- Portal endpoints exist, but there is no customer portal React route. `PayInvoice` lacks the complete merchant checkout descriptor.
- The business UI is concentrated in `AppBizBilling.tsx`; initial request failures become empty lists, hiding outages. Configuration uses raw JSON rather than task-specific forms.
- `BizNotifier` is best-effort; the scheduler can advance reminder progress even when admission/delivery failed.
- The integration suite can skip business billing when its feature flag is disabled. Current green general CI is not sufficient evidence for this module.

These are source-level findings; no production database was queried or changed for this design.

## Approaches considered

**Selected: strengthen the existing Elasticsearch-based module.** Retain its APIs and indices, introduce versioned operations, optimistic concurrency, durable event processing, and an app-scoped gateway adapter. Reuse the existing secret codec and notification engine. This minimizes migration and integration risk, but requires explicit recovery for operations spanning documents; Elasticsearch is not a multi-document transaction system.

**Alternative: a new SQL-backed billing engine behind the old APIs.** Relational transactions would simplify some accounting invariants. It adds infrastructure, dual-write/cutover risks, and a substantial migration. The user selected the existing-module approach; introducing another database is not part of this release.

## Scope and boundaries

The shipped core journey is: configure business and gateway → create product/plan → select subscriber → create subscription or one-time invoice → issue invoice → customer pays → captured payment reconciles → receipt and statement update → renew/remind/refund as appropriate.

Existing advanced features remain available and receive regression coverage for touched paths. Do not delete or rebuild estimates, contracts, expenses, retainers, coupons, or analytics merely to simplify the UI. Do not expand this release into a new ERP, new payment providers, statutory e-invoicing, SDK languages, or a generic workflow framework.

Provider constraints are real product constraints. Initial automatic collection supports fixed-amount INR plans with cycles and terms supported by the connected merchant's Razorpay Subscriptions capability. Existing usage-based, custom-cycle, variable-charge, and other-currency invoices remain supported through their current/manual collection path where valid. Unsupported automatic combinations must be rejected before authorization with a clear explanation; never silently switch modes or approximate a price. Extending those automatic combinations is separate scope.

## Compatibility and production-data preservation

1. Preserve existing routes, authentication, RBAC, identifiers, response envelopes, amount units, and status meanings. Add optional fields and endpoints; do not rename legacy fields. Existing SSE, notification, and platform-payment integrations remain unchanged.
2. Missing subscription collection mode means `invoice`; existing records never acquire a payment mandate or automatic charges through migration.
3. Existing `frn_biz_*` records and indices are retained. New operation, gateway-connection, and event documents are additive. Do not reindex into empty replacements, delete history, regenerate invoice numbers, or recalculate issued historical totals.
4. Preserve stored currencies and minor units. New calculations use checked integer arithmetic, explicit rounding rules, and immutable issued-price/tax snapshots. Do not rewrite historic float exchange rates or silently convert currencies.
5. Scope business records and authorization by existing `app_id`. The new merchant connection has separate `test` and `live` slots. Persist the selected slot/account reference on each order, payment, and mandate; a dashboard switch never changes ownership of an existing transaction.
6. Old server-wide gateway orders remain linked to their original gateway account and callback path. Connecting a new business account must not adopt, refund, or recreate those orders. Keep the legacy reader/settlement adapter for existing transactions only; new orders require an explicit app merchant connection.
7. Migrations default to dry run and resume with checkpoints. Report counts, missing references, currency mismatches, and unresolved legacy orders before any approved write. Ambiguous records are reported, never guessed or deleted.
8. Release preparation requires verified backups and a tested restore on isolated infrastructure. Production backup, migration, credential changes, and deployment require a later explicit rollout instruction.
9. Upgrade the billing lifecycle per app through an explicit, recorded readiness/cutover step. Apps not yet upgraded retain their legacy API behavior until their accounts and balances are reviewed; do not globally activate new collection semantics on deployment. For upgraded apps, new gateway orders require their own merchant account, and old orders still settle through their original binding. Keep one active mutation path per app/version; do not dual-write money through legacy and journal writers. A migration/connection prerequisite must be visible before an admin activates the upgrade, not discovered after an existing payment flow is silently rerouted.

## Architecture and code boundaries

Keep the existing domain and HTTP contracts. Separate responsibilities behind small interfaces rather than adding parallel service hierarchies:

- `internal/domain/bizbilling`: collection mode, immutable transaction/account references, operation records, money/state invariants, and gateway capability contracts.
- `internal/infrastructure/bizbillingrepo`: conditional create, versioned updates using `if_seq_no`/`if_primary_term`, operation/inbox/outbox storage, and reconciliation queries.
- `internal/infrastructure/payment`: an app-scoped Razorpay adapter for orders, payment lookup, refunds, plans, subscriptions, and their capabilities. Keep platform billing's adapter behavior independent.
- `internal/usecases/services`: focused merchant onboarding, invoice/payment settlement, recurring collection, reconciliation, and notification-outbox responsibilities. Split the large lifecycle service only at these boundaries, retaining a thin compatible facade where callers need it.
- `internal/interfaces/http/handlers` and existing routes: authenticated app management, customer portal, and signed merchant callbacks.
- `ui/src/components/biz-billing` and focused pages/hooks: domain forms and lists; shared accessible primitives; separate public customer portal.

Reuse the existing AES-GCM secret codec through a narrow billing wrapper. Bind ciphertext to app, merchant connection, mode, provider, and secret field. Retain key IDs for rotation. Never serialize secrets through the generic billing config endpoint or return plaintext from reads. No new crypto implementation or provider plugin framework.

## Payment correctness and durable recovery

### Operations, events, and projections

Every money-changing action has a durable, conditionally-created operation with an immutable intent, scope, idempotency key, and progress. Gateway identities are scoped by app, merchant account, mode, and provider. Reusing a key with different intent is a conflict.

A single accepted journal entry is the durable accounting fact. Invoices, payment history, customer credit, and reports are projections of those facts. Apply each projection with version checks and a durable applied-operation marker or ordered stream checkpoint in the same conditional update as its amount change. Customer-balance updates use an ordered customer/currency stream so independent invoices cannot lose credits. Preserve journal entries for audit; bound transient worker state without dropping deduplication evidence.

There is no claim of an atomic transaction across Elasticsearch documents. Track incomplete projections and replay them after interruption. A synchronous action returns its normal success only once the necessary projections are applied; uncertain/incomplete actions expose an additive operation reference and a retryable error/status. Never display a completed refund or paid invoice on the basis of an unconfirmed request. Replays resume the existing operation rather than performing the money effect again.

All billing mutations—including manual payments, credit notes, customer balance, retainers, coupons, late fees, refunds, and scheduler actions—must use the protected path where they affect money. Fixing only the gateway callback would leave competing legacy writers unsafe.

### Gateway events and external effects

Verify each callback against the exact raw bytes and the correct merchant/mode secret. Retain prior secret revisions for valid in-flight callbacks. Persist the verified event before acknowledgment; deduplicate both provider event IDs and the underlying business effect. Handle duplicate and out-of-order events without regressing settled states. Validate provider account, order/subscription linkage, payment identity, captured status, amount, and currency server-side.

Persist an external-effect intent before creating an order, mandate, or refund. Use documented provider idempotency where available. If a timeout leaves the result unknown, query/reconcile using stored references before another external effect. Do not assume every Razorpay API offers idempotency, and do not blindly retry an uncertain charge or refund. An unresolved action remains pending and visible for reconciliation.

Manual cash/bank payments remain available under existing write permissions. Manual records explicitly identify their source; passing a gateway transaction ID does not prove a captured gateway payment. Offline refund recording is distinct from an online gateway refund.

### Collection exclusivity (proposed safety default)

An invoice committed to an automatic collection has a frozen total and one collection owner. Until that collection is captured or definitively stopped, reject manual/portal payments, credit or retainer applications, late-fee changes, voiding, and write-offs against that invoice with an actionable conflict. Do not accept the local change and hope to cancel the remote debit later. Once captured, corrections use explicit credit/refund operations; they do not change the captured charge.

To make an adjustment before collection, the business must first stop the automatic collection and obtain provider confirmation. The invoice can then move to manual collection and be adjusted. If Razorpay cannot stop the relevant debit safely, retain the lock and reconcile its outcome; do not offer an unsupported skip-cycle operation. Future automatic periods can resume only through the supported authorization/lifecycle path. Unissued drafts remain editable. If funds arrive externally despite this lock, record the verified money fact once and reconcile the excess; never discard a real payment merely because the UI would have blocked initiating it.

### Legacy credit currency (proposed compatibility default)

New credit notes, retainers, customer-balance entries, and adjustment requests receive an additive currency field. Currency-less new requests continue to use the app's primary billing currency; snapshot that primary currency at onboarding/upgrade, using the existing configured default for the API default when the business has not selected another. Changing it later does not relabel balances. Keep the legacy scalar balance fields as the primary-currency view after migration; additional currencies use separate balances and statements. Never sum currencies or silently convert them.

This default for **new requests** is not evidence about old money. Retain the original currency-less records unchanged. Attribute a legacy opening balance only where source records unambiguously establish its currency. Mixed or untraceable history is held in an unresolved legacy bucket and reported for an explicit business-owner assignment through the migration review UI/tool. It cannot be spent, refunded, or counted as newly available primary-currency credit before assignment. Confirmation records an auditable opening journal entry, not a silent overwrite of the original data. Existing requests keep their shape; attempts to use unresolved historical credit return a specific reconciliation-required error. Compatibility cannot justify guessing the currency of money.

### Refund and credit policy (proposed implementation defaults)

A refund request reserves the remaining refundable amount under concurrency control. Its receipt becomes final only after gateway confirmation or explicit authorized offline recording. Repeated requests/webhooks cannot over-refund. Returning money does not silently reopen an issued/paid invoice; retain its historical paid fact and record the refund/credit adjustment separately. A business must issue a new adjustment/invoice to collect again. Available customer credit and account statements derive from the same journal, with no automatic double credit for a cash refund.

Track the source payment of overpayment credit and its applications. A refund may reverse still-unspent excess credit as part of the same durable operation, but cannot return funds that currently back credit already spent elsewhere. For a 1,200-paisa payment settling a 1,000-paisa invoice, if all 200 excess paisa have paid another invoice, at most 1,000 paisa are refundable until that credit application is explicitly reversed. Never create a negative customer balance or reopen another invoice implicitly. Refund reservations coordinate with concurrent credit spending. Historical payments whose credit provenance cannot be established require reconciliation before a potentially excessive refund; do not infer a safe full refund from the original payment amount alone.

## Collection modes and subscription lifecycle

`collection_mode` is additive: `invoice` by default, or explicit `automatic`. One-time invoices always use customer-initiated checkout or manual recording.

- **Invoice mode:** generate one invoice per subscription period, send a scoped portal/payment link, reconcile payment, and run configured reminders. There is no saved authorization to debit later.
- **Automatic mode:** create the supported provider plan/subscription, send the customer through Razorpay authorization, persist provider references, and activate collection only after verified mandate/subscription state. Browser success alone never marks a bill paid.
- Map Razorpay-generated invoice/payment events to the corresponding internal period invoice. A single logical period must not produce two customer charges or two internal invoices.
- Cancellation, pause, resume, and supported plan changes coordinate with Razorpay. Unconfirmed remote changes remain pending; do not claim that future charges stopped when provider outcome is unknown.
- New scheduled-change UI flows and automatic-mode changes default to the next period. Preserve existing manual-mode change-plan API behavior, including immediate proration when no new timing field is sent; add an explicit timing choice without changing that legacy default. Do not retroactively edit issued invoices. Automatic plan changes requiring new customer authorization remain pending until that authorization succeeds.
- Changing invoice → automatic requires new authorization for future periods; it does not charge historical outstanding invoices automatically. Changing automatic → invoice requires confirmation that future automatic debits are stopped. In-flight captures are still reconciled.
- Use a deterministic period identity, durable job claims, and recovery records. A crash between period advancement and invoice creation is recoverable. Multiple API/worker instances and delayed/catch-up schedules must not create duplicate periods.
- Process beyond the first 100 records with stable pagination and bounded batches; do not trade correctness for bulk throughput.

For new subscriptions, use UTC instants and an explicit anniversary anchor: month-based cycles clamp to the final day of short months while retaining the original anniversary for later periods. Preserve stored period bounds for existing subscriptions and establish the next anchor from their documented schedule; report ambiguous history rather than shifting an already billed period. In automatic mode the confirmed provider period is authoritative and must map to the same internal period identity. Money calculations retain the current truncation-to-minor-unit behavior using checked integer/rational arithmetic; invalid negative rates/quantities and overflow are errors. Provide an explicit additive tax-exemption/zero-rate override rather than changing the legacy zero-means-default interpretation or recalculating issued invoices.

## Business and customer UI

Business screens cover Overview, Products and Plans, Subscribers/Subscriptions, Invoices, Payments/Refunds, and Settings, with existing advanced features kept accessible. Break up `AppBizBilling.tsx` by responsibility; do not add a second competing billing frontend.

Guided setup covers business identity, invoice numbering/tax settings, merchant credentials, test/live readiness, callback URL, and notification preferences. Use typed forms with inline validation instead of raw JSON. Show readiness and actionable errors without exposing keys. No silent shared-account fallback.

Use subscriber search instead of requiring pasted user IDs. Provide clear currency units, invoice previews, confirmation for financially significant changes, pending/reconciliation indicators, and real errors with retry actions. Preserve an unsaved form on retry. Fetch and paginate the active view rather than loading every resource at startup.

At 360, 390, 768, and 1280 CSS pixels, primary actions and forms must work without page-wide horizontal scrolling. Use cards or contained table scrolling for dense secondary data, accessible labels, visible keyboard focus, and adequate touch targets. Test light/dark modes.

The public portal must work on mobile without a dashboard login. Customers can view their scoped invoices/subscriptions, download an invoice/receipt, pay, authorize automatic collection, and see payment-pending or failure recovery. Use branded, print-ready invoice/receipt views with PDF output, retaining the immutable issued business/customer/line-item/tax snapshot. Never expose business administration controls.

Retain existing portal links until their existing expiry. New links use strong scoped tokens with expiry, revocation, and safe storage; do not log tokens or leak them through referrers/analytics. Validate app and customer ownership on every portal action. Expired links explain how to obtain a new one without revealing another customer's information.

## Notifications and operational visibility

Persist notification intent alongside the accepted billing operation. An outbox retries admission independently of financial settlement. Record accepted, delivered, failed, or unknown states distinctly; an enqueue attempt is not proof of delivery. Reminder progress is keyed by invoice and configured step/channel, not just day offset, so two channels on the same day are not skipped.

Use the existing notification engine and app-specific channel configuration. Email, WhatsApp, and SMS obey configured preferences and channel readiness. Payment success remains valid when notification delivery fails. Avoid automatic multi-channel duplicates unless the business explicitly chose those channels. Surface failed reminders/receipts and retry safely; acknowledge that external delivery cannot be guaranteed exactly once across an unknown provider outcome.

Log app/operation/provider references and reconciliation outcomes; redact credentials, portal tokens, sensitive payloads, and payment/customer data. Monitor reconciliation backlog age, unmatched gateway events, failed reminders, renewal lag, and DLQ depth. Define alerts and measurable delivery baselines, without promising email inbox arrival within a fixed second.

## GitHub issue priorities

Primary business-billing correctness work precedes unrelated feature expansion.

1. **#67:** implement and test audit date filtering as part of billing traceability. Preserve pagination/count behavior; validate date-only and timestamp bounds, inclusive end dates, and invalid ranges.
2. **#87:** revalidate app-specific WhatsApp broadcast with regression fixtures and an explicit later provider canary. Recent merged fixes may cover parts of the old report; do not declare it fixed without evidence.
3. **#93:** benchmark notification admission, queue delay, provider latency, and fan-out under configured limits; optimize demonstrated bottlenecks affecting billing reminders. No promise of zero network latency.
4. **#114:** document actual telemetry fields, sampling, redaction, and operational setup for the completed flows.
5. **#105/#106/#109:** template categorization/search/export and external-source media are a separate UX/media workstream; preserve existing behavior during billing delivery.
6. **#88/#85:** native Slack poll semantics and wider SDK coverage are separate integration workstreams; do not imply text poll fallbacks implement interactive voting.
7. **#104/#7:** historical notification deletion and time-series storage require separate retention/scope decisions; do not delete production history during billing cleanup. Existing notification DELETE routes cancel notifications, not the requested permanent-history deletion.
8. **#122:** platform payment/credit-allocation notifications are explicitly outside the user's chosen billing scope. Do not close it as completed by subscriber billing.

Reading issues is authorized; posting comments, closing issues, and publishing rollout announcements are not part of this design.

## Proposed delivery phases and evidence

**Phase 1 — compatibility and financial foundation.** Capture representative legacy fixtures; add versioned repository operations, journal/recovery, currency-aware customer balances with legacy review, and audit date filtering. Acceptance includes concurrent manual payments/credits, currency-less API calls, unresolved mixed-currency history, explicit zero tax, and crash replay without changed historical totals or IDs.

**Phase 2 — each business's Razorpay account.** Encrypted merchant onboarding, test/live separation, checkout, signed event inbox, server-side capture checks, real refunds, and reconciliation. Acceptance includes two apps/accounts, duplicate/reordered callbacks, wrong currency/amount/account, secret rotation, and uncertain provider outcomes.

**Phase 3 — subscriptions and invoices end to end.** Both collection modes, customer authorization, deterministic renewal/invoice identities, lifecycle synchronization, adjustments, receipts, and notification outbox. Acceptance includes multiple schedulers, >100 due subscriptions, short-month/leap-year anchors, restart midway through a period, same-day multi-channel reminders, partial payments, refunds of spent/unspent overpayment credit, automatic/manual collection races, and switching collection mode without extra charges.

**Phase 4 — responsive business UI and customer portal.** Typed setup/forms, focused views, subscriber selection, invoice/PDF/receipt views, checkout/authorization, real error handling, and accessible mobile flows. Acceptance includes the specified viewport widths, permission variations, expired links, network errors, and payment-pending recovery.

**Phase 5 — related delivery issues and documentation.** Verify #87, measure/fix #93 bottlenecks, complete #114, and publish app-billing API/portal/onboarding recipes. Preserve SSE and existing custom-provider integrations. Provider canaries are identified separately from local/mock tests.

**Phase 6 — release verification and recovery preparation.** Dedicated CI with business billing enabled on disposable ES/Redis, race tests, isolated gateway fixtures, browser tests, compatibility fixtures, secret scan, migration dry-run/restore rehearsal, and a feature-gated rollout runbook. Required business tests must fail rather than silently skip when configuration is missing.

One branch, reviewable commits and phase-wise updates. Reuse existing infrastructure; no new database. Production remains untouched unless a later rollout is authorized. Deploy readers/new-schema support before writers and new schedulers; do not allow old mutation paths to run alongside new journal writers for enabled apps. Roll back UI/feature activation independently; never roll back to a writer that cannot understand newly recorded operations. Recovery continues for accepted operations even when new collection is disabled.

## Decisions left to implementation planning

Exact new endpoint/type/file names, journal indexing and bounded checkpoint layout, job batch sizes/retry limits, and PDF tooling are engineering choices. They must satisfy this design, use existing dependencies where practical, and include focused tests; they are not reasons to invent another framework.

External access needed later: merchant test-mode credentials/capabilities and a public staging callback endpoint. No customer secrets belong in the repo or the design. Record real provider test coverage separately from deterministic CI fixtures.

## Primary references

- Razorpay webhook validation, duplicate events, and ordering: https://razorpay.com/docs/webhooks/validate-test
- Razorpay plan-based automatic payments and provider-generated invoices: https://razorpay.com/docs/payments/subscriptions/
- Existing implementation: `internal/usecases/services/bizbilling_lifecycle_service.go`, `bizbilling_scheduler.go`, `bizbilling_notifier.go`; `internal/infrastructure/bizbillingrepo`; `internal/interfaces/http/handlers/biz_portal_handler.go`; `ui/src/components/AppBizBilling.tsx`.
- Existing business-billing E2E suite: `tests/integration/biz_billing_e2e_test.go`.

## Builder review

A fresh read-only builder review identified three decisions: automatic collection versus manual adjustments; the currency of legacy credit resources and future currency-less calls; and refunding payments whose overpayment credit has been spent. All three now have explicit proposed defaults in the sections above. These are conservative engineering proposals for user approval, not business policies claimed to have been selected by the user. No new implementation or provider test was performed as part of this design review.
