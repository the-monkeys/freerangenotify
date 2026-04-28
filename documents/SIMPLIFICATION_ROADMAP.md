# FreeRangeNotify — Simplification Roadmap

> **Audience**: Product, Engineering, Sales  
> **Purpose**: Identify every friction point a new user faces and propose concrete solutions to make FreeRangeNotify dead-simple to adopt.  
> **Date**: March 2026

---

## Table of Contents

1. [The Problem — A Sales Perspective](#1-the-problem--a-sales-perspective)
2. [Current User Journey (Step-by-Step)](#2-current-user-journey-step-by-step)
3. [Pain Point Analysis](#3-pain-point-analysis)
   - [3.1 Onboarding & First Notification](#31-onboarding--first-notification)
   - [3.2 Application Setup](#32-application-setup)
   - [3.3 User Management](#33-user-management)
   - [3.4 Template Management (The Biggest Pain)](#34-template-management-the-biggest-pain)
   - [3.5 Sending Notifications](#35-sending-notifications)
   - [3.6 Monitoring & Debugging](#36-monitoring--debugging)
4. [Proposed Solutions](#4-proposed-solutions)
   - [4.1 Quick-Start Wizard](#41-quick-start-wizard)
   - [4.2 Template Simplification](#42-template-simplification)
   - [4.3 Send Flow Simplification](#43-send-flow-simplification)
   - [4.4 User Management Improvements](#44-user-management-improvements)
   - [4.5 Dashboard & Observability](#45-dashboard--observability)
   - [4.6 SDK & Integration Layer](#46-sdk--integration-layer)
   - [4.7 API Simplification](#47-api-simplification)
5. [Priority Matrix](#5-priority-matrix)
6. [Appendix — Current API Call Sequence](#6-appendix--current-api-call-sequence)

---

## 1. The Problem — A Sales Perspective

Imagine you're a developer evaluating FreeRangeNotify for your SaaS product. You sign up, and here's what you have to do **before you can send a single notification**:

```
Step 1: Register an admin account (POST /v1/auth/register)
Step 2: Log in to get a JWT token (POST /v1/auth/login)
Step 3: Create an Application (POST /v1/apps) → get an API key
Step 4: Create a User (POST /v1/users with the API key)
Step 5: Create a Template (POST /v1/templates with the API key)
Step 6: Now FINALLY send a notification (POST /v1/notifications)
```

**That's 6 API calls and 3 different auth mechanisms** (JWT for admin, API key for users/templates/notifications, and knowing which to use when) — just to see "Hello World" arrive at a webhook.

Compare this to competitors:
- **OneSignal**: Install SDK → call `sendNotification({ message: "Hello" })` → done.
- **the reference platform**: Create trigger → send event → done.
- **Firebase Cloud Messaging**: Get server key → POST to FCM → done.

FreeRangeNotify is architecturally superior to many of these (priority queues, smart delivery, template rendering, multi-channel), but the onboarding friction is killing adoption before users ever see those features.

---

## 2. Current User Journey (Step-by-Step)

Here's what a developer actually experiences today, annotated with friction points:

### Step 1: Authentication Confusion
```
Question: "Do I use JWT or API key?"
Answer: JWT for admin/app management. API key for everything else.
But: This is not documented clearly. AUTH_QUICKSTART.md mentions JWT.
     API_DOCUMENTATION.md mentions API key. Neither explains the relationship.
```

### Step 2: Create Application
```bash
# Need JWT first
curl -X POST http://localhost:8080/v1/auth/register \
  -d '{"username":"admin","email":"admin@x.com","password":"SecurePass123!"}'

curl -X POST http://localhost:8080/v1/auth/login \
  -d '{"email":"admin@x.com","password":"SecurePass123!"}'
# Response: JWT token

# Now create app with JWT
curl -X POST http://localhost:8080/v1/apps \
  -H "Authorization: Bearer <JWT>" \
  -d '{"app_name":"My App"}'
# Response: API key (frn_xxxxx)
```
**Friction**: Two auth flows before you even start.

### Step 3: Create Users
```bash
curl -X POST http://localhost:8080/v1/users \
  -H "Authorization: Bearer frn_xxxxx" \
  -d '{
    "email": "john@example.com",
    "timezone": "UTC",          # Free text, no validation
    "language": "en",           # Free text, no validation
    "preferences": {
      "email_enabled": true,
      "push_enabled": true,
      "sms_enabled": true
    }
  }'
# Response: internal UUID (you need THIS, not the email, to send notifications)
```
**Friction**: The response gives you an internal UUID. You **must** save this UUID to send notifications later. There's no way to send a notification by email address.

### Step 4: Create Template (The Hard Part)
```bash
curl -X POST http://localhost:8080/v1/templates \
  -H "Authorization: Bearer frn_xxxxx" \
  -d '{
    "app_id": "<app_uuid>",        # Wait, why do I need this? It is auto-set from the API key
    "name": "welcome",
    "channel": "email",            # Must match the channel you send on
    "body": "Hello {{.name}}, welcome to {{.product}}!",
    "variables": ["name", "product"],   # Must manually list every variable
    "locale": "en",                # Required even for single-language apps
    "subject": "Welcome!"
  }'
```
**Friction**:  
- `app_id` is required in the DTO but auto-set from context — confusing.  
- `variables` must be manually listed AND match the `{{.name}}` patterns in body — easy to desync.  
- `locale` is required even when you don't need i18n.  
- If you want HTML email newsletters, you paste raw HTML into a JSON string field. No preview, no WYSIWYG, no upload.  
- Templates are **channel-locked** — a "welcome" template for email can't be reused for push.  
- No template editing in the UI — you must delete and re-create (losing the template ID).

### Step 5: Send Notification
```bash
curl -X POST http://localhost:8080/v1/notifications \
  -H "Authorization: Bearer frn_xxxxx" \
  -d '{
    "user_id": "<internal_uuid>",    # NOT the email, not the external ID
    "channel": "email",
    "priority": "normal",
    "template_id": "<template_uuid>",
    "data": {
      "name": "John",
      "product": "Acme"
    }
  }'
```
**Friction**:  
- Must know user's internal UUID (not email or external ID) — requires a lookup.  
- Must know template UUID — requires a lookup.  
- Must know which variables to pass — no discovery mechanism.  
- Channel must be specified AND must match the template's channel — redundant.  
- Even for a simple "Hello World" webhook, you need ALL of the above.

---

## 3. Pain Point Analysis

### 3.1 Onboarding & First Notification

| Issue | Severity | Detail |
|-------|----------|--------|
| **6-step setup for first notification** | Critical | Takes 15-30 minutes for an experienced developer |
| **Dual auth confusion (JWT + API key)** | High | No clear documentation on when to use which |
| **No "Hello World" quickstart** | Critical | No script or wizard to get a working example in < 2 minutes |
| **Migration tool is a no-op** | Medium | `migrate` command prints fake success messages but does nothing |
| **setup_data.py only sets up webhook** | Medium | No email, push, or SSE example data |

### 3.2 Application Setup

| Issue | Severity | Detail |
|-------|----------|--------|
| **AppForm.tsx is outdated** | Low | Uses raw CSS, missing webhook fields; superseded by AppDetail |
| **Settings form is fragile** | Medium | Deep nested state with `as any` casts; no validation feedback |
| **Named webhooks are buried in Overview** | Low | Should be in Settings or dedicated section |
| **No provider status visibility** | Medium | Can't see if SMTP/APNS/FCM is configured and healthy |

### 3.3 User Management

| Issue | Severity | Detail |
|-------|----------|--------|
| **No lookup by email/external ID** | High | Must use internal UUID everywhere |
| **No bulk import** | High | Adding 1000 users requires 1000 API calls |
| **Timezone/language are free text** | Medium | "UTC" vs "utc" vs "America/New_York" — no validation |
| **User table shows minimal info** | Low | No UUID, timezone, or preference summary visible |
| **No phone field in UI** | Medium | Backend supports SMS but UI can't set phone numbers |
| **No device management in UI** | Medium | Push notifications require device tokens but UI can't add them |
| **No pagination** | Medium | Will break with > 100 users |

### 3.4 Template Management (The Biggest Pain)

| Issue | Severity | Detail |
|-------|----------|--------|
| **No template editing** | Critical | Must delete and re-create to change a typo |
| **HTML templates as JSON strings** | Critical | No WYSIWYG, no file upload, no preview in creation flow |
| **Variables must be manually listed** | High | Auto-detect in UI adds but never removes; API requires manual list |
| **Channel-locked templates** | High | Same message needs separate templates per channel |
| **`app_id` required but auto-set** | Medium | Confusing redundant field |
| **Locale required for single-lang** | Medium | Unnecessary friction; should default to "en" |
| **No versioning UI** | Medium | Backend supports versions but UI doesn't expose it |
| **Go text/template syntax** | High | Users must learn `{{.variable}}` syntax; no Handlebars/Mustache |
| **No template library/marketplace** | Medium | No pre-built templates for common use cases |
| **Uses text/template not html/template** | Medium | XSS risk for email HTML content |

### 3.5 Sending Notifications

| Issue | Severity | Detail |
|-------|----------|--------|
| **User ID is internal UUID only** | High | Can't send by email address or external ID |
| **Channel must match template** | Medium | Redundant specification — template already knows its channel |
| **Send form is extremely dense** | High | Webhook targets, scheduling, recurrence, multi-user — all in one form |
| **Multi-send doesn't use bulk endpoint** | Medium | UI fires parallel individual calls instead of `POST /bulk` |
| **No real-time status updates** | Medium | Must manually refresh to see delivery status |
| **No retry/cancel from notification list** | Medium | Backend supports it but UI doesn't expose it |
| **Broadcast does N sequential API calls** | Medium | Backend already has `POST /broadcast` but service loops internally |

### 3.6 Monitoring & Debugging

| Issue | Severity | Detail |
|-------|----------|--------|
| **No delivery log viewer** | High | Can't see provider responses, error messages, or latency |
| **No analytics dashboard** | Medium | Backend model exists but no UI or data pipeline |
| **History has no pagination/filter** | Medium | All notifications fetched at once |
| **DLQ has no dashboard or alerting** | Medium | Failed notifications silently enter dead letter queue |
| **No health status for providers** | Medium | Can't tell if SMTP is connected, APNS cert is valid, etc. |
| **Debug logging in production code** | Low | `fmt.Printf("DEBUG: ...")` statements in notification_handler.go |

---

## 4. Proposed Solutions

### 4.1 Quick-Start Wizard

**Goal**: Send a notification in < 2 minutes from signup.

#### A) "Try It Now" Sandbox Mode
After creating an app, automatically:
1. Create a **sandbox user** with the admin's email
2. Create a **default webhook template** ("Hello {{.name}}!")
3. Show a one-click "Send Test Notification" button
4. Display the webhook/SSE payload in real-time

```
Dashboard after app creation:
┌──────────────────────────────────────────┐
│  ✅ App created! Here's your API key:    │
│  frn_abc123...                           │
│                                          │
│  🚀 Try it now:                          │
│  [Send Test Notification]                │
│                                          │
│  📡 Waiting for delivery...              │
│  ✅ Delivered! Webhook received:         │
│  { "title": "Hello Admin!", ... }        │
└──────────────────────────────────────────┘
```

#### B) Guided Setup Flow (for the UI)
Replace the current "create users → create templates → send" flow with a **step-by-step wizard**:

```
Step 1: What do you want to send?
   [ ] Email  [ ] Push  [ ] SMS  [ ] Webhook  [ ] Browser (SSE)

Step 2: Write your message
   Subject: [Welcome to {{.product}}!        ]
   Body:    [Rich text editor with variables  ]
   
   Detected variables: product, name
   (auto-saved as template)

Step 3: Who should receive it?
   [ ] Paste email addresses
   [ ] Upload CSV
   [ ] Select existing users
   [ ] Everyone (broadcast)

Step 4: When?
   (•) Send now  ( ) Schedule  ( ) Recurring

   [Send Notification →]
```

#### C) One-Line API Quickstart
For API users, provide a **single-call "just send it"** endpoint:

```bash
# NEW: Simplified send — no template, no user pre-registration
curl -X POST http://localhost:8080/v1/quick-send \
  -H "Authorization: Bearer frn_xxxxx" \
  -d '{
    "to": "john@example.com",
    "channel": "email",
    "subject": "Welcome!",
    "body": "Hello John, welcome to Acme!"
  }'
```

The backend would:
1. Auto-create or lookup the user by email
2. Use an inline body (no template required)
3. Default priority to "normal"
4. Return the notification status

### 4.2 Template Simplification

#### A) Enable Template Editing
The backend `PUT /v1/templates/:id` already works. The UI's `AppTemplates.tsx` needs an Edit button + pre-filled form. This is the #1 quickest win.

#### B) Rich HTML Editor for Email Templates
Replace the raw textarea with a WYSIWYG editor:
- **Option 1 (Recommended)**: Integrate [Unlayer](https://unlayer.com/) or [React Email Editor](https://github.com/nickel-org/react-email-editor) — drag-and-drop HTML email builder
- **Option 2**: Use [TipTap](https://tiptap.dev/) for rich text with HTML output
- **Option 3**: File upload endpoint — let users upload `.html` files

#### C) Multi-Channel Templates
Allow a single "template group" to have channel-specific variants:
```json
{
  "name": "welcome",
  "variants": {
    "email": { "subject": "Welcome!", "body": "<h1>Hello {{.name}}</h1>" },
    "push":  { "body": "Welcome to {{.product}}, {{.name}}!" },
    "sms":   { "body": "Welcome {{.name}}! Reply STOP to unsubscribe" }
  }
}
```
When sending, the system auto-selects the right variant based on the channel.

#### D) Template Variable Auto-Detection
Remove the manual `variables` array requirement. The backend should parse `{{.xxx}}` from the body and auto-populate the variables list. The TemplateService already validates them — just reverse the flow.

#### E) Default Locale
Make `locale` default to `"en"` when not specified. Most apps start single-language.

#### F) Pre-Built Template Library
Ship with common templates:
- Welcome email (HTML)
- Password reset
- Order confirmation
- Weekly digest/newsletter
- Alert/warning
- Promotional offer

Users clone from the library instead of starting from scratch.

#### G) Template Preview with Live Data
The backend already has `POST /v1/templates/:id/render`. The UI has a preview panel. Improve it by:
- Showing **rendered HTML** in an iframe (not just text)
- Auto-suggesting test data based on variable names
- Adding a "Send Test" button that delivers to the admin's email

### 4.3 Send Flow Simplification

#### A) Send by Email/External ID (not just internal UUID)
Add a `to` field that accepts:
- Email address → auto-lookup user
- External ID → auto-lookup user  
- Internal UUID → direct (current behavior)

```json
{
  "to": "john@example.com",       // NEW: resolves to user internally
  "template": "welcome",          // NEW: by name, not just UUID
  "data": { "name": "John" }
  // channel auto-inferred from template
  // priority defaults to "normal"
}
```

#### B) Infer Channel from Template
The template already specifies its channel. When sending a notification with a `template_id`, don't require `channel` to be specified separately — infer it from the template.

#### C) Simplified Send Form in UI
Split the current monolithic send form into 3 modes:
1. **Quick Send**: Recipient + Template + Data → Send
2. **Advanced Send**: Full form with scheduling, recurrence, priority, webhook targets
3. **Broadcast**: Template + Data → Send to all

#### D) Use Bulk Endpoint in UI
The `AppNotifications.tsx` multi-send currently fires parallel individual API calls:
```typescript
// Current (broken):
await Promise.all(selectedUsers.map(userId => api.post('/notifications', { user_id: userId, ... })))

// Should be:
await api.post('/notifications/bulk', { user_ids: selectedUsers, ... })
```

### 4.4 User Management Improvements

#### A) Bulk User Import
Add `POST /v1/users/bulk` endpoint accepting CSV or JSON array:
```json
{
  "users": [
    { "email": "alice@x.com", "language": "en" },
    { "email": "bob@x.com", "language": "fr" }
  ]
}
```
The backend `user.Repository` already has `BulkCreate` — just wire it to a handler.

#### B) User Lookup by Email
Add `GET /v1/users?email=john@example.com` support. The repository already has `GetByEmail`.

#### C) Validated Dropdowns
In the UI, replace free-text timezone and language with dropdown selects:
- Timezone: IANA timezone list (America/New_York, Europe/London, etc.)
- Language: ISO 639-1 codes (en, fr, es, de, etc.)

#### D) Phone Number Field in UI
The backend supports `phone` but the UI's create user form doesn't show it. Just add the field.

#### E) Device Token Management in UI
For push notifications (Apple Push Notification service / Firebase Cloud Messaging), expose the `POST /v1/users/:id/devices` endpoint in the UI with fields for platform and token.

### 4.5 Dashboard & Observability

#### A) Notification Activity Feed
Real-time feed showing notification status changes:
```
✅ 12:01 PM — "Welcome email" delivered to john@x.com (142ms)
❌ 12:02 PM — "Order confirm" failed for bob@x.com — SMTP timeout
🔄 12:02 PM — Retrying "Order confirm" (attempt 2/3)
✅ 12:03 PM — "Order confirm" delivered to bob@x.com (89ms)
```

#### B) Provider Health Dashboard
Show the status of each configured provider:
```
┌─────────────┬─────────┬──────────┬──────────────┐
│ Provider    │ Status  │ Latency  │ Success Rate │
├─────────────┼─────────┼──────────┼──────────────┤
│ SMTP        │ ✅ Up   │ 230ms    │ 99.2%        │
│ APNS        │ ✅ Up   │ 120ms    │ 98.8%        │
│ FCM         │ ❌ Down │ —        │ —            │
│ Twilio SMS  │ ✅ Up   │ 340ms    │ 97.5%        │
│ Webhook     │ ✅ Up   │ 85ms     │ 99.9%        │
│ SSE         │ ✅ Up   │ 2ms      │ 100%         │
└─────────────┴─────────┴──────────┴──────────────┘
```

#### C) Dead Letter Queue Viewer
Surface the existing admin DLQ API in the UI:
- List of failed notifications with error messages
- One-click retry
- Bulk retry/dismiss

#### D) Notification Search & Filtering
Add filters to the notification history:
- By status (pending, sent, failed, delivered)
- By channel
- By user
- By date range
- By template
- Full-text search on content

### 4.6 SDK & Integration Layer

#### A) Client SDKs
Provide thin wrapper libraries that hide the setup complexity:

**JavaScript/TypeScript SDK:**
```typescript
import { FreeRangeNotify } from '@freerangenotify/sdk';

const frn = new FreeRangeNotify('frn_your_api_key');

// One-liner to send
await frn.send({
  to: 'john@example.com',
  template: 'welcome',
  data: { name: 'John', product: 'Acme' }
});
```

**Go SDK:**
```go
client := freerangenotify.New("frn_your_api_key")

client.Send(ctx, freerangenotify.Notification{
    To:       "john@example.com",
    Template: "welcome",
    Data:     map[string]any{"name": "John"},
})
```

**Python SDK:**
```python
from freerangenotify import Client

frn = Client("frn_your_api_key")
frn.send(to="john@example.com", template="welcome", data={"name": "John"})
```

#### B) Framework Integrations
- **Next.js receiver** (already exists in `examples/nextjs-receiver`)
- **Express.js middleware** for webhook reception
- **React component** for in-app notification bell
- **Browser SDK** for SSE subscription

#### C) Webhook Playground
An in-dashboard tool that:
1. Generates a temporary webhook URL (like webhook.site)
2. Sends a test notification
3. Shows the received payload in real-time

### 4.7 API Simplification

#### A) Reduce Required Fields
Current required fields for `POST /v1/notifications`:
- `user_id` (internal UUID)
- `channel` (must match template)
- `priority` (usually "normal")
- `template_id` (UUID, not name)

Proposed minimum required fields:
- `to` (email, external ID, or UUID — any works)
- `template` (name or UUID — either works)

Everything else has sensible defaults.

#### B) Template References by Name
Allow templates to be referenced by name instead of UUID:
```json
{ "template": "welcome" }          // Resolves to latest active "welcome" template
{ "template_id": "uuid-123" }      // Explicit UUID (current behavior, still works)
```

#### C) Inline Content (No Template Required)
For simple notifications, allow inline content:
```json
{
  "to": "john@example.com",
  "channel": "email",
  "subject": "Hello!",
  "body": "Welcome to our platform."
}
```
No template creation needed. The system creates a transient content block.

#### D) Unified Auth Documentation
Create a single page explaining:
- **JWT tokens**: For admin operations (app CRUD, dashboard login)
- **API keys** (`frn_` prefix): For programmatic access (send notifications, manage users/templates)
- **When to use which**: Clear decision table
- **SSO**: When and how OIDC works

---

## 5. Priority Matrix

### Phase 1 — Quick Wins (1-2 weeks, immediate impact)

| # | Change | Effort | Impact | Description |
|---|--------|--------|--------|-------------|
| 1 | **Enable template editing in UI** | Low | High | Add Edit button to AppTemplates.tsx — backend already supports PUT |
| 2 | **Default locale to "en"** | Low | Medium | One line change in TemplateService.Create |
| 3 | **Auto-detect variables** | Low | Medium | Remove `variables` from required fields; auto-parse from body |
| 4 | **Fix multi-send to use bulk API** | Low | Medium | Change Promise.all → single POST /bulk call |
| 5 | **Add phone field to UI** | Low | Low | Simple input field addition |
| 6 | **Remove debug fmt.Printf** | Low | Low | Clean up notification_handler.go |
| 7 | **Add pagination to user/template/notification lists** | Medium | Medium | Prevents UI from breaking at scale |

### Phase 2 — Core Simplifications (2-4 weeks)

| # | Change | Effort | Impact | Description |
|---|--------|--------|--------|-------------|
| 8 | **Quick-Send API endpoint** | Medium | Critical | `POST /v1/quick-send` with email resolution and optional inline content |
| 9 | **Send by email / external ID** | Medium | High | User lookup layer in notification service |
| 10 | **Template reference by name** | Medium | High | Allow `"template": "welcome"` in send requests |
| 11 | **Infer channel from template** | Low | Medium | Remove redundant channel requirement |
| 12 | **Simplified send form in UI** | Medium | High | Quick/Advanced/Broadcast mode split |
| 13 | **Timezone/language dropdowns** | Low | Medium | Replace free-text inputs with validated selects |
| 14 | **Unified auth documentation** | Medium | High | Single page explaining JWT vs API key |
| 15 | **Bulk user import endpoint** | Medium | Medium | Wire existing BulkCreate to handler |

### Phase 3 — Premium Experience (1-2 months)

| # | Change | Effort | Impact | Description |
|---|--------|--------|--------|-------------|
| 16 | **WYSIWYG email template editor** | High | Critical | Unlayer/TipTap integration for HTML emails |
| 17 | **Setup wizard in UI** | High | High | Guided flow: channel → message → recipients → send |
| 18 | **Pre-built template library** | Medium | High | Ship common templates (welcome, reset, digest) |
| 19 | **Notification activity feed** | Medium | High | Real-time delivery status stream |
| 20 | **Provider health dashboard** | Medium | Medium | Status, latency, success rate per provider |
| 21 | **DLQ viewer in UI** | Medium | Medium | List, retry, dismiss failed notifications |
| 22 | **Multi-channel template groups** | High | High | Single template with per-channel variants |
| 23 | **Client SDKs (JS, Go, Python)** | High | High | Hide API complexity behind libraries |

### Phase 4 — Competitive Edge (2-3 months)

| # | Change | Effort | Impact | Description |
|---|--------|--------|--------|-------------|
| 24 | **Webhook playground** | Medium | Medium | In-dashboard webhook testing tool |
| 25 | **Notification search & filtering** | Medium | Medium | Full-text search with status/channel/date filters |
| 26 | **React notification bell component** | Medium | Medium | Drop-in UI component for in-app notifications |
| 27 | **Template diff/versioning UI** | Medium | Low | Visual comparison between template versions |
| 28 | **Analytics dashboard** | High | Medium | Delivery rates, channel breakdown, user engagement |
| 29 | **Provider fallback chains** | High | Medium | Auto-failover: SendGrid → SMTP, FCM → APNS |

---

## 6. Appendix — Current API Call Sequence

### Today: Sending a Welcome Email (6 calls, ~3 minutes)

```
1. POST /v1/auth/register    → { username, email, password }    → JWT token
2. POST /v1/auth/login       → { email, password }              → JWT token
3. POST /v1/apps             → { app_name } + JWT               → API key + app_id
4. POST /v1/users            → { email, preferences } + APIkey  → user_id (UUID)
5. POST /v1/templates        → { name, channel, body, variables, locale } + APIkey → template_id
6. POST /v1/notifications    → { user_id, channel, priority, template_id, data } + APIkey → notification
```

### Goal: Sending a Welcome Email (2 calls, ~30 seconds)

```
1. POST /v1/auth/register    → { email, password }              → JWT + auto-created app + API key
2. POST /v1/quick-send       → { to: "john@x.com", subject: "Welcome!", body: "Hello!" } + APIkey → notification
```

### Goal for SDK Users (1 call, ~10 seconds)

```javascript
// Assumes API key is configured
frn.send({ to: "john@x.com", template: "welcome", data: { name: "John" } })
```

---

## Summary

FreeRangeNotify has a **strong architecture** — priority queues, smart delivery, multi-channel routing, template rendering, SSE real-time, circuit breakers, rate limiting, quiet hours, DND. These are enterprise-grade features.

But the **developer experience is optimized for the system's internal complexity, not the user's mental model**. Users don't think in terms of "create application, then create user with internal UUID, then create a channel-locked template with Go syntax variables, then send a notification by specifying the channel again." They think: **"Send this message to this person."**

Every solution in this document moves toward that mental model: fewer required fields, smarter defaults, auto-resolution, and guided flows. The architecture stays the same — we just need to build a simpler surface on top of it.

The quickest wins (Phase 1) can ship in a week and immediately reduce the "time to first notification" from 15 minutes to 5 minutes. Phase 2 gets it under 1 minute. Phase 3 makes it competitive with the best in the market.
