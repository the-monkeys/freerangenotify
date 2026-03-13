package billing

import "context"

// CheckoutResponse represents the result of initializing a checkout flow.
type CheckoutResponse struct {
	URL     string `json:"url"`      // E.g., Stripe Checkout URL
	OrderID string `json:"order_id"` // E.g., Razorpay Order ID
	Tier    string `json:"tier"`
}

// WebhookEvent represents a parsed payment webhook event.
type WebhookEvent struct {
	TenantID string
	Tier     string
	IsActive bool
}

// Provider defines the interface for interacting with payment gateways (Stripe, Razorpay, Mock).
type Provider interface {
	CreateCheckoutSession(ctx context.Context, tenantID, tier string) (CheckoutResponse, error)
	VerifyWebhook(payload []byte, signature string) (WebhookEvent, error)
}
