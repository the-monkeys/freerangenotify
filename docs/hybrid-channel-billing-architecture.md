# Hybrid Channel Billing Architecture
## System Credentials vs BYOC (Bring Your Own Credentials)

### Problem Statement

FreeRangeNotify supports multiple delivery channels (Email, WhatsApp, SMS, Push, Slack, Discord, Teams, Webhooks). Users can either:

1. **Use system credentials** (from `.env`) — FreeRangeNotify pays the carrier cost → **must charge the user**.
2. **Bring their own credentials (BYOC)** — User pays the carrier directly → **charge platform fee only**.
3. **Hybrid** — Some channels on system creds, others on BYOC → **per-channel billing split**.

Getting this wrong causes direct financial loss at scale. This document covers the full architecture, pricing model, edge cases, and risk mitigation strategy.

---

## 1. Current Architecture Snapshot

### Credential Resolution (Already Implemented)

| Channel | System Creds | Per-App Override | Context Key |
|---------|-------------|-----------------|-------------|
| Email (SMTP) | ✅ `.env` | ✅ `EmailConfigKey` | `email_config` |
| WhatsApp | ✅ `.env` | ✅ `WhatsAppConfigKey` | `whatsapp_config` |
| SMS (Twilio) | ✅ `.env` | ❌ **Not yet** | — |
| Discord | ✅ `.env` | ❌ | — |
| Slack | ✅ `.env` | ❌ | — |
| Teams | ✅ `.env` | ❌ | — |
| Push (FCM/APNS) | ✅ `.env` | ❌ | — |
| Webhook/Custom | N/A (user-defined URL) | N/A | — |
| In-App / SSE | N/A (no external cost) | N/A | — |

### Key Observation  
The `Provider.Send()` → `Result.Metadata` pipeline already exists but does **not** tag whether system or user credentials were used. This is the critical gap.

---

## 2. Proposed Architecture

### 2.1 Credential Source Tagging

Every provider's `Send()` method must set a `credential_source` field in `Result.Metadata` **before returning**:

```go
// After credential resolution in every provider Send()
result.Metadata["credential_source"] = "system"  // or "byoc"
result.Metadata["billing_channel"] = "whatsapp"   // canonical channel name
```

**Decision logic** (inside each provider's `Send()`):

```
IF per-app config exists in context AND has non-empty AccountSID/AuthToken:
    credential_source = "byoc"
ELSE IF system .env creds are used:
    credential_source = "system"
ELSE:
    credential_source = "none" (should not reach send)
```

### 2.2 Metering Pipeline

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐     ┌──────────────┐
│ Notification │────▸│   Provider   │────▸│  Result with    │────▸│   Metering   │
│   Worker     │     │   .Send()    │     │  credential_src │     │   Service    │
└─────────────┘     └──────────────┘     └─────────────────┘     └──────┬───────┘
                                                                        │
                                                              ┌─────────▼─────────┐
                                                              │  Usage Ledger     │
                                                              │  (Elasticsearch)  │
                                                              │                   │
                                                              │ • tenant_id       │
                                                              │ • app_id          │
                                                              │ • channel         │
                                                              │ • cred_source     │
                                                              │ • message_type    │
                                                              │ • cost_unit       │
                                                              │ • timestamp       │
                                                              └───────────────────┘
```

**After every successful `Send()`**, the notification worker records a usage event:

```go
type UsageEvent struct {
    TenantID         string    `json:"tenant_id"`
    AppID            string    `json:"app_id"`
    Channel          string    `json:"channel"`          // "email", "whatsapp", "sms"
    CredentialSource string    `json:"credential_source"` // "system" | "byoc"
    MessageType      string    `json:"message_type"`      // "marketing", "utility", "transactional"
    CostUnit         float64   `json:"cost_unit"`         // our cost in INR for this message
    BilledAmount     float64   `json:"billed_amount"`     // what we charge the user in INR
    Timestamp        time.Time `json:"timestamp"`
}
```

### 2.3 Provider Changes Required

| Provider | Change Needed |
|----------|--------------|
| `smtp_provider.go` | Tag `credential_source` in `Result.Metadata` after credential resolution (line ~96) |
| `whatsapp_provider.go` | Tag `credential_source` in `Result.Metadata` after credential resolution (line ~70) |
| `twilio_provider.go` (SMS) | Add per-app credential override (similar to WhatsApp), then tag |
| `sendgrid_provider.go` | Add per-app override + tag |
| `mailgun_provider.go` | Add per-app override + tag |
| `postmark_provider.go` | Add per-app override + tag |
| `resend_provider.go` | Add per-app override + tag |
| `ses_provider.go` | Add per-app override + tag |
| `vonage_provider.go` | Add per-app override + tag |
| `slack_provider.go` | Always BYOC (user provides webhook) → `credential_source = "byoc"` |
| `discord_provider.go` | Always BYOC (user provides webhook) → `credential_source = "byoc"` |
| `teams_provider.go` | Always BYOC (user provides webhook) → `credential_source = "byoc"` |
| `webhook_provider.go` | Always BYOC → `credential_source = "byoc"` |
| `inapp_provider.go` | No external cost → `credential_source = "platform"` |
| `sse_provider.go` | No external cost → `credential_source = "platform"` |
| `fcm_provider.go` | Add per-app override + tag |
| `apns_provider.go` | Add per-app override + tag |

---

## 3. Pricing Model for India

### 3.1 Pricing Philosophy

| Credential Mode | What We Charge | Why |
|-----------------|---------------|-----|
| **System Creds** | Carrier cost + markup + platform fee | We bear the carrier cost, need margin |
| **BYOC** | Platform fee only | User pays carrier directly, we charge for infrastructure |
| **Hybrid** | Per-channel split | Each channel billed based on its own credential mode |

### 3.2 Carrier Cost Reference (India, 2026)

| Channel | Carrier Cost (our cost) | Source |
|---------|------------------------|--------|
| **WhatsApp Marketing** | ₹1.09/msg | Meta per-message pricing (Jan 2026) |
| **WhatsApp Utility** | ₹0.145/msg | Meta per-message pricing (Jan 2026) |
| **WhatsApp Authentication** | ₹0.145/msg | Meta per-message pricing (Jan 2026) |
| **WhatsApp Service** | Free (within 24h window) | Meta policy |
| **SMS (Twilio, India)** | ~₹0.17/msg ($0.002 × ₹85) | Twilio India rate |
| **Email (SMTP/SES)** | ~₹0.08/email ($0.001/email on SES) | AWS SES pricing |
| **Email (SendGrid)** | ~₹0.04–₹0.08/email | SendGrid free tier + overage |
| **Push (FCM)** | Free | Google FCM is free |
| **Push (APNS)** | Free | Apple APNS is free |

### 3.3 Proposed FreeRangeNotify Pricing (India)

#### Tier Structure

| Plan | Monthly Fee (₹) | Included System-Cred Messages | Platform Fee per BYOC msg |
|------|-----------------|-------------------------------|--------------------------|
| **Free Trial** | ₹0 (30 days) | 500 emails, 50 WhatsApp, 50 SMS | ₹0 |
| **Starter** | ₹999/mo | 5,000 emails, 500 WhatsApp, 500 SMS | ₹0.01/msg |
| **Growth** | ₹4,999/mo | 25,000 emails, 2,500 WhatsApp, 2,500 SMS | ₹0.008/msg |
| **Scale** | ₹14,999/mo | 100,000 emails, 10,000 WhatsApp, 10,000 SMS | ₹0.005/msg |
| **Enterprise** | Custom | Custom | Custom |

#### System-Credentials Overage Rates (per message beyond included)

| Channel | Overage Rate (₹) | Our Cost (₹) | Margin |
|---------|------------------|--------------|--------|
| **Email** | ₹0.15/email | ~₹0.08 | ~47% |
| **WhatsApp (Utility)** | ₹0.25/msg | ₹0.145 | ~42% |
| **WhatsApp (Marketing)** | ₹1.80/msg | ₹1.09 | ~39% |
| **SMS** | ₹0.30/msg | ~₹0.17 | ~43% |
| **Push** | ₹0.02/push | ₹0 | 100% (infra only) |

#### BYOC Pricing (platform fee only)

| Channel | Platform Fee (₹) |
|---------|------------------|
| **Email (own SMTP/SES/SendGrid)** | ₹0.01/email |
| **WhatsApp (own Twilio)** | ₹0.02/msg |
| **SMS (own Twilio/Vonage)** | ₹0.02/msg |
| **Push (own FCM/APNS keys)** | ₹0.005/push |
| **Webhook/Slack/Discord/Teams** | Included in plan |

---

## 4. Edge Cases & Risk Vectors

### 4.1 Critical Edge Cases

| # | Edge Case | Risk | Mitigation |
|---|-----------|------|------------|
| 1 | **User sets BYOC creds but they're invalid** | Message fails, user blames platform | Validate credentials on save (test API call). Show clear error: "Your WhatsApp credentials failed. Message was not sent." |
| 2 | **User switches from system to BYOC mid-billing-cycle** | Already-queued messages use old creds, billing mismatch | Credential resolution happens at **send time**, not queue time. The `Send()` reads context at execution. |
| 3 | **User provides BYOC creds then removes them** | Messages fall back to system creds silently → we pay, user isn't charged | **NEVER silently fall back.** If BYOC was configured but creds are now empty, fail the message with `ErrorTypeConfiguration` and notify user. |
| 4 | **Rate limit exhaustion on system creds** | One heavy user exhausts Twilio rate limits affecting all users | Per-tenant rate limiting on system creds. Implement `RateLimiter` in the manager keyed by `credential_source + tenant_id`. |
| 5 | **User sets system creds as "BYOC"** | User copies our `.env` creds into their app settings → free-rides system creds | Compare BYOC `AccountSID` against system `.env` `AccountSID`. If they match → reject with "Cannot use system credentials as custom credentials." |
| 6 | **WhatsApp template not approved** | Meta rejects template, Twilio returns 63016 error | Parse Twilio error codes, surface to user: "Your WhatsApp template was not approved by Meta." |
| 7 | **Hybrid user exceeds only one channel's quota** | User has 5,000 emails (system) + unlimited WhatsApp (BYOC). Hits email overage but not WhatsApp. | Per-channel-per-credential-source metering. Each channel tracked independently. |
| 8 | **Currency fluctuation** | USD carrier costs change but INR pricing is fixed | Review carrier costs quarterly. Build 15-20% buffer into margins. |
| 9 | **Bulk send via CSV (batch upload)** | 10,000 messages queued → system creds overwhelmed | Batch sends must respect per-tenant rate limits. Queue with backpressure. System cred batches capped at 100/min for WhatsApp (Meta limit). |
| 10 | **User has org-level creds AND app-level creds** | Which takes precedence? | Resolution order: **App-level > Tenant-level > System-level**. Document clearly. |
| 11 | **Subscription expires mid-batch** | 5,000 of 10,000 messages sent, subscription expires | Check subscription validity **per message** at send time, not at queue time. Remaining messages fail gracefully with "Subscription expired." |
| 12 | **Free trial abuse** | User creates multiple accounts for unlimited free trials | Rate limit by IP + email domain. Flag accounts with disposable email domains. |
| 13 | **Credential rotation** | User updates Twilio creds while messages are in-flight | Credentials are read from DB at send time (not cached at queue time). Rotation is safe. |

### 4.2 Security Risks

| Risk | Mitigation |
|------|------------|
| User credentials stored in plain text | Encrypt BYOC credentials at rest using AES-256. Decrypt only at send time. |
| System `.env` credentials exposed via API | Never return system credentials in any API response. BYOC creds shown masked (`AC****...1234`). |
| Credential validation endpoint becomes an oracle | Rate-limit credential validation to 3 attempts/hour per user. |

---

## 5. Implementation Phases

### Phase 1: Credential Source Tagging (Backend)
- Add `credential_source` field to `Result.Metadata` in all providers
- Add per-app credential override to SMS (`twilio_provider.go`)
- Add `UsageEvent` struct and ES index
- Modify notification worker to emit `UsageEvent` after each successful send

### Phase 2: Usage Ledger & Metering
- Create `internal/domain/billing/usage.go` with `UsageEvent` and `UsageSummary`
- Create `internal/infrastructure/billing/es_usage_repo.go` for ES-backed usage storage
- Build aggregation queries: per-tenant, per-channel, per-credential-source, per-period

### Phase 3: Billing Calculation Engine
- Create `internal/domain/billing/calculator.go`
- Implement tiered pricing logic with per-channel overage rates
- Differentiate system vs BYOC costs
- Generate monthly invoices

### Phase 4: UI Integration
- Extend `WorkspaceBilling.tsx` to show per-channel usage breakdown
- Show system vs BYOC split with cost attribution
- Add credential management UI in app settings (validate on save)
- Add overage alerts and budget caps

### Phase 5: Edge Case Hardening
- Implement credential similarity check (prevent system-cred-as-BYOC)
- Add per-tenant rate limiting on system creds
- Add subscription validity check at send time
- Implement credential encryption at rest

---

## 6. Database Schema (Elasticsearch)

### New Index: `frn_usage_events`

```json
{
  "mappings": {
    "properties": {
      "tenant_id":         { "type": "keyword" },
      "app_id":            { "type": "keyword" },
      "notification_id":   { "type": "keyword" },
      "channel":           { "type": "keyword" },
      "provider":          { "type": "keyword" },
      "credential_source": { "type": "keyword" },
      "message_type":      { "type": "keyword" },
      "cost_unit":         { "type": "float" },
      "billed_amount":     { "type": "float" },
      "currency":          { "type": "keyword" },
      "status":            { "type": "keyword" },
      "timestamp":         { "type": "date" }
    }
  }
}
```

### Aggregation Query: Monthly Invoice

```json
{
  "aggs": {
    "by_channel": {
      "terms": { "field": "channel" },
      "aggs": {
        "by_cred_source": {
          "terms": { "field": "credential_source" },
          "aggs": {
            "total_cost": { "sum": { "field": "billed_amount" } },
            "message_count": { "value_count": { "field": "notification_id" } }
          }
        }
      }
    }
  }
}
```

---

## 7. API Surface Changes

### New Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/billing/usage/breakdown` | Per-channel, per-credential-source usage for current period |
| `GET` | `/v1/billing/invoice/:period` | Generated invoice for a billing period |
| `POST` | `/v1/apps/:id/credentials/validate` | Validate BYOC credentials before saving |
| `GET` | `/v1/billing/rates` | Current pricing rates for all channels |

### Modified Endpoints

| Method | Path | Change |
|--------|------|--------|
| `PUT` | `/v1/apps/:id/settings` | Add credential encryption + similarity check |

---

## 8. Verification Plan

### Automated Tests
- Unit tests for `credential_source` tagging in every provider
- Unit tests for billing calculator with hybrid scenarios
- Integration test: system cred email + BYOC WhatsApp in same notification batch

### Manual Verification
- Create test account, send WhatsApp via system creds → verify usage event tagged `system`
- Configure BYOC Twilio creds on app → send WhatsApp → verify tagged `byoc`
- Attempt to set system AccountSID as BYOC → verify rejection
- Exhaust free tier quota → verify overage billing kicks in
- Remove BYOC creds mid-cycle → verify messages fail (no silent fallback)
