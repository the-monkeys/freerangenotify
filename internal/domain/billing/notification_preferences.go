package billing

import (
	"context"
	"time"
)

// PaymentNotificationPreferences controls how a tenant's admin is notified
// when a payment succeeds and tokens are allocated to their account.
// When no document exists for a tenant, all channels default to enabled and
// contact details are pulled live from the tenant's AdminUser record.
type PaymentNotificationPreferences struct {
	TenantID        string    `json:"tenant_id"`
	EmailEnabled    bool      `json:"email_enabled"`
	EmailAddress    string    `json:"email_address,omitempty"`
	SMSEnabled      bool      `json:"sms_enabled"`
	PhoneNumber     string    `json:"phone_number,omitempty"`
	WhatsAppEnabled bool      `json:"whatsapp_enabled"`
	WhatsAppNumber  string    `json:"whatsapp_number,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// PaymentNotificationPreferencesRepository persists per-tenant notification preferences.
type PaymentNotificationPreferencesRepository interface {
	GetByTenantID(ctx context.Context, tenantID string) (*PaymentNotificationPreferences, error)
	Upsert(ctx context.Context, prefs *PaymentNotificationPreferences) error
}
