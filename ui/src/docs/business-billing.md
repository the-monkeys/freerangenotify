# Billing your customers

For a short dashboard walkthrough, see [receiving payments](/docs/business-billing-payments) and [the customer portal](/docs/customer-billing-portal). Operators can also review [monitoring and privacy](/docs/observability).

Use **Applications > your application > Billing Setup** to manage your own products, plans, customer subscriptions, invoices and payments. Your customers are the subscribers in that application. Notification delivery credits are managed separately under [Pricing & Credits](/docs/pricing).

Business billing must be enabled on your installation. Existing applications keep their current billing lifecycle until an explicit, reviewed upgrade. Installing a release does not activate new payment collection or change historical invoices.

## Set up billing in the dashboard

1. Open your application and choose **Billing Setup > Setup**. This area bills your own customers.
2. Open **Business settings**. Set the business name, contact details, tax, invoice prefixes and payment terms.
3. Choose the **Default currency** for new forms. New setups start with INR; USD, EUR and GBP are also available. Existing configured choices, catalog prices, issued invoices and payments retain their original currencies and amounts. Add-ons inherit their parent plan's currency. Currency conversion is separate future work.
4. Open **Catalog** and create a product, then a plan with its price and frequency. Use search and pagination to find existing offerings. Archiving an offering retains its records.
5. For online payments, select **Test** or **Live** and connect that business's own Razorpay account in Business settings. Credentials are write-only. Configure signed callbacks at the saved connection's webhook URL. API authentication, verified account linkage and automatic-payment readiness are separate checks.

Cash, direct UPI and bank payments can be recorded without connecting Razorpay. A staff-entered receipt records an externally confirmed payment; it does not verify a UPI transfer automatically. Viewer access is read-only. Owners, admins and editors retain the billing-write permission enforced by the API.

If another administrator changes a payment account while you edit it, refresh its saved status and review your inputs before explicitly retrying. Unsaved credential fields stay in memory during a failed save and clear after a successful save. New collection can be disconnected while original payment recovery records are retained.

Existing applications require a reviewed upgrade before the new recoverable lifecycle is activated. Setup shows unresolved balances and retained account references. A deployment administrator must attest service readiness; a dashboard checkbox cannot substitute for that check. Review failures remain visible and never activate collection automatically.

The form default is available through `GET /v1/biz/config/preferences`. To change it, send `PUT /v1/biz/config/preferences` with the flat body `{"default_currency":"INR"}`. API integrations continue to select currency explicitly in their requests. This setting does not convert money or change existing financial account scopes.

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

## Choose how a subscription collects money

Missing `collection_mode` means `invoice`. Existing subscriptions do not acquire a payment mandate when the application is upgraded. Invoice collection works without Razorpay: issue the bill and record confirmed cash, direct UPI or bank payments, or let the customer pay online if a gateway is connected.

For an upgraded subscription, choose a collection method with `POST /v1/biz/subscriptions/SUBSCRIPTION_ID/collection-mode`. Send an `Idempotency-Key` and reuse it for an identical retry:

```json
{
  "collection_mode": "automatic",
  "automatic_cycles": 12
}
```

The business's choice does not authorize a customer mandate. Request authorization through `POST /v1/portal/TOKEN/subscriptions/SUBSCRIPTION_ID/authorize`, with an `Idempotency-Key` and `{"mode":"test"}` or `{"mode":"live"}` matching that subscription. The token must belong to its customer. Use the returned public `key_id` and `gateway_subscription_id` with the Razorpay subscription checkout. After checkout, send `razorpay_subscription_id`, `razorpay_payment_id` and `razorpay_signature` to `POST /v1/portal/TOKEN/subscriptions/SUBSCRIPTION_ID/verify-authorization`. The server verifies the saved business key and fetches provider state. Browser success and mandate authorization do not record an invoice payment.

The initial automatic-collection support is for fixed INR plans with weekly, monthly, quarterly, half-yearly or yearly cycles. It includes the saved quantity and tax calculation. Choose 1–120 billing cycles explicitly. Tiered or usage-based prices, custom cycles, contracts, setup fees and add-ons continue to use invoice collection. Unsupported combinations are rejected before a provider plan or mandate is created. No currency conversion is performed.

For a subscription already in progress, authorization applies to future periods after its existing period ends; outstanding invoices are not charged automatically. The authorization response shows its amount, currency, start, cycle count and expiry. Provider timestamps use whole seconds, so a new future start can be rounded up by less than one second without changing a historical period. New automatic subscriptions wait for authorization and their first provider-confirmed billing period; they do not generate a manual first bill or wait a free month before the first charge. Trial subscriptions start their first paid period after the trial. Invoice collection creates its own unpaid bills, with recovery that keeps one invoice per billing period. Neither a renewal nor authorization proves payment.

To return to invoice collection, send `{"collection_mode":"invoice"}` with a new key. If an authorization or cancellation outcome is unknown, the subscription remains pending until lookup and confirmed cancellation establish that future automatic debits stopped. A cancellation retry stays bound to its original mandate; it cannot cancel a later authorization. Keep the original request key and operation reference instead of creating another mandate. Use `GET /v1/biz/gateway-effects/OPERATION_ID` to inspect saved external work.

Payment-account readiness distinguishes authenticated API credentials, verified account linkage and automatic capability. Account linkage requires a signed callback checked against server-fetched facts. Configure the callback for that connection and retain access to the original account for existing mandates. Disconnecting the current gateway slot prevents new setup; it does not cancel an existing provider subscription. See the [Razorpay subscription API](https://razorpay.com/docs/api/payments/subscriptions/) for the hosted authorization flow.

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

## Invoice and receipt delivery

For upgraded applications, invoice notices, payment receipts and overdue reminders have their own saved delivery progress. A failed notice never reverses a recorded payment. Notification sending still requires the application's configured channel, recipient readiness and notification credits. Businesses recording cash or direct UPI payments do not need a Razorpay account.

Use `GET /v1/biz/notification-deliveries?limit=20&offset=0` to list progress for the authenticated application. `GET /v1/biz/notification-deliveries/DELIVERY_ID` reads one item. `pending` means admission is waiting or retrying, `accepted` means the notification pipeline accepted the message, and `delivered` requires an observed delivery/read result. Provider acceptance alone is not recipient delivery. `failed` is a known stopped attempt; `unknown` needs reconciliation and must not be treated as delivered.

After fixing a known admission problem, an authorized billing manager can use `POST /v1/biz/notification-deliveries/DELIVERY_ID/retry`. This retries notification admission only; it cannot charge or refund a customer. Accepted, delivered, uncertain and sandbox-suppressed notices cannot be resent through this action. Uncertain admissions are looked up using their saved notification identity before any further action. Automatic admission retries are bounded; unknown provider outcomes remain visible.

New delivery processing does not replay historical invoices or legacy reminder markers. Sandbox financial actions suppress customer notices by default. Immutable billing journal facts contain document references rather than bearer links. A retryable link is encrypted in the delivery record and omitted from delivery API responses. The ordinary notification engine can retain the rendered message containing that link under its existing notification retention policy; this does not mean every copy of a delivered link is hashed or encrypted.

## Retry an action safely

For an upgraded application, send a fresh `Idempotency-Key` for each intended financial action and reuse that exact key and payload when retrying it. Keep keys at most 200 characters. Use different keys for creating a draft, issuing it, recording a payment and refunding it.

A completed keyed retry returns that action's saved result. Reusing a key with a changed request returns a conflict. Without a key, each request represents a new action; repeated calls cannot be assumed to be duplicates. For customer creation with opening credit, retain the customer ID and creation key so an interrupted request can be recovered.

A temporary failure can include `operation_id` and `operation_state`. Keep them. Read `GET /v1/biz/operations/OPERATION_ID`; if needed, call `POST /v1/biz/operations/OPERATION_ID/resume` without changing the original action. Do not create another payment merely because a request timed out.

`409` can mean a concurrent change, a reused key with different input, or an invoice locked by automatic collection. `503` with a reconciliation or upgrade code means the saved state is not ready to use; check the operation and retry instructions. A partially saved balance is never a confirmed spendable balance.

These journal and operation guarantees apply after an application's reviewed upgrade. Legacy applications retain their existing API behavior until that cutover.


## Renewals and subscription changes

Keep an `Idempotency-Key` when creating or changing a subscription. For reviewed applications, creation and the first manual invoice recover together; trial conversion retains the accepted price, setup fee and tax. Existing subscription IDs and stored billing dates are preserved. Short months retain the original anniversary rather than drifting permanently to February's date.

`POST /v1/biz/subscriptions/SUBSCRIPTION_ID/change-plan` keeps immediate manual proration when `change_timing` is omitted. Send `{"plan_id":"PLAN_ID","change_timing":"next_period"}` to freeze a change for the next billing period instead. Issued invoices are not rewritten. Changing currency requires a separate subscription. Stop and confirm an existing automatic mandate before changing its price, then request fresh customer authorization for automatic collection.

Automatic pause/resume waits for provider confirmation. Pause is available once the provider mandate is active. A future authorization can be stopped through invoice collection without creating a pending pause. A timeout remains pending and is looked up rather than blindly submitted again. Cancelling renewal first stops the automatic mandate; period-end cancellation keeps service access until the already stored period end. It does not create a refund. Re-enabling automatic billing requires new consent. Cash and direct UPI payments remain available with invoice collection.

## Review an existing application's upgrade

Existing applications stay on their current lifecycle until their sources have been reviewed and an operator explicitly activates the new lifecycle. Merchant collection also requires enabled installation settings, the business's own connected account and customer authorization; an upgrade does not silently enable automatic charges.

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

### Dashboard invoice and payment workflow

Under **Business billing → Invoices**, choose **New invoice**, search an existing subscriber by name or email, and add line items. Prices are entered in major currency units with at most two decimal places; requests use integer minor units. Leave a tax override blank to use business settings; explicit `0` means zero tax. Preview the server-calculated totals, save a draft, then issue it. Issuing does not collect money.

**Amount paid** shows saved payments. Credits, amount due and refunds are shown separately. For cash, direct UPI, bank transfer or cheque, choose **Record payment** only after receiving the money, enter a reference, and confirm receipt. Automatically collected invoices disable manual payment recording. A business can use this path without connecting Razorpay.

Subscriptions use the same searchable subscriber picker. Choose invoice collection or automatic collection. Automatic collection requires a supported fixed INR recurring plan, a ready merchant account and customer authorization; creating the subscription is not payment confirmation. Plan/quantity changes explicitly choose immediately or next billing period.

If a monetary response is interrupted, **Retry saved request** reuses the same details and request key. If an operation reference was returned, **Check saved action** resumes the saved operation. Pending actions remain available while navigating this dashboard session, scoped to the business and signed-in administrator. Do not start a replacement payment after closing/reloading the browser; first check the invoice and payments, because unfinished request details are intentionally not stored in browser storage.

Use **Payments** to record a confirmed refund. Spent overpayment credit can prevent refunding that amount; the original payment remains visible. **Messages / receipts** shows delivery independently from financial success. A message accepted for sending is not yet delivered; an uncertain result requires review and cannot be blindly retried.

Reports identify their reporting currency. Upgraded reports use the business primary currency; invoices and payments retain their own currencies. No exchange-rate conversion is performed.

### Customer billing portal

Create a customer link from the customer's billing tools, or call `POST /v1/biz/users/{user_id}/portal-link` from your backend with the application API key. The response contains `url`, `expires_at`, and an additive `token_id`. Treat the URL as a private credential. New links are stored as SHA-256 digests; existing links retain their original expiry. Revoke a link with `POST /v1/biz/portal-tokens/{token_id}/revoke`. Revocation does not delete invoices or payments.

The customer opens the link without a dashboard login. They can view invoices, saved receipts and subscriptions. An invoice payment uses the business's connected account; a recurring authorization requires the customer's explicit consent. The server checks provider signatures and captured amounts before confirming payment. A completed internal action alone is not proof of payment or authorization. Pending results show a status-check action; do not ask the customer to pay again while confirmation is outstanding.

**Print / Save as PDF** prints an isolated copy of the issued document, so the private portal URL is not copied into browser print headers. Issued business/customer details, prices, taxes and references come from the saved invoice snapshot. Refunds are shown separately from the original received payment.

The public portal transport sends no dashboard Authorization/API-key headers or cookies. Access is scoped to the link's application and customer, rate-limited, and returned with `Cache-Control: no-store` and `Referrer-Policy: no-referrer`. Its API and initial page requests do not log raw bearer URLs. A link that expires while checkout is open cannot regain access; a valid signed provider callback can still settle its payment independently.

Public customer endpoints (the token is the credential):

- `GET /v1/portal/{token}/invoices?limit=20&offset=0` and `GET /v1/portal/{token}/invoices/{id}`. The invoice list retains its historical default of 50 if `limit` is omitted.
- `GET /v1/portal/{token}/invoices/{id}/document` for the issued invoice and recorded receipts; older invoices without saved issue-time identity need historical review.
- `POST /v1/portal/{token}/invoices/{id}/pay` to obtain public checkout parameters, and `POST .../verify-payment` to verify the SDK result. Reuse one `Idempotency-Key` for an interrupted checkout request.
- `GET /v1/portal/{token}/subscriptions` and `GET .../subscriptions/{id}`.
- `POST /v1/portal/{token}/subscriptions/{id}/authorize` and `POST .../verify-authorization` for a business-selected automatic subscription. Authorization does not pay arrears.
- `GET /v1/portal/{token}/operations/{id}` for a customer-owned financial/provider status, and `GET /v1/portal/{token}/payments/{id}/receipt` for a customer-owned payment receipt.

Local automated checkout tests use deterministic provider fixtures. A real Razorpay test-mode SDK and signed-callback rehearsal must be completed with the business's test account before production rollout.

## Executable configuration examples

These examples use `BASE_URL` ending in `/v1` and a backend application key. The saved default only affects new forms. Configure an approved provider template before enabling WhatsApp reminders; a billing template is not vendor approval.

### Default currency

<!-- api-example:biz-default-currency:response -->
```json
{
  "success": true,
  "data": {
    "app_id": "${APP_ID}",
    "config_type": "preferences",
    "data": {
      "default_currency": "INR"
    }
  }
}
```

<!-- api-example:biz-default-currency:curl -->
```sh
curl -X GET "$BASE_URL/biz/config/preferences" -H "Authorization: Bearer $API_KEY"
```

### Select currency

<!-- api-example:biz-select-currency:request -->
```json
{
  "default_currency": "USD"
}
```

<!-- api-example:biz-select-currency:response -->
```json
{
  "success": true,
  "data": {
    "app_id": "${APP_ID}",
    "config_type": "preferences",
    "data": {
      "default_currency": "USD"
    }
  }
}
```

<!-- api-example:biz-select-currency:curl -->
```sh
curl -X PUT "$BASE_URL/biz/config/preferences" -H "Authorization: Bearer $API_KEY" -H "Content-Type: application/json" --data-binary @- <<JSON
{
  "default_currency": "USD"
}
JSON
```

### Saved currency

<!-- api-example:biz-saved-currency:response -->
```json
{
  "success": true,
  "data": {
    "app_id": "${APP_ID}",
    "config_type": "preferences",
    "data": {
      "default_currency": "USD"
    }
  }
}
```

<!-- api-example:biz-saved-currency:curl -->
```sh
curl -X GET "$BASE_URL/biz/config/preferences" -H "Authorization: Bearer $API_KEY"
```

### Manual dunning

<!-- api-example:biz-manual-dunning:request -->
```json
{
  "enabled": true,
  "steps": [
    {
      "day_offset": 1,
      "channel": "whatsapp",
      "template": "biz_reminder_whatsapp"
    }
  ]
}
```

<!-- api-example:biz-manual-dunning:response -->
```json
{
  "success": true,
  "data": {
    "app_id": "${APP_ID}",
    "config_type": "dunning",
    "data": {
      "enabled": true,
      "steps": [
        {
          "day_offset": 1,
          "channel": "whatsapp",
          "template": "biz_reminder_whatsapp"
        }
      ]
    }
  }
}
```

<!-- api-example:biz-manual-dunning:curl -->
```sh
curl -X PUT "$BASE_URL/biz/config/dunning" -H "Authorization: Bearer $API_KEY" -H "Content-Type: application/json" --data-binary @- <<JSON
{
  "enabled": true,
  "steps": [
    {
      "day_offset": 1,
      "channel": "whatsapp",
      "template": "biz_reminder_whatsapp"
    }
  ]
}
JSON
```
