# FreeRangeNotify — Admin CLI Reference

Operator guide for hosted deployments. All commands are implemented in [`cmd/frn/`](../cmd/frn/) under the `frn admin` tree.

For licensing architecture and security design, see [CLI_LICENSE_ADMIN.md](./CLI_LICENSE_ADMIN.md).

---

## Setup

### Build or install the CLI

```bash
go build -o frn ./cmd/frn
# or use a release binary from your distribution channel
```

### Configuration

Settings are read from environment variables and `~/.frn/config.json` (see [`cmd/frn/config.go`](../cmd/frn/config.go)).

| Variable | Purpose |
|----------|---------|
| `FREERANGE_API_URL` | API base URL (default: `http://localhost:8080`) |
| `FREERANGE_ADMIN_TOKEN` | Admin JWT for billing rate-card commands |
| `FREERANGE_OPS_SECRET` | Ops secret for subscription / credit / account commands |

Persist values with `frn config`:

```bash
frn config --set-api-url https://api.freerangenotify.com
frn config --set-admin-token "<admin-jwt>"
frn config --set-ops-secret "<ops-secret>"
frn config --show
```

### Two authentication planes

| Commands | Auth | API prefix |
|----------|------|------------|
| `frn admin billing rates …` | Admin JWT (`FREERANGE_ADMIN_TOKEN`) | `/v1/admin/billing/rates*` |
| `frn admin renew-license`, `grant-credits`, `delete-account` | Ops secret + signed headers | `/v1/ops/*` |

Ops commands send `Authorization: Bearer ops:<secret>` plus `X-Ops-Timestamp`, `X-Ops-Nonce`, and `X-Ops-Signature` (HMAC-SHA256 over method, request URI, timestamp, and nonce).

**Requirements:** hosted deployment, `security.ops_secret` set on the server, billing enabled for rate-card and credit-grant features.

---

## Billing rate card

Rate cards define how many **credits** each channel consumes per send (email, SMS, WhatsApp, etc.). Changes create a new versioned card; `set` both updates a channel cost and activates the new version.

Default channel costs (when no custom card is active) are defined in [`internal/domain/billing/rates.go`](../internal/domain/billing/rates.go):

| Channel | Credits per send |
|---------|------------------|
| `inapp` | 1 |
| `webhook` | 1 |
| `sse` | 1 |
| `email` | 3 |
| `sms` | 80 |
| `whatsapp` | 108 |

Valid `--channel` values: `email`, `sms`, `whatsapp`, `inapp`, `webhook`, `sse`.

### Show active rate card

```bash
frn admin billing rates show
```

Calls `GET /v1/admin/billing/rates`. Prints JSON including the active card’s `version`, `channel_credit_cost`, `credit_value_inr`, and `overage_per_message`.

**Flags:** `--api-url`, `--admin-token`

### Set channel credits (create + activate)

```bash
frn admin billing rates set --channel email --credits 3
frn admin billing rates set --channel sms --credits 80
frn admin billing rates set --channel whatsapp --credits 108
```

Calls `POST /v1/admin/billing/rates/set` with body `{"channel":"<channel>","credits":<n>}`. Clones the current active card, updates one channel’s cost, creates a new version, and activates it immediately.

**Flags:** `--channel` (required), `--credits` (required, > 0), `--api-url`, `--admin-token`

**Note:** Each `set` only changes one channel at a time. Other channels keep their values from the previously active card.

### Activate a specific version

Use when a version was created but not activated, or to re-apply a known good version:

```bash
frn admin billing rates activate --version v1740000000000000000
```

Calls `POST /v1/admin/billing/rates/activate` with `{"version":"<version>"}`.

**Flags:** `--version` (required), `--api-url`, `--admin-token`

Get version strings from `frn admin billing rates show` (field `active.version`) or from the `updated` object returned by `set`.

### Rollback to a previous version

```bash
frn admin billing rates rollback --version v1739000000000000000
```

Calls `POST /v1/admin/billing/rates/rollback` — operationally the same as `activate` (switches the active version to the one you specify). Use the version ID from a prior `show` output before the change.

**Flags:** `--version` (required), `--api-url`, `--admin-token`

### Typical rate-card workflow

```bash
# 1. Inspect current pricing
frn admin billing rates show

# 2. Change one channel (note the new version in the response)
frn admin billing rates set --channel email --credits 4

# 3. Confirm active card
frn admin billing rates show

# 4. If needed, rollback
frn admin billing rates rollback --version v1739000000000000000
```

---

## Grant credits to a user

Adds credits to a user’s wallet **additively** (does not replace plan-included credits). Writes an audit row to the credit ledger (`entry_type: adjust`).

### Command

```bash
frn admin grant-credits \
  --user-id 68118bcc-e29b-41d4-a716-446655440000 \
  --credits 5000 \
  --reason "Partner top-up Q2"
```

Calls `POST /v1/ops/credits/grant`.

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--user-id` | Yes | User UUID (tenant ID for personal accounts) |
| `--credits` | Yes | Positive integer credits to add |
| `--reason` | Yes | Audit reason (stored in ledger metadata) |
| `--api-url` | No | Overrides `FREERANGE_API_URL` |
| `--ops-secret` | No | Overrides `FREERANGE_OPS_SECRET` |

### Preconditions

1. **User exists** — otherwise `404 user not found`.
2. **Active subscription** — otherwise `400 no active subscription`. Create or extend with:
   ```bash
   frn admin renew-license --user-id <uuid> --months 1 --plan standard --reason "Ops setup"
   ```
3. **Credits billing model** — legacy (message-limit) subscriptions return `400 legacy billing model does not support credit grants`. Paid renewal / trial activation with `billing_model: credits` is required.

### Example response

```json
{
  "success": true,
  "data": {
    "tenant_id": "68118bcc-...",
    "credits_total": 5500,
    "credits_remaining": 5100,
    "credits_reserved": 0,
    "credits_granted": 5000
  }
}
```

`credits_total` and `credits_remaining` both increase by the granted amount. Reserved credits are unchanged.

### Common errors

| HTTP | Meaning |
|------|---------|
| `400` | Missing/invalid input, no subscription, or legacy billing |
| `404` | Unknown `user_id` |
| `503` | Credit service unavailable (billing feature disabled on server) |
| `401` | Invalid or missing ops auth |

---

## Other ops commands (summary)

### Renew or create subscription

```bash
frn admin renew-license \
  --user-id <uuid> \
  --months 1 \
  --plan ops_granted \
  --reason "Partner exemption"
```

At least one of `--user-id`, `--tenant-id`, or `--app-id` is required. Uses ops auth → `POST /v1/ops/subscriptions/renew`.

### Delete account (destructive)

```bash
frn admin delete-account --user-id <uuid> --reason "GDPR request" --confirm
```

Requires `--confirm`. Uses ops auth → `DELETE /v1/ops/users/:user_id`.

---

## Command tree

```
frn admin
├── renew-license      # ops: extend/create subscription
├── grant-credits      # ops: add credits to user wallet
├── delete-account     # ops: cascade delete user
└── billing
    └── rates
        ├── show       # admin JWT: active rate card
        ├── set        # admin JWT: update channel + activate
        ├── activate   # admin JWT: switch active version
        └── rollback   # admin JWT: switch to older version
```

---

## HTTP API mapping

| CLI command | Method | Path |
|-------------|--------|------|
| `billing rates show` | `GET` | `/v1/admin/billing/rates` |
| `billing rates set` | `POST` | `/v1/admin/billing/rates/set` |
| `billing rates activate` | `POST` | `/v1/admin/billing/rates/activate` |
| `billing rates rollback` | `POST` | `/v1/admin/billing/rates/rollback` |
| `grant-credits` | `POST` | `/v1/ops/credits/grant` |
| `renew-license` | `POST` | `/v1/ops/subscriptions/renew` |
| `delete-account` | `DELETE` | `/v1/ops/users/:user_id` |

Request/response JSON for ops endpoints is documented in [CLI_LICENSE_ADMIN.md](./CLI_LICENSE_ADMIN.md#requestresponse-contracts).
