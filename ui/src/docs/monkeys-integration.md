# Example: a blogging platform

This example uses safe placeholders. Follow [Getting started](/docs/getting-started) to create your app, obtain its owner-authorized key and register readers with stable `external_id` values. Keep the API key on the blogging backend and save the actual returned `data.user_id`.

## Send an event

Choose channels deliberately: `in_app` keeps an inbox record, `sse` signals an active browser, and `email` reaches the configured mailbox. They are separate notification operations with their own outcome and potential credit charge. One successful channel does not prove all channels succeeded.

```typescript
import { FreeRangeNotify } from '@freerangenotify/sdk';
const client = new FreeRangeNotify('YOUR_API_KEY', {
  baseURL: 'https://notify.example.test/v1',
});
const result = await client.notifications.send({
  user_id: 'YOUR_USER_ID', channel: 'in_app',
  title: 'New comment', body: 'Someone replied to your post.',
  data: { post_id: 'example-post' },
});
console.log(result.notification_id);
```

Use an email template for email and configure the selected app's SMTP or other email provider before sending. Handle each accepted/failed result instead of suppressing errors. Use idempotency only where the endpoint supports it; never promise exactly-once provider effects across a multi-channel event.

## Browser integration

Have the authenticated blogging backend request `/sse/tokens` for that reader. Give the browser only `sse_token`, then listen with `addEventListener('notification', ...)` as shown in [SSE](/docs/sse). On an event, your browser can refresh its inbox through your authorized backend. Close on logout and obtain a fresh token on expiry.

For inbox pagination, read/archive and preferences use [In-app](/docs/in-app), [Users](/docs/users) and [Notifications](/docs/notifications). Preserve existing customer IDs, SSE payloads and authentication paths while upgrading.
