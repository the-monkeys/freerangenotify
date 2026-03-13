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

// CreateCheckoutSession immediately returns success for the mock provider.
func (p *MockProvider) CreateCheckoutSession(ctx context.Context, tenantID, tier string) (billing.CheckoutResponse, error) {
	return billing.CheckoutResponse{
		URL:     "mock_success", // The frontend can handle 'mock_success' to simulate a successful redirect
		OrderID: "mock_order_123",
		Tier:    tier,
	}, nil
}

// VerifyWebhook mimics verifying a webhook event. Not deeply used in the mock flow.
func (p *MockProvider) VerifyWebhook(payload []byte, signature string) (billing.WebhookEvent, error) {
	return billing.WebhookEvent{
		TenantID: "mock_tenant",
		Tier:     "pro",
		IsActive: true,
	}, nil
}
