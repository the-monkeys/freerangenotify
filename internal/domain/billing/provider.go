package billing

import "context"

// CheckoutResponse represents the result of initializing a checkout flow.
type CheckoutResponse struct {
	URL       string `json:"url,omitempty"`   // Redirect URL (Stripe-style). Empty for Razorpay.
	OrderID   string `json:"order_id"`        // Razorpay Order ID or Stripe Session ID
	Tier      string `json:"tier"`
	AmountINR int64  `json:"amount_inr"`      // Amount in smallest unit (paisa for INR)
	Currency  string `json:"currency"`        // e.g. "INR"
	KeyID     string `json:"key_id,omitempty"` // Razorpay public key ID (safe for frontend)
}

// PaymentVerification holds the data sent by the client after Razorpay checkout completes.
type PaymentVerification struct {
	OrderID   string `json:"razorpay_order_id" validate:"required"`
	PaymentID string `json:"razorpay_payment_id" validate:"required"`
	Signature string `json:"razorpay_signature" validate:"required"`
}

// WebhookEvent represents a parsed payment webhook event.
type WebhookEvent struct {
	EventType string // "payment.captured", "payment.failed", etc.
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
