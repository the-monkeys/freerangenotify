# Delivery Channels

FreeRangeNotify supports six delivery channels. Each channel requires specific configuration in your application's provider settings.

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
curl -X POST http://localhost:8080/v1/users/USER_ID/devices \
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
curl -X POST http://localhost:8080/v1/users/USER_ID/devices \
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
curl -X PUT http://localhost:8080/v1/users/USER_ID \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"phone": "+15551234567"}'
```

---

## Server-Sent Events (SSE)

Real-time browser notifications via SSE — no polling, no refresh needed.

### How It Works

1. Client opens a persistent connection to the SSE endpoint
2. Server pushes notifications in real-time as they're processed
3. Uses Redis Pub/Sub for horizontal scaling across workers

### Client Connection

```javascript
const eventSource = new EventSource(
  'http://localhost:8080/v1/sse?user_id=INTERNAL_USER_UUID'
);

eventSource.addEventListener('notification', (event) => {
  const notification = JSON.parse(event.data);
  console.log('New notification:', notification);
});
```

> **Important:** The `user_id` parameter must be the **internal UUID** returned during user creation — not the external ID.

### Sending SSE Notifications

```bash
curl -X POST http://localhost:8080/v1/notifications/ \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "INTERNAL_USER_UUID",
    "channel": "sse",
    "priority": "high",
    "title": "New Message",
    "body": "You have a new message"
  }'
```

### Test Client

FreeRangeNotify includes a test SSE client at `test-sse-client/` — a Next.js app for verifying real-time delivery during development.

---

## In-App Notifications

Store notifications in FreeRangeNotify's internal inbox for retrieval via the Inbox API. No external delivery — notifications are queried on-demand.

### How It Works

1. Send a notification with `"channel": "in_app"`
2. The notification is stored in Elasticsearch with status `pending`
3. Users query their inbox via `GET /v1/notifications?user_id=UUID`
4. Mark as read, snooze, or archive via the Inbox APIs

### Sending

```bash
curl -X POST http://localhost:8080/v1/notifications/ \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "INTERNAL_USER_UUID",
    "channel": "in_app",
    "title": "New Comment",
    "body": "Someone replied to your post"
  }'
```

### Inbox Operations

- **Unread count:** `GET /v1/notifications/unread-count?user_id=UUID`
- **Mark read:** `POST /v1/notifications/mark-read` with `{ "notification_ids": [...] }`
- **Mark all read:** `POST /v1/notifications/mark-all-read` with `{ "user_id": "UUID" }`
- **Snooze:** `POST /v1/notifications/{id}/snooze` with `{ "until": "ISO8601" }`
- **Archive:** `POST /v1/notifications/archive` with `{ "notification_ids": [...] }`

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

### WhatsApp (via Business API)

If you have access to the [WhatsApp Business API](https://developers.facebook.com/docs/whatsapp/cloud-api/), point a webhook at your middleware that converts the payload into WhatsApp's API format:

```json
{
  "channel": "webhook",
  "webhook_url": "https://your-middleware.com/whatsapp/send",
  "data": {
    "phone": "+15551234567",
    "template_name": "order_update"
  }
}
```

> **Note:** WhatsApp requires pre-approved message templates. Your middleware should map FreeRangeNotify's payload to WhatsApp's template API.
