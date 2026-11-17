# Your customer billing portal

The portal lets a customer view their own invoices, subscriptions and saved receipts without a dashboard account. Each private link belongs to one customer in one application.

## Share a private link

Create the link from the customer's billing tools, or use your backend application key with `POST /v1/biz/users/{user_id}/portal-link`. Save the returned `token_id` if you need to revoke it later. The response also includes `url` and `expires_at`.

Treat the URL like a password. Send it only to that customer. Do not place it in screenshots, public tickets, analytics events or example source code. New links are stored as digests; existing links keep their original expiry. `POST /v1/biz/portal-tokens/{token_id}/revoke` disables access without deleting financial records.

## What the customer sees

- Invoices with the actual total, amount paid, credits and remaining balance.
- Payment and receipt history scoped to that customer.
- Subscriptions and supported recurring authorization with explicit consent.
- **Print / Save as PDF** for issued documents, using the business and customer details frozen when issued.

A draft or an invoice marked open or overdue is not proof of payment. Online payment is available only when the business and invoice support it. Cash or direct UPI received outside the portal must first be confirmed by staff.

## Pending or expired sessions

When checkout confirmation is pending, check its saved status rather than paying again. A valid signed callback can settle a payment even if the customer link expires during checkout. The expired link still cannot read customer records; ask the business for a new link.

The portal sends no dashboard credentials or cookies. API responses and the portal page use no-store caching and no-referrer policies. The application's print button uses an isolated invoice copy to keep the private URL out of browser-generated print headers. Do not share the original page URL with a public print service.

See [customer billing](/docs/business-billing) for the public endpoint list and [payments](/docs/business-billing-payments) for settlement and refunds.
