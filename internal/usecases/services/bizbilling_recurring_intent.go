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
	if sub.PendingChange != nil {
		target := *sub
		target.PlanID, target.Quantity, target.AddonIDs, target.PriceSnapshot = sub.PendingChange.PlanID, sub.PendingChange.Quantity, sub.PendingChange.AddonIDs, &sub.PendingChange.Price
		sub = &target
	}
	plan, err := s.lifecycle.stores.Plans.Get(ctx, sub.PlanID)
	if err != nil {
		return intent, err
	}
	if plan.AppID != sub.AppID {
		return intent, apperrors.NotFound("plan", sub.PlanID)
	}
	if sub.PriceSnapshot != nil {
		if sub.PriceSnapshot.PlanID != plan.ID || sub.PriceSnapshot.Currency != sub.Currency {
			return intent, bizbilling.ErrInvalidMoney
		}
		accepted := *plan
		accepted.BillingCycle, accepted.CycleDays, accepted.Currency, accepted.Name = sub.PriceSnapshot.BillingCycle, sub.PriceSnapshot.CycleDays, sub.PriceSnapshot.Currency, sub.PriceSnapshot.PlanName
		plan = &accepted
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
	if !sub.InitialAutomaticPeriod && !sub.CurrentPeriodEnd.After(now.Add(10*time.Minute)) {
		return intent, apperrors.BadRequest("Renew the current billing period before authorizing future automatic collection")
	}
	if sub.Status != bizbilling.SubscriptionStatusActive && sub.Status != bizbilling.SubscriptionStatusTrialing && sub.Status != bizbilling.SubscriptionStatusPastDue && !(sub.InitialAutomaticPeriod && sub.Status == bizbilling.SubscriptionStatusAwaitingAuthorization) {
		return intent, apperrors.BadRequest("This subscription cannot request automatic authorization")
	}
	price := sub.PriceSnapshot
	if price == nil {
		price, err = s.lifecycle.operations.subscriptionPrice(ctx, sub.FinancialScope(), sub, false)
		if err != nil {
			return intent, err
		}
	}
	if _, err = validatedSnapshotTotals(price, sub.FinancialScope()); err != nil {
		return intent, err
	}
	if price.TotalPaisa < 100 || price.TotalPaisa > math.MaxUint32 {
		return intent, apperrors.BadRequest("This amount is outside the supported automatic plan range; use invoice collection")
	}
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
	startAt := sub.CurrentPeriodEnd
	startOnAuthorization := sub.InitialAutomaticPeriod && (sub.TrialEnd == nil || !sub.TrialEnd.After(now.Add(10*time.Minute)))
	if sub.InitialAutomaticPeriod && sub.TrialEnd != nil && !startOnAuthorization {
		startAt = *sub.TrialEnd
	}
	start := startAt.Truncate(time.Second)
	if start.Before(startAt) {
		start = start.Add(time.Second)
	}
	expires := now.Add(24 * time.Hour).Truncate(time.Second)
	if startOnAuthorization {
		start = time.Time{}
	} else if limit := start.Add(-time.Minute); limit.Before(expires) {
		expires = limit
	}
	return bizbilling.RecurringIntent{StartOnAuthorization: startOnAuthorization, OperationID: id, SubscriptionID: sub.ID, Scope: sub.FinancialScope(), Gateway: ref, Name: plan.Name, Period: period, Interval: interval, TotalCount: sub.AutomaticCycles, AmountPaisa: price.TotalPaisa, Currency: plan.Currency, StartAt: start, ExpireBy: expires, Price: *price}, nil
}
