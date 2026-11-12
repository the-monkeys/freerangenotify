package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
)

func (s *BizOperationService) buildSubscriptionCreate(ctx context.Context, command bizbilling.Command, event *bizbilling.JournalEvent) error {
	request, now := command.SubscriptionCreate.Request, event.CreatedAt
	if _, err := s.billingCustomerState(ctx, command.Scope); err != nil {
		return err
	}
	plan, err := s.stores.Plans.Get(ctx, request.PlanID)
	if err != nil {
		return err
	}
	if plan.AppID != command.Scope.AppID || !plan.Active {
		return bizbilling.ErrInvalidMoney
	}
	quantity := request.Quantity
	if quantity == 0 {
		quantity = 1
	}
	mode := request.GatewayMode
	if mode == "" {
		mode = bizbilling.GatewayLive
	}
	if (mode == bizbilling.GatewayTest) != (command.Scope.Mode == bizbilling.GatewayTest) {
		return bizbilling.ErrModeMismatch
	}
	collection := request.CollectionMode
	if collection == "" {
		collection = bizbilling.CollectionInvoice
	}
	sub := &bizbilling.BizSubscription{ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(event.OperationID+":subscription")).String(), AppID: command.Scope.AppID, UserID: command.Scope.UserID, PlanID: plan.ID, Currency: command.Scope.Currency, Quantity: quantity, AddonIDs: request.AddonIDs, GatewayMode: mode, CollectionMode: collection, AutomaticCycles: request.AutomaticCycles, Status: bizbilling.SubscriptionStatusActive, CurrentPeriodStart: now, AnchorDay: now.Day(), Metadata: request.Metadata, LastSequence: event.Sequence, CreatedAt: now, UpdatedAt: now}
	sub.PriceSnapshot, err = s.subscriptionPrice(ctx, command.Scope, sub, false)
	if err != nil {
		return err
	}
	trial := request.TrialDays
	if trial == 0 {
		trial = plan.TrialDays
	}
	if trial < 0 || trial > 3650 {
		return bizbilling.ErrInvalidPeriod
	}
	if trial > 0 {
		end := now.AddDate(0, 0, trial)
		sub.Status, sub.TrialStart, sub.TrialEnd = bizbilling.SubscriptionStatusTrialing, &now, &end
		sub.CurrentPeriodStart, sub.AnchorDay = end, end.Day()
	}
	sub.CurrentPeriodEnd, err = bizbilling.NextAnchoredPeriod(sub.CurrentPeriodStart, plan.BillingCycle, plan.CycleDays, sub.AnchorDay)
	if err != nil {
		return err
	}
	if trial > 0 && collection == bizbilling.CollectionInvoice {
		sub.InitialPriceSnapshot, err = s.subscriptionPrice(ctx, command.Scope, sub, true)
		if err != nil {
			return err
		}
	}
	if collection == bizbilling.CollectionAutomatic {
		if err = validateAutomaticPlan(plan, sub); err != nil {
			return err
		}
		sub.InitialAutomaticPeriod, sub.CollectionState, sub.AuthorizationGeneration = true, "awaiting_authorization", 1
		if trial == 0 {
			sub.Status = bizbilling.SubscriptionStatusAwaitingAuthorization
			sub.CurrentPeriodStart, sub.CurrentPeriodEnd = time.Time{}, time.Time{}
		}
	} else if trial == 0 {
		first, err := s.subscriptionPrice(ctx, command.Scope, sub, true)
		if err != nil {
			return err
		}
		if err = s.buildPeriodInvoice(ctx, command, event, sub, first, sub.CurrentPeriodStart, sub.CurrentPeriodEnd, ""); err != nil {
			return err
		}
	}
	event.SubscriptionAfter = sub
	event.Resources = append(event.Resources, bizbilling.ResourceRef{Kind: "subscription", ID: sub.ID})
	return nil
}

func validateAutomaticPlan(plan *bizbilling.Plan, sub *bizbilling.BizSubscription) error {
	if sub.AutomaticCycles < 1 || sub.AutomaticCycles > 120 || plan.Currency != "INR" || (plan.PricingModel != "" && plan.PricingModel != bizbilling.PricingModelFlat && plan.PricingModel != bizbilling.PricingModelPerUnit) || plan.SetupFeePaisa != 0 || len(sub.AddonIDs) != 0 || sub.ContractID != "" {
		return bizbilling.ErrInvalidMoney
	}
	if plan.BillingCycle == bizbilling.BillingCycleCustom || !bizbilling.ValidBillingCycles[plan.BillingCycle] {
		return bizbilling.ErrInvalidPeriod
	}
	if sub.PriceSnapshot.TotalPaisa < 100 {
		return bizbilling.ErrInvalidMoney
	}
	return nil
}

func (s *BizOperationService) buildPeriodInvoice(ctx context.Context, command bizbilling.Command, event *bizbilling.JournalEvent, sub *bizbilling.BizSubscription, price *bizbilling.SubscriptionPriceSnapshot, start, end time.Time, provider string) error {
	draft := command
	draft.InvoiceDraft = &bizbilling.InvoiceDraft{InvoiceType: bizbilling.InvoiceTypeRecurring, SubscriptionID: sub.ID, Request: bizbilling.CreateInvoiceRequest{UserID: sub.UserID, Currency: command.Scope.Currency, GatewayMode: sub.EffectiveGatewayMode(), LineItems: append([]bizbilling.InvoiceLineItem(nil), price.LineItems...), PaymentTerms: price.PaymentTerms}}
	if err := s.buildFinancialInvoiceDraftUsingPrice(ctx, draft, event, price); err != nil {
		return err
	}
	id := bizbillingrepo.PeriodID(command.Scope.AppID, sub.ID, start)
	invoice := event.InvoiceAfter
	invoice.PeriodID, invoice.ProviderInvoiceID = id, provider
	if sub.EffectiveCollectionMode() == bizbilling.CollectionAutomatic {
		invoice.CollectionOwner = "razorpay:" + sub.ID
	}
	if err := s.issueFinancialInvoice(ctx, event, invoice); err != nil {
		return err
	}
	sub.LastPeriodID = id
	event.PeriodAfter = &bizbilling.BillingPeriod{ID: id, Scope: command.Scope, SubscriptionID: sub.ID, Start: start, End: end, InvoiceID: invoice.ID, ProviderInvoiceID: provider, LastSequence: event.Sequence}
	event.Resources = append(event.Resources, bizbilling.ResourceRef{Kind: "period", ID: id})
	return nil
}
