# Topics & Subscriptions

Topics enable fan-out notifications — send one message and deliver it to all subscribed users. This is ideal for announcements, feature updates, and broadcast alerts.

## Creating Topics

Topics are identified by a unique `topic_key` and belong to an application.

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/topics/ \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "topic_key": "product_updates",
    "name": "Product Updates",
    "description": "New features and release announcements"
  }'
```

## Managing Subscriptions

### Subscribe a User

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/topics/TOPIC_ID/subscribers \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"user_id": "YOUR_USER_ID"}'
```

### Unsubscribe a User

```bash
curl -X DELETE https://freerangenotify.monkeys.support/v1/topics/TOPIC_ID/subscribers/YOUR_USER_ID \
  -H "X-API-Key: YOUR_API_KEY"
```

### List Subscribers

```bash
curl https://freerangenotify.monkeys.support/v1/topics/TOPIC_ID/subscribers \
  -H "X-API-Key: YOUR_API_KEY"
```

## Broadcasting

Send a notification to all users subscribed to a topic:

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/notifications/broadcast \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "topic_key": "product_updates",
    "channel": "email",
    "priority": "normal",
    "title": "New Feature: Workflows",
    "body": "We just launched multi-step workflows!",
    "data": {"feature": "workflows"}
  }'
```

Each subscriber receives an individual notification, processed through the same Worker pipeline (template rendering, preference checking, delivery).

## Best Practices

- **Keep topic keys descriptive:** Use patterns like `billing_alerts`, `security_updates`, `product_news`
- **Respect preferences:** Users can opt out of specific topics via their preference settings
- **Don't over-broadcast:** High-frequency broadcasts increase queue depth — use digest workflows for batching

## API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/topics/` | Create a topic |
| GET | `/v1/topics/` | List all topics |
| GET | `/v1/topics/:id` | Get topic details |
| DELETE | `/v1/topics/:id` | Delete a topic |
| POST | `/v1/topics/:id/subscribers` | Subscribe a user |
| DELETE | `/v1/topics/:id/subscribers/:user_id` | Unsubscribe a user |
| GET | `/v1/topics/:id/subscribers` | List subscribers |
