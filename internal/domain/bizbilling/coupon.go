package bizbilling

import (
	"errors"
	"time"
)

// ─── Coupon Constants ────────────────────────────────────────────────────

type DiscountType string

const (
	DiscountTypePercentage DiscountType = "percentage" // DiscountValue = percent (e.g. 10 for 10%)
	DiscountTypeFixed      DiscountType = "fixed"      // DiscountValue = paisa
)

// Coupon validation errors — mapped to HTTP codes by the handler.
var (
	ErrCouponInactive       = errors.New("coupon is not active")
	ErrCouponNotStarted     = errors.New("coupon is not yet valid")
	ErrCouponExpired        = errors.New("coupon has expired")
	ErrCouponExhausted      = errors.New("coupon usage limit reached")
	ErrCouponPlanRestricted = errors.New("coupon does not apply to this plan")
)

// ─── Domain Models ───────────────────────────────────────────────────────

// Coupon represents a discount code.
type Coupon struct {
	ID                string       `json:"id"`
	AppID             string       `json:"app_id"`
	Code              string       `json:"code"`
	DiscountType      DiscountType `json:"discount_type"`
	DiscountValue     int64        `json:"discount_value"`
	MaxUses           int          `json:"max_uses,omitempty"` // 0 = unlimited
	CurrentUses       int          `json:"current_uses"`
	MaxUsesPerUser    int          `json:"max_uses_per_user,omitempty"`
	ApplicablePlanIDs []string     `json:"applicable_plan_ids,omitempty"` // empty = all plans
	Stackable         bool         `json:"stackable"`
	ValidFrom         *time.Time   `json:"valid_from,omitempty"`
	ValidUntil        *time.Time   `json:"valid_until,omitempty"`
	Active            bool         `json:"active"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
}

// Validate checks whether the coupon can be redeemed right now for the given
// plan (planID may be empty for one-time invoices). Pure function.
func (c *Coupon) Validate(now time.Time, planID string) error {
	if !c.Active {
		return ErrCouponInactive
	}
	if c.ValidFrom != nil && now.Before(*c.ValidFrom) {
		return ErrCouponNotStarted
	}
	if c.ValidUntil != nil && now.After(*c.ValidUntil) {
		return ErrCouponExpired
	}
	if c.MaxUses > 0 && c.CurrentUses >= c.MaxUses {
		return ErrCouponExhausted
	}
	if len(c.ApplicablePlanIDs) > 0 {
		found := false
		for _, id := range c.ApplicablePlanIDs {
			if id == planID {
				found = true
				break
			}
		}
		if !found {
			return ErrCouponPlanRestricted
		}
	}
	return nil
}

// DiscountFor computes the discount amount for the given subtotal.
// The result never exceeds the subtotal.
func (c *Coupon) DiscountFor(subtotalPaisa int64) int64 {
	var discount int64
	switch c.DiscountType {
	case DiscountTypePercentage:
		discount = subtotalPaisa * c.DiscountValue / 100
	case DiscountTypeFixed:
		discount = c.DiscountValue
	}
	if discount > subtotalPaisa {
		discount = subtotalPaisa
	}
	if discount < 0 {
		discount = 0
	}
	return discount
}

// ─── DTOs ────────────────────────────────────────────────────────────────

// CreateCouponRequest is the payload for creating a coupon.
type CreateCouponRequest struct {
	Code              string       `json:"code" validate:"required"`
	DiscountType      DiscountType `json:"discount_type" validate:"required"`
	DiscountValue     int64        `json:"discount_value" validate:"required,gt=0"`
	MaxUses           int          `json:"max_uses,omitempty"`
	MaxUsesPerUser    int          `json:"max_uses_per_user,omitempty"`
	ApplicablePlanIDs []string     `json:"applicable_plan_ids,omitempty"`
	Stackable         bool         `json:"stackable,omitempty"`
	ValidFrom         *time.Time   `json:"valid_from,omitempty"`
	ValidUntil        *time.Time   `json:"valid_until,omitempty"`
}

// UpdateCouponRequest is the payload for updating a coupon.
type UpdateCouponRequest struct {
	DiscountValue     *int64     `json:"discount_value,omitempty"`
	MaxUses           *int       `json:"max_uses,omitempty"`
	MaxUsesPerUser    *int       `json:"max_uses_per_user,omitempty"`
	ApplicablePlanIDs []string   `json:"applicable_plan_ids,omitempty"`
	Stackable         *bool      `json:"stackable,omitempty"`
	ValidFrom         *time.Time `json:"valid_from,omitempty"`
	ValidUntil        *time.Time `json:"valid_until,omitempty"`
	Active            *bool      `json:"active,omitempty"`
}

// BulkGenerateCouponsRequest generates N unique codes sharing the same rules.
type BulkGenerateCouponsRequest struct {
	Count  int                 `json:"count" validate:"required,min=1,max=1000"`
	Prefix string              `json:"prefix,omitempty"`
	Rules  CreateCouponRequest `json:"rules" validate:"required"`
}

// CouponFilter defines criteria for listing coupons.
type CouponFilter struct {
	AppID  string
	Code   string
	Active *bool
	Limit  int
	Offset int
}
