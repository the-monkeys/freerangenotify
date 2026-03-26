# Hybrid Channel Billing — Detailed Implementation Specification

> **Companion to**: [hybrid-channel-billing-architecture.md](./hybrid-channel-billing-architecture.md)
> **Scope**: Exact files, structs, functions, code diffs, and risk/mitigation per phase.

---

## Global Prerequisite: Feature Flag

All billing metering logic MUST be gated behind `FREERANGE_FEATURES_BILLING_ENABLED`.
When `false`, the system behaves identically to today — no usage events are emitted, no billing checks are performed.

### Changes Required

#### [MODIFY] `.env`
Add alongside existing feature flags (line ~63):
```diff
 FREERANGE_FEATURES_TRIAL_WELCOME_ENABLED=true
+FREERANGE_FEATURES_BILLING_ENABLED=false
```

#### [MODIFY] `internal/config/config.go`

**1. Add to `FeaturesConfig` struct (line ~60):**
```diff
 // Registration/Billing UX
 TrialWelcomeEnabled bool `mapstructure:"trial_welcome_enabled" yaml:"trial_welcome_enabled"`
+BillingEnabled      bool `mapstructure:"billing_enabled" yaml:"billing_enabled"`
```

**2. Add viper default (line ~378):**
```diff
 viper.SetDefault("features.trial_welcome_enabled", true)
+viper.SetDefault("features.billing_enabled", false)
```

**How to check in code:**
```go
if cfg.Features.BillingEnabled {
    // emit usage event, enforce quotas, etc.
}
```

> **Risk**: If someone deploys to production without setting `FREERANGE_FEATURES_BILLING_ENABLED=true`, billing silently does nothing.
> **Mitigation**: Default is `false` (safe). Add a startup log warning: `"WARN: Billing metering is DISABLED. Set FREERANGE_FEATURES_BILLING_ENABLED=true for production."`.

---

## Phase 1: Credential Source Tagging

**Goal**: Every provider's `Send()` method tags `Result.Metadata` with `credential_source` so downstream metering knows who paid the carrier.

### 1.1 Provider Interface — Add Constants

#### [MODIFY] `internal/infrastructure/providers/provider.go`
Add after line 71 (after error type constants):

```go
// Credential source constants for billing metering
const (
    CredSourceSystem   = "system"   // System .env credentials — we pay the carrier
    CredSourceBYOC     = "byoc"     // User's own credentials — they pay the carrier
    CredSourcePlatform = "platform" // No external cost (in-app, SSE, push)
)
```

### 1.2 WhatsApp Provider — Tag credential source

#### [MODIFY] `internal/infrastructure/providers/whatsapp_provider.go`
After the credential resolution block (line ~82), before the validation check:

```diff
+    // Tag credential source for billing metering
+    credSource := CredSourceSystem
     if appCfg, ok := ctx.Value(WhatsAppConfigKey).(*application.WhatsAppAppConfig); ok && appCfg != nil {
         if appCfg.AccountSID != "" && appCfg.AuthToken != "" {
             accountSID = appCfg.AccountSID
             authToken = appCfg.AuthToken
+            credSource = CredSourceBYOC
             ...
         }
     }
```

Then after successful send (line ~190), before returning result:
```diff
 result := NewResult(providerMsgID, time.Since(start))
+result.Metadata["credential_source"] = credSource
+result.Metadata["billing_channel"] = "whatsapp"
```

### 1.3 SMTP Provider — Tag credential source

#### [MODIFY] `internal/infrastructure/providers/smtp_provider.go`
After credential resolution (line ~106):

```diff
+    credSource := CredSourceSystem
     if cfg, ok := ctx.Value(EmailConfigKey).(*application.EmailConfig); ok && cfg != nil {
         if cfg.ProviderType == "smtp" && cfg.SMTP != nil {
             host = cfg.SMTP.Host
             ...
+            credSource = CredSourceBYOC
         }
     }
```

After successful send (line ~143):
```diff
 result := NewResult("smtp-"+notif.NotificationID, deliveryTime)
+result.Metadata["credential_source"] = credSource
+result.Metadata["billing_channel"] = "email"
```

### 1.4 SMS (Twilio) Provider — Add per-app override + tag

#### [MODIFY] `internal/infrastructure/providers/twilio_provider.go`
This provider currently has NO per-app credential override. We need to add one.

**Step 1**: Add context key to `provider.go`:
```diff
 const (
     EmailConfigKey    contextKey = "email_config"
     WhatsAppConfigKey contextKey = "whatsapp_config"
+    SMSConfigKey      contextKey = "sms_config"
 )
```

**Step 2**: Add `SMSAppConfig` to `internal/domain/application/models.go`:
```diff
+// SMSAppConfig holds per-app Twilio SMS credentials
+type SMSAppConfig struct {
+    AccountSID string `json:"account_sid" es:"account_sid"`
+    AuthToken  string `json:"auth_token" es:"auth_token"`
+    FromNumber string `json:"from_number" es:"from_number"`
+}
```

**Step 3**: Add `SMS` field to `Settings` struct in `application/models.go`:
```diff
 WhatsApp  *WhatsAppAppConfig `json:"whatsapp_config,omitempty" es:"whatsapp_config"`
+SMS       *SMSAppConfig      `json:"sms_config,omitempty" es:"sms_config"`
```

**Step 4**: In `twilio_provider.go` `Send()`, add credential resolution:
```go
credSource := CredSourceSystem
if appCfg, ok := ctx.Value(SMSConfigKey).(*application.SMSAppConfig); ok && appCfg != nil {
    if appCfg.AccountSID != "" && appCfg.AuthToken != "" {
        p.accountSID = appCfg.AccountSID  // use local vars, not mutate struct
        p.authToken = appCfg.AuthToken
        if appCfg.FromNumber != "" {
            p.fromNumber = appCfg.FromNumber
        }
        credSource = CredSourceBYOC
    }
}
// ... after successful send:
result.Metadata["credential_source"] = credSource
result.Metadata["billing_channel"] = "sms"
```

> **Risk**: Mutating the provider struct fields directly causes race conditions in concurrent sends.
> **Mitigation**: Use local variables (like WhatsApp provider already does), NOT struct field mutation.

### 1.5 Other Providers — Tag credential source

| Provider File | `credential_source` | `billing_channel` | Notes |
|---|---|---|---|
| `sendgrid_provider.go` | Add per-app override → `byoc`/`system` | `"email"` | Uses `EmailConfigKey` |
| `mailgun_provider.go` | Same pattern | `"email"` | |
| `postmark_provider.go` | Same pattern | `"email"` | |
| `resend_provider.go` | Same pattern | `"email"` | |
| `ses_provider.go` | Same pattern | `"email"` | |
| `vonage_provider.go` | Add per-app override → `byoc`/`system` | `"sms"` | New context key needed |
| `slack_provider.go` | Always `CredSourceBYOC` | `"slack"` | User always provides webhook |
| `discord_provider.go` | Always `CredSourceBYOC` | `"discord"` | User always provides webhook |
| `teams_provider.go` | Always `CredSourceBYOC` | `"teams"` | User always provides webhook |
| `webhook_provider.go` | Always `CredSourceBYOC` | `"webhook"` | User-defined URL |
| `fcm_provider.go` | `CredSourcePlatform` | `"push"` | FCM is free |
| `apns_provider.go` | `CredSourcePlatform` | `"push"` | APNS is free |
| `inapp_provider.go` | `CredSourcePlatform` | `"inapp"` | No external cost |
| `sse_provider.go` | `CredSourcePlatform` | `"sse"` | No external cost |

### 1.6 Manager — Post-send usage event emission

#### [MODIFY] `internal/infrastructure/providers/manager.go`

The `Manager.Send()` method must emit a usage event AFTER a successful send, only when `BillingEnabled` is true.

After the success log at line ~188:
```go
// Emit billing usage event (gated behind feature flag)
if m.billingEnabled && result != nil && result.Success {
    credSource, _ := result.Metadata["credential_source"].(string)
    billingChannel, _ := result.Metadata["billing_channel"].(string)
    if credSource != "" && billingChannel != "" {
        go m.emitUsageEvent(ctx, notif, credSource, billingChannel)
    }
}
```

**New field on Manager struct:**
```diff
 type Manager struct {
     providers      map[notification.Channel]Provider
     namedProviders map[string]Provider
     breakers       map[string]*CircuitBreaker
     metrics        *metrics.NotificationMetrics
     presenceRepo   user.PresenceRepository
+    billingEnabled bool
+    usageEmitter   billing.UsageEmitter  // interface, nil-safe
     logger         *zap.Logger
     mu             sync.RWMutex
 }
```

> **Risk**: `go m.emitUsageEvent()` goroutine could silently fail, losing usage data.
> **Mitigation**: `emitUsageEvent()` should retry 3× with exponential backoff. If all retries fail, log at ERROR level with full notification context for manual reconciliation.

---

## Phase 2: Usage Ledger & Metering Service

**Goal**: Create the domain model and Elasticsearch persistence for usage events.

### 2.1 Domain Model

#### [NEW] `internal/domain/billing/usage.go`

```go
package billing

import (
    "context"
    "time"
)

// UsageEvent represents a single billable action.
type UsageEvent struct {
    ID               string    `json:"id" es:"id"`
    TenantID         string    `json:"tenant_id" es:"tenant_id"`
    AppID            string    `json:"app_id" es:"app_id"`
    NotificationID   string    `json:"notification_id" es:"notification_id"`
    Channel          string    `json:"channel" es:"channel"`
    Provider         string    `json:"provider" es:"provider"`
    CredentialSource string    `json:"credential_source" es:"credential_source"`
    MessageType      string    `json:"message_type" es:"message_type"`
    CostUnit         float64   `json:"cost_unit" es:"cost_unit"`
    BilledAmount     float64   `json:"billed_amount" es:"billed_amount"`
    Currency         string    `json:"currency" es:"currency"`
    Status           string    `json:"status" es:"status"`
    Timestamp        time.Time `json:"timestamp" es:"timestamp"`
}

// UsageSummary aggregates usage over a billing period.
type UsageSummary struct {
    TenantID         string  `json:"tenant_id"`
    Channel          string  `json:"channel"`
    CredentialSource string  `json:"credential_source"`
    MessageCount     int64   `json:"message_count"`
    TotalCost        float64 `json:"total_cost"`
    PeriodStart      string  `json:"period_start"`
    PeriodEnd        string  `json:"period_end"`
}

// UsageEmitter writes usage events to the ledger.
type UsageEmitter interface {
    Emit(ctx context.Context, event *UsageEvent) error
}

// UsageRepository reads/aggregates usage data.
type UsageRepository interface {
    Store(ctx context.Context, event *UsageEvent) error
    GetSummary(ctx context.Context, tenantID string, periodStart, periodEnd time.Time) ([]UsageSummary, error)
    GetBreakdown(ctx context.Context, tenantID string, periodStart, periodEnd time.Time) ([]UsageEvent, error)
}
```

### 2.2 Elasticsearch Repository

#### [NEW] `internal/infrastructure/billing/es_usage_repo.go`

Follows the same pattern as existing ES repositories (`internal/infrastructure/elasticsearch/`):
- Index name: `freerange_usage_events`
- Uses bulk indexing for high-throughput
- Aggregation queries for `GetSummary()` using terms + sum aggs

### 2.3 Index Bootstrap

#### [MODIFY] `internal/infrastructure/elasticsearch/indices.go`

Add `freerange_usage_events` index creation in the `EnsureIndices()` function with the mapping defined in the architecture doc.

> **Risk**: High write volume on system creds could create ES hot spots.
> **Mitigation**: Use `_routing` by `tenant_id` and ILM (Index Lifecycle Management) to auto-rollover at 50GB/30 days.

---

## Phase 3: Billing Calculation Engine

**Goal**: Compute per-tenant invoices based on usage events, plan tiers, and credential source.

### 3.1 Rate Configuration

#### [NEW] `internal/domain/billing/rates.go`

```go
package billing

// ChannelRate defines the cost/charge for a single channel+credential combination.
type ChannelRate struct {
    Channel          string  `json:"channel"`
    CredentialSource string  `json:"credential_source"`
    CostPerUnit      float64 `json:"cost_per_unit"`   // our carrier cost (INR)
    PricePerUnit     float64 `json:"price_per_unit"`   // what we charge user (INR)
    Currency         string  `json:"currency"`
}

// PlanTier defines included quotas and overage rates per plan.
type PlanTier struct {
    Name            string              `json:"name"`
    MonthlyFeeINR   float64             `json:"monthly_fee_inr"`
    IncludedQuotas  map[string]int64    `json:"included_quotas"`   // channel -> count
    OverageRates    map[string]float64  `json:"overage_rates"`     // channel -> INR/msg
    BYOCPlatformFee map[string]float64  `json:"byoc_platform_fee"` // channel -> INR/msg
}
```

### 3.2 Calculator Service

#### [NEW] `internal/domain/billing/calculator.go`

Core function signature:
```go
func (c *Calculator) ComputeInvoice(
    ctx context.Context,
    tenantID string,
    plan PlanTier,
    usage []UsageSummary,
) (*Invoice, error)
```

Logic:
1. For each channel in `usage`, check `credential_source`
2. If `system` → count against `plan.IncludedQuotas[channel]`, charge `OverageRates` for excess
3. If `byoc` → charge `BYOCPlatformFee[channel]` per message
4. If `platform` → charge small platform fee (push, in-app)
5. Sum all → `Invoice.TotalAmount`

> **Risk**: Float precision errors accumulate over millions of messages.
> **Mitigation**: Use integer arithmetic in paisa (1 INR = 100 paisa). Store all amounts as `int64` paisa, convert to INR only for display.

---

## Phase 4: UI Integration

### 4.1 Billing Breakdown Component

#### [MODIFY] `ui/src/pages/WorkspaceBilling.tsx`

Add a new card or expandable section below the existing 3-card grid:
- Per-channel usage table: Email, WhatsApp, SMS, Push
- Columns: Channel | Credential Mode | Messages Sent | Cost (₹)
- Color-coded: green for BYOC, blue for system, grey for platform

### 4.2 App Settings Credential Management

#### [MODIFY] `ui/src/pages/AppDetail.tsx`

In the app settings panel, add sections for:
- **WhatsApp Credentials**: AccountSID, AuthToken, FromNumber fields
- **SMS Credentials**: Same pattern
- **Validate button**: Calls `POST /v1/apps/:id/credentials/validate`
- Show badge: "Using System Credentials" or "Using Your Credentials"

### 4.3 New API Endpoint for Usage Breakdown

#### [NEW] Handler: `internal/interfaces/http/handlers/billing_handler.go`

Add `GetUsageBreakdown` method:
```go
// GetUsageBreakdown handles GET /v1/billing/usage/breakdown
func (h *BillingHandler) GetUsageBreakdown(c *fiber.Ctx) error {
    if !h.billingEnabled {
        return c.JSON(fiber.Map{"billing_enabled": false})
    }
    // ... aggregate from usage repo, return per-channel-per-cred-source breakdown
}
```

#### [MODIFY] `internal/interfaces/http/routes/routes.go`

```diff
 billing := v1.Group("/billing", jwtMiddleware)
 billing.Get("/usage", billingHandler.GetUsage)
 billing.Get("/subscription", billingHandler.GetSubscription)
 billing.Post("/accept-trial", billingHandler.AcceptTrial)
+billing.Get("/usage/breakdown", billingHandler.GetUsageBreakdown)
+billing.Get("/rates", billingHandler.GetRates)
```

### 4.4 Frontend API Service

#### [MODIFY] `ui/src/services/api.ts`

```diff
 export const billingAPI = {
   getUsage: async () => { ... },
   getSubscription: async () => { ... },
   acceptTrial: async () => { ... },
+  getUsageBreakdown: async () => {
+    const { data } = await api.get('/billing/usage/breakdown');
+    return data;
+  },
+  getRates: async () => {
+    const { data } = await api.get('/billing/rates');
+    return data;
+  },
 };
```

> **Risk**: Usage breakdown query on large datasets is slow.
> **Mitigation**: Pre-aggregate daily summaries via a nightly cron. The breakdown API reads from pre-computed summaries, not raw events.

---

## Phase 5: Edge Case Hardening

### 5.1 Prevent System-Cred-as-BYOC

#### [MODIFY] Wherever per-app credentials are saved (app update handler)

```go
// In the app settings update handler:
func validateNotSystemCreds(appCreds, systemCreds string) error {
    if appCreds == systemCreds {
        return fmt.Errorf("cannot use system credentials as custom credentials")
    }
    return nil
}
```

Compare `app.Settings.WhatsApp.AccountSID` against `cfg.Providers.WhatsApp.AccountSID` from config.
Do the same for Email, SMS.

### 5.2 No Silent Fallback

#### [MODIFY] `internal/infrastructure/providers/whatsapp_provider.go` (and all others)

If BYOC was configured (context key exists with non-nil value) but credentials are empty/invalid:
```go
if appCfg != nil && (appCfg.AccountSID == "" || appCfg.AuthToken == "") {
    return NewErrorResult(
        fmt.Errorf("custom WhatsApp credentials are configured but incomplete; message not sent"),
        ErrorTypeConfiguration,
    ), nil
    // Do NOT fall through to system creds
}
```

### 5.3 Per-Tenant Rate Limiting on System Creds

#### [NEW] `internal/infrastructure/billing/rate_limiter.go`

Redis-based sliding window rate limiter keyed by `system:{tenant_id}:{channel}`:
```go
func (rl *SystemCredRateLimiter) Allow(ctx context.Context, tenantID, channel string) (bool, error) {
    key := fmt.Sprintf("billing:ratelimit:system:%s:%s", tenantID, channel)
    // Redis INCR + EXPIRE with sliding window
}
```

Default limits (configurable via plan tier):
| Channel | System Cred Rate Limit |
|---------|----------------------|
| Email | 500/hour/tenant |
| WhatsApp | 100/hour/tenant (Meta limit) |
| SMS | 100/hour/tenant |

### 5.4 Subscription Validity at Send Time

#### [MODIFY] `internal/infrastructure/providers/manager.go`

At the top of `Manager.Send()`, before provider resolution:
```go
if m.billingEnabled && credSource == CredSourceSystem {
    // Check subscription is still valid
    valid, err := m.subscriptionChecker.IsValid(ctx, tenantID)
    if !valid {
        return NewErrorResult(
            fmt.Errorf("subscription expired; cannot send via system credentials"),
            ErrorTypeAuth,
        ), nil
    }
}
```

> **Risk**: This adds latency to every send.
> **Mitigation**: Cache subscription validity in Redis with 60s TTL. The checker reads from cache first.

---

## Phase 6: Testing Strategy

**Goal**: Validate every phase with unit tests, integration tests, and end-to-end tests. Tests must verify both `BILLING_ENABLED=true` AND `BILLING_ENABLED=false` paths.

### 6.1 Unit Tests (per phase)

#### Phase 0 — Feature Flag

#### [NEW] `internal/config/config_test.go` (add test cases)

```go
func TestBillingFeatureFlag_DefaultFalse(t *testing.T) {
    // Ensure billing defaults to false when env var not set
    cfg, err := config.Load()
    require.NoError(t, err)
    assert.False(t, cfg.Features.BillingEnabled)
}

func TestBillingFeatureFlag_EnabledFromEnv(t *testing.T) {
    t.Setenv("FREERANGE_FEATURES_BILLING_ENABLED", "true")
    cfg, err := config.Load()
    require.NoError(t, err)
    assert.True(t, cfg.Features.BillingEnabled)
}
```

#### Phase 1 — Credential Source Tagging

#### [NEW] `internal/infrastructure/providers/whatsapp_provider_test.go`

```go
func TestWhatsAppProvider_TagsSystemCreds(t *testing.T) {
    // Send with no per-app config in context → result.Metadata["credential_source"] == "system"
}

func TestWhatsAppProvider_TagsBYOCCreds(t *testing.T) {
    // Inject WhatsAppAppConfig into context → result.Metadata["credential_source"] == "byoc"
}

func TestWhatsAppProvider_FailsOnIncompleteBYOC(t *testing.T) {
    // WhatsAppAppConfig with empty AuthToken → ErrorTypeConfiguration, NOT fallback to system
}
```

#### [NEW] `internal/infrastructure/providers/twilio_provider_billing_test.go`

```go
func TestTwilioProvider_TagsSystemCreds(t *testing.T) {
    // No SMSConfigKey in context → credential_source == "system"
}

func TestTwilioProvider_TagsBYOCCreds(t *testing.T) {
    // SMSAppConfig in context → credential_source == "byoc"
}

func TestTwilioProvider_LocalVarsNotMutated(t *testing.T) {
    // Concurrent sends with different per-app configs → no race, no cross-contamination
    // Run with: go test -race ./internal/infrastructure/providers/ -run TestTwilioProvider_LocalVars
}
```

Same pattern for: `smtp_provider_test.go`, `sendgrid_provider_test.go`, etc.

#### Phase 3 — Calculator

#### [NEW] `internal/domain/billing/calculator_test.go`

```go
func TestCalculator_SystemCredWithinQuota(t *testing.T) {
    // 100 system emails on Starter plan (5000 included) → billed ₹0 overage
}

func TestCalculator_SystemCredOverQuota(t *testing.T) {
    // 6000 system emails on Starter plan (5000 included) → 1000 × ₹0.15 = ₹150 overage
}

func TestCalculator_BYOCPlatformFeeOnly(t *testing.T) {
    // 10,000 BYOC WhatsApp msgs → 10000 × ₹0.02 = ₹200 platform fee, ₹0 carrier fee
}

func TestCalculator_HybridScenario(t *testing.T) {
    // 3000 system emails + 5000 BYOC WhatsApp → split billing correctly
}

func TestCalculator_PaisaPrecision(t *testing.T) {
    // 1,000,000 messages at ₹0.01 each → verify no float drift, exact ₹10,000.00
}
```

#### Phase 5 — Edge Cases

#### [NEW] `internal/infrastructure/billing/rate_limiter_test.go`

```go
func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
    // 50 requests with limit 100 → all allowed
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
    // 101st request with limit 100 → blocked
}

func TestRateLimiter_SlidingWindowResets(t *testing.T) {
    // After window expires, new requests allowed
}
```

#### [NEW] `internal/interfaces/http/handlers/billing_handler_test.go`

```go
func TestValidateNotSystemCreds_Rejects(t *testing.T) {
    // App AccountSID == System AccountSID → error
}

func TestValidateNotSystemCreds_AllowsDifferent(t *testing.T) {
    // App AccountSID != System AccountSID → no error
}
```

---

### 6.2 Integration Tests

These test the full pipeline from API request → provider dispatch → usage event emission → ES storage.

#### [NEW] `tests/integration/billing_integration_test.go`

**Prerequisites**: Running Docker Compose stack (Elasticsearch + Redis + notification-service).

```go
// TestIntegration_SystemCredEmail_EmitsUsageEvent
// 1. Create test app (no custom email config)
// 2. Send notification via POST /v1/apps/:id/notifications (channel: email)
// 3. Wait 2 seconds for async processing
// 4. Query ES index freerange_usage_events with filter: notification_id = X
// 5. Assert: credential_source == "system", billing_channel == "email"

// TestIntegration_BYOCWhatsApp_EmitsUsageEvent
// 1. Create test app with Settings.WhatsApp = { AccountSID: "AC_test", AuthToken: "...", FromNumber: "..." }
// 2. Send WhatsApp notification
// 3. Query ES usage events
// 4. Assert: credential_source == "byoc", billing_channel == "whatsapp"

// TestIntegration_HybridBilling_SplitCorrectly
// 1. Create app with BYOC WhatsApp + system email
// 2. Send 1 email + 1 WhatsApp notification
// 3. Verify 2 usage events: one "system"+"email", one "byoc"+"whatsapp"

// TestIntegration_BillingDisabled_NoUsageEvents
// 1. Set FREERANGE_FEATURES_BILLING_ENABLED=false
// 2. Send notification
// 3. Query ES → assert 0 usage events emitted

// TestIntegration_SystemCredAsBoYC_Rejected
// 1. Read system WhatsApp AccountSID from config
// 2. Attempt PUT /v1/apps/:id with Settings.WhatsApp.AccountSID = system SID
// 3. Assert: 400 Bad Request with "cannot use system credentials"

// TestIntegration_ExpiredSubscription_BlocksSystemCred
// 1. Create user with expired subscription (current_period_end in the past)
// 2. Attempt to send via system creds
// 3. Assert: send fails with "subscription expired"

// TestIntegration_UsageBreakdownAPI
// 1. Seed 50 usage events: 30 system email, 20 BYOC whatsapp
// 2. GET /v1/billing/usage/breakdown
// 3. Assert response contains correct channel+cred_source groupings with accurate counts

// TestIntegration_RateLimiter_BlocksExcessSystemCred
// 1. Set system cred rate limit to 5/hour for email
// 2. Send 6 emails via system creds in rapid succession
// 3. Assert: 5 succeed, 6th fails with rate limit error
```

#### Running Integration Tests

```bash
# From project root, with Docker Compose stack running:
FREERANGE_FEATURES_BILLING_ENABLED=true \
  go test -v -tags=integration -timeout 120s ./tests/integration/...
```

#### [NEW] `tests/integration/setup_test.go`

Test suite setup/teardown:
```go
func TestMain(m *testing.M) {
    // 1. Start Docker Compose if not running (or skip if CI)
    // 2. Wait for ES + Redis health checks
    // 3. Create test-specific ES indices with unique prefix
    // 4. Run tests
    // 5. Cleanup: delete test indices
    os.Exit(m.Run())
}
```

---

### 6.3 End-to-End (E2E) Browser Tests

These validate the full user journey through the UI.

#### [NEW] `tests/e2e/billing_e2e_test.md` (test plan for browser subagent)

| # | Test Case | Steps | Expected Result |
|---|-----------|-------|-----------------|
| 1 | **Billing page shows plan** | Login → Navigate to `/billing` | Shows plan name, days remaining, usage stats |
| 2 | **Sidebar badge visible** | Login → Check sidebar | "Billing & Licensing" has `X days` badge |
| 3 | **Usage breakdown renders** | Login → `/billing` → scroll to breakdown | Per-channel table with System/BYOC columns |
| 4 | **BYOC creds save** | Go to app settings → enter WhatsApp creds → Save | Success toast, badge changes to "Your Credentials" |
| 5 | **BYOC validation error** | Enter invalid AccountSID → click Validate | Error message: "Invalid credentials" |
| 6 | **System-cred-as-BYOC blocked** | Copy system AccountSID into app settings → Save | Error: "Cannot use system credentials" |
| 7 | **Billing disabled hides metrics** | Set `BILLING_ENABLED=false` → rebuild → `/billing` | Shows plan info but no usage breakdown table |

---

### 6.4 Test File Index

| Phase | Test File | Type | # Tests |
|-------|-----------|------|---------|
| 0 | `internal/config/config_test.go` | Unit | 2 |
| 1 | `internal/infrastructure/providers/whatsapp_provider_test.go` | Unit | 3 |
| 1 | `internal/infrastructure/providers/twilio_provider_billing_test.go` | Unit | 3 |
| 1 | `internal/infrastructure/providers/smtp_provider_test.go` | Unit | 3 |
| 3 | `internal/domain/billing/calculator_test.go` | Unit | 5 |
| 5 | `internal/infrastructure/billing/rate_limiter_test.go` | Unit | 3 |
| 5 | `internal/interfaces/http/handlers/billing_handler_test.go` | Unit | 2 |
| ALL | `tests/integration/billing_integration_test.go` | Integration | 8 |
| ALL | `tests/e2e/billing_e2e_test.md` | E2E (manual) | 7 |
| | | **Total** | **36** |

---

## Risk Summary Table

| Phase | Risk | Severity | Mitigation | Owner |
|-------|------|----------|------------|-------|
| 0 | Billing flag forgotten in prod | Medium | Default `false`, startup warning log | DevOps |
| 1 | Race condition in SMS provider creds | High | Use local vars, never mutate struct | Backend |
| 1 | Usage event lost (goroutine fails) | Medium | 3× retry + ERROR log for reconciliation | Backend |
| 2 | ES hot spots from high write volume | Medium | `_routing` by tenant + ILM rollover | Infra |
| 3 | Float precision on millions of msgs | High | Use int64 paisa arithmetic | Backend |
| 4 | Slow breakdown queries | Low | Pre-aggregate daily summaries | Backend |
| 5 | System-cred-as-BYOC abuse | High | Compare SID at save time, reject | Backend |
| 5 | Silent fallback billing leak | Critical | Fail-closed when BYOC configured but empty | Backend |
| 5 | One tenant exhausts system rate limit | High | Redis sliding window per-tenant per-channel | Backend |
| 5 | Subscription check latency | Low | 60s Redis cache | Backend |

---

## File Change Index

| Phase | File | Action | Lines Affected |
|-------|------|--------|---------------|
| 0 | `.env` | MODIFY | +1 line |
| 0 | `internal/config/config.go` | MODIFY | +2 lines (struct + default) |
| 1 | `internal/infrastructure/providers/provider.go` | MODIFY | +6 lines (constants + context key) |
| 1 | `internal/infrastructure/providers/whatsapp_provider.go` | MODIFY | +5 lines |
| 1 | `internal/infrastructure/providers/smtp_provider.go` | MODIFY | +5 lines |
| 1 | `internal/infrastructure/providers/twilio_provider.go` | MODIFY | +20 lines (override + tag) |
| 1 | `internal/infrastructure/providers/manager.go` | MODIFY | +15 lines (emitter) |
| 1 | All other 11 providers | MODIFY | +2-3 lines each |
| 1 | `internal/domain/application/models.go` | MODIFY | +6 lines (SMSAppConfig) |
| 2 | `internal/domain/billing/usage.go` | NEW | ~50 lines |
| 2 | `internal/infrastructure/billing/es_usage_repo.go` | NEW | ~150 lines |
| 2 | `internal/infrastructure/elasticsearch/indices.go` | MODIFY | +20 lines |
| 3 | `internal/domain/billing/rates.go` | NEW | ~40 lines |
| 3 | `internal/domain/billing/calculator.go` | NEW | ~120 lines |
| 4 | `ui/src/pages/WorkspaceBilling.tsx` | MODIFY | +60 lines |
| 4 | `ui/src/pages/AppDetail.tsx` | MODIFY | +40 lines |
| 4 | `ui/src/services/api.ts` | MODIFY | +10 lines |
| 4 | `internal/interfaces/http/routes/routes.go` | MODIFY | +2 lines |
| 4 | `internal/interfaces/http/handlers/billing_handler.go` | MODIFY | +50 lines |
| 5 | `internal/infrastructure/billing/rate_limiter.go` | NEW | ~60 lines |
| 5 | Multiple provider files | MODIFY | +5-10 lines each |
