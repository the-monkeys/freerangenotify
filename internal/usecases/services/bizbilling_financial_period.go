package services

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (s *BizOperationService) buildSubscriptionPeriod(ctx context.Context, command bizbilling.Command, event *bizbilling.JournalEvent) error {
	input := command.SubscriptionPeriod
	id := bizbillingrepo.PeriodID(command.Scope.AppID, input.SubscriptionID, input.Start)
	period, err := s.stores.Periods.Get(ctx, id)
	if err == nil {
		if period.Scope != command.Scope || period.SubscriptionID != input.SubscriptionID || !period.Start.Equal(input.Start) || !period.End.Equal(input.End) {
			return bizbilling.ErrConflict
		}
		event.Resources = []bizbilling.ResourceRef{{Kind: "period", ID: id}, {Kind: "invoice", ID: period.InvoiceID}}
		if input.ProviderInvoiceID == "" || period.ProviderInvoiceID == input.ProviderInvoiceID {
			return nil
		}
		if period.ProviderInvoiceID != "" {
			return bizbilling.ErrConflict
		}
		invoice, err := s.financialInvoice(ctx, command.Scope, period.InvoiceID)
		if err != nil {
			return err
		}
		if invoice.PeriodID != period.ID || invoice.ProviderInvoiceID != "" && invoice.ProviderInvoiceID != input.ProviderInvoiceID {
			return bizbilling.ErrConflict
		}
		period.ProviderInvoiceID, period.LastSequence = input.ProviderInvoiceID, event.Sequence
		invoice.ProviderInvoiceID, invoice.LastSequence, invoice.UpdatedAt = input.ProviderInvoiceID, event.Sequence, event.CreatedAt
		event.PeriodAfter, event.InvoiceAfter = period, invoice
		return nil
	}
	if !apperrors.IsNotFound(err) {
		return err
	}
	sub, err := s.stores.Subscriptions.Get(ctx, input.SubscriptionID)
	if err != nil {
		return err
	}
	if err = financialScopeMatches(sub.FinancialScope(), command.Scope); err != nil {
		return err
	}
	if sub.Status != bizbilling.SubscriptionStatusActive && sub.Status != bizbilling.SubscriptionStatusPastDue && sub.Status != bizbilling.SubscriptionStatusTrialing {
		if input.ProviderProof == nil {
			return bizbilling.ErrInvalidMoney
		}
	}
	if input.ProviderProof == nil && (sub.CancelAtPeriodEnd || sub.NonRenewing) {
		return bizbilling.ErrInvalidMoney
	}
	if sub.EffectiveCollectionMode() == bizbilling.CollectionAutomatic && input.ProviderProof == nil {
		return bizbilling.ErrAutomaticCollectionInProgress
	}
	if pending := sub.PendingChange; pending != nil {
		sub.PlanID, sub.Quantity, sub.AddonIDs, sub.PriceSnapshot, sub.PendingChange = pending.PlanID, pending.Quantity, pending.AddonIDs, &pending.Price, nil
		sub.InitialPriceSnapshot = pending.InitialPrice
	}
	price := sub.PriceSnapshot
	providerPeriod := input.ProviderProof != nil
	providerCurrent := false
	if providerPeriod {
		proof := input.ProviderProof
		effect, err := s.gatewayEffects.Get(ctx, proof.EffectID)
		if err != nil {
			return err
		}
		if effect.Kind != "subscription_authorize" || effect.Scope != command.Scope || effect.SubscriptionID != sub.ID || effect.RecurringIntent == nil || effect.ProviderID != proof.Invoice.SubscriptionID || effect.Gateway != proof.Invoice.Gateway || proof.Invoice.ID != input.ProviderInvoiceID || !proof.Invoice.Start.Equal(input.Start) || !proof.Invoice.End.Equal(input.End) || proof.Invoice.Currency != command.Scope.Currency || proof.Invoice.AmountPaisa != effect.RecurringIntent.AmountPaisa || proof.Invoice.OrderID == "" {
			return bizbilling.ErrInvalidGatewayProof
		}
		price = &effect.RecurringIntent.Price
		providerCurrent = sub.EffectiveCollectionMode() == bizbilling.CollectionAutomatic && sub.AuthorizationEffectID == effect.ID && sub.AuthorizationGeneration == effect.SubscriptionGeneration
	}
	if price == nil {
		price, err = s.subscriptionPrice(ctx, command.Scope, sub, false)
		if err != nil {
			return err
		}
	}
	anchor, err := subscriptionAnchor(sub)
	if err != nil && !providerPeriod {
		return err
	}
	if providerPeriod && (anchor < 1 || sub.InitialAutomaticPeriod) {
		anchor = input.Start.Day()
	}
	expectedStart := sub.CurrentPeriodEnd
	if sub.Status == bizbilling.SubscriptionStatusTrialing && sub.TrialEnd != nil {
		expectedStart = *sub.TrialEnd
		anchor = expectedStart.Day()
	}
	if !providerPeriod && !input.Start.Equal(expectedStart) {
		return bizbilling.ErrConflict
	}
	expectedEnd, err := bizbilling.NextAnchoredPeriod(input.Start, price.BillingCycle, price.CycleDays, anchor)
	if err != nil {
		return err
	}
	if !providerPeriod && !input.End.Equal(expectedEnd) {
		return bizbilling.ErrInvalidPeriod
	}
	invoicePrice := price
	if sub.Status == bizbilling.SubscriptionStatusTrialing && !providerPeriod {
		invoicePrice = sub.InitialPriceSnapshot
		if invoicePrice == nil {
			invoicePrice, err = s.subscriptionPrice(ctx, command.Scope, sub, true)
			if err != nil {
				return err
			}
		}
	}
	if err = s.buildPeriodInvoice(ctx, command, event, sub, invoicePrice, input.Start, input.End, input.ProviderInvoiceID); err != nil {
		return err
	}
	if providerPeriod {
		event.InvoiceAfter.CollectionOwner = "razorpay:" + sub.ID
		event.InvoiceAfter.GatewayOrderID = input.ProviderProof.Invoice.OrderID
	}
	// Late invoices from a retired authorization still represent real bills,
	// but must never rewind today's subscription or replace its current price.
	if providerPeriod && (!providerCurrent || !sub.InitialAutomaticPeriod && !input.End.After(sub.CurrentPeriodEnd)) {
		return nil
	}
	sub.CurrentPeriodStart, sub.CurrentPeriodEnd = input.Start, input.End
	sub.AnchorDay, sub.PriceSnapshot, sub.LastPeriodID = anchor, price, id
	sub.InitialAutomaticPeriod, sub.InitialPriceSnapshot = false, nil
	if providerPeriod && sub.Status == bizbilling.SubscriptionStatusAwaitingAuthorization {
		sub.Status = bizbilling.SubscriptionStatusActive
	}
	if sub.Status == bizbilling.SubscriptionStatusTrialing {
		sub.Status = bizbilling.SubscriptionStatusActive
	}
	sub.LastSequence, sub.UpdatedAt = event.Sequence, event.CreatedAt
	event.PeriodAfter = &bizbilling.BillingPeriod{ID: id, Scope: command.Scope, SubscriptionID: sub.ID, Start: input.Start, End: input.End, InvoiceID: event.InvoiceAfter.ID, ProviderInvoiceID: input.ProviderInvoiceID, LastSequence: event.Sequence}
	event.SubscriptionAfter = sub
	event.Resources = append(event.Resources, bizbilling.ResourceRef{Kind: "subscription", ID: sub.ID})
	return nil
}

func subscriptionAnchor(sub *bizbilling.BizSubscription) (int, error) {
	if sub.AnchorDay > 0 && sub.AnchorDay <= 31 {
		return sub.AnchorDay, nil
	}
	if sub.CurrentPeriodStart.IsZero() || sub.CurrentPeriodEnd.IsZero() {
		return 0, bizbilling.ErrLegacyReviewRequired
	}
	start, end := sub.CurrentPeriodStart.UTC(), sub.CurrentPeriodEnd.UTC()
	if start.Day() == end.Day() {
		return start.Day(), nil
	}
	last := time.Date(end.Year(), end.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if end.Day() == last && start.Day() > end.Day() {
		return start.Day(), nil
	}
	return 0, bizbilling.ErrLegacyReviewRequired
}
