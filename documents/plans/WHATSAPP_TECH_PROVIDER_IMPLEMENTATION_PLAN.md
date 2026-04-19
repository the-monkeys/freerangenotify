# WhatsApp Tech Provider — Detailed Implementation Plan

> **Parent:** [WHATSAPP_INTEGRATION_STRATEGY.md](../WHATSAPP_INTEGRATION_STRATEGY.md)  
> **Date:** March 12, 2026  
> **Status:** Planning  

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Phase 0: Fix Meta Provider Wiring (Prerequisite)](#2-phase-0-fix-meta-provider-wiring)
3. [Phase 1: Meta Webhook Receiver (Inbound + Status)](#3-phase-1-meta-webhook-receiver)
4. [Phase 2: Embedded Signup (Client Onboarding)](#4-phase-2-embedded-signup)
5. [Phase 3: WhatsApp Template Management API](#5-phase-3-whatsapp-template-management)
6. [Phase 4: Conversation Inbox](#6-phase-4-conversation-inbox)
7. [Phase 5: Interactive Messages](#7-phase-5-interactive-messages)
8. [Backward Compatibility](#8-backward-compatibility)
9. [Environment Variables Reference](#9-environment-variables-reference)
10. [Docker Compose Changes](#10-docker-compose-changes)

---

## 1. Executive Summary

Transform FreeRangeNotify from a WhatsApp send-only platform (Twilio-backed) into a full **Meta Tech Provider** with two-way messaging, Embedded Signup, template management, and a conversation inbox. All changes are feature-gated and backward-compatible.

**Feature flag:** `features.whatsapp_meta_enabled: true`  
**Default:** `false` — existing Twilio-only behavior is completely untouched.

---

## 2. Phase 0: Fix Meta Provider Wiring

> **Effort:** 2-3 days | **Risk:** Low | **Breaking changes:** None

### 2.1 Problem

The `meta_whatsapp_provider.go` code exists but is never instantiated because:

1. `config.go` has no `MetaWhatsAppProviderConfig` struct
2. `cmd/worker/main.go` never adds `meta_whatsapp` to `providerConfigs`
3. `processor.go` only injects Twilio BYOC creds into context — Meta-only apps are skipped

### 2.2 Code Changes

#### 2.2.1 `internal/config/config.go` — Add Meta WhatsApp config struct

```go
// ADD after WhatsAppProviderConfig

// MetaWhatsAppProviderConfig contains Meta Cloud API WhatsApp provider configuration.
type MetaWhatsAppProviderConfig struct {
    Enabled       bool   `mapstructure:"enabled"`
    PhoneNumberID string `mapstructure:"phone_number_id"`
    WABAID        string `mapstructure:"waba_id"`
    AccessToken   string `mapstructure:"access_token"`
    APIVersion    string `mapstructure:"api_version"`
    AppSecret     string `mapstructure:"app_secret"`     // For webhook signature verification
    WebhookVerify string `mapstructure:"webhook_verify"`  // Webhook verification token
    Timeout       int    `mapstructure:"timeout"`
    MaxRetries    int    `mapstructure:"max_retries"`
}
```

```go
// MODIFY ProvidersConfig — add field
type ProvidersConfig struct {
    // ... existing fields ...
    WhatsApp     WhatsAppProviderConfig     `mapstructure:"whatsapp"`
    MetaWhatsApp MetaWhatsAppProviderConfig `mapstructure:"meta_whatsapp"` // NEW
    // ... rest ...
}
```

#### 2.2.2 `internal/config/config.go` — Add defaults in `Load()`

```go
viper.SetDefault("providers.meta_whatsapp.enabled", false)
viper.SetDefault("providers.meta_whatsapp.api_version", "v23.0")
viper.SetDefault("providers.meta_whatsapp.timeout", 15)
viper.SetDefault("providers.meta_whatsapp.max_retries", 3)
```

#### 2.2.3 `internal/config/config.go` — Add feature flag

```go
// MODIFY FeaturesConfig
type FeaturesConfig struct {
    // ... existing ...
    WhatsAppMetaEnabled bool `mapstructure:"whatsapp_meta_enabled" yaml:"whatsapp_meta_enabled"` // NEW
}
```

```go
viper.SetDefault("features.whatsapp_meta_enabled", false)
```

#### 2.2.4 `cmd/worker/main.go` — Add `meta_whatsapp` to providerConfigs

```go
// ADD after the "whatsapp" block (~line 116)
"meta_whatsapp": {
    "enabled":         cfg.Providers.MetaWhatsApp.Enabled,
    "phone_number_id": cfg.Providers.MetaWhatsApp.PhoneNumberID,
    "waba_id":         cfg.Providers.MetaWhatsApp.WABAID,
    "access_token":    cfg.Providers.MetaWhatsApp.AccessToken,
    "api_version":     cfg.Providers.MetaWhatsApp.APIVersion,
    "timeout":         float64(cfg.Providers.MetaWhatsApp.Timeout),
    "max_retries":     float64(cfg.Providers.MetaWhatsApp.MaxRetries),
},
```

#### 2.2.5 `cmd/worker/processor.go` — Fix WhatsApp context injection for Meta

Current code (~line 695-724) only checks for Twilio creds (`AccountSID`/`AuthToken`). Must also handle Meta-only apps:

```go
// REPLACE the WhatsApp block in sendNotification():
if notif.Channel == notification.ChannelWhatsApp {
    app, err := p.appRepo.GetByID(ctx, notif.AppID)
    isByoc := false
    if err == nil && app != nil && app.Settings.WhatsApp != nil {
        waCfg := app.Settings.WhatsApp
        // Twilio BYOC
        if waCfg.AccountSID != "" && waCfg.AuthToken != "" {
            ctx = context.WithValue(ctx, providers.WhatsAppConfigKey, waCfg)
            isByoc = true
        }
        // Meta BYOC
        if waCfg.Provider == "meta" && waCfg.MetaPhoneNumberID != "" && waCfg.MetaAccessToken != "" {
            ctx = context.WithValue(ctx, providers.WhatsAppConfigKey, waCfg)
            isByoc = true
        }
    }

    if !isByoc && app != nil {
        adminUser, err := p.authService.GetCurrentUser(ctx, app.AdminUserID)
        if err != nil {
            p.logger.Error("Failed to fetch admin user for WhatsApp phone verification check",
                zap.String("admin_id", app.AdminUserID), zap.Error(err))
            return fmt.Errorf("phone verification check failed")
        }
        if !adminUser.PhoneVerified {
            p.logger.Warn("Blocked system WhatsApp send due to unverified phone",
                zap.String("app_id", app.AppID),
                zap.String("admin_id", app.AdminUserID))
            return fmt.Errorf("phone_verification_required")
        }
    }
}
```

#### 2.2.6 `internal/infrastructure/providers/registry.go` — Fix provider ordering

Currently, both `whatsapp` and `meta_whatsapp` register for `ChannelWhatsApp`. The first one registered wins as default. We need deterministic ordering:

```go
// In Manager.RegisterProvider or InstantiateAll:
// If both whatsapp (Twilio) and meta_whatsapp are active, meta_whatsapp should be
// the primary for ChannelWhatsApp. Twilio becomes fallback.
// This is handled by the app-level ProviderFallback config, but the default
// registration order should prefer Meta when enabled.
```

### 2.3 Config Changes

#### `config/config.yaml` — already has `meta_whatsapp` section (no change needed)

#### `.env` additions

```bash
# Meta WhatsApp Cloud API (global/system credentials — optional when using BYOC)
FREERANGE_PROVIDERS_META_WHATSAPP_ENABLED=false
FREERANGE_PROVIDERS_META_WHATSAPP_PHONE_NUMBER_ID=
FREERANGE_PROVIDERS_META_WHATSAPP_WABA_ID=
FREERANGE_PROVIDERS_META_WHATSAPP_ACCESS_TOKEN=
FREERANGE_PROVIDERS_META_WHATSAPP_API_VERSION=v23.0
FREERANGE_PROVIDERS_META_WHATSAPP_APP_SECRET=
FREERANGE_PROVIDERS_META_WHATSAPP_WEBHOOK_VERIFY=

# Feature flag
FREERANGE_FEATURES_WHATSAPP_META_ENABLED=false
```

### 2.4 Backward Compatibility

- Feature flag defaults to `false` — zero behavior change for existing users
- Twilio provider continues to work identically
- No database migrations needed
- No API changes

---

## 3. Phase 1: Meta Webhook Receiver

> **Effort:** 1-2 weeks | **Risk:** Medium | **Breaking changes:** None (new endpoints only)

### 3.1 High-Level Design

```
Meta Cloud API
      │
      │ POST (inbound msg / status update / template update)
      ▼
┌──────────────────────────────────────────────────┐
│  GET/POST /v1/webhooks/meta/whatsapp             │
│  (Public — no API key, signature-verified)        │
│                                                    │
│  1. GET  → hub.challenge verification              │
│  2. POST → X-Hub-Signature-256 validation          │
│  3. Parse entry[].changes[].value                  │
│  4. Route by payload type:                         │
│     ├── messages[]    → inbound handler            │
│     ├── statuses[]    → delivery status handler    │
│     └── other         → account/template webhooks  │
└──────────────────────────────────────────────────┘
           │                    │                │
           ▼                    ▼                ▼
┌─────────────────┐  ┌─────────────────┐  ┌──────────────┐
│ Inbound Message │  │ Status Update   │  │ Template     │
│ Handler          │  │ Handler         │  │ Status       │
│                  │  │                 │  │ Handler      │
│ • Resolve WABA   │  │ • Update notif  │  │              │
│   → FRN app     │  │   status in ES  │  │ • Update     │
│ • Resolve sender │  │ • Track read    │  │   template   │
│   → FRN user    │  │   receipts      │  │   approval   │
│ • Store message  │  │ • SSE push      │  │   status     │
│ • Open CSW       │  │ • Analytics     │  │              │
│ • Route action   │  │                 │  │              │
└─────────────────┘  └─────────────────┘  └──────────────┘
```

### 3.2 New Files

| File | Purpose |
|------|---------|
| `internal/domain/whatsapp/models.go` | Domain models: `InboundMessage`, `Conversation`, `CSWindow`, `WebhookPayload` |
| `internal/domain/whatsapp/repository.go` | Repository interface for `whatsapp_messages` ES index |
| `internal/domain/whatsapp/service.go` | Service interface: `HandleInbound`, `HandleStatus`, `GetConversations`, `Reply` |
| `internal/infrastructure/repository/whatsapp_repository.go` | ES implementation |
| `internal/infrastructure/database/index_templates.go` | Add `GetWhatsAppMessagesTemplate()` |
| `internal/usecases/services/whatsapp_service_impl.go` | Service implementation |
| `internal/interfaces/http/handlers/meta_webhook_handler.go` | HTTP handler for Meta webhooks |
| `internal/interfaces/http/dto/whatsapp_dto.go` | DTOs for webhook payloads, conversation list, reply |

### 3.3 Domain Models

```go
// internal/domain/whatsapp/models.go
package whatsapp

import "time"

type MessageDirection string
const (
    DirectionInbound  MessageDirection = "inbound"
    DirectionOutbound MessageDirection = "outbound"
)

type InboundMessage struct {
    ID            string           `json:"id" es:"id"`
    AppID         string           `json:"app_id" es:"app_id"`
    WABAID        string           `json:"waba_id" es:"waba_id"`
    PhoneNumberID string           `json:"phone_number_id" es:"phone_number_id"`
    ContactWAID   string           `json:"contact_wa_id" es:"contact_wa_id"`
    ContactName   string           `json:"contact_name" es:"contact_name"`
    UserID        string           `json:"user_id,omitempty" es:"user_id"`
    Direction     MessageDirection `json:"direction" es:"direction"`
    MessageType   string           `json:"message_type" es:"message_type"` // text, image, video, audio, document, location, contacts, interactive, reaction, order
    MetaMessageID string           `json:"meta_message_id" es:"meta_message_id"`
    Timestamp     time.Time        `json:"timestamp" es:"timestamp"`
    
    // Content (varies by type)
    TextBody      string                 `json:"text_body,omitempty" es:"text_body"`
    MediaURL      string                 `json:"media_url,omitempty" es:"media_url"`
    MediaMimeType string                 `json:"media_mime_type,omitempty" es:"media_mime_type"`
    Latitude      float64                `json:"latitude,omitempty" es:"latitude"`
    Longitude     float64                `json:"longitude,omitempty" es:"longitude"`
    RawPayload    map[string]interface{} `json:"raw_payload,omitempty" es:"raw_payload"`
    
    // Context
    ContextMessageID string `json:"context_message_id,omitempty" es:"context_message_id"` // Reply-to
    IsForwarded      bool   `json:"is_forwarded,omitempty" es:"is_forwarded"`
    
    CreatedAt time.Time `json:"created_at" es:"created_at"`
}

type DeliveryStatus struct {
    MetaMessageID    string    `json:"meta_message_id"`
    NotificationID   string    `json:"notification_id,omitempty"` // Resolved from provider_message_id
    Status           string    `json:"status"` // sent, delivered, read, failed
    Timestamp        time.Time `json:"timestamp"`
    RecipientID      string    `json:"recipient_id"`
    ConversationID   string    `json:"conversation_id,omitempty"`
    ConversationOrigin string  `json:"conversation_origin,omitempty"` // user_initiated, business_initiated, referral_conversion
    Billable         bool      `json:"billable,omitempty"`
    PricingCategory  string    `json:"pricing_category,omitempty"` // service, authentication, marketing, utility
    ErrorCode        int       `json:"error_code,omitempty"`
    ErrorMessage     string    `json:"error_message,omitempty"`
}

type InboundRouteAction string
const (
    RouteAutoReply      InboundRouteAction = "auto_reply"
    RouteWorkflow       InboundRouteAction = "workflow_trigger"
    RouteWebhookForward InboundRouteAction = "webhook_forward"
    RouteInbox          InboundRouteAction = "inbox"
)

// WhatsAppInboundConfig per-app configuration for inbound message handling.
// Stored in application.Settings.
type InboundConfig struct {
    Enabled        bool               `json:"enabled" es:"enabled"`
    RouteAction    InboundRouteAction `json:"route_action" es:"route_action"`
    AutoReplyText  string             `json:"auto_reply_text,omitempty" es:"auto_reply_text"`
    AutoReplyTemplateID string        `json:"auto_reply_template_id,omitempty" es:"auto_reply_template_id"`
    WorkflowTriggerID   string        `json:"workflow_trigger_id,omitempty" es:"workflow_trigger_id"`
    WebhookForwardURL   string        `json:"webhook_forward_url,omitempty" es:"webhook_forward_url"`
}
```

### 3.4 Elasticsearch Index

```go
// ADD to internal/infrastructure/database/index_templates.go
func (it *IndexTemplates) GetWhatsAppMessagesTemplate() map[string]interface{} {
    return map[string]interface{}{
        "mappings": map[string]interface{}{
            "properties": map[string]interface{}{
                "id":               map[string]interface{}{"type": "keyword"},
                "app_id":           map[string]interface{}{"type": "keyword"},
                "waba_id":          map[string]interface{}{"type": "keyword"},
                "phone_number_id":  map[string]interface{}{"type": "keyword"},
                "contact_wa_id":    map[string]interface{}{"type": "keyword"},
                "contact_name":     map[string]interface{}{"type": "text"},
                "user_id":          map[string]interface{}{"type": "keyword"},
                "direction":        map[string]interface{}{"type": "keyword"},
                "message_type":     map[string]interface{}{"type": "keyword"},
                "meta_message_id":  map[string]interface{}{"type": "keyword"},
                "timestamp":        map[string]interface{}{"type": "date"},
                "text_body":        map[string]interface{}{"type": "text"},
                "media_url":        map[string]interface{}{"type": "keyword"},
                "created_at":       map[string]interface{}{"type": "date"},
            },
        },
    }
}
```

Register in `database_manager.go`:

```go
"whatsapp_messages": im.templates.GetWhatsAppMessagesTemplate,
```

### 3.5 Webhook Handler

```go
// internal/interfaces/http/handlers/meta_webhook_handler.go

type MetaWebhookHandler struct {
    whatsappService whatsapp.Service
    appRepo         application.Repository
    logger          *zap.Logger
    appSecret       string // Global app secret for signature verification
}

// VerifyWebhook handles GET /v1/webhooks/meta/whatsapp (hub.challenge)
func (h *MetaWebhookHandler) VerifyWebhook(c *fiber.Ctx) error {
    mode := c.Query("hub.mode")
    token := c.Query("hub.verify_token")
    challenge := c.Query("hub.challenge")
    
    if mode == "subscribe" && token == h.verifyToken {
        return c.SendString(challenge)
    }
    return c.SendStatus(fiber.StatusForbidden)
}

// HandleWebhook handles POST /v1/webhooks/meta/whatsapp
func (h *MetaWebhookHandler) HandleWebhook(c *fiber.Ctx) error {
    // 1. Verify X-Hub-Signature-256
    // 2. Parse payload
    // 3. Route to inbound / status / template handler
    // 4. Always return 200 OK (Meta requirement)
}
```

### 3.6 Route Registration

```go
// internal/interfaces/http/routes/routes.go — in setupPublicRoutes()
// ADD (public — no auth; signature-verified inside handler)

if c.MetaWebhookHandler != nil {
    metaWH := v1.Group("/webhooks/meta")
    metaWH.Get("/whatsapp", c.MetaWebhookHandler.VerifyWebhook)
    metaWH.Post("/whatsapp", c.MetaWebhookHandler.HandleWebhook)
}
```

### 3.7 Container Wiring

```go
// internal/container/container.go

// ADD to Container struct
MetaWebhookHandler *handlers.MetaWebhookHandler
WhatsAppService    whatsapp.Service

// ADD in NewContainer(), gated by feature flag
if cfg.Features.WhatsAppMetaEnabled {
    waRepo := repository.NewWhatsAppRepository(dbManager.Client.GetClient(), logger)
    container.WhatsAppService = services.NewWhatsAppService(
        waRepo,
        repos.Application,
        repos.User,
        repos.Notification,
        container.SSEBroadcaster,
        container.WorkflowService, // may be nil
        redisClient,
        logger,
    )
    container.MetaWebhookHandler = handlers.NewMetaWebhookHandler(
        container.WhatsAppService,
        repos.Application,
        cfg.Providers.MetaWhatsApp.AppSecret,
        cfg.Providers.MetaWhatsApp.WebhookVerify,
        logger,
    )
    logger.Info("WhatsApp Meta integration enabled (webhook receiver, inbound messaging)")
}
```

### 3.8 CSW Tracking (Redis)

```go
// internal/usecases/services/whatsapp_service_impl.go

// On every inbound message:
func (s *whatsappService) trackCSW(ctx context.Context, appID, contactWAID string) {
    key := fmt.Sprintf("csw:%s:%s", appID, contactWAID)
    s.redis.Set(ctx, key, time.Now().Unix(), 24*time.Hour)
}

// Before sending a reply:
func (s *whatsappService) isCSWOpen(ctx context.Context, appID, contactWAID string) bool {
    key := fmt.Sprintf("csw:%s:%s", appID, contactWAID)
    _, err := s.redis.Get(ctx, key).Result()
    return err == nil
}
```

### 3.9 Delivery Status → Notification Update

When a `statuses` webhook arrives:

1. Extract `meta_message_id` from `statuses[].id`
2. Look up notification by `provider_message_id` in ES
3. Update notification `status` field: `sent` → `delivered` → `read` (only forward transitions)
4. Store read timestamp for analytics
5. Push SSE event to dashboard: `{ event: "delivery_status", notification_id, status }`

### 3.10 Application Model Extension

```go
// MODIFY internal/domain/application/models.go — Settings struct
type Settings struct {
    // ... existing fields ...
    WhatsAppInbound *whatsapp.InboundConfig `json:"whatsapp_inbound,omitempty" es:"whatsapp_inbound"` // NEW
}
```

### 3.11 Backward Compatibility

- New public routes (`/v1/webhooks/meta/whatsapp`) — no conflict with existing routes
- Handler is `nil` unless feature flag is on — routes not registered
- No changes to existing notification DTO or send flow
- Existing `whatsapp_provider.go` (Twilio) untouched
- ES index `whatsapp_messages` is new — no migration needed

---

## 4. Phase 2: Embedded Signup

> **Effort:** 1-2 weeks | **Risk:** Medium | **Depends on:** Meta App Review (2-4 weeks external)

### 4.1 High-Level Design

```
FRN Dashboard UI                FRN Backend                  Meta
┌──────────────┐          ┌─────────────────────┐      ┌──────────────┐
│ "Connect     │ ──click──▶ GET /admin/whatsapp/ │      │              │
│  WhatsApp"   │          │ embedded-signup-url  │      │              │
│  button      │          └──────────┬──────────┘      │              │
│              │                     │                  │              │
│  ┌───────────▼───────────┐        │                  │              │
│  │ Meta OAuth Popup      │ ◀──────┘ (redirect)       │              │
│  │ (Embedded Signup v4)  │ ─── OAuth flow ──────────▶│ Meta Login   │
│  │ • Login to Meta       │ ◀─── WABA + phone ───────│ Create WABA  │
│  │ • Select/Create WABA  │                           │ Register #   │
│  │ • Register phone      │                           │ Grant perms  │
│  └───────────┬───────────┘                           └──────────────┘
│              │ (callback with code)
│  ┌───────────▼───────────┐
│  │ FRN Backend           │
│  │ POST /admin/whatsapp/ │
│  │ connect               │
│  │                       │
│  │ • Exchange code →     │
│  │   access_token        │
│  │ • Store WABA ID,      │
│  │   phone_number_id,    │
│  │   token per-app       │
│  │ • Subscribe webhooks  │
│  └───────────────────────┘
```

### 4.2 New API Endpoints (JWT-protected admin routes)

| Method | Route | Purpose |
|--------|-------|---------|
| GET | `/v1/admin/whatsapp/status` | Get WhatsApp connection status for current app |
| POST | `/v1/admin/whatsapp/connect` | Complete Embedded Signup (exchange code, store creds) |
| POST | `/v1/admin/whatsapp/disconnect` | Remove WhatsApp connection |
| POST | `/v1/admin/whatsapp/subscribe-webhooks` | Subscribe app's WABA to our webhook URL |

### 4.3 WhatsAppAppConfig Extension

```go
// MODIFY internal/domain/application/models.go
type WhatsAppAppConfig struct {
    Provider string `json:"provider,omitempty" es:"provider"`

    // Twilio fields (existing)
    AccountSID string `json:"account_sid,omitempty" es:"account_sid"`
    AuthToken  string `json:"auth_token,omitempty" es:"auth_token"`
    FromNumber string `json:"from_number,omitempty" es:"from_number"`

    // Meta Cloud API fields (existing)
    MetaPhoneNumberID string `json:"meta_phone_number_id,omitempty" es:"meta_phone_number_id"`
    MetaWABAID        string `json:"meta_waba_id,omitempty" es:"meta_waba_id"`
    MetaAccessToken   string `json:"meta_access_token,omitempty" es:"meta_access_token"`

    // NEW: Embedded Signup metadata
    MetaBusinessID    string `json:"meta_business_id,omitempty" es:"meta_business_id"`
    MetaAppID         string `json:"meta_app_id,omitempty" es:"meta_app_id"`       // Our FRN Meta App ID
    ConnectionStatus  string `json:"connection_status,omitempty" es:"connection_status"` // connected, disconnected, pending
    ConnectedAt       string `json:"connected_at,omitempty" es:"connected_at"`
    DisplayPhoneNumber string `json:"display_phone_number,omitempty" es:"display_phone_number"`
    QualityRating     string `json:"quality_rating,omitempty" es:"quality_rating"`
}
```

### 4.4 Backward Compatibility

- New admin routes only — no effect on existing routes
- `WhatsAppAppConfig` additions are all `omitempty` — existing data untouched
- Manual credential entry still works (Option B from strategy doc)

---

## 5. Phase 3: WhatsApp Template Management

> **Effort:** 2 weeks | **Risk:** Low | **Depends on:** Phase 0 (Meta creds working)

### 5.1 New API Endpoints (API-key protected)

| Method | Route | Purpose |
|--------|-------|---------|
| POST | `/v1/whatsapp/templates` | Create template and submit to Meta for approval |
| GET | `/v1/whatsapp/templates` | List templates with Meta approval status |
| GET | `/v1/whatsapp/templates/:name` | Get template details + status |
| DELETE | `/v1/whatsapp/templates/:name` | Delete template from Meta |
| POST | `/v1/whatsapp/templates/:name/sync` | Force sync status from Meta |

### 5.2 Template Sync Service

```go
// internal/usecases/services/whatsapp_template_service.go

type WhatsAppTemplateService struct {
    // Calls Meta Graph API:
    // POST   /{waba-id}/message_templates  (create)
    // GET    /{waba-id}/message_templates  (list)
    // DELETE /{waba-id}/message_templates  (delete)
    // Webhook: message_template_status_update (approved/rejected/paused)
}
```

### 5.3 Webhook Integration

Template status webhooks from Meta (handled by Phase 1 webhook receiver):

```go
// In meta_webhook_handler.go HandleWebhook():
case "message_template_status_update":
    // Update template approval status in ES
    // Push SSE event: { event: "template_status", name, status }
```

---

## 6. Phase 4: Conversation Inbox

> **Effort:** 2-3 weeks | **Risk:** Medium | **Depends on:** Phase 1

### 6.1 New API Endpoints (API-key protected)

| Method | Route | Purpose |
|--------|-------|---------|
| GET | `/v1/whatsapp/conversations` | List conversations (grouped by contact) |
| GET | `/v1/whatsapp/conversations/:contact_id/messages` | Get message history |
| POST | `/v1/whatsapp/conversations/:contact_id/reply` | Send reply (uses CSW if open, else requires template) |
| POST | `/v1/whatsapp/conversations/:contact_id/read` | Mark messages as read (sends read receipt to Meta) |

### 6.2 SSE Integration

Real-time inbound messages streamed to dashboard via existing SSE infrastructure:

```go
// In whatsapp_service HandleInbound():
s.broadcaster.PublishJSON("admin:"+appID, sse.SSEMessage{
    Event: "whatsapp_inbound",
    Data: map[string]interface{}{
        "contact_wa_id": msg.ContactWAID,
        "contact_name":  msg.ContactName,
        "message_type":  msg.MessageType,
        "text_body":     msg.TextBody,
        "timestamp":     msg.Timestamp,
    },
})
```

### 6.3 Reply Logic

```
User clicks "Reply" in inbox
          │
          ▼
    Is CSW open?
     (Redis check)
     ┌────┴────┐
    Yes        No
     │          │
     ▼          ▼
  Send free    Must use
  text/media   approved
  message      template
     │          │
     ▼          ▼
  POST to     POST to
  Meta API    Meta API
  type=text   type=template
```

---

## 7. Phase 5: Interactive Messages

> **Effort:** 1-2 weeks | **Risk:** Low | **Depends on:** Phase 0

### 7.1 Extend Meta Provider

Add support for interactive message types in `meta_whatsapp_provider.go`:

```go
// New message types in buildMessage():
// - type=interactive, action=buttons  (reply buttons, up to 3)
// - type=interactive, action=list     (list message with sections)
// - type=interactive, action=cta_url  (CTA URL button)
// - type=location                     (send location)
// - type=contacts                     (send contact card)
// - type=reaction                     (react to a message)
// - type=video / type=audio / type=document / type=sticker
```

### 7.2 DTO Extension

```go
// MODIFY internal/interfaces/http/dto/notification_dto.go
// Add to SendNotificationRequest.Data convention:

// data.whatsapp_interactive:
//   type: "buttons" | "list" | "cta_url"
//   header: { type: "text", text: "..." }
//   body: { text: "..." }
//   footer: { text: "..." }
//   action:
//     buttons: [{ type: "reply", reply: { id: "btn1", title: "Yes" } }]
//     -- or --
//     button: "See More"
//     sections: [{ title: "...", rows: [{ id: "row1", title: "...", description: "..." }] }]
```

### 7.3 Backward Compatibility

- Existing `data.whatsapp_template` convention continues to work
- New `data.whatsapp_interactive` is additive
- Plain text messages (no `data.whatsapp_*`) continue to send as `type=text`

---

## 8. Backward Compatibility Summary

| Area | Change | Impact on Existing Users |
|------|--------|--------------------------|
| **Feature flag** | `whatsapp_meta_enabled: false` (default) | **Zero** — all new code is gated |
| **Config struct** | New `MetaWhatsAppProviderConfig` field | **Zero** — `omitempty`, defaults exist |
| **Application model** | New optional fields on `WhatsAppAppConfig` | **Zero** — all `omitempty`, ES handles unknown fields |
| **Settings model** | New `WhatsAppInbound` field | **Zero** — `omitempty` |
| **Routes** | New routes only; no existing route changes | **Zero** |
| **Twilio provider** | Completely untouched | **Zero** |
| **Worker processor** | Added Meta BYOC branch alongside Twilio | **Zero** — Twilio path unchanged |
| **ES indices** | New `whatsapp_messages` index | **Zero** — new index, no migration |
| **Docker Compose** | No structural changes | **Zero** |
| **Existing API DTOs** | No changes to request/response format | **Zero** |

---

## 9. Environment Variables Reference

### New Variables (All Optional)

```bash
# ─── Feature Flag ───
FREERANGE_FEATURES_WHATSAPP_META_ENABLED=false

# ─── Meta WhatsApp Cloud API (Global/System) ───
FREERANGE_PROVIDERS_META_WHATSAPP_ENABLED=false
FREERANGE_PROVIDERS_META_WHATSAPP_PHONE_NUMBER_ID=
FREERANGE_PROVIDERS_META_WHATSAPP_WABA_ID=
FREERANGE_PROVIDERS_META_WHATSAPP_ACCESS_TOKEN=
FREERANGE_PROVIDERS_META_WHATSAPP_API_VERSION=v23.0
FREERANGE_PROVIDERS_META_WHATSAPP_APP_SECRET=        # For webhook X-Hub-Signature-256 verification
FREERANGE_PROVIDERS_META_WHATSAPP_WEBHOOK_VERIFY=    # hub.verify_token for webhook registration
FREERANGE_PROVIDERS_META_WHATSAPP_TIMEOUT=15
FREERANGE_PROVIDERS_META_WHATSAPP_MAX_RETRIES=3

# ─── Existing (unchanged) ───
FREERANGE_PROVIDERS_WHATSAPP_ENABLED=true            # Twilio WhatsApp (unchanged)
FREERANGE_PROVIDERS_WHATSAPP_ACCOUNT_SID=
FREERANGE_PROVIDERS_WHATSAPP_AUTH_TOKEN=
FREERANGE_PROVIDERS_WHATSAPP_FROM_NUMBER=
```

### Mapping to Config

| Env Variable | Config Path | Type | Default |
|---|---|---|---|
| `FREERANGE_FEATURES_WHATSAPP_META_ENABLED` | `features.whatsapp_meta_enabled` | bool | `false` |
| `FREERANGE_PROVIDERS_META_WHATSAPP_ENABLED` | `providers.meta_whatsapp.enabled` | bool | `false` |
| `FREERANGE_PROVIDERS_META_WHATSAPP_PHONE_NUMBER_ID` | `providers.meta_whatsapp.phone_number_id` | string | `""` |
| `FREERANGE_PROVIDERS_META_WHATSAPP_WABA_ID` | `providers.meta_whatsapp.waba_id` | string | `""` |
| `FREERANGE_PROVIDERS_META_WHATSAPP_ACCESS_TOKEN` | `providers.meta_whatsapp.access_token` | string | `""` |
| `FREERANGE_PROVIDERS_META_WHATSAPP_API_VERSION` | `providers.meta_whatsapp.api_version` | string | `"v23.0"` |
| `FREERANGE_PROVIDERS_META_WHATSAPP_APP_SECRET` | `providers.meta_whatsapp.app_secret` | string | `""` |
| `FREERANGE_PROVIDERS_META_WHATSAPP_WEBHOOK_VERIFY` | `providers.meta_whatsapp.webhook_verify` | string | `""` |

---

## 10. Docker Compose Changes

### No structural changes needed.

The existing `docker-compose.yml` already:
- Passes `env_file: .env` to both `notification-service` and `notification-worker`
- Both containers share the same image and config
- New env vars are picked up automatically via `.env`

### For Meta webhooks to work, the server must be publicly accessible

In production, the `notification-service` must be behind a reverse proxy (nginx, Caddy, Cloudflare Tunnel) with HTTPS. Meta requires HTTPS for webhook URLs.

```bash
# .env addition for webhook URL registration
FREERANGE_SERVER_PUBLIC_URL=https://api.yourapp.com
```

Meta webhook callback URL will be: `https://api.yourapp.com/v1/webhooks/meta/whatsapp`

---

## Appendix A: Complete File Change List

### New Files (Create)

| File | Phase | Purpose |
|------|-------|---------|
| `internal/domain/whatsapp/models.go` | P1 | Domain models |
| `internal/domain/whatsapp/repository.go` | P1 | Repository interface |
| `internal/domain/whatsapp/service.go` | P1 | Service interface |
| `internal/infrastructure/repository/whatsapp_repository.go` | P1 | ES repository |
| `internal/usecases/services/whatsapp_service_impl.go` | P1 | Service implementation |
| `internal/interfaces/http/handlers/meta_webhook_handler.go` | P1 | Webhook handler |
| `internal/interfaces/http/handlers/whatsapp_admin_handler.go` | P2 | Embedded Signup + template mgmt |
| `internal/interfaces/http/dto/whatsapp_dto.go` | P1 | DTOs |

### Modified Files

| File | Phase | Change |
|------|-------|--------|
| `internal/config/config.go` | P0 | Add `MetaWhatsAppProviderConfig`, `WhatsAppMetaEnabled` feature flag |
| `config/config.yaml` | P0 | Already has `meta_whatsapp` section — add `app_secret`, `webhook_verify` |
| `cmd/worker/main.go` | P0 | Add `meta_whatsapp` to `providerConfigs` map |
| `cmd/worker/processor.go` | P0 | Fix WhatsApp BYOC injection for Meta-only apps |
| `internal/container/container.go` | P1 | Wire `WhatsAppService`, `MetaWebhookHandler` |
| `internal/interfaces/http/routes/routes.go` | P1 | Add webhook routes + admin WhatsApp routes |
| `internal/domain/application/models.go` | P1 | Add `WhatsAppInbound` to Settings; extend `WhatsAppAppConfig` |
| `internal/infrastructure/database/index_templates.go` | P1 | Add `GetWhatsAppMessagesTemplate()` |
| `internal/infrastructure/database/database_manager.go` | P1 | Register new index |
| `internal/infrastructure/providers/meta_whatsapp_provider.go` | P5 | Add interactive message types |
| `internal/interfaces/http/dto/notification_dto.go` | P5 | Document `data.whatsapp_interactive` convention |

### Untouched Files (Confirmed No Changes)

| File | Reason |
|------|--------|
| `internal/infrastructure/providers/whatsapp_provider.go` | Twilio provider — completely unchanged |
| `internal/interfaces/http/middleware/auth.go` | No auth changes |
| `internal/interfaces/http/handlers/notification_handler.go` | No DTO changes |
| `docker-compose.yml` | No structural changes |
| All existing handler files | No modifications needed |

---

## Appendix B: Implementation Order & Dependencies

```
Phase 0: Fix Meta Provider Wiring (2-3 days)
  ├── config.go (MetaWhatsAppProviderConfig + feature flag)
  ├── worker/main.go (providerConfigs)
  ├── worker/processor.go (Meta BYOC context injection)
  └── config.yaml (already done)
        │
        ▼
Phase 1: Webhook Receiver (1-2 weeks)  ◀── Can start immediately after P0
  ├── domain/whatsapp/* (models, repo interface, service interface)
  ├── repository/whatsapp_repository.go
  ├── services/whatsapp_service_impl.go
  ├── handlers/meta_webhook_handler.go
  ├── dto/whatsapp_dto.go
  ├── container.go (wiring)
  ├── routes.go (webhook routes)
  ├── index_templates.go (ES index)
  └── application/models.go (InboundConfig)
        │
        ├── Phase 2: Embedded Signup (1-2 weeks)  ◀── Requires Meta App Review
        │     ├── handlers/whatsapp_admin_handler.go
        │     ├── routes.go (admin WhatsApp routes)
        │     └── application/models.go (WhatsAppAppConfig extensions)
        │
        ├── Phase 3: Template Management (2 weeks)  ◀── Requires Phase 0
        │     ├── services/whatsapp_template_service.go
        │     ├── handlers/whatsapp_admin_handler.go (additions)
        │     └── routes.go (template routes)
        │
        ├── Phase 4: Conversation Inbox (2-3 weeks)  ◀── Requires Phase 1
        │     ├── handlers/whatsapp_admin_handler.go (conversation endpoints)
        │     ├── routes.go (conversation routes)
        │     └── UI work (React inbox component)
        │
        └── Phase 5: Interactive Messages (1-2 weeks)  ◀── Requires Phase 0
              ├── meta_whatsapp_provider.go (extend buildMessage)
              └── dto docs (whatsapp_interactive convention)
```

**Critical path:** P0 → P1 → P4 (gets you send + receive + inbox)  
**Parallel work:** P2 (Embedded Signup) can proceed during Meta App Review wait  
**Independent:** P3 and P5 can happen in parallel with anything after P0

---

*This plan should be reviewed and refined before implementation begins. Update checklists as work progresses.*
