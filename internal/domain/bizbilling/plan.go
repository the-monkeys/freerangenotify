package bizbilling

import (
	"context"
	"time"
)

// ─── Billing Cycle Constants ─────────────────────────────────────────────

// BillingCycle defines how often a subscription is billed.
type BillingCycle string

const (
	BillingCycleWeekly     BillingCycle = "weekly"
	BillingCycleMonthly    BillingCycle = "monthly"
	BillingCycleQuarterly  BillingCycle = "quarterly"
	BillingCycleHalfYearly BillingCycle = "half_yearly"
	BillingCycleYearly     BillingCycle = "yearly"
	BillingCycleCustom     BillingCycle = "custom" // Uses CycleDays field
)

// ValidBillingCycles contains all valid billing cycle values.
var ValidBillingCycles = map[BillingCycle]bool{
	BillingCycleWeekly:     true,
	BillingCycleMonthly:    true,
	BillingCycleQuarterly:  true,
	BillingCycleHalfYearly: true,
	BillingCycleYearly:     true,
	BillingCycleCustom:     true,
}

// ─── Pricing Model Constants ─────────────────────────────────────────────

// PricingModel defines how the plan price is calculated.
type PricingModel string

const (
	PricingModelFlat    PricingModel = "flat"     // Fixed price per cycle
	PricingModelPerUnit PricingModel = "per_unit" // Price × quantity (e.g., per seat)
	PricingModelTiered  PricingModel = "tiered"   // Different price per tier (all units in each tier)
	PricingModelVolume  PricingModel = "volume"   // Single price based on total volume
)

// ValidPricingModels contains all valid pricing model values.
var ValidPricingModels = map[PricingModel]bool{
	PricingModelFlat:    true,
	PricingModelPerUnit: true,
	PricingModelTiered:  true,
	PricingModelVolume:  true,
}

// ─── Plan Model ──────────────────────────────────────────────────────────

// PricingTier represents a single tier in tiered/volume pricing.
// For tiered: first N units at UnitPaisa, next M units at a lower rate, etc.
// For volume: if total quantity falls in this tier, UnitPaisa applies to ALL units.
type PricingTier struct {
	UpTo      int64 `json:"up_to"`                // Upper bound of this tier (0 = unlimited / last tier)
	UnitPaisa int64 `json:"unit_paisa"`           // Price per unit in paisa
	FlatPaisa int64 `json:"flat_paisa,omitempty"` // Optional flat fee for this tier
}

// Plan represents a pricing configuration for a Product.
// Examples: "Monthly ₹499", "Yearly ₹4,999", "Per-seat ₹99/user/month".
type Plan struct {
	ID            string                 `json:"id" es:"id"`
	AppID         string                 `json:"app_id" es:"app_id"`
	ProductID     string                 `json:"product_id" es:"product_id"`
	Name          string                 `json:"name" es:"name"`
	AmountPaisa   int64                  `json:"amount_paisa" es:"amount_paisa"` // Base price in paisa (used for flat/per_unit models)
	Currency      string                 `json:"currency" es:"currency"`         // Default: "INR"
	BillingCycle  BillingCycle           `json:"billing_cycle" es:"billing_cycle"`
	CycleDays     int                    `json:"cycle_days,omitempty" es:"cycle_days"` // Only used when BillingCycle = "custom"
	TrialDays     int                    `json:"trial_days,omitempty" es:"trial_days"`
	SetupFeePaisa int64                  `json:"setup_fee_paisa,omitempty" es:"setup_fee_paisa"`
	TaxInclusive  bool                   `json:"tax_inclusive" es:"tax_inclusive"`
	PricingModel  PricingModel           `json:"pricing_model" es:"pricing_model"`
	Tiers         []PricingTier          `json:"tiers,omitempty" es:"tiers"` // Only for tiered/volume pricing
	Active        bool                   `json:"active" es:"active"`
	Metadata      map[string]interface{} `json:"metadata,omitempty" es:"metadata"`
	CreatedAt     time.Time              `json:"created_at" es:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at" es:"updated_at"`
}

// ─── Request DTOs ────────────────────────────────────────────────────────

// CreatePlanRequest is the request DTO for creating a plan.
type CreatePlanRequest struct {
	ProductID     string                 `json:"product_id" validate:"required"`
	Name          string                 `json:"name" validate:"required,min=1,max=255"`
	AmountPaisa   int64                  `json:"amount_paisa" validate:"min=0"`
	Currency      string                 `json:"currency,omitempty"` // Defaults to "INR"
	BillingCycle  BillingCycle           `json:"billing_cycle" validate:"required"`
	CycleDays     int                    `json:"cycle_days,omitempty" validate:"min=0"`
	TrialDays     int                    `json:"trial_days,omitempty" validate:"min=0,max=365"`
	SetupFeePaisa int64                  `json:"setup_fee_paisa,omitempty" validate:"min=0"`
	TaxInclusive  bool                   `json:"tax_inclusive"`
	PricingModel  PricingModel           `json:"pricing_model,omitempty"` // Defaults to "flat"
	Tiers         []PricingTier          `json:"tiers,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// UpdatePlanRequest is the request DTO for updating a plan.
type UpdatePlanRequest struct {
	Name          *string                `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	AmountPaisa   *int64                 `json:"amount_paisa,omitempty" validate:"omitempty,min=0"`
	Currency      *string                `json:"currency,omitempty"`
	BillingCycle  *BillingCycle          `json:"billing_cycle,omitempty"`
	CycleDays     *int                   `json:"cycle_days,omitempty" validate:"omitempty,min=0"`
	TrialDays     *int                   `json:"trial_days,omitempty" validate:"omitempty,min=0,max=365"`
	SetupFeePaisa *int64                 `json:"setup_fee_paisa,omitempty" validate:"omitempty,min=0"`
	TaxInclusive  *bool                  `json:"tax_inclusive,omitempty"`
	PricingModel  *PricingModel          `json:"pricing_model,omitempty"`
	Tiers         []PricingTier          `json:"tiers,omitempty"`
	Active        *bool                  `json:"active,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// PlanFilter defines query filters for listing plans.
type PlanFilter struct {
	AppID     string `json:"app_id"`
	ProductID string `json:"product_id,omitempty"`
	Active    *bool  `json:"active,omitempty"`
	Search    string `json:"search,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
}

// ─── Repository Interface ────────────────────────────────────────────────

// PlanRepository defines persistence operations for plans.
type PlanRepository interface {
	Create(ctx context.Context, plan *Plan) error
	GetByID(ctx context.Context, id string) (*Plan, error)
	Update(ctx context.Context, plan *Plan) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter PlanFilter) ([]*Plan, int64, error)
}
