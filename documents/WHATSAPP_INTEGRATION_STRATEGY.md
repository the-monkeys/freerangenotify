# WhatsApp Business Integration Strategy — FreeRangeNotify

**Author**: Engineering Team  
**Date**: April 8, 2026  
**Status**: Strategic Planning Document  

---

## Table of Contents

1. [Current State — What We Have](#1-current-state--what-we-have)
2. [WhatsApp App vs. WhatsApp Business API — Why Pay?](#2-whatsapp-app-vs-whatsapp-business-api--why-pay)
3. [Twilio vs. Meta Cloud API — Direct Comparison](#3-twilio-vs-meta-cloud-api--direct-comparison)
4. [Strategic Decision: Provider Roadmap](#4-strategic-decision-provider-roadmap)
5. [BYOC (Bring Your Own Credentials) — User Number Registration](#5-byoc-bring-your-own-credentials--user-number-registration)
6. [FreeRangeNotify Value Proposition Over Native WhatsApp](#6-freerangenotify-value-proposition-over-native-whatsapp)
7. [Feature Gap Analysis — What to Build Next](#7-feature-gap-analysis--what-to-build-next)
8. [Implementation Phases](#8-implementation-phases)
9. [Meta API Pricing Deep Dive (April 2026)](#9-meta-api-pricing-deep-dive-april-2026)
10. [Architecture — Dual Provider Design](#10-architecture--dual-provider-design)
11. [Meta Tech Provider — Full Feature Access Map](#11-meta-tech-provider--full-feature-access-map)
12. [Two-Way Messaging — Inbound Message Architecture](#12-two-way-messaging--inbound-message-architecture)
13. [What We Build That Others Don't](#13-what-we-build-that-others-dont)

---

## 1. Current State — What We Have

### WhatsApp Provider
- **Backend**: `internal/infrastructure/providers/whatsapp_provider.go`
- **Provider**: Twilio-backed (uses `https://api.twilio.com/2010-04-01/Accounts/{SID}/Messages.json`)
- **Config**: `AccountSID`, `AuthToken`, `FromNumber` (Twilio credentials)
- **Per-App Override**: BYOC system via `application.WhatsAppAppConfig` — each tenant app can supply their own Twilio credentials
- **Features**: Media attachments, title+body formatting, credential source tracking (System vs BYOC)
- **Limitations**: No Meta Cloud API support, no HSM template management, no delivery webhooks from Meta

### Notification System (Relevant Features)
| Feature | Status | How It Works |
|---|---|---|
| **Broadcast** | ✅ Working | `newsletter-broadcaster` sends to all app users via SDK `Broadcast()` |
| **Topics/Pub-Sub** | ✅ Working | Users subscribe to topics; `SendToTopic()` resolves subscribers and delivers |
| **Workflows** | ✅ Working | Multi-step pipelines: Channel → Delay → Digest → Condition branching |
| **Digest Rules** | ✅ Working | Batch notifications by `DigestKey` over time window, then deliver aggregated |
| **Scheduled** | ✅ Working | One-time (`ScheduledAt`) + recurring (`CronExpression`) with IANA timezone |
| **Smart Delivery** | ✅ Working | Presence-based routing via Redis; dynamic URL override for active users |
| **Channel Routing** | ✅ Working | 10 channels: push, email, SMS, webhook, in_app, SSE, Slack, Discord, WhatsApp, Teams |
| **Provider Fallback** | ✅ Working | Ordered fallback list per channel per app (e.g., SendGrid → Mailgun → SMTP) |
| **Template System** | ✅ Working | `{{variable}}` syntax, per-channel templates, versioning, locale support |

### SMS Providers
| Provider | Status | Implementation |
|---|---|---|
| Twilio | ✅ Working | Official SDK, BYOC per-app | 
| Vonage | ✅ Working | Raw HTTP, API key/secret |

---

## 2. WhatsApp App vs. WhatsApp Business API — Why Pay?

This is the core question: *"WhatsApp already lets you broadcast and send to lists for free. Why would someone pay for API access through FreeRangeNotify?"*

### What WhatsApp App Gives You Free
| Feature | WhatsApp Personal | WhatsApp Business App |
|---|---|---|
| 1-to-1 messaging | ✅ | ✅ |
| Broadcast lists | ✅ (256 limit) | ✅ (256 limit) |
| Groups | ✅ (1024 members) | ✅ (1024 members) |
| Business profile | ❌ | ✅ |
| Quick replies | ❌ | ✅ (saved responses) |
| Labels | ❌ | ✅ |
| Catalog | ❌ | ✅ |
| Auto-away messages | ❌ | ✅ |

### Why Free WhatsApp Breaks Down for Businesses

**1. Scale Ceiling**
- Broadcast list: **256 contacts max**, recipient must have saved your number
- Groups: Max 1024, recipients see each other's numbers (privacy disaster)
- No API → everything is manual, one phone at a time

**2. No Automation**
- No scheduled messages
- No triggered notifications (e.g., order shipped, payment received)
- No workflow chains (delay → condition → deliver)
- No digest batching

**3. No Integration**
- Can't connect to your CRM, ERP, or e-commerce platform
- Can't trigger from app events (user signup, cart abandonment, etc.)
- No webhooks for delivery status tracking

**4. No Multi-Agent / Team Support**
- Single phone, single operator
- No role-based access, no audit trail
- No handoff between agents

**5. No Analytics**
- No delivery rate tracking per campaign
- No read rate analytics
- No A/B testing on templates
- No cost attribution per tenant

**6. Compliance and Reputation Risk**
- WhatsApp bans numbers that mass-send without API approval
- No official opt-in tracking
- No template quality scoring (Meta penalizes low-quality messages)

### The FreeRangeNotify Answer
FreeRangeNotify turns WhatsApp from a "manual phone feature" into a **programmable infrastructure channel** — on par with email and SMS. The value is not sending a single message; it's orchestrating millions of messages across channels with scheduling, workflows, digests, topics, and multi-tenancy.

---

## 3. Twilio vs. Meta Cloud API — Direct Comparison

### Cost Comparison (India, per message, April 2026)

| Category | Meta Cloud API Direct | Twilio WhatsApp |
|---|---|---|
| **Marketing template** | ₹0.80–1.05 (volume-tiered) | ₹0.80–1.05 + Twilio markup (~$0.005/msg) |
| **Utility template** | Free within CSW; ₹0.14 outside | Same + Twilio markup |
| **Authentication** | ₹0.28 | Same + Twilio markup |
| **Non-template (within CSW)** | **Free** | **Free** + Twilio per-message fee |
| **Free entry point (72h)** | **All messages free** | Same + Twilio fee |

**Bottom line: Twilio adds ~$0.005–0.008 per message on top of Meta's price.** At scale (100K+ messages/month), this adds up to $500–800/month in pure middleman cost.

### Feature Comparison

| Capability | Meta Cloud API | Twilio WhatsApp |
|---|---|---|
| **Setup complexity** | Medium (Meta Business Manager, app review, webhook setup) | Low (Twilio console, API key, done) |
| **Template management** | Full API: create, update, delete, review status | Via Twilio Content API (wraps Meta) |
| **Template approval** | Direct with Meta (faster) | Through Twilio → Meta (can be slower) |
| **Delivery webhooks** | Native (status: sent, delivered, read, failed) | Twilio StatusCallback (wraps Meta webhooks) |
| **Media support** | Images, video, documents, audio, stickers, location | Same (forwarded) |
| **Interactive messages** | Buttons, list messages, flows, CTA URLs | Partial (buttons, lists) |
| **WhatsApp Flows** | ✅ (multi-screen forms inside chat) | ❌ Not supported |
| **WhatsApp Business Calling** | ✅ (VOIP calls via API) | ❌ Not supported |
| **Marketing Messages API** | ✅ (quality-based delivery, auto-optimization, benchmarks) | ❌ Not available |
| **Group messaging API** | ✅ (create/manage groups via API) | ❌ Not available |
| **Throughput** | 80 msg/sec default (upgradable) | 1 msg/sec per number (shared Twilio infra) |
| **Phone number BYO** | ✅ Register any number (landline, mobile, virtual) | ✅ But goes through Twilio |
| **Rate limits** | Generous (scales with verification) | More restrictive |
| **Webhook reliability** | Direct from Meta (lower latency) | Proxied through Twilio |
| **Multi-WABA support** | ✅ (multiple business accounts per portfolio) | ✅ (but limited control) |
| **Analytics** | Rich: pricing_analytics, template_analytics | Basic |
| **SDK/Library** | No official Go SDK (raw HTTP/Graph API) | Official Go SDK |
| **Documentation** | Good, but scattered across Graph API docs | Excellent, Twilio-quality |

### Verdict

| Use Case | Recommendation |
|---|---|
| Quick MVP / proof of concept | **Twilio** — 30-minute setup, familiar API |
| Cost-sensitive at scale (>50K msgs/month) | **Meta Cloud API** — no middleman markup |
| Need WhatsApp Flows, Calling, Marketing API | **Meta Cloud API** — Twilio doesn't support these |
| Multi-provider fallback | **Both** — use Twilio as fallback for Meta |

---

## 4. Strategic Decision: Provider Roadmap

### Phase 1: Now → Next 3 Months (Twilio Only)
- **Status**: Already working
- Keep Twilio as the sole WhatsApp provider
- BYOC already supported per-app
- Focus: Template seed system, delivery status webhooks via Twilio StatusCallback
- Cost: Pass-through Twilio pricing to tenants

### Phase 2: Month 3–6 (Add Meta Cloud API as Primary Provider)
- Build `meta_whatsapp_provider.go` alongside existing Twilio provider
- Register via Meta Graph API directly: `https://graph.facebook.com/v23.0/{PHONE_NUMBER_ID}/messages`
- Support Meta-native template management (create/update/delete via API)
- Implement Meta webhook receiver for delivery statuses
- Use Twilio as **fallback provider** (provider fallback chain: Meta → Twilio)

### Phase 3: Month 6–9 (Advanced Meta Features)
- WhatsApp Flows integration (multi-screen interactive forms)
- Marketing Messages API (optimized delivery, performance benchmarks)
- WhatsApp Business Calling API
- Group messaging via API
- Volume tier tracking + cost analytics per tenant

### Phase 4: Month 9–12 (BYOC Self-Service)
- Users register their own Meta WhatsApp number through FreeRangeNotify UI/API
- Full Embedded Signup flow via Meta SDK
- Template management UI in FreeRangeNotify dashboard
- Billing passthrough (users pay Meta directly for messaging, FreeRangeNotify charges for platform)

---

## 5. BYOC (Bring Your Own Credentials) — User Number Registration

### Current State
- BYOC for Twilio is **already working**: per-app `WhatsAppAppConfig` with `AccountSID`, `AuthToken`, `FromNumber`
- Users provide their own Twilio credentials in app settings
- System uses their creds at send time, tracks as `CredSourceBYOC`

### Can Users Register Their Own Number with Meta via FreeRangeNotify?

**Yes, this is architecturally possible.** Here's how:

#### Option A: Embedded Signup (Recommended)
Meta provides an [Embedded Signup](https://developers.facebook.com/docs/whatsapp/embedded-signup) flow that lets Solution Partners onboard clients directly:

1. FreeRangeNotify registers as a **Meta Solution Partner** (Tech Provider)
2. User clicks "Connect WhatsApp" in FreeRangeNotify dashboard
3. A Meta OAuth popup opens → user logs into their Meta Business account
4. User selects or creates a WhatsApp Business Account (WABA)
5. User registers their phone number
6. Meta returns a `WABA_ID`, `PHONE_NUMBER_ID`, and access token
7. FreeRangeNotify stores these per-app (like existing BYOC)
8. **User pays Meta directly** for messaging costs via Meta Billing Hub
9. FreeRangeNotify charges only the platform subscription fee

#### Option B: Manual Credential Entry
1. User goes to Meta Business Manager → creates WABA → registers phone → gets System User token
2. User enters `WABA_ID`, `PHONE_NUMBER_ID`, `ACCESS_TOKEN` in FreeRangeNotify app settings
3. FreeRangeNotify uses these to send via Graph API
4. **User pays Meta directly**

**Option A is the better UX** and what competitors (Novu, Intercom, Bird) use.

#### Technical Requirements for Embedded Signup
- FreeRangeNotify must be a registered Meta app (App Dashboard)
- Need `whatsapp_business_management` and `whatsapp_business_messaging` permissions
- Must pass Meta's App Review for these permissions
- Must implement Meta's webhook receiver for status callbacks
- Timeline: 2-4 weeks for app review + implementation

#### Revenue Model with BYOC
| Component | Who Pays | Who Gets Paid |
|---|---|---|
| Per-message WhatsApp cost | Tenant → Meta (direct billing) | Meta |
| FreeRangeNotify platform fee | Tenant → FreeRangeNotify | FreeRangeNotify |
| Template management | Free (Meta API) | — |
| Orchestration (workflows, digests, scheduling) | Included in platform fee | FreeRangeNotify |

This is the model used by Novu, OneSignal, and similar platforms. **FreeRangeNotify earns from the platform, not from message markup.**

---

## 6. FreeRangeNotify Value Proposition Over Native WhatsApp

Why would someone use FreeRangeNotify for WhatsApp instead of just using the WhatsApp Business app or calling the Meta API directly?

### Features WhatsApp Doesn't Provide (That We Do)

| Feature | WhatsApp App/Business | Meta API Direct | FreeRangeNotify |
|---|---|---|---|
| **Multi-channel orchestration** | WhatsApp only | WhatsApp only | Email + SMS + Push + WhatsApp + Slack + Discord + Teams + Webhook + SSE + In-App |
| **Workflow automation** | ❌ | Build yourself | ✅ Delay → Condition → Channel → Digest pipelines |
| **Scheduled messages** | ❌ | Build yourself | ✅ Cron + one-time + timezone-aware |
| **Digest/Batching** | ❌ | Build yourself | ✅ Aggregate by key + time window + max batch |
| **Topic Pub/Sub** | ❌ | Build yourself | ✅ Subscribe users → broadcast to topic |
| **Template versioning** | ❌ | ❌ | ✅ Version tracking + locale support |
| **Multi-tenancy** | ❌ | Build yourself | ✅ App isolation, per-app credentials, RBAC |
| **Provider fallback** | ❌ | ❌ | ✅ Meta → Twilio → Vonage chain |
| **Smart delivery** | ❌ | ❌ | ✅ Presence-based routing, instant flush |
| **User preferences** | ❌ | Build yourself | ✅ Quiet hours, DND, per-channel opt-in/out |
| **Rate limiting** | ❌ | ❌ | ✅ Per-app + per-user throttling |
| **Analytics dashboard** | ❌ | Basic | ✅ Delivery rates, channel breakdown, cost tracking |
| **Unified API** | ❌ | WhatsApp-specific | ✅ Single `POST /v1/notifications` for any channel |
| **Audit trail** | ❌ | ❌ | ✅ Every action logged in Elasticsearch |
| **Webhook delivery** | ❌ | Build yourself | ✅ HMAC-signed webhooks with retry |

### The Killer Pitch
> "Send a scheduled WhatsApp marketing blast every Monday at 9am to your 'product-updates' topic subscribers, with automatic fallback to SMS for users who don't have WhatsApp, and email digest for users in DND mode — all from a single API call."

**No one does this with just the WhatsApp app.** You'd need to build FreeRangeNotify from scratch.

---

## 7. Feature Gap Analysis — What to Build Next

### High-Value Features to Differentiate

#### A. Meta Cloud API Provider (Priority: Critical)
- Direct Meta integration eliminates Twilio markup ($0.005-0.008/msg savings)
- Access to WhatsApp Flows, Calling, Marketing Messages API
- **Effort**: 3-4 weeks for core provider + webhook receiver

#### B. WhatsApp Template Management via Dashboard (Priority: High)
- Create, edit, submit for approval, track status — all from FreeRangeNotify UI
- Currently users must go to Twilio Console or WhatsApp Manager separately
- Map FreeRangeNotify templates to Meta HSM template IDs
- **Effort**: 2-3 weeks (UI + API)

#### C. WhatsApp Flows Integration (Priority: Medium)
- Multi-screen interactive forms inside WhatsApp chat
- Use cases: appointment booking, product catalog, survey, order tracking
- This is Meta-exclusive — Twilio doesn't support it
- Can be triggered from FreeRangeNotify workflows
- **Effort**: 3-4 weeks

#### D. Two-Way Conversation Handling (Priority: High)
- Receive inbound WhatsApp messages via Meta webhooks
- Route to appropriate handler (workflow trigger, bot, human agent)
- Customer Service Window tracking (24h window for free non-template messages)
- **Effort**: 2-3 weeks

#### E. Read Receipt Analytics (Priority: Medium)
- Meta provides `read` status via webhooks
- Track read rates per template, per campaign, per user segment
- Compare across channels (email open rate vs WhatsApp read rate)
- **Effort**: 1-2 weeks

#### F. WhatsApp Marketing Message Optimization (Priority: Medium)
- Marketing Messages API gives quality-based delivery optimization
- Automatic creative enhancements by Meta
- Performance benchmarks vs similar businesses
- Only available via Meta direct (not Twilio)
- **Effort**: 2 weeks

#### G. WhatsApp Groups API (Priority: Low)
- Create and manage WhatsApp groups programmatically
- Message all group members via API
- Use case: community management, internal team notifications
- **Effort**: 2 weeks

#### H. WhatsApp Business Calling (Priority: Low)
- VOIP calls via API
- Use case: appointment reminders with call option, OTP via voice
- **Effort**: 3 weeks

---

## 8. Implementation Phases

### Phase 1: Immediate (This Sprint)
- [x] Validate Twilio SMS sending works (tested, confirmed)
- [x] Validate Twilio WhatsApp requires production setup (sandbox is test-only)
- [ ] Configure Twilio WhatsApp Sender for production number
- [ ] Add Twilio StatusCallback webhook endpoint for delivery tracking
- [ ] Add WhatsApp template seed to migration

### Phase 2: Meta Cloud API Provider (Month 1-2)
```
internal/infrastructure/providers/
├── whatsapp_provider.go          ← existing (Twilio-backed)
├── meta_whatsapp_provider.go     ← NEW (Meta Cloud API)
└── manager.go                    ← wire both, fallback chain
```

**New Config**:
```go
type MetaWhatsAppConfig struct {
    PhoneNumberID  string // Meta phone number ID
    WABAID         string // WhatsApp Business Account ID
    AccessToken    string // System User token or OAuth token
    WebhookVerify  string // Webhook verification token
    AppSecret      string // For webhook signature validation
    Timeout        time.Duration
    MaxRetries     int
}
```

**Per-App BYOC**: Extend `WhatsAppAppConfig` to support both:
```go
type WhatsAppAppConfig struct {
    Provider string // "twilio" or "meta" (default: system setting)
    
    // Twilio credentials (existing)
    AccountSID string
    AuthToken  string
    FromNumber string
    
    // Meta credentials (new)
    PhoneNumberID string
    WABAID        string
    AccessToken   string
}
```

### Phase 3: Template Sync (Month 2-3)
- Bidirectional sync between FreeRangeNotify templates and Meta HSM templates
- On template create in FRN → submit to Meta for approval via API
- On Meta approval webhook → update template status in FRN
- Store Meta template ID in `Template.Metadata["meta_template_id"]`

### Phase 4: Embedded Signup BYOC (Month 4-6)
- Register FreeRangeNotify as Meta Solution Partner
- Build OAuth flow for user WABA onboarding
- Store per-app Meta credentials after signup
- Dashboard UI for managing WhatsApp connection

---

## 9. Meta API Pricing Deep Dive (April 2026)

### Pricing Model (Per-Message, since July 2025)
| What's Charged | When |
|---|---|
| Template messages (marketing, utility, auth) | Always, when delivered outside CSW |
| Utility templates within CSW | **Free** |
| Non-template messages within CSW | **Free** |
| All messages within Free Entry Point window (72h) | **Free** |

### India Rates (INR, April 2026)
| Category | List Rate | Volume Tier 1 | Volume Tier 2 |
|---|---|---|---|
| Marketing | ₹0.80–1.05 | Unlocks at 25K msgs/month | Further discount at 100K+ |
| Utility (outside CSW) | ₹0.14 | Similar tier discounts | — |
| Authentication | ₹0.28 | Similar tier discounts | — |

### Key Pricing Facts
1. **Non-template replies are FREE** — within the 24h Customer Service Window
2. **Utility templates inside CSW are FREE** — order confirmations, shipping updates
3. **72h free window** — if user comes from Click-to-WhatsApp Ad or FB CTA
4. **Volume tiers** — more you send in a month, lower the per-message rate
5. **Billing is per-WABA** — FreeRangeNotify can track costs per tenant app via separate WABAs
6. **Authentication-international rates** — higher for cross-border OTPs

### Cost Optimization Strategies for FreeRangeNotify
- **CSW-aware sending**: Track open customer service windows; prefer free utility templates within CSW
- **Template category optimization**: Auto-classify templates to cheapest category that qualifies
- **Volume tier tracking**: Show tenants their tier progress; incentivize consolidation
- **Batching**: Use digest rules to batch multiple updates into single template messages
- **Time zone scheduling**: Send marketing templates at optimal times for engagement (lowers opt-out, maintains quality rating)

---

## 10. Architecture — Dual Provider Design

```
                    ┌──────────────────────────────────────────┐
                    │         POST /v1/notifications           │
                    │         channel: "whatsapp"              │
                    └───────────────┬──────────────────────────┘
                                    │
                    ┌───────────────▼──────────────────────────┐
                    │            Worker                         │
                    │  1. Resolve template                      │
                    │  2. Check user preferences                │
                    │  3. Select provider                       │
                    └───────────────┬──────────────────────────┘
                                    │
                    ┌───────────────▼──────────────────────────┐
                    │        Provider Manager                   │
                    │   WhatsApp fallback chain:                │
                    │   [meta_cloud_api] → [twilio]             │
                    └──────┬───────────────────┬───────────────┘
                           │                   │
              ┌────────────▼────────┐  ┌──────▼─────────────┐
              │  Meta Cloud API     │  │  Twilio WhatsApp   │
              │  Provider           │  │  Provider          │
              │                     │  │                    │
              │  graph.facebook.com │  │  api.twilio.com    │
              │  /v23.0/{PHONE}/    │  │  /Messages.json    │
              │  messages           │  │                    │
              └─────────────────────┘  └────────────────────┘
                           │                   │
              ┌────────────▼────────┐  ┌──────▼─────────────┐
              │  Meta Webhooks      │  │  Twilio Status     │
              │  POST /v1/webhooks/ │  │  Callback          │
              │  meta/whatsapp      │  │                    │
              └─────────────────────┘  └────────────────────┘
```

### Per-App Provider Selection Logic
```
1. Check app.Settings.WhatsApp.Provider
   ├── "meta"   → Use Meta Cloud API with app's Meta credentials
   ├── "twilio" → Use Twilio with app's Twilio credentials  
   └── ""       → Use system default (global config)
2. If selected provider fails → try next in fallback chain
3. Track which provider was used → billing metadata
```

---

## Summary

| Question | Answer |
|---|---|
| **Why pay for API when WhatsApp app is free?** | Scale (256 limit), automation (workflows, schedules, digests), multi-channel, multi-tenancy, analytics, compliance |
| **Twilio or Meta direct?** | Both. Twilio now (working), Meta direct in 2-3 months (cheaper at scale, exclusive features) |
| **Can users register their own number via FreeRangeNotify?** | Yes — via Meta Embedded Signup (requires Solution Partner registration) |
| **Who pays for messages?** | BYOC: tenant pays Meta directly. System: FreeRangeNotify pays, marks up or bundles in subscription |
| **What features to add?** | Meta provider, template management UI, two-way conversations, WhatsApp Flows, read receipt analytics |
| **Timeline to Meta API?** | Core provider: 3-4 weeks. Template sync: 2-3 weeks. Embedded Signup: 4-6 weeks |

---

## 11. Meta Tech Provider — Full Feature Access Map

> **Reference**: [Become a Tech Provider](https://developers.facebook.com/documentation/business-messaging/whatsapp/solution-providers/get-started-for-tech-providers), [Permissions](https://developers.facebook.com/docs/whatsapp/permissions)

### 11.1 What Is a Tech Provider?

A Tech Provider is a registered Meta partner that builds software for other businesses to use WhatsApp Business Platform. As a Tech Provider, FreeRangeNotify can:

- **Onboard business customers** through Embedded Signup (OAuth-style 3-click flow)
- **Send messages on behalf of clients** using their WhatsApp Business Account
- **Receive webhooks** (inbound messages, delivery statuses, template updates) for all client WABAs
- **Manage templates, phone numbers, and business profiles** via API

**Billing model**: Clients pay Meta directly for message costs. FreeRangeNotify charges only the platform subscription. No credit line from Meta (that's Solution Partner tier).

### 11.2 Permissions We Get (After App Review)

| Permission | What It Unlocks |
|------------|-----------------|
| `whatsapp_business_messaging` (Advanced Access) | Send all message types (text, template, interactive, media) to WhatsApp users; receive inbound message webhooks; receive message status webhooks (sent, delivered, read, failed) |
| `whatsapp_business_management` (Advanced Access) | Access WABA metadata; create/update/delete message templates via API; manage business phone numbers; access analytics & insights; receive account-level webhooks (template status, quality, phone number changes) |
| `whatsapp_business_manage_events` (Optional) | Track marketing conversions via Conversions API; required for Marketing Messages API |
| `business_management` (Optional) | Programmatic access to business portfolio; rarely needed |
| `ads_read` (Optional) | Marketing campaign metrics via Insights API |

### 11.3 Complete Feature Access as Tech Provider

#### A. Outbound Messaging (Sending)

| Feature | Available? | API | FRN Integration |
|---------|-----------|-----|-----------------|
| **Text messages** | Yes | `POST /{phone-number-id}/messages` type=text | Already built (`meta_whatsapp_provider.go`) |
| **Template messages** (marketing, utility, auth) | Yes | type=template | Already built (basic) |
| **Image messages** | Yes | type=image | Already built (URL-based) |
| **Video messages** | Yes | type=video | To build |
| **Audio messages** | Yes | type=audio | To build |
| **Document messages** (PDF, etc.) | Yes | type=document | To build |
| **Sticker messages** | Yes | type=sticker | To build |
| **Location messages** | Yes | type=location | To build |
| **Location request** | Yes | type=interactive, action=location_request_message | To build |
| **Contact card messages** | Yes | type=contacts | To build |
| **Reaction messages** (emoji react to a msg) | Yes | type=reaction | To build |
| **Reply buttons** (up to 3 buttons) | Yes | type=interactive, action=buttons | To build |
| **List messages** (menu with sections) | Yes | type=interactive, action=list | To build |
| **CTA URL buttons** | Yes | type=interactive, action=cta_url | To build |
| **Media card carousel** | Yes | Template with carousel components | To build |
| **Address messages** | Yes | type=address_message | To build |
| **Contextual replies** (quote a message) | Yes | context.message_id in payload | To build |
| **Read receipts** (mark as read) | Yes | `PUT /{phone-number-id}/messages` | To build |
| **Typing indicators** | Yes | `POST /{phone-number-id}/messages` type=typing | To build |
| **Link previews** | Yes | preview_url=true in text messages | To build |

#### B. Inbound Messaging (Receiving) — NOT YET IMPLEMENTED

| Feature | Available? | Webhook Field | FRN Integration |
|---------|-----------|---------------|-----------------|
| **Text messages from users** | Yes | `messages[].type=text` | To build |
| **Image/video/audio/document from users** | Yes | `messages[].type=image/video/audio/document` | To build |
| **Location shared by user** | Yes | `messages[].type=location` | To build |
| **Contact shared by user** | Yes | `messages[].type=contacts` | To build |
| **Button reply from user** | Yes | `messages[].type=interactive` | To build |
| **List selection from user** | Yes | `messages[].type=interactive` | To build |
| **Order messages** (from catalog) | Yes | `messages[].type=order` | To build |
| **Message reactions from user** | Yes | `messages[].type=reaction` | To build |
| **Message edits** | Yes | `messages[].type=edit` | To build |
| **Message revokes** (unsend/delete) | Yes | `messages[].type=revoke` | To build |
| **Group messages** | Yes | `messages[].type=*` with group context | To build |

#### C. Delivery Status Tracking — NOT YET IMPLEMENTED

| Status | Available? | Webhook Field | FRN Integration |
|--------|-----------|---------------|-----------------|
| **sent** (message left our server) | Yes | `statuses[].status=sent` | To build |
| **delivered** (reached user's device) | Yes | `statuses[].status=delivered` | To build |
| **read** (user opened/saw it) | Yes | `statuses[].status=read` | To build |
| **failed** (delivery failed) | Yes | `statuses[].status=failed` + error | To build |
| **Pricing info** (billable, category) | Yes | `statuses[].pricing` | To build |
| **Conversation origin** (user-initiated vs business) | Yes | `statuses[].conversation.origin.type` | To build |

#### D. Template Management via API

| Feature | Available? | API | FRN Integration |
|---------|-----------|-----|-----------------|
| **Create template** | Yes | `POST /{waba-id}/message_templates` | To build |
| **List templates** | Yes | `GET /{waba-id}/message_templates` | To build |
| **Get template by name** | Yes | `GET /{waba-id}/message_templates?name=...` | To build |
| **Update template** | Yes | `POST /{template-id}` | To build |
| **Delete template** | Yes | `DELETE /{waba-id}/message_templates?name=...` | To build |
| **Template status webhook** (approved/rejected/paused) | Yes | `message_template_status_update` | To build |
| **Template quality webhook** | Yes | `message_template_quality_update` | To build |
| **Template category update webhook** | Yes | `template_category_update` | To build |
| **Template component update webhook** | Yes | `message_template_components_update` | To build |
| **Template library** (pre-approved templates) | Yes | Library API | To build |
| **Template migration** (across WABAs) | Yes | Migration API | To build |
| **Configure TTL** (time-to-live) | Yes | `POST /{template-id}` with message_send_ttl_seconds | To build |

#### E. Customer Onboarding (Embedded Signup)

| Feature | Available? | Notes |
|---------|-----------|-------|
| **Embedded Signup v4** | Yes | Latest version; OAuth popup; user creates or selects WABA + registers phone |
| **Hosted Embedded Signup** | Yes | Meta hosts the signup UI; simpler integration |
| **Custom flows** | Yes | Customize which screens appear |
| **Pre-verified phone numbers** | Yes | Skip phone verification for known numbers |
| **Bypass phone number screens** | Yes | Useful for enterprise onboarding |
| **App-only install** | Yes | Connect without requiring a website |
| **Partner-initiated WABA creation** | Yes | Create WABAs programmatically on behalf of clients |
| **Onboard up to 200 clients per 7-day window** | Yes | Rolling limit after verification |

#### F. Account & Phone Number Management

| Feature | Available? | API |
|---------|-----------|-----|
| **Register phone numbers** | Yes | Phone Number Registration API |
| **Deregister phone numbers** | Yes | Phone Number Deregister API |
| **Two-step verification** | Yes | Settings API |
| **Business profile management** | Yes | WhatsApp Business Profile API |
| **Display name management** | Yes | Via WABA settings |
| **Official Business Account status** | Yes | OBA Status API |
| **QR code generation** | Yes | QR Code API |
| **Message links** (wa.me links) | Yes | Standard |
| **Block/unblock users** | Yes | Block API |
| **Phone number quality monitoring** | Yes | `phone_number_quality_update` webhook |

#### G. Advanced Features (Meta-Exclusive, Not Available via Twilio)

| Feature | Available? | Notes |
|---------|-----------|-------|
| **WhatsApp Flows** (multi-screen interactive forms) | Yes | Surveys, appointments, order forms — inside WhatsApp chat |
| **WhatsApp Business Calling** (VOIP) | Yes | Business-initiated and user-initiated calls via API; SIP integration |
| **Groups API** (create/manage groups programmatically) | Yes | Create groups, add participants, send group messages, manage invite links |
| **Marketing Messages API** (optimized delivery) | Yes | Quality-based delivery optimization, auto-creative enhancements, benchmarks |
| **Product Catalogs** | Yes | Upload inventory, share products, carousel messages, multi-product messages |
| **Payments** (India UPI, Brazil Pix/Boleto) | Yes | Order details templates, checkout buttons, payment links |
| **Conversational Automation API** | Yes | Automated conversation management |
| **WhatsApp Business Bot API** | Yes | Bot configuration and management |
| **Message History Events API** | Yes | Retrieve historical message events |
| **Insights / Analytics API** | Yes | Conversation analytics, template analytics, cost analytics |
| **Ads that Click to WhatsApp** | Yes | Welcome message sequences for CTWA ads |
| **Multi-Partner Solutions** | Yes | Partner with Solution Partners; share WABAs across solutions |
| **Pixel tracking** | Yes | Track conversions from WhatsApp to web |

#### H. Webhooks We Can Receive

| Webhook | Description |
|---------|-------------|
| `messages` | Inbound messages from users (text, media, interactive, location, contacts, orders, reactions, edits, revokes) |
| `messages` (statuses) | Outbound message statuses (sent, delivered, read, failed) with pricing info |
| `message_template_status_update` | Template approved, rejected, paused, or flagged |
| `message_template_quality_update` | Template quality rating changed |
| `message_template_components_update` | Template component modified by Meta |
| `template_category_update` | Template re-categorized by Meta |
| `account_alerts` | Account-level alerts |
| `account_review_update` | Business verification / account review status changes |
| `account_update` | WABA settings changed |
| `business_capability_update` | Messaging limits or features changed |
| `phone_number_name_update` | Display name approved/rejected |
| `phone_number_quality_update` | Phone number quality tier changed |
| `security` | Security events (e.g., two-step verification changes) |
| `partner_solutions` | Partner-specific events |
| `payment_configuration_update` | Payment settings changed |

### 11.4 Tech Provider vs Solution Partner vs Direct Developer

| Capability | Direct Developer | Tech Provider | Solution Partner |
|------------|-----------------|---------------|-----------------|
| Send messages for own business | Yes | Yes | Yes |
| Send messages for client businesses | No | **Yes** | **Yes** |
| Onboard clients via Embedded Signup | No | **Yes** | **Yes** |
| Manage client WABAs | No | **Yes** | **Yes** |
| Credit line from Meta | No | No | **Yes** |
| Bill clients for message costs | No | No (clients pay Meta) | **Yes** |
| Accelerator program access | No | No | **Yes** |
| App Review required | No | **Yes** | **Yes** |
| Business verification required | No | **Yes** | **Yes** |
| Max client onboarding | N/A | 200 per 7-day window | Higher |
| Token type | System User | Business Token | System Token |
| Upgrade path | → Tech Provider | → Tech Partner | — |

**Our choice: Tech Provider** — aligns with FreeRangeNotify's model: we charge for the platform, clients pay Meta for messages. No credit line management overhead.

### 11.5 What We Cannot Do Without Solution Partner Tier

- Cannot bill clients for Meta message costs (clients must add their own payment method via Meta)
- No credit line from Meta (cannot subsidize or pre-pay for clients)
- No accelerator program perks (dedicated support, early feature access)
- Limited to 200 client onboardings per 7-day rolling window

These are acceptable trade-offs. If volume demands it later, we can upgrade to Tech Partner (verified Tech Provider with accelerator access).

---

## 12. Two-Way Messaging — Inbound Message Architecture

### 12.1 The Missing Piece

Today FreeRangeNotify is **send-only** for WhatsApp. A user receives a message but if they reply, that reply goes into a void — we don't receive it, store it, or act on it. This is the single biggest gap vs competitors like Bird and Intercom.

### 12.2 How Meta Delivers Inbound Messages

When a WhatsApp user messages a business phone number, Meta sends a webhook POST to the registered callback URL:

```json
{
  "object": "whatsapp_business_account",
  "entry": [{
    "id": "WABA_ID",
    "changes": [{
      "value": {
        "messaging_product": "whatsapp",
        "metadata": {
          "display_phone_number": "15550783881",
          "phone_number_id": "106540352242922"
        },
        "contacts": [{
          "profile": { "name": "Customer Name" },
          "wa_id": "16505551234"
        }],
        "messages": [{
          "from": "16505551234",
          "id": "wamid.HBgLMTY1MD...",
          "timestamp": "1749416383",
          "type": "text",
          "text": { "body": "Does it come in another color?" }
        }]
      },
      "field": "messages"
    }]
  }]
}
```

### 12.3 New Endpoints

| Method | Route | Purpose |
|--------|-------|---------|
| GET | `/v1/webhooks/meta/whatsapp` | Meta webhook verification (hub.challenge) |
| POST | `/v1/webhooks/meta/whatsapp` | Receive inbound messages + delivery statuses |
| GET | `/v1/whatsapp/conversations` | List conversations for an app (admin dashboard) |
| GET | `/v1/whatsapp/conversations/:contact_id` | Get messages for a contact |
| POST | `/v1/whatsapp/conversations/:contact_id/reply` | Send a reply within CSW |
| GET | `/v1/whatsapp/templates` | List Meta templates with approval status |
| POST | `/v1/whatsapp/templates` | Create & submit template to Meta |
| DELETE | `/v1/whatsapp/templates/:name` | Delete template from Meta |

### 12.4 Inbound Message Routing

When we receive an inbound message, it flows through a routing pipeline:

```
Meta Webhook POST
      │
      ▼
┌─────────────────────────┐
│ 1. Verify X-Hub-Signature│
│    (HMAC-SHA256)         │
└──────────┬──────────────┘
           │
      ▼
┌─────────────────────────┐
│ 2. Resolve WABA → App   │
│    (phone_number_id →   │
│     FRN app lookup)     │
└──────────┬──────────────┘
           │
      ▼
┌─────────────────────────┐
│ 3. Resolve sender →     │
│    FRN User             │
│    (wa_id → user by     │
│     phone, or create)   │
└──────────┬──────────────┘
           │
      ▼
┌─────────────────────────┐
│ 4. Store in             │
│    whatsapp_messages    │
│    ES index             │
└──────────┬──────────────┘
           │
      ▼
┌─────────────────────────┐
│ 5. Open/extend CSW      │
│    (24h free reply      │
│     window)             │
└──────────┬──────────────┘
           │
      ▼
┌─────────────────────────┐
│ 6. Route based on app   │
│    config:              │
│                         │
│  ┌─ auto_reply:         │
│  │  Send configured     │
│  │  template/text back  │
│  │                      │
│  ├─ workflow_trigger:   │
│  │  Fire a workflow     │
│  │  (event: wa_inbound) │
│  │                      │
│  ├─ webhook_forward:    │
│  │  POST to business's  │
│  │  own endpoint        │
│  │                      │
│  └─ inbox:              │
│     SSE push to admin   │
│     dashboard for       │
│     agent reply         │
└─────────────────────────┘
```

### 12.5 Customer Service Window (CSW) Tracking

Meta allows free non-template replies within 24 hours of the last user message. This is critical for cost optimization.

```
Per app+contact, track in Redis:
  key:   csw:{app_id}:{wa_id}
  value: timestamp of last inbound message
  TTL:   24 hours

Before sending a reply:
  if CSW is open → send free-form text (no template needed, no charge)
  if CSW is closed → must use approved template (charged)
```

This saves businesses significant money and FreeRangeNotify can surface this in the UI: "Reply now for free — window closes in 3h 22m".

---

## 13. What We Build That Others Don't

### 13.1 Competitive Matrix — WhatsApp Features

| Capability | Novu | OneSignal | Bird | Intercom | **FreeRangeNotify** |
|-----------|------|-----------|------|----------|---------------------|
| WhatsApp send (text) | Via Twilio | Via Twilio | Native Meta | Native Meta | **Native Meta + Twilio fallback** |
| WhatsApp send (template) | Manual | Manual | Yes | Yes | **Yes + template manager UI** |
| WhatsApp send (interactive: buttons/lists) | No | No | Yes | Partial | **Yes (all types)** |
| WhatsApp send (media: image/video/doc/audio) | Partial | Partial | Yes | Yes | **Yes (all types)** |
| WhatsApp send (location, contacts, reactions) | No | No | Yes | No | **Yes** |
| WhatsApp receive (inbound messages) | No | No | Yes (paid) | Yes (paid) | **Yes (included)** |
| Conversation inbox (2-way chat) | No | No | Yes (paid) | Yes (core) | **Yes (included)** |
| Delivery status tracking (sent/delivered/read) | No | Basic | Yes | Yes | **Yes + analytics** |
| CSW tracking & cost optimization | No | No | No | No | **Yes (unique)** |
| Template management from dashboard | No | No | Yes | Yes | **Yes** |
| Template approval status tracking | No | No | Partial | Yes | **Yes + webhooks** |
| Embedded Signup (1-click connect) | No | No | Yes | Yes | **Yes** |
| WhatsApp Flows (multi-screen forms) | No | No | No | No | **Yes (unique in OSS)** |
| WhatsApp Calling (VOIP) | No | No | No | No | **To build (unique in OSS)** |
| Groups API | No | No | No | No | **To build (unique)** |
| Product catalogs / commerce | No | No | No | No | **To build** |
| Payments (India UPI / Brazil Pix) | No | No | No | No | **To build (unique)** |
| Multi-channel fallback (WA → SMS → Email) | Partial | Yes | Yes | Partial | **Yes + presence-aware smart routing** |
| Workflow automation + WhatsApp | Limited | No | Limited | Workflows | **Full (delay/condition/digest/topic)** |
| Self-hostable | Yes | No | No | No | **Yes** |
| Open source | Yes | No | No | No | **Yes** |

### 13.2 Features That Win Deals

**For Non-Technical Users (the "I don't understand APIs" crowd):**
1. **Connect WhatsApp in 3 clicks** — Embedded Signup, no API keys, no Meta Business Manager
2. **Visual template builder** — drag-and-drop header/body/footer/buttons, submit for approval, track status
3. **Conversation inbox** — WhatsApp chat view right in the dashboard, reply to customers
4. **Campaign builder** — "Send this template to these users on Monday at 9am" — no code
5. **CSW cost indicator** — "Reply now for free" badge on open conversations

**For Developers / Technical Teams:**
1. **Single API for everything** — `POST /v1/notifications` with `channel: "whatsapp"` covers text, template, interactive, media — unified across all 10 channels
2. **Workflow engine** — trigger WhatsApp from events, add delays, conditions, digest batching, fallback chains
3. **Inbound webhook routing** — receive user replies, trigger workflows, forward to your own endpoint
4. **Go SDK + JS SDK** — `client.Notifications.Send()` with WhatsApp, no Meta API knowledge needed
5. **Self-hostable** — run the whole thing on your own infra, no vendor lock-in

**For Businesses That Want to Win on WhatsApp:**
1. **WhatsApp Flows** — appointment booking, surveys, order tracking — multi-screen forms inside chat
2. **Smart cost optimization** — CSW tracking, template category advisor, volume tier dashboard
3. **Read receipt analytics** — compare WhatsApp read rates vs email open rates vs push tap rates
4. **Provider fallback** — if Meta is down, auto-fallback to Twilio; if WhatsApp fails, fall to SMS
5. **All of Meta's advanced features** — calling, groups, catalogs, payments — exposed as simple FRN APIs
