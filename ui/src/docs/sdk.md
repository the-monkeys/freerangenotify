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
    client := frn.NewClient("YOUR_API_KEY", frn.WithBaseURL("https://freerangenotify.monkeys.support/v1"))

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

### Webhook Rich Content (Discord / Slack / Teams)

The Go SDK ships builder helpers for the rich-content webhook channel. Set
`webhook_target` to the **Name** of a custom provider registered on the app
and chain rich fields fluently:

```go
import frn "github.com/the-monkeys/freerangenotify/sdk/go/freerangenotify"

p := frn.NewSlackAlert("Slack Alerts", "DB CPU > 90%", "Sustained on db-prod-1.", "danger").
    WithFields(
        frn.ContentField{Key: "Host", Value: "db-prod-1", Inline: true},
        frn.ContentField{Key: "Region", Value: "us-east-1", Inline: true},
    ).
    WithActions(
        frn.ContentAction{Type: "link", Label: "Acknowledge", URL: "https://oncall.example.com/ack/123", Style: "primary"},
        frn.ContentAction{Type: "link", Label: "Runbook",     URL: "https://wiki.example.com/runbooks/db-cpu"},
    ).
    To("user-uuid")

_, err := client.Notifications.Send(ctx, p)
```

Available factories: `NewWebhookNotification`, `NewDiscordAlert`,
`NewSlackAlert`, `NewTeamsAlert`. Chainable methods on
`NotificationSendParams`: `WithFields`, `WithActions`, `WithAttachments`,
`WithMentions`, `WithPoll(question, choices...)`, `WithSeverity`,
`WithColor(hex)`, `To(userID)`.

Per-target capability matrix:

| Field         | Discord  | Slack       | Teams      | Generic |
| ------------- | -------- | ----------- | ---------- | ------- |
| `Attachments` | yes      | yes         | yes        | raw     |
| `Actions`     | mdlinks  | buttons     | buttons    | raw     |
| `Fields`      | yes      | yes         | FactSet    | raw     |
| `Poll`        | native   | numbered    | numbered   | raw     |
| `Style.Color` | yes      | sidebar bar | themeColor | n/a     |
| `Mentions`    | yes      | yes         | yes        | n/a     |

Discord incoming webhooks drop interactive components, so `Actions` render as
a markdown link list. Slack and Microsoft Teams have no native poll element
on incoming webhooks — `Poll` falls back to a numbered list of choices.

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
  baseURL: 'https://freerangenotify.monkeys.support/v1',
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

### Webhook Rich Content (Discord / Slack / Teams)

The JavaScript SDK exports a `webhook` namespace plus standalone factory
functions for the rich-content webhook channel:

```ts
import { FreeRangeNotify, webhook } from '@freerangenotify/sdk';

const client = new FreeRangeNotify('YOUR_API_KEY', { baseURL: '...' });

await client.notifications.send({
    ...webhook.slack('Slack Alerts', 'DB CPU > 90%', 'Sustained on db-prod-1.', 'danger'),
    user_id: 'user-uuid',
    fields: [
        { key: 'Host',   value: 'db-prod-1', inline: true },
        { key: 'Region', value: 'us-east-1', inline: true },
    ],
    actions: [
        { type: 'link', label: 'Acknowledge', url: 'https://oncall.example.com/ack/123', style: 'primary' },
        { type: 'link', label: 'Runbook',     url: 'https://wiki.example.com/runbooks/db-cpu' },
    ],
});
```

Available factories on the `webhook` namespace (or as named exports):

| Function | Purpose |
|----------|---------|
| `webhook.notification(target, title, body)` | Generic webhook |
| `webhook.discord(target, title, body, severity?)` | Discord alert with embed color |
| `webhook.slack(target, title, body, severity?)` | Slack alert with sidebar bar |
| `webhook.teams(target, title, body, severity?)` | Teams alert with themeColor |
| `webhook.withPoll(params, question, choices)` | Attach a poll to existing params |

Rich fields (`attachments`, `actions`, `fields`, `mentions`, `poll`,
`style`) are top-level keys on the request payload. Capability matrix and
fallback behaviour are identical to the Go SDK above.

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
      baseURL="https://freerangenotify.monkeys.support/v1"
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
