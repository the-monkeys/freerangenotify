# Billing your customers

Use **Applications > your application > Billing Setup** to manage your own products, plans, customer subscriptions, invoices and payments. Your customers are the subscribers in that application. Notification delivery credits are managed separately under [Pricing & Credits](/docs/pricing).

Business billing must be enabled on your installation. Existing applications keep their current billing lifecycle until an explicit, reviewed upgrade. Installing a release does not activate new payment collection or change historical invoices.

## Connect your backend

Create an application in the dashboard, open **Integration**, and copy its application key. Keep the key on your backend. The key selects the application; an application ID alone does not authorize billing requests. See [Authentication](/docs/authentication).

Use `Authorization: Bearer YOUR_APP_KEY` or `X-API-Key: YOUR_APP_KEY`. All paths below start at your installation's API origin. Use the [API Reference](/docs/api) for request schemas and available routes.

Create a customer with `POST /v1/users`. Save the returned `data.user_id` and use it as the billing `user_id`. See [Users](/docs/users) for profile create, read, update and delete operations.

Amounts are integers in the currency's minor units. For INR, `10000` means INR 100.00. Send currency codes such as `INR`; never send rupee amounts as decimal values in a `*_paisa` field.

## Create and issue an invoice

Create a draft with `POST /v1/biz/invoices`:

```json
{
  "user_id": "CUSTOMER_USER_ID",
  "currency": "INR",
  "payment_terms": "net_30",
  "line_items": [
    {
      "description": "Monthly service",
      "quantity": 1,
      "unit_price": 10000,
      "tax_rate": 18
    }
  ]
}
```

Read the returned invoice total, tax and amount due; do not calculate the payable amount from the line price alone. Tax settings can change whether prices include tax. Historically, `tax_rate: 0` uses the configured default; it does not declare an exemption. On an upgraded invoice, use `tax_rate_override: 0` or `tax_exempt: true` on a line to declare explicit zero tax. Legacy invoices and estimate requests reject these new override fields instead of silently ignoring them.

Use `PUT /v1/biz/invoices/INVOICE_ID` to edit an unissued draft, then `POST /v1/biz/invoices/INVOICE_ID/issue` to issue it. Use `GET /v1/biz/invoices/INVOICE_ID` to read its saved state. Issued amounts are historical facts; changes require an explicit supported adjustment.

To check the calculation first, send the same creation payload to `POST /v1/biz/invoices/preview`. Preview does not save a draft or consume an invoice number. Its result uses the current customer and tax settings; creating the draft saves those calculated amounts.

After issue, `GET /v1/biz/invoices/INVOICE_ID/document` returns the original business identity, customer identity, lines, tax and invoice total alongside current settlement amounts and payment references. Editing a customer or business profile does not change that issued document. Historical invoices without a saved issue snapshot require review for this new document view; the existing invoice endpoint still returns their original saved records. No historical identity or amount is reconstructed automatically.

Razorpay is optional for offline billing. You can issue invoices and record money received in cash, direct UPI, bank transfer or cheque without connecting a payment gateway. Staff must confirm receipt independently before recording an offline payment.

## Record a payment or refund

Record money already received offline with `POST /v1/biz/invoices/INVOICE_ID/payments`:

```json
{
  "method": "bank_transfer",
  "amount_paisa": 11800,
  "reference": "BANK-STATEMENT-REFERENCE",
  "notes": "Received and confirmed by staff"
}
```

Supported amounts depend on the invoice's current due balance. `amount_paisa: 0` requests the full amount due. A supplied TDS amount also settles that part of the invoice; TDS is not extra customer credit.

Use `method: "cash"` or `method: "upi"` for those offline methods. The optional `reference` is a staff-recorded receipt or transaction reference; it is not provider verification. A partial payment leaves the remaining amount due. Credit notes, retainers and write-offs remain separate from cash received.

For an upgraded application, excess cash becomes customer credit with its payment source retained. Record an authorized offline refund through `POST /v1/biz/payments/PAYMENT_ID/refund`, supplying `amount_paisa` and `reason`. This records a return already made offline; it does not send a bank transfer. A cash refund remains separate from the invoice's historical paid state.

Do not use manual payment requests to assert a gateway capture or gateway refund. Provider money must be verified through its original payment-account binding. An upgraded application's new online collection requires its own supported merchant connection. Never substitute the platform's payment account or move old orders to a newly connected account.

## Optional online collection with Razorpay

Cash, direct UPI and bank-transfer recording work without a gateway. Online collection uses **each business's own Razorpay account**; funds do not pass through FreeRangeNotify's notification-credit account. New merchant collection also requires the installation's collection feature flag and a reviewed lifecycle upgrade. A legacy application continues using its existing integration until that upgrade.

Manage the connection from your backend with `GET`, `PUT` or `DELETE /v1/biz/gateway-connections/test` (or `/live`). Writes require billing-management permission. Read the current snapshot first and send its `revision` as `expected_revision`. Connecting takes `account_id`, `key_id`, `key_secret` and `webhook_secret`; secrets are write-only and encrypted with the installation's connection-encryption key. Keep encryption keys available while retained payments need them. Use a test key only in the test slot and a live key only in the live slot.

Authentication checks the API credentials read-only. The account is not marked verified until a signed callback and a server-side payment lookup agree. This does not enable automatic renewals; those require separate authorization and capabilities. After connecting, configure your API origin plus `/v1/webhooks/biz/razorpay/CONNECTION_ID` on your Razorpay account, using the returned `connection_id` and the same webhook secret. This route is separate from platform-credit and notification webhooks. Callback acceptance means the event was stored, not that an invoice is paid.

For an upgraded invoice, `gateway_mode` is pinned at creation; omitted means `live`. Use `gateway_mode: "test"` explicitly for sandbox invoices. Test invoice numbers start with `TEST-`; test funds have separate balances and cannot settle live invoices. Default customer statements and real-money totals exclude sandbox activity. Sandbox transactions do not represent actual customer money.

A customer portal payment uses `POST /v1/portal/TOKEN/invoices/INVOICE_ID/pay`; the token must belong to the invoice's customer. The response supplies `key_id`, `gateway_order_id`, payable minor units, currency, state and `operation_id`. Use the returned public key for checkout. Submit `razorpay_order_id`, `razorpay_payment_id` and `razorpay_signature` to `POST /v1/portal/TOKEN/invoices/INVOICE_ID/verify-payment`. The server verifies the signature and independently fetches captured payment facts. An authorized payment alone is not paid. Keep portal tokens private.

Gateway refunds use the same payment refund route above with an `Idempotency-Key` header. The request first reserves refundable money, then contacts the original merchant account. A pending or unknown result keeps its reservation; it does not increase `refunded_paisa`. Only a matching provider-confirmed processed refund settles it. A definitive failure releases its holds. A spent overpayment cannot be refunded again as customer credit, and a refund does not reopen the paid invoice. `amount_paisa: 0` requests the remaining refundable amount. Provider processing does not promise immediate bank credit.

For a timeout or `BILLING_GATEWAY_PENDING`, inspect `GET /v1/biz/gateway-effects/OPERATION_ID` and retry the identical request with the **same key**. Recovery looks up the original attempt rather than sending another refund. `GET /v1/biz/gateway-events?state=unmatched&limit=50` lists stored events needing investigation; use `GET /v1/biz/gateway-events/EVENT_ID` for one event. A valid callback with no matching local refund remains visible for reconciliation and never silently changes a balance. Historical gateway payments without a provable account binding require explicit review.

Disconnecting a merchant stops new collection through that slot; it retains encrypted revisions needed to reconcile old orders and refunds. Rotation may use a newer verified revision of the **same account** for API recovery. It never moves an old payment to a replacement account. Do not delete retained provider credentials or local payment history while outcomes are unresolved.

## Customer credit

For a reviewed, upgraded application:

- `GET /v1/biz/users/USER_ID/balance?currency=INR` returns completed available and reserved credit for that currency.
- `POST /v1/biz/users/USER_ID/balance` accepts an adjustment such as `{"amount_paisa":500,"reason":"Account correction"}`. This adds INR 5.00; a negative adjustment subtracts available credit.
- `POST /v1/biz/invoices/INVOICE_ID/apply-credit` spends available credit against that invoice.

Omitting currency on a new credit request uses the primary currency reviewed for the application. Choosing another currency never converts or relabels existing credit. Historical money whose currency or payment source is unresolved must be reconciled before use.

Reserved money is held for an unfinished action and is not available to spend. A confirmed cancellation can release a reservation. An unknown provider outcome must be reconciled before it can be released; waiting alone is not proof that no payment occurred.

## Retry an action safely

For an upgraded application, send a fresh `Idempotency-Key` for each intended financial action and reuse that exact key and payload when retrying it. Keep keys at most 200 characters. Use different keys for creating a draft, issuing it, recording a payment and refunding it.

A completed keyed retry returns that action's saved result. Reusing a key with a changed request returns a conflict. Without a key, each request represents a new action; repeated calls cannot be assumed to be duplicates. For customer creation with opening credit, retain the customer ID and creation key so an interrupted request can be recovered.

A temporary failure can include `operation_id` and `operation_state`. Keep them. Read `GET /v1/biz/operations/OPERATION_ID`; if needed, call `POST /v1/biz/operations/OPERATION_ID/resume` without changing the original action. Do not create another payment merely because a request timed out.

`409` can mean a concurrent change, a reused key with different input, or an invoice locked by automatic collection. `503` with a reconciliation or upgrade code means the saved state is not ready to use; check the operation and retry instructions. A partially saved balance is never a confirmed spendable balance.

These journal and operation guarantees apply after an application's reviewed upgrade. Legacy applications retain their existing API behavior until that cutover.


## Review an existing application's upgrade

This release adds the financial foundation. Existing applications stay on their current lifecycle. New automatic renewals and merchant collection require their later supported phases; an upgrade does not silently enable them.

1. Send `POST /v1/biz/lifecycle/prepare` with `{"primary_currency":"INR"}`. This is a read-only inventory by default. It returns a report ID, source revision hashes, counts, readiness and unresolved issues.
2. To save the reviewed inventory, send the same request with `"dry_run":false` and `"expected_report_id":"REPORT_ID"`. Saving a review does not change original balances or activate the new lifecycle.
3. A historical currency issue can be resolved through `POST /v1/biz/lifecycle/reviews/REPORT_ID/resolve` using its `issue_id`, `expected_source_hash`, assigned `currency` and a `reason`. The authenticated actor is recorded. Review evidence must support the chosen currency; choosing a primary currency alone does not assign old money to it.
4. Prepare the same inventory again after resolutions. If a source changed, review a new report. Missing references, inconsistent totals, old gateway bindings and unsupported subscription transitions block activation.
5. An operator must first drain older API and worker instances. The operator can then run the reviewed migration command below. A browser request cannot attest that rollout on the operator's behalf.

Read-only operator preview:

```sh
migrate biz-billing-v2 --app-id APP_ID --currency INR
```

Explicit apply after reviewing the report and draining older writers:

```sh
migrate biz-billing-v2 --app-id APP_ID --currency INR --apply --report-id REPORT_ID --writers-upgraded
```

A cutover temporarily blocks new money writes for that application, checks the original revisions again, and records opening journal facts without editing the original records. Repeating a completed migration uses the same opening identities. A partially completed cutover remains fenced until its saved report is resumed; do not manually clear writer permits or reservations based on their age.

Upgraded dashboard money reports use the application's primary currency, exact minor units and completed financial revisions. Cash collected, refunds and invoice debt remain separate. Monthly recurring revenue is labelled as a current-plan estimate until subscription contract snapshots are available. Expense conversion reuses a saved conversion for the same source batch; converted expenses cannot be edited independently.

`GET /v1/biz/lifecycle` shows the active lifecycle and installation readiness. `GET /v1/biz/lifecycle/reviews` lists reviewed inventories. The installation flag `FREERANGE_BIZ_BILLING_NEW_COLLECTION_ENABLED` defaults to `false` and blocks new lifecycle activation. Disabling it does not erase accepted financial work or prevent recovery. It also does not cancel an existing provider-authorized subscription; stopping provider collection requires a confirmed provider pause or cancellation.


## Recover an abandoned billing writer

If an interrupted process leaves a billing writer permit behind, an operator can inspect its `writer_id` and `epoch` in the read-only migration inventory. First recover any accepted financial operations, stop the selected writer origins, and confirm their outstanding storage requests have drained. Age or an expired permit does not prove those requests stopped.

After reviewing a fresh inventory, use its report ID and the exact permit epoch:

```sh
migrate biz-billing-v2 --app-id APP_ID --currency INR --apply --report-id REPORT_ID --recover-writer WRITER_ID --expected-writer-epoch 0 --writer-drained --reviewer OPERATOR_ID --reason "Origin stopped; storage requests drained"
```

Select multiple abandoned writers with comma-separated IDs. Other registered writers must finish first. Recovery fences the application, verifies source revisions and pending operations, records the decision, and removes only the reviewed permits. It preserves money and invalidates the removed writers' old epoch. The same request can be retried after an uncertain acknowledgement. Recovery stays available when new collection is disabled. Do not clear permits directly in the database.
