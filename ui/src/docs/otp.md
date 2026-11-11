# OTP as a service

Call OTP endpoints from your backend with an application API key. Use exactly one of `recipient`, `user_id`, or `external_id`. A registered user must have the address required by the channel. Direct recipients use an email address or an E.164 phone number. Email needs an available email/SMTP provider; SMS needs an enabled connection and sender; WhatsApp requires the selected account and an approved authentication template when outside its service window. See [Channels](/docs/channels).

## Send an email code

`POST /v1/otp/send` returns HTTP 202 with `request_id`, `notification_id`, `channel`, `expires_at` and `max_attempts`. No code is returned. Track delivery through the notification endpoint; acceptance does not prove receipt.

Defaults are six numeric characters, 300 seconds and five attempts. `length` accepts 4–10, `ttl_seconds` 30–900, `max_attempts` 1–10; `alphanumeric` is optional. A custom `template_body` must contain `{{code}}`. Do not log generated codes or verification input.

## Otp email

HTTP 202. Response below shows the relevant fields.

<!-- api-example:otp-email:request -->
```json
{
  "channel": "email",
  "recipient": "alice@example.test",
  "ttl_seconds": 300
}
```

<!-- api-example:otp-email:curl -->
```bash
curl -sS -X POST "$BASE_URL/otp/send" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "channel": "email",
  "recipient": "alice@example.test",
  "ttl_seconds": 300
}
JSON
```

<!-- api-example:otp-email:response -->
```json
{
  "request_id": "${OTP_REQUEST_ID}",
  "notification_id": "${OTP_NOTIFICATION_ID}",
  "channel": "email",
  "max_attempts": 5
}
```

## Otp invalid code

HTTP 400. Response below shows the relevant fields.

<!-- api-example:otp-invalid-code:request -->
```json
{
  "request_id": "${OTP_REQUEST_ID}",
  "code": "WRONG_CODE"
}
```

<!-- api-example:otp-invalid-code:curl -->
```bash
curl -sS -X POST "$BASE_URL/otp/verify" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "request_id": "${OTP_REQUEST_ID}",
  "code": "WRONG_CODE"
}
JSON
```

<!-- api-example:otp-invalid-code:response -->
```json
{
  "verified": false,
  "request_id": "${OTP_REQUEST_ID}",
  "error": "invalid_code",
  "attempts_remaining": 4
}
```

## Otp cooldown

HTTP 409. Response below shows the relevant fields.

<!-- api-example:otp-cooldown:request -->
```json
{
  "request_id": "${OTP_REQUEST_ID}"
}
```

<!-- api-example:otp-cooldown:curl -->
```bash
curl -sS -X POST "$BASE_URL/otp/resend" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "request_id": "${OTP_REQUEST_ID}"
}
JSON
```

<!-- api-example:otp-cooldown:response -->
```json
{
  "error": "otp: resend cooldown in effect"
}
```

## Otp sms

HTTP 202. Response below shows the relevant fields.

<!-- api-example:otp-sms:request -->
```json
{
  "channel": "sms",
  "recipient": "+14155551234"
}
```

<!-- api-example:otp-sms:curl -->
```bash
curl -sS -X POST "$BASE_URL/otp/send" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "channel": "sms",
  "recipient": "+14155551234"
}
JSON
```

<!-- api-example:otp-sms:response -->
```json
{
  "channel": "sms",
  "max_attempts": 5
}
```

## Otp whatsapp

HTTP 202. Response below shows the relevant fields.

<!-- api-example:otp-whatsapp:request -->
```json
{
  "channel": "whatsapp",
  "recipient": "+14155551234",
  "template_data": {
    "whatsapp_template": {
      "name": "approved_authentication",
      "language": "en_US",
      "components": [
        {
          "type": "body",
          "parameters": [
            {
              "type": "text",
              "text": "{{code}}"
            }
          ]
        }
      ]
    }
  }
}
```

<!-- api-example:otp-whatsapp:curl -->
```bash
curl -sS -X POST "$BASE_URL/otp/send" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  --data @- <<JSON
{
  "channel": "whatsapp",
  "recipient": "+14155551234",
  "template_data": {
    "whatsapp_template": {
      "name": "approved_authentication",
      "language": "en_US",
      "components": [
        {
          "type": "body",
          "parameters": [
            {
              "type": "text",
              "text": "{{code}}"
            }
          ]
        }
      ]
    }
  }
}
JSON
```

<!-- api-example:otp-whatsapp:response -->
```json
{
  "channel": "whatsapp",
  "max_attempts": 5
}
```

## Verify, resend and errors

Send the user's input to `POST /v1/otp/verify` with the same app/environment credential and `request_id`. A successful result has `verified: true`. Repeating the same successful code is idempotent; other codes remain invalid. App/environment ownership is enforced; guessing an ID does not grant access.

Send and resend share a five-per-hour recipient budget. Resend requires a 60-second cooldown and generates a fresh code while preserving the original expiry. After expiry, start a new send rather than resending the expired request. A pending/uncertain delivery blocks verification and resend until authority is resolved; do not blindly retry a send after a timeout.

OTP validation/send errors use a flat `error` string, for example `{"error":"otp: resend cooldown in effect"}` (409). Verify failures use `verified: false`, `request_id`, an error code and, where applicable, `attempts_remaining`. Codes include `invalid_code`/`attempts_exhausted` (400), `not_found` (404), `expired` (410), `operation_pending` (409), and `state_unavailable` (503). Send/resend can return 429 for the recipient budget and 409 for cooldown/already-verified/pending state. Authentication and other middleware can return their separate error envelope; do not parse all errors as OTP codes. SDKs recognize known errors and retain generic API errors for unknown shapes.

The short-lived OTP authority stores a hash, but delivery notifications may retain rendered code or template data. Apply storage access, retention and encryption policies to delivery content; do not claim plaintext is never persisted. See [SDK Guide](/docs/sdk) for backend clients and [Notifications](/docs/notifications) for delivery status.
