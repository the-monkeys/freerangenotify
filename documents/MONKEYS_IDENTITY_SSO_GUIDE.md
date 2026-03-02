# Monkeys Identity SSO Integration Guide

End-to-end guide for integrating **Login with Monkeys Identity** (OIDC SSO) into FreeRangeNotify.

---

## Architecture Overview

```
Browser ──► FreeRangeNotify UI (React, :3000)
              │
              │  Click "Log in with Monkeys Identity"
              ▼
         FreeRangeNotify API (:8080)
         GET /v1/auth/sso/login
              │
              │  Sets sso_state cookie
              │  Redirects to Monkeys Identity
              ▼
         Monkeys Identity (:8085 local / identity.monkeys.support)
         /api/v1/oidc/authorize
              │
              │  User authenticates & consents
              │  Redirects with ?code=...&state=...
              ▼
         FreeRangeNotify API (:8080)
         GET /v1/auth/sso/callback
              │
              │  Exchanges code for tokens (server-to-server)
              │  Verifies ID token (RS256, JWKS)
              │  JIT provisions user from email/name claims
              │  Issues FreeRangeNotify JWT (HS256)
              │  Redirects to frontend with tokens
              ▼
         FreeRangeNotify UI (:3000)
         /auth/callback?access_token=...&refresh_token=...
              │
              │  Stores tokens in localStorage
              │  Fetches user profile via GET /v1/admin/me
              │  Navigates to /apps
              ▼
         Dashboard (authenticated)
```

**Protocol**: OpenID Connect 1.0 (Authorization Code Flow)  
**Token Signing**: Monkeys Identity signs ID tokens with RS256. FreeRangeNotify issues its own HS256 JWTs after SSO login.

---

## Prerequisites

| Requirement | Details |
|---|---|
| Monkeys Identity running | Local: `http://localhost:8085` / Prod: `https://identity.monkeys.support` |
| Admin account on Monkeys Identity | Email + password with permission to register OIDC clients |
| FreeRangeNotify API running | Default: `http://localhost:8080` |
| FreeRangeNotify UI running | Default: `http://localhost:3000` |

---

## Step 1: Register OIDC Client in Monkeys Identity

Use the provided registration script:

```powershell
# Local development
.\scripts\register-oidc-client.ps1 `
  -IdentityURL "http://localhost:8085" `
  -Email "your-admin@email.com" `
  -Password "YourPassword" `
  -CallbackURL "http://localhost:8080/v1/auth/sso/callback"
```

```powershell
# Production
.\scripts\register-oidc-client.ps1 `
  -IdentityURL "https://identity.monkeys.support" `
  -Email "your-admin@email.com" `
  -Password "YourPassword" `
  -CallbackURL "https://your-domain.com/v1/auth/sso/callback"
```

The script will:
1. Log in to Monkeys Identity to obtain an admin token
2. Register `FreeRangeNotify` as an OIDC client with `openid profile email` scopes
3. Output the `client_id` and `client_secret`
4. Automatically update your `.env` file

### Manual Registration (Alternative)

If you prefer manual registration, call the Monkeys Identity API directly:

```bash
# 1. Login
TOKEN=$(curl -s -X POST ${IDENTITY_URL}/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@email.com","password":"password"}' | jq -r '.token')

# 2. Register OIDC client
curl -X POST ${IDENTITY_URL}/api/v1/oidc/clients \
  -H "Authorization: Bearer ${TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "FreeRangeNotify",
    "redirect_uris": ["http://localhost:8080/v1/auth/sso/callback"],
    "scopes": ["openid", "profile", "email"]
  }'
```

Save the returned `client_id` and `client_secret`.

---

## Step 2: Configure Environment Variables

Add these to your `.env` file:

```dotenv
# OIDC (Monkeys Identity)
FREERANGE_OIDC_ENABLED=true
FREERANGE_OIDC_ISSUER=http://localhost:8085
FREERANGE_OIDC_CLIENT_ID=<your-client-id>
FREERANGE_OIDC_CLIENT_SECRET=<your-client-secret>
FREERANGE_OIDC_REDIRECT_URL=http://localhost:8080/v1/auth/sso/callback
FREERANGE_OIDC_FRONTEND_URL=http://localhost:3000
```

### Production Values

```dotenv
FREERANGE_OIDC_ENABLED=true
FREERANGE_OIDC_ISSUER=https://identity.monkeys.support
FREERANGE_OIDC_CLIENT_ID=<production-client-id>
FREERANGE_OIDC_CLIENT_SECRET=<production-client-secret>
FREERANGE_OIDC_REDIRECT_URL=https://your-domain.com/v1/auth/sso/callback
FREERANGE_OIDC_FRONTEND_URL=https://your-frontend-domain.com
```

### Variable Reference

| Variable | Description |
|---|---|
| `FREERANGE_OIDC_ENABLED` | Set `true` to enable SSO. Routes are only registered when enabled. |
| `FREERANGE_OIDC_ISSUER` | The base URL of Monkeys Identity. Must match the `iss` claim in its tokens exactly (including `http` vs `https`). |
| `FREERANGE_OIDC_CLIENT_ID` | UUID returned from client registration. |
| `FREERANGE_OIDC_CLIENT_SECRET` | Secret returned from client registration. Keep this secure. |
| `FREERANGE_OIDC_REDIRECT_URL` | The callback URL on FreeRangeNotify's API server. Must match what was registered in Step 1. |
| `FREERANGE_OIDC_FRONTEND_URL` | The URL of the FreeRangeNotify React UI. Used for post-login redirects. |

---

## Step 3: Docker Networking (Local Development)

When running FreeRangeNotify in Docker and Monkeys Identity on the host, the container cannot reach `localhost:8085` by default. Add `extra_hosts` to `docker-compose.yml`:

```yaml
services:
  notification-service:
    # ... existing config ...
    extra_hosts:
      - "localhost:host-gateway"
```

This maps `localhost` inside the container to the host machine's network, allowing the OIDC discovery and token exchange to reach Monkeys Identity.

---

## Step 4: Verify OIDC Discovery

Before starting FreeRangeNotify, verify that the OIDC discovery endpoint is accessible:

```powershell
# From the host (or from within Docker if using extra_hosts)
Invoke-RestMethod http://localhost:8085/.well-known/openid-configuration
```

Expected response:
```json
{
  "issuer": "http://localhost:8085",
  "authorization_endpoint": "http://localhost:8085/api/v1/oidc/authorize",
  "token_endpoint": "http://localhost:8085/api/v1/oidc/token",
  "jwks_uri": "http://localhost:8085/.well-known/jwks.json",
  "response_types_supported": ["code"],
  "subject_types_supported": ["public"],
  "id_token_signing_alg_values_supported": ["RS256"],
  "scopes_supported": ["openid", "profile", "email"]
}
```

Also verify the JWKS:
```powershell
Invoke-RestMethod http://localhost:8085/.well-known/jwks.json
```

The `n` parameter in the key should be a base64url-encoded string (~342 chars for 2048-bit RSA), **not** a hex string.

---

## Step 5: Start Services and Test

```powershell
# Rebuild FreeRangeNotify with OIDC config
docker-compose up -d --build notification-service

# Check OIDC initialized successfully
docker-compose logs notification-service | Select-String "OIDC"
```

Expected log line:
```
OIDC provider initialized successfully {"issuer": "http://localhost:8085", "client_id": "...", "redirect_url": "http://localhost:8080/v1/auth/sso/callback"}
```

If you see "OIDC is enabled but client_id is empty" or "Failed to initialize OIDC provider", check:
- `.env` values are correct and not empty
- The issuer URL is reachable from inside the Docker container
- The `/.well-known/openid-configuration` returns valid JSON

### Test the Login Flow

1. Open `http://localhost:3000/login`
2. Click **"Log in with Monkeys Identity"**
3. You should be redirected to Monkeys Identity's login page
4. Sign in with your Monkeys Identity credentials
5. On the consent screen, click **"Allow Access"**
6. You should be redirected back to FreeRangeNotify's dashboard at `/apps`

---

## How It Works (Code Walkthrough)

### Configuration Loading

[internal/config/config.go](../internal/config/config.go) defines the `OIDCConfig` struct:
```go
type OIDCConfig struct {
    Enabled      bool   `mapstructure:"enabled"`
    Issuer       string `mapstructure:"issuer"`
    ClientID     string `mapstructure:"client_id"`
    ClientSecret string `mapstructure:"client_secret"`
    RedirectURL  string `mapstructure:"redirect_url"`
    FrontendURL  string `mapstructure:"frontend_url"`
}
```

Environment variables are mapped via Viper with the `FREERANGE_` prefix (e.g., `FREERANGE_OIDC_CLIENT_ID` → `oidc.client_id`).

### OIDC Provider Initialization

[internal/container/container.go](../internal/container/container.go) initializes the OIDC provider during startup:

1. Validates that `OIDC.Enabled` is `true` and `ClientID` is non-empty
2. Performs OIDC discovery against the issuer's `/.well-known/openid-configuration`
3. Creates an `oauth2.Config` with the discovered endpoint URLs
4. Creates an ID token verifier configured for the client ID
5. Stores `OIDCProvider`, `OAuth2Config`, and `OIDCVerifier` on the container

### Route Registration

[internal/interfaces/http/routes/routes.go](../internal/interfaces/http/routes/routes.go) conditionally registers SSO routes:

```go
if c.OIDCProvider != nil && c.OAuth2Config != nil {
    auth.Get("/sso/login", c.AuthHandler.HandleSSOLogin(c.OAuth2Config))
    auth.Get("/sso/callback", c.AuthHandler.HandleSSOCallback(
        c.OAuth2Config, c.OIDCVerifier, frontendURL,
    ))
}
```

Routes are **only** registered when OIDC initialization succeeds. If it fails, the SSO button on the frontend will get a 404.

### SSO Login Handler

`GET /v1/auth/sso/login` ([auth_handler.go](../internal/interfaces/http/handlers/auth_handler.go)):

1. Generates a cryptographically random state parameter (base64url-encoded, URL-safe)
2. Stores the state in an `sso_state` cookie (HttpOnly, 5-minute TTL, SameSite=Lax)
3. Redirects the browser to the identity provider's authorization endpoint with `response_type=code`, the registered scopes, and the state parameter

### SSO Callback Handler

`GET /v1/auth/sso/callback` ([auth_handler.go](../internal/interfaces/http/handlers/auth_handler.go)):

1. **State verification**: Compares the `state` query parameter with the `sso_state` cookie to prevent CSRF attacks
2. **Code exchange**: Sends the authorization code to the identity provider's token endpoint (server-to-server) in exchange for an access token and ID token
3. **ID token verification**: Validates the ID token's signature using the JWKS, checks issuer, audience, and expiry
4. **Claims extraction**: Reads `email`, `name`, and `preferred_username` from the ID token claims
5. **JIT provisioning**: Calls `authService.SSOLogin(email, name)` which:
   - Looks up the user by email
   - If not found: creates a new `AdminUser` with a random password (SSO users don't use password login)
   - If found: updates last login time
   - Generates a FreeRangeNotify JWT token pair (HS256)
6. **Frontend redirect**: Redirects to `{frontendURL}/auth/callback?access_token=...&refresh_token=...`

### Frontend Callback

[ui/src/pages/SSOCallback.tsx](../ui/src/pages/SSOCallback.tsx):

1. Parses `access_token` and `refresh_token` from URL query parameters
2. Stores them in `localStorage`
3. Fetches the user profile via `GET /v1/admin/me` to hydrate auth state
4. Navigates to `/apps`

---

## Troubleshooting

### "Cannot GET /v1/auth/sso/login" (404)

SSO routes were not registered. Check:
- `FREERANGE_OIDC_ENABLED=true` in `.env`
- `FREERANGE_OIDC_CLIENT_ID` and `FREERANGE_OIDC_CLIENT_SECRET` are set
- The issuer URL is reachable from the Docker container
- Container logs for OIDC initialization errors

### "invalid_state" error on login page

The state cookie didn't match the state in the callback URL. Causes:
- State contained `+` or `/` characters (use base64url encoding, not standard base64)
- Cookie was not sent back (cross-origin issue, check SameSite settings)
- Session expired (state cookie has a 5-minute TTL)

### "invalid_id_token" error

The ID token failed signature verification. Check:
- Monkeys Identity JWKS endpoint returns base64url-encoded RSA key parameters (not hex)
- The `kid` header in signed JWTs matches the `kid` in JWKS
- The issuer in `FREERANGE_OIDC_ISSUER` exactly matches the `iss` claim (including scheme `http` vs `https`)

### "missing_email" error

The ID token didn't contain an `email` claim. Ensure:
- Monkeys Identity's token service includes user profile claims (`email`, `name`) in the ID token
- The registered client has `email` and `profile` scopes

### Apps not visible after login

After SSO login, the JWT includes the user's internal UUID. Applications are scoped to the user (`admin_user_id` field). If apps were created with a different user account (e.g., registered via email/password), they won't appear under the SSO account even if the email is the same — unless the system matched them during JIT provisioning. Verify the user ID is consistent.

### Docker container can't reach Monkeys Identity

For local development where Monkeys Identity runs on the host:
```yaml
extra_hosts:
  - "localhost:host-gateway"
```

### Nginx/Reverse Proxy in Production

If Monkeys Identity is behind Nginx, the Nginx config must route OIDC endpoints to the Go backend, not the Vite frontend:

```nginx
location /.well-known/ {
    proxy_pass http://127.0.0.1:8085;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
}

location /api/ {
    proxy_pass http://127.0.0.1:8085;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

---

## Security Considerations

1. **Client secret**: Store `FREERANGE_OIDC_CLIENT_SECRET` securely. Never commit it to version control. Use environment variables or a secrets manager.
2. **State parameter**: Protects against CSRF attacks. Generated server-side, stored in an HttpOnly cookie, and verified on callback.
3. **Token transport**: After SSO, tokens are passed via URL query parameters to the frontend. These appear in browser history and server logs. For higher security environments, consider using HTTP-only cookies or a server-side session.
4. **ID token verification**: Performed using the identity provider's JWKS (public key). The signature algorithm is RS256. The verifier checks `iss`, `aud`, and `exp` claims.
5. **JIT provisioning**: SSO users get a randomly generated password hash. They can only authenticate via SSO or by explicitly setting a password through the change-password flow.
