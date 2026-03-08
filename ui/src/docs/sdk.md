# SDK Reference

FreeRangeNotify provides official SDKs for Go, JavaScript, and React. Each SDK wraps the REST API with typed methods and handles authentication, retries, and error formatting.

---

## Go SDK

### Installation

```bash
go get github.com/the-monkeys/freerangenotify-go
```

### Quick Start

```go
package main

import (
    frn "github.com/the-monkeys/freerangenotify-go"
)

func main() {
    client := frn.NewClient("YOUR_API_KEY", frn.WithBaseURL("http://localhost:8080/v1"))

    // Send a notification
    resp, err := client.Notifications.Send(&frn.NotificationRequest{
        UserID:     "user-uuid",
        Channel:    "email",
        Priority:   "normal",
        TemplateID: "welcome_email",
        Data: map[string]interface{}{
            "name":    "Alice",
            "product": "Acme",
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Sent:", resp.NotificationID)
}
```

### Available Methods

| Method | Description |
|--------|-------------|
| `client.Notifications.Send()` | Send a single notification |
| `client.Notifications.SendBulk()` | Send to multiple users |
| `client.Notifications.Broadcast()` | Broadcast to a topic |
| `client.Templates.Create()` | Create a template |
| `client.Templates.List()` | List templates |
| `client.Templates.Render()` | Preview a template |
| `client.Users.Create()` | Register a user |
| `client.Users.Update()` | Update user preferences |
| `client.Topics.Create()` | Create a topic |
| `client.Topics.Subscribe()` | Subscribe user to topic |
| `client.Workflows.Trigger()` | Trigger a workflow |

---

## JavaScript SDK

### Installation

```bash
npm install @freerangenotify/js
```

### Quick Start

```javascript
import { FreeRangeNotify } from '@freerangenotify/js';

const client = new FreeRangeNotify({
  apiKey: 'YOUR_API_KEY',
  baseURL: 'http://localhost:8080/v1',
});

// Send a notification
const result = await client.notifications.send({
  userId: 'user-uuid',
  channel: 'email',
  priority: 'normal',
  templateId: 'welcome_email',
  data: { name: 'Alice', product: 'Acme' },
});

console.log('Sent:', result.notificationId);
```

### SSE Real-Time Subscription

```javascript
const unsubscribe = client.sse.subscribe('user-uuid', (notification) => {
  console.log('Real-time notification:', notification);
});

// Later: clean up
unsubscribe();
```

### Available Methods

| Method | Description |
|--------|-------------|
| `client.notifications.send()` | Send a notification |
| `client.notifications.sendBulk()` | Send to multiple users |
| `client.notifications.broadcast()` | Broadcast to a topic |
| `client.notifications.list()` | List notifications |
| `client.templates.create()` | Create a template |
| `client.templates.list()` | List templates |
| `client.users.create()` | Register a user |
| `client.users.update()` | Update user |
| `client.topics.create()` | Create a topic |
| `client.topics.subscribe()` | Subscribe to topic |
| `client.sse.subscribe()` | Listen for real-time events |

---

## React SDK

### Installation

```bash
npm install @freerangenotify/react @freerangenotify/js
```

### Provider Setup

Wrap your app with `FreeRangeProvider`:

```jsx
import { FreeRangeProvider } from '@freerangenotify/react';

function App() {
  return (
    <FreeRangeProvider
      apiKey="YOUR_API_KEY"
      userId="current-user-uuid"
      baseURL="http://localhost:8080/v1"
    >
      <YourApp />
    </FreeRangeProvider>
  );
}
```

### NotificationBell Component

Drop-in notification bell with unread count and popover:

```jsx
import { NotificationBell } from '@freerangenotify/react';

function Header() {
  return (
    <nav>
      <NotificationBell
        position="bottom-right"
        onNotificationClick={(notification) => {
          console.log('Clicked:', notification);
        }}
      />
    </nav>
  );
}
```

### Hooks

```jsx
import {
  useNotifications,
  useUnreadCount,
  usePreferences,
} from '@freerangenotify/react';

function MyComponent() {
  const { notifications, loading, markAsRead } = useNotifications();
  const { count } = useUnreadCount();
  const { preferences, updatePreferences } = usePreferences();

  return (
    <div>
      <span>Unread: {count}</span>
      {notifications.map(n => (
        <div key={n.id} onClick={() => markAsRead(n.id)}>
          {n.title}
        </div>
      ))}
    </div>
  );
}
```

### Available Hooks

| Hook | Returns |
|------|---------|
| `useNotifications()` | `{ notifications, loading, error, markAsRead, markAllAsRead, refetch }` |
| `useUnreadCount()` | `{ count, loading }` |
| `usePreferences()` | `{ preferences, loading, updatePreferences }` |
| `useFreeRange()` | Raw client instance for direct API access |
