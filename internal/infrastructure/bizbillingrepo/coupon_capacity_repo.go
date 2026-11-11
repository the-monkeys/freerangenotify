package bizbillingrepo

import (
	"context"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// CouponCapacityRepo serializes one short counter transition per coupon.
// Durable redemption receipts retain deduplication; the coupon keeps only one
// transient claim, and each customer counter keeps only its latest marker.
type CouponCapacityRepo struct {
	coupons     *Store[bizbilling.Coupon]
	redemptions *Store[bizbilling.CouponRedemption]
	customers   *Store[bizbilling.CouponCustomerUsage]
	streams     *StreamRepo
}

func NewCouponCapacityRepo(es *elasticsearch.Client, logger *zap.Logger) *CouponCapacityRepo {
	return &CouponCapacityRepo{coupons: NewStore[bizbilling.Coupon](es, CouponIndex, logger, repository.RefreshFalse), redemptions: NewStore[bizbilling.CouponRedemption](es, CouponRedemptionIndex, logger, repository.RefreshFalse), customers: NewStore[bizbilling.CouponCustomerUsage](es, CouponCustomerUsageIndex, logger, repository.RefreshFalse), streams: NewStreamRepo(es, logger)}
}

func (r *CouponCapacityRepo) Reserve(ctx context.Context, claim bizbilling.StreamClaim, input bizbilling.CouponRedemption) (*bizbilling.CouponRedemption, error) {
	if input.ID != claim.OperationID+"-coupon" || input.Scope != claim.Scope || input.DiscountPaisa <= 0 {
		return nil, bizbilling.ErrInvalidMoney
	}
	if err := r.streams.Check(ctx, claim); err != nil {
		return nil, err
	}
	_, err := r.redemptions.CreateIfAbsent(ctx, input.ID, &input)
	if err != nil {
		return nil, err
	}
	row, err := r.redemptions.Get(ctx, input.ID)
	if err != nil {
		return nil, err
	}
	if row.Scope != input.Scope || row.CouponID != input.CouponID || row.InvoiceID != input.InvoiceID {
		return nil, bizbilling.ErrReconciliationRequired
	}
	if row.State == "reserved" || row.State == "consumed" {
		return row, r.unlock(ctx, row.CouponID, row.ID, "reserve")
	}
	if row.State != "prepared" {
		if err = r.unlock(ctx, row.CouponID, row.ID, "reserve"); err != nil {
			return nil, err
		}
		return nil, bizbilling.ErrCouponExhausted
	}
	coupon, err := r.lock(ctx, row.CouponID, row.Scope.AppID, row.ID, "reserve")
	if err != nil {
		return nil, err
	}
	if coupon.PendingCapacity.Stage != "counted" {
		if coupon.CurrentUses < 0 || coupon.MaxUses < 0 {
			return nil, bizbilling.ErrReconciliationRequired
		}
		if coupon.MaxUses > 0 && coupon.CurrentUses >= coupon.MaxUses {
			return nil, r.rejectReservation(ctx, row)
		}
		if err = r.changeCustomer(ctx, *row, "reserve:"+row.ID, 1, row.MaxUsesPerUser); err != nil {
			if err == bizbilling.ErrCouponExhausted {
				return nil, r.rejectReservation(ctx, row)
			}
			return nil, err
		}
		_, err = r.coupons.Update(ctx, row.CouponID, func(c *bizbilling.Coupon) error {
			if !capacityMatches(c, row.ID, "reserve") {
				return bizbilling.ErrConflict
			}
			if c.PendingCapacity.Stage == "counted" {
				return nil
			}
			if c.CurrentUses == int(^uint(0)>>1) {
				return bizbilling.ErrMoneyOverflow
			}
			c.CurrentUses++
			c.PendingCapacity.Stage = "counted"
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	row, err = r.redemptions.Update(ctx, row.ID, func(current *bizbilling.CouponRedemption) error {
		if current.State != "prepared" && current.State != "reserved" {
			return bizbilling.ErrReconciliationRequired
		}
		current.State = "reserved"
		return nil
	})
	if err != nil {
		return nil, err
	}
	return row, r.unlock(ctx, row.CouponID, row.ID, "reserve")
}

func (r *CouponCapacityRepo) rejectReservation(ctx context.Context, row *bizbilling.CouponRedemption) error {
	_, err := r.redemptions.Update(ctx, row.ID, func(current *bizbilling.CouponRedemption) error {
		if current.State != "prepared" && current.State != "rejected" {
			return bizbilling.ErrReconciliationRequired
		}
		current.State = "rejected"
		return nil
	})
	if err != nil {
		return err
	}
	if err = r.unlock(ctx, row.CouponID, row.ID, "reserve"); err != nil {
		return err
	}
	return bizbilling.ErrCouponExhausted
}

func capacityMatches(c *bizbilling.Coupon, id, action string) bool {
	return c.PendingCapacity != nil && c.PendingCapacity.ID == id && c.PendingCapacity.Action == action
}
func (r *CouponCapacityRepo) lock(ctx context.Context, couponID, appID, id, action string) (*bizbilling.Coupon, error) {
	return r.coupons.Update(ctx, couponID, func(c *bizbilling.Coupon) error {
		if c.AppID != appID {
			return apperrors.NotFound("coupon", couponID)
		}
		if c.PendingCapacity != nil && !capacityMatches(c, id, action) {
			return bizbilling.ErrConflict
		}
		if c.PendingCapacity == nil {
			c.PendingCapacity = &bizbilling.CouponCapacityClaim{ID: id, Action: action, Stage: "locked"}
		}
		return nil
	})
}
func (r *CouponCapacityRepo) unlock(ctx context.Context, couponID, id, action string) error {
	_, err := r.coupons.Update(ctx, couponID, func(c *bizbilling.Coupon) error {
		if capacityMatches(c, id, action) {
			c.PendingCapacity = nil
		}
		return nil
	})
	return err
}

func (r *CouponCapacityRepo) changeCustomer(ctx context.Context, row bizbilling.CouponRedemption, marker string, delta, limit int) error {
	id := ScopeID(bizbilling.Scope{AppID: row.Scope.AppID, UserID: row.Scope.UserID, Currency: row.CouponID})
	_, err := r.customers.Get(ctx, id)
	if apperrors.IsNotFound(err) {
		if delta < 0 {
			return bizbilling.ErrReconciliationRequired
		}
		_, err = r.customers.CreateIfAbsent(ctx, id, &bizbilling.CouponCustomerUsage{ID: id, AppID: row.Scope.AppID, UserID: row.Scope.UserID, CouponID: row.CouponID})
	}
	if err != nil {
		return err
	}
	_, err = r.customers.Update(ctx, id, func(usage *bizbilling.CouponCustomerUsage) error {
		if usage.AppID != row.Scope.AppID || usage.UserID != row.Scope.UserID || usage.CouponID != row.CouponID {
			return bizbilling.ErrReconciliationRequired
		}
		if usage.LastMarker == marker {
			return nil
		}
		if usage.Uses < 0 || (delta < 0 && usage.Uses == 0) {
			return bizbilling.ErrReconciliationRequired
		}
		if delta > 0 && limit > 0 && usage.Uses >= limit {
			return bizbilling.ErrCouponExhausted
		}
		if delta > 0 && usage.Uses == int(^uint(0)>>1) {
			return bizbilling.ErrMoneyOverflow
		}
		usage.Uses += delta
		usage.LastMarker = marker
		return nil
	})
	return err
}

func (r *CouponCapacityRepo) Release(ctx context.Context, claim bizbilling.StreamClaim, id string) (*bizbilling.CouponRedemption, error) {
	if err := r.streams.Check(ctx, claim); err != nil {
		return nil, err
	}
	row, err := r.redemptions.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if row.Scope != claim.Scope || (row.State != "consumed" && row.State != "reserved" && row.State != "released") {
		return nil, bizbilling.ErrReconciliationRequired
	}
	marker := claim.OperationID
	if row.CapacityReleased {
		if row.ReleaseOperationID != marker {
			return nil, bizbilling.ErrInvalidMoney
		}
		return row, r.unlock(ctx, row.CouponID, marker, "release")
	}
	coupon, err := r.lock(ctx, row.CouponID, row.Scope.AppID, marker, "release")
	if err != nil {
		return nil, err
	}
	if coupon.PendingCapacity.Stage != "counted" {
		if err = r.changeCustomer(ctx, *row, "release:"+id, -1, 0); err != nil {
			return nil, err
		}
		_, err = r.coupons.Update(ctx, row.CouponID, func(c *bizbilling.Coupon) error {
			if !capacityMatches(c, marker, "release") {
				return bizbilling.ErrConflict
			}
			if c.PendingCapacity.Stage == "counted" {
				return nil
			}
			if c.CurrentUses <= 0 {
				return bizbilling.ErrReconciliationRequired
			}
			c.CurrentUses--
			c.PendingCapacity.Stage = "counted"
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	row, err = r.redemptions.Update(ctx, id, func(current *bizbilling.CouponRedemption) error {
		if current.CapacityReleased && current.ReleaseOperationID != marker {
			return bizbilling.ErrConflict
		}
		current.CapacityReleased = true
		current.ReleaseOperationID = marker
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("coupon release receipt pending: %w", err)
	}
	return row, r.unlock(ctx, row.CouponID, marker, "release")
}
