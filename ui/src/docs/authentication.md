# Authentication

## Application keys for backend integrations

Create an app in **Applications**, then open **Integration**. The owner can reveal/copy its full key using the authorized action. Masked keys and `dashboard:APP_ID` references are not SDK credentials.

Use either header for app-scoped users, templates, notifications, files and OTP:

```http
Authorization: Bearer YOUR_API_KEY
```

```http
X-API-Key: YOUR_API_KEY
```

The key identifies the app and effective environment. An app ID is a reference, not a credential. Some DTOs, including template creation, still require the matching `app_id`.

## Dashboard JWT for account and app management

Use `Authorization: Bearer DASHBOARD_JWT` for login-protected app, environment and account management. An app key cannot create an application. Team roles restrict writes and key reveal. Middleware errors can have a nested `error` object; OTP verification uses separate flat error strings.

## Environment keys

Use the selected environment's full key when environments are enabled. Existing app keys retain legacy/default-environment behavior. Changing a dashboard environment does not change a deployed backend credential.

## Public browsers and mobile apps

Keep application keys on your backend. Authenticate your user there and request a short-lived [SSE token](/docs/sse), giving the browser only that token. Proxy CRUD and OTP through a backend that authorizes the end user. The existing React provider requires an app key; it is not a token-only browser API.

## Failures

401 means missing, invalid or expired credentials; 403 means insufficient access. Honor `Retry-After` on 429. Rotate keys deliberately and update backend secrets. Never put full keys in public docs, logs or support tickets.
