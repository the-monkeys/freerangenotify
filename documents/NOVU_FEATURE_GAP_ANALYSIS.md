# FreeRangeNotify vs Novu - Feature Gap Analysis

> Generated: March 6, 2026
> Reference: https://novu.co/

This document compares the feature set of [Novu](https://novu.co/) (the open-source notification infrastructure platform) against FreeRangeNotify and identifies every gap. Features FreeRangeNotify already has are marked for reference; the focus is on what we are **missing**.

---

## Summary

| Category | FRN Has | Missing | Partial |
|----------|---------|---------|---------|
| Channels | 6 | 3 | 0 |
| Inbox / Client SDKs | 3 | 3 | 0 |
| Workflow Engine | 1 | 3 | 1 |
| Content & Templates | 2 | 2 | 0 |
| User Management | 2 | 2 | 1 |
| Platform & Ops | 3 | 4 | 1 |
| Security & Compliance | 3 | 3 | 0 |
| **Total** | **20** | **20** | **3** |

---

## 1. Delivery Channels

| Feature | Novu | FRN | Status |
|---------|------|-----|--------|
| Email (SMTP) | Yes | Yes | **EXISTS** |
| Email (SendGrid) | Yes | Yes | **EXISTS** |
| Push - FCM | Yes | Yes | **EXISTS** |
| Push - APNS | Yes | Yes | **EXISTS** |
| SMS (Twilio) | Yes | Yes | **EXISTS** |
| Webhook | Yes | Yes | **EXISTS** |
| In-App (SSE) | Yes | Yes | **EXISTS** — via SSE + Redis pub/sub |
| Chat - Slack | Yes | No | **MISSING** |
| Chat - WhatsApp | Yes | No | **MISSING** |
| Chat - Discord / Telegram | Yes | No | **MISSING** |
| Custom Channel SDK | Yes | No | **MISSING** — no plugin interface for custom providers |

### Gap Detail

Novu treats chat platforms (Slack, MS Teams, WhatsApp, Discord) as first-class channels with dedicated provider integrations. FRN has no chat provider implementations — Slack/Discord/Telegram only appear as social link URLs in email templates.

**Recommendation:** Implement a `ChatProvider` interface and add Slack (via Incoming Webhooks / Bot API) and Discord (via Webhooks) as the first two. WhatsApp can follow via Twilio's WhatsApp API (shared Twilio credentials).

---

## 2. In-App Inbox & Client SDKs

| Feature | Novu | FRN | Status |
|---------|------|-----|--------|
| `<Inbox />` drop-in component | Yes | Partial | **EXISTS** — `<NotificationBell>` in `@freerangenotify/react` |
| `<Preferences />` component | Yes | No | **MISSING** — preferences API exists but no embeddable UI component |
| Bell indicator with unread count | Yes | Yes | **EXISTS** |
| Notification tabs/categories | Yes | No | **MISSING** — no tab-based filtering (e.g., "All", "Alerts", "Updates") |
| Headless SDK (vanilla JS) | Yes | Yes | **EXISTS** — `sdk/js` |
| Notification actions (mark read, archive, bulk) | Yes | No | **MISSING** — no bulk actions; mark-read is partial |
| Snooze | Yes | No | **MISSING** |

### Gap Detail

Novu's `<Inbox />` supports notification tabs, bulk actions (mark all read, archive), snooze-to-later, and a built-in `<Preferences />` component. FRN has a functional `<NotificationBell>` with SSE-based real-time delivery but lacks tab filtering, bulk operations, snooze, and an embeddable preferences component.

**Recommendation:**
- Add a `<Preferences />` React component that calls the existing preferences API.
- Add tab support to `<NotificationBell />` using notification categories.
- Add `PATCH /v1/notifications/bulk` endpoint for mark-read/archive operations.
- Snooze can be a deferred feature (requires delay queue integration).

---

## 3. Workflow & Orchestration Engine

| Feature | Novu | FRN | Status |
|---------|------|-----|--------|
| Scheduling / Delay step | Yes | Yes | **EXISTS** — `ScheduledAt`, cron recurrence |
| Digest / Batching engine | Yes | No | **MISSING** |
| Multi-step workflow orchestration | Yes | No | **MISSING** |
| Conditional branching | Yes | No | **MISSING** |
| Code-first workflow DSL (Framework SDK) | Yes | No | **MISSING** |
| Throttle step (per-subscriber) | Yes | No | **PARTIAL** — app-level rate limiter exists, no per-subscriber throttle |

### Gap Detail

This is the **largest feature gap**. Novu's core product is a workflow engine where a single trigger can execute a chain of steps: in-app → wait 1h → check if read → if not, send email → digest for 24h → send SMS summary. FRN has none of this — it processes each notification as an isolated unit.

**Novu Digest Engine:**
- Time-based: collect events in a window (e.g., 1 hour), deliver one combined message.
- Look-back: check if a digest is already pending, append to it.
- Cron-scheduled: deliver digests on a schedule (e.g., every Monday 9am).

**Novu Workflow Framework:**
- Developers define workflows in TypeScript with `workflow('name', async ({ step }) => { ... })`.
- Steps: `step.inApp()`, `step.email()`, `step.sms()`, `step.digest()`, `step.delay()`, `step.push()`.
- Each step can have `skip()` conditions and custom control schemas.
- Product teams can edit content controls without touching code.

**Recommendation:** This requires a phased approach:
1. **Phase 1 — Digest Engine:** Add a digest step that aggregates notifications by (user_id + template + key) within a time window using Redis sorted sets.
2. **Phase 2 — Workflow Model:** Define a `Workflow` entity with ordered steps (channel steps, delay, digest, condition). Store in Elasticsearch.
3. **Phase 3 — Orchestrator:** Build a workflow runner in the worker that executes steps sequentially with state persistence.
4. **Phase 4 — Code-first SDK:** Extend the Go SDK to define workflows programmatically.

---

## 4. Content & Template Management

| Feature | Novu | FRN | Status |
|---------|------|-----|--------|
| Template variables with Go templates | N/A | Yes | **EXISTS** |
| Template library (seed/clone) | Partial | Yes | **EXISTS** |
| Block-based visual email editor | Yes | No | **MISSING** |
| React Email integration | Yes | No | **MISSING** |
| Non-technical content editing | Yes | No | **MISSING** — templates require HTML knowledge |
| Template versioning | Yes | No | **MISSING** |

### Gap Detail

Novu provides a block-based email editor powered by React Email where non-technical users can create and edit emails visually. FRN templates are raw HTML with Go template variables — only developers can create/modify them.

**Recommendation:**
- **Short-term:** Integrate an open-source block email editor (e.g., [Email Editor](https://usewaypoint.github.io/email-builder-js/)) into the FRN dashboard.
- **Long-term:** Add template versioning with diff preview and rollback.

---

## 5. User & Subscriber Management

| Feature | Novu | FRN | Status |
|---------|------|-----|--------|
| User/Subscriber CRUD | Yes | Yes | **EXISTS** |
| User preferences (per-channel) | Yes | Yes | **EXISTS** |
| Topics / Subscriber groups | Yes | No | **MISSING** |
| Segments / Audience targeting | Yes | No | **MISSING** |
| Broadcast to all users | Yes | Yes | **EXISTS** |
| User preference UI component | Yes | No | **MISSING** — API-only |

### Gap Detail

Novu has **Topics** — named groups of subscribers that you can send to with a single trigger. For example, `topic: "project-123-watchers"` delivers to all subscribers added to that topic. FRN only has broadcast (all users of an app) or individual send — no middle ground.

**Recommendation:**
- Add a `Topic` entity: `{ id, app_id, name, subscriber_ids[] }`.
- API: `POST /v1/topics`, `POST /v1/topics/:id/subscribers`, `POST /v1/notifications` with `topic_id`.
- The worker fans out the topic into individual notifications at enqueue time.

---

## 6. Platform & Operations

| Feature | Novu | FRN | Status |
|---------|------|-----|--------|
| Activity feed / Notification logs | Yes | Yes | **EXISTS** — Dashboard + ES storage |
| Multi-tenancy (app isolation) | Yes | Yes | **EXISTS** — per-app API keys |
| Multiple environments (dev/staging/prod) | Yes | Partial | **PARTIAL** — config-level only, not a product feature |
| Analytics dashboard | Yes | Partial | **PARTIAL** — Prometheus metrics exist, no analytics UI |
| Template translations (i18n) | Yes | Partial | **PARTIAL** — locale field on templates, but no translation management |
| Audit logs | Yes | No | **MISSING** |
| RBAC (Role-Based Access Control) | Yes | No | **MISSING** |
| Team member management | Yes | No | **MISSING** |
| Changelog / Version history | Yes | No | **MISSING** |

### Gap Detail

**Environments:** Novu lets you manage dev/staging/prod environments within the product UI with separate API keys, subscribers, and templates per environment. FRN only switches via config files.

**RBAC:** Novu supports roles (Admin, Editor, Viewer) with scoped permissions. FRN has JWT + API key auth but no role model.

**Audit Logs:** Novu records all admin actions (template edits, setting changes) with timestamps and actors. FRN has this documented as a TODO.

**Recommendation:**
- **RBAC:** Add a `Role` enum to the app/user relationship. Enforce in middleware.
- **Audit Logs:** Log all mutating API calls to an `audit_log` Elasticsearch index.
- **Environments:** Add an `environment` dimension to API keys and data — this is a significant architectural change.

---

## 7. Security & Compliance

| Feature | Novu | FRN | Status |
|---------|------|-----|--------|
| HMAC signing (webhooks) | Yes | Yes | **EXISTS** |
| JWT authentication | Yes | Yes | **EXISTS** |
| API key authentication | Yes | Yes | **EXISTS** |
| HMAC for Inbox component | Yes | No | **MISSING** — SSE endpoint uses plain user_id |
| SOC 2 Type II | Yes | No | **MISSING** — not applicable to self-hosted |
| HIPAA compliance | Yes | No | **MISSING** |
| Data residency (multi-region) | Yes | No | **MISSING** |
| SSO (SAML/OIDC) for dashboard | Yes | Yes | **EXISTS** — OIDC via Monkeys Identity |

### Gap Detail

**HMAC for Inbox:** Novu signs the subscriber identifier so that the `<Inbox />` component can't be spoofed by changing the user ID in the client. FRN's SSE endpoint (`/v1/sse?user_id=...`) has no such protection.

**Recommendation:** Add HMAC validation to the SSE endpoint. The server generates `HMAC(user_id, app_secret)` which the client must pass as a query param or header.

---

## Priority Matrix

Ranked by impact on feature parity and user adoption:

| Priority | Feature | Effort | Impact |
|----------|---------|--------|--------|
| **P0** | Digest / Batching engine | High | Critical — this is Novu's headline feature |
| **P0** | Workflow orchestration (multi-step) | Very High | Critical — enables delay → check → fallback patterns |
| **P1** | Topics / Subscriber groups | Medium | High — enables targeted notifications |
| **P1** | HMAC for SSE/Inbox | Low | High — security gap |
| **P1** | `<Preferences />` component | Low | Medium — preferences API exists already |
| **P2** | Block-based email editor | High | Medium — major UX improvement for non-devs |
| **P2** | Chat channels (Slack/Discord) | Medium | Medium — broadens channel coverage |
| **P2** | RBAC | Medium | Medium — required for team usage |
| **P2** | Audit logs | Low | Medium — compliance requirement |
| **P3** | Notification tabs + bulk actions | Low | Low — inbox UX polish |
| **P3** | Per-subscriber throttle | Medium | Low — niche use case |
| **P3** | Template versioning | Medium | Low — developer convenience |
| **P3** | Code-first workflow SDK | Very High | Low — depends on workflow engine first |
| **P3** | Snooze | Medium | Low — consumer feature |
| **P3** | Multi-environment product feature | Very High | Low — fundamental architecture change |

---

## What FreeRangeNotify Does Better

FRN is not just behind — it has strengths Novu lacks:

1. **Smart Delivery / Presence Routing** — Dynamic delivery to active receiver sessions via Redis presence. Novu has no equivalent.
2. **Newsletter Broadcaster** — Automated blog-to-newsletter pipeline with blog API integration. Novu is not designed for content curation.
3. **Truly self-hosted first** — No cloud dependency. Novu's self-hosted option is secondary to their SaaS offering.
4. **Single binary deployment** — One Docker image runs both server and worker. Novu requires 5+ services (API, worker, web, MongoDB, Redis).
5. **Elasticsearch-native** — Full-text search across all notifications, templates, and users. Novu uses MongoDB.

---

## Conclusion

The two biggest gaps that prevent FreeRangeNotify from being a credible Novu alternative are:

1. **No Workflow/Digest Engine** — Without multi-step orchestration and event digesting, FRN is a notification delivery service, not a notification *platform*.
2. **No Topics** — Without subscriber grouping, every notification is either "send to one user" or "broadcast to everyone" — no middle ground for targeted delivery.

Closing these two gaps would cover ~60% of the feature distance. The remaining items (block editor, RBAC, chat channels, etc.) are incremental improvements that can be tackled over time.
