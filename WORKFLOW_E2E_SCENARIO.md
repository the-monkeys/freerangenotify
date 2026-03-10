# Workflow & Digest E2E Test Scenario

## What is Workflow?

A **Workflow** is a multi-step notification pipeline. Instead of sending one notification at a time, you define a sequence of steps that run automatically when triggered.

### Step Types

| Type | Purpose |
|------|---------|
| **Channel** | Send a notification (email, push, webhook, SSE, etc.) using a template |
| **Delay** | Wait for a duration (e.g. `5m`, `24h`) before continuing |
| **Condition** | Branch on a condition (e.g. `payload.plan == "pro"` → different path) |
| **Digest** | Aggregate events over a time window (batched notifications) |

### Practical Uses

- **Welcome series**: Send welcome email → wait 24h → send "getting started" tips
- **Order confirmation**: Immediate confirmation email → delay 3 days → follow-up "how's your purchase?" email
- **Activity digest**: Instead of 10 separate notifications, batch them into one "You have 10 updates" email
- **Re-engagement**: Trigger on abandoned cart → delay 2h → send reminder email

### How It Works

1. You create a workflow (draft) with steps.
2. Activate it (`status: "active"`).
3. Your backend calls `POST /v1/workflows/trigger` with `trigger_id`, `user_id`, and `payload`.
4. The **worker** processes the workflow queue, executes each step (sends notifications via the normal pipeline), and advances to the next step.

**Important:** The **worker process** (`cmd/worker`) must be running. The API enqueues work; the worker executes it.

---

## Prerequisites

- **Both** `notification-service` (API) **and** `notification-worker` running
- Feature flags: `workflow_enabled: true`, `digest_enabled: true` (already set in `config/config.yaml`)
- **Email (SMTP)** configured for a real delivery test, or **webhook** for a quick end-to-end check without email

### SMTP (Email)

Set in `.env` or environment:

```
FREERANGE_PROVIDERS_SMTP_HOST=smtp.example.com
FREERANGE_PROVIDERS_SMTP_PORT=587
FREERANGE_PROVIDERS_SMTP_USERNAME=you@example.com
FREERANGE_PROVIDERS_SMTP_PASSWORD=...
FREERANGE_PROVIDERS_SMTP_FROM_EMAIL=noreply@example.com
FREERANGE_PROVIDERS_SMTP_FROM_NAME=FreeRangeNotify
```

---

## Test Scenario 1: Workflow with Email Channel

### Step 1: Create an App and Get API Key

1. Log in to the dashboard at `http://localhost:3000`
2. Go to **Applications** → **+ New Application**
3. Create an app (e.g. `Workflow Test App`)
4. Copy the **API Key** (e.g. `frn_xxx`) — you'll need it for the trigger

### Step 2: Create a User (with email for email channel)

Users are per-application. Create one via the API or via the app’s Users tab.

```bash
curl -X POST "http://localhost:8080/v1/users/" \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "external_id": "alice-001",
    "email": "alice@example.com",
    "preferences": { "email_enabled": true }
  }'
```

Save the returned `user_id` (UUID).

### Step 3: Create an Email Template

Templates are tied to the app. Create one with `channel: "email"`:

```bash
curl -X POST "http://localhost:8080/v1/templates/" \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "welcome_email",
    "channel": "email",
    "content": {
      "subject": "Welcome, {{.name}}!",
      "body": "Hello {{.name}}, thanks for signing up. Your plan: {{.plan}}.",
      "html_body": "<h1>Welcome!</h1><p>Hello {{.name}}, thanks for signing up. Your plan: {{.plan}}.</p>"
    }
  }'
```

Save the returned `template_id` (or use the template `id` from the list).

### Step 4: Create the Workflow

```bash
curl -X POST "http://localhost:8080/v1/workflows/" \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Welcome Email",
    "trigger_id": "user_signup",
    "description": "Sends welcome email on signup",
    "steps": [
      {
        "id": "step-1",
        "name": "Send welcome email",
        "type": "channel",
        "order": 1,
        "config": {
          "channel": "email",
          "template_id": "TEMPLATE_ID_FROM_STEP_3"
        },
        "on_success": ""
      }
    ]
  }'
```

Save the workflow `id`. Then **activate** it:

```bash
curl -X PUT "http://localhost:8080/v1/workflows/WORKFLOW_ID" \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "status": "active" }'
```

### Step 5: Trigger the Workflow

```bash
curl -X POST "http://localhost:8080/v1/workflows/trigger" \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "trigger_id": "user_signup",
    "user_id": "USER_ID_FROM_STEP_2",
    "payload": {
      "name": "Alice",
      "plan": "Pro"
    }
  }'
```

### Step 6: Verify

1. **Execution**: `GET /v1/workflows/executions` — you should see an execution with `status: "completed"`.
2. **Email**: If SMTP is configured, Alice should receive the welcome email at `alice@example.com`.
3. **Worker logs**: `docker-compose logs notification-worker` — look for "Workflow triggered", "Channel step", and delivery.

---

## Test Scenario 2: Workflow with Delay (Welcome + Follow-up)

Add a second step with a short delay (e.g. 1 minute for testing). **Important:** Each step must have `on_success` set to the ID of the next step, or the workflow can get stuck (the delay step would re-enqueue itself instead of advancing).

```json
{
  "steps": [
    {
      "id": "step-1",
      "name": "Send welcome",
      "type": "channel",
      "order": 1,
      "config": { "channel": "email", "template_id": "..." },
      "on_success": "step-2"
    },
    {
      "id": "step-2",
      "name": "Wait 1 min",
      "type": "delay",
      "order": 2,
      "config": { "duration": "1m" },
      "on_success": "step-3"
    },
    {
      "id": "step-3",
      "name": "Send follow-up",
      "type": "channel",
      "order": 3,
      "config": { "channel": "email", "template_id": "..." },
      "on_success": ""
    }
  ]
}
```

Trigger once. After ~1 minute, the second email should be sent.

---

## Test Scenario 3: Digest Rule (Batch Notifications)

Digest rules accumulate notifications that have `metadata.digest_key` and deliver them in a batch after a time window.

### Step 1: Create Digest Rule

```bash
curl -X POST "http://localhost:8080/v1/digest-rules/" \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Daily activity digest",
    "digest_key": "daily_activity",
    "window": "1m",
    "channel": "in_app",
    "template_id": ""
  }'
```

**Note:** The current digest flush sends to `in_app` by default. For email digest you’d need the flush logic to use the rule’s channel and template.

### Step 2: Send Notifications with `digest_key`

When sending notifications, include `metadata.digest_key` to route them into the digest:

```bash
curl -X POST "http://localhost:8080/v1/notifications/" \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "USER_ID",
    "channel": "in_app",
    "title": "New comment",
    "body": "Someone commented on your post",
    "metadata": { "digest_key": "daily_activity" }
  }'
```

Send 2–3 such notifications. After the window (e.g. 1 minute), the worker’s digest flush will send one batched notification.

---

## Quick Checklist

| Item | Status |
|------|--------|
| `notification-service` running | |
| `notification-worker` running | |
| `workflow_enabled: true` | |
| `digest_enabled: true` | |
| SMTP configured (for email) | |
| App created + API key | |
| User created with `email` | |
| Template created (`channel: email`) | |
| Workflow created + activated | |
| Trigger called | |
| Email received / execution completed | |

---

## Troubleshooting

- **Workflow not running:** Ensure the worker is running and `workflow_enabled: true`.
- **No email received:** Check SMTP env vars and worker logs for provider errors.
- **User not found:** `user_id` must be the internal UUID from the users index.
- **Template not found:** Use the exact `template_id` (or template ID) from your app.
- **First email works but second never arrives (delay step):** The delay step must have `on_success` set to the next step's ID. If empty, the engine used to re-enqueue the delay step itself, causing an infinite loop. As of the fix, it falls back to the next step by order. Check worker logs for "Delay step scheduled" and "Delayed workflow item moved to queue".
