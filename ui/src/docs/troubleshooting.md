# Troubleshooting

Start with the app, environment, notification ID, channel, status, `error_message`, retry count and timestamps. HTTP 202 only means accepted. Failed notifications can fail at admission, credit reservation, rendering or provider delivery.

## Insufficient or reserved credits

Open Billing and compare remaining, reserved and available. Available controls sending. Recharge adds to unexpired credits; it does not erase prior remaining balance. A prolonged hold requires operator reconciliation, not repeated sends or manually clearing reservations during active processing.

## Email and custom SMTP

Check the selected application's email settings and effective environment, not only the shared server provider. Confirm host, port/TLS mode, username, password and permitted sender address with your mail operator. A masked saved password is not a new password. Check safe failure metadata for configuration, authentication, network or rate-limit failures. Never send SMTP passwords or API keys in a support ticket.

## WhatsApp and SMS

Configure the app's channel connection and recipient E.164 phone. WhatsApp requires opt-in, a supported provider/account and either an approved template or an open service window for supported free-form messages. Credits do not override vendor approval or account restrictions. See [Channels](/docs/channels).

## Queue delays

Check whether workers are running and inspect provider throttling and retries. Large broadcasts are asynchronous; adding HTTP requests does not make a limited SMTP sender faster. Operators can inspect queue depth and dead letters through their authorized operational tools. Customer API keys do not grant unrestricted ops access.

## Webhooks

Use a publicly reachable HTTPS receiver. Generic webhooks use `X-Webhook-Signature`, a hex HMAC-SHA256 over the exact request body with the configured signing secret. Verify bytes before parsing JSON. Slack, Teams, Discord, inbound connectors and provider callbacks have different contracts; don't reuse a generic webhook signature algorithm for all of them. Browser CORS does not govern a server-to-server POST.

## Templates and identifiers

Use the exact response ID and matching app/environment. `external_id` is your stable user identifier; `data.user_id` is the internal ID. Template creation returns `id`, notification creation returns `notification_id`. Match template variables to `data` keys and preview via render. Updates do not automatically create snapshots.

## SSE

Use a backend-issued short-lived token and listen for the named `notification` event. Obtain a fresh token after expiry. Preserve the actual user ID accepted by your integration, and check deployment CORS/proxy streaming configuration. See [SSE](/docs/sse). Don't expose an application key to fix a browser connection.

## OTP

Check application/environment scope, expiry, attempt budget and resend cooldown. Pending/uncertain dispatch returns a retryable state; do not initiate automatic replacement sends for an unknown queue outcome. After expiry start a fresh send. The verify record stores a hash; delivery records still contain message content. See [OTP](/docs/otp).
