# Phase 3 — Channels & Delivery: Detailed Design Document

> **Status**: Ready for Implementation  
> **Depends On**: Phase 2 (Topics, Throttle, Audit Logs, RBAC) — ✅ Complete  
> **Estimated Duration**: 3–4 weeks  
> **Feature Count**: 4 features (Slack, Discord, WhatsApp, Custom Channel Provider)

---

## Table of Contents

- [Section 0: Concepts — What Are These Features?](#section-0-concepts--what-are-these-features)
- [Section 1: Codebase Audit — Provider Infrastructure](#section-1-codebase-audit--provider-infrastructure)
  - [1.1 Provider Interface & Manager](#11-provider-interface--manager)
  - [1.2 Channel Registration System](#12-channel-registration-system)
  - [1.3 Config Mapping Pattern](#13-config-mapping-pattern)
  - [1.4 Worker Registration Pattern](#14-worker-registration-pattern)
  - [1.5 Key Gaps & Technical Debt](#15-key-gaps--technical-debt)
- [Section 2: Slack Provider](#section-2-slack-provider)
  - [2.1 Channel Constant](#21-channel-constant)
  - [2.2 Provider Implementation](#22-provider-implementation)
  - [2.3 Config Structs](#23-config-structs)
  - [2.4 User & App Model Extensions](#24-user--app-model-extensions)
  - [2.5 Worker Registration](#25-worker-registration)
  - [2.6 API Contracts](#26-api-contracts)
- [Section 3: Discord Provider](#section-3-discord-provider)
  - [3.1 Channel Constant](#31-channel-constant)
  - [3.2 Provider Implementation](#32-provider-implementation)
  - [3.3 Config Structs](#33-config-structs)
  - [3.4 User & App Model Extensions](#34-user--app-model-extensions)
  - [3.5 Worker Registration](#35-worker-registration)
- [Section 4: WhatsApp Provider (via Twilio)](#section-4-whatsapp-provider-via-twilio)
  - [4.1 Channel Constant](#41-channel-constant)
  - [4.2 Provider Implementation](#42-provider-implementation)
  - [4.3 Config Structs](#43-config-structs)
  - [4.4 Worker Registration](#44-worker-registration)
- [Section 5: Custom Channel Provider SDK](#section-5-custom-channel-provider-sdk)
  - [5.1 Design Philosophy](#51-design-philosophy)
  - [5.2 CustomProviderConfig Model](#52-customproviderconfig-model)
  - [5.3 Provider Implementation](#53-provider-implementation)
  - [5.4 Registration API](#54-registration-api)
  - [5.5 Handler & Routes](#55-handler--routes)
  - [5.6 Dynamic Provider Loading in Worker](#56-dynamic-provider-loading-in-worker)
  - [5.7 Channel Validation Changes](#57-channel-validation-changes)
  - [5.8 API Contracts](#58-api-contracts)
- [Section 6: Wiring — Config, Container, Routes, Worker](#section-6-wiring--config-container-routes-worker)
  - [6.1 Config Changes](#61-config-changes)
  - [6.2 Feature Flags](#62-feature-flags)
  - [6.3 Worker main.go Changes](#63-worker-maingo-changes)
- [Section 7: User Preferences Extension](#section-7-user-preferences-extension)
  - [7.1 New Channel Preference Fields](#71-new-channel-preference-fields)
  - [7.2 Worker Preference Check Updates](#72-worker-preference-check-updates)
- [Section 8: Prometheus Metrics](#section-8-prometheus-metrics)
- [Section 9: File Inventory](#section-9-file-inventory)
- [Section 10: Backward Compatibility Contract](#section-10-backward-compatibility-contract)
- [Section 11: Implementation Order](#section-11-implementation-order)

---

## Section 0: Concepts — What Are These Features?

### Slack Provider

FreeRangeNotify delivers notifications via email, push, SMS, webhook, and SSE today.  **Slack** adds a seventh channel.  When a notification is sent with `channel: "slack"`, the system delivers it to a Slack workspace — either via an **Incoming Webhook URL** (simple, per-channel) or a **Bot Token** (richer, supporting DMs and multiple channels).

**Example:** A CI/CD tool creates a FRN notification with `channel: "slack"`.  The worker resolves the user's Slack webhook URL (or the app's default) and POST's a formatted Block Kit message to Slack.

### Discord Provider

Discord uses a nearly identical webhook model to Slack but with its own embed format.  The `channel: "discord"` provider delivers notifications as rich embeds to Discord channels via webhook URLs.

**Example:** A gaming leaderboard app sends `channel: "discord"` notifications.  The provider formats the content as a Discord embed with title, description, and color, then POST's to the configured webhook.

### WhatsApp Provider (via Twilio)

WhatsApp delivery reuses the existing Twilio infrastructure.  The only difference is the `To` field format (`whatsapp:+1234567890`) and the API endpoint.  This leverages the already-configured Twilio credentials — the `AccountSID` and `AuthToken` are shared, only the `FromNumber` changes to a WhatsApp-enabled sender.

**Example:** An e-commerce app sends order confirmations via `channel: "whatsapp"`.  The provider prefixes the user's phone number with `whatsapp:` and routes through the Twilio Messages API.

### Custom Channel Provider SDK

The **Custom Provider** is a webhook-based plugin system.  App owners register a custom delivery endpoint — FRN calls it via HTTP POST when delivering notifications on that custom channel.  This allows integrating with any messaging system (Telegram, Microsoft Teams, Line, internal tools) without modifying FRN source code.

**Example:** A company registers a custom provider named `"ms-teams"` pointing to `https://hooks.company.com/teams-relay`.  When a notification is sent with `channel: "ms-teams"`, FRN POST's the full notification payload to that URL with HMAC signing.

---

## Section 1: Codebase Audit — Provider Infrastructure

### 1.1 Provider Interface & Manager

Every provider implements the `Provider` interface in `internal/infrastructure/providers/provider.go`:

```go
type Provider interface {
    Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error)
    GetName() string
    GetSupportedChannel() notification.Channel
    IsHealthy(ctx context.Context) bool
    Close() error
}
```

The `Manager` (in `manager.go`) holds two maps:
- `providers map[notification.Channel]Provider` — default provider per channel (first registered wins)
- `namedProviders map[string]Provider` — keyed as `"name-channel"` for fallback chains

Key behaviors:
- `RegisterProvider(p)` sets the channel default if none exists + always adds to named map
- `Send(ctx, notif, usr)` resolves default provider by channel, checks circuit breaker, routes
- `SendWithFallback(ctx, notif, usr, names)` iterates named providers in order, first success wins
- `IsHealthy` / `HealthStatus` return per-provider health with circuit breaker state

### 1.2 Channel Registration System

Channels are string constants in `internal/domain/notification/models.go`:

```go
type Channel string
const (
    ChannelPush    Channel = "push"
    ChannelEmail   Channel = "email"
    ChannelSMS     Channel = "sms"
    ChannelWebhook Channel = "webhook"
    ChannelInApp   Channel = "in_app"
    ChannelSSE     Channel = "sse"
)
```

`Channel.Valid()` returns `true` only for these six values.  New channels require:
1. A new constant
2. Addition to `Valid()`
3. A provider that returns the new constant from `GetSupportedChannel()`

### 1.3 Config Mapping Pattern

Config types live in `internal/config/config.go` (`SMTPConfig`, `TwilioConfig`, etc.) and use `int` for timeouts (seconds).  Provider types in `internal/infrastructure/providers/` use `time.Duration`.  The worker manually maps between them:

```go
smtpProvider, err := providers.NewSMTPProvider(providers.SMTPConfig{
    Config: providers.Config{
        Timeout:    30 * time.Second,    // config.SMTP.Timeout
        MaxRetries: 3,                   // config.SMTP.MaxRetries
        RetryDelay: 1 * time.Second,     // hardcoded
    },
    Host:      cfg.Providers.SMTP.Host,
    // ...
}, logger)
```

Phase 3 providers follow this same pattern for consistency.

### 1.4 Worker Registration Pattern

In `cmd/worker/main.go`, each provider:
1. Is guarded by a config check (e.g., `cfg.Providers.SMTP.Host != ""`)
2. Gets constructed with provider-specific config + `logger`
3. Calls `providerManager.RegisterProvider(provider)` on success
4. Logs `Warn` on failure and continues (non-fatal)

Phase 3 providers follow this exact pattern.

### 1.5 Key Gaps & Technical Debt

| Gap | Impact on Phase 3 | Resolution |
|-----|-------------------|------------|
| `ChannelInApp` declared valid but has no provider | Not blocking; Phase 3 won't touch it | Leave as-is |
| No `webhook:` section in `config.yaml` | Not blocking; webhook uses env vars | Leave as-is |
| User preferences only cover 3 channels (email/push/sms) | New channels (slack/discord/whatsapp) have no preference toggle | Add `SlackEnabled`, `DiscordEnabled`, `WhatsAppEnabled` to `Preferences` and `DefaultPreferences` |
| `Manager.Send` has email-specific context routing | Custom providers don't need this | Leave as-is; custom provider uses standard path |
| Two providers can share one channel (FCM + APNS for push) | Slack is one-provider-one-channel, no conflict | No issue |

---

## Section 2: Slack Provider

### 2.1 Channel Constant

Add to `internal/domain/notification/models.go`:

```go
const ChannelSlack Channel = "slack"
```

Add `ChannelSlack` to `Valid()`.

### 2.2 Provider Implementation

**New file: `internal/infrastructure/providers/slack_provider.go`**

```go
package providers

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "github.com/the-monkeys/freerangenotify/internal/domain/notification"
    "github.com/the-monkeys/freerangenotify/internal/domain/user"
    "go.uber.org/zap"
)

// SlackConfig holds configuration for the Slack provider.
type SlackConfig struct {
    Config                                // Common: Timeout, MaxRetries, RetryDelay
    DefaultWebhookURL string             // App-level fallback webhook
}

// SlackProvider delivers notifications to Slack via Incoming Webhooks.
type SlackProvider struct {
    config     SlackConfig
    httpClient *http.Client
    logger     *zap.Logger
}

// NewSlackProvider creates a new SlackProvider.
func NewSlackProvider(config SlackConfig, logger *zap.Logger) (*SlackProvider, error) {
    return &SlackProvider{
        config: config,
        httpClient: &http.Client{Timeout: config.Timeout},
        logger: logger,
    }, nil
}

func (p *SlackProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
    start := time.Now()

    // Resolve webhook URL: metadata override > user field > app default
    webhookURL := p.resolveWebhookURL(notif, usr)
    if webhookURL == "" {
        return NewErrorResult(
            fmt.Errorf("no Slack webhook URL configured for user %s", notif.UserID),
            ErrorTypeInvalid,
        ), nil
    }

    // Build Slack Block Kit payload
    payload := p.buildPayload(notif)
    body, err := json.Marshal(payload)
    if err != nil {
        return NewErrorResult(fmt.Errorf("failed to marshal Slack payload: %w", err), ErrorTypeInvalid), nil
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
    if err != nil {
        return NewErrorResult(fmt.Errorf("failed to create request: %w", err), ErrorTypeUnknown), nil
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.httpClient.Do(req)
    if err != nil {
        return NewErrorResult(fmt.Errorf("Slack webhook request failed: %w", err), ErrorTypeNetwork), nil
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        respBody, _ := io.ReadAll(resp.Body)
        return NewErrorResult(
            fmt.Errorf("Slack returned status %d: %s", resp.StatusCode, string(respBody)),
            ErrorTypeProviderAPI,
        ), nil
    }

    return NewResult("slack-"+notif.NotificationID, time.Since(start)), nil
}

func (p *SlackProvider) GetName() string                           { return "slack" }
func (p *SlackProvider) GetSupportedChannel() notification.Channel { return notification.ChannelSlack }
func (p *SlackProvider) IsHealthy(_ context.Context) bool          { return true }
func (p *SlackProvider) Close() error                              { return nil }

// resolveWebhookURL determines the target Slack webhook.
// Priority: notification metadata > user-level > app-level default.
func (p *SlackProvider) resolveWebhookURL(notif *notification.Notification, usr *user.User) string {
    // 1. Notification-level override (set by template or caller)
    if notif.Metadata != nil {
        if url, ok := notif.Metadata["slack_webhook_url"].(string); ok && url != "" {
            return url
        }
    }
    // 2. User-level (per-subscriber Slack webhook)
    if usr != nil && usr.SlackWebhookURL != "" {
        return usr.SlackWebhookURL
    }
    // 3. App-level default
    return p.config.DefaultWebhookURL
}

// buildPayload constructs a Slack Block Kit message.
func (p *SlackProvider) buildPayload(notif *notification.Notification) map[string]interface{} {
    blocks := []map[string]interface{}{
        {
            "type": "section",
            "text": map[string]interface{}{
                "type": "mrkdwn",
                "text": fmt.Sprintf("*%s*\n%s", notif.Content.Title, notif.Content.Body),
            },
        },
    }

    // Add action URL button if present in data
    if notif.Content.Data != nil {
        if actionURL, ok := notif.Content.Data["action_url"].(string); ok && actionURL != "" {
            actionLabel := "View"
            if label, ok := notif.Content.Data["action_label"].(string); ok && label != "" {
                actionLabel = label
            }
            blocks = append(blocks, map[string]interface{}{
                "type": "actions",
                "elements": []map[string]interface{}{
                    {
                        "type": "button",
                        "text": map[string]interface{}{
                            "type": "plain_text",
                            "text": actionLabel,
                        },
                        "url": actionURL,
                    },
                },
            })
        }
    }

    return map[string]interface{}{
        "text":   notif.Content.Title, // Fallback for notifications
        "blocks": blocks,
    }
}
```

### 2.3 Config Structs

**In `internal/config/config.go`**, add `SlackProviderConfig` and add it to `ProvidersConfig`:

```go
type SlackProviderConfig struct {
    Enabled           bool   `mapstructure:"enabled"`
    DefaultWebhookURL string `mapstructure:"default_webhook_url"`
    Timeout           int    `mapstructure:"timeout"`
    MaxRetries        int    `mapstructure:"max_retries"`
}
```

**In `config/config.yaml`**, add under `providers:`:

```yaml
  slack:
    enabled: false
    default_webhook_url: "${FREERANGE_PROVIDERS_SLACK_DEFAULT_WEBHOOK_URL:-}"
    timeout: 10
    max_retries: 3
```

### 2.4 User & App Model Extensions

**User model** — add channel-specific field:

```go
type User struct {
    // ... existing fields ...
    SlackWebhookURL string `json:"slack_webhook_url,omitempty" es:"slack_webhook_url"`
}
```

**App Settings** — add `SlackConfig`:

```go
type SlackConfig struct {
    WebhookURL string `json:"webhook_url,omitempty" es:"webhook_url"`
}
```

Add `Slack *SlackConfig` to `Settings`.

### 2.5 Worker Registration

In `cmd/worker/main.go`, after existing provider registrations:

```go
// Slack provider
if cfg.Providers.Slack.Enabled {
    slackProvider, err := providers.NewSlackProvider(providers.SlackConfig{
        Config: providers.Config{
            Timeout:    time.Duration(cfg.Providers.Slack.Timeout) * time.Second,
            MaxRetries: cfg.Providers.Slack.MaxRetries,
            RetryDelay: 2 * time.Second,
        },
        DefaultWebhookURL: cfg.Providers.Slack.DefaultWebhookURL,
    }, logger)
    if err != nil {
        logger.Warn("Failed to initialize Slack provider", zap.Error(err))
    } else {
        if err := providerManager.RegisterProvider(slackProvider); err != nil {
            logger.Warn("Failed to register Slack provider", zap.Error(err))
        } else {
            logger.Info("Registered Slack provider")
        }
    }
}
```

### 2.6 API Contracts

No new API endpoints.  Slack delivery is triggered by sending a notification with `channel: "slack"`:

```json
POST /v1/notifications
{
    "user_id": "user-uuid",
    "channel": "slack",
    "content": {
        "title": "Build #142 passed",
        "body": "All 847 tests passed in 3m12s.",
        "data": {
            "action_url": "https://ci.example.com/builds/142",
            "action_label": "View Build"
        }
    }
}
```

The webhook URL is resolved from: notification `metadata.slack_webhook_url` → user `slack_webhook_url` → app settings `slack.webhook_url` → config default.

---

## Section 3: Discord Provider

### 3.1 Channel Constant

Add to `internal/domain/notification/models.go`:

```go
const ChannelDiscord Channel = "discord"
```

Add `ChannelDiscord` to `Valid()`.

### 3.2 Provider Implementation

**New file: `internal/infrastructure/providers/discord_provider.go`**

```go
package providers

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "github.com/the-monkeys/freerangenotify/internal/domain/notification"
    "github.com/the-monkeys/freerangenotify/internal/domain/user"
    "go.uber.org/zap"
)

// DiscordConfig holds configuration for the Discord provider.
type DiscordConfig struct {
    Config                                // Common: Timeout, MaxRetries, RetryDelay
    DefaultWebhookURL string             // App-level fallback webhook
}

// DiscordProvider delivers notifications to Discord via Incoming Webhooks.
type DiscordProvider struct {
    config     DiscordConfig
    httpClient *http.Client
    logger     *zap.Logger
}

// NewDiscordProvider creates a new DiscordProvider.
func NewDiscordProvider(config DiscordConfig, logger *zap.Logger) (*DiscordProvider, error) {
    return &DiscordProvider{
        config: config,
        httpClient: &http.Client{Timeout: config.Timeout},
        logger: logger,
    }, nil
}

func (p *DiscordProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
    start := time.Now()

    webhookURL := p.resolveWebhookURL(notif, usr)
    if webhookURL == "" {
        return NewErrorResult(
            fmt.Errorf("no Discord webhook URL for user %s", notif.UserID),
            ErrorTypeInvalid,
        ), nil
    }

    payload := p.buildPayload(notif)
    body, err := json.Marshal(payload)
    if err != nil {
        return NewErrorResult(fmt.Errorf("failed to marshal Discord payload: %w", err), ErrorTypeInvalid), nil
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
    if err != nil {
        return NewErrorResult(fmt.Errorf("failed to create request: %w", err), ErrorTypeUnknown), nil
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.httpClient.Do(req)
    if err != nil {
        return NewErrorResult(fmt.Errorf("Discord webhook request failed: %w", err), ErrorTypeNetwork), nil
    }
    defer resp.Body.Close()

    // Discord returns 204 No Content on success
    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
        respBody, _ := io.ReadAll(resp.Body)
        return NewErrorResult(
            fmt.Errorf("Discord returned status %d: %s", resp.StatusCode, string(respBody)),
            ErrorTypeProviderAPI,
        ), nil
    }

    return NewResult("discord-"+notif.NotificationID, time.Since(start)), nil
}

func (p *DiscordProvider) GetName() string                           { return "discord" }
func (p *DiscordProvider) GetSupportedChannel() notification.Channel { return notification.ChannelDiscord }
func (p *DiscordProvider) IsHealthy(_ context.Context) bool          { return true }
func (p *DiscordProvider) Close() error                              { return nil }

func (p *DiscordProvider) resolveWebhookURL(notif *notification.Notification, usr *user.User) string {
    if notif.Metadata != nil {
        if url, ok := notif.Metadata["discord_webhook_url"].(string); ok && url != "" {
            return url
        }
    }
    if usr != nil && usr.DiscordWebhookURL != "" {
        return usr.DiscordWebhookURL
    }
    return p.config.DefaultWebhookURL
}

// buildPayload constructs a Discord embed message.
func (p *DiscordProvider) buildPayload(notif *notification.Notification) map[string]interface{} {
    embed := map[string]interface{}{
        "title":       notif.Content.Title,
        "description": notif.Content.Body,
        "color":       3447003, // Discord blue (#3498DB)
    }

    // Add URL field if action_url is present
    if notif.Content.Data != nil {
        if actionURL, ok := notif.Content.Data["action_url"].(string); ok && actionURL != "" {
            embed["url"] = actionURL
        }
    }

    return map[string]interface{}{
        "content": notif.Content.Title,
        "embeds":  []map[string]interface{}{embed},
    }
}
```

### 3.3 Config Structs

**In `internal/config/config.go`**:

```go
type DiscordProviderConfig struct {
    Enabled           bool   `mapstructure:"enabled"`
    DefaultWebhookURL string `mapstructure:"default_webhook_url"`
    Timeout           int    `mapstructure:"timeout"`
    MaxRetries        int    `mapstructure:"max_retries"`
}
```

**In `config/config.yaml`**:

```yaml
  discord:
    enabled: false
    default_webhook_url: "${FREERANGE_PROVIDERS_DISCORD_DEFAULT_WEBHOOK_URL:-}"
    timeout: 10
    max_retries: 3
```

### 3.4 User & App Model Extensions

**User model** — add:

```go
DiscordWebhookURL string `json:"discord_webhook_url,omitempty" es:"discord_webhook_url"`
```

**App Settings** — add `DiscordConfig`:

```go
type DiscordConfig struct {
    WebhookURL string `json:"webhook_url,omitempty" es:"webhook_url"`
}
```

Add `Discord *DiscordConfig` to `Settings`.

### 3.5 Worker Registration

```go
// Discord provider
if cfg.Providers.Discord.Enabled {
    discordProvider, err := providers.NewDiscordProvider(providers.DiscordConfig{
        Config: providers.Config{
            Timeout:    time.Duration(cfg.Providers.Discord.Timeout) * time.Second,
            MaxRetries: cfg.Providers.Discord.MaxRetries,
            RetryDelay: 2 * time.Second,
        },
        DefaultWebhookURL: cfg.Providers.Discord.DefaultWebhookURL,
    }, logger)
    if err != nil {
        logger.Warn("Failed to initialize Discord provider", zap.Error(err))
    } else {
        if err := providerManager.RegisterProvider(discordProvider); err != nil {
            logger.Warn("Failed to register Discord provider", zap.Error(err))
        } else {
            logger.Info("Registered Discord provider")
        }
    }
}
```

---

## Section 4: WhatsApp Provider (via Twilio)

### 4.1 Channel Constant

Add to `internal/domain/notification/models.go`:

```go
const ChannelWhatsApp Channel = "whatsapp"
```

Add `ChannelWhatsApp` to `Valid()`.

### 4.2 Provider Implementation

**New file: `internal/infrastructure/providers/whatsapp_provider.go`**

The WhatsApp provider reuses Twilio's Messages API.  The only differences from the existing `TwilioProvider`:
1. `To` field uses `whatsapp:+number` prefix
2. `From` field uses a WhatsApp-enabled Twilio sender number
3. Supports Twilio Content API templates for structured messages

```go
package providers

import (
    "context"
    "fmt"
    "net/http"
    "net/url"
    "strings"
    "time"

    "github.com/the-monkeys/freerangenotify/internal/domain/notification"
    "github.com/the-monkeys/freerangenotify/internal/domain/user"
    "go.uber.org/zap"
)

// WhatsAppConfig holds configuration for the WhatsApp provider (Twilio-backed).
type WhatsAppConfig struct {
    Config                    // Common: Timeout, MaxRetries, RetryDelay
    AccountSID string        // Twilio Account SID
    AuthToken  string        // Twilio Auth Token
    FromNumber string        // WhatsApp sender (whatsapp:+14155238886)
}

// WhatsAppProvider delivers notifications via WhatsApp using the Twilio Messages API.
type WhatsAppProvider struct {
    config     WhatsAppConfig
    httpClient *http.Client
    logger     *zap.Logger
}

// NewWhatsAppProvider creates a new WhatsAppProvider.
func NewWhatsAppProvider(config WhatsAppConfig, logger *zap.Logger) (*WhatsAppProvider, error) {
    if config.AccountSID == "" || config.AuthToken == "" {
        return nil, fmt.Errorf("WhatsApp provider requires Twilio AccountSID and AuthToken")
    }
    // Ensure from number has whatsapp: prefix
    if !strings.HasPrefix(config.FromNumber, "whatsapp:") {
        config.FromNumber = "whatsapp:" + config.FromNumber
    }
    return &WhatsAppProvider{
        config:     config,
        httpClient: &http.Client{Timeout: config.Timeout},
        logger:     logger,
    }, nil
}

func (p *WhatsAppProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
    start := time.Now()

    if usr == nil || usr.Phone == "" {
        return NewErrorResult(
            fmt.Errorf("user has no phone number for WhatsApp delivery"),
            ErrorTypeInvalid,
        ), nil
    }

    // Build message body (title + body)
    messageBody := notif.Content.Body
    if notif.Content.Title != "" {
        messageBody = fmt.Sprintf("*%s*\n\n%s", notif.Content.Title, notif.Content.Body)
    }

    // Twilio Messages API
    apiURL := fmt.Sprintf(
        "https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json",
        p.config.AccountSID,
    )

    toNumber := usr.Phone
    if !strings.HasPrefix(toNumber, "whatsapp:") {
        toNumber = "whatsapp:" + toNumber
    }

    data := url.Values{
        "To":   {toNumber},
        "From": {p.config.FromNumber},
        "Body": {messageBody},
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(data.Encode()))
    if err != nil {
        return NewErrorResult(fmt.Errorf("failed to create WhatsApp request: %w", err), ErrorTypeUnknown), nil
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.SetBasicAuth(p.config.AccountSID, p.config.AuthToken)

    resp, err := p.httpClient.Do(req)
    if err != nil {
        return NewErrorResult(fmt.Errorf("WhatsApp request failed: %w", err), ErrorTypeNetwork), nil
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
        return NewErrorResult(
            fmt.Errorf("Twilio WhatsApp API returned status %d", resp.StatusCode),
            ErrorTypeProviderAPI,
        ), nil
    }

    return NewResult("whatsapp-"+notif.NotificationID, time.Since(start)), nil
}

func (p *WhatsAppProvider) GetName() string                           { return "whatsapp" }
func (p *WhatsAppProvider) GetSupportedChannel() notification.Channel { return notification.ChannelWhatsApp }
func (p *WhatsAppProvider) IsHealthy(_ context.Context) bool          { return true }
func (p *WhatsAppProvider) Close() error                              { return nil }
```

### 4.3 Config Structs

**In `internal/config/config.go`**:

```go
type WhatsAppProviderConfig struct {
    Enabled    bool   `mapstructure:"enabled"`
    AccountSID string `mapstructure:"account_sid"` // Can share with Twilio SMS
    AuthToken  string `mapstructure:"auth_token"`   // Can share with Twilio SMS
    FromNumber string `mapstructure:"from_number"`  // WhatsApp-enabled sender
    Timeout    int    `mapstructure:"timeout"`
    MaxRetries int    `mapstructure:"max_retries"`
}
```

**In `config/config.yaml`**:

```yaml
  whatsapp:
    enabled: false
    account_sid: "${FREERANGE_PROVIDERS_WHATSAPP_ACCOUNT_SID:-}"
    auth_token: "${FREERANGE_PROVIDERS_WHATSAPP_AUTH_TOKEN:-}"
    from_number: "${FREERANGE_PROVIDERS_WHATSAPP_FROM_NUMBER:-}"
    timeout: 15
    max_retries: 3
```

### 4.4 Worker Registration

```go
// WhatsApp provider (Twilio-backed)
if cfg.Providers.WhatsApp.Enabled {
    whatsappProvider, err := providers.NewWhatsAppProvider(providers.WhatsAppConfig{
        Config: providers.Config{
            Timeout:    time.Duration(cfg.Providers.WhatsApp.Timeout) * time.Second,
            MaxRetries: cfg.Providers.WhatsApp.MaxRetries,
            RetryDelay: 2 * time.Second,
        },
        AccountSID: cfg.Providers.WhatsApp.AccountSID,
        AuthToken:  cfg.Providers.WhatsApp.AuthToken,
        FromNumber: cfg.Providers.WhatsApp.FromNumber,
    }, logger)
    if err != nil {
        logger.Warn("Failed to initialize WhatsApp provider", zap.Error(err))
    } else {
        if err := providerManager.RegisterProvider(whatsappProvider); err != nil {
            logger.Warn("Failed to register WhatsApp provider", zap.Error(err))
        } else {
            logger.Info("Registered WhatsApp provider")
        }
    }
}
```

---

## Section 5: Custom Channel Provider SDK

### 5.1 Design Philosophy

The custom provider is a **webhook relay**.  App owners register a delivery endpoint in FRN.  When a notification targets that custom channel, FRN:

1. Serializes the notification + user context into a standard JSON payload
2. Signs the payload with HMAC-SHA256 (same scheme as the existing webhook provider)
3. POST's to the registered endpoint
4. Treats 2xx as success, anything else as failure (with standard retry logic)

This design lets FRN deliver to any messaging system without requiring custom Go code.

### 5.2 CustomProviderConfig Model

Add to `internal/domain/application/models.go`:

```go
// CustomProviderConfig defines a user-registered custom delivery channel.
// Stored in app Settings.CustomProviders.
type CustomProviderConfig struct {
    ProviderID string            `json:"provider_id" es:"provider_id"`
    Name       string            `json:"name" es:"name"`               // e.g., "ms-teams"
    Channel    string            `json:"channel" es:"channel"`         // Arbitrary channel name
    WebhookURL string            `json:"webhook_url" es:"webhook_url"`
    Headers    map[string]string `json:"headers,omitempty" es:"headers"`
    SigningKey  string           `json:"signing_key" es:"signing_key"` // HMAC-SHA256 key
    Active     bool              `json:"active" es:"active"`
    CreatedAt  string            `json:"created_at,omitempty" es:"created_at"`
}
```

Add to `Settings`:

```go
CustomProviders []CustomProviderConfig `json:"custom_providers,omitempty" es:"custom_providers"`
```

### 5.3 Provider Implementation

**New file: `internal/infrastructure/providers/custom_provider.go`**

```go
package providers

import (
    "bytes"
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "github.com/the-monkeys/freerangenotify/internal/domain/notification"
    "github.com/the-monkeys/freerangenotify/internal/domain/user"
    "go.uber.org/zap"
)

// CustomProvider delivers notifications to a user-registered webhook endpoint.
type CustomProvider struct {
    name       string
    channel    notification.Channel
    webhookURL string
    headers    map[string]string
    signingKey string
    httpClient *http.Client
    logger     *zap.Logger
}

// NewCustomProvider creates a custom webhook-based provider.
func NewCustomProvider(name, channel, webhookURL, signingKey string, headers map[string]string, logger *zap.Logger) *CustomProvider {
    return &CustomProvider{
        name:       name,
        channel:    notification.Channel(channel),
        webhookURL: webhookURL,
        headers:    headers,
        signingKey: signingKey,
        httpClient: &http.Client{Timeout: 30 * time.Second},
        logger:     logger,
    }
}

func (p *CustomProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
    start := time.Now()

    // Build standardized payload
    payload := map[string]interface{}{
        "notification_id": notif.NotificationID,
        "app_id":          notif.AppID,
        "user_id":         notif.UserID,
        "channel":         string(p.channel),
        "content":         notif.Content,
        "metadata":        notif.Metadata,
        "priority":        string(notif.Priority),
        "category":        notif.Category,
        "created_at":      notif.CreatedAt,
    }

    // Include user context if available
    if usr != nil {
        payload["user"] = map[string]interface{}{
            "email":       usr.Email,
            "phone":       usr.Phone,
            "external_id": usr.ExternalID,
            "timezone":    usr.Timezone,
            "language":    usr.Language,
        }
    }

    body, err := json.Marshal(payload)
    if err != nil {
        return NewErrorResult(fmt.Errorf("failed to marshal custom payload: %w", err), ErrorTypeInvalid), nil
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.webhookURL, bytes.NewReader(body))
    if err != nil {
        return NewErrorResult(fmt.Errorf("failed to create request: %w", err), ErrorTypeUnknown), nil
    }

    // Set standard headers
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("User-Agent", "FreeRangeNotify/1.0")

    // Set custom headers
    for k, v := range p.headers {
        req.Header.Set(k, v)
    }

    // HMAC signature
    if p.signingKey != "" {
        mac := hmac.New(sha256.New, []byte(p.signingKey))
        mac.Write(body)
        signature := hex.EncodeToString(mac.Sum(nil))
        req.Header.Set("X-FRN-Signature", signature)
    }

    resp, err := p.httpClient.Do(req)
    if err != nil {
        return NewErrorResult(fmt.Errorf("custom provider request failed: %w", err), ErrorTypeNetwork), nil
    }
    defer resp.Body.Close()

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        respBody, _ := io.ReadAll(resp.Body)
        return NewErrorResult(
            fmt.Errorf("custom provider returned status %d: %s", resp.StatusCode, string(respBody)),
            ErrorTypeProviderAPI,
        ), nil
    }

    return NewResult(fmt.Sprintf("custom-%s-%s", p.name, notif.NotificationID), time.Since(start)), nil
}

func (p *CustomProvider) GetName() string                           { return p.name }
func (p *CustomProvider) GetSupportedChannel() notification.Channel { return p.channel }
func (p *CustomProvider) IsHealthy(_ context.Context) bool          { return true }
func (p *CustomProvider) Close() error                              { return nil }
```

### 5.4 Registration API

Custom providers are managed through the application settings endpoint.  Since `CustomProviders` is a slice on `Settings`, no new index or repository is needed — they ride on the existing `Application` document in Elasticsearch.

However, for a clean DX, we add dedicated endpoints:

| Method | Path | Description |
|--------|------|-------------|
| `POST`   | `/v1/apps/:id/providers` | Register a custom provider |
| `GET`    | `/v1/apps/:id/providers` | List custom providers |
| `DELETE` | `/v1/apps/:id/providers/:provider_id` | Remove a custom provider |

### 5.5 Handler & Routes

**New file: `internal/interfaces/http/handlers/custom_provider_handler.go`**

```go
package handlers

type CustomProviderHandler struct {
    appService  usecases.ApplicationService
    logger      *zap.Logger
}

func NewCustomProviderHandler(appService usecases.ApplicationService, logger *zap.Logger) *CustomProviderHandler
```

Methods:

- **`Register(c *fiber.Ctx)`**: Parses `CustomProviderConfig` from body, generates `provider_id` (UUID) and `signing_key` (32-byte hex), appends to `app.Settings.CustomProviders`, saves app.
- **`List(c *fiber.Ctx)`**: Returns `app.Settings.CustomProviders` (with `signing_key` redacted to last 4 chars).
- **`Remove(c *fiber.Ctx)`**: Filters out the provider by ID, saves app.

**Routes** (in `setupAdminRoutes` or JWT-protected app routes):

```go
apps.Post("/:id/providers", customProviderHandler.Register)
apps.Get("/:id/providers", customProviderHandler.List)
apps.Delete("/:id/providers/:provider_id", customProviderHandler.Remove)
```

### 5.6 Dynamic Provider Loading in Worker

Custom providers are defined at the application level and can change at runtime.  The worker cannot know all custom providers at startup.  Two approaches:

**Approach A (Simple — recommended for MVP):** Resolve at delivery time.

In `cmd/worker/processor.go`, within `sendNotification`:

```go
// If no built-in provider found for channel, check for custom provider on the app
if _, err := p.providerManager.GetProvider(notif.Channel); err != nil {
    app, appErr := p.appRepo.GetByID(ctx, notif.AppID)
    if appErr == nil && app != nil {
        for _, cp := range app.Settings.CustomProviders {
            if cp.Channel == string(notif.Channel) && cp.Active {
                customProvider := providers.NewCustomProvider(
                    cp.Name, cp.Channel, cp.WebhookURL, cp.SigningKey, cp.Headers, p.logger,
                )
                result, sendErr := customProvider.Send(ctx, notif, usr)
                // handle result...
            }
        }
    }
}
```

This avoids any background polling or caching complexity.  The app document is already fetched during notification processing (for retry config, email config, etc.), so this adds zero extra reads.

**Approach B (Future optimization):** Background poller refreshes custom providers from ES every 5 minutes and registers them with the `Manager`.  Implement this only if custom provider usage becomes high-volume.

### 5.7 Channel Validation Changes

Currently, `Channel.Valid()` is a hardcoded switch.  Custom channels break this pattern because channel names are dynamic.

**Solution:** Add a `ValidCustom(appCustomChannels []string)` method or change `Valid()` to also accept any non-empty string when invoked from a context that knows custom channels are enabled.

Simpler approach for MVP: in the notification service `Send()` method, skip the `Valid()` check if a custom provider exists for the app. The validation already happens when the worker tries to resolve the provider — if no provider is found, it fails with a clear error.

**Implementation:** Add an `IsCustomChannel` check in `SendRequest.Validate()`:

```go
func (r *SendRequest) Validate() error {
    if r.Channel == "" {
        return fmt.Errorf("channel is required")
    }
    // Allow recognized built-in channels + any non-empty custom channel
    if !r.Channel.Valid() {
        // Custom channels are allowed — validation happens at delivery time
        // when the provider is resolved. Just ensure it's not empty.
    }
    // ...
}
```

### 5.8 API Contracts

**Register Custom Provider:**

```http
POST /v1/apps/:id/providers
Authorization: Bearer <jwt>

{
    "name": "ms-teams",
    "channel": "ms-teams",
    "webhook_url": "https://hooks.company.com/teams-relay",
    "headers": {
        "X-Tenant": "acme-corp"
    }
}
```

**Response (201):**

```json
{
    "provider_id": "uuid",
    "name": "ms-teams",
    "channel": "ms-teams",
    "webhook_url": "https://hooks.company.com/teams-relay",
    "signing_key": "a1b2c3d4...64chars",
    "active": true,
    "created_at": "2026-03-07T..."
}
```

**Send via Custom Channel:**

```http
POST /v1/notifications
Authorization: Bearer <api-key>

{
    "user_id": "user-uuid",
    "channel": "ms-teams",
    "content": {
        "title": "New ticket assigned",
        "body": "Ticket #5432 has been assigned to you."
    }
}
```

The worker resolves `channel: "ms-teams"` → finds no built-in provider → checks `app.Settings.CustomProviders` → finds the matching entry → POST's to the webhook URL with HMAC signing.

---

## Section 6: Wiring — Config, Container, Routes, Worker

### 6.1 Config Changes

**`internal/config/config.go`** — add to `ProvidersConfig`:

```go
type ProvidersConfig struct {
    FCM      FCMConfig              `mapstructure:"fcm"`
    APNS     APNSConfig             `mapstructure:"apns"`
    SendGrid SendGridConfig         `mapstructure:"sendgrid"`
    Twilio   TwilioConfig           `mapstructure:"twilio"`
    SMTP     SMTPConfig             `mapstructure:"smtp"`
    Webhook  WebhookConfig          `mapstructure:"webhook"`
    // Phase 3
    Slack    SlackProviderConfig    `mapstructure:"slack"`
    Discord  DiscordProviderConfig  `mapstructure:"discord"`
    WhatsApp WhatsAppProviderConfig `mapstructure:"whatsapp"`
}
```

**`config/config.yaml`** — add under `providers:`:

```yaml
  # Phase 3
  slack:
    enabled: false
    default_webhook_url: "${FREERANGE_PROVIDERS_SLACK_DEFAULT_WEBHOOK_URL:-}"
    timeout: 10
    max_retries: 3

  discord:
    enabled: false
    default_webhook_url: "${FREERANGE_PROVIDERS_DISCORD_DEFAULT_WEBHOOK_URL:-}"
    timeout: 10
    max_retries: 3

  whatsapp:
    enabled: false
    account_sid: "${FREERANGE_PROVIDERS_WHATSAPP_ACCOUNT_SID:-}"
    auth_token: "${FREERANGE_PROVIDERS_WHATSAPP_AUTH_TOKEN:-}"
    from_number: "${FREERANGE_PROVIDERS_WHATSAPP_FROM_NUMBER:-}"
    timeout: 15
    max_retries: 3
```

### 6.2 Feature Flags

Phase 3 does **not** use `FeaturesConfig` feature flags.  Provider enablement is controlled by the `enabled` field in each provider's config — this follows the existing pattern (SMTP is enabled when `host != ""`, SendGrid when `api_key != ""`).  Phase 3 providers use an explicit `enabled: true/false` toggle, which is cleaner.

Custom providers have no global feature flag — they are implicitly enabled when an app registers one via the API.

### 6.3 Worker main.go Changes

Add Slack, Discord, and WhatsApp provider registration blocks after the SSE provider block, each guarded by `cfg.Providers.X.Enabled`.

For custom provider resolution, modify `processor.go`'s `sendNotification` method to check `app.Settings.CustomProviders` as a fallback when no built-in provider is found.

---

## Section 7: User Preferences Extension

### 7.1 New Channel Preference Fields

Add to `Preferences` in `internal/domain/user/models.go`:

```go
type Preferences struct {
    // ... existing ...
    SlackEnabled    *bool `json:"slack_enabled,omitempty" es:"slack_enabled"`
    DiscordEnabled  *bool `json:"discord_enabled,omitempty" es:"discord_enabled"`
    WhatsAppEnabled *bool `json:"whatsapp_enabled,omitempty" es:"whatsapp_enabled"`
}
```

Add to `DefaultPreferences` in `internal/domain/application/models.go`:

```go
type DefaultPreferences struct {
    EmailEnabled    *bool `json:"email_enabled" es:"email_enabled"`
    PushEnabled     *bool `json:"push_enabled" es:"push_enabled"`
    SMSEnabled      *bool `json:"sms_enabled" es:"sms_enabled"`
    SlackEnabled    *bool `json:"slack_enabled,omitempty" es:"slack_enabled"`
    DiscordEnabled  *bool `json:"discord_enabled,omitempty" es:"discord_enabled"`
    WhatsAppEnabled *bool `json:"whatsapp_enabled,omitempty" es:"whatsapp_enabled"`
}
```

### 7.2 Worker Preference Check Updates

In `cmd/worker/processor.go`, update `checkUserPreferences`:

```go
func (p *NotificationProcessor) checkUserPreferences(usr *user.User, notif *notification.Notification) bool {
    switch notif.Channel {
    case notification.ChannelEmail:
        if !utils.BoolValue(usr.Preferences.EmailEnabled) { return false }
    case notification.ChannelPush:
        if !utils.BoolValue(usr.Preferences.PushEnabled) { return false }
    case notification.ChannelSMS:
        if !utils.BoolValue(usr.Preferences.SMSEnabled) { return false }
    case notification.ChannelSlack:
        if usr.Preferences.SlackEnabled != nil && !*usr.Preferences.SlackEnabled { return false }
    case notification.ChannelDiscord:
        if usr.Preferences.DiscordEnabled != nil && !*usr.Preferences.DiscordEnabled { return false }
    case notification.ChannelWhatsApp:
        if usr.Preferences.WhatsAppEnabled != nil && !*usr.Preferences.WhatsAppEnabled { return false }
    }
    // Quiet hours & DND checks ...
}
```

**Semantic difference:** Existing channels default to `false` (opt-in via `BoolValue`).  New channels default to `true` (opt-out) — notifications go through unless the user explicitly disables them.  This is because existing users won't have these fields set; blocking by default would break delivery.

---

## Section 8: Prometheus Metrics

Add to `internal/infrastructure/metrics/metrics.go`:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `frn_provider_requests_total` | Counter | `provider`, `channel`, `status` | Already exists; Phase 3 adds `slack`, `discord`, `whatsapp`, `custom-*` label values |
| `frn_custom_provider_registrations_total` | Counter | `app_id` | Custom provider registration events |

No new metric structs needed — the existing `RecordDeliverySuccess(channel, provider)` and `RecordDeliveryFailure(channel, provider, errorType)` on `NotificationMetrics` already accept arbitrary string labels.  Phase 3 providers automatically contribute to these counters.

---

## Section 9: File Inventory

### New Files (5)

| File | Description |
|------|-------------|
| `internal/infrastructure/providers/slack_provider.go` | Slack delivery via Incoming Webhooks |
| `internal/infrastructure/providers/discord_provider.go` | Discord delivery via Webhooks |
| `internal/infrastructure/providers/whatsapp_provider.go` | WhatsApp delivery via Twilio Messages API |
| `internal/infrastructure/providers/custom_provider.go` | Generic webhook-based custom channel relay |
| `internal/interfaces/http/handlers/custom_provider_handler.go` | CRUD for custom provider registration |

### Modified Files (8)

| File | Changes |
|------|---------|
| `internal/domain/notification/models.go` | Add `ChannelSlack`, `ChannelDiscord`, `ChannelWhatsApp` constants; update `Valid()` |
| `internal/domain/user/models.go` | Add `SlackWebhookURL`, `DiscordWebhookURL` to `User`; add `SlackEnabled`, `DiscordEnabled`, `WhatsAppEnabled` to `Preferences` |
| `internal/domain/application/models.go` | Add `SlackConfig`, `DiscordConfig`, `CustomProviderConfig` types; add `Slack`, `Discord`, `CustomProviders` fields to `Settings`; add preference fields to `DefaultPreferences` |
| `internal/config/config.go` | Add `SlackProviderConfig`, `DiscordProviderConfig`, `WhatsAppProviderConfig` structs; add to `ProvidersConfig` |
| `config/config.yaml` | Add `slack:`, `discord:`, `whatsapp:` sections under `providers:` |
| `cmd/worker/main.go` | Register Slack, Discord, WhatsApp providers (guarded by `enabled` flag) |
| `cmd/worker/processor.go` | Custom provider fallback in `sendNotification`; update `checkUserPreferences` for new channels |
| `internal/interfaces/http/routes/routes.go` | Register custom provider CRUD routes under `/v1/apps/:id/providers` |

### No New Elasticsearch Indices

Phase 3 creates no new indices.  Custom providers are stored inside the existing `Application` document as an array field.

---

## Section 10: Backward Compatibility Contract

1. **Zero breaking changes** — all existing channels, providers, and APIs work identically.
2. **New channels are opt-in** — Slack/Discord/WhatsApp require `enabled: true` in config before they can be used.
3. **Custom providers are app-scoped** — registering a custom provider on one app has no effect on other apps.
4. **Preference defaults** — new channel preference fields (`SlackEnabled`, `DiscordEnabled`, `WhatsAppEnabled`) default to `nil`.  `nil` is treated as "enabled" (opt-out model) to avoid blocking delivery for users who haven't set preferences yet.
5. **`Channel.Valid()` still validates known channels** — custom channels bypass `Valid()` and are validated at delivery time.
6. **No index migration** — no new ES indices, no changes to existing index mappings.  New user/app fields are automatically handled by ES dynamic mapping.
7. **Worker backward compat** — a new worker binary gracefully ignores notifications targeting unknown channels (logs warning, fails with clear error message).

---

## Section 11: Implementation Order

Execute in this exact sequence.  Each step builds on the previous.

| Step | Task | Files | Depends On |
|------|------|-------|------------|
| 1 | Add channel constants (`ChannelSlack`, `ChannelDiscord`, `ChannelWhatsApp`) and update `Valid()` | `notification/models.go` | — |
| 2 | Add config structs for Slack, Discord, WhatsApp | `config/config.go`, `config/config.yaml` | — |
| 3 | Add user model fields (`SlackWebhookURL`, `DiscordWebhookURL`, preference toggles) | `user/models.go` | — |
| 4 | Add app model fields (`SlackConfig`, `DiscordConfig`, `CustomProviderConfig`, `CustomProviders`) | `application/models.go` | — |
| 5 | Implement Slack provider | `providers/slack_provider.go` | Step 1 |
| 6 | Implement Discord provider | `providers/discord_provider.go` | Step 1 |
| 7 | Implement WhatsApp provider | `providers/whatsapp_provider.go` | Step 1 |
| 8 | Implement Custom provider | `providers/custom_provider.go` | Step 4 |
| 9 | Create custom provider handler | `handlers/custom_provider_handler.go` | Step 4, 8 |
| 10 | Register Slack, Discord, WhatsApp providers in worker | `cmd/worker/main.go` | Steps 2, 5–7 |
| 11 | Add custom provider fallback in processor + update preference check | `cmd/worker/processor.go` | Steps 3, 8 |
| 12 | Register custom provider routes | `routes/routes.go` | Step 9 |
| 13 | Build verification: `go build ./...` + `go vet ./...` | — | All |

**Parallelism:** Steps 1–4 are independent and should be done together.  Steps 5–8 are independent of each other and can be done in parallel.  Steps 9–12 depend on prior steps but are lightweight.
