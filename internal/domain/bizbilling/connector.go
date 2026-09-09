package bizbilling

import "time"

// ─── Third-Party Billing Connectors (Layer 2) ────────────────────────────

// Supported connector providers.
const (
	ConnectorProviderRazorpay = "razorpay"
	ConnectorProviderStripe   = "stripe"
	ConnectorProviderCustom   = "custom"
)

// ValidConnectorProviders contains all supported connector providers.
var ValidConnectorProviders = map[string]bool{
	ConnectorProviderRazorpay: true,
	ConnectorProviderStripe:   true,
	ConnectorProviderCustom:   true,
}

// Connector links an app to an external billing tool whose webhooks
// FreeRangeNotify consumes for delivery + dunning.
type Connector struct {
	ID            string                 `json:"id"`
	AppID         string                 `json:"app_id"`
	Provider      string                 `json:"provider"`
	WebhookSecret string                 `json:"webhook_secret,omitempty"` // HMAC secret; redacted in list responses
	EventMappings map[string]interface{} `json:"event_mappings,omitempty"`
	Active        bool                   `json:"active"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// ConnectorEvent is a normalized event parsed from a third-party webhook.
type ConnectorEvent struct {
	Provider       string `json:"provider"`
	EventType      string `json:"event_type"` // normalized: invoice.paid | invoice.payment_failed | subscription.canceled | ...
	ExternalUserID string `json:"external_user_id,omitempty"`
	Email          string `json:"email,omitempty"`
	Phone          string `json:"phone,omitempty"`
	InvoiceRef     string `json:"invoice_ref,omitempty"`
	PaymentRef     string `json:"payment_ref,omitempty"`
	AmountPaisa    int64  `json:"amount_paisa,omitempty"`
	Currency       string `json:"currency,omitempty"`
}

// ─── DTOs ────────────────────────────────────────────────────────────────

// CreateConnectorRequest registers a third-party connector.
type CreateConnectorRequest struct {
	AppID         string                 `json:"app_id" validate:"required"`
	Provider      string                 `json:"provider" validate:"required"`
	WebhookSecret string                 `json:"webhook_secret,omitempty"`
	EventMappings map[string]interface{} `json:"event_mappings,omitempty"`
}

// UpdateConnectorRequest updates a connector.
type UpdateConnectorRequest struct {
	WebhookSecret *string                `json:"webhook_secret,omitempty"`
	EventMappings map[string]interface{} `json:"event_mappings,omitempty"`
	Active        *bool                  `json:"active,omitempty"`
}

// ─── Customer Portal Tokens ──────────────────────────────────────────────

// PortalToken grants an end customer time-limited access to the self-service
// portal without a login.
type PortalToken struct {
	Token     string    `json:"token"`
	AppID     string    `json:"app_id"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
