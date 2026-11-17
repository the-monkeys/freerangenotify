# Business billing rollout and recovery

This runbook concerns each application's own product and subscriber billing. Notification delivery credits and platform subscription billing are separate systems. No production release, migration, live charge or refund is authorized by this document.

## Release gates

- Complete the required `business-billing` CI job: owned Elasticsearch/Redis and HTTP fixtures, all required cases executed with zero required skips, migration and restore rehearsals, Linux race checks, mobile browser checks, generated API contracts, SDK compatibility and UI production build.
- Review the whole branch against `94acfe1`; repair significant confirmed findings with focused regressions.
- Scan the outgoing history and committed tree with a checksum-verified secret scanner. Never include merchant credentials, private portal links, production environment files or raw provider bodies in review artifacts.
- Rehearse the demonstrated manual payment flow in the actual local dashboard. Automated provider fixtures do not replace a business-owned Razorpay test-account checkout, signed callback and refund rehearsal.
- Keep the broader Templates, Subscribers, Notifications, SSE and notification-credit journeys in regression coverage. Billing-only checks are not evidence that every screen works.

CI builds images after the required billing, messaging and documentation checks. Publishing is restricted to pushes on `main`; manual verification on a feature branch does not publish images. A passing job does not authorize production deployment.

## Data and account inventory

Before a separately authorized production cutover, record the deployment version, application and business identity, primary currency, existing merchant bindings and open operations. Inventory the actual indices and aliases used by the deployment, including applications, users, products, plans, subscriptions, invoices, payments, credit notes, retainers, balances, credit lots/applications, refund reservations, operation streams, immutable journal, lifecycle and migration review/resolution records, gateway connections/events/effects, portal token records and notification outbox.

Record per-index counts, per-ID canonical source checksums, document IDs, invoice numbers, currencies and financial totals. Retain unknown fields. An ambiguous legacy balance must be reviewed and assigned through the supported resolution workflow; never guess currency from the business's current preference. Changing a form default does not convert existing money.

The backup inventory also covers Redis durability/accepted work, file volumes, application configuration and the encryption keys needed to recover merchant credentials and encrypted delivery links. Store key backups separately with restricted access. A database snapshot without its required encryption keys is not a complete recovery plan.

## Backup and restore gate

1. Create a supported Elasticsearch snapshot in an approved repository and verify it completed successfully. Preserve existing snapshots and volumes.
2. Restore into a separate, owned recovery environment with renamed indices or a separate cluster. Do not overwrite production indices for a rehearsal.
3. Compare counts, every captured ID/source checksum, invoice numbering, currency totals, journal sequence/projection checkpoints and account bindings. Re-run reads and scoped financial reconciliation against the restored data.
4. Confirm the recovery environment cannot send real notifications, accept customer payments or replay provider effects. Use synthetic callbacks and provider fixtures.
5. Record the snapshot ID, time, source versions, restored results and operator approval. Local synthetic rehearsal evidence is not proof that a production backup has been taken or restored.

For local automation, use:

```sh
node scripts/biz-billing-fixtures.mjs run --stack -- node scripts/verify-biz-billing.mjs
```

The fixture runner creates one uniquely owned disposable project, binds stores to loopback, removes customer environment variables and generates fixture credentials. It refuses to run alongside an existing FreeRangeNotify project. Stop the saved walkthrough project without deleting its volumes first, and restore that same project afterward. Do not globally prune Docker resources or touch other projects.

The migration rehearsal registers an owned filesystem snapshot repository, restores into new `frn_rehearsal_*` indices, verifies original and restored source hashes, and runs twice with fresh synthetic IDs in the same disposable cluster. Its cleanup is restricted to those temporary indices/snapshots and the disposable project's labelled resources. It never restores over the saved walkthrough or production data.

## Reader-first deployment and per-app cutover

1. Deploy compatible readers and recoverable writers before activating any application. Confirm every API, worker, scheduler and operator process that can write billing money uses the upgraded writer fence.
2. Review accepted and unknown operations. Drain old unaware writers and their outstanding storage requests. An old lease or timestamp does not prove a writer stopped.
3. Keep new collection disabled until deployment/account readiness is confirmed. Cash, staff-confirmed direct UPI and bank receipts remain available through the supported legacy/manual path.
4. Run the supported migration inventory/dry-run for one selected application. Review source hashes, ambiguous balances, unknown gateway bindings, open authorizations, currency and subscription dates.
5. Record explicit balance/currency decisions through the reviewed resolution API/CLI. Re-run the inventory after any source or decision changes. A stale report must not authorize activation.
6. Prepare against the exact reviewed report. Writer readiness is a deployment/operator attestation, not a dashboard checkbox supplied by a customer.
7. Activate that application through the supported command, then verify balances, opening facts, IDs, invoice numbers, dates and source hashes. Interrupted activation must resume the accepted operation rather than repeat financial effects.
8. Verify manual collection first. Enable online admission only for a business with its own verified account and supported capabilities. Never replace the original account binding of old orders or mandates.

The dry-run does not mutate history. Migration adds reviewed sidecars/opening facts and lifecycle checkpoints; it does not rewrite old invoice totals, dates or unknown fields. Leave unreviewed applications on the legacy lifecycle. Never bulk-flip lifecycle fields directly in Elasticsearch.

## Stop new collection safely

Set `FREERANGE_BIZ_BILLING_NEW_COLLECTION_ENABLED=false` to stop new FreeRangeNotify-initiated collection. Keep callbacks, accepted operations, reconciliation and recovery workers running. This flag does not cancel a mandate already authorized at Razorpay. Existing provider mandates can continue charging until a separately confirmed provider pause/cancel completes.

Do not disable the whole business module or revert to unaware old writers while accepted v2 operations exist. Do not delete operations, reservations, journal entries, callbacks or merchant bindings to remove a pending state. Unknown external results require lookup/reconciliation against the original account before any replacement action.

## Financial and messaging checks

- An issued invoice is not a payment. Compare total, actual cash paid, credit applied, due and cash refunded separately, in the recorded currency.
- Repeating the same payment/refund key returns its original operation/result. An interrupted response is not permission to submit a new key.
- Partial payments retain the unpaid remainder. Source-tracked overpayment credit cannot be refunded after it has funded another invoice without an explicit supported adjustment.
- After closing checkout, a changed invoice balance may create a new retained order only when the provider positively identifies the old order as never attempted and unpaid. Missing state, attempted payments and lookup failures keep collection pending. The old order is not canceled remotely: any later genuine capture is still reconciled once, with excess tracked as customer credit.
- A signed recurring callback can establish account proof for an exact replacement key by reading the retained mandate and matching its original intent. Recovery preserves the original account, mode and credential binding; it never moves old work to another merchant.
- Sandbox subscriptions and credit notes stay outside live customer balances and business revenue reports. Invoice resends use durable, scoped delivery records; sandbox resends cannot contact customers.
- Signed callbacks must match the original application/account/order, amount and currency. Changing a replay header must not settle the same signed payment twice.
- Automatic collection requires supported pricing and customer consent. Do not manually collect or change an invoice whose automatic collection is still owned by an active mandate.
- Payment success survives receipt admission failure. Inspect outbox states separately: pending, accepted, delivered, failed and unknown mean different things. Never blindly retry an uncertain message.
- WhatsApp requires the correct application's Meta/Twilio connection, a valid recipient and an approved provider template or valid conversation window. A business billing template is not provider approval. Per-business SMTP credentials remain isolated from the system provider.
- Portal links are credentials: never put them in logs, traces, artifacts or public tickets. New links are digest-stored; existing expiry remains intact. Expiry blocks reads but must not prevent a valid callback from settling an already accepted payment.

## Recovery after a storage or provider incident

Freeze new admissions and stop all relevant writer origins before considering a restore. Reconcile provider money accepted after the backup: a successful gateway capture/refund cannot safely be erased by restoring an earlier database state. Verify the provider/account records, durable inbox and accepted operations, then plan recovery under an explicitly approved incident procedure.

Resume accepted operations from their saved checkpoints. Do not reset invoice numbers, issue destructive down migrations or roll back immutable financial history. Keep new writers and original merchant references available until reconciliation completes. Re-enable admission only after currency/account/journal invariants and a small canary pass.

## Evidence and remaining acceptance

The authoritative progress and exact local results live in `docs/superpowers/plans/2026-11-11-business-billing-end-to-end.md`. Distinguish completed automated local/CI checks, the joint dashboard smoke test, real Razorpay test-account acceptance, and separately authorized production backup/cutover. Do not claim zero future failures from any one of these gates.
