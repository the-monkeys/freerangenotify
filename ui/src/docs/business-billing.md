# Billing your customers

Use **Applications → your application → Billing Setup** to manage your own products, plans, customer subscriptions, invoices and payments. Your customers are the subscribers in that application. Notification delivery credits are managed separately under [Pricing & Credits](/docs/pricing).

Business billing must be enabled on your installation. Existing applications keep their current billing lifecycle until an explicit, reviewed upgrade. Installing a release does not activate new payment collection or change historical invoices.

## Connect your backend

Create an application in the dashboard, open **Integration**, and copy its application key. Keep the key on your backend. The key selects the application; an application ID alone does not authorize billing requests. See [Authentication](/docs/authentication).

Use `Authorization: Bearer YOUR_APP_KEY` or `X-API-Key: YOUR_APP_KEY`. All paths below start at your installation's API origin. Use the [API Reference](/docs/api) for request schemas and available routes.

Create a customer with `POST /v1/users`. Save the returned `data.user_id` and use it as the billing `user_id`. See [Users](/docs/users) for profile create, read, update and delete operations.

Amounts are integers in the currency's minor units. For INR, `10000` means ₹100.00. Send currency codes such as `INR`; never send rupee amounts as decimal values in a `*_paisa` field.

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

## Record a payment or refund

Record money already received offline with `POST /v1/biz/invoices/INVOICE_ID/payments`:

```json
{
  "method": "bank_transfer",
  "amount_paisa": 11800,
  "notes": "Bank statement reference"
}
```

Supported amounts depend on the invoice's current due balance. `amount_paisa: 0` requests the full amount due. A supplied TDS amount also settles that part of the invoice; TDS is not extra customer credit.

For an upgraded application, excess cash becomes customer credit with its payment source retained. Record an authorized offline refund through `POST /v1/biz/payments/PAYMENT_ID/refund`, supplying `amount_paisa` and `reason`. This records a return already made offline; it does not send a bank transfer. A cash refund remains separate from the invoice's historical paid state.

Do not use manual payment requests to assert a gateway capture or gateway refund. Provider money must be verified through its original payment-account binding. An upgraded application's new online collection requires its own supported merchant connection. Never substitute the platform's payment account or move old orders to a newly connected account.

## Customer credit

For a reviewed, upgraded application:

- `GET /v1/biz/users/USER_ID/balance?currency=INR` returns completed available and reserved credit for that currency.
- `POST /v1/biz/users/USER_ID/balance` accepts an adjustment such as `{"amount_paisa":500,"reason":"Account correction"}`. This adds ₹5.00; a negative adjustment subtracts available credit.
- `POST /v1/biz/invoices/INVOICE_ID/apply-credit` spends available credit against that invoice.

Omitting currency on a new credit request uses the primary currency reviewed for the application. Choosing another currency never converts or relabels existing credit. Historical money whose currency or payment source is unresolved must be reconciled before use.

Reserved money is held for an unfinished action and is not available to spend. A confirmed cancellation can release a reservation. An unknown provider outcome must be reconciled before it can be released; waiting alone is not proof that no payment occurred.

## Retry an action safely

For an upgraded application, send a fresh `Idempotency-Key` for each intended financial action and reuse that exact key and payload when retrying it. Keep keys at most 200 characters. Use different keys for creating a draft, issuing it, recording a payment and refunding it.

A completed keyed retry returns that action's saved result. Reusing a key with a changed request returns a conflict. Without a key, each request represents a new action; repeated calls cannot be assumed to be duplicates. For customer creation with opening credit, retain the customer ID and creation key so an interrupted request can be recovered.

A temporary failure can include `operation_id` and `operation_state`. Keep them. Read `GET /v1/biz/operations/OPERATION_ID`; if needed, call `POST /v1/biz/operations/OPERATION_ID/resume` without changing the original action. Do not create another payment merely because a request timed out.

`409` can mean a concurrent change, a reused key with different input, or an invoice locked by automatic collection. `503` with a reconciliation or upgrade code means the saved state is not ready to use; check the operation and retry instructions. A partially saved balance is never a confirmed spendable balance.

These journal and operation guarantees apply after an application's reviewed upgrade. Legacy applications retain their existing API behavior until that cutover.
