# OTP as a Service

Programmatic one-time-passcode (OTP) delivery and verification over **SMS**, **WhatsApp**, and **Email**. Use it for sign-in, sign-up, step-up auth, and transaction confirmation flows.

The send path reuses the standard notification pipeline (templates, providers, credit billing). The verify path adds constant-time comparison, atomic attempt counting, and per-recipient rate limiting — security primitives that are not safe to reimplement on top of `/v1/notifications` yourself.

---

## Endpoints

All three require the `X-API-Key` header. Your `app_id` is derived from the API key server-side — it is never accepted from the request body.

| Method | Path             | Purpose                                          |
| ------ | ---------------- | ------------------------------------------------ |
| POST   | `/v1/otp/send`   | Generate a code, hash it, dispatch via channel   |
| POST   | `/v1/otp/verify` | Verify a code against a `request_id`             |
| POST   | `/v1/otp/resend` | Re-issue a fresh code (60 s cooldown per record) |

---

## Quick start

### 1. Send a code

SMS, defaults (6 digits, 5-minute TTL, 5 attempts):

```bash
curl -X POST http://localhost:8080/v1/otp/send \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"channel":"sms","recipient":"+14155551212"}'
```

Response (`202 Accepted`):

```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "notification_id": "abc123",
  "channel": "sms",
  "expires_at": "2026-05-24T12:35:00Z",
  "ttl_seconds": 300,
  "max_attempts": 5
}
```

Persist the `request_id` in your session/cache — your verify call needs it.

### 2. Verify the code

```bash
curl -X POST http://localhost:8080/v1/otp/verify \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"request_id":"550e8400-e29b-41d4-a716-446655440000","code":"482913"}'
```

Success (`200 OK`):

```json
{ "verified": true, "request_id": "...", "verified_at": "2026-05-24T12:31:42Z" }
```

Wrong code (`400`):

```json
{ "verified": false, "error": "invalid_code", "attempts_remaining": 4 }
```

### 3. Resend (after the 60 s cooldown)

```bash
curl -X POST http://localhost:8080/v1/otp/resend \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"request_id":"550e8400-e29b-41d4-a716-446655440000"}'
```

The previous code is invalidated, a fresh code is generated, and the attempt counter resets.

---

## Channels & recipient formats

There are **three ways** to identify the recipient. Supply exactly one of `recipient`, `user_id`, or `external_id` in the request body. The `channel` field always selects which contact address is used.

| Identifier      | What you pass                                | When to use                                                      |
| --------------- | -------------------------------------------- | ---------------------------------------------------------------- |
| `recipient`     | Raw email or E.164 phone                     | Anonymous flows; first-time users. Auto-creates a user record.   |
| `user_id`       | Internal FRN `user_id` (UUID)                | You stored FRN's `user_id` when creating the user.               |
| `external_id`   | Your own user identifier                     | You stored your customer ID as the user's `external_id`.         |

When `user_id` or `external_id` is supplied, the channel-appropriate contact field is read from the stored user record:

| Channel    | Reads from `user`           | Raw `recipient` format     |
| ---------- | --------------------------- | -------------------------- |
| `sms`      | `phone`                     | E.164, e.g. `+14155551212` |
| `whatsapp` | `phone`                     | E.164, e.g. `+14155551212` |
| `email`    | `email`                     | RFC 5322 address           |

If the resolved user has no value for the field the channel needs (e.g. `channel=sms` but the user record has no `phone`), the API returns `400` with `error: "otp: resolved user has no address for the requested channel"`.

The first send to a *raw* recipient auto-creates a user record with `email_enabled`, `sms_enabled`, and `whatsapp_enabled` all `true`. Sends by `user_id` / `external_id` never auto-create.

### Examples

Send by `external_id`:

```bash
curl -X POST https://api.freerangenotify.com/v1/otp/send \
  -H "X-API-Key: $FRN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"channel":"email","external_id":"customer_ext_42"}'
```

Send by internal `user_id`:

```bash
curl -X POST https://api.freerangenotify.com/v1/otp/send \
  -H "X-API-Key: $FRN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"channel":"sms","user_id":"11111111-2222-3333-4444-555555555555"}'
```

---

## Customising the message body

Bring-your-own body via `template_body`. It **must** contain a `{{code}}` placeholder.

| Placeholder                              | Value                                |
| ---------------------------------------- | ------------------------------------ |
| `{{code}}`, `{{ code }}`, `{{.code}}`    | The generated OTP                    |
| `{{ttl}}`, `{{ ttl }}`, `{{.ttl}}`       | TTL **in minutes** (e.g. `5`)        |
| `{{<key>}}` from `template_data`         | Any extra string variable you pass   |

Example:

```json
{
  "channel": "whatsapp",
  "recipient": "+14155551212",
  "template_body": "Your {{app_name}} code is {{code}}. Expires in {{ttl}} minutes.",
  "template_data": { "app_name": "Acme" }
}
```

If `template_body` is omitted, a channel-appropriate default is used (e.g. `Your verification code is 482913. It expires in 5 minutes.`).

---

## Code generation knobs

| Field          | Default | Range  | Notes                                                                                          |
| -------------- | ------- | ------ | ---------------------------------------------------------------------------------------------- |
| `length`       | 6       | 4–10   | Number of characters in the code.                                                              |
| `alphanumeric` | false   |        | When `true`, draws from `ABCDEFGHJKLMNPQRSTUVWXYZ23456789` — lookalikes `0/O`, `1/I/l` excluded. |
| `ttl_seconds`  | 300     | 30–900 | Code lifetime (max 15 minutes).                                                                |
| `max_attempts` | 5       | 1–10   | Verify attempts allowed before `attempts_exhausted`.                                            |

---

## Security model

- **Storage** — Only `SHA-256(code + 16-byte random salt)` is persisted in Redis. The plaintext code is discarded immediately after dispatch and is **never logged**.
- **Verification** — `crypto/subtle.ConstantTimeCompare` guards against timing oracles. Attempt counting uses an atomic Redis `WATCH/MULTI` loop so concurrent guesses cannot exceed `max_attempts`.
- **Per-recipient rate limit** — Fixed window of 5 sends per `(app_id, recipient)` per 10 minutes. Exceeding it returns `429`.
- **Resend cooldown** — 60 seconds per `request_id`, enforced server-side.
- **Tenant isolation** — `app_id` comes from the API key, never the body. Each record is keyed by a v4 UUID, so cross-tenant `request_id` access is impossible.

---

## Error reference

| HTTP | `error`              | Cause                                                            | Endpoint(s)             |
| ---- | -------------------- | ---------------------------------------------------------------- | ----------------------- |
| 400  | _validation message_ | Bad JSON or DTO violation                                        | send / verify / resend  |
| 400  | _validation message_ | Invalid channel, recipient, length, TTL, attempts, or template   | send / resend           |
| 400  | `otp: provide exactly one of recipient, user_id, or external_id` | More than one identifier supplied | send |
| 400  | `otp: resolved user has no address for the requested channel` | Resolved user lacks `email`/`phone` for the chosen channel | send |
| 400  | `invalid_code`       | Code mismatch (response also carries `attempts_remaining`)       | verify                  |
| 400  | `attempts_exhausted` | `max_attempts` reached on this `request_id`                      | verify                  |
| 401  | _auth message_       | Missing / invalid `X-API-Key`                                    | all                     |
| 404  | `not_found`          | `request_id` does not exist (or already expired and purged)      | verify / resend         |
| 404  | `otp: user not found` | `user_id` / `external_id` does not match any user in this app   | send                    |
| 409  | `already_verified`   | The OTP was already consumed                                     | resend                  |
| 409  | `resend_cooldown`    | Within the 60 s cooldown                                         | resend                  |
| 410  | `expired`            | OTP TTL elapsed                                                  | verify / resend         |
| 429  | `rate_limited`       | Per-recipient send budget exceeded                               | send / resend           |
| 500  | `internal_error`     | Unhandled server failure                                         | all                     |

---

## Billing

Each `send` and each `resend` dispatches one notification through the standard pipeline, which charges the per-channel credit rate just like `/v1/notifications`. **There is no separate OTP rate card.** The OTP endpoints are a thin security policy layer (generate → hash → verify) on top of channels you are already paying for.

See [Pricing & Credits](/docs/pricing) for current per-channel rates.

---

## Dashboard surface

The OTP API is intentionally **programmatic-only** in v1 — there are no dedicated OTP authoring or inspection screens in the dashboard, by design:

- OTP records are short-lived (≤ 15 minutes) and security-sensitive. The plaintext code is never persisted, so a "recent OTPs" panel could only display hashes (useless) or, worse, leak plaintext into the UI.
- Standard delivery analytics (provider status, latency, cost) for each OTP are already visible via the linked `notification_id` in the existing **Notifications** view.

If you need an "OTP diagnostics" panel that surfaces `request_id`, channel, status, and attempts (without ever exposing the code), tell us — it's a candidate for a future release.

---

## See also

- [API Reference](/docs/api) — full OpenAPI schemas (`docs/openapi/otp.yaml`)
- [Channels](/docs/channels) — underlying SMS / WhatsApp / Email provider configuration
- [Pricing & Credits](/docs/pricing) — per-channel credit rates

---

## SDK usage

Both the Go and JavaScript/TypeScript SDKs ship typed wrappers under the `OTP` / `otp` sub-client. Use them instead of raw HTTP — they map every API failure mode to a typed error you can branch on without parsing response bodies.

### Go

```go
import (
    "context"
    "errors"

    "github.com/the-monkeys/freerangenotify/sdk/go/freerangenotify"
)

client := freerangenotify.New("frn_xxx")

// 1. Send
res, err := client.OTP.Send(ctx, freerangenotify.OTPSendParams{
    Channel:   freerangenotify.OTPChannelSMS,
    Recipient: "+14155551212",
})
if errors.Is(err, freerangenotify.ErrOTPRateLimited) {
    // back off
}
if err != nil { return err }

// 2. Verify
verify, err := client.OTP.Verify(ctx, freerangenotify.OTPVerifyParams{
    RequestID: res.RequestID,
    Code:      userInput,
})
switch {
case errors.Is(err, freerangenotify.ErrOTPInvalidCode):
    // show "wrong code, X attempts left" using verify.AttemptsRemaining
case errors.Is(err, freerangenotify.ErrOTPAttemptsExhausted):
    // lock the form
case errors.Is(err, freerangenotify.ErrOTPExpired):
    // prompt to resend
case err != nil:
    return err
default:
    // verify.Verified == true
}

// 3. Resend
_, _ = client.OTP.Resend(ctx, res.RequestID)
```

Sentinel errors: `ErrOTPInvalidCode`, `ErrOTPAttemptsExhausted`, `ErrOTPExpired`, `ErrOTPNotFound`, `ErrOTPAlreadyVerified`, `ErrOTPResendCooldown`, `ErrOTPRateLimited`. The underlying `*APIError` is still reachable via `errors.As` if you need the raw HTTP status / body.

### TypeScript / JavaScript

```ts
import { FreeRangeNotify, OTPError } from '@freerangenotify/sdk';

const client = new FreeRangeNotify('frn_xxx');

// 1. Send
const res = await client.otp.send({
    channel: 'sms',
    recipient: '+14155551212',
});

// 2. Verify
try {
    const verify = await client.otp.verify({ requestId: res.request_id, code: userInput });
    // verify.verified === true
} catch (err) {
    if (err instanceof OTPError) {
        switch (err.code) {
            case 'invalid_code':       /* show err.attemptsRemaining */ break;
            case 'attempts_exhausted': /* lock the form */              break;
            case 'expired':            /* prompt to resend */           break;
            case 'not_found':          /* session lost */               break;
            default: throw err;
        }
    } else { throw err; }
}

// 3. Resend
await client.otp.resend(res.request_id);
```

`OTPError.code` values: `invalid_code` · `attempts_exhausted` · `expired` · `not_found` · `already_verified` · `resend_cooldown` · `rate_limited`.
