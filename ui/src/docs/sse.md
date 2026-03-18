# SSE (Server-Sent Events) — Complete Integration Guide

Real-time push notifications to browsers and web apps via Server-Sent Events. No polling, no WebSocket complexity — just a persistent HTTP connection that delivers notifications the instant they're processed.

---

## Architecture Overview

```
Your Backend                    FreeRangeNotify                      Your Frontend
───────────                     ───────────────                      ─────────────
POST /v1/notifications ──────►  API Server (queues) ──────►  Worker
  { channel: "sse" }                                            │
                                                                ▼
                                                          Redis Pub/Sub
                                                         (sse:notifications)
                                                                │
                                                                ▼
                                                          SSE Broadcaster
                                                                │
                                                 ┌──────────────┼──────────────┐
                                                 ▼              ▼              ▼
                                            Client A       Client B       Client C
                                          (EventSource)  (EventSource)  (EventSource)
                                          GET /v1/sse    GET /v1/sse    GET /v1/sse
```

**Flow:**
1. Your backend sends a notification to FreeRangeNotify with `"channel": "sse"`
2. The worker processes it and publishes to Redis Pub/Sub channel `sse:notifications`
3. The SSE Broadcaster picks it up and pushes it to all connected clients for that user
4. Your frontend receives the event instantly via `EventSource`

---

## Prerequisites

Before integrating SSE, you need:

1. **An Application** with an API key
2. **A registered User** — identified by any of: your platform's username/ID (`external_id`), their email, or the FRN internal UUID
3. **A Template** (optional — you can send raw title/body instead)

> **User ID resolution:** Every FreeRangeNotify endpoint that accepts a `user_id` automatically resolves it. You can pass your platform's username (e.g., `alice_monkeys`), an email address, or the internal UUID — FRN figures out the rest. No need to store or look up internal UUIDs.

If you haven't set these up yet, follow the [Getting Started](/docs/getting-started) guide first.

---

## Step 1: Generate a Subscriber Hash (HMAC Authentication)

SSE connections are authenticated using HMAC-SHA256. This prevents users from spoofing another user's notification stream.

**Server-side — generate the hash using your platform's user ID:**

```bash
# Pass your external user ID (e.g., monkeys.com.co username) — FRN resolves it automatically
curl https://freerangenotify.monkeys.support/v1/users/alice_monkeys/subscriber-hash \
  -H "X-API-Key: YOUR_API_KEY"
```

**Response:**
```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "subscriber_hash": "a1b2c3d4e5f6..."
}
```

The endpoint resolves your `external_id` to the internal UUID, then returns `HMAC-SHA256(internal_user_id, api_key)`. The hash is deterministic for the same user.

> **Note:** If you compute the hash yourself, you must use the **internal UUID** (returned in the `user_id` response field), not the external_id. The simplest approach is to call the endpoint above and let FRN handle it.

```javascript
// Node.js — only if you already have the internal UUID
const crypto = require('crypto');
const hash = crypto.createHmac('sha256', API_KEY).update(INTERNAL_UUID).digest('hex');
```

```go
// Go — only if you already have the internal UUID
mac := hmac.New(sha256.New, []byte(apiKey))
mac.Write([]byte(internalUUID))
hash := hex.EncodeToString(mac.Sum(nil))
```

```python
# Python — only if you already have the internal UUID
import hmac, hashlib
hash = hmac.new(api_key.encode(), internal_uuid.encode(), hashlib.sha256).hexdigest()
```

> **Important:** Generate the subscriber hash on your **backend** and pass it to your frontend. Never expose your API key in client-side code.

---

## Step 2: Connect to the SSE Stream

### Option A: SSE Token (Recommended)

SSE tokens are short-lived (15 minutes), single-use tokens that don't expose your API key to the browser.

**1. Create a token (server-side):**

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/sse/tokens \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"user_id": "USER_EXTERNAL_ID_OR_UUID"}'
```

**Response:**
```json
{
  "sse_token": "sset_a1b2c3d4e5f6...",
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "expires_in": 900
}
```

**2. Connect from the browser:**

```javascript
const eventSource = new EventSource(
  'https://freerangenotify.monkeys.support/v1/sse?sse_token=sset_a1b2c3d4e5f6...'
);
```

### Option B: API Key + Subscriber Hash

Pass the API key and HMAC hash directly as query parameters:

```javascript
const eventSource = new EventSource(
  'https://freerangenotify.monkeys.support/v1/sse?user_id=YOUR_USER_ID&token=YOUR_API_KEY&subscriber_hash=HMAC_HASH'
);
```

### Option C: API Key Only (Legacy — not recommended)

If HMAC enforcement is disabled on the server:

```javascript
const eventSource = new EventSource(
  'https://freerangenotify.monkeys.support/v1/sse?user_id=YOUR_USER_ID&token=YOUR_API_KEY'
);
```

> **Note:** All three connection methods accept your `external_id` (e.g., your platform username). FreeRangeNotify resolves it to the internal UUID automatically.

---

## Step 3: Handle Events

The SSE stream emits three types of events:

### `connected` — Connection established

```javascript
eventSource.addEventListener('connected', (event) => {
  const data = JSON.parse(event.data);
  console.log('Connected for user:', data.user_id);
});
```

**Payload:**
```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### `notification` — New notification received

```javascript
eventSource.addEventListener('notification', (event) => {
  const notification = JSON.parse(event.data);

  // Show a browser notification, update your UI, play a sound, etc.
  console.log(notification.title, notification.body);
});
```

**Payload (ClientPayload):**
```json
{
  "notification_id": "notif-uuid-here",
  "title": "New Order Received",
  "body": "Order #12345 from Alice has been placed",
  "channel": "sse",
  "category": "orders",
  "status": "sent",
  "data": {
    "order_id": "12345",
    "customer_name": "Alice"
  },
  "created_at": "2026-03-18T14:30:00Z"
}
```

> The payload sent to the browser is a **clean subset** of the internal notification. Fields like `app_id`, `retry_count`, and `error_message` are never exposed to the client.

### Keepalive — Connection health

The server sends a keepalive comment every 20 seconds (`: keepalive\n\n`). This is handled automatically by `EventSource` — no action needed.

---

## Step 4: Send an SSE Notification

From your backend, send a notification with `"channel": "sse"`:

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/notifications/ \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "alice_monkeys",
    "channel": "sse",
    "priority": "high",
    "template_id": "order_received",
    "data": {
      "order_id": "12345",
      "customer_name": "Alice"
    }
  }'
```

**Response:**
```json
{
  "notification_id": "notif-abc123",
  "app_id": "app-xyz",
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "channel": "sse",
  "priority": "high",
  "status": "queued",
  "content": {
    "title": "New Order Received",
    "body": "Order #12345 from Alice has been placed"
  },
  "created_at": "2026-03-18T14:30:00Z",
  "updated_at": "2026-03-18T14:30:00Z"
}
```

You can also send without a template by providing `title` and `body` directly:

```json
{
  "user_id": "alice_monkeys",
  "channel": "sse",
  "priority": "normal",
  "title": "Direct Message",
  "body": "Hey, your deployment is complete!",
  "template_id": "generic",
  "category": "alerts"
}
```

---

## Step 5: SDK Integration

### JavaScript SDK

```bash
npm install @freerangenotify/js
```

```javascript
import { FreeRangeNotify } from '@freerangenotify/js';

const client = new FreeRangeNotify({
  apiKey: 'YOUR_API_KEY',
  baseURL: 'https://freerangenotify.monkeys.support/v1',
});

// Connect to SSE with auto-reconnect
const connection = client.sse.subscribe('YOUR_USER_ID', (notification) => {
  console.log('Real-time:', notification.title, notification.body);
}, {
  subscriberHash: 'HMAC_HASH',       // Recommended
  onConnected: () => console.log('SSE connected'),
  onConnectionChange: (connected) => console.log('Connection:', connected),
  onUnreadCountChange: (count) => console.log('Unread:', count),
  autoReconnect: true,                // Default: true
  reconnectInterval: 3000,            // Default: 3000ms
});

// Later: disconnect
connection();
```

### React SDK

```bash
npm install @freerangenotify/react @freerangenotify/js
```

**1. Wrap your app with the provider:**

```jsx
import { FreeRangeProvider } from '@freerangenotify/react';

function App() {
  return (
    <FreeRangeProvider
      apiKey="YOUR_API_KEY"
      userId="YOUR_USER_ID"
      apiBaseURL="https://freerangenotify.monkeys.support/v1"
      subscriberHash="HMAC_HASH"
    >
      <YourApp />
    </FreeRangeProvider>
  );
}
```

> The provider automatically connects to SSE when mounted and disconnects on unmount.

**2. Drop in the NotificationBell:**

```jsx
import { NotificationBell } from '@freerangenotify/react';

function Header() {
  return (
    <nav>
      <NotificationBell
        theme="light"
        tabs={['All', 'Alerts', 'Updates', 'Social']}
        onNotification={(n) => console.log('New:', n)}
        maxItems={50}
        pageSize={20}
      />
    </nav>
  );
}
```

**3. Or build a custom UI with hooks:**

```jsx
import { useNotifications, useUnreadCount } from '@freerangenotify/react';

function NotificationPanel() {
  const { notifications, loading, markRead, markAllRead, archive, snooze, loadMore, hasMore } = useNotifications();
  const { count } = useUnreadCount();

  return (
    <div>
      <h2>Notifications ({count} unread)</h2>
      <button onClick={markAllRead}>Mark all read</button>
      {notifications.map(n => (
        <div key={n.notification_id}>
          <strong>{n.title}</strong>
          <p>{n.body}</p>
          <button onClick={() => markRead([n.notification_id])}>Read</button>
          <button onClick={() => archive([n.notification_id])}>Archive</button>
          <button onClick={() => snooze(n.notification_id, '1h')}>Snooze 1h</button>
        </div>
      ))}
      {hasMore && <button onClick={loadMore}>Load more</button>}
    </div>
  );
}
```

---

## Queued Notification Flush

When a user connects to SSE, FreeRangeNotify automatically **flushes all queued notifications** for that user. Any SSE notifications sent while the user was offline are delivered immediately upon connection — no manual action needed.

This means your frontend doesn't need to separately fetch missed notifications. They arrive as normal `notification` events right after the `connected` event.

---

## Connection Behavior

| Aspect | Behavior |
|--------|----------|
| **Keepalive** | Server sends `: keepalive` comment every 20 seconds |
| **Auto-reconnect** | Browser `EventSource` reconnects automatically on disconnect |
| **Multiple tabs** | Each tab maintains its own SSE connection — all receive events |
| **Horizontal scaling** | Redis Pub/Sub ensures events reach the correct server instance |
| **Failed delivery** | If a push to a connected client fails, the notification status is reset to `queued` for retry |
| **Token expiry** | SSE tokens expire after 15 minutes. Generate a new token and reconnect. |

---

## SSE Playground (Testing)

The admin dashboard includes an SSE Playground for quick testing without writing code.

**1. Create a playground session:**

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/admin/playground/sse \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

**Response:**
```json
{
  "id": "sse-a1b2c3d4",
  "sse_url": "https://freerangenotify.monkeys.support/v1/sse?user_id=sse-a1b2c3d4",
  "expires_in": "30m"
}
```

**2. Open the SSE URL in a browser tab** (or use `curl`):

```bash
curl -N "https://freerangenotify.monkeys.support/v1/sse?user_id=sse-a1b2c3d4"
```

**3. Send a test message:**

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/admin/playground/sse/sse-a1b2c3d4/send \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Test Notification",
    "body": "Hello from the playground!",
    "category": "test",
    "data": {"source": "playground"}
  }'
```

You should see the event appear instantly in the connected tab. Playground sessions expire after 30 minutes.

---

## Full Integration Example: monkeys.com.co

Here's the complete flow for a third-party application integrating FreeRangeNotify SSE:

### Backend Setup (one-time)

```javascript
// your-backend/setup.js
const FRN_API_KEY = process.env.FRN_API_KEY;
const FRN_BASE_URL = 'https://freerangenotify.monkeys.support/v1';

// 1. Register your user when they sign up on monkeys.com.co
async function registerUser(email, username) {
  const res = await fetch(`${FRN_BASE_URL}/users/`, {
    method: 'POST',
    headers: {
      'X-API-Key': FRN_API_KEY,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      email: email,
      user_id: username,  // your platform's username — used as external_id
    }),
  });
  return await res.json();
  // No need to store internal UUIDs — use the username everywhere
}
```

### Backend: Generate SSE credentials for frontend

```javascript
// your-backend/api/sse-credentials.js

app.get('/api/sse-credentials', authenticateUser, async (req, res) => {
  const username = req.user.username; // your platform's username

  // SSE Token (recommended) — pass your username, FRN resolves it
  const tokenRes = await fetch(`${FRN_BASE_URL}/sse/tokens`, {
    method: 'POST',
    headers: {
      'X-API-Key': FRN_API_KEY,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ user_id: username }),
  });
  const { sse_token, user_id } = await tokenRes.json();

  // Subscriber Hash — let FRN compute it via the API
  const hashRes = await fetch(`${FRN_BASE_URL}/users/${username}/subscriber-hash`, {
    headers: { 'X-API-Key': FRN_API_KEY },
  });
  const { subscriber_hash } = await hashRes.json();

  res.json({
    userId: user_id,       // internal UUID returned by FRN
    sseToken: sse_token,
    subscriberHash: subscriber_hash,
  });
});
```

### Backend: Send notifications when events occur

```javascript
// your-backend/services/notifications.js
async function notifyUser(username, event) {
  await fetch(`${FRN_BASE_URL}/notifications/`, {
    method: 'POST',
    headers: {
      'X-API-Key': FRN_API_KEY,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      user_id: username,  // your platform's username — FRN resolves it
      channel: 'sse',
      priority: event.urgent ? 'high' : 'normal',
      template_id: 'event_notification',
      category: event.type,
      data: {
        event_name: event.name,
        description: event.description,
        action_url: event.actionUrl,
      },
    }),
  });
}
```

### Frontend: Connect and display

```jsx
// your-frontend/src/App.jsx
import { FreeRangeProvider, NotificationBell } from '@freerangenotify/react';
import { useEffect, useState } from 'react';

function App() {
  const [creds, setCreds] = useState(null);

  useEffect(() => {
    fetch('/api/sse-credentials', { credentials: 'include' })
      .then(r => r.json())
      .then(setCreds);
  }, []);

  if (!creds) return <Loading />;

  return (
    <FreeRangeProvider
      apiKey={creds.sseToken}
      userId={creds.userId}
      apiBaseURL="https://freerangenotify.monkeys.support/v1"
      subscriberHash={creds.subscriberHash}
    >
      <Header />
      <Routes />
    </FreeRangeProvider>
  );
}

function Header() {
  return (
    <nav>
      <Logo />
      <NotificationBell
        theme="dark"
        onNotification={(n) => {
          // Optional: show a toast
          toast.info(n.title);
        }}
      />
    </nav>
  );
}
```

---

## Debugging

### Check notification status

```bash
curl https://freerangenotify.monkeys.support/v1/notifications/NOTIFICATION_ID \
  -H "X-API-Key: YOUR_API_KEY"
```

Look at the `status` field:
- `queued` — In the queue, not yet processed
- `sent` — Delivered to Redis Pub/Sub (user may not be connected)
- `delivered` — Pushed to a connected SSE client
- `failed` — Delivery failed after retries

### Watch worker logs

```bash
docker-compose logs -f notification-worker
```

Look for:
- `"Publishing SSE notification"` — Worker sent to Redis
- `"SSE client connected"` — User opened a connection
- `"Broadcasting to user"` — Broadcaster pushing to client

### Verify Redis Pub/Sub

```bash
docker-compose exec redis redis-cli SUBSCRIBE sse:notifications
```

### Common issues

| Problem | Cause | Fix |
|---------|-------|-----|
| `401 Unauthorized` on `/v1/sse` | Invalid or expired SSE token | Generate a fresh token via `POST /v1/sse/tokens` |
| `403 invalid subscriber hash` | HMAC mismatch | Use the `/v1/users/:id/subscriber-hash` endpoint to generate the hash — it handles external_id resolution automatically |
| Events not arriving | User not connected when notification sent | Check that the SSE connection is established before or use queued flush |
| Connection drops frequently | Proxy/load balancer timeout | Configure your reverse proxy for long-lived connections (e.g., `proxy_read_timeout 86400s` in nginx) |
| Duplicate events across tabs | Each tab has its own connection | This is expected behavior — deduplicate in your frontend state |

---

## API Reference Summary

| Endpoint | Method | Auth | Purpose |
|----------|--------|------|---------|
| `/v1/sse` | GET | SSE Token or API Key | Open SSE connection |
| `/v1/sse/tokens` | POST | API Key | Create short-lived SSE token |
| `/v1/users/:id/subscriber-hash` | GET | API Key | Generate HMAC subscriber hash |
| `/v1/notifications/` | POST | API Key | Send a notification (use `channel: "sse"`) |
| `/v1/admin/playground/sse` | POST | JWT | Create SSE playground session |
| `/v1/admin/playground/sse/:id/send` | POST | JWT | Send test message to playground |
