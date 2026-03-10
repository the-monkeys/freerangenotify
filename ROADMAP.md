# FreeRangeNotify Roadmap

> **Last updated:** March 2026 — after completing the 5 architectural improvements (ES write performance, queue reliability, provider registry, cursor pagination, auth tokens to Redis).

---

## Completed: Architecture Foundation (5 Structural Changes)

- [x] ES write performance: tiered refresh + N+1 fixes + atomic operations
- [x] Queue reliability: visibility timeout + processing set + at-least-once delivery
- [x] Provider registry: self-registration + dynamic channel validation
- [x] Cursor-based pagination: `search_after` + lightweight query builder
- [x] Auth tokens: refresh + password reset moved from ES to Redis with TTL

---

## UI Changes (Optional / Incremental)

| Area | Status | Notes |
|------|--------|-------|
| Cursor pagination in list views | Pending | Notifications, Users, Templates could use "Load more" with `next_cursor` instead of page-based. Backend supports both; migration is UX improvement. |
| Other architectural changes | No UI impact | ES perf, queue, provider registry, auth — all transparent to frontend. |

---

## Phase A — Developer Experience

| # | Item | Status | Description |
|---|------|--------|-------------|
| A1 | OpenAPI / Swagger | Done | Swagger UI at `/swagger/`, OpenAPI spec at `/openapi/swagger.yaml`. Run `make swagger` to regenerate from code. |
| A2 | CLI tool | Done | `frn` command-line tool: send, health, config, version. Run `make build` then `bin/frn --help`. |
| A3 | Code-first SDK (TypeScript/Node.js) | Done | `client.trigger('welcome-email', { to, data })` added. Full SDK in sdk/js. |

---

## Phase B — Feature Parity with Novu

| # | Item | Status | Description |
|---|------|--------|-------------|
| B1 | Embeddable notification center | Done | `<NotificationCenter />` and `<NotificationBell />` in sdk/react. Use `embedded` prop for full-page inbox. |
| B2 | WYSIWYG template editor | Done | Block-based email builder in TemplateEditor: Text, Heading, Image, Button, Divider. Drag to reorder. Variable inserter. |
| B3 | Idempotency keys | Done | `Idempotency-Key` header on quick-send, send, bulk, batch, broadcast. Redis cache 24h. SDK supports `idempotencyKey`. |

---

## Phase C — Advanced Features (Beyond Novu)

| # | Item | Status | Description |
|---|------|--------|-------------|
| C1 | Tenant / organization support | Done | Multi-tenancy: tenants/organizations, member invites (admin/member), apps linked to tenants. UI: Organizations list, member management, optional tenant when creating apps. |
| C2 | Change management | Done | Version history, diff view, promote-to-production (Environments tab → Promote: staging → prod). |
| C3 | Additional providers | Done | AWS SES added. Resend, Mailgun, Vonage, Postmark, SendGrid, Twilio, SMTP, FCM, APNS already exist. |

---

## Current Focus

**Phase C** — C1, C2, and C3 complete.
