# SSE: live notification events

Use SSE to receive named `notification` events. Existing authenticated SSE integrations and payloads remain supported. A normal app/user notification stream is separate from private dashboard activity events.

## 1. Issue a token on your backend

Authenticate your end user in your own application, verify they may access the chosen recipient, and call:

```bash
curl -sS -X POST "$BASE_URL/sse/tokens" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"user_id":"YOUR_USER_ID"}'
```

The response has `sse_token`, the resolved `user_id` and `expires_in` (currently 900 seconds). Give only the short-lived token to the browser. Do not forward the application key. Keep existing user IDs; resolve your external identifier through the existing API rather than changing stored production identifiers.

## 2. Listen in the browser

```javascript
// apiBase includes /v1. sseToken comes from your authenticated backend.
const url = new URL(apiBase + '/sse');
url.searchParams.set('sse_token', sseToken);
const stream = new EventSource(url);
stream.addEventListener('notification', event => {
  const notification = JSON.parse(event.data);
  console.log(notification);
});
// Close on logout/unmount. After expiry, obtain a fresh token before reconnecting.
// stream.close();
```

`onmessage` receives unnamed events and is not the notification listener for this stream. Handle network errors with bounded reconnect/backoff; don't keep retrying an expired credential. Reverse proxies must allow streaming, avoid response buffering and allow an appropriate idle timeout.

## 3. Publish from the backend

```json
{
  "user_id":"YOUR_USER_ID",
  "channel":"sse",
  "title":"Order update",
  "body":"Your order is ready."
}
```

POST this flat body to `/notifications` with the application/environment key. A template send can instead supply `template_id` and `data`. HTTP 202 means queued. Live delivery needs an active subscriber connection; SSE is not a guaranteed durable replay transport. For a durable inbox see [In-app](/docs/in-app).

## Existing connections

The legacy authenticated API-key/subscriber-hash connection paths remain supported for existing integrations. Keep their current authentication rules and do not share raw keys in new public clients. Dashboard activity endpoints require the authenticated dashboard principal; an arbitrary `user_id` does not grant access. See [Authentication](/docs/authentication).
