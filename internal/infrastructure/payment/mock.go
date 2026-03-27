package payment

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
)

// MockProvider implements billing.Provider for testing and early development without a real gateway.
type MockProvider struct{}

// NewMockProvider creates a new mock payment provider.
func NewMockProvider() billing.Provider {
	return &MockProvider{}
}

// CreateOrder immediately returns a mock order for dev/testing.
func (p *MockProvider) CreateOrder(ctx context.Context, tenantID, tier string, amountPaisa int64) (billing.CheckoutResponse, error) {
	return billing.CheckoutResponse{
		URL:       "mock_success",
		OrderID:   "mock_order_" + tenantID,
		Tier:      tier,
		AmountINR: amountPaisa,
		Currency:  "INR",
		KeyID:     "mock_key_id",
	}, nil
}

// VerifyPayment always succeeds for mock — no real signature check.
func (p *MockProvider) VerifyPayment(ctx context.Context, v billing.PaymentVerification) error {
	return nil
}

// VerifyWebhook returns a successful mock webhook event.
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
