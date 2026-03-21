# In-App Notifications — Complete Integration Guide

Store notifications in FreeRangeNotify's built-in inbox and let users query, read, snooze, and archive them on demand. In-App notifications are the foundation for notification centers, activity feeds, and inbox UIs.

---

## Architecture Overview

```
Your Backend                    FreeRangeNotify                    Your Frontend
───────────                     ───────────────                    ─────────────
POST /v1/notifications ──────►  API Server ──────► Elasticsearch   GET /v1/notifications
  { channel: "in_app" }              │              (indexed)       ◄────── Query inbox
                                     │                 │
                                     ▼                 ▼
                                  Worker           Stored as
                                (no delivery)      "pending"
                                                       │
                                                       ▼
                                              User fetches on demand
                                              Mark read / Archive / Snooze
```

**Key difference from SSE:** In-App notifications are **not pushed** to the client. They're stored in Elasticsearch and retrieved on demand via the Inbox API. Use this channel when:

- You want a persistent notification history
- Notifications should survive page refreshes and reconnections
- Users need to manage notifications (read, snooze, archive)
- You're building a notification center or inbox UI

> **Combine with SSE:** For the best experience, send notifications on **both** channels. Use SSE for real-time push and In-App for persistent storage. See [Combining SSE + In-App](#combining-sse--in-app) below.

---

## Prerequisites

Before integrating In-App notifications, you need:

1. **An Application** with an API key
2. **A registered User** — identified by any of: your platform's username/ID (`external_id`), their email, or the FRN internal UUID
3. **A Template** (optional — you can send raw title/body instead)

> **User ID resolution:** Every FreeRangeNotify endpoint that accepts a `user_id` automatically resolves it. You can pass your platform's username (e.g., `alice_monkeys`), an email address, or the internal UUID — FRN figures out the rest. No need to store or look up internal UUIDs.

If you haven't set these up yet, follow the [Getting Started](/docs/getting-started) guide first.

---

## Step 1: Send an In-App Notification

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/notifications/ \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "alice_monkeys",
    "channel": "in_app",
    "priority": "normal",
    "template_id": "new_comment",
    "category": "social",
    "data": {
      "commenter": "Alice",
      "post_title": "Getting Started with FRN",
      "comment_preview": "Great article! I especially liked..."
    }
  }'
```

**Response:**
```json
{
  "notification_id": "notif-abc123",
  "app_id": "app-xyz",
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "channel": "in_app",
  "priority": "normal",
  "status": "pending",
  "content": {
    "title": "New Comment from Alice",
    "body": "Alice commented on 'Getting Started with FRN': Great article! I especially liked..."
  },
  "category": "social",
  "created_at": "2026-03-18T14:30:00Z",
  "updated_at": "2026-03-18T14:30:00Z"
}
```

Without a template, provide `title` and `body` directly:

```json
{
  "user_id": "alice_monkeys",
  "channel": "in_app",
  "priority": "high",
  "title": "New follower",
  "body": "Bob started following you",
  "template_id": "generic",
  "category": "social",
  "data": {
    "follower_id": "user-bob-456",
    "action_url": "/profile/bob"
  }
}
```

---

## Step 2: Query the Inbox

### List all notifications for a user

```bash
curl "https://freerangenotify.monkeys.support/v1/notifications?user_id=YOUR_USER_ID&page=1&page_size=20" \
  -H "X-API-Key: YOUR_API_KEY"
```

**Response:**
```json
{
  "notifications": [
    {
      "notification_id": "notif-abc123",
      "app_id": "app-xyz",
      "user_id": "550e8400-e29b-41d4-a716-446655440000",
      "channel": "in_app",
      "priority": "normal",
      "status": "pending",
      "content": {
        "title": "New Comment from Alice",
        "body": "Alice commented on your post...",
        "data": {
          "commenter": "Alice",
          "post_title": "Getting Started with FRN"
        }
      },
      "template_id": "new_comment",
      "category": "social",
      "created_at": "2026-03-18T14:30:00Z",
      "updated_at": "2026-03-18T14:30:00Z",
      "read_at": null,
      "archived_at": null,
      "snoozed_until": null
    }
  ],
  "total": 42,
  "page": 1,
  "page_size": 20
}
```

### Filter by status, channel, category, priority

```bash
# Unread only
GET /v1/notifications/unread?user_id=YOUR_USER_ID&page=1&page_size=20

# Filter by category
GET /v1/notifications?user_id=YOUR_USER_ID&category=social

# Filter by channel (useful when mixing SSE + in_app)
GET /v1/notifications?user_id=YOUR_USER_ID&channel=in_app

# Filter by status
GET /v1/notifications?user_id=YOUR_USER_ID&status=pending

# Filter by priority
GET /v1/notifications?user_id=YOUR_USER_ID&priority=high

# Filter by date range (RFC3339 or YYYY-MM-DD)
GET /v1/notifications?user_id=YOUR_USER_ID&from_date=2026-03-01&to_date=2026-03-18

# Combine filters
GET /v1/notifications?user_id=YOUR_USER_ID&channel=in_app&category=social&priority=high&page=1&page_size=10
```

### Get unread count

```bash
curl "https://freerangenotify.monkeys.support/v1/notifications/unread/count?user_id=YOUR_USER_ID" \
  -H "X-API-Key: YOUR_API_KEY"
```

**Response:**
```json
{
  "count": 15
}
```

### Get a single notification

```bash
curl "https://freerangenotify.monkeys.support/v1/notifications/NOTIFICATION_ID" \
  -H "X-API-Key: YOUR_API_KEY"
```

---

## Step 3: Manage Notifications

### Mark as read

Mark specific notifications as read:

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/notifications/read \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "YOUR_USER_ID",
    "notification_ids": ["notif-abc123", "notif-def456"]
  }'
```

### Mark all as read

Mark all notifications as read, optionally filtered by category:

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/notifications/read-all \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "YOUR_USER_ID",
    "category": "social"
  }'
```

Omit `category` to mark all notifications as read.

### Archive

Remove notifications from the active inbox without deleting them:

```bash
curl -X PATCH https://freerangenotify.monkeys.support/v1/notifications/bulk/archive \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "YOUR_USER_ID",
    "notification_ids": ["notif-abc123", "notif-def456"]
  }'
```

**Response:**
```json
{
  "message": "notifications archived",
  "count": 2
}
```

### Snooze

Defer a notification so it reappears later:

```bash
# Snooze for a duration
curl -X POST https://freerangenotify.monkeys.support/v1/notifications/notif-abc123/snooze \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"duration": "1h"}'

# Snooze until a specific time
curl -X POST https://freerangenotify.monkeys.support/v1/notifications/notif-abc123/snooze \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"until": "2026-03-19T09:00:00Z"}'
```

**Response:**
```json
{
  "message": "notification snoozed",
  "snoozed_until": "2026-03-18T15:30:00Z"
}
```

**Duration format:** Go duration strings — `"30m"`, `"1h"`, `"24h"`, `"1h30m"`.

### Unsnooze

Cancel a snooze and make the notification active again:

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/notifications/notif-abc123/unsnooze \
  -H "X-API-Key: YOUR_API_KEY"
```

### Cancel a notification

```bash
curl -X DELETE https://freerangenotify.monkeys.support/v1/notifications/notif-abc123 \
  -H "X-API-Key: YOUR_API_KEY"
```

### Cancel multiple notifications

```bash
curl -X DELETE https://freerangenotify.monkeys.support/v1/notifications/batch \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"notification_ids": ["notif-abc123", "notif-def456"]}'
```

### Retry a failed notification

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/notifications/notif-abc123/retry \
  -H "X-API-Key: YOUR_API_KEY"
```

---

## Step 4: SDK Integration

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

// Send an in-app notification
await client.notifications.send({
  user_id: 'YOUR_USER_ID',
  channel: 'in_app',
  priority: 'normal',
  template_id: 'new_comment',
  category: 'social',
  data: { commenter: 'Alice' },
});

// Query inbox
const inbox = await client.notifications.list({
  userId: 'YOUR_USER_ID',
  channel: 'in_app',
  page: 1,
  pageSize: 20,
});

// Get unread count
const count = await client.notifications.getUnreadCount('YOUR_USER_ID');

// List unread only
const unread = await client.notifications.listUnread('YOUR_USER_ID', 1, 20);

// Mark as read
await client.notifications.markRead('YOUR_USER_ID', ['notif-abc123']);

// Mark all as read
await client.notifications.markAllRead('YOUR_USER_ID');

// Mark all in a category as read
await client.notifications.markAllRead('YOUR_USER_ID', 'social');

// Archive
await client.notifications.archive('YOUR_USER_ID', ['notif-abc123', 'notif-def456']);

// Snooze for 1 hour
await client.notifications.snooze('notif-abc123', '1h');

// Unsnooze
await client.notifications.unsnooze('notif-abc123');

// Cancel
await client.notifications.cancel('notif-abc123');

// Retry failed
await client.notifications.retry('notif-abc123');
```

### React SDK

```bash
npm install @freerangenotify/react @freerangenotify/js
```

**Provider setup:**

```jsx
import { FreeRangeProvider } from '@freerangenotify/react';

function App() {
  return (
    <FreeRangeProvider
      apiKey="YOUR_API_KEY"
      userId="YOUR_USER_ID"
      apiBaseURL="https://freerangenotify.monkeys.support/v1"
    >
      <YourApp />
    </FreeRangeProvider>
  );
}
```

**NotificationBell — drop-in inbox UI:**

```jsx
import { NotificationBell } from '@freerangenotify/react';

function Header() {
  return (
    <nav>
      <NotificationBell
        theme="light"
        tabs={['All', 'Alerts', 'Updates', 'Social']}
        maxItems={50}
        pageSize={20}
        onNotificationClick={(n) => {
          // Navigate to the relevant page
          router.push(n.data?.action_url || '/notifications');
        }}
      />
    </nav>
  );
}
```

**Custom inbox with hooks:**

```jsx
import { useNotifications, useUnreadCount } from '@freerangenotify/react';

function InboxPage() {
  const {
    notifications,
    loading,
    markRead,
    markAllRead,
    archive,
    snooze,
    loadMore,
    hasMore,
    refresh,
  } = useNotifications({ category: 'social' });

  const { count } = useUnreadCount();

  if (loading) return <Spinner />;

  return (
    <div>
      <div className="inbox-header">
        <h1>Inbox ({count} unread)</h1>
        <button onClick={markAllRead}>Mark all read</button>
        <button onClick={refresh}>Refresh</button>
      </div>

      {notifications.map(n => (
        <div key={n.notification_id} className={n.read_at ? 'read' : 'unread'}>
          <strong>{n.content?.title}</strong>
          <p>{n.content?.body}</p>
          <span>{new Date(n.created_at).toLocaleString()}</span>

          <div className="actions">
            {!n.read_at && (
              <button onClick={() => markRead([n.notification_id])}>
                Mark read
              </button>
            )}
            <button onClick={() => archive([n.notification_id])}>
              Archive
            </button>
            <button onClick={() => snooze(n.notification_id, '1h')}>
              Snooze 1h
            </button>
          </div>
        </div>
      ))}

      {hasMore && <button onClick={loadMore}>Load more</button>}
    </div>
  );
}
```

**User preferences component:**

```jsx
import { Preferences } from '@freerangenotify/react';

function SettingsPage() {
  return (
    <div>
      <h2>Notification Preferences</h2>
      <Preferences
        theme="light"
        onSave={(prefs) => console.log('Saved:', prefs)}
      />
    </div>
  );
}
```

---

## Combining SSE + In-App

For the best user experience, send notifications on both channels simultaneously. SSE provides instant push; In-App provides persistent history.

**Option 1: Send two notifications**

```javascript
// Real-time push
await client.notifications.send({
  user_id: userId,
  channel: 'sse',
  priority: 'high',
  template_id: 'new_order',
  category: 'orders',
  data: orderData,
});

// Persistent inbox
await client.notifications.send({
  user_id: userId,
  channel: 'in_app',
  priority: 'high',
  template_id: 'new_order',
  category: 'orders',
  data: orderData,
});
```

**Option 2: Use a Workflow (recommended)**

[Workflows](/docs/workflows) let you define multi-step notification sequences. A single trigger can deliver to both SSE and In-App:

```bash
# Trigger a workflow that sends to both channels
curl -X POST https://freerangenotify.monkeys.support/v1/workflows/trigger \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "workflow_id": "new_order_workflow",
    "user_id": "YOUR_USER_ID",
    "data": {
      "order_id": "12345",
      "customer_name": "Alice"
    }
  }'
```

**Frontend handling:**

When using both channels, the React SDK handles everything automatically. The `NotificationBell` shows the persistent inbox (In-App) and updates the unread count in real-time via SSE. No extra wiring needed — just set up the `FreeRangeProvider` with SSE credentials and the bell component does the rest.

---

## Full Integration Example: monkeys.com.co

Here's the complete flow for a third-party app integrating In-App notifications:

### Backend: Send notifications on user events

```javascript
// your-backend/services/notifications.js
const FRN_API_KEY = process.env.FRN_API_KEY;
const FRN_BASE_URL = 'https://freerangenotify.monkeys.support/v1';

async function notifyNewComment(postOwnerUsername, comment) {
  // Send as both SSE (real-time) and in_app (persistent)
  const payload = {
    user_id: postOwnerUsername,  // your platform's username — FRN resolves it
    priority: 'normal',
    template_id: 'new_comment',
    category: 'social',
    data: {
      commenter: comment.author.name,
      post_title: comment.post.title,
      comment_preview: comment.body.substring(0, 100),
      action_url: `/posts/${comment.post.id}#comment-${comment.id}`,
    },
  };

  await Promise.all([
    fetch(`${FRN_BASE_URL}/notifications/`, {
      method: 'POST',
      headers: { 'X-API-Key': FRN_API_KEY, 'Content-Type': 'application/json' },
      body: JSON.stringify({ ...payload, channel: 'sse' }),
    }),
    fetch(`${FRN_BASE_URL}/notifications/`, {
      method: 'POST',
      headers: { 'X-API-Key': FRN_API_KEY, 'Content-Type': 'application/json' },
      body: JSON.stringify({ ...payload, channel: 'in_app' }),
    }),
  ]);
}
```

### Frontend: Display full inbox

```jsx
// your-frontend/src/App.jsx
import { FreeRangeProvider, NotificationBell } from '@freerangenotify/react';

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
    <nav className="flex items-center justify-between px-6 py-4">
      <Logo />
      <div className="flex items-center gap-4">
        <NotificationBell
          theme="dark"
          tabs={['All', 'Social', 'Orders', 'System']}
          onNotificationClick={(n) => {
            if (n.data?.action_url) {
              router.push(n.data.action_url);
            }
          }}
        />
        <UserMenu />
      </div>
    </nav>
  );
}
```

---

## Notification Lifecycle

| Status | Meaning |
|--------|---------|
| `pending` | Stored, awaiting processing or user query |
| `queued` | In the processing queue |
| `read` | User marked as read via `POST /v1/notifications/read` |
| `snoozed` | Deferred until `snoozed_until` timestamp |
| `archived` | Removed from active inbox, still queryable |
| `cancelled` | Cancelled via `DELETE /v1/notifications/:id` |

---

## API Reference Summary

| Endpoint | Method | Auth | Purpose |
|----------|--------|------|---------|
| `/v1/notifications/` | POST | API Key | Send a notification |
| `/v1/notifications` | GET | API Key | List notifications (with filters) |
| `/v1/notifications/:id` | GET | API Key | Get single notification |
| `/v1/notifications/unread` | GET | API Key | List unread notifications |
| `/v1/notifications/unread/count` | GET | API Key | Get unread count |
| `/v1/notifications/read` | POST | API Key | Mark specific notifications as read |
| `/v1/notifications/read-all` | POST | API Key | Mark all as read (optional category filter) |
| `/v1/notifications/bulk/archive` | PATCH | API Key | Archive multiple notifications |
| `/v1/notifications/:id/snooze` | POST | API Key | Snooze a notification |
| `/v1/notifications/:id/unsnooze` | POST | API Key | Unsnooze a notification |
| `/v1/notifications/:id` | DELETE | API Key | Cancel a notification |
| `/v1/notifications/batch` | DELETE | API Key | Cancel multiple notifications |
| `/v1/notifications/:id/retry` | POST | API Key | Retry a failed notification |
| `/v1/notifications/:id/status` | PUT | API Key | Update notification status |
