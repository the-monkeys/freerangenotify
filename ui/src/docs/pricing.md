# Pricing and credits

Use the [Pricing page](/pricing) and workspace **Billing** for current packs, currency, taxes and validity. Use authenticated `GET /billing/rates` and `GET /billing/usage` with your dashboard JWT to inspect the active rate card and wallet. Fixed channel tariffs in a copied guide can be stale; channel/provider/destination/category and your credential mode can affect the quote.

## Balance terms

- **Remaining** is the wallet balance before active holds.
- **Reserved** is committed to in-flight delivery operations and cannot be spent twice.
- **Available** is remaining minus reserved; it gates admission.

Reservations settle on delivery outcome and bounded reconciliation. A reserved balance is not a separate credit pack. If a hold persists unexpectedly, contact support with the app and notification IDs and failure details, without exposing API keys. Don't manually zero holds while sends may still be active.

## Recharge

Paid allocation adds to the current unexpired wallet. With 400 remaining, a 35,000-credit recharge results in **35,400 remaining**; any active reserved portion still reduces availability. Recharge preserves active holds and does not shorten existing validity. The current wallet model reports its effective expiry; it does not expose independent per-pack expiry lots. Already-expired unheld credits are not restored by a recharge. Check the checkout terms and returned expiry.

## Provider charges and caps

System delivery and bring-your-own credentials can have different platform/provider charges. External vendors bill your own account separately where applicable. Free/trial eligibility, daily caps and channel connection readiness still apply even if the wallet has credits. A successful API acceptance or queue count is not proof of a billable provider delivery.

Application requests/hour, worker concurrency, subscriber preferences and provider throughput are separate controls. See [Notifications](/docs/notifications) and [Channels](/docs/channels).
