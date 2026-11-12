package services

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (s *BizOperationService) subscriptionPrice(ctx context.Context, scope bizbilling.Scope, sub *bizbilling.BizSubscription, first bool) (*bizbilling.SubscriptionPriceSnapshot, error) {
	plan, err := s.stores.Plans.Get(ctx, sub.PlanID)
	if err != nil {
		return nil, err
	}
	if plan.AppID != scope.AppID {
		return nil, apperrors.NotFound("plan", sub.PlanID)
	}
	if plan.Currency != scope.Currency {
		return nil, bizbilling.ErrCurrencyMismatch
	}
	amount, err := bizbilling.PlanChargeForChecked(plan, sub.Quantity)
	if err != nil {
		return nil, err
	}
	// Aggregate the checked charge once. Dividing a flat/tiered total by the
	// quantity and multiplying it back used to lose fractional minor units.
	items := []bizbilling.InvoiceLineItem{{Description: plan.Name, Quantity: 1, UnitPrice: amount}}
	if first && plan.SetupFeePaisa > 0 {
		items = append(items, bizbilling.InvoiceLineItem{Description: "One-time setup fee", Quantity: 1, UnitPrice: plan.SetupFeePaisa})
	}
	seen := map[string]bool{}
	for _, id := range sub.AddonIDs {
		if seen[id] {
			return nil, bizbilling.ErrInvalidMoney
		}
		seen[id] = true
		addon, err := s.stores.Addons.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if addon.AppID != scope.AppID || addon.PlanID != sub.PlanID {
			return nil, apperrors.NotFound("plan addon", id)
		}
		if addon.BillingCycle != "" && addon.BillingCycle != plan.BillingCycle {
			return nil, bizbilling.ErrLegacyReviewRequired
		}
		items = append(items, bizbilling.InvoiceLineItem{Description: addon.Name, Quantity: 1, UnitPrice: addon.AmountPaisa})
	}
	state, err := s.billingCustomerState(ctx, scope)
	if err != nil {
		return nil, err
	}
	tax, err := s.config.TaxConfig(ctx, scope.AppID)
	if err != nil {
		return nil, err
	}
	if plan.TaxInclusive {
		tax.TaxInclusive = true
	}
	totals, err := bizbilling.ComputeInvoiceTotalsChecked(items, tax, state, nil)
	if err != nil {
		return nil, err
	}
	terms, err := s.config.PaymentTermsConfig(ctx, scope.AppID)
	if err != nil {
		return nil, err
	}
	return &bizbilling.SubscriptionPriceSnapshot{PlanID: plan.ID, PlanName: plan.Name, BillingCycle: plan.BillingCycle, CycleDays: plan.CycleDays, InvoicePreview: bizbilling.InvoicePreview{Currency: scope.Currency, LineItems: items, SubtotalPaisa: totals.SubtotalPaisa, DiscountPaisa: totals.DiscountPaisa, TaxAmountPaisa: totals.TaxPaisa, TaxBreakdown: totals.Breakdown, TotalPaisa: totals.TotalPaisa, PaymentTerms: terms.DefaultTerms}}, nil
}

func validatedSnapshotTotals(price *bizbilling.SubscriptionPriceSnapshot, scope bizbilling.Scope) (bizbilling.InvoiceTotals, error) {
	var result bizbilling.InvoiceTotals
	if price == nil || price.Currency != scope.Currency || len(price.LineItems) == 0 || price.SubtotalPaisa < 0 || price.DiscountPaisa < 0 || price.TaxAmountPaisa < 0 || price.TotalPaisa < 0 {
		return result, bizbilling.ErrInvalidMoney
	}
	sum := int64(0)
	for _, item := range price.LineItems {
		if item.Quantity < 1 || item.UnitPrice < 0 || item.AmountPaisa < 0 || item.DiscountPaisa < 0 || item.TaxRate < 0 || item.TaxRate > 100 {
			return result, bizbilling.ErrInvalidMoney
		}
		var err error
		sum, err = bizbilling.CheckedAdd(sum, item.AmountPaisa)
		if err != nil {
			return result, err
		}
	}
	if sum != price.SubtotalPaisa {
		return result, bizbilling.ErrReconciliationRequired
	}
	total, err := bizbilling.CheckedAdd(price.SubtotalPaisa, price.TaxAmountPaisa)
	if err != nil {
		return result, err
	}
	if total != price.TotalPaisa {
		return result, bizbilling.ErrReconciliationRequired
	}
	return bizbilling.InvoiceTotals{SubtotalPaisa: price.SubtotalPaisa, DiscountPaisa: price.DiscountPaisa, TaxPaisa: price.TaxAmountPaisa, Breakdown: price.TaxBreakdown, TotalPaisa: price.TotalPaisa}, nil
}
