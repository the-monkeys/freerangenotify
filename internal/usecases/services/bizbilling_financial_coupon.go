package services

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (s *BizOperationService) buildCouponApplication(ctx context.Context, command bizbilling.Command, claim bizbilling.StreamClaim, event *bizbilling.JournalEvent) error {
	input := command.CouponApply
	inv, err := s.financialInvoice(ctx, command.Scope, input.InvoiceID)
	if err != nil {
		return err
	}
	if inv.CollectionOwner != "" {
		return bizbilling.ErrAutomaticCollectionInProgress
	}
	if inv.Status != bizbilling.InvoiceStatusDraft || inv.CouponID != "" || inv.CreditsApplied != 0 || inv.AmountPaidPaisa != 0 || inv.TDSPaisa != 0 {
		return bizbilling.ErrInvalidMoney
	}
	id := event.OperationID + "-coupon"
	row, err := s.stores.CouponRedemptions.Get(ctx, id)
	if err != nil && !apperrors.IsNotFound(err) {
		return err
	}
	if row == nil {
		coupons, _, err := s.stores.Coupons.Find(ctx, bizbillingrepo.NewQuery(command.Scope.AppID).Term("code", input.Code).Body("created_at", 2, 0))
		if err != nil {
			return err
		}
		if len(coupons) != 1 {
			return apperrors.NotFound("coupon", input.Code)
		}
		coupon, err := s.stores.Coupons.Get(ctx, coupons[0].ID)
		if err != nil {
			return err
		}
		planID := ""
		if inv.SubscriptionID != "" {
			sub, err := s.stores.Subscriptions.Get(ctx, inv.SubscriptionID)
			if err != nil {
				return err
			}
			if sub.AppID != command.Scope.AppID || sub.UserID != command.Scope.UserID {
				return bizbilling.ErrReconciliationRequired
			}
			planID = sub.PlanID
		}
		if err = coupon.Validate(event.CreatedAt, planID); err != nil {
			return err
		}
		if coupon.DiscountValue <= 0 {
			return bizbilling.ErrInvalidMoney
		}
		discount := coupon.DiscountValue
		switch coupon.DiscountType {
		case bizbilling.DiscountTypePercentage:
			if coupon.DiscountValue > 100 {
				return bizbilling.ErrInvalidMoney
			}
			discount, err = bizbilling.ProrateMinor(inv.SubtotalPaisa, coupon.DiscountValue, 100)
			if err != nil {
				return err
			}
		case bizbilling.DiscountTypeFixed:
		default:
			return bizbilling.ErrInvalidMoney
		}
		discount = min(discount, inv.TotalPaisa)
		if discount <= 0 {
			return bizbilling.ErrInvalidMoney
		}
		row = &bizbilling.CouponRedemption{ID: id, Scope: command.Scope, CouponID: coupon.ID, InvoiceID: inv.ID, DiscountPaisa: discount, MaxUsesPerUser: coupon.MaxUsesPerUser, State: "prepared"}
	}
	if row.Scope != command.Scope || row.InvoiceID != inv.ID || row.DiscountPaisa <= 0 || row.DiscountPaisa > inv.TotalPaisa {
		return bizbilling.ErrReconciliationRequired
	}
	inv.DiscountPaisa, err = bizbilling.CheckedAdd(inv.DiscountPaisa, row.DiscountPaisa)
	if err != nil {
		return err
	}
	inv.TotalPaisa -= row.DiscountPaisa
	inv.AmountDuePaisa -= row.DiscountPaisa
	inv.CouponID = row.CouponID
	inv.CouponRedemptionID = row.ID
	inv.LastSequence = event.Sequence
	inv.UpdatedAt = event.CreatedAt
	if err = invoiceAccountingInvariant(inv); err != nil {
		return err
	}
	// This recoverable cross-customer reservation is last: no definitive input
	// rejection is allowed after capacity has been reserved.
	row, err = s.couponCapacity.Reserve(ctx, claim, *row)
	if err != nil {
		return err
	}
	row.State = "consumed"
	row.LastSequence = event.Sequence
	event.CouponRedemptionAfter = row
	event.InvoiceAfter = inv
	event.Resources = []bizbilling.ResourceRef{{Kind: "invoice", ID: inv.ID}, {Kind: "coupon_redemption", ID: row.ID}}
	return nil
}
