# In-app inbox

Use `in_app` for persistent notification records and authorized inbox queries. Use [SSE](/docs/sse) when you also need a live event. These are separate sends and may have separate credit costs under the active rate card.

Create a recipient using [Users](/docs/users), then send this flat body to `POST /notifications` from your backend:

```json
{"user_id":"YOUR_USER_ID","channel":"in_app","title":"Order ready","body":"Your order is ready to collect."}
```

HTTP 202 means queued. The worker processes the record through the normal lifecycle; don't interpret a pending record as proof of completed processing.

## Read and manage the inbox

Use `GET /notifications?user_id=YOUR_USER_ID&page=1&page_size=20&channel=in_app`. Its response has `notifications`, `total`, `page` and `page_size`. Inspect each record's status/content/timestamps. Don't use `created_at` as a substitute for actual `sent_at`.

Mark read with `POST /notifications/read` and `{ "user_id":"YOUR_USER_ID", "notification_ids":["NOTIFICATION_ID"] }`. Archive with `PATCH /notifications/bulk/archive` and the same body. Snooze eligible records with `POST /notifications/NOTIFICATION_ID/snooze` and `{ "duration":"1h" }`; unsnooze can queue delivery again. See [Notifications](/docs/notifications) for full examples and lifecycle restrictions.

Enforce end-user ownership in your backend before proxying app-key requests. The application key has broader privileges than an individual inbox user. Public browsers should use backend requests plus short-lived SSE tokens. The existing React provider requires an application key and is only appropriate where that key can remain trusted; it is not a token-only embed.
