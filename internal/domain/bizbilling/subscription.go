package bizbilling

import (
	"time"
)

// ─── Subscription Status Constants ───────────────────────────────────────

type SubscriptionStatus string

const (
	SubscriptionStatusActive   SubscriptionStatus = "active"
	SubscriptionStatusPaused   SubscriptionStatus = "paused"
	SubscriptionStatusCanceled SubscriptionStatus = "canceled"
	SubscriptionStatusTrialing SubscriptionStatus = "trialing"
	SubscriptionStatusPastDue  SubscriptionStatus = "past_due"
	SubscriptionStatusUnpaid   SubscriptionStatus = "unpaid"
)

// ─── Domain Models ───────────────────────────────────────────────────────

// BizSubscription represents a customer's subscription to a plan.
type BizSubscription struct {
	ID                 string                 `json:"id"`
	AppID              string                 `json:"app_id"`
	UserID             string                 `json:"user_id"`
	PlanID             string                 `json:"plan_id"`
	AddonIDs           []string               `json:"addon_ids,omitempty"`
	ContractID         string                 `json:"contract_id,omitempty"`
	Status             SubscriptionStatus     `json:"status"`
	Quantity           int                    `json:"quantity"`
	CurrentPeriodStart time.Time              `json:"current_period_start"`
	CurrentPeriodEnd   time.Time              `json:"current_period_end"`
	TrialStart         *time.Time             `json:"trial_start,omitempty"`
	TrialEnd           *time.Time             `json:"trial_end,omitempty"`
	CanceledAt         *time.Time             `json:"canceled_at,omitempty"`
	CancelAtPeriodEnd  bool                   `json:"cancel_at_period_end"`
	PausedAt           *time.Time             `json:"paused_at,omitempty"`
	NonRenewing        bool                   `json:"non_renewing"`
	DunningWorkflowID  string                 `json:"dunning_workflow_id,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`

	// Enriched fields (populated at query time from Plan)
	PlanName        string `json:"plan_name,omitempty"`
	PlanAmountPaisa int64  `json:"plan_amount_paisa,omitempty"`
	BillingCycle    string `json:"billing_cycle,omitempty"`
}

// ─── DTOs for Creation/Updates ───────────────────────────────────────────

// CreateSubscriptionRequest represents the payload for creating a subscription.
type CreateSubscriptionRequest struct {
	UserID    string                 `json:"user_id" validate:"required"`
	PlanID    string                 `json:"plan_id" validate:"required"`
	AddonIDs  []string               `json:"addon_ids,omitempty"`
	Quantity  int                    `json:"quantity" validate:"min=1"`
	TrialDays int                    `json:"trial_days,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateSubscriptionRequest represents the payload for updating a subscription.
type UpdateSubscriptionRequest struct {
	PlanID            *string                `json:"plan_id,omitempty"`
	AddonIDs          []string               `json:"addon_ids,omitempty"`
	Quantity          *int                   `json:"quantity,omitempty"`
	CancelAtPeriodEnd *bool                  `json:"cancel_at_period_end,omitempty"`
	NonRenewing       *bool                  `json:"non_renewing,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}
