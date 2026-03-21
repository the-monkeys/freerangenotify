# Getting Started with FreeRangeNotify

Welcome to FreeRangeNotify — a high-performance, universal notification service. This guide walks you through sending your first notification in under 5 minutes.

## Overview

FreeRangeNotify uses a **Hub-and-Spoke architecture** to decouple notification ingestion from delivery. The API Server (Hub) accepts requests and queues them; Workers (Spokes) handle rendering, routing, and delivery across every channel.

## Quick Start

### Step 1: Create an Application

Every notification belongs to an Application. Create one from the **Applications** page, or via the API:

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/apps/ \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "My App", "description": "Production notifications"}'
```

The response includes your **API Key** — save it. All subsequent calls use this key.

### Step 2: Register a User

Users represent notification recipients. Each user gets a unique internal UUID. You can also set your own `user_id` (e.g., your platform's username) which becomes the user's `external_id`.

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/users/ \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"email": "alice@example.com", "user_id": "alice_monkeys"}'
```

> **Tip:** When sending notifications, you can use either your `user_id` (e.g., `alice_monkeys`), the user's email, or the internal UUID. FreeRangeNotify resolves all of them automatically.

### Step 3: Create a Template

Templates define notification content. Use Go template variables like `{{.name}}` for dynamic content.

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/templates/ \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "app_id": "YOUR_APP_ID",
    "name": "welcome_email",
    "channel": "email",
    "subject": "Welcome, {{.name}}!",
    "body": "<h1>Hello {{.name}}</h1><p>Welcome to {{.product}}.</p>",
    "variables": ["name", "product"],
    "locale": "en"
  }'
```

### Step 4: Send Your First Notification

```bash
curl -X POST https://freerangenotify.monkeys.support/v1/notifications/ \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "YOUR_USER_ID",
    "channel": "email",
    "priority": "normal",
    "title": "Welcome!",
    "body": "Hello from FreeRangeNotify",
    "template_id": "YOUR_TEMPLATE_ID",
    "data": {"name": "Alice", "product": "Acme"}
  }'
```

### Step 5: Check Delivery

View the notification status in the **Dashboard** or query the API:

```bash
curl https://freerangenotify.monkeys.support/v1/notifications/NOTIFICATION_ID \
  -H "X-API-Key: YOUR_API_KEY"
```

Status progression: `pending` → `processing` → `sent` (or `failed`).

## Key Concepts

| Concept | Description |
|---------|-------------|
| **Application** | An isolated context with its own API key, users, and templates |
| **User** | A notification recipient with email, phone, preferences |
| **Template** | Reusable notification content with variable interpolation |
| **Channel** | Delivery method: email, push, webhook, SMS, SSE |
| **Priority** | Queue priority: `low`, `normal`, `high`, `critical` |
| **Worker** | Background processor that renders templates and delivers notifications |

## What's Next?

- **[Templates](/docs/templates)** — Learn about versioning, diff, and rollback
- **[Workflows](/docs/workflows)** — Build multi-step notification pipelines
- **[Channels](/docs/channels)** — Configure email, push, webhook, SMS, and SSE
- **[SDK Guide](/docs/sdk)** — Integrate with Go, JavaScript, or React
