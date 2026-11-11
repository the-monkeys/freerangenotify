# SDK guide

Use the SDK on your backend with the application's full key. Set the API base to your hosted or self-hosted deployment, including `/v1` once. `environment` options are informational; the key selects the effective environment. SDKs do not promise automatic retries or exactly-once delivery for every operation.

## JavaScript and TypeScript

The package is **`@freerangenotify/sdk`**. The constructor takes a key first, then an options object.

```typescript
import { FreeRangeNotify } from '@freerangenotify/sdk';
const client = new FreeRangeNotify('YOUR_API_KEY', {
  baseURL: 'https://notify.example.test/v1',
});
const user = await client.users.create({
  email: 'alice@example.test', external_id: 'alice',
});
const template = await client.templates.create({
  app_id: 'YOUR_APP_ID', name: 'welcome_email', channel: 'email',
  subject: 'Welcome', body: 'Hello {{.name}}', variables: ['name'], locale: 'en',
});
const result = await client.notifications.send({
  user_id: user.user_id, template_id: template.id,
  channel: 'email', data: { name: 'Alice' },
});
console.log(result.notification_id, result.status);
```

Standard resource methods use wire-style `snake_case` request fields. The legacy `client.send(...)` convenience method uses quick-send fields such as `to`, `template` and camelCase `scheduledAt`; it targets `/quick-send`. These are separate request contracts. Keep working integrations on their existing method unless you deliberately migrate.

User methods return resource fields such as `user_id`; decoding supports both current success/data responses and older flat responses. Existing envelope properties remain available in JavaScript. Use `users.get`, `users.update` and `users.delete` for profile CRUD. Check endpoint-specific return shapes for preferences and devices.

## Go

Install/import `github.com/the-monkeys/freerangenotify/sdk/go/freerangenotify`. The constructor is `New`, and calls take a context.

```go
client := freerangenotify.New("YOUR_API_KEY",
    freerangenotify.WithBaseURL("https://notify.example.test/v1"))
result, err := client.Notifications.Send(ctx, freerangenotify.NotificationSendParams{
    UserID: "YOUR_USER_ID", TemplateID: "YOUR_TEMPLATE_ID", Channel: "email",
    Data: map[string]interface{}{"name": "Alice"},
})
if err != nil { return err }
fmt.Println(result.NotificationID)
```

The repository's compiled example includes the imports and executable entry point. `Client.Send` remains the quick-send convenience API; `Client.Notifications.Send` uses the standard endpoint.

## React SDK compatibility

`@freerangenotify/react` exports `FreeRangeProvider`, `NotificationBell`, `NotificationCenter`, `Preferences`, `useNotifications`, `usePreferences`, `useSSE`, `useUnreadCount` and `useFreeRange`.

The existing provider requires `apiKey` and `userId`; its base URL prop is `apiBaseURL`. It does **not** accept a token-only credential prop. Preserve existing trusted installations, but do not embed a master application key in a public browser bundle. For public customers use your backend for authorized inbox/CRUD calls and issue short-lived [SSE tokens](/docs/sse).

```tsx
// Existing provider API, for trusted environments only.
<FreeRangeProvider apiKey="YOUR_API_KEY" userId="YOUR_USER_ID"
  apiBaseURL="https://notify.example.test/v1">
  <NotificationBell />
</FreeRangeProvider>
```

`useNotifications({pageSize: 20})` returns `notifications`, `loading`, `unreadCount`, `markRead`, `markAllRead`, `archive`, `snooze`, `loadMore`, `hasMore` and `refresh`. Pass ID arrays to `markRead` and `archive`. `useFreeRange()` returns a context containing `client` and `userId`, not the raw client alone.

See [Users](/docs/users), [Notifications](/docs/notifications), [OTP](/docs/otp) and [File attachments](/docs/file-attachments) for endpoint behavior. HTTP acceptance remains distinct from provider delivery.
