# Playwright E2E

End-to-end tests for the three send paths in the Notifications form: **Quick Send**, **Bulk Send**, and **Broadcast**. Tests intercept the outbound HTTP request and assert payload shape, so a UI bug like the recent `webhook_target` routing-key leak (Slack URL → Discord) fails fast at PR time.

## Prereqs

1. Local stack running: `docker-compose up -d` (UI at `:3000`, API at `:8080`).
2. A real OTP-verified test user with at least one application that has:
    - one `email` template (clone any from the library tab),
    - one `webhook` template (e.g. `webhook_rich_alert`) — required for the routing-regression test,
    - at least one webhook custom provider (optional, used by some specs),
    - at least 2 users (specs auto-create dummy users if fewer exist).
3. Copy `.env.example` to `.env` in this folder and fill `E2E_USER_PASSWORD`.

## Install (one-time)

```powershell
cd ui
npm install
npm run test:e2e:install   # downloads Chromium
```

## Run

```powershell
cd ui
npm run test:e2e           # headless
npm run test:e2e:ui        # Playwright UI mode (interactive)
npm run test:e2e:debug     # step-through debug
```

Or use the **Playwright Test for VSCode** extension Test Explorer.

## What each spec covers

| Spec | UI tab | What it asserts |
|---|---|---|
| `quick-send.spec.ts` | Quick Send | Webhook flow does not leak `webhook_target` from `sample_data` into request `data`; explicit `webhook_url` reaches the wire. Email flow posts `to` + `template`. |
| `bulk-send.spec.ts` | Bulk Send | Selecting 2+ users routes to `POST /v1/notifications/bulk` with `user_ids[]` + `template_id` + `channel`. |
| `broadcast.spec.ts` | Broadcast | Confirmation flow posts to `POST /v1/notifications/broadcast` with `channel` + `template_id` and **no** `user_ids` / `to` / `webhook_url`. |

## How it works

`global-setup.ts` runs once before all specs:

1. Logs in via `POST /auth/login` with the env credentials.
2. Picks the first app (or `E2E_APP_ID`), reads its `api_key`.
3. Discovers an email template, a webhook template, and a webhook custom provider.
4. Ensures ≥ 2 users exist (creates dummies via `POST /users/` otherwise).
5. Writes `.state/state.json` (consumed by specs) and `.state/storage.json` (Playwright `storageState` with auth tokens in `localStorage`, so specs skip the login form).

Specs use `captureRequestBody(page, urlSubstring, action)` from `fixtures.ts` — it awaits the next outbound POST matching the substring and returns the parsed JSON body. This is the key primitive: assertions live on **what the UI sent**, not on what the worker did, so we catch UI-layer payload bugs deterministically.

## Why request-shape, not real delivery?

Today's bug (Slack URL → Discord) was a UI-layer bug — the request body was wrong before it ever hit the worker. A delivery-based test would have masked the root cause behind worker routing. Request-shape tests catch the bug at the boundary that owns it.

A future phase 2 can add a small mock receiver (Express on a random port, registered as a custom provider) to verify end-to-end delivery for the worker's routing precedence. Not done here to keep this scaffold finite.

## When a spec fails

- HTML report opens at `playwright-report/index.html`.
- Traces (DOM + network) retained on failure: `test-results/<spec>/trace.zip` — open with `npx playwright show-trace <path>`.
- Screenshots + video for the failing test alongside the trace.

## Adding new tests

Reuse the `test` and `captureRequestBody` exports from [fixtures.ts](fixtures.ts). State (app id, api key, template ids, users) is exposed on the `state` fixture — never hard-code IDs.
