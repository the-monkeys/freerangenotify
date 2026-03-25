package billing

import (
	"context"
	"time"
)

// Credential source values — mirrors providers.CredSource* constants.
// Defined here so the billing domain has no import cycle with providers.
const (
	CredSourceSystem   = "system"   // FreeRangeNotify system credentials — we pay the carrier
	CredSourceBYOC     = "byoc"     // User's own credentials — they pay the carrier
	CredSourcePlatform = "platform" // No external carrier cost (in-app, SSE, push)
)

// UsageEvent represents a single billable delivery action.
// Stored in the `frn_usage_events` Elasticsearch index.
// All monetary amounts are stored as int64 paisa (1 INR = 100 paisa)
// to avoid float precision drift at scale.
type UsageEvent struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenant_id"`
	AppID            string    `json:"app_id"`
	NotificationID   string    `json:"notification_id"`
	Channel          string    `json:"channel"`           // "email", "whatsapp", "sms", "push", etc.
	Provider         string    `json:"provider"`          // "smtp", "sendgrid", "twilio", etc.
	CredentialSource string    `json:"credential_source"` // "system" | "byoc" | "platform"
	MessageType      string    `json:"message_type"`      // "marketing" | "utility" | "transactional"
	CostUnitPaisa    int64     `json:"cost_unit_paisa"`   // our carrier cost in paisa
	BilledPaisa      int64     `json:"billed_paisa"`      // what we charge the user in paisa
	Currency         string    `json:"currency"`          // always "INR"
	Status           string    `json:"status"`            // "charged" | "free_tier" | "platform"
	Timestamp        time.Time `json:"timestamp"`
}

// UsageSummary aggregates usage events over a billing period, grouped by
// channel and credential source. Used for invoice generation and the
// /v1/billing/usage/breakdown API response.
type UsageSummary struct {
	TenantID         string `json:"tenant_id"`
	Channel          string `json:"channel"`
	CredentialSource string `json:"credential_source"`
	MessageCount     int64  `json:"message_count"`
	TotalBilledPaisa int64  `json:"total_billed_paisa"`
	PeriodStart      string `json:"period_start"` // RFC3339
	PeriodEnd        string `json:"period_end"`   // RFC3339
}

// UsageEmitter writes a UsageEvent to the ledger asynchronously.
// Implementations must be goroutine-safe. The Manager calls this after
// every successful Send() only when billing is enabled.
type UsageEmitter interface {
	Emit(ctx context.Context, event *UsageEvent) error
}

// UsageRepository reads and aggregates usage data from the ledger.
type UsageRepository interface {
	UsageEmitter

	// Store persists a single usage event.
	Store(ctx context.Context, event *UsageEvent) error

	// GetSummary returns per-channel, per-credential-source totals for a period.
	GetSummary(ctx context.Context, tenantIDs []string, from, to time.Time) ([]UsageSummary, error)

	// GetEvents returns raw events for a tenant in a period (for audit/export).
	GetEvents(ctx context.Context, tenantID string, from, to time.Time, limit int) ([]UsageEvent, error)
}
