# Troubleshooting

Common issues, debugging techniques, and frequently asked questions.

---

## Common Issues

### Notification stuck in "pending"

**Cause:** The worker isn't running or can't connect to Redis.

**Fix:**
1. Check the worker is running: `docker-compose logs -f notification-worker`
2. Check queue depth: `GET /v1/admin/queues/stats`
3. If the queue is full, the worker might be rate-limited — check provider health

### Notification marked as "failed"

**Cause:** The delivery provider returned an error.

**Fix:**
1. Check the notification detail for the error message
2. Verify provider configuration (API keys, endpoints, credentials)
3. Check provider health: `GET /v1/admin/providers/health`
4. Review worker logs for detailed error traces

### Webhook not receiving payloads

**Cause:** The webhook URL is unreachable from the server, or HMAC verification is failing.

**Fix:**
1. Verify `webhook_url` is accessible from the server (not `localhost` unless in Docker network)
2. Check HMAC signature verification in your handler — use the `X-Signature-256` header
3. Use the **Webhook Playground** in Dashboard → Tools to test delivery
4. Check for firewall or CORS blocking the POST request

### Template variables not rendering

**Cause:** Variable names in the template don't match the `data` payload.

**Fix:**
1. Template uses `{{.user_name}}` → data must include `"user_name": "value"`
2. Note: variable names are **case-sensitive**
3. Use the **Render Preview** feature to test with sample data before sending

### SSE not connecting

**Cause:** Wrong user ID format, expired JWT, or browser compatibility issue.

**Fix:**
1. The `user_id` can be your platform's username (`external_id`), email, or internal UUID — FRN resolves all formats
2. Verify the JWT token is valid and not expired
3. Check browser supports `EventSource` (all modern browsers do)
4. If using HTTPS, ensure the SSL certificate is valid
5. Check CORS headers allow the SSE endpoint origin

### Email delivery failing

**Cause:** SMTP credentials are invalid or the email provider is rejecting messages.

**Fix:**
1. Verify SMTP host, port, username, and password in provider settings
2. Check if the from address is verified with your email provider
3. For SendGrid: ensure your API key has send permissions
4. Check spam folders — test emails often get filtered

---

## Debugging

### Service Logs

```bash
# Follow all service logs
docker-compose logs -f

# Follow specific service
docker-compose logs -f notification-service   # API server
docker-compose logs -f notification-worker    # Worker

# Enable debug logging
# Set FREERANGE_LOG_LEVEL=debug in docker-compose.yml or .env
```

### Health Checks

```bash
# Provider health
curl https://freerangenotify.monkeys.support/v1/admin/providers/health

# Queue statistics
curl https://freerangenotify.monkeys.support/v1/admin/queues/stats

# Dead letter queue
curl https://freerangenotify.monkeys.support/v1/admin/dlq
```

### Dashboard Tools

- **Dashboard → Overview:** Queue depths, provider health, system stats
- **Dashboard → Activity:** Real-time notification feed with status updates
- **Dashboard → Tools → Webhook Playground:** Test webhook delivery interactively
- **Dashboard → Tools → Quick Test:** Send test notifications with template preview

---

## FAQ

**Q: Can I use `external_id` instead of `user_id` when sending notifications?**

A: Yes — all FreeRangeNotify endpoints that accept a `user_id` automatically resolve it. You can pass your platform's username/ID (`external_id`), an email address, or the internal UUID. This works for sending notifications, SSE connections, subscriber hash generation, and all inbox operations.

**Q: What's the maximum notification payload size?**

A: 64KB for the entire request body including all fields.

**Q: How does rate limiting work?**

A: Rate limits are configurable per application. The default is 100 requests/second per API key. When exceeded, the API returns `429 Too Many Requests` with a `Retry-After` header.

**Q: Can I schedule notifications for the future?**

A: Yes — include a `scheduled_at` field (ISO 8601 format) in the notification request. The worker will hold it until the scheduled time.

**Q: How do I set up recurring notifications?**

A: Include a `recurrence` field with a `cron_expression`. The system will create new notification instances based on the schedule.

**Q: What happens when a provider is down?**

A: The circuit breaker opens after consecutive failures. Notifications are retried with exponential backoff. After maximum retries, they move to the Dead Letter Queue (DLQ) where you can replay them manually.

**Q: How do I migrate templates between environments?**

A: Use the Promote API: `POST /v1/apps/:id/environments/promote` with source and target environment IDs.
