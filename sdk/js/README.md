# FreeRangeNotify JavaScript/TypeScript SDK

Official SDK for the [FreeRangeNotify](https://github.com/the-monkeys/freerangenotify) notification service.

## Installation

```bash
npm install @freerangenotify/sdk
```

## Quick Start

```typescript
import { FreeRangeNotify } from '@freerangenotify/sdk';

const client = new FreeRangeNotify('your-api-key', {
  baseURL: 'http://localhost:8080/v1',
});

// Send a notification using a template
await client.send({
  to: 'user@example.com',
  template: 'welcome_email',
  data: { name: 'Alice', product: 'MyApp' },
});

// Send a plain notification (no template)
await client.send({
  to: 'user@example.com',
  subject: 'Hello!',
  body: '<h1>Hello, World!</h1>',
  channel: 'email',
});

// Broadcast to all users
await client.broadcast({
  template: 'maintenance_notice',
  data: { downtime: '2 hours' },
});

// Create a user
const user = await client.createUser({
  email: 'alice@example.com',
  timezone: 'America/New_York',
});
```

## API

### `new FreeRangeNotify(apiKey, options?)`

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `apiKey` | `string` | required | Your application API key |
| `options.baseURL` | `string` | `http://localhost:8080/v1` | API base URL |

### `client.send(params)`

Send a notification to a single recipient via Quick-Send.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `to` | `string` | yes | Recipient email, phone, or user ID |
| `template` | `string` | no | Template name to use |
| `subject` | `string` | no | Subject (for email/push) |
| `body` | `string` | no | Body content |
| `data` | `object` | no | Template variables |
| `channel` | `string` | no | Force channel (email, sms, push, webhook, sse) |
| `priority` | `string` | no | `low`, `normal`, `high`, or `critical` |
| `scheduledAt` | `Date` | no | Schedule for future delivery |

### `client.broadcast(params)`

Broadcast a notification to all application users.

### `client.createUser(params)`

Create a user profile.

### `client.listUsers(page?, pageSize?)`

List users in the application.

## Error Handling

```typescript
import { FreeRangeNotifyError } from '@freerangenotify/sdk';

try {
  await client.send({ to: 'user@example.com', body: 'Hello' });
} catch (err) {
  if (err instanceof FreeRangeNotifyError) {
    console.error(`API Error ${err.status}: ${err.body}`);
  }
}
```
