# API Documentation

FreeRangeNotify provides comprehensive REST API documentation using OpenAPI 3.0 (Swagger) specification.

## Accessing API Documentation

### Interactive Swagger UI

Once the server is running, access the interactive API documentation at:

```
http://localhost:8080/swagger/index.html
```

The Swagger UI provides:
- **Interactive testing** - Try out API endpoints directly from the browser
- **Request/response examples** - See sample payloads for all endpoints
- **Schema documentation** - Detailed models and data structures
- **Authentication setup** - Easy Bearer token configuration

### OpenAPI Specification Files

The raw OpenAPI specification is available in multiple formats:

- **YAML**: `/swagger/doc.yaml` or `docs/openapi/swagger.yaml`
- **JSON**: `/swagger/doc.json` or `docs/swagger.json`

## API Overview

### Base URL
```
http://localhost:8080
```

### Authentication

Protected endpoints require Bearer token authentication using your application's API key:

```http
Authorization: Bearer frn_YOUR_API_KEY_HERE
```

**Example with curl:**
```bash
curl -X POST http://localhost:8080/v1/users \
  -H "Authorization: Bearer frn_GxSftc5urm7bxVrYh-RHNEmNDvS4auxwOhwjqXwC-kM=" \
  -H "Content-Type: application/json" \
  -d '{
    "external_user_id": "user-123",
    "email": "user@example.com"
  }'
```

**Example with PowerShell:**
```powershell
$headers = @{
    "Authorization" = "Bearer frn_YOUR_API_KEY"
    "Content-Type" = "application/json"
}
$body = @{
    external_user_id = "user-123"
    email = "user@example.com"
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:8080/v1/users" `
    -Method Post -Headers $headers -Body $body
```

## API Endpoints

### System Endpoints

#### Health Check
```http
GET /health
```
Returns service health status and database connectivity.

**Response:**
```json
{
  "service": "FreeRangeNotify",
  "version": "1.0.0",
  "database": "ok"
}
```

---

### Application Management (Public)

#### Create Application
```http
POST /v1/apps
```
Create a new application and receive an API key.

**Request Body:**
```json
{
  "app_name": "My App",
  "webhook_url": "https://example.com/webhook",
  "settings": {
    "rate_limit": 1000,
    "retry_attempts": 3,
    "enable_webhooks": true,
    "enable_analytics": true
  }
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "app_id": "uuid",
    "app_name": "My App",
    "api_key": "frn_FULL_API_KEY_HERE",
    "webhook_url": "https://example.com/webhook"
  }
}
```

#### Get Application
```http
GET /v1/apps/{app_id}
```

#### List Applications
```http
GET /v1/apps?page=1&page_size=10
```

#### Update Application
```http
PUT /v1/apps/{app_id}
```

#### Delete Application
```http
DELETE /v1/apps/{app_id}
```

#### Regenerate API Key
```http
POST /v1/apps/{app_id}/regenerate-key
```

#### Get/Update Settings
```http
GET /v1/apps/{app_id}/settings
PUT /v1/apps/{app_id}/settings
```

---

### User Management (Protected)

All user endpoints require Bearer authentication.

#### Create User
```http
POST /v1/users
Authorization: Bearer {api_key}
```

**Request Body:**
```json
{
  "external_user_id": "user-123",
  "email": "user@example.com",
  "phone_number": "+1234567890",
  "preferences": {
    "channels": ["email", "push"],
    "timezone": "America/New_York",
    "language": "en",
    "quiet_hours": {
      "enabled": true,
      "start": "22:00",
      "end": "08:00"
    }
  }
}
```

#### Get User
```http
GET /v1/users/{user_id}
Authorization: Bearer {api_key}
```

#### List Users
```http
GET /v1/users?page=1&page_size=10
Authorization: Bearer {api_key}
```

#### Update User
```http
PUT /v1/users/{user_id}
Authorization: Bearer {api_key}
```

#### Delete User
```http
DELETE /v1/users/{user_id}
Authorization: Bearer {api_key}
```

---

### Device Management (Protected)

#### Add Device
```http
POST /v1/users/{user_id}/devices
Authorization: Bearer {api_key}
```

**Request Body:**
```json
{
  "platform": "ios",
  "token": "device-token-abc123"
}
```

**Supported platforms:** `ios`, `android`, `web`

#### Get User Devices
```http
GET /v1/users/{user_id}/devices
Authorization: Bearer {api_key}
```

#### Delete Device
```http
DELETE /v1/users/{user_id}/devices/{device_id}
Authorization: Bearer {api_key}
```

---

### Preference Management (Protected)

#### Get Preferences
```http
GET /v1/users/{user_id}/preferences
Authorization: Bearer {api_key}
```

#### Update Preferences
```http
PUT /v1/users/{user_id}/preferences
Authorization: Bearer {api_key}
```

**Request Body:**
```json
{
  "channels": ["email", "push", "sms"],
  "timezone": "America/Los_Angeles",
  "language": "es",
  "quiet_hours": {
    "enabled": true,
    "start": "23:00",
    "end": "07:00"
  }
}
```

---

## Data Models

### Application
```json
{
  "app_id": "uuid",
  "app_name": "string",
  "api_key": "string (masked after creation)",
  "webhook_url": "string (url)",
  "settings": {
    "rate_limit": "integer",
    "retry_attempts": "integer",
    "default_template": "string",
    "enable_webhooks": "boolean",
    "enable_analytics": "boolean"
  },
  "created_at": "datetime",
  "updated_at": "datetime"
}
```

### User
```json
{
  "user_id": "uuid",
  "app_id": "uuid",
  "external_user_id": "string",
  "email": "string (email format)",
  "phone_number": "string (E.164 format)",
  "preferences": {
    "email_enabled": "boolean",
    "push_enabled": "boolean",
    "sms_enabled": "boolean",
    "quiet_hours": {}
  },
  "created_at": "datetime",
  "updated_at": "datetime"
}
```

### Device
```json
{
  "device_id": "uuid",
  "platform": "enum (ios, android, web)",
  "token": "string",
  "active": "boolean",
  "registered_at": "datetime",
  "last_seen": "datetime"
}
```

---

## Error Responses

All error responses follow this format:

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "details": {
      "field_name": "Specific validation error"
    }
  }
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `VALIDATION_ERROR` | 400 | Request validation failed |
| `UNAUTHORIZED` | 401 | Missing authorization header |
| `INVALID_API_KEY` | 401 | Invalid or expired API key |
| `NOT_FOUND` | 404 | Resource not found |
| `INTERNAL_ERROR` | 500 | Internal server error |
| `DATABASE_ERROR` | 500 | Database operation failed |

---

## Rate Limiting

Rate limits are configurable per application (default: 1000 requests/minute).

**Headers returned:**
```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 950
X-RateLimit-Reset: 1638360000
```

---

## Pagination

List endpoints support pagination with query parameters:

- `page` - Page number (starts at 1, default: 1)
- `page_size` - Items per page (default: 10, max: 100)

**Example:**
```http
GET /v1/users?page=2&page_size=20
```

**Response:**
```json
{
  "success": true,
  "data": {
    "users": [...],
    "total_count": 150,
    "page": 2,
    "page_size": 20
  }
}
```

---

## Filtering

Some endpoints support filtering:

**Applications:**
- `app_name` - Filter by application name

**Users:**
- `external_user_id` - Filter by external user ID
- `email` - Filter by email address

**Example:**
```http
GET /v1/users?email=john@example.com
```

---

## Generating/Updating Documentation

### Generate Swagger Docs

```bash
# Using Makefile
make swagger

# Or directly
swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal
```

### Format Swagger Annotations

```bash
make swagger-fmt
```

---

## Using Swagger UI

1. **Start the server:**
   ```bash
   docker-compose up -d
   # or
   go run cmd/server/main.go
   ```

2. **Open Swagger UI:**
   Navigate to `http://localhost:8080/swagger/index.html`

3. **Authorize (for protected endpoints):**
   - Click "Authorize" button at the top
   - Enter your API key: `frn_YOUR_API_KEY`
   - Click "Authorize" then "Close"

4. **Try out endpoints:**
   - Expand any endpoint
   - Click "Try it out"
   - Fill in parameters/body
   - Click "Execute"
   - View response

---

## Postman Collection

You can import the OpenAPI spec into Postman:

1. Open Postman
2. Click "Import"
3. Select "Link" tab
4. Enter: `http://localhost:8080/swagger/doc.json`
5. Click "Continue" and "Import"

---

## Integration Examples

### JavaScript/Node.js
```javascript
const axios = require('axios');

const api = axios.create({
  baseURL: 'http://localhost:8080',
  headers: {
    'Authorization': 'Bearer frn_YOUR_API_KEY',
    'Content-Type': 'application/json'
  }
});

// Create user
const response = await api.post('/v1/users', {
  external_user_id: 'user-123',
  email: 'user@example.com'
});
```

### Python
```python
import requests

headers = {
    'Authorization': 'Bearer frn_YOUR_API_KEY',
    'Content-Type': 'application/json'
}

response = requests.post(
    'http://localhost:8080/v1/users',
    headers=headers,
    json={
        'external_user_id': 'user-123',
        'email': 'user@example.com'
    }
)
```

### Go
```go
client := &http.Client{}
body := strings.NewReader(`{"external_user_id":"user-123","email":"user@example.com"}`)

req, _ := http.NewRequest("POST", "http://localhost:8080/v1/users", body)
req.Header.Set("Authorization", "Bearer frn_YOUR_API_KEY")
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

---

## Support

For API support or questions:
- Email: support@freerangenotify.com
- Documentation: http://localhost:8080/swagger/
- GitHub: https://github.com/the-monkeys/freerangenotify

---

## Webhook Rich Content (Discord / Slack / Teams)

The `POST /v1/notifications`, `POST /v1/notifications/bulk`, `POST /v1/notifications/broadcast`, and `POST /v1/notifications/quick-send` endpoints accept the following **optional** top-level fields when the resolved channel is `webhook` (or kind-specific aliases `discord`, `slack`, `teams`). Unsupported fields for a given target degrade gracefully — a payload that requests a poll on Slack will render the choices as a numbered list rather than failing.

### Per-provider capability matrix

| Field         | Discord  | Slack       | Teams      | Generic webhook |
| ------------- | -------- | ----------- | ---------- | --------------- |
| `attachments` | yes      | yes         | yes        | passed through  |
| `actions`     | markdown | buttons     | buttons    | passed through  |
| `fields`      | yes      | yes         | FactSet    | passed through  |
| `mentions`    | yes      | yes         | yes        | n/a             |
| `poll`        | native   | numbered    | numbered   | passed through  |
| `style.color` | yes      | sidebar bar | themeColor | n/a             |

Notes:
- Discord incoming webhooks do not accept interactive components, so `actions` render as a markdown link list inside the embed.
- Slack and Microsoft Teams incoming webhooks have no native poll element, so `poll` renders as a numbered list of choices in the message body.
- `style.severity` accepts `info | success | warning | danger` and is mapped to a per-provider color preset. `style.color` (hex) overrides the preset.
- `mentions[].platform` must equal the resolved target (`discord` / `slack` / `teams`); mentions whose platform does not match are silently dropped by the renderer.

### Example: Slack alert with fields, actions, and severity color

```http
POST /v1/notifications
Authorization: Bearer frn_app_<API_KEY>
Content-Type: application/json

{
  "user_id": "u_alice",
  "channel": "webhook",
  "priority": "high",
  "webhook_target": "Slack Alerts",
  "title": "Database CPU > 90%",
  "body": "Sustained high CPU on db-prod-1 for 5m.",
  "fields": [
    { "key": "Host",     "value": "db-prod-1", "inline": true },
    { "key": "Region",   "value": "us-east-1", "inline": true },
    { "key": "Severity", "value": "high",      "inline": true }
  ],
  "actions": [
    { "type": "link", "label": "Acknowledge", "url": "https://oncall.example.com/ack/123", "style": "primary" },
    { "type": "link", "label": "Runbook",     "url": "https://wiki.example.com/runbooks/db-cpu" }
  ],
  "style": { "severity": "danger" }
}
```

### Example: Discord poll with attachment

```http
POST /v1/notifications
Authorization: Bearer frn_app_<API_KEY>
Content-Type: application/json

{
  "user_id": "u_team_lead",
  "channel": "webhook",
  "priority": "normal",
  "webhook_target": "Discord Alerts",
  "title": "Friday lunch order",
  "body": "Vote by 11:30.",
  "poll": {
    "question": "Where to?",
    "choices": [
      { "label": "Tacos" },
      { "label": "Pizza" },
      { "label": "Sushi" }
    ],
    "duration_hours": 2
  },
  "attachments": [
    { "type": "image", "url": "https://picsum.photos/seed/lunch/600/300", "name": "menu.jpg" }
  ]
}
```

### SDK helpers

The Go and JavaScript SDKs ship one-line builders that pre-populate `channel` and `webhook_target`:

```go
// Go
import "github.com/the-monkeys/freerangenotify/sdk/go/freerangenotify"

p := freerangenotify.NewSlackAlert("Slack Alerts", "Database CPU > 90%", "Sustained high CPU on db-prod-1 for 5m.", "danger").
    WithFields(
        freerangenotify.ContentField{Key: "Host", Value: "db-prod-1", Inline: true},
        freerangenotify.ContentField{Key: "Region", Value: "us-east-1", Inline: true},
    ).
    WithActions(
        freerangenotify.ContentAction{Type: "link", Label: "Acknowledge", URL: "https://oncall.example.com/ack/123", Style: "primary"},
    ).
    To("u_alice")
_, _ = client.Notifications.Send(ctx, p)
```

```ts
// TypeScript
import { FreeRangeNotify, webhook } from '@freerangenotify/sdk';

const params = {
    ...webhook.slack('Slack Alerts', 'Database CPU > 90%', 'Sustained high CPU on db-prod-1 for 5m.', 'danger'),
    user_id: 'u_alice',
    fields: [
        { key: 'Host',   value: 'db-prod-1', inline: true },
        { key: 'Region', value: 'us-east-1', inline: true },
    ],
    actions: [
        { type: 'link', label: 'Acknowledge', url: 'https://oncall.example.com/ack/123', style: 'primary' },
    ],
};
await client.notifications.send(params);
```

---

## WhatsApp Rich Templates

See [docs/openapi/whatsapp-rich.yaml](../docs/openapi/whatsapp-rich.yaml) for the full OpenAPI fragment covering authoring, sync, preview, click attribution, and Twilio webhooks.

### Authoring

`POST /v1/whatsapp/rich-templates` validates a typed template, submits it to every configured provider (Meta Cloud API and/or Twilio Content API), and persists the resulting bindings. Meta failures return 400 with the rejection reason; Twilio submission is best-effort and reconciled via the sync endpoint.

Example carousel payload:

```json
{
  "kind": "carousel",
  "name": "diwali_carousel",
  "language": "en_US",
  "body": "Hi {{1}}, check these out:",
  "cards": [
    {
      "header_image_url": "https://cdn.example.com/p1.jpg",
      "body": "Trendy Polo {{1}}",
      "buttons": [
        { "type": "URL", "text": "Shop", "url": "https://shop.example/p/{{1}}", "track_clicks": true }
      ]
    },
    {
      "header_image_url": "https://cdn.example.com/p2.jpg",
      "body": "Classic Shirt {{1}}",
      "buttons": [
        { "type": "URL", "text": "Shop", "url": "https://shop.example/p/{{1}}", "track_clicks": true }
      ]
    }
  ]
}
```

### Listing, sync, preview

- `GET /v1/whatsapp/rich-templates?kind=carousel&status=approved&limit=50` — paginated list.
- `POST /v1/whatsapp/rich-templates/{id}/sync` — re-fetch provider approval state.
- `POST /v1/whatsapp/rich-templates/{id}/preview` — render with a `variables` map, returns Meta wire-format JSON for inspection.

### Sending

Reference an approved rich template from any send path by attaching the typed payload to `data.whatsapp_rich`. When this object carries a `template_id`, `content_sid` is not required on the request:

```json
{
  "channel": "whatsapp",
  "user_id": "user-internal-uuid",
  "data": {
    "whatsapp_rich": {
      "template_id": "rich-tpl-abc123",
      "variables": { "1": "Asha" },
      "cards": [
        { "variables": { "1": "Polo" },  "button_values": ["polo-sku"] },
        { "variables": { "1": "Shirt" }, "button_values": ["shirt-sku"] }
      ]
    }
  }
}
```

The worker resolves `template_id` against the configured provider before dispatch:

- **Meta path**: `whatsapp_rich` is expanded into the provider's interactive/template JSON.
- **Twilio path**: `whatsapp_rich` is flattened into `content_sid` + `content_variables`, with carousel per-card overrides emitted as `<card_index>.<position>` keys (1-indexed).

### Click attribution

Buttons with `track_clicks: true` are wrapped at render-time with a signed `/v1/r/{sig}` redirect so taps land in the analytics index. `server.public_url` must be set for wrapping to take effect — when it is empty, URLs degrade to pass-through and a warning is logged at boot.

### Twilio webhooks

- `POST /v1/webhooks/twilio/content-status` — Twilio Content API approval updates. The handler maps `ContentSid` → FRN-side `TwilioBinding` via the rich-template service. Always returns 200 to prevent retries.
- `POST /v1/webhooks/twilio/whatsapp` — Twilio Programmable Messaging inbound webhook for replies, button taps, and list selections. Resolves the target FRN app by matching `To` against `application.Settings.WhatsApp.FromNumber`, then forwards a normalised `InboundMessage` to the WhatsApp service which emits `whatsapp.button_clicked` / `whatsapp.list_selected` / `whatsapp.text_received` workflow events. Signature is verified against `providers.whatsapp.auth_token` when configured; requests with a bad signature are rejected with `403`.
