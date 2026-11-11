# Getting started

Create an application in the dashboard, keep its key on your backend, then register a recipient and send a notification.

## 1. Create your application

Open **Applications → + New Application**. Enter a name and choose **Create Application**. Open the app, then **Integration** to copy its app ID and use the owner-authorized key reveal/copy action. Save the full key securely. The creation response includes it once; ordinary app details use masked values. A `dashboard:...` reference used internally by the dashboard is not an SDK credential.

Set these variables on your backend or in a private terminal. Replace the example hostname with the hosted API origin provided for your deployment, or your self-hosted public server URL and any reverse-proxy prefix. Include `/v1` exactly once.

```bash
export BASE_URL="https://notify.example.test/v1"
export API_KEY="YOUR_API_KEY"
export APP_ID="YOUR_APP_ID"
```

Keep keys out of browser bundles, mobile apps and source control. See [Authentication](/docs/authentication) for environments and browser access.

### API alternative: create an app

App management uses your dashboard login JWT. Set `DASHBOARD_JWT` from your authenticated session before this optional request.

<!-- api-example:create-app:request -->
```json
{
  "app_name": "My App",
  "description": "Production notifications"
}
```

<!-- api-example:create-app:curl -->
```bash
curl -sS -X POST "$BASE_URL/apps" \
  -H "Authorization: Bearer $DASHBOARD_JWT" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "app_name": "My App",
  "description": "Production notifications"
}
JSON
```

HTTP 201; relevant response fields (other fields omitted):

<!-- api-example:create-app:response -->
```json
{
  "success": true,
  "data": {
    "app_id": "${APP_ID}",
    "api_key": "${API_KEY}",
    "app_name": "My App"
  }
}
```

Save `data.app_id` as `APP_ID` and `data.api_key` as `API_KEY`. `${...}` values below refer to your saved IDs; curl's here-doc expands these shell variables.

## 2. Register a recipient

Use `external_id` for your platform's stable identifier. Save the returned `data.user_id` as `USER_ID`. Existing customers can keep their current identifiers.

<!-- api-example:create-user:request -->
```json
{
  "email": "alice@example.test",
  "external_id": "alice",
  "full_name": "Alice"
}
```

<!-- api-example:create-user:curl -->
```bash
curl -sS -X POST "$BASE_URL/users" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "email": "alice@example.test",
  "external_id": "alice",
  "full_name": "Alice"
}
JSON
```

HTTP 201; relevant response fields (other fields omitted):

<!-- api-example:create-user:response -->
```json
{
  "success": true,
  "data": {
    "user_id": "${USER_ID}",
    "external_id": "alice",
    "email": "alice@example.test"
  }
}
```

## 3. Create a template

Configure a working email provider in application **Settings** before delivery. Template creation requires the matching `app_id` even though the API key scopes the app. Save `id` from this response as `TEMPLATE_ID`.

<!-- api-example:create-template:request -->
```json
{
  "app_id": "${APP_ID}",
  "name": "welcome_email",
  "channel": "email",
  "subject": "Welcome, {{.name}}",
  "body": "Hello {{.name}}",
  "variables": [
    "name"
  ],
  "locale": "en"
}
```

<!-- api-example:create-template:curl -->
```bash
curl -sS -X POST "$BASE_URL/templates" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "app_id": "${APP_ID}",
  "name": "welcome_email",
  "channel": "email",
  "subject": "Welcome, {{.name}}",
  "body": "Hello {{.name}}",
  "variables": [
    "name"
  ],
  "locale": "en"
}
JSON
```

HTTP 201; relevant response fields (other fields omitted):

<!-- api-example:create-template:response -->
```json
{
  "id": "${TEMPLATE_ID}",
  "name": "welcome_email",
  "version": 1
}
```

## 4. Queue a notification

Pass variables in `data`. Omit direct `title` and `body` for a template send unless you intend to override both.

<!-- api-example:send-template:request -->
```json
{
  "user_id": "${USER_ID}",
  "template_id": "${TEMPLATE_ID}",
  "channel": "email",
  "data": {
    "name": "Alice"
  }
}
```

<!-- api-example:send-template:curl -->
```bash
curl -sS -X POST "$BASE_URL/notifications" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "user_id": "${USER_ID}",
  "template_id": "${TEMPLATE_ID}",
  "channel": "email",
  "data": {
    "name": "Alice"
  }
}
JSON
```

HTTP 202; relevant response fields (other fields omitted):

<!-- api-example:send-template:response -->
```json
{
  "notification_id": "${NOTIFICATION_ID}",
  "status": "queued",
  "template_id": "${TEMPLATE_ID}"
}
```

Save `notification_id` as `NOTIFICATION_ID`. HTTP 202 means accepted for processing; it does not prove provider delivery.

## 5. Inspect delivery

<!-- api-example:get-notification:curl -->
```bash
curl -sS -X GET "$BASE_URL/notifications/${NOTIFICATION_ID}" \
  -H "Authorization: Bearer $API_KEY"
```

HTTP 200; relevant response fields (other fields omitted):

<!-- api-example:get-notification:response -->
```json
{
  "notification_id": "${NOTIFICATION_ID}",
  "status": "queued"
}
```

Workers move queued notifications through processing to sent or failed. Provider callbacks can add delivered/read states where supported. Check error, failure metadata and retry count when delivery fails. External email, SMS and WhatsApp delivery is asynchronous.

Continue with [Users](/docs/users), [Templates](/docs/templates), [Notifications](/docs/notifications), [Channels](/docs/channels) and [SDKs](/docs/sdk).
