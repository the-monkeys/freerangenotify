# Receiving and returning payments

Start in **Applications → your application → Billing Setup**. This area bills your customers for your products. Buying notification credits is a separate transaction under [Pricing & Credits](/docs/pricing).

## Cash, direct UPI and bank transfer

You do not need Razorpay to use invoices or record money you have received.

1. Open **Invoices**, create a draft and review the customer, currency, lines and tax.
2. Preview the calculated total, save the draft, then issue it. Issuing an invoice does not collect money.
3. After receiving and checking the payment, choose **Record payment**. Select the actual method and enter the amount and receipt or transaction reference.
4. Check **Amount paid** and **Amount due** on the saved invoice. A partial payment leaves a balance due. A receipt message has its own delivery status under **Messages / receipts**.

Staff confirmation of direct UPI is a record of an externally checked transfer. It is not automatic bank verification. Credits, TDS, write-offs and refunds are separate from cash received.

## Online collection

Connect your business's own Razorpay account in **Business settings**. Test and live connections are separate. Credentials are write-only; never place the secret key in a browser, mobile app or customer message. Account authentication, verified ownership and recurring-payment readiness are separate checks.

Online collection also requires the installation administrator to enable new collection and complete the application's reviewed billing upgrade. A legacy application retains its previous behavior until that upgrade. Connecting an account does not rewrite historical orders or transfer them between accounts.

For each subscription, choose invoice collection or automatic collection. Automatic collection requires a supported fixed INR plan, a ready account and the customer's authorization. Creating a subscription or completing an internal operation does not prove that money was captured.

The server verifies callback signatures, original account/order binding, amount and currency before applying provider money. Do not submit a manual cash payment to imitate a gateway capture.

## Interrupted actions

If the connection drops after you submit a payment, choose **Retry saved request** to reuse the original request key and details. When an operation reference is available, use **Check saved action**. Do not create a replacement payment while confirmation is pending.

Unfinished form details stay only in the current dashboard session. After closing or reloading, check saved invoices and payments before submitting another action. An unknown provider outcome requires reconciliation; it is not permission to charge again.

## Refunds

Use **Payments** to review the original payment and request a refund. A manual refund records money already returned outside the application. A gateway refund must be confirmed against the original payment account. Pending and unknown refunds remain pending or unknown until verified.

Overpayment credit retains its source. Credit already spent on another invoice can limit the refundable amount. Historical invoice payment and returned cash are displayed separately; refunding does not erase the original financial history.

For backend routes, minor-unit amounts, request keys and operation recovery, see the [customer billing API guide](/docs/business-billing). For customer access, see [the billing portal](/docs/customer-billing-portal).
