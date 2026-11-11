# Workflows

Workflows must be enabled by the deployment. Their customer API uses an application key. Use existing recipient/template IDs from [Getting started](/docs/getting-started), then create and activate a definition before triggering it.

A workflow has `trigger_id`, an explicit status and ordered steps. Channel, delay, condition and digest settings belong inside each step's `config`. A definition defaults to draft if status is omitted; a draft cannot be triggered.

## Create workflow

Create an active workflow. Save `data.id` as `WORKFLOW_ID`.

<!-- api-example:create-workflow:request -->
```json
{
  "name": "Welcome workflow",
  "trigger_id": "welcome-v1",
  "status": "active",
  "steps": [
    {
      "name": "Email",
      "type": "channel",
      "config": {
        "channel": "email",
        "template_id": "${TEMPLATE_ID}"
      }
    }
  ]
}
```

<!-- api-example:create-workflow:curl -->
```bash
curl -sS -X POST "$BASE_URL/workflows" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "name": "Welcome workflow",
  "trigger_id": "welcome-v1",
  "status": "active",
  "steps": [
    {
      "name": "Email",
      "type": "channel",
      "config": {
        "channel": "email",
        "template_id": "${TEMPLATE_ID}"
      }
    }
  ]
}
JSON
```

HTTP 201; relevant response fields:

<!-- api-example:create-workflow:response -->
```json
{
  "success": true,
  "data": {
    "id": "${WORKFLOW_ID}",
    "trigger_id": "welcome-v1",
    "status": "active"
  }
}
```

## Trigger workflow

Trigger by **trigger ID**, at `/workflows/trigger`. `payload` supplies template data. Save `data.id` as `EXECUTION_ID`; acceptance does not imply workflow completion.

<!-- api-example:trigger-workflow:request -->
```json
{
  "trigger_id": "welcome-v1",
  "user_id": "${USER_ID}",
  "payload": {
    "name": "Alice"
  }
}
```

<!-- api-example:trigger-workflow:curl -->
```bash
curl -sS -X POST "$BASE_URL/workflows/trigger" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "trigger_id": "welcome-v1",
  "user_id": "${USER_ID}",
  "payload": {
    "name": "Alice"
  }
}
JSON
```

HTTP 202; relevant response fields:

<!-- api-example:trigger-workflow:response -->
```json
{
  "success": true,
  "data": {
    "id": "${EXECUTION_ID}",
    "status": "running"
  }
}
```

Inspect execution with `GET /workflows/executions/EXECUTION_ID`; cancel with `POST /workflows/executions/EXECUTION_ID/cancel`. Definition CRUD uses `/workflows/WORKFLOW_ID`. Optional `transaction_id` deduplicates matching active runs; do not treat it as a permanent exactly-once record. Provider and feature prerequisites still apply to every channel step.
