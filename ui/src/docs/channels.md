# Delivery Channels

FreeRangeNotify supports multiple delivery channels. Each channel requires specific configuration in your application's provider settings.

---

## Webhook

HTTP POST delivery with HMAC-SHA256 signing for payload integrity.

### Configuration

Set `webhook_url` per user profile or per notification:

```json
{
  "channel": "webhook",
  "webhook_url": "https://your-app.com/webhooks/notifications"
}
```

### Payload Format

```json
{
  "notification_id": "uuid",
  "user_id": "uuid",
  "title": "Order Shipped",
  "body": "Your order #12345 has shipped",
  "data": {"order_id": "12345"},
  "timestamp": "2025-01-15T10:00:00Z"
}
```

### Security

Every webhook includes an `X-Signature-256` header containing an HMAC-SHA256 signature. Verify this in your handler to ensure the payload wasn't tampered with.

### Smart Delivery

If a user has an active **presence** (checked in via a receiver), the worker overrides the static webhook URL with the user's dynamic URL — enabling real-time delivery to active sessions.

---

## Email (SMTP)

Direct email delivery via standard SMTP servers.

### Configuration

Configure SMTP settings in your application's provider configuration:

- **Host:** SMTP server hostname
- **Port:** 587 (TLS) or 465 (SSL)
- **Username/Password:** SMTP credentials
- **From Address:** Sender email address

### Template Support

Email templates support full HTML with Go template variables. The `subject` field on templates is used as the email subject line.

---

## SendGrid

Cloud email delivery via SendGrid's API.

### Configuration

- **API Key:** Your SendGrid API key
- **From Address:** Verified sender email

SendGrid is configured as an alternative email provider. When both SMTP and SendGrid are registered, you can set the preferred provider per environment.

---

## Apple Push Notification service (APNS)

Native iOS push notifications.

### Configuration

- **Auth Key:** `.p8` authentication key from Apple Developer
- **Key ID:** The key identifier
- **Team ID:** Your Apple Developer Team ID
- **Bundle ID:** Your app's bundle identifier
- **Environment:** `development` or `production`

### Device Tokens

Register device tokens via the User Devices API:

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/users/USER_ID/devices \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"platform": "ios", "token": "DEVICE_TOKEN"}'
```

---

## Firebase Cloud Messaging (FCM)

Cross-platform push notifications for Android, iOS, and Web.

### Configuration

- **Service Account JSON:** Firebase project service account credentials

### Device Tokens

Register Android/web device tokens the same way as APNS:

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/users/USER_ID/devices \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"platform": "android", "token": "FCM_DEVICE_TOKEN"}'
```

---

## SMS (Twilio)

Programmable messaging via Twilio.

### Configuration

- **Account SID:** Twilio account identifier
- **Auth Token:** Twilio authentication token
- **From Number:** Twilio phone number (E.164 format)

### User Setup

Ensure users have a `phone` field set in their profile:

```bash
curl -X PUT https://freerangenotify.monkeys.support/v1/users/USER_ID \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"phone": "+15551234567"}'
```

---

## WhatsApp (Twilio)

Send WhatsApp messages via Twilio's Programmable Messaging API.

### Configuration

- **Account SID:** Twilio account identifier
- **Auth Token:** Twilio authentication token
- **From Number:** Twilio WhatsApp-enabled phone number (e.g., `+14155238886` for sandbox)

### Environment Variables

```env
FREERANGE_PROVIDERS_WHATSAPP_ENABLED=true
FREERANGE_PROVIDERS_WHATSAPP_ACCOUNT_SID=ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
FREERANGE_PROVIDERS_WHATSAPP_AUTH_TOKEN=your_auth_token_here
FREERANGE_PROVIDERS_WHATSAPP_FROM_NUMBER=+14155238886
FREERANGE_PROVIDERS_WHATSAPP_TIMEOUT=15
FREERANGE_PROVIDERS_WHATSAPP_MAX_RETRIES=3
```

### Per-App Configuration

Override global credentials at the application level:

```json
{
  "whatsapp": {
    "enabled": true,
    "account_sid": "ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    "auth_token": "your_auth_token_here",
    "from_number": "+14155238886"
  }
}
```

### User Setup

Ensure users have a `phone` field with country code:

```bash
curl -X PUT http://localhost:8080/v1/users/USER_ID \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"phone": "+49XXXXXXXXXX"}'
```

### Message Format

- **Text messages:** Plain text with optional title formatting
- **Title rendering:** Titles appear as bold text (`*Title*`) followed by body
- **Media support:** Images, documents, audio, and video via `media_url`

### Number Format

Both sender and recipient phone numbers are automatically prefixed with `whatsapp:` when sent to Twilio's API:

- **From (Sender):** `whatsapp:+14155238886` (configured `from_number` + automatic prefix)
- **To (Recipient):** `whatsapp:+49XXXXXXXXXX` (user's `phone` field + automatic prefix)

> **Note:** Store phone numbers in configuration and user profiles **without** the `whatsapp:` prefix. The provider automatically adds the prefix during delivery.

### Testing with Sandbox

For development, use Twilio's WhatsApp Sandbox:

1. Log in to [Twilio Console](https://console.twilio.com/)
2. Go to **Messaging → Try it out → Send an SMS** to find your sandbox number
3. Send a WhatsApp message to the sandbox number with the join code to activate your account
4. Use the sandbox number as `FROM_NUMBER` in configuration
5. Send test notifications to any WhatsApp number with country code

### Sending Notifications

```bash
curl -X POST http://localhost:8080/v1/notifications/ \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "INTERNAL_USER_UUID",
    "channel": "whatsapp",
    "title": "Order Update",
    "body": "Your order #12345 has been shipped",
    "media_url": "https://example.com/image.jpg"
  }'
```

### Troubleshooting

**Error 63007: "Could not find a Channel with the specified From address"**
- The `from_number` is not configured as a WhatsApp Channel in your Twilio account
- Verify the number in your Twilio WhatsApp settings (sandbox or production)
- Ensure you've activated the sandbox account by sending a message to it

**Delivery Failures**
- Confirm recipient phone numbers include country code
- Verify `whatsapp_enabled` is not `false` in user preferences
- Check application and user preference settings in the API

---

## Server-Sent Events (SSE)

Real-time browser notifications via SSE — no polling, no refresh needed. Uses Redis Pub/Sub for horizontal scaling across workers.

```javascript
const eventSource = new EventSource(
  'https://freerangenotify.monkeys.support/v1/sse?sse_token=sset_abc123...'
);

eventSource.addEventListener('notification', (event) => {
  const notification = JSON.parse(event.data);
  console.log('New notification:', notification);
});
```

> **Full guide:** See the [SSE Integration Guide](/docs/sse) for complete setup — authentication, SDK integration, connection handling, debugging, and a full third-party integration example.

---

## In-App Notifications

Store notifications in FreeRangeNotify's internal inbox for retrieval on demand. Supports mark-read, snooze, archive, and bulk operations.

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/notifications/ \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "YOUR_USER_ID",
    "channel": "in_app",
    "title": "New Comment",
    "body": "Someone replied to your post"
  }'
```

> **Full guide:** See the [In-App Integration Guide](/docs/in-app) for the complete Inbox API — querying, filtering, mark-read, snooze, archive, SDK integration, and combining with SSE for real-time + persistent notifications.

---

## Custom Channels via Webhook

FreeRangeNotify doesn't have native integrations for every messaging platform, but you can deliver to any HTTP-capable service using the **Webhook** channel with the appropriate URL.

### Slack

Use a [Slack Incoming Webhook URL](https://api.slack.com/messaging/webhooks):

```json
{
  "channel": "webhook",
  "webhook_url": "https://hooks.slack.com/services/T00000/B00000/XXXX",
  "title": "Deployment Complete",
  "body": "v2.1.0 deployed to production"
}
```

Slack will render the `title` and `body` as a message. For richer formatting, include Slack Block Kit JSON in the `data` field and handle it in a custom webhook receiver.

### Discord

Use a [Discord Webhook URL](https://discord.com/developers/docs/resources/webhook):

```json
{
  "channel": "webhook",
  "webhook_url": "https://discord.com/api/webhooks/CHANNEL_ID/TOKEN",
  "title": "Alert",
  "body": "CPU usage exceeded 90%"
}
```

### Microsoft Teams

Use a [Teams Incoming Webhook connector](https://learn.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/how-to/add-incoming-webhook):

```json
{
  "channel": "webhook",
  "webhook_url": "https://outlook.office.com/webhook/...",
  "title": "New Issue Assigned",
  "body": "Issue #4521 has been assigned to you"
}
```

### Alternative Custom Integrations

For messaging platforms without native provider support, use the **Webhook** channel to deliver notifications to your custom middleware.
