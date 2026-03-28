# Razorpay Payment Gateway Integration — Detailed Implementation Specification

> **Companion to**: [hybrid-billing-implementation-spec.md](./hybrid-billing-implementation-spec.md), [BILLING_SUBSCRIPTION_PLAN.md](../local/nonstaged_2026-03-16/BILLING_SUBSCRIPTION_PLAN.md)
> **Scope**: Razorpay integration, subscription renewal (payment + `frn` CLI), secrets management, exact code diffs, and verification plan.

---

## Overview

FreeRangeNotify uses a hybrid billing model where users get a free trial, then must pay to continue using system credentials. This spec adds **Razorpay** as the production payment gateway, replacing the current `MockProvider`. It covers:

1. **Razorpay Order Creation** — Backend creates a Razorpay Order when user initiates checkout
2. **Client-Side Payment** — Frontend opens Razorpay Checkout modal (no redirect)
3. **Payment Verification** — Backend verifies payment signature server-side
4. **Webhook Handling** — Razorpay webhook for async payment events (failures, refunds)
5. **Subscription Renewal** — Two paths: Razorpay payment OR `frn` CLI admin command
6. **Secrets Management** — All credentials in `.env`, `.env.example` committed for reference

---

## Global Prerequisites

### Secrets Safety (Open Source)

> [!CAUTION]
> This is open-source code. **Never** commit real API keys, secrets, or credentials.

- `.env` is already in `.gitignore` — confirmed.
- A new `.env.example` file will be created with placeholder values.
- All Razorpay secrets are read from environment variables via `viper`.
- Code review checklist: no hardcoded keys, no test keys in committed files.

---

## Phase 1: Configuration & Secrets

**Goal**: Add Razorpay config struct, `.env` variables, and `.env.example`.

---

### 1.1 Environment Variables

#### [MODIFY] `.env`

Add after the `# ── Billing Quotas` section (line ~77):

```diff
 FREERANGE_BILLING_FREE_TRIAL_PUSH_QUOTA=1000

+# ── Razorpay Payment Gateway ────────────────────────────────
+FREERANGE_PAYMENT_PROVIDER=mock
+FREERANGE_PAYMENT_RAZORPAY_KEY_ID=rzp_test_xxxxxxxxxxxx
+FREERANGE_PAYMENT_RAZORPAY_KEY_SECRET=xxxxxxxxxxxxxxxxxxxxxxxx
+FREERANGE_PAYMENT_RAZORPAY_WEBHOOK_SECRET=xxxxxxxxxxxxxxxxxxxxxxxx
+FREERANGE_PAYMENT_RAZORPAY_CURRENCY=INR
```

#### [NEW] `.env.example`

Create a committed reference file with all environment variables and placeholder values:

```env
# ============================================================
# FreeRangeNotify — Environment Variables Reference
# ============================================================
# Copy this file to .env and fill in real values.
# NEVER commit .env — only .env.example is tracked in git.
# ============================================================

# ── App ──────────────────────────────────────────────────────
FREERANGE_APP_ENVIRONMENT=development
FREERANGE_APP_DEBUG=true
FREERANGE_APP_LOG_LEVEL=debug

# ── Server ───────────────────────────────────────────────────
FREERANGE_SERVER_PORT=8080
FREERANGE_SERVER_HOST=0.0.0.0
FREERANGE_SERVER_PUBLIC_URL=http://0.0.0.0:8080

# ── Database (Elasticsearch) ────────────────────────────────
FREERANGE_DATABASE_USERNAME=
FREERANGE_DATABASE_PASSWORD=

# ── Redis ────────────────────────────────────────────────────
FREERANGE_REDIS_PASSWORD=

# ── Security / Auth ──────────────────────────────────────────
FREERANGE_SECURITY_JWT_SECRET=<generate-a-random-256-bit-key>
JWT_SECRET=<same-as-above>
FREERANGE_SECURITY_OPS_ENABLED=true
FREERANGE_SECURITY_OPS_SECRET=<generate-a-uuid>

# ── UI ───────────────────────────────────────────────────────
FREERANGE_FRONTEND_URL=http://localhost:3000

# ── SMTP Provider ────────────────────────────────────────────
FREERANGE_PROVIDERS_SMTP_HOST=smtp.gmail.com
FREERANGE_PROVIDERS_SMTP_PORT=587
FREERANGE_PROVIDERS_SMTP_USERNAME=<your-email>
FREERANGE_PROVIDERS_SMTP_PASSWORD=<your-app-password>
FREERANGE_PROVIDERS_SMTP_FROM_EMAIL=<your-email>
FREERANGE_PROVIDERS_SMTP_FROM_NAME=FreeRangeNotify

# ── Other Providers (leave blank to disable) ─────────────────
FREERANGE_PROVIDERS_WEBHOOK_SECRET=<your-webhook-secret>
FREERANGE_PROVIDERS_FCM_SERVER_KEY=
FREERANGE_PROVIDERS_APNS_KEY_ID=
FREERANGE_PROVIDERS_SENDGRID_API_KEY=
FREERANGE_PROVIDERS_TWILIO_ACCOUNT_SID=<your-twilio-sid>
FREERANGE_PROVIDERS_TWILIO_AUTH_TOKEN=<your-twilio-token>
FREERANGE_PROVIDERS_TWILIO_FROM_NUMBER=<your-twilio-number>
FREERANGE_PROVIDERS_WHATSAPP_ENABLED=true
FREERANGE_PROVIDERS_WHATSAPP_ACCOUNT_SID=<your-twilio-sid>
FREERANGE_PROVIDERS_WHATSAPP_AUTH_TOKEN=<your-twilio-token>
FREERANGE_PROVIDERS_WHATSAPP_FROM_NUMBER=whatsapp:<your-number>

# ── Feature Flags ────────────────────────────────────────────
FREERANGE_FEATURES_WORKFLOW_ENABLED=true
FREERANGE_FEATURES_DIGEST_ENABLED=true
FREERANGE_FEATURES_TOPICS_ENABLED=true
FREERANGE_FEATURES_AUDIT_ENABLED=true
FREERANGE_FEATURES_SNOOZE_ENABLED=true
FREERANGE_FEATURES_TRIAL_WELCOME_ENABLED=true
FREERANGE_FEATURES_BILLING_ENABLED=false

# ── Billing Quotas (Free Trial) ──────────────────────────────
FREERANGE_BILLING_FREE_TRIAL_EMAIL_QUOTA=500
FREERANGE_BILLING_FREE_TRIAL_WHATSAPP_QUOTA=50
FREERANGE_BILLING_FREE_TRIAL_SMS_QUOTA=50
FREERANGE_BILLING_FREE_TRIAL_PUSH_QUOTA=1000

# ── Razorpay Payment Gateway ────────────────────────────────
# Set FREERANGE_PAYMENT_PROVIDER=razorpay for production.
# Get keys from https://dashboard.razorpay.com/app/keys
FREERANGE_PAYMENT_PROVIDER=mock
FREERANGE_PAYMENT_RAZORPAY_KEY_ID=<your-razorpay-key-id>
FREERANGE_PAYMENT_RAZORPAY_KEY_SECRET=<your-razorpay-key-secret>
FREERANGE_PAYMENT_RAZORPAY_WEBHOOK_SECRET=<your-razorpay-webhook-secret>
FREERANGE_PAYMENT_RAZORPAY_CURRENCY=INR

# ── OIDC / SSO (Monkeys Identity) ───────────────────────────
FREERANGE_OIDC_ENABLED=true
FREERANGE_OIDC_ISSUER=http://localhost:8085
FREERANGE_OIDC_CLIENT_ID=<your-oidc-client-id>
FREERANGE_OIDC_CLIENT_SECRET=<your-oidc-client-secret>
FREERANGE_OIDC_REDIRECT_URL=http://localhost:8080/v1/auth/sso/callback
FREERANGE_OIDC_FRONTEND_URL=http://localhost:3000

# ── Licensing ────────────────────────────────────────────────
FREERANGE_LICENSING_ENABLED=true
FREERANGE_LICENSING_DEPLOYMENT_MODE=hosted
FREERANGE_LICENSING_SELF_HOSTED_LICENSE_KEY=<your_signed_license>
FREERANGE_LICENSING_SELF_HOSTED_PUBLIC_KEY_PEM=<public_key_pem>
```

---

### 1.2 Config Struct

#### [MODIFY] `internal/config/config.go`

**Step 1**: Add `PaymentConfig` struct after `BillingConfig` (line ~59):

```go
// PaymentConfig selects the payment gateway and holds provider-specific credentials.
// All secrets MUST come from environment variables — never hardcode.
type PaymentConfig struct {
	Provider string         `mapstructure:"provider"` // "mock" | "razorpay"
	Razorpay RazorpayConfig `mapstructure:"razorpay"`
}

// RazorpayConfig holds Razorpay API credentials.
// Get keys from https://dashboard.razorpay.com/app/keys
type RazorpayConfig struct {
	KeyID         string `mapstructure:"key_id"`
	KeySecret     string `mapstructure:"key_secret"`
	WebhookSecret string `mapstructure:"webhook_secret"`
	Currency      string `mapstructure:"currency"`
}
```

**Step 2**: Add `Payment` field to `Config` struct (line ~24, after `Billing`):

```diff
 	Billing    BillingConfig    `mapstructure:"billing"`
+	Payment    PaymentConfig    `mapstructure:"payment"`
 	Licensing  LicensingConfig  `mapstructure:"licensing"`
```

**Step 3**: Add viper defaults in `Load()` function (line ~394, after billing defaults):

```diff
 	viper.SetDefault("billing.free_trial_push_quota", 1000)
+
+	viper.SetDefault("payment.provider", "mock")
+	viper.SetDefault("payment.razorpay.key_id", "")
+	viper.SetDefault("payment.razorpay.key_secret", "")
+	viper.SetDefault("payment.razorpay.webhook_secret", "")
+	viper.SetDefault("payment.razorpay.currency", "INR")
```

**Step 4**: Add validation in `Validate()` (line ~518, before `return nil`):

```go
	if c.Payment.Provider == "razorpay" {
		if strings.TrimSpace(c.Payment.Razorpay.KeyID) == "" {
			return fmt.Errorf("payment.razorpay.key_id is required when payment.provider=razorpay")
		}
		if strings.TrimSpace(c.Payment.Razorpay.KeySecret) == "" {
			return fmt.Errorf("payment.razorpay.key_secret is required when payment.provider=razorpay")
		}
	}
```

> **Risk**: Someone sets `provider=razorpay` without providing keys.
> **Mitigation**: Validation fails at startup with a clear message. The `mock` provider is the default.

---

## Phase 2: Enhanced Domain Models

**Goal**: Extend `billing.Provider` interface and `billing.CheckoutResponse` to support Razorpay's client-side payment flow (order creation → client modal → server verification).

---

### 2.1 Extend Provider Interface

#### [MODIFY] `internal/domain/billing/provider.go`

Replace the entire file with:

```go
package billing

import "context"

// CheckoutResponse represents the result of initializing a checkout flow.
type CheckoutResponse struct {
	URL       string `json:"url,omitempty"`       // Redirect URL (Stripe-style). Empty for Razorpay.
	OrderID   string `json:"order_id"`            // Razorpay Order ID or Stripe Session ID
	Tier      string `json:"tier"`
	AmountINR int64  `json:"amount_inr"`          // Amount in smallest unit (paisa for INR)
	Currency  string `json:"currency"`            // e.g. "INR"
	KeyID     string `json:"key_id,omitempty"`     // Razorpay public key ID (safe for frontend)
}

// PaymentVerification holds the data sent by the client after Razorpay checkout completes.
type PaymentVerification struct {
	OrderID   string `json:"razorpay_order_id" validate:"required"`
	PaymentID string `json:"razorpay_payment_id" validate:"required"`
	Signature string `json:"razorpay_signature" validate:"required"`
}

// WebhookEvent represents a parsed payment webhook event.
type WebhookEvent struct {
	EventType string // "payment.captured", "payment.failed", "subscription.charged", etc.
	TenantID  string
	Tier      string
	OrderID   string
	PaymentID string
	IsActive  bool
}

// Provider defines the interface for interacting with payment gateways (Stripe, Razorpay, Mock).
type Provider interface {
	// CreateOrder creates a payment order. Returns order details for client-side checkout.
	CreateOrder(ctx context.Context, tenantID, tier string, amountPaisa int64) (CheckoutResponse, error)

	// VerifyPayment validates the payment signature after client-side checkout.
	// Returns nil if the signature is valid, error otherwise.
	VerifyPayment(ctx context.Context, verification PaymentVerification) error

	// VerifyWebhook parses and validates an incoming webhook payload.
	VerifyWebhook(payload []byte, signature string) (WebhookEvent, error)
}
```

> **Breaking change**: `CreateCheckoutSession` is renamed to `CreateOrder` with a new signature. 
> `VerifyPayment` is new. Both `MockProvider` and the tenant handler must be updated.

---

### 2.2 Add Renewal Status to Subscription

#### [MODIFY] `internal/domain/license/models.go`

Add new subscription status constant (line ~12):

```diff
 	SubscriptionStatusTrial    SubscriptionStatus = "trial"
+	SubscriptionStatusPending  SubscriptionStatus = "pending_renewal"
 )
```

This status is used when a subscription has expired and the user has initiated but not yet completed a renewal payment.

---

## Phase 3: Razorpay Provider Implementation

**Goal**: Implement `billing.Provider` for Razorpay using their Go SDK.

---

### 3.1 Go Module Dependency

Add the Razorpay Go SDK:

```bash
go get github.com/razorpay/razorpay-go@latest
```

---

### 3.2 Razorpay Provider

#### [NEW] `internal/infrastructure/payment/razorpay.go`

```go
package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	razorpay "github.com/razorpay/razorpay-go"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"go.uber.org/zap"
)

// RazorpayProvider implements billing.Provider using Razorpay's payment gateway.
// It supports:
//   - Order creation for client-side Razorpay Checkout
//   - Server-side payment signature verification (HMAC-SHA256)
//   - Webhook payload verification
//
// All secrets are injected via config — never hardcoded.
type RazorpayProvider struct {
	client        *razorpay.Client
	keyID         string
	keySecret     string
	webhookSecret string
	currency      string
	logger        *zap.Logger
}

// NewRazorpayProvider creates a new Razorpay payment provider.
// keyID and keySecret come from environment variables via config.
func NewRazorpayProvider(keyID, keySecret, webhookSecret, currency string, logger *zap.Logger) billing.Provider {
	client := razorpay.NewClient(keyID, keySecret)
	if currency == "" {
		currency = "INR"
	}
	return &RazorpayProvider{
		client:        client,
		keyID:         keyID,
		keySecret:     keySecret,
		webhookSecret: webhookSecret,
		currency:      currency,
		logger:        logger,
	}
}

// CreateOrder creates a Razorpay Order for the given amount.
// The frontend opens Razorpay Checkout with the returned order_id.
func (p *RazorpayProvider) CreateOrder(ctx context.Context, tenantID, tier string, amountPaisa int64) (billing.CheckoutResponse, error) {
	orderData := map[string]interface{}{
		"amount":   amountPaisa,
		"currency": p.currency,
		"receipt":  fmt.Sprintf("rcpt_%s_%s", tenantID, tier),
		"notes": map[string]interface{}{
			"tenant_id": tenantID,
			"tier":      tier,
		},
	}

	order, err := p.client.Order.Create(orderData, nil)
	if err != nil {
		p.logger.Error("razorpay: failed to create order",
			zap.String("tenant_id", tenantID),
			zap.String("tier", tier),
			zap.Error(err),
		)
		return billing.CheckoutResponse{}, fmt.Errorf("razorpay: create order failed: %w", err)
	}

	orderID, _ := order["id"].(string)
	if orderID == "" {
		return billing.CheckoutResponse{}, fmt.Errorf("razorpay: order response missing 'id'")
	}

	p.logger.Info("razorpay: order created",
		zap.String("order_id", orderID),
		zap.String("tenant_id", tenantID),
		zap.Int64("amount_paisa", amountPaisa),
	)

	return billing.CheckoutResponse{
		OrderID:   orderID,
		Tier:      tier,
		AmountINR: amountPaisa,
		Currency:  p.currency,
		KeyID:     p.keyID, // Public key — safe to send to frontend
	}, nil
}

// VerifyPayment validates the payment signature using HMAC-SHA256.
// Razorpay requires: HMAC_SHA256(order_id + "|" + payment_id, key_secret) == signature
func (p *RazorpayProvider) VerifyPayment(ctx context.Context, v billing.PaymentVerification) error {
	expectedSignature := p.generateSignature(v.OrderID + "|" + v.PaymentID)
	if !hmac.Equal([]byte(expectedSignature), []byte(v.Signature)) {
		p.logger.Warn("razorpay: payment signature mismatch",
			zap.String("order_id", v.OrderID),
			zap.String("payment_id", v.PaymentID),
		)
		return fmt.Errorf("razorpay: invalid payment signature")
	}

	p.logger.Info("razorpay: payment verified",
		zap.String("order_id", v.OrderID),
		zap.String("payment_id", v.PaymentID),
	)
	return nil
}

// VerifyWebhook validates and parses a Razorpay webhook event.
// Uses HMAC-SHA256 of the raw payload body against the webhook secret.
func (p *RazorpayProvider) VerifyWebhook(payload []byte, signature string) (billing.WebhookEvent, error) {
	expectedSig := p.generateWebhookSignature(payload)
	if !hmac.Equal([]byte(expectedSig), []byte(signature)) {
		return billing.WebhookEvent{}, fmt.Errorf("razorpay: invalid webhook signature")
	}

	var body struct {
		Event   string `json:"event"`
		Payload struct {
			Payment struct {
				Entity struct {
					ID      string                 `json:"id"`
					OrderID string                 `json:"order_id"`
					Status  string                 `json:"status"`
					Notes   map[string]interface{} `json:"notes"`
				} `json:"entity"`
			} `json:"payment"`
		} `json:"payload"`
	}

	if err := json.Unmarshal(payload, &body); err != nil {
		return billing.WebhookEvent{}, fmt.Errorf("razorpay: parse webhook body: %w", err)
	}

	entity := body.Payload.Payment.Entity
	tenantID, _ := entity.Notes["tenant_id"].(string)
	tier, _ := entity.Notes["tier"].(string)

	return billing.WebhookEvent{
		EventType: body.Event,
		TenantID:  tenantID,
		Tier:      tier,
		OrderID:   entity.OrderID,
		PaymentID: entity.ID,
		IsActive:  body.Event == "payment.captured",
	}, nil
}

// generateSignature creates HMAC-SHA256 using the Razorpay key_secret.
func (p *RazorpayProvider) generateSignature(data string) string {
	h := hmac.New(sha256.New, []byte(p.keySecret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// generateWebhookSignature creates HMAC-SHA256 using the webhook secret.
func (p *RazorpayProvider) generateWebhookSignature(payload []byte) string {
	h := hmac.New(sha256.New, []byte(p.webhookSecret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}
```

---

### 3.3 Update Mock Provider

#### [MODIFY] `internal/infrastructure/payment/mock.go`

Replace the entire file to match the new `billing.Provider` interface:

```go
package payment

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
)

// MockProvider implements billing.Provider for testing and development.
type MockProvider struct{}

func NewMockProvider() billing.Provider {
	return &MockProvider{}
}

func (p *MockProvider) CreateOrder(ctx context.Context, tenantID, tier string, amountPaisa int64) (billing.CheckoutResponse, error) {
	return billing.CheckoutResponse{
		OrderID:   "mock_order_" + tenantID,
		Tier:      tier,
		AmountINR: amountPaisa,
		Currency:  "INR",
		KeyID:     "mock_key_id",
	}, nil
}

func (p *MockProvider) VerifyPayment(ctx context.Context, v billing.PaymentVerification) error {
	// Mock always succeeds
	return nil
}

func (p *MockProvider) VerifyWebhook(payload []byte, signature string) (billing.WebhookEvent, error) {
	return billing.WebhookEvent{
		EventType: "payment.captured",
		TenantID:  "mock_tenant",
		Tier:      "pro",
		OrderID:   "mock_order",
		PaymentID: "mock_payment",
		IsActive:  true,
	}, nil
}
```

---

## Phase 4: Payment Handler

**Goal**: Create HTTP handlers for order creation, payment verification, and webhook processing.

---

### 4.1 Payment Handler

#### [NEW] `internal/interfaces/http/handlers/payment_handler.go`

```go
package handlers

import (
	"io"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

// PaymentHandler handles payment-related HTTP requests.
type PaymentHandler struct {
	paymentProvider billing.Provider
	subRepo         license.Repository
	rateCard        map[string]billing.PlanTier
	billingEnabled  bool
	logger          *zap.Logger
}

// NewPaymentHandler creates a new PaymentHandler.
func NewPaymentHandler(
	provider billing.Provider,
	subRepo license.Repository,
	rateCard map[string]billing.PlanTier,
	billingEnabled bool,
	logger *zap.Logger,
) *PaymentHandler {
	return &PaymentHandler{
		paymentProvider: provider,
		subRepo:         subRepo,
		rateCard:        rateCard,
		billingEnabled:  billingEnabled,
		logger:          logger,
	}
}

// CreateOrder handles POST /v1/billing/checkout
// Creates a Razorpay order for subscription payment.
func (h *PaymentHandler) CreateOrder(c *fiber.Ctx) error {
	if !h.billingEnabled {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "billing is not enabled",
		})
	}

	userID := c.Locals("user_id").(string)

	var req struct {
		Tier string `json:"tier" validate:"required"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Tier == "" {
		req.Tier = "starter"
	}

	// Look up plan pricing
	plan, ok := h.rateCard[req.Tier]
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unknown plan tier: " + req.Tier,
		})
	}

	// Check for existing active subscription — prevent double payment
	now := time.Now().UTC()
	existingSub, _ := h.subRepo.GetActiveSubscription(c.Context(), userID, "", now)
	if existingSub != nil && existingSub.Status == license.SubscriptionStatusActive {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error":              "active subscription already exists",
			"current_period_end": existingSub.CurrentPeriodEnd.Format(time.RFC3339),
		})
	}

	// Create payment order
	checkout, err := h.paymentProvider.CreateOrder(c.Context(), userID, req.Tier, plan.MonthlyFeePaisa)
	if err != nil {
		h.logger.Error("failed to create payment order",
			zap.String("user_id", userID),
			zap.String("tier", req.Tier),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create payment order",
		})
	}

	h.logger.Info("payment order created",
		zap.String("user_id", userID),
		zap.String("order_id", checkout.OrderID),
		zap.String("tier", req.Tier),
	)

	return c.JSON(fiber.Map{
		"success":    true,
		"order_id":   checkout.OrderID,
		"amount":     checkout.AmountINR,
		"currency":   checkout.Currency,
		"key_id":     checkout.KeyID,
		"tier":       checkout.Tier,
	})
}

// VerifyPayment handles POST /v1/billing/verify-payment
// Verifies Razorpay payment signature and activates the subscription.
func (h *PaymentHandler) VerifyPayment(c *fiber.Ctx) error {
	if !h.billingEnabled {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "billing is not enabled",
		})
	}

	userID := c.Locals("user_id").(string)

	var req billing.PaymentVerification
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.OrderID == "" || req.PaymentID == "" || req.Signature == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "razorpay_order_id, razorpay_payment_id, and razorpay_signature are required",
		})
	}

	// Verify the payment signature
	if err := h.paymentProvider.VerifyPayment(c.Context(), req); err != nil {
		h.logger.Warn("payment verification failed",
			zap.String("user_id", userID),
			zap.String("order_id", req.OrderID),
			zap.Error(err),
		)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "payment verification failed: invalid signature",
		})
	}

	// Activate subscription
	now := time.Now().UTC()
	periodEnd := now.AddDate(0, 1, 0) // +1 month

	// Find existing subscription to update, or create new
	sub, _ := h.subRepo.GetActiveSubscription(c.Context(), userID, "", now)
	if sub == nil {
		// Look for expired/trial subscription to renew
		subs, _ := h.subRepo.List(c.Context(), license.SubscriptionFilter{
			TenantID: userID,
			Limit:    1,
		})
		if len(subs) > 0 {
			sub = subs[0]
		}
	}

	if sub != nil {
		// Renew existing subscription
		sub.Status = license.SubscriptionStatusActive
		sub.Plan = "starter" // or derive from order notes
		sub.CurrentPeriodStart = now
		sub.CurrentPeriodEnd = periodEnd
		sub.UpdatedAt = now
		if sub.Metadata == nil {
			sub.Metadata = make(map[string]interface{})
		}
		sub.Metadata["messages_sent"] = 0
		sub.Metadata["last_payment_id"] = req.PaymentID
		sub.Metadata["last_order_id"] = req.OrderID
		sub.Metadata["last_payment_at"] = now.Format(time.RFC3339)
		sub.Metadata["renewal_method"] = "razorpay"

		if err := h.subRepo.Update(c.Context(), sub); err != nil {
			h.logger.Error("failed to activate subscription after payment",
				zap.String("user_id", userID),
				zap.Error(err),
			)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "payment succeeded but subscription activation failed — contact support",
			})
		}
	}

	h.logger.Info("subscription activated via payment",
		zap.String("user_id", userID),
		zap.String("payment_id", req.PaymentID),
		zap.String("order_id", req.OrderID),
	)

	return c.JSON(fiber.Map{
		"success":              true,
		"message":              "payment verified, subscription activated",
		"plan":                 sub.Plan,
		"status":               string(sub.Status),
		"current_period_start": sub.CurrentPeriodStart.Format(time.RFC3339),
		"current_period_end":   sub.CurrentPeriodEnd.Format(time.RFC3339),
	})
}

// HandleWebhook handles POST /v1/billing/webhook
// Processes Razorpay webhook events (payment.captured, payment.failed, etc.)
// This endpoint does NOT require JWT auth — it uses webhook signature verification.
func (h *PaymentHandler) HandleWebhook(c *fiber.Ctx) error {
	signature := c.Get("X-Razorpay-Signature")
	if signature == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing signature header"})
	}

	body := c.Body()
	if len(body) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "empty webhook body"})
	}

	event, err := h.paymentProvider.VerifyWebhook(body, signature)
	if err != nil {
		h.logger.Warn("webhook signature verification failed", zap.Error(err))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid webhook signature"})
	}

	h.logger.Info("razorpay webhook received",
		zap.String("event_type", event.EventType),
		zap.String("tenant_id", event.TenantID),
		zap.String("payment_id", event.PaymentID),
	)

	switch event.EventType {
	case "payment.captured":
		// Payment successful — ensure subscription is active
		// (This is a fallback; primary activation happens via VerifyPayment)
		if event.TenantID != "" {
			now := time.Now().UTC()
			subs, _ := h.subRepo.List(c.Context(), license.SubscriptionFilter{
				TenantID: event.TenantID,
				Limit:    1,
			})
			if len(subs) > 0 {
				sub := subs[0]
				if sub.Status != license.SubscriptionStatusActive {
					sub.Status = license.SubscriptionStatusActive
					sub.CurrentPeriodStart = now
					sub.CurrentPeriodEnd = now.AddDate(0, 1, 0)
					sub.UpdatedAt = now
					if sub.Metadata == nil {
						sub.Metadata = make(map[string]interface{})
					}
					sub.Metadata["messages_sent"] = 0
					sub.Metadata["webhook_payment_id"] = event.PaymentID
					sub.Metadata["renewal_method"] = "razorpay_webhook"
					_ = h.subRepo.Update(c.Context(), sub)
				}
			}
		}

	case "payment.failed":
		h.logger.Warn("razorpay payment failed",
			zap.String("tenant_id", event.TenantID),
			zap.String("payment_id", event.PaymentID),
		)
		// No action — subscription remains in current state

	default:
		h.logger.Debug("unhandled razorpay event", zap.String("event_type", event.EventType))
	}

	// Always return 200 to Razorpay to acknowledge receipt
	return c.JSON(fiber.Map{"status": "ok"})
}
```

---

## Phase 5: Subscription Renewal Logic

**Goal**: Support two renewal paths:
1. **Razorpay payment** — user pays via Razorpay checkout (covered in Phase 4)
2. **`frn` CLI command** — admin renews subscription without payment (e.g., for free/sponsored users)

---

### 5.1 Renewal Handler

#### [NEW] `internal/interfaces/http/handlers/renewal_handler.go`

```go
package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

// RenewalHandler handles admin-initiated subscription renewals.
type RenewalHandler struct {
	subRepo license.Repository
	logger  *zap.Logger
}

func NewRenewalHandler(subRepo license.Repository, logger *zap.Logger) *RenewalHandler {
	return &RenewalHandler{subRepo: subRepo, logger: logger}
}

// AdminRenew handles POST /v1/admin/subscriptions/:id/renew
// Renews a subscription for 1 month without requiring payment.
// Protected by admin/ops auth — NOT public.
func (h *RenewalHandler) AdminRenew(c *fiber.Ctx) error {
	subID := c.Params("id")
	if subID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "subscription id is required"})
	}

	var req struct {
		Plan     string `json:"plan"`      // optional, defaults to current plan
		Months   int    `json:"months"`    // optional, defaults to 1
		Reason   string `json:"reason"`    // required audit trail
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Months <= 0 {
		req.Months = 1
	}

	sub, err := h.subRepo.GetByID(c.Context(), subID)
	if err != nil || sub == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "subscription not found"})
	}

	now := time.Now().UTC()
	if req.Plan != "" {
		sub.Plan = req.Plan
	}
	sub.Status = license.SubscriptionStatusActive
	sub.CurrentPeriodStart = now
	sub.CurrentPeriodEnd = now.AddDate(0, req.Months, 0)
	sub.UpdatedAt = now

	if sub.Metadata == nil {
		sub.Metadata = make(map[string]interface{})
	}
	sub.Metadata["messages_sent"] = 0
	sub.Metadata["renewed_at"] = now.Format(time.RFC3339)
	sub.Metadata["renewal_method"] = "admin_cli"
	sub.Metadata["renewal_reason"] = req.Reason

	if err := h.subRepo.Update(c.Context(), sub); err != nil {
		h.logger.Error("admin renewal failed",
			zap.String("subscription_id", subID),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to renew subscription"})
	}

	h.logger.Info("subscription renewed via admin CLI",
		zap.String("subscription_id", subID),
		zap.String("tenant_id", sub.TenantID),
		zap.String("plan", sub.Plan),
		zap.Int("months", req.Months),
		zap.String("reason", req.Reason),
	)

	return c.JSON(fiber.Map{
		"success":              true,
		"message":              "subscription renewed",
		"subscription_id":      sub.ID,
		"plan":                 sub.Plan,
		"status":               string(sub.Status),
		"current_period_start": sub.CurrentPeriodStart.Format(time.RFC3339),
		"current_period_end":   sub.CurrentPeriodEnd.Format(time.RFC3339),
		"renewal_method":       "admin_cli",
	})
}
```

---

### 5.2 `frn` CLI — Subscription Renewal Command

#### [MODIFY] `cmd/frn/license.go`

Add a new subcommand to the existing `license` command group. Add after `newLicensePatchCmd()` (line ~28):

```diff
 	cmd.AddCommand(newLicenseVerifyCmd())
 	cmd.AddCommand(newLicensePatchCmd())
+	cmd.AddCommand(newSubscriptionRenewCmd())
```

Then add the following function at the end of the file (before `doJSONRequest`):

```go
// newSubscriptionRenewCmd creates the `frn license renew` subcommand.
// Usage: frn license renew --subscription-id <id> --months 1 --reason "sponsored"
func newSubscriptionRenewCmd() *cobra.Command {
	var apiURL, adminToken string
	var subscriptionID, plan, reason string
	var months int

	cmd := &cobra.Command{
		Use:   "renew",
		Short: "Renew a subscription without payment (admin)",
		Long:  "Renews a subscription for the specified number of months. No payment required. Used for sponsored/free renewals.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig()
			if apiURL != "" {
				cfg.APIURL = apiURL
			}
			if adminToken != "" {
				cfg.AdminToken = adminToken
			}
			if cfg.APIURL == "" {
				cfg.APIURL = "http://localhost:8080"
			}
			if cfg.AdminToken == "" {
				return fmt.Errorf("admin token required: set FREERANGE_ADMIN_TOKEN or use --admin-token")
			}
			if subscriptionID == "" {
				return fmt.Errorf("--subscription-id is required")
			}

			payload := map[string]interface{}{
				"months": months,
				"reason": reason,
			}
			if plan != "" {
				payload["plan"] = plan
			}

			respBody, err := doJSONRequest(
				http.MethodPost,
				fmt.Sprintf("%s/v1/admin/subscriptions/%s/renew", cfg.APIURL, subscriptionID),
				payload,
				map[string]string{
					"Authorization": "Bearer " + cfg.AdminToken,
				},
			)
			if err != nil {
				return err
			}

			fmt.Fprintln(os.Stdout, "Subscription renewed successfully")
			return printJSON(respBody)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL (env: FREERANGE_API_URL)")
	cmd.Flags().StringVar(&adminToken, "admin-token", "", "Admin JWT token (env: FREERANGE_ADMIN_TOKEN)")
	cmd.Flags().StringVar(&subscriptionID, "subscription-id", "", "Subscription ID to renew (required)")
	cmd.Flags().StringVar(&plan, "plan", "", "Plan tier to set (optional, keeps current if empty)")
	cmd.Flags().IntVar(&months, "months", 1, "Number of months to renew for")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason for renewal (audit trail)")

	_ = cmd.MarkFlagRequired("subscription-id")

	return cmd
}
```

---

## Phase 6: Wire Into Container & Routes

**Goal**: Register payment provider, handlers, and new routes.

---

### 6.1 Update Tenant Handler

#### [MODIFY] `internal/interfaces/http/handlers/tenant_handler.go`

**Step 1**: Add `paymentProvider` field to `TenantHandler` struct (line ~15):

```diff
 type TenantHandler struct {
 	service   tenant.Service
+	paymentProvider billing.Provider
 	validator *validator.Validator
 	logger    *zap.Logger
 }
```

**Step 2**: Update `NewTenantHandler` (line ~21):

```diff
-func NewTenantHandler(service tenant.Service, v *validator.Validator, logger *zap.Logger) *TenantHandler {
+func NewTenantHandler(service tenant.Service, paymentProvider billing.Provider, v *validator.Validator, logger *zap.Logger) *TenantHandler {
 	return &TenantHandler{
-		service:   service,
-		validator: v,
-		logger:    logger,
+		service:         service,
+		paymentProvider: paymentProvider,
+		validator:       v,
+		logger:          logger,
 	}
 }
```

**Step 3**: Update `Checkout` method to use the real `paymentProvider` (replace lines 265–281):

```go
	// Create a payment order via the configured payment provider
	plan := h.service.GetPlanForTier("pro")
	checkout, err := h.paymentProvider.CreateOrder(c.Context(), tenantID, "pro", plan.MonthlyFeePaisa)
	if err != nil {
		h.logger.Error("failed to create checkout session",
			zap.String("tenant_id", tenantID),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create checkout session",
		})
	}

	return c.JSON(fiber.Map{
		"success":  true,
		"message":  "checkout order created",
		"data": fiber.Map{
			"order_id": checkout.OrderID,
			"amount":   checkout.AmountINR,
			"currency": checkout.Currency,
			"key_id":   checkout.KeyID,
			"tier":     checkout.Tier,
		},
	})
```

> **Note**: If `tenant.Service` doesn't have `GetPlanForTier`, use the rate card directly. The exact wiring depends on how the container initializes the handler.

---

### 6.2 Add Routes

#### [MODIFY] `internal/interfaces/http/routes/routes.go`

Add new billing routes alongside existing ones (after line ~258):

```diff
 	billing.Get("/rates", c.BillingHandler.GetRates)
+
+	// Payment routes (Razorpay / mock)
+	billing.Post("/checkout", c.PaymentHandler.CreateOrder)
+	billing.Post("/verify-payment", c.PaymentHandler.VerifyPayment)
+
+	// Webhook route — no JWT auth, signature-verified
+	v1.Post("/billing/webhook", c.PaymentHandler.HandleWebhook)
```

Add admin renewal route (near the admin routes section, after line ~284):

```diff
 		tenants.Post("/:id/billing/checkout", c.TenantHandler.Checkout)
+
+		// Admin subscription management
+		admin := v1.Group("/admin", adminAuth)
+		subscriptions := admin.Group("/subscriptions")
+		subscriptions.Post("/:id/renew", c.RenewalHandler.AdminRenew)
```

> **Note**: If an `admin` group already exists in routes.go, add the renewal route inside it.

---

### 6.3 Container Wiring

#### [MODIFY] Container file (where handlers are initialized)

Add payment provider initialization:

```go
// Initialize payment provider based on config
var paymentProvider billing.Provider
switch cfg.Payment.Provider {
case "razorpay":
	paymentProvider = payment.NewRazorpayProvider(
		cfg.Payment.Razorpay.KeyID,
		cfg.Payment.Razorpay.KeySecret,
		cfg.Payment.Razorpay.WebhookSecret,
		cfg.Payment.Razorpay.Currency,
		logger,
	)
	logger.Info("payment provider: razorpay")
default:
	paymentProvider = payment.NewMockProvider()
	logger.Info("payment provider: mock")
}

// Initialize PaymentHandler
paymentHandler := handlers.NewPaymentHandler(
	paymentProvider,
	subscriptionRepo,
	rateCard,
	cfg.Features.BillingEnabled,
	logger,
)

// Initialize RenewalHandler
renewalHandler := handlers.NewRenewalHandler(subscriptionRepo, logger)
```

Make sure `PaymentHandler` and `RenewalHandler` are accessible from the container struct.

---

## Phase 7: Frontend Integration

**Goal**: Add Razorpay checkout flow to the frontend billing page.

---

### 7.1 Frontend API Service

#### [MODIFY] `ui/src/services/api.ts`

Add billing API functions (at the end of the file, before the last export):

```typescript
// ============= Billing APIs =============
export const billingAPI = {
  getUsage: async () => {
    const { data } = await api.get('/billing/usage');
    return data;
  },

  getSubscription: async () => {
    const { data } = await api.get('/billing/subscription');
    return data;
  },

  acceptTrial: async () => {
    const { data } = await api.post('/billing/accept-trial');
    return data;
  },

  getUsageBreakdown: async () => {
    const { data } = await api.get('/billing/usage/breakdown');
    return data;
  },

  getRates: async () => {
    const { data } = await api.get('/billing/rates');
    return data;
  },

  // Payment integration
  createOrder: async (tier: string) => {
    const { data } = await api.post('/billing/checkout', { tier });
    return data;
  },

  verifyPayment: async (payload: {
    razorpay_order_id: string;
    razorpay_payment_id: string;
    razorpay_signature: string;
  }) => {
    const { data } = await api.post('/billing/verify-payment', payload);
    return data;
  },
};
```

---

### 7.2 Razorpay Checkout Script

#### [MODIFY] `ui/index.html`

Add the Razorpay checkout script in the `<head>`:

```diff
+    <script src="https://checkout.razorpay.com/v1/checkout.js"></script>
```

---

### 7.3 Payment Hook

#### [NEW] `ui/src/hooks/useRazorpayCheckout.ts`

```typescript
import { useState, useCallback } from 'react';
import { billingAPI } from '../services/api';
import { toast } from 'sonner';

interface RazorpayCheckoutOptions {
  onSuccess?: () => void;
  onError?: (error: string) => void;
}

declare global {
  interface Window {
    Razorpay: any;
  }
}

export function useRazorpayCheckout(options: RazorpayCheckoutOptions = {}) {
  const [loading, setLoading] = useState(false);

  const initiateCheckout = useCallback(async (tier: string) => {
    setLoading(true);
    try {
      // 1. Create order on backend
      const order = await billingAPI.createOrder(tier);
      if (!order.success) {
        throw new Error(order.error || 'Failed to create order');
      }

      // 2. Open Razorpay checkout modal
      const rzp = new window.Razorpay({
        key: order.key_id,
        amount: order.amount,
        currency: order.currency,
        order_id: order.order_id,
        name: 'FreeRangeNotify',
        description: `${tier.charAt(0).toUpperCase() + tier.slice(1)} Plan - Monthly Subscription`,
        handler: async (response: {
          razorpay_order_id: string;
          razorpay_payment_id: string;
          razorpay_signature: string;
        }) => {
          try {
            // 3. Verify payment on backend
            const result = await billingAPI.verifyPayment(response);
            if (result.success) {
              toast.success('Payment successful! Your subscription is now active.');
              options.onSuccess?.();
            } else {
              toast.error('Payment verification failed. Please contact support.');
              options.onError?.('verification_failed');
            }
          } catch (err) {
            toast.error('Payment verification failed. Please contact support.');
            options.onError?.('verification_error');
          }
        },
        modal: {
          ondismiss: () => {
            setLoading(false);
          },
        },
        theme: {
          color: '#6366f1', // Indigo to match FRN branding
        },
      });

      rzp.on('payment.failed', (response: any) => {
        toast.error(`Payment failed: ${response.error.description}`);
        options.onError?.(response.error.code);
      });

      rzp.open();
    } catch (err: any) {
      toast.error(err.message || 'Failed to initiate checkout');
      options.onError?.(err.message);
    } finally {
      setLoading(false);
    }
  }, [options]);

  return { initiateCheckout, loading };
}
```

---

### 7.4 Update Billing Page

#### [MODIFY] `ui/src/pages/WorkspaceBilling.tsx`

In the "Subscribe Now" / "Upgrade" button's `onClick` handler, replace the mock checkout with:

```typescript
import { useRazorpayCheckout } from '../hooks/useRazorpayCheckout';

// Inside the component:
const { initiateCheckout, loading: checkoutLoading } = useRazorpayCheckout({
  onSuccess: () => {
    // Refetch subscription data
    refetch();
  },
});

// In the button JSX:
<Button
  onClick={() => initiateCheckout('starter')}
  disabled={checkoutLoading}
>
  {checkoutLoading ? 'Processing...' : 'Subscribe Now'}
</Button>
```

---

## Phase 8: Renewal Flow Summary

### When does renewal happen?

| Trigger | Path | Payment Required | Who |
|---|---|---|---|
| Subscription period ends | User must manually renew | Yes (Razorpay) | User |
| User's free trial uses exceeded | User sees upgrade modal | Yes (Razorpay checkout) | User |
| Admin renewal via `frn` CLI | `frn license renew --subscription-id X` | No | Admin |
| Razorpay webhook `payment.captured` | Automatic activation | Already paid | System |

### How does renewal work?

1. **Paid Renewal (Razorpay)**:
   - User clicks "Renew" / "Subscribe" on billing page
   - Frontend calls `POST /v1/billing/checkout` → gets order_id
   - Frontend opens Razorpay modal with order_id
   - User completes payment
   - Razorpay JS calls handler → frontend sends to `POST /v1/billing/verify-payment`
   - Backend verifies HMAC signature, activates subscription for +1 month
   - Usage counters (`messages_sent`) reset to 0

2. **Free Renewal (`frn` CLI)**:
   - Admin runs: `frn license renew --subscription-id <id> --months 1 --reason "community sponsor"`
   - CLI calls `POST /v1/admin/subscriptions/:id/renew`
   - Backend activates subscription, resets counters
   - No payment required

---

## File Change Index

| Phase | File | Action | Description |
|---|---|---|---|
| 1 | `.env` | MODIFY | Add Razorpay env vars (+5 lines) |
| 1 | `.env.example` | NEW | Full env reference file (~90 lines) |
| 1 | `internal/config/config.go` | MODIFY | Add `PaymentConfig` struct, viper defaults, validation (+30 lines) |
| 2 | `internal/domain/billing/provider.go` | MODIFY | Rewrite interface: `CreateOrder`, `VerifyPayment`, `VerifyWebhook` |
| 2 | `internal/domain/license/models.go` | MODIFY | Add `SubscriptionStatusPending` constant (+1 line) |
| 3 | `go.mod` / `go.sum` | MODIFY | Add `github.com/razorpay/razorpay-go` dependency |
| 3 | `internal/infrastructure/payment/razorpay.go` | NEW | Razorpay provider (~170 lines) |
| 3 | `internal/infrastructure/payment/mock.go` | MODIFY | Update to new interface (~35 lines) |
| 4 | `internal/interfaces/http/handlers/payment_handler.go` | NEW | CreateOrder, VerifyPayment, HandleWebhook (~220 lines) |
| 5 | `internal/interfaces/http/handlers/renewal_handler.go` | NEW | AdminRenew endpoint (~80 lines) |
| 5 | `cmd/frn/license.go` | MODIFY | Add `frn license renew` subcommand (+50 lines) |
| 6 | `internal/interfaces/http/handlers/tenant_handler.go` | MODIFY | Wire paymentProvider into Checkout |
| 6 | `internal/interfaces/http/routes/routes.go` | MODIFY | Add billing/payment/webhook/admin routes (+6 lines) |
| 6 | Container file | MODIFY | Wire provider, PaymentHandler, RenewalHandler |
| 7 | `ui/src/services/api.ts` | MODIFY | Add `billingAPI` functions (+30 lines) |
| 7 | `ui/index.html` | MODIFY | Add Razorpay checkout script (+1 line) |
| 7 | `ui/src/hooks/useRazorpayCheckout.ts` | NEW | Razorpay checkout hook (~80 lines) |
| 7 | `ui/src/pages/WorkspaceBilling.tsx` | MODIFY | Wire checkout hook |

---

## Risk Summary

| Risk | Severity | Mitigation |
|---|---|---|
| Razorpay keys committed to git | **Critical** | `.env` in `.gitignore` (already done). `.env.example` has placeholders only. |
| Payment verified on frontend but backend fails to activate | **High** | Webhook fallback activates on `payment.captured`. Retry logic in `VerifyPayment` handler. |
| Double payment / duplicate orders | **Medium** | `CreateOrder` checks for existing active subscription first. Returns 409 Conflict. |
| `frn renew` used without authorization | **Medium** | Route is behind admin auth middleware. Token required for CLI. |
| HMAC signature verification bypass | **Critical** | Uses `crypto/hmac.Equal()` for constant-time comparison. Never use `==`. |
| Razorpay SDK version drift | **Low** | Pin SDK version in `go.mod`. |
| Float precision in amount calculation | **High** | All amounts stored as `int64` paisa. Conversion to INR only at display layer. |
| Webhook replay attack | **Medium** | Log `order_id` + `payment_id` in metadata. Idempotent activation (check if already active). |

---

## Verification Plan

### Unit Tests

#### [NEW] `internal/infrastructure/payment/razorpay_test.go`

```go
func TestRazorpayProvider_VerifyPayment_ValidSignature(t *testing.T)
func TestRazorpayProvider_VerifyPayment_InvalidSignature(t *testing.T)
func TestRazorpayProvider_VerifyWebhook_ValidSignature(t *testing.T)
func TestRazorpayProvider_VerifyWebhook_InvalidSignature(t *testing.T)
func TestRazorpayProvider_VerifyWebhook_ParsesEvent(t *testing.T)
```

These tests verify HMAC signature generation/verification without calling the Razorpay API.

**Run command:**
```bash
go test -v ./internal/infrastructure/payment/ -run TestRazorpay
```

#### [NEW] `internal/interfaces/http/handlers/payment_handler_test.go`

```go
func TestPaymentHandler_CreateOrder_BillingDisabled(t *testing.T)
func TestPaymentHandler_CreateOrder_UnknownTier(t *testing.T)
func TestPaymentHandler_CreateOrder_ActiveSubExists(t *testing.T)
func TestPaymentHandler_VerifyPayment_MissingFields(t *testing.T)
func TestPaymentHandler_VerifyPayment_InvalidSignature(t *testing.T)
```

**Run command:**
```bash
go test -v ./internal/interfaces/http/handlers/ -run TestPaymentHandler
```

#### [NEW] `internal/interfaces/http/handlers/renewal_handler_test.go`

```go
func TestRenewalHandler_AdminRenew_Success(t *testing.T)
func TestRenewalHandler_AdminRenew_NotFound(t *testing.T)
func TestRenewalHandler_AdminRenew_DefaultMonths(t *testing.T)
```

**Run command:**
```bash
go test -v ./internal/interfaces/http/handlers/ -run TestRenewalHandler
```

#### Existing test for config validation:

The file `internal/config/licensing_default_test.go` tests config loading. We should add:

```go
func TestPaymentConfig_RazorpayValidation(t *testing.T)
func TestPaymentConfig_MockDefault(t *testing.T)
```

**Run command:**
```bash
go test -v ./internal/config/ -run TestPaymentConfig
```

### Manual Verification

The following manual tests require the Docker Compose stack running and a Razorpay test account:

1. **Mock provider flow (no Razorpay keys needed)**:
   - Set `FREERANGE_PAYMENT_PROVIDER=mock` in `.env`
   - Start the stack: `docker-compose up -d`
   - Login to UI at `http://localhost:3000`
   - Navigate to Billing page
   - Click "Subscribe Now" → should immediately succeed (mock)
   - Verify subscription status changes to `active`

2. **Razorpay test mode flow** (requires Razorpay test keys):
   - Set `FREERANGE_PAYMENT_PROVIDER=razorpay` and test keys in `.env`
   - Start the stack: `docker-compose up -d`
   - Navigate to Billing page → click "Subscribe Now"
   - Razorpay modal should open with test card (`4111 1111 1111 1111`)
   - Complete payment → verify subscription activates

3. **CLI renewal**:
   - Get a subscription ID from `GET /v1/billing/subscription`
   - Run: `frn license renew --subscription-id <id> --months 1 --reason "test" --admin-token <token>`
   - Verify subscription period extended by 1 month

4. **Secrets safety check**:
   - Run `git diff --cached` before committing
   - Verify no real keys appear in any committed file
   - Verify `.env.example` only has placeholder values
