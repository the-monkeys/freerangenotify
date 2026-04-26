# Handoff: Twilio WhatsApp prod vs local

**Last updated:** 2026-04-16 (from user context; update when state changes).

## What we are debugging

- **Stack:** Meta WhatsApp (partial) + **Twilio** for sends. **SMS (Twilio) works in prod and local.**
- **Gap:** **WhatsApp (Twilio) works locally but not in prod**, with user reporting same Twilio credentials and verified account. **Credentials rotation planned after WA works in prod.**
- **Not in scope for this file:** full Meta/Embedded Signup—focus is **Twilio WhatsApp prod failure**.

## Environments (authoritative)

| | Backend | Frontend |
|---|---------|------------|
| **Local** | `http://localhost:8080` | `http://localhost:3000` |
| **Prod** | `https://freerangenotify.monkeys.support` | `https://freerangenotify.com` (Vercel UI) |

**Prod path:** `Cloudflare → (public IP) → nginx → 192.168.1.14:8080` (on‑prem). User runs Docker Compose from **`./prod` on the server**; that tree was **not** present in a repo snapshot (only `config/config.prod.yaml`, `ui/Dockerfile.prod` here).

## Code fact (worker): phone verification gate (system creds, non‑BYOC)

For **WhatsApp** using **shared/system** Twilio (no per-app BYOC: no `AccountSID`+`AuthToken` in app WhatsApp settings), the worker **blocks** the send if the **app admin** has `phone_verified == false` and logs: `Blocked system WhatsApp send due to unverified phone` / error `phone_verification_required`. See `cmd/worker/processor.go` (WhatsApp block before SMS block).

- **Implication:** If prod **SMS** already works with the **same** “system” path, the admin is likely verified for that app—but still **confirm in worker logs** for a **failed WA** (not assume).
- If prod uses **BYOC** for WhatsApp (per-app sid/token in ES), the gate is skipped when BYOC is active.

## What the next person should do first

1. **One failing prod WA:** capture **worker** log line + notification id (and API response if from quick-send).
2. Distinguish **`phone_verification_required`** vs **Twilio API** body (e.g. template, sender, 630XX errors).
3. Compare **local vs prod** for same app: `app.settings.whatsapp` in ES (BYOC vs system, provider `twilio` vs `meta`).
4. If Twilio: confirm **Content/API template approval** and **WhatsApp sender** for prod match what local uses; Twilio WA often fails on **template** or **from** mismatch even when SMS works.

## Open questions (ask the user if still failing)

- Exact **HTTP status or Twilio error code** in prod worker logs for WhatsApp.
- Is failing send **system Twilio** or **BYOC** for that `app_id`?
- Is the message **freeform body** or **template SID**? (Provider/worker may differ from UI “templates” list.)

## Finding from prod logs (2026-04-26) — not phone verification

- **Worker:** Twilio **[21656](https://www.twilio.com/docs/errors/21656)**: *“The Content Variables parameter is invalid.”*
- **Template in use:** `content_sid` = `HXbbea7c84cb59fd9cab07d5fd246cbd87` (log: “Forcing Twilio WhatsApp provider (content_sid present)”).
- **Implication:** The Twilio **Content** template defines placeholders (e.g. `{{1}}`, `{{2}}`); the JSON sent as `ContentVariables` must use the **matching keys** (often `"1"`, `"2"`, …) and include every required variable. If `content_variables` is missing, wrong shape, or keys do not match the template, Twilio returns 21656. See `internal/infrastructure/providers/whatsapp_provider.go` (only `map` or `string` is forwarded).
- **Service log (same window):** `POST /v1/notifications/bulk` enqueued notifications `498e7511-...` and `7ae03f7f-...` for app `a312b3ec-d621-4253-b2c8-69ce67a6a602`; worker failed those with 21656 above.
- **API fetch note:** A pasted prod JWT once returned `Invalid or expired token` from `GET /v1/apps/`; use a **fresh** Bearer token to pull `GET /v1/apps/:id/settings` and **rotate** if it was shared.
