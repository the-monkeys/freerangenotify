package bizbilling

import (
	"context"
	"time"
)

// PlanAddon represents an optional add-on that can be attached to a subscription.
// Examples: "Extra locker ₹100/mo", "Priority support ₹500/mo".
type PlanAddon struct {
	ID           string       `json:"id" es:"id"`
	AppID        string       `json:"app_id" es:"app_id"`
	PlanID       string       `json:"plan_id" es:"plan_id"`
	Name         string       `json:"name" es:"name"`
	AmountPaisa  int64        `json:"amount_paisa" es:"amount_paisa"`
	BillingCycle BillingCycle `json:"billing_cycle" es:"billing_cycle"` // Inherits from plan or set independently
	Active       bool         `json:"active" es:"active"`
	CreatedAt    time.Time    `json:"created_at" es:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at" es:"updated_at"`
}

// CreatePlanAddonRequest is the request DTO for creating a plan addon.
type CreatePlanAddonRequest struct {
	Name         string       `json:"name" validate:"required,min=1,max=255"`
	AmountPaisa  int64        `json:"amount_paisa" validate:"required,min=1"`
	BillingCycle BillingCycle `json:"billing_cycle,omitempty"` // Defaults to parent plan's cycle
}

// PlanAddonRepository defines persistence operations for plan addons.
type PlanAddonRepository interface {
	Create(ctx context.Context, addon *PlanAddon) error
	GetByID(ctx context.Context, id string) (*PlanAddon, error)
	Update(ctx context.Context, addon *PlanAddon) error
	Delete(ctx context.Context, id string) error
	ListByPlanID(ctx context.Context, appID, planID string) ([]*PlanAddon, error)
}
