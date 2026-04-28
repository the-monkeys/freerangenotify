# FreeRangeNotify Go SDK

Official Go client for the [FreeRangeNotify](https://github.com/the-monkeys/freerangenotify) notification service.

Uses a **sub-client pattern** (like Stripe, Twilio, leading platforms) for resource-oriented API access with full backward compatibility.

## Installation

```bash
go get github.com/the-monkeys/freerangenotify/sdk/go/freerangenotify
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    frn "github.com/the-monkeys/freerangenotify/sdk/go/freerangenotify"
)

func main() {
    client := frn.New("frn_your-api-key",
        frn.WithBaseURL("http://localhost:8080/v1"),
    )

    ctx := context.Background()

    // Quick-Send (backward-compatible top-level method)
    result, err := client.Send(ctx, frn.SendParams{
        To:       "user@example.com",
        Template: "welcome_email",
        Data:     map[string]interface{}{"name": "Alice"},
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Sent: %s (status: %s)\n", result.NotificationID, result.Status)
}
```

## Sub-Clients

### Notifications

```go
// Quick-Send
client.Notifications.QuickSend(ctx, frn.SendParams{...})

// Standard Send
client.Notifications.Send(ctx, frn.NotificationSendParams{
    UserID:     "user-uuid",
    Channel:    "webhook",
    TemplateID: "order_confirmation",
    Data:       map[string]interface{}{"order_id": "12345"},
})

// Bulk & Batch Send
client.Notifications.SendBulk(ctx, frn.BulkSendParams{UserIDs: []string{"a", "b"}, ...})
client.Notifications.SendBatch(ctx, []frn.NotificationSendParams{...})

// Broadcast
client.Notifications.Broadcast(ctx, frn.BroadcastParams{Template: "maintenance"})

// List & Get
list, _ := client.Notifications.List(ctx, frn.ListNotificationsOptions{UserID: "uuid", Page: 1})
notif, _ := client.Notifications.Get(ctx, "notification-id")

// Unread
count, _ := client.Notifications.GetUnreadCount(ctx, "user-uuid")
unread, _ := client.Notifications.ListUnread(ctx, "user-uuid", 1, 20)

// Mark Read
client.Notifications.MarkRead(ctx, "user-uuid", []string{"notif-1", "notif-2"})
client.Notifications.MarkAllRead(ctx, "user-uuid", "")  // empty category = all

// Status & Lifecycle
client.Notifications.UpdateStatus(ctx, "notif-id", "delivered", "")
client.Notifications.Cancel(ctx, "notif-id")
client.Notifications.CancelBatch(ctx, []string{"notif-1", "notif-2"})
client.Notifications.Retry(ctx, "notif-id")

// Snooze & Archive
client.Notifications.Snooze(ctx, "notif-id", "1h")
client.Notifications.Unsnooze(ctx, "notif-id")
client.Notifications.Archive(ctx, "user-uuid", []string{"notif-1", "notif-2"})
```

### Users

```go
user, _ := client.Users.Create(ctx, frn.CreateUserParams{Email: "alice@example.com"})
user, _ = client.Users.Get(ctx, user.UserID)
user, _ = client.Users.Update(ctx, user.UserID, frn.UpdateUserParams{Timezone: "UTC"})
client.Users.Delete(ctx, user.UserID)

list, _ := client.Users.List(ctx, 1, 20)
result, _ := client.Users.BulkCreate(ctx, []frn.CreateUserParams{...})

// Devices (Apple Push Notification service / Firebase Cloud Messaging)
device, _ := client.Users.AddDevice(ctx, userID, frn.AddDeviceParams{Platform: "ios", Token: "..."})
devices, _ := client.Users.GetDevices(ctx, userID)
client.Users.RemoveDevice(ctx, userID, device.DeviceID)

// Preferences
prefs, _ := client.Users.GetPreferences(ctx, userID)
client.Users.UpdatePreferences(ctx, userID, frn.Preferences{EmailEnabled: boolPtr(true)})

// Subscriber Hash (HMAC for SSE authentication)
hash, _ := client.Users.GetSubscriberHash(ctx, userID)
```

### Templates

```go
tmpl, _ := client.Templates.Create(ctx, frn.CreateTemplateParams{
    AppID: "app-id", Name: "welcome", Channel: "email", Body: "<h1>Hello {{.name}}</h1>",
})
tmpl, _ = client.Templates.Get(ctx, tmpl.ID)
tmpl, _ = client.Templates.Update(ctx, tmpl.ID, frn.UpdateTemplateParams{Body: "..."})
client.Templates.Delete(ctx, tmpl.ID)

list, _ := client.Templates.List(ctx, frn.ListTemplatesOptions{Channel: "email"})

// Library
library, _ := client.Templates.GetLibrary(ctx, "transactional")
cloned, _ := client.Templates.CloneFromLibrary(ctx, "welcome_default", frn.CloneTemplateParams{AppID: "app-id"})

// Versioning
versions, _ := client.Templates.GetVersions(ctx, "app-id", "welcome")
newVer, _ := client.Templates.CreateVersion(ctx, "app-id", "welcome", frn.CreateVersionParams{Body: "..."})
rolled, _ := client.Templates.Rollback(ctx, tmpl.ID, 1, "admin@example.com")

// Diff & Render
diff, _ := client.Templates.Diff(ctx, tmpl.ID, 1, 2)
rendered, _ := client.Templates.Render(ctx, tmpl.ID, map[string]interface{}{"name": "Alice"})
client.Templates.SendTest(ctx, tmpl.ID, "test@example.com", map[string]interface{}{"name": "Test"})
```

### Workflows

```go
wf, _ := client.Workflows.Create(ctx, frn.CreateWorkflowParams{
    Name: "onboarding", TriggerID: "user_signup",
    Steps: []frn.WorkflowStep{{Type: "channel", Channel: "email", TemplateID: "welcome"}},
})
wf, _ = client.Workflows.Get(ctx, wf.ID)
wf, _ = client.Workflows.Update(ctx, wf.ID, frn.UpdateWorkflowParams{Status: "active"})
client.Workflows.Delete(ctx, wf.ID)

list, _ := client.Workflows.List(ctx, 1, 20)

// Trigger & Executions
exec, _ := client.Workflows.Trigger(ctx, frn.TriggerWorkflowParams{
    TriggerID: "user_signup", UserID: "user-uuid",
    Payload: map[string]interface{}{"plan": "pro"},
})
exec, _ = client.Workflows.GetExecution(ctx, exec.ID)
execs, _ := client.Workflows.ListExecutions(ctx, 1, 20)
client.Workflows.CancelExecution(ctx, exec.ID)
```

### Topics

```go
topic, _ := client.Topics.Create(ctx, frn.CreateTopicParams{Name: "Releases", Key: "releases"})
topic, _ = client.Topics.Get(ctx, topic.ID)
topic, _ = client.Topics.GetByKey(ctx, "releases")
client.Topics.Delete(ctx, topic.ID)

list, _ := client.Topics.List(ctx, 1, 20)

// Subscribers
client.Topics.AddSubscribers(ctx, topic.ID, []string{"user-1", "user-2"})
client.Topics.RemoveSubscribers(ctx, topic.ID, []string{"user-1"})
subs, _ := client.Topics.GetSubscribers(ctx, topic.ID, 1, 20)
```

### Presence

```go
// Smart Delivery check-in
client.Presence.CheckIn(ctx, frn.CheckInParams{
    UserID:     "user-uuid",
    WebhookURL: "http://localhost:9999/webhook",
})
```

## Error Handling

```go
result, err := client.Notifications.Send(ctx, params)
if err != nil {
    var apiErr *frn.APIError
    if errors.As(err, &apiErr) {
        fmt.Printf("API Error %d: %s\n", apiErr.StatusCode, apiErr.Body)

        if apiErr.IsNotFound() { /* 404 */ }
        if apiErr.IsUnauthorized() { /* 401 */ }
        if apiErr.IsRateLimited() { /* 429 */ }
        if apiErr.IsValidationError() { /* 400/422 */ }
    }
}
```

## Backward Compatibility

Top-level convenience methods delegate to sub-clients:

| Legacy Method | Delegates To |
|---|---|
| `client.Send(ctx, params)` | `client.Notifications.QuickSend(ctx, params)` |
| `client.Broadcast(ctx, params)` | `client.Notifications.Broadcast(ctx, params)` |
| `client.CreateUser(ctx, params)` | `client.Users.Create(ctx, params)` |
| `client.UpdateUser(ctx, id, params)` | `client.Users.Update(ctx, id, params)` |

## Options

| Option | Description |
|--------|-------------|
| `WithBaseURL(url)` | Set custom API base URL (default: `http://localhost:8080/v1`) |
| `WithHTTPClient(client)` | Provide a custom `*http.Client` for transports, TLS, etc. |
| `WithTimeout(duration)` | Override the default 30s HTTP timeout |

## File Structure

```
sdk/go/freerangenotify/
├── client.go          # Core client, HTTP transport, sub-client wiring
├── notifications.go   # NotificationsClient (18 methods)
├── users.go           # UsersClient (12 methods)
├── templates.go       # TemplatesClient (13 methods)
├── workflows.go       # WorkflowsClient (9 methods)
├── topics.go          # TopicsClient (8 methods)
├── presence.go        # PresenceClient (1 method)
├── types.go           # All shared types and models
├── errors.go          # APIError with helper methods
└── README.md          # This file
```
