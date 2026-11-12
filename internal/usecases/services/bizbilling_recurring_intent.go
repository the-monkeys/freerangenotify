package services

import (
	"context"
	"math"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (s *BizRecurringService) recurringIntent(ctx context.Context, sub *bizbilling.BizSubscription, mode bizbilling.GatewayMode, id string) (bizbilling.RecurringIntent, error) {
	var intent bizbilling.RecurringIntent
	plan, err := s.lifecycle.stores.Plans.Get(ctx, sub.PlanID)
	if err != nil {
		return intent, err
	}
	if plan.AppID != sub.AppID {
		return intent, apperrors.NotFound("plan", sub.PlanID)
	}
	if plan.Currency != "INR" || sub.Currency != plan.Currency {
		return intent, apperrors.BadRequest("Automatic collection currently supports fixed INR plans; use invoice collection for this currency")
	}
	if (plan.PricingModel != "" && plan.PricingModel != bizbilling.PricingModelFlat && plan.PricingModel != bizbilling.PricingModelPerUnit) || len(sub.AddonIDs) != 0 || plan.SetupFeePaisa != 0 || sub.ContractID != "" {
		return intent, apperrors.BadRequest("Use invoice collection for usage, tiered, contract, setup-fee or add-on pricing")
	}
	if sub.AutomaticCycles < 1 || sub.AutomaticCycles > 120 {
		return intent, apperrors.BadRequest("Choose 1 to 120 automatic billing cycles")
	}
	period, interval := "", 1
	switch plan.BillingCycle {
	case bizbilling.BillingCycleWeekly:
		period = "weekly"
	case bizbilling.BillingCycleMonthly:
		period = "monthly"
	case bizbilling.BillingCycleQuarterly:
		period, interval = "monthly", 3
	case bizbilling.BillingCycleHalfYearly:
		period, interval = "monthly", 6
	case bizbilling.BillingCycleYearly:
		period = "yearly"
	default:
		return intent, apperrors.BadRequest("This billing cycle requires invoice collection")
	}
	now := time.Now().UTC()
	if !sub.CurrentPeriodEnd.After(now.Add(10 * time.Minute)) {
		return intent, apperrors.BadRequest("Renew the current billing period before authorizing future automatic collection")
	}
	if sub.Status != bizbilling.SubscriptionStatusActive && sub.Status != bizbilling.SubscriptionStatusTrialing && sub.Status != bizbilling.SubscriptionStatusPastDue {
		return intent, apperrors.BadRequest("This subscription cannot request automatic authorization")
	}
	amount, err := bizbilling.PlanChargeForChecked(plan, sub.Quantity)
	if err != nil {
		return intent, err
	}
	items := []bizbilling.InvoiceLineItem{{Description: plan.Name, Quantity: 1, UnitPrice: amount}}
	profile, err := s.lifecycle.users.GetByID(ctx, sub.UserID)
	if err != nil {
		return intent, err
	}
	if profile.AppID != sub.AppID {
		return intent, apperrors.NotFound("customer", sub.UserID)
	}
	state := ""
	if profile.BillingAddress != nil {
		state = profile.BillingAddress.State
	}
	tax, err := s.lifecycle.configSvc.TaxConfig(ctx, sub.AppID)
	if err != nil {
		return intent, err
	}
	if plan.TaxInclusive {
		tax.TaxInclusive = true
	}
	totals, err := bizbilling.ComputeInvoiceTotalsChecked(items, tax, state, nil)
	if err != nil {
		return intent, err
	}
	if totals.TotalPaisa < 100 || totals.TotalPaisa > math.MaxUint32 {
		return intent, apperrors.BadRequest("This amount is outside the supported automatic plan range; use invoice collection")
	}
	terms, _, err := validatedInvoiceTerms(ctx, s.lifecycle.configSvc, sub.AppID, "", now)
	if err != nil {
		return intent, err
	}
	price := bizbilling.SubscriptionPriceSnapshot{PlanID: plan.ID, PlanName: plan.Name, BillingCycle: plan.BillingCycle, CycleDays: plan.CycleDays, InvoicePreview: bizbilling.InvoicePreview{Currency: plan.Currency, LineItems: items, SubtotalPaisa: totals.SubtotalPaisa, DiscountPaisa: totals.DiscountPaisa, TaxAmountPaisa: totals.TaxPaisa, TaxBreakdown: totals.Breakdown, TotalPaisa: totals.TotalPaisa, PaymentTerms: terms}}
	merchant, err := s.merchants.Snapshot(ctx, sub.AppID, mode)
	if err != nil {
		return intent, err
	}
	if !merchant.Configured || !merchant.APIAuthenticated {
		return intent, bizbilling.ErrMerchantNotReady
	}
	ref := bizbilling.GatewayRef{ConnectionID: merchant.ConnectionID, AccountID: merchant.AccountID, Mode: mode, CredentialRevision: merchant.Revision}
	gateway, err := s.gateway(ctx, ref)
	if err != nil {
		return intent, err
	}
	if err = gateway.CheckRecurring(ctx); err != nil {
		return intent, err
	}
	if err = s.merchants.connections.RecordRecurringCapability(ctx, bizbilling.RecurringCapability{AppID: sub.AppID, Gateway: ref, VerifiedAt: now}); err != nil {
		return intent, err
	}
	start := sub.CurrentPeriodEnd.Truncate(time.Second)
	if start.Before(sub.CurrentPeriodEnd) {
		start = start.Add(time.Second)
	}
	expires := now.Add(24 * time.Hour).Truncate(time.Second)
	if limit := start.Add(-time.Minute); limit.Before(expires) {
		expires = limit
	}
	return bizbilling.RecurringIntent{OperationID: id, SubscriptionID: sub.ID, Scope: sub.FinancialScope(), Gateway: ref, Name: plan.Name, Period: period, Interval: interval, TotalCount: sub.AutomaticCycles, AmountPaisa: totals.TotalPaisa, Currency: plan.Currency, StartAt: start, ExpireBy: expires, Price: price}, nil
}
