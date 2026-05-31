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
