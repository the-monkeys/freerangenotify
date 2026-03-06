# FreeRangeNotify JavaScript/TypeScript SDK

Official SDK for the [FreeRangeNotify](https://github.com/the-monkeys/freerangenotify) notification service.

Uses a **sub-client pattern** (like Stripe, Twilio, Novu) for resource-oriented API access with full backward compatibility.

## Installation

```bash
npm install @freerangenotify/sdk
```

## Quick Start

```typescript
import { FreeRangeNotify } from '@freerangenotify/sdk';

const client = new FreeRangeNotify('frn_xxx', {
  baseURL: 'http://localhost:8080/v1',
});

// Quick-Send (backward-compatible)
await client.send({
  to: 'user@example.com',
  template: 'welcome_email',
  data: { name: 'Alice' },
});

// Sub-client pattern
await client.notifications.send({
  user_id: 'user-uuid',
  channel: 'email',
  template_id: 'welcome_email',
});
```

## Sub-Clients

### Notifications (18 methods)

```typescript
// Quick-Send
await client.notifications.quickSend({ to: 'user@example.com', template: 'welcome' });

// Standard Send
await client.notifications.send({ user_id: 'uuid', channel: 'webhook', template_id: 'order_confirmation' });

// Bulk & Batch
await client.notifications.sendBulk({ user_ids: ['a', 'b'], template_id: 'alert' });
await client.notifications.sendBatch([{ user_id: 'a', ... }, { user_id: 'b', ... }]);

// Broadcast
await client.notifications.broadcast({ template: 'maintenance', data: { downtime: '2h' } });

// List & Get
const list = await client.notifications.list({ userId: 'uuid', page: 1, unreadOnly: true });
const notif = await client.notifications.get('notification-id');

// Unread
const count = await client.notifications.getUnreadCount('user-uuid');
const unread = await client.notifications.listUnread('user-uuid', 1, 20);

// Mark Read
await client.notifications.markRead('user-uuid', ['notif-1', 'notif-2']);
await client.notifications.markAllRead('user-uuid');

// Status & Lifecycle
await client.notifications.updateStatus('notif-id', 'delivered');
await client.notifications.cancel('notif-id');
await client.notifications.cancelBatch(['notif-1', 'notif-2']);
await client.notifications.retry('notif-id');

// Snooze & Archive
await client.notifications.snooze('notif-id', '1h');
await client.notifications.unsnooze('notif-id');
await client.notifications.archive('user-uuid', ['notif-1', 'notif-2']);
```

### Users (12 methods)

```typescript
const user = await client.users.create({ email: 'alice@example.com' });
const fetched = await client.users.get(user.user_id);
await client.users.update(user.user_id, { timezone: 'UTC' });
await client.users.delete(user.user_id);
const users = await client.users.list(1, 20);
await client.users.bulkCreate([{ email: 'a@x.com' }, { email: 'b@x.com' }]);

// Devices (Apple Push Notification service / Firebase Cloud Messaging)
await client.users.addDevice(userId, { platform: 'ios', token: '...' });
const devices = await client.users.getDevices(userId);
await client.users.removeDevice(userId, deviceId);

// Preferences
const prefs = await client.users.getPreferences(userId);
await client.users.updatePreferences(userId, { email_enabled: true });

// Subscriber Hash (HMAC for SSE)
const hash = await client.users.getSubscriberHash(userId);
```

### Templates (13 methods)

```typescript
const tmpl = await client.templates.create({ app_id: 'app-id', name: 'welcome', channel: 'email', body: '<h1>Hello</h1>' });
await client.templates.get(tmpl.id);
await client.templates.update(tmpl.id, { body: '...' });
await client.templates.delete(tmpl.id);
await client.templates.list({ channel: 'email' });

// Library
const library = await client.templates.getLibrary('transactional');
await client.templates.cloneFromLibrary('welcome_default', 'app-id');

// Versioning
await client.templates.getVersions('app-id', 'welcome');
await client.templates.createVersion('app-id', 'welcome', { body: '...' });
await client.templates.rollback(tmpl.id, 1, 'admin@example.com');

// Diff & Render
await client.templates.diff(tmpl.id, 1, 2);
const rendered = await client.templates.render(tmpl.id, { name: 'Alice' });
await client.templates.sendTest(tmpl.id, 'test@example.com', { name: 'Test' });
```

### Workflows (9 methods)

```typescript
const wf = await client.workflows.create({ name: 'onboarding', trigger_id: 'signup', steps: [...] });
await client.workflows.get(wf.id);
await client.workflows.update(wf.id, { status: 'active' });
await client.workflows.delete(wf.id);
await client.workflows.list(1, 20);

// Trigger & Executions
const exec = await client.workflows.trigger({ trigger_id: 'signup', user_id: 'uuid' });
await client.workflows.getExecution(exec.id);
await client.workflows.listExecutions(1, 20);
await client.workflows.cancelExecution(exec.id);
```

### Topics (8 methods)

```typescript
const topic = await client.topics.create({ name: 'Releases', key: 'releases' });
await client.topics.get(topic.id);
await client.topics.getByKey('releases');
await client.topics.delete(topic.id);
await client.topics.list(1, 20);

// Subscribers
await client.topics.addSubscribers(topic.id, ['user-1', 'user-2']);
await client.topics.removeSubscribers(topic.id, ['user-1']);
await client.topics.getSubscribers(topic.id, 1, 20);
```

### Presence (1 method)

```typescript
await client.presence.checkIn({ user_id: 'uuid', webhook_url: 'http://localhost:9999/webhook' });
```

## SSE (Real-time)

```typescript
const conn = client.connectSSE('user-uuid', {
  onNotification: (n) => console.log(n.title, n.body),
  onConnected: () => console.log('Connected'),
  onConnectionChange: (connected) => console.log('Connection:', connected),
  onUnreadCountChange: (count) => console.log('Unread:', count),
  subscriberHash: 'hmac-hash',
  autoReconnect: true,
  reconnectInterval: 3000,
});

// Later: conn.close();
```

## Error Handling

```typescript
import { FreeRangeNotifyError } from '@freerangenotify/sdk';

try {
  await client.notifications.send({ ... });
} catch (err) {
  if (err instanceof FreeRangeNotifyError) {
    console.error(`API Error ${err.status}: ${err.body}`);
    if (err.isNotFound) { /* 404 */ }
    if (err.isUnauthorized) { /* 401 */ }
    if (err.isRateLimited) { /* 429 */ }
    if (err.isValidationError) { /* 400/422 */ }
  }
}
```

## Backward Compatibility

| Legacy Method | Delegates To |
|---|---|
| `client.send(params)` | `client.notifications.quickSend(params)` |
| `client.broadcast(params)` | `client.notifications.broadcast(params)` |
| `client.createUser(params)` | `client.users.create(params)` |
| `client.updateUser(id, params)` | `client.users.update(id, params)` |
| `client.listUsers(page, size)` | `client.users.list(page, size)` |

## File Structure

```
sdk/js/src/
├── index.ts           # FreeRangeNotify class + re-exports
├── client.ts          # Core HTTP transport
├── notifications.ts   # NotificationsClient (18 methods)
├── users.ts           # UsersClient (12 methods)
├── templates.ts       # TemplatesClient (13 methods)
├── workflows.ts       # WorkflowsClient (9 methods)
├── topics.ts          # TopicsClient (8 methods)
├── presence.ts        # PresenceClient (1 method)
├── sse.ts             # SSE connection with auto-reconnect
├── types.ts           # All TypeScript interfaces
└── errors.ts          # FreeRangeNotifyError with helpers
```
