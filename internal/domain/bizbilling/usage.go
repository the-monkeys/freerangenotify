package bizbilling

import "time"

// ─── Usage Metering ──────────────────────────────────────────────────────

// Aggregation strategies for a usage meter over a billing period.
const (
	AggregationSum  = "sum"
	AggregationMax  = "max"
	AggregationLast = "last"
)

// UsageMeter defines a metric a business tracks for usage-based billing
// (e.g. "API Calls", "Storage GB").
type UsageMeter struct {
	ID          string    `json:"id"`
	AppID       string    `json:"app_id"`
	Name        string    `json:"name"`
	Unit        string    `json:"unit,omitempty"`
	Aggregation string    `json:"aggregation"` // sum | max | last
	CreatedAt   time.Time `json:"created_at"`
}

// UsageEvent is a single usage report tied to a meter.
type BizUsageEvent struct {
	ID             string    `json:"id"`
	AppID          string    `json:"app_id"`
	UserID         string    `json:"user_id"`
	SubscriptionID string    `json:"subscription_id,omitempty"`
	MeterID        string    `json:"meter_id"`
	Quantity       int64     `json:"quantity"`
	Timestamp      time.Time `json:"timestamp"`
}

// UsageSummary is the aggregated usage for one meter in a period.
type BizUsageSummary struct {
	MeterID     string    `json:"meter_id"`
	MeterName   string    `json:"meter_name,omitempty"`
	Unit        string    `json:"unit,omitempty"`
	Aggregation string    `json:"aggregation"`
	Quantity    int64     `json:"quantity"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
}

// ─── DTOs ────────────────────────────────────────────────────────────────

// CreateUsageMeterRequest is the payload for creating a usage meter.
type CreateUsageMeterRequest struct {
	Name        string `json:"name" validate:"required"`
	Unit        string `json:"unit,omitempty"`
	Aggregation string `json:"aggregation,omitempty"` // defaults to sum
}

// ReportUsageRequest reports a batch of usage events.
type ReportUsageRequest struct {
	Events []ReportUsageEvent `json:"events" validate:"required,min=1,max=500"`
}

// ReportUsageEvent is a single event inside a batch report.
type ReportUsageEvent struct {
	UserID         string     `json:"user_id" validate:"required"`
	SubscriptionID string     `json:"subscription_id,omitempty"`
	MeterID        string     `json:"meter_id" validate:"required"`
	Quantity       int64      `json:"quantity" validate:"required"`
	Timestamp      *time.Time `json:"timestamp,omitempty"` // defaults to now
}

// ─── Tiered Usage Pricing (pure function) ────────────────────────────────

// TieredUsageCharge computes the charge for `quantity` units against
// graduated pricing tiers. Each tier covers units up to `UpTo` (0 = infinity)
// at `UnitPaisa` per unit, plus an optional `FlatPaisa` once the tier is
// entered. Tiers must be sorted ascending by UpTo.
func TieredUsageCharge(tiers []PricingTier, quantity int64) int64 {
	if quantity <= 0 || len(tiers) == 0 {
		return 0
	}

	var charge int64
	var covered int64
	for _, tier := range tiers {
		if covered >= quantity {
			break
		}

		upper := tier.UpTo
		if upper <= 0 { // final, unbounded tier
			upper = quantity
		}
		if upper > quantity {
			upper = quantity
		}

		units := upper - covered
		if units <= 0 {
			continue
		}

		charge += units*tier.UnitPaisa + tier.FlatPaisa
		covered = upper
	}

	// Quantity beyond the last bounded tier is charged at the last tier's rate.
	if covered < quantity {
		last := tiers[len(tiers)-1]
		charge += (quantity - covered) * last.UnitPaisa
	}

	return charge
}
