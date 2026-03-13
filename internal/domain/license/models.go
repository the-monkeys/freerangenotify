package license

import "time"

// SubscriptionStatus represents subscription lifecycle states.
type SubscriptionStatus string

const (
	SubscriptionStatusActive   SubscriptionStatus = "active"
	SubscriptionStatusExpired  SubscriptionStatus = "expired"
	SubscriptionStatusCanceled SubscriptionStatus = "canceled"
	SubscriptionStatusTrial    SubscriptionStatus = "trial"
)

// Subscription represents a hosted subscription record.
// Scope can be app-level (AppID set) or tenant-level (TenantID set).
type Subscription struct {
	ID                 string                 `json:"id" es:"id"`
	TenantID           string                 `json:"tenant_id,omitempty" es:"tenant_id"`
	AppID              string                 `json:"app_id,omitempty" es:"app_id"`
	Plan               string                 `json:"plan" es:"plan"`
	Status             SubscriptionStatus     `json:"status" es:"status"`
	CurrentPeriodStart time.Time              `json:"current_period_start" es:"current_period_start"`
	CurrentPeriodEnd   time.Time              `json:"current_period_end" es:"current_period_end"`
	Metadata           map[string]interface{} `json:"metadata,omitempty" es:"metadata"`
	CreatedAt          time.Time              `json:"created_at" es:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at" es:"updated_at"`
}

// IsActiveAt returns true when the subscription is active for the given timestamp.
func (s *Subscription) IsActiveAt(now time.Time) bool {
	if s == nil {
		return false
	}

	if s.Status != SubscriptionStatusActive && s.Status != SubscriptionStatusTrial {
		return false
	}

	return !now.Before(s.CurrentPeriodStart) && !now.After(s.CurrentPeriodEnd)
}

// SubscriptionFilter defines query filters for listing subscriptions.
type SubscriptionFilter struct {
	TenantID string               `json:"tenant_id,omitempty"`
	AppID    string               `json:"app_id,omitempty"`
	Statuses []SubscriptionStatus `json:"statuses,omitempty"`
	Limit    int                  `json:"limit,omitempty"`
	Offset   int                  `json:"offset,omitempty"`
}
