# Workflows

Workflows let you build multi-step notification pipelines that trigger automatically based on events. They support delays, conditions, digests, and multi-channel delivery.

## Concepts

| Concept | Description |
|---------|-------------|
| **Workflow** | A reusable pipeline definition with ordered steps |
| **Trigger** | An event that starts a workflow execution |
| **Step** | A single action: send notification, wait, check condition, or digest |
| **Execution** | A running instance of a workflow for a specific user |

## Step Types

### Channel Step

Sends a notification on a specific channel (email, push, webhook, SMS, or SSE).

- Select the template to use
- Choose priority level
- Optionally set custom data overrides

### Delay Step

Pauses execution for a specified duration before continuing.

- Duration: seconds, minutes, hours, or days
- Use cases: follow-up reminders, cooling periods

### Condition Step

Evaluates an expression and branches the workflow.

- Expression syntax: simple boolean conditions
- Skip remaining steps if condition fails

### Digest Step

Aggregates multiple notifications into a single summary.

- Batch window: 5 minutes to 24 hours
- Combines all pending notifications for a user
- Reduces notification fatigue

## Creating Workflows

### Via the Dashboard

1. Navigate to **Workflows** → **Create New**
2. Add a trigger name (e.g., `order_placed`)
3. Add steps using the visual builder
4. Configure each step's template, channel, and timing
5. Save and activate

### Via the API

```bash
curl -X POST http://localhost:8080/v1/workflows/ \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Welcome Series",
    "trigger": "user_signup",
    "steps": [
      {"type": "channel", "channel": "email", "template_id": "welcome_email"},
      {"type": "delay", "duration": "24h"},
      {"type": "channel", "channel": "email", "template_id": "getting_started"}
    ]
  }'
```

## Triggering Workflows

Send a trigger event to start a workflow execution:

```bash
curl -X POST http://localhost:8080/v1/workflows/WORKFLOW_ID/trigger \
  -H "X-API-Key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "USER_UUID",
    "data": {"user_name": "Alice", "plan": "Pro"}
  }'
```

## Monitoring Executions

### Execution Timeline

Each execution shows a step-by-step timeline with:

- Step status: pending, running, completed, failed, skipped
- Timestamps for each step
- Error details for failed steps

### Controls

- **Pause** — Temporarily halt execution
- **Resume** — Continue a paused execution
- **Cancel** — Stop execution permanently

### Via the Dashboard

Navigate to **Workflows** → **Executions** for a filterable list of all running and completed executions.

## API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/workflows/` | Create a workflow |
| GET | `/v1/workflows/` | List all workflows |
| GET | `/v1/workflows/:id` | Get workflow details |
| PUT | `/v1/workflows/:id` | Update a workflow |
| DELETE | `/v1/workflows/:id` | Delete a workflow |
| POST | `/v1/workflows/:id/trigger` | Trigger a workflow |
| GET | `/v1/workflows/executions` | List executions |
| POST | `/v1/workflows/executions/:id/cancel` | Cancel an execution |
