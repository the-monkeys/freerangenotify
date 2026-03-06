# FreeRangeNotify Authentication Guide

Comprehensive reference for all authentication and authorization mechanisms in FreeRangeNotify. This document unifies JWT authentication, API key authorization, and SSO/OIDC integration into a single source of truth.

---

## Table of Contents

1. [Authentication Overview](#authentication-overview)
2. [Two Auth Models](#two-auth-models)
3. [JWT Authentication (Admin Dashboard)](#jwt-authentication-admin-dashboard)
   - [Token Structure](#token-structure)
   - [Registration Flow](#registration-flow)
   - [Login Flow](#login-flow)
   - [Token Refresh Flow](#token-refresh-flow)
   - [Password Reset Flow](#password-reset-flow)
   - [Logout & Change Password](#logout--change-password)
4. [API Key Authentication (Service-to-Service)](#api-key-authentication-service-to-service)
   - [How API Keys Work](#how-api-keys-work)
   - [Using API Keys](#using-api-keys)
5. [SSO / OIDC Integration (Monkeys Identity)](#sso--oidc-integration-monkeys-identity)
   - [Architecture](#sso-architecture)
   - [OIDC Setup](#oidc-setup)
   - [SSO Login Flow](#sso-login-flow)
   - [JIT Provisioning](#jit-provisioning)
6. [Route Authorization Matrix](#route-authorization-matrix)
7. [Middleware Reference](#middleware-reference)
8. [Configuration Reference](#configuration-reference)
9. [Code Architecture](#code-architecture)
10. [Frontend Integration](#frontend-integration)
11. [Security Considerations](#security-considerations)
12. [Quick Start](#quick-start)
13. [Troubleshooting](#troubleshooting)

---

## Authentication Overview

FreeRangeNotify uses **two independent authentication models** that serve different purposes:

| Model | Purpose | Credential | Token Format | Protected Resources |
|---|---|---|---|---|
| **JWT (HS256)** | Admin dashboard users | Email + Password (or SSO) | `Bearer <jwt_access_token>` | App management, admin endpoints, user profile |
| **API Key** | Client applications sending notifications | API key (UUID generated per app) | `Bearer <api_key>` | Notification delivery, user/template CRUD, presence |

Both models use the `Authorization: Bearer <token>` header, but they are validated by different middleware and protect different route groups.

---

## Two Auth Models

```
┌──────────────────────────────────────────────────────────────────┐
│                    FreeRangeNotify API (:8080)                    │
├──────────────────┬──────────────────┬────────────────────────────┤
│  Public Routes   │  API Key Routes  │      JWT Admin Routes      │
│  (No Auth)       │  (Service Auth)  │    (Dashboard Auth)        │
├──────────────────┼──────────────────┼────────────────────────────┤
│ POST /auth/*     │ POST /notify     │ GET  /admin/me             │
│ GET  /health     │ POST /users      │ POST /auth/logout          │
│ GET  /sse        │ GET  /users/:id  │ POST /auth/change-password │
│                  │ POST /templates  │ POST /apps                 │
│                  │ POST /presence/* │ GET  /apps                 │
│                  │ POST /quick-send │ PUT  /apps/:id             │
│                  │ ...              │ ...                        │
└──────────────────┴──────────────────┴────────────────────────────┘
```

---

## JWT Authentication (Admin Dashboard)

JWT authentication secures the admin dashboard. Human operators (admins) register with email/password, receive JWT tokens, and use them to manage applications, view analytics, and configure the system.

### Token Structure

FreeRangeNotify uses the `github.com/golang-jwt/jwt/v5` library with **HS256** (HMAC-SHA256) signing.

**JWT Claims:**

```go
type Claims struct {
    UserID string `json:"user_id"`
    Email  string `json:"email"`
    jwt.RegisteredClaims  // exp, iat, nbf
}
```

**Decoded Access Token Example:**

```json
{
  "user_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "email": "admin@example.com",
  "exp": 1706040300,
  "iat": 1706039400,
  "nbf": 1706039400
}
```

**Token Lifetimes (defaults):**

| Token | Duration | Configurable Via |
|---|---|---|
| Access Token | 15 minutes | `security.jwt_access_expiration` |
| Refresh Token | 7 days (10080 min) | `security.jwt_refresh_expiration` |
| Password Reset Token | 1 hour | Hardcoded |

### Registration Flow

```
Client                          API Server                    Elasticsearch
  │  POST /v1/auth/register       │                              │
  │  {email, password, full_name}  │                              │
  │ ──────────────────────────────►│                              │
  │                                │  Validate input              │
  │                                │  Check email uniqueness ────►│
  │                                │◄─────────────────────────────│
  │                                │  Hash password (bcrypt/10)   │
  │                                │  Create AdminUser ──────────►│
  │                                │  Generate access token       │
  │                                │  Generate refresh token      │
  │                                │  Store refresh token ───────►│
  │  {user, access_token,          │                              │
  │   refresh_token, expires_at}   │                              │
  │◄──────────────────────────────│                              │
```

**Request:**

```bash
curl -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "SecurePass123",
    "full_name": "Admin User"
  }'
```

**Validation Rules:**
- `email`: Required, valid email format
- `password`: Required, minimum 8 characters
- `full_name`: Required, minimum 2 characters

**Response (201 Created):**

```json
{
  "user": {
    "user_id": "uuid",
    "email": "admin@example.com",
    "full_name": "Admin User",
    "is_active": true,
    "created_at": "2026-01-23T10:00:00Z",
    "updated_at": "2026-01-23T10:00:00Z"
  },
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_at": "2026-01-23T10:15:00Z"
}
```

### Login Flow

```
Client                          API Server                    Elasticsearch
  │  POST /v1/auth/login          │                              │
  │  {email, password}            │                              │
  │ ─────────────────────────────►│                              │
  │                               │  Lookup user by email ──────►│
  │                               │◄─────────────────────────────│
  │                               │  bcrypt.Compare(hash, pass)  │
  │                               │  Check user is_active        │
  │                               │  Update last_login_at ──────►│
  │                               │  Generate new token pair     │
  │                               │  Store refresh token ───────►│
  │  {user, access_token,         │                              │
  │   refresh_token, expires_at}  │                              │
  │◄─────────────────────────────│                              │
```

**Request:**

```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "SecurePass123"
  }'
```

**Response (200 OK):** Same shape as registration response.

### Token Refresh Flow

When the access token expires (after 15 minutes by default), the client exchanges the refresh token for a new token pair.

```
Client                          API Server                    Elasticsearch
  │  POST /v1/auth/refresh        │                              │
  │  {refresh_token}              │                              │
  │ ─────────────────────────────►│                              │
  │                               │  Validate JWT signature      │
  │                               │  Lookup refresh token  ─────►│
  │                               │◄─────────────────────────────│
  │                               │  Check not revoked           │
  │                               │  Check not expired           │
  │                               │  Revoke old refresh token ──►│
  │                               │  Generate new token pair     │
  │                               │  Store new refresh token ───►│
  │  {access_token,               │                              │
  │   refresh_token, expires_at}  │                              │
  │◄─────────────────────────────│                              │
```

**Request:**

```bash
curl -X POST http://localhost:8080/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
  }'
```

**Response (200 OK):**

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_at": "2026-01-23T10:30:00Z"
}
```

### Password Reset Flow

```
Client                          API Server                    Elasticsearch
  │  POST /v1/auth/forgot-password │                             │
  │  {email}                       │                             │
  │ ──────────────────────────────►│                             │
  │                                │  Lookup user by email ─────►│
  │                                │  Generate random token      │
  │                                │   (32 bytes, hex-encoded)   │
  │                                │  Store with 1h expiry ─────►│
  │                                │  (Email sending TBD)        │
  │  {message: "instructions..."}  │                             │
  │◄──────────────────────────────│                             │
  │                                │                             │
  │  POST /v1/auth/reset-password  │                             │
  │  {token, new_password}         │                             │
  │ ──────────────────────────────►│                             │
  │                                │  Validate: exists, unused,  │
  │                                │    not expired              │
  │                                │  Hash new password          │
  │                                │  Update user password ─────►│
  │                                │  Mark token as used ───────►│
  │                                │  Revoke ALL refresh tokens ►│
  │  {message: "reset success"}    │                             │
  │◄──────────────────────────────│                             │
```

**Step 1 — Request Reset:**

```bash
curl -X POST http://localhost:8080/v1/auth/forgot-password \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@example.com"}'
```

During development, the reset token is logged at INFO level. Check logs:

```bash
docker-compose logs -f notification-service | grep "Password reset token"
```

**Step 2 — Reset Password:**

```bash
curl -X POST http://localhost:8080/v1/auth/reset-password \
  -H "Content-Type: application/json" \
  -d '{
    "token": "<hex_token_from_logs_or_email>",
    "new_password": "NewSecurePass456"
  }'
```

### Logout & Change Password

Both require a valid JWT access token in the `Authorization` header.

**Logout** — Revokes all refresh tokens for the user:

```bash
curl -X POST http://localhost:8080/v1/auth/logout \
  -H "Authorization: Bearer <access_token>"
```

**Change Password** — Requires old password verification, then revokes all refresh tokens:

```bash
curl -X POST http://localhost:8080/v1/auth/change-password \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "old_password": "current_password",
    "new_password": "new_secure_password"
  }'
```

**Get Current User:**

```bash
curl -X GET http://localhost:8080/v1/admin/me \
  -H "Authorization: Bearer <access_token>"
```

---

## API Key Authentication (Service-to-Service)

API key authentication is used by **client applications** to interact with FreeRangeNotify's core functionality: sending notifications, managing users, templates, and presence.

### How API Keys Work

1. An admin creates an **Application** via the JWT-protected `/v1/apps` endpoint.
2. The system generates a unique API key for that application.
3. The client application uses this API key to authenticate all notification-related API calls.
4. The API key can be regenerated via `POST /v1/apps/:id/regenerate-key`.

API keys are validated by looking up the application record in Elasticsearch. If the key matches an active application, the request is authorized and the `app_id` and `app_name` are injected into the request context.

### Using API Keys

Pass the API key as a Bearer token in the `Authorization` header:

```bash
# Send a notification
curl -X POST http://localhost:8080/v1/notifications \
  -H "Authorization: Bearer <api_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-uuid",
    "channel": "webhook",
    "title": "Hello",
    "body": "World"
  }'
```

```bash
# Create a user
curl -X POST http://localhost:8080/v1/users \
  -H "Authorization: Bearer <api_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "external_id": "ext-123",
    "email": "user@client-app.com"
  }'
```

```bash
# Quick send (simplified endpoint)
curl -X POST http://localhost:8080/v1/quick-send \
  -H "Authorization: Bearer <api_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "user@example.com",
    "message": "Your order has shipped!"
  }'
```

---

## SSO / OIDC Integration (Monkeys Identity)

FreeRangeNotify supports **Login with Monkeys Identity** via the OpenID Connect 1.0 Authorization Code Flow. This is an alternative to email/password registration for admin dashboard access.

### SSO Architecture

```
Browser ──► FreeRangeNotify UI (:3000)
              │
              │  Click "Log in with Monkeys Identity"
              ▼
         FreeRangeNotify API (:8080)
         GET /v1/auth/sso/login
              │
              │  Sets sso_state cookie (HttpOnly, 5min TTL, SameSite=Lax)
              │  Redirects to Monkeys Identity
              ▼
         Monkeys Identity (:8085 / identity.monkeys.support)
         /api/v1/oidc/authorize
              │
              │  User authenticates & consents
              │  Redirects with ?code=...&state=...
              ▼
         FreeRangeNotify API (:8080)
         GET /v1/auth/sso/callback
              │
              │  Exchanges code for tokens (server-to-server)
              │  Verifies ID token (RS256 via JWKS)
              │  JIT provisions user from email/name claims
              │  Issues FreeRangeNotify JWT pair (HS256)
              │  Redirects to frontend with tokens
              ▼
         FreeRangeNotify UI (:3000)
         /auth/callback?access_token=...&refresh_token=...
              │
              │  Stores tokens in localStorage
              │  Fetches /v1/admin/me
              │  Navigates to /apps
```

**Protocol:** OpenID Connect 1.0 (Authorization Code Flow)
**Identity Provider Signs:** RS256 (asymmetric, verified via JWKS)
**FreeRangeNotify Signs:** HS256 (symmetric, after SSO login succeeds)

### OIDC Setup

**Step 1 — Register OIDC Client in Monkeys Identity:**

```powershell
.\scripts\register-oidc-client.ps1 `
  -IdentityURL "http://localhost:8085" `
  -Email "your-admin@email.com" `
  -Password "YourPassword" `
  -CallbackURL "http://localhost:8080/v1/auth/sso/callback"
```

The script registers `FreeRangeNotify` as an OIDC client with `openid profile email` scopes and outputs the `client_id` and `client_secret`.

**Step 2 — Configure Environment:**

```dotenv
FREERANGE_OIDC_ENABLED=true
FREERANGE_OIDC_ISSUER=http://localhost:8085
FREERANGE_OIDC_CLIENT_ID=<your-client-id>
FREERANGE_OIDC_CLIENT_SECRET=<your-client-secret>
FREERANGE_OIDC_REDIRECT_URL=http://localhost:8080/v1/auth/sso/callback
FREERANGE_OIDC_FRONTEND_URL=http://localhost:3000
```

**Step 3 — Verify Discovery:**

```bash
curl http://localhost:8085/.well-known/openid-configuration
```

**Step 4 — Restart FreeRangeNotify:**

```bash
docker-compose up -d --build notification-service
docker-compose logs notification-service | grep "OIDC"
# Expected: "OIDC provider initialized successfully"
```

SSO routes (`/v1/auth/sso/login` and `/v1/auth/sso/callback`) are **only registered** when OIDC initialization succeeds. If it fails, these routes return 404.

### SSO Login Flow

1. **`GET /v1/auth/sso/login`** — Generates a cryptographically random state parameter, stores it in an `sso_state` cookie, and redirects to the identity provider's authorization endpoint.
2. **User authenticates** at Monkeys Identity and grants consent.
3. **`GET /v1/auth/sso/callback?code=...&state=...`** — The callback handler:
   - Verifies the `state` parameter against the cookie (CSRF protection)
   - Exchanges the authorization `code` for tokens (server-to-server)
   - Verifies the ID token signature via JWKS
   - Extracts `email`, `name`, and `preferred_username` claims
   - Calls `authService.SSOLogin(email, name)` for JIT provisioning
   - Redirects to `{frontendURL}/auth/callback?access_token=...&refresh_token=...`

### JIT Provisioning

When an SSO user logs in for the first time, `SSOLogin` automatically:

1. Searches for an existing `AdminUser` with the same email
2. **If found:** Updates `last_login_at` and generates new JWT pair
3. **If not found:** Creates a new `AdminUser` with:
   - The email and name from the ID token claims
   - A randomly generated password hash (SSO users don't use password login)
   - `is_active = true`
4. Returns the FreeRangeNotify JWT token pair (HS256)

SSO-provisioned users can later set a password through the change-password flow if they want to also log in with email/password.

---

## Route Authorization Matrix

### Public Routes (No Authentication)

| Method | Path | Handler | Purpose |
|---|---|---|---|
| POST | `/v1/auth/register` | `AuthHandler.Register` | Create admin account |
| POST | `/v1/auth/login` | `AuthHandler.Login` | Email/password login |
| POST | `/v1/auth/refresh` | `AuthHandler.RefreshToken` | Exchange refresh token |
| POST | `/v1/auth/forgot-password` | `AuthHandler.ForgotPassword` | Request password reset |
| POST | `/v1/auth/reset-password` | `AuthHandler.ResetPassword` | Reset password with token |
| GET | `/v1/auth/sso/login` | `AuthHandler.HandleSSOLogin` | Initiate SSO flow (if OIDC enabled) |
| GET | `/v1/auth/sso/callback` | `AuthHandler.HandleSSOCallback` | SSO callback (if OIDC enabled) |
| GET | `/v1/health` | `HealthHandler.Check` | Service health check |
| GET | `/v1/sse` | `SSEHandler.Connect` | Server-Sent Events stream |

### API Key Protected Routes (Service-to-Service)

All routes below require `Authorization: Bearer <api_key>` validated by `middleware.APIKeyAuth`.

| Method | Path | Handler | Purpose |
|---|---|---|---|
| **Users** | | | |
| POST | `/v1/users` | `UserHandler.Create` | Create notification user |
| POST | `/v1/users/bulk` | `UserHandler.BulkCreate` | Bulk create users |
| GET | `/v1/users/:id` | `UserHandler.GetByID` | Get user by ID |
| PUT | `/v1/users/:id` | `UserHandler.Update` | Update user |
| DELETE | `/v1/users/:id` | `UserHandler.Delete` | Delete user |
| GET | `/v1/users` | `UserHandler.List` | List users |
| POST | `/v1/users/:id/devices` | `UserHandler.AddDevice` | Add device to user |
| GET | `/v1/users/:id/devices` | `UserHandler.GetDevices` | List user devices |
| DELETE | `/v1/users/:id/devices/:device_id` | `UserHandler.RemoveDevice` | Remove device |
| PUT | `/v1/users/:id/preferences` | `UserHandler.UpdatePreferences` | Update user notification preferences |
| GET | `/v1/users/:id/preferences` | `UserHandler.GetPreferences` | Get user notification preferences |
| **Presence** | | | |
| POST | `/v1/presence/check-in` | `PresenceHandler.CheckIn` | Register user presence for smart delivery |
| **Notifications** | | | |
| POST | `/v1/notifications` | `NotificationHandler.Send` | Send notification |
| POST | `/v1/notifications/bulk` | `NotificationHandler.SendBulk` | Bulk send |
| POST | `/v1/notifications/broadcast` | `NotificationHandler.Broadcast` | Broadcast to all users |
| POST | `/v1/notifications/batch` | `NotificationHandler.SendBatch` | Send batch |
| GET | `/v1/notifications` | `NotificationHandler.List` | List notifications |
| GET | `/v1/notifications/unread/count` | `NotificationHandler.GetUnreadCount` | Unread count |
| GET | `/v1/notifications/unread` | `NotificationHandler.ListUnread` | List unread |
| POST | `/v1/notifications/read` | `NotificationHandler.MarkRead` | Mark as read |
| GET | `/v1/notifications/:id` | `NotificationHandler.Get` | Get single notification |
| PUT | `/v1/notifications/:id/status` | `NotificationHandler.UpdateStatus` | Update status |
| DELETE | `/v1/notifications/batch` | `NotificationHandler.CancelBatch` | Cancel batch |
| DELETE | `/v1/notifications/:id` | `NotificationHandler.Cancel` | Cancel notification |
| POST | `/v1/notifications/:id/retry` | `NotificationHandler.Retry` | Retry failed notification |
| **Templates** | | | |
| POST | `/v1/templates` | `TemplateHandler.CreateTemplate` | Create template |
| GET | `/v1/templates` | `TemplateHandler.ListTemplates` | List templates |
| GET | `/v1/templates/:id` | `TemplateHandler.GetTemplate` | Get template |
| PUT | `/v1/templates/:id` | `TemplateHandler.UpdateTemplate` | Update template |
| DELETE | `/v1/templates/:id` | `TemplateHandler.DeleteTemplate` | Delete template |
| POST | `/v1/templates/:id/render` | `TemplateHandler.RenderTemplate` | Render template |
| POST | `/v1/templates/:app_id/:name/versions` | `TemplateHandler.CreateTemplateVersion` | Create version |
| GET | `/v1/templates/:app_id/:name/versions` | `TemplateHandler.GetTemplateVersions` | List versions |
| **Quick Send** | | | |
| POST | `/v1/quick-send` | `QuickSendHandler.Send` | Simplified notification send |

### JWT Protected Routes (Admin Dashboard)

All routes below require `Authorization: Bearer <jwt_access_token>` validated by `middleware.JWTAuth`.

| Method | Path | Handler | Purpose |
|---|---|---|---|
| **Admin Profile** | | | |
| GET | `/v1/admin/me` | `AuthHandler.GetCurrentUser` | Get current admin user |
| POST | `/v1/auth/logout` | `AuthHandler.Logout` | Logout (revoke tokens) |
| POST | `/v1/auth/change-password` | `AuthHandler.ChangePassword` | Change password |
| **Application Management** | | | |
| POST | `/v1/apps` | `ApplicationHandler.Create` | Create application (generates API key) |
| GET | `/v1/apps` | `ApplicationHandler.List` | List applications |
| GET | `/v1/apps/:id` | `ApplicationHandler.GetByID` | Get application |
| PUT | `/v1/apps/:id` | `ApplicationHandler.Update` | Update application |
| DELETE | `/v1/apps/:id` | `ApplicationHandler.Delete` | Delete application |
| POST | `/v1/apps/:id/regenerate-key` | `ApplicationHandler.RegenerateAPIKey` | Regenerate API key |
| PUT | `/v1/apps/:id/settings` | `ApplicationHandler.UpdateSettings` | Update app settings |
| GET | `/v1/apps/:id/settings` | `ApplicationHandler.GetSettings` | Get app settings |

### Admin Routes (No Auth — Internal Only)

These routes are intended for internal monitoring and are not protected by any middleware:

| Method | Path | Handler | Purpose |
|---|---|---|---|
| GET | `/v1/admin/queues/stats` | `AdminHandler.GetQueueStats` | Queue depth and stats |
| GET | `/v1/admin/queues/dlq` | `AdminHandler.ListDLQ` | Dead letter queue |
| POST | `/v1/admin/queues/dlq/replay` | `AdminHandler.ReplayDLQ` | Replay dead letters |

> **Note:** Queue admin routes are not behind authentication middleware. In production, these should be restricted by network policy or reverse proxy rules.

---

## Middleware Reference

### `middleware.APIKeyAuth`

**Location:** `internal/interfaces/http/middleware/auth.go`

```go
func APIKeyAuth(appService usecases.ApplicationService, logger *zap.Logger) fiber.Handler
```

**Behavior:**
1. Reads the `Authorization` header
2. Strips the `Bearer ` prefix if present
3. Calls `appService.ValidateAPIKey(ctx, apiKey)` to look up the application in Elasticsearch
4. On success: stores `app_id`, `app_name`, and `app` in Fiber's `c.Locals()`
5. On failure: returns `401 Unauthorized` with error code `ErrCodeInvalidAPIKey`

**Context values set:**
- `c.Locals("app_id")` — Application UUID
- `c.Locals("app_name")` — Application name
- `c.Locals("app")` — Full application object

### `middleware.JWTAuth`

**Location:** `internal/interfaces/http/middleware/jwt_auth.go`

```go
func JWTAuth(authService auth.Service, logger *zap.Logger) fiber.Handler
```

**Behavior:**
1. Reads the `Authorization` header
2. Validates it starts with `Bearer ` (rejects otherwise)
3. Calls `authService.ValidateToken(ctx, token)` which:
   - Parses the JWT and verifies the HS256 signature
   - Extracts `user_id` from claims
   - Looks up the `AdminUser` by ID in Elasticsearch
   - Returns the user if found and active
4. On success: stores `user_id`, `user_email`, and `user` in Fiber's `c.Locals()`
5. On failure: returns `401 Unauthorized`

**Context values set:**
- `c.Locals("user_id")` — Admin user UUID
- `c.Locals("user_email")` — Admin user email
- `c.Locals("user")` — Full `AdminUser` object

### `middleware.OptionalJWTAuth` / `middleware.OptionalAPIKeyAuth`

Both variants allow requests to proceed without authentication. If a valid token/key is provided, the context is populated; otherwise, the request continues unauthenticated. These are not currently used on any route but are available for future use.

---

## Configuration Reference

### Security Configuration (`config/config.yaml`)

```yaml
security:
  jwt_secret: "${JWT_SECRET:-your-secret-key-change-in-production}"
  jwt_access_expiration: 15       # minutes
  jwt_refresh_expiration: 10080   # 7 days in minutes
  api_key_header: "X-API-Key"     # legacy reference, actual auth uses Authorization header
  rate_limit: 1000
  rate_limit_window: 3600         # seconds
```

### OIDC Configuration (`config/config.yaml`)

```yaml
oidc:
  enabled: false
  issuer: "https://identity.monkeys.support"
  client_id: ""
  client_secret: ""
  redirect_url: "http://localhost:8080/v1/auth/sso/callback"
  frontend_url: "http://localhost:3000"
```

### Environment Variable Overrides

All config values can be overridden via environment variables with the `FREERANGE_` prefix. Dots become underscores:

| Environment Variable | Config Path | Description |
|---|---|---|
| `FREERANGE_SECURITY_JWT_SECRET` | `security.jwt_secret` | HS256 signing key (min 32 chars in production) |
| `FREERANGE_SECURITY_JWT_ACCESS_EXPIRATION` | `security.jwt_access_expiration` | Access token TTL in minutes |
| `FREERANGE_SECURITY_JWT_REFRESH_EXPIRATION` | `security.jwt_refresh_expiration` | Refresh token TTL in minutes |
| `FREERANGE_OIDC_ENABLED` | `oidc.enabled` | Enable SSO routes |
| `FREERANGE_OIDC_ISSUER` | `oidc.issuer` | OIDC provider base URL |
| `FREERANGE_OIDC_CLIENT_ID` | `oidc.client_id` | OIDC client ID |
| `FREERANGE_OIDC_CLIENT_SECRET` | `oidc.client_secret` | OIDC client secret |
| `FREERANGE_OIDC_REDIRECT_URL` | `oidc.redirect_url` | OAuth2 callback URL |
| `FREERANGE_OIDC_FRONTEND_URL` | `oidc.frontend_url` | Frontend URL for post-login redirect |

You can also use the short form `JWT_SECRET` (without prefix) in a `.env` file.

---

## Code Architecture

```
pkg/jwt/
└── jwt.go                          # Manager: GenerateAccessToken, GenerateRefreshToken, ValidateToken

internal/domain/auth/
└── models.go                       # AdminUser, RefreshToken, PasswordResetToken, Repository interface, Service interface

internal/infrastructure/repository/
└── auth_repository.go              # Elasticsearch persistence for auth entities

internal/usecases/services/
└── auth_service_impl.go            # Business logic: Register, Login, SSOLogin, RefreshAccessToken, etc.

internal/interfaces/http/
├── handlers/auth_handler.go        # HTTP handlers for all auth endpoints
├── middleware/auth.go              # APIKeyAuth middleware
├── middleware/jwt_auth.go          # JWTAuth middleware
└── dto/auth_dto.go                # Request/Response DTOs

internal/interfaces/http/routes/
└── routes.go                       # Route registration with middleware assignment

internal/infrastructure/database/
└── index_templates.go              # Elasticsearch index mappings for auth_users, refresh_tokens, password_reset_tokens

internal/config/
└── config.go                       # SecurityConfig + OIDCConfig structs, Viper loading

internal/container/
└── container.go                    # Dependency injection: initializes JWTManager, OIDC provider, wires everything
```

### Elasticsearch Indices

| Index | Purpose | Key Fields |
|---|---|---|
| `auth_users` | Admin user accounts | `user_id`, `email`, `password_hash`, `is_active`, `last_login_at` |
| `refresh_tokens` | JWT refresh tokens | `token_id`, `user_id`, `token`, `expires_at`, `revoked` |
| `password_reset_tokens` | Password reset tokens | `token_id`, `user_id`, `token`, `expires_at`, `used` |

Created by running `docker-compose exec notification-service /app/migrate`.

---

## Frontend Integration

### Auth Context (`ui/src/contexts/AuthContext.tsx`)

The React UI uses an `AuthContext` that provides:
- `user` state (current admin user or null)
- `login(email, password)` — calls `/v1/auth/login`
- `register(email, password, fullName)` — calls `/v1/auth/register`
- `logout()` — calls `/v1/auth/logout`, clears localStorage
- `isAuthenticated` / `loading` state flags

### Token Storage

Tokens are stored in `localStorage`:
- `access_token` — the JWT access token
- `refresh_token` — the JWT refresh token

### Axios Interceptors (`ui/src/services/api.ts`)

1. **Request interceptor:** Attaches `Authorization: Bearer <access_token>` to every outgoing request.
2. **Response interceptor:** On 401 response:
   - Attempts to refresh using the stored `refresh_token`
   - On success: updates tokens in localStorage, retries the original request
   - On failure: redirects to `/login`

### Protected Routes (`ui/src/components/ProtectedRoute.tsx`)

Wraps dashboard pages. If the user is not authenticated, redirects to `/login`. Shows a loading spinner while the auth check is in progress.

### SSO Callback (`ui/src/pages/SSOCallback.tsx`)

Handles the redirect from the SSO flow:
1. Parses `access_token` and `refresh_token` from URL query parameters
2. Stores them in `localStorage`
3. Fetches the user profile via `GET /v1/admin/me`
4. Navigates to `/apps`

---

## Security Considerations

### Password Security
- Hashed with **bcrypt** (cost factor 10)
- Minimum 8 characters enforced via validation
- Password hashes are never returned in API responses
- Password hashes are not indexed in Elasticsearch (stored but not searchable)

### Token Security
- JWTs signed with **HS256** — keep the secret key secure and rotate it periodically
- Access tokens are short-lived (15 min default) — limits damage from token leakage
- Refresh tokens are long-lived (7 days) but **revocable** — stored in Elasticsearch, checked on every refresh
- All refresh tokens are revoked on: password change, password reset, or logout
- SSO state parameter stored in HttpOnly cookie to prevent CSRF

### API Key Security
- API keys are generated per application and stored in Elasticsearch
- Keys can be regenerated via the admin dashboard (`POST /v1/apps/:id/regenerate-key`)
- Transmitted as Bearer tokens — **always use HTTPS in production**

### SSO Security
- ID token verification uses RS256 with JWKS (asymmetric — only the identity provider has the private key)
- State parameter prevents CSRF attacks on the authorization flow
- Client secret is used server-to-server for the code-to-token exchange
- SSO-provisioned users get a random password hash and cannot log in with password unless they explicitly change it

### Production Checklist
- [ ] Set a strong, random `JWT_SECRET` (minimum 32 characters)
- [ ] Enable HTTPS / TLS termination
- [ ] Store `OIDC_CLIENT_SECRET` in a secrets manager, never in version control
- [ ] Restrict `/v1/admin/queues/*` endpoints via network policy
- [ ] Configure CORS `allowed_origins` to specific domains
- [ ] Enable rate limiting on auth endpoints
- [ ] Monitor failed login attempts via structured logs

---

## Quick Start

### 1. Environment Setup

```bash
# Create .env file
cat > .env << EOF
JWT_SECRET=your-super-secret-jwt-key-change-this-in-production-min-32-chars
ELASTICSEARCH_URL=http://localhost:9200
REDIS_HOST=localhost
REDIS_PORT=6379
EOF
```

### 2. Start Services

```bash
docker-compose down -v
docker-compose build
docker-compose up -d
sleep 30
docker-compose exec notification-service /app/migrate
```

### 3. Register an Admin

```bash
curl -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "SecurePass123",
    "full_name": "Admin User"
  }'
```

Save the `access_token` from the response.

### 4. Create an Application (Get API Key)

```bash
curl -X POST http://localhost:8080/v1/apps \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My App",
    "description": "Test application"
  }'
```

Save the `api_key` from the response.

### 5. Send a Notification

```bash
curl -X POST http://localhost:8080/v1/notifications \
  -H "Authorization: Bearer <api_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "<user-uuid>",
    "channel": "webhook",
    "title": "Hello",
    "body": "Your first notification!"
  }'
```

### 6. Start the UI

```bash
cd ui && npm install && npm run dev
```

Navigate to `http://localhost:3000` and log in with the admin credentials.

---

## Troubleshooting

### JWT Issues

| Problem | Cause | Solution |
|---|---|---|
| `Missing Authorization header` | No `Authorization` header sent | Add `Authorization: Bearer <token>` header |
| `Invalid Authorization header format` | Header doesn't start with `Bearer ` | Use format `Bearer <token>` (note the space) |
| `Invalid or expired token` | Token expired or secret mismatch | Re-login; ensure `JWT_SECRET` is consistent across restarts |
| `Token expired` immediately after login | System clock skew | Synchronize server time (NTP) |
| Protected routes always return 401 | Wrong token type used | JWT routes need JWT access tokens; API routes need API keys |

### API Key Issues

| Problem | Cause | Solution |
|---|---|---|
| `Invalid API key` | Key doesn't match any application | Verify key via `GET /v1/apps`; regenerate if needed |
| `Missing API key` | Empty Authorization header value | Ensure the key is included after `Bearer ` |

### SSO Issues

| Problem | Cause | Solution |
|---|---|---|
| `404` on `/v1/auth/sso/login` | OIDC not initialized | Set `FREERANGE_OIDC_ENABLED=true` and provide valid client credentials; check logs for init errors |
| `invalid_state` | State cookie mismatch | Ensure base64url encoding (not standard base64); check SameSite cookie settings; state cookie has 5min TTL |
| `invalid_id_token` | Signature verification failed | Verify JWKS returns base64url-encoded keys; check issuer URL matches exactly (http vs https) |
| `missing_email` | ID token lacks email claim | Ensure `email` scope is registered and identity provider includes email in token claims |
| Apps not visible after SSO login | Different user account | JIT provisioning creates a new user; apps created under a different user won't appear |

### Database Issues

| Problem | Solution |
|---|---|
| `Index does not exist` | Run `docker-compose exec notification-service /app/migrate` |
| `Cannot connect to Elasticsearch` | Check `curl http://localhost:9200` and `docker-compose logs elasticsearch` |

### Frontend Issues

| Problem | Solution |
|---|---|
| Network errors | Check API is running: `curl http://localhost:8080/v1/health` |
| Constant login redirects | Open DevTools → Application → Local Storage; verify `access_token` and `refresh_token` exist |
| Token refresh loops | Clear localStorage, log in again; check API logs for token validation errors |

### Useful Debug Commands

```bash
# Follow auth-related logs
docker-compose logs -f notification-service | grep -i "auth\|jwt\|oidc\|sso\|token"

# Check Elasticsearch auth indices
curl http://localhost:9200/_cat/indices?v | grep auth

# List all admin users
curl "http://localhost:9200/auth_users/_search?pretty" \
  -H "Content-Type: application/json" \
  -d '{"query":{"match_all":{}}}'

# List refresh tokens
curl "http://localhost:9200/refresh_tokens/_search?pretty" \
  -H "Content-Type: application/json" \
  -d '{"query":{"match_all":{}}}'

# Check Redis keys for presence data
docker-compose exec redis redis-cli KEYS "*"
```
