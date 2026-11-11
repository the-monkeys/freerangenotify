# Webhooks and rich messages

Outbound notifications deliver to your receiver. Provider callbacks (delivery receipts, WhatsApp inbound messages) are separate inbound endpoints with provider-specific verification. Neither is an application API-key substitute.

## Generic outbound receiver

Set `channel: "webhook"` and an explicit `webhook_url`. A recipient is not required. The receiver gets JSON with `id`, nested `content`, and `created_at`. Attachments remain descriptors rather than fetched binary files. If the operator configured a signing secret, `X-Webhook-Signature` is the hex HMAC-SHA256 of the exact request body bytes. Without a secret the signature is omitted; decide explicitly whether your receiver allows unsigned traffic.

Read raw bytes before parsing JSON. Use constant-time MAC verification; reserializing parsed JSON changes the signed bytes. The compiled [receiver example](https://github.com/the-monkeys/FreeRangeNotify/blob/main/examples/docs/typescript/webhook-receiver.ts) uses WebCrypto. This generic scheme is distinct from custom-provider signature versions and inbound callback verification. Authenticate business events, deduplicate by notification ID and return a 2xx response after durable acceptance. Delivery can retry.

## Registered Slack, Teams and Discord

Configure a named custom webhook provider in your application's Settings, including its kind and destination URL. `webhook_target` selects that registered name. It does not change the generic receiver envelope. The example below assumes a registered Slack target named `registered-slack`.

Rich `poll` uses `question`, `choices` (objects with `label`), and `duration_hours`. Slack and Teams display poll text; they do not provide voting controls or collect votes. Discord uses its native poll payload and falls back to a text/embed only after a recognized HTTP 400 poll-field rejection. Other errors remain errors. Generic SMS and WhatsApp voting is not implemented by this field. For file IDs, Slack/Teams links use short-lived signed download URLs; receivers must reach the deployment's configured public base URL before expiry.

## Generic webhook

HTTP 202. Response below shows the relevant fields.

<!-- api-example:generic-webhook:request -->
```json
{
  "channel": "webhook",
  "title": "Order ready",
  "body": "Ready for collection",
  "webhook_url": "https://receiver.example.test/events",
  "attachments": [
    {
      "type": "file",
      "url": "https://cdn.example.test/report.pdf",
      "name": "report.pdf"
    }
  ]
}
```

<!-- api-example:generic-webhook:curl -->
```bash
curl -sS -X POST "$BASE_URL/notifications" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "channel": "webhook",
  "title": "Order ready",
  "body": "Ready for collection",
  "webhook_url": "https://receiver.example.test/events",
  "attachments": [
    {
      "type": "file",
      "url": "https://cdn.example.test/report.pdf",
      "name": "report.pdf"
    }
  ]
}
JSON
```

<!-- api-example:generic-webhook:response -->
```json
{
  "status": "queued",
  "content": {
    "attachments": [
      {
        "type": "file",
        "url": "https://cdn.example.test/report.pdf",
        "name": "report.pdf"
      }
    ]
  }
}
```

## Custom webhook poll

HTTP 202. Response below shows the relevant fields.

<!-- api-example:custom-webhook-poll:request -->
```json
{
  "channel": "webhook",
  "title": "Choose a time",
  "body": "Team planning",
  "webhook_target": "registered-slack",
  "poll": {
    "question": "When?",
    "choices": [
      {
        "label": "Morning"
      },
      {
        "label": "Afternoon"
      }
    ],
    "duration_hours": 24
  }
}
```

<!-- api-example:custom-webhook-poll:curl -->
```bash
curl -sS -X POST "$BASE_URL/notifications" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "channel": "webhook",
  "title": "Choose a time",
  "body": "Team planning",
  "webhook_target": "registered-slack",
  "poll": {
    "question": "When?",
    "choices": [
      {
        "label": "Morning"
      },
      {
        "label": "Afternoon"
      }
    ],
    "duration_hours": 24
  }
}
JSON
```

<!-- api-example:custom-webhook-poll:response -->
```json
{
  "status": "queued",
  "content": {
    "poll": {
      "question": "When?"
    }
  }
}
```
