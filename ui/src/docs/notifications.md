# Notifications and delivery lifecycle

Use [Authentication](/docs/authentication) and save the IDs from [Getting started](/docs/getting-started). Creation uses a **flat** request. `content` is the response shape; don't nest the send body inside it.

For reusable content, send `template_id` plus `data`. For direct content, supply both `title` and `body`. An accepted send is asynchronous: HTTP 202 and `queued` mean processing was accepted, not that the recipient received it. Inspect `GET /notifications/NOTIFICATION_ID` and `GET /notifications?page=1&page_size=20` for status and failure details.

Notifications are delivery records. You cannot edit a delivered email/SMS through generic CRUD. Read/archive actions change the inbox record, cancellation targets work that has not reached a final state, and retry requeues eligible failed work within configured retry limits.

## Send direct

Direct send: save `notification_id` as `DIRECT_ID`.

<!-- api-example:send-direct:request -->
```json
{
  "user_id": "${USER_ID}",
  "channel": "email",
  "title": "Service update",
  "body": "Your update is ready."
}
```

<!-- api-example:send-direct:curl -->
```bash
curl -sS -X POST "$BASE_URL/notifications" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "user_id": "${USER_ID}",
  "channel": "email",
  "title": "Service update",
  "body": "Your update is ready."
}
JSON
```

HTTP 202; relevant response fields:

<!-- api-example:send-direct:response -->
```json
{
  "notification_id": "${DIRECT_ID}",
  "status": "queued"
}
```

## Bulk notifications

Bulk sends one shared message to explicit `user_ids`. The legacy `sent` count means accepted records, not confirmed external deliveries. Inspect the returned `items`.

<!-- api-example:bulk-notifications:request -->
```json
{
  "user_ids": [
    "${USER_ID}"
  ],
  "channel": "email",
  "template_id": "${TEMPLATE_ID}",
  "priority": "normal",
  "data": {
    "name": "Alice"
  }
}
```

<!-- api-example:bulk-notifications:curl -->
```bash
curl -sS -X POST "$BASE_URL/notifications/bulk" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "user_ids": [
    "${USER_ID}"
  ],
  "channel": "email",
  "template_id": "${TEMPLATE_ID}",
  "priority": "normal",
  "data": {
    "name": "Alice"
  }
}
JSON
```

HTTP 202; relevant response fields:

<!-- api-example:bulk-notifications:response -->
```json
{
  "sent": 1,
  "total": 1
}
```

## Batch notifications

Batch sends distinct flat requests inside `notifications`. Inspect each item, since a batch can contain failures.

<!-- api-example:batch-notifications:request -->
```json
{
  "notifications": [
    {
      "user_id": "${USER_ID}",
      "channel": "email",
      "title": "Update",
      "body": "Ready."
    }
  ]
}
```

<!-- api-example:batch-notifications:curl -->
```bash
curl -sS -X POST "$BASE_URL/notifications/batch" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "notifications": [
    {
      "user_id": "${USER_ID}",
      "channel": "email",
      "title": "Update",
      "body": "Ready."
    }
  ]
}
JSON
```

HTTP 202; relevant response fields:

<!-- api-example:batch-notifications:response -->
```json
{
  "sent": 1,
  "total": 1
}
```

## Broadcast notifications

Broadcast targets the app/environment recipient set. One broadcast is one HTTP request; its recipients still have separate delivery, provider and billing work. Inspect partial/rejected/skipped/unknown/not-attempted outcomes when returned. Do not blindly retry an uncertain operation. An operator-configured idempotency store supports broadcast operation replay/conflict detection with the same `Idempotency-Key`; this is not an exactly-once provider guarantee.

<!-- api-example:broadcast-notifications:request -->
```json
{
  "channel": "email",
  "template_id": "${TEMPLATE_ID}",
  "priority": "normal",
  "data": {
    "name": "Alice"
  }
}
```

<!-- api-example:broadcast-notifications:curl -->
```bash
curl -sS -X POST "$BASE_URL/notifications/broadcast" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "channel": "email",
  "template_id": "${TEMPLATE_ID}",
  "priority": "normal",
  "data": {
    "name": "Alice"
  }
}
JSON
```

HTTP 202; relevant response fields:

<!-- api-example:broadcast-notifications:response -->
```json
{
  "sent": 1,
  "total": 1
}
```

## Schedule notification

Schedule with an RFC3339 timestamp in the future. Replace this example date with your desired time and save `notification_id` as `SCHEDULED_ID`.

<!-- api-example:schedule-notification:request -->
```json
{
  "user_id": "${USER_ID}",
  "channel": "email",
  "title": "Reminder",
  "body": "Ready.",
  "scheduled_at": "2099-01-01T00:00:00Z"
}
```

<!-- api-example:schedule-notification:curl -->
```bash
curl -sS -X POST "$BASE_URL/notifications" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "user_id": "${USER_ID}",
  "channel": "email",
  "title": "Reminder",
  "body": "Ready.",
  "scheduled_at": "2099-01-01T00:00:00Z"
}
JSON
```

HTTP 202; relevant response fields:

<!-- api-example:schedule-notification:response -->
```json
{
  "notification_id": "${SCHEDULED_ID}",
  "status": "pending"
}
```

## Snooze notification

Snooze defers eligible work. Go duration syntax supports `30m` and `1h`; use `24h` for a day, or an absolute `until`.

<!-- api-example:snooze-notification:request -->
```json
{
  "duration": "1h"
}
```

<!-- api-example:snooze-notification:curl -->
```bash
curl -sS -X POST "$BASE_URL/notifications/${SCHEDULED_ID}/snooze" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "duration": "1h"
}
JSON
```

HTTP 200; relevant response fields:

<!-- api-example:snooze-notification:response -->
```json
{
  "message": "notification snoozed"
}
```

## Unsnooze notification

Unsnooze immediately queues the record again. This can result in provider delivery.

<!-- api-example:unsnooze-notification:request -->
```json
{}
```

<!-- api-example:unsnooze-notification:curl -->
```bash
curl -sS -X POST "$BASE_URL/notifications/${SCHEDULED_ID}/unsnooze" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{}
JSON
```

HTTP 200; relevant response fields:

<!-- api-example:unsnooze-notification:response -->
```json
{
  "message": "notification unsnoozed"
}
```

## Status notification

Status update is an administrative/provider integration operation, not a content editor. Only report a failure established by your delivery system. This fixture marks a known test record failed to demonstrate the retry route; do not mark real queued work failed to manufacture a retry.

<!-- api-example:status-notification:request -->
```json
{
  "status": "failed",
  "error_message": "Provider rejected the message."
}
```

<!-- api-example:status-notification:curl -->
```bash
curl -sS -X PUT "$BASE_URL/notifications/${SCHEDULED_ID}/status" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "status": "failed",
  "error_message": "Provider rejected the message."
}
JSON
```

HTTP 200; relevant response fields:

<!-- api-example:status-notification:response -->
```json
{
  "notification_id": "${SCHEDULED_ID}",
  "status": "failed"
}
```

## Retry notification

Retry only an eligible failed notification; final state, exhausted retries or expired OTP deadlines can reject it. A retry can send another provider message.

<!-- api-example:retry-notification:request -->
```json
{}
```

<!-- api-example:retry-notification:curl -->
```bash
curl -sS -X POST "$BASE_URL/notifications/${SCHEDULED_ID}/retry" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{}
JSON
```

HTTP 200; relevant response fields:

<!-- api-example:retry-notification:response -->
```json
{
  "message": "notification queued for retry"
}
```

## Cancel notification

Cancel before delivery/final state. A cancellation racing an external provider cannot retract a message already accepted there.

<!-- api-example:cancel-notification:curl -->
```bash
curl -sS -X DELETE "$BASE_URL/notifications/${SCHEDULED_ID}" \
  -H "Authorization: Bearer $API_KEY"
```

HTTP 200; relevant response fields:

<!-- api-example:cancel-notification:response -->
```json
{
  "message": "notification cancelled successfully"
}
```

## Mark read

Read state belongs to the specified recipient; enforce end-user authorization in your backend before proxying an app-key request.

<!-- api-example:mark-read:request -->
```json
{
  "user_id": "${USER_ID}",
  "notification_ids": [
    "${NOTIFICATION_ID}"
  ]
}
```

<!-- api-example:mark-read:curl -->
```bash
curl -sS -X POST "$BASE_URL/notifications/read" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "user_id": "${USER_ID}",
  "notification_ids": [
    "${NOTIFICATION_ID}"
  ]
}
JSON
```

HTTP 200; relevant response fields:

<!-- api-example:mark-read:response -->
```json
{
  "message": "notifications marked as read"
}
```

## Archive notifications

Archive changes inbox state. It does not delete an external message.

<!-- api-example:archive-notifications:request -->
```json
{
  "user_id": "${USER_ID}",
  "notification_ids": [
    "${NOTIFICATION_ID}"
  ]
}
```

<!-- api-example:archive-notifications:curl -->
```bash
curl -sS -X PATCH "$BASE_URL/notifications/bulk/archive" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "user_id": "${USER_ID}",
  "notification_ids": [
    "${NOTIFICATION_ID}"
  ]
}
JSON
```

HTTP 200; relevant response fields:

<!-- api-example:archive-notifications:response -->
```json
{
  "message": "notifications archived"
}
```

## Limits and throughput

The application setting is **requests per hour**, enforced only when the deployment enables its application quota rollout. HTTP 429 includes retry information. API quotas differ from subscriber daily limits, wallet availability and provider throughput. Large fan-out queues work; worker concurrency, sender limits and external provider latency determine completion time. There is no zero-time or unlimited-delivery guarantee.
