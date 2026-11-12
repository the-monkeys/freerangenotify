package services

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

// A stopped mandate no longer owns invoice collection. Derive this from a
// committed, scoped lifecycle transition, never a client-supplied preference.
func (s *BizOperationService) releaseStoppedInvoiceOwner(ctx context.Context, invoice *bizbilling.BizInvoice) error {
	if invoice.SubscriptionID == "" || invoice.CollectionOwner != "razorpay:"+invoice.SubscriptionID {
		return nil
	}
	sub, err := s.stores.Subscriptions.Get(ctx, invoice.SubscriptionID)
	if apperrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if sub.FinancialScope() != invoice.FinancialScope() || sub.LastSequence < 1 || sub.EffectiveCollectionMode() != bizbilling.CollectionInvoice || sub.CollectionState != "cancelled" || sub.NativeTransitionID != "" {
		return nil
	}
	if err = guardFinancialRead(ctx, s.gate, s, sub.FinancialScope(), sub.LastSequence); err != nil {
		return err
	}
	invoice.CollectionOwner = ""
	return nil
}

func (s *BizRecurringService) completeInvoiceCollectionChoice(ctx context.Context, app, subscription, key string, previous bizbilling.OperationResult) (bizbilling.OperationResult, error) {
	sub, err := s.lifecycle.GetSubscription(ctx, app, subscription)
	if err != nil {
		return previous, err
	}
	startKey := gatewayEffectID("invoice_collection_start", app, key)
	id := bizbillingrepo.OperationIDForKey(sub.FinancialScope(), startKey)
	if _, err = s.lifecycle.operations.operations.Get(ctx, id); err == nil {
		return s.lifecycle.operations.Resume(ctx, app, id)
	} else if !apperrors.IsNotFound(err) {
		return previous, err
	}
	if !sub.InitialAutomaticPeriod || sub.Status == bizbilling.SubscriptionStatusCanceled {
		return previous, nil
	}
	return s.lifecycle.operations.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandSubscriptionChange, Scope: sub.FinancialScope(), IdempotencyKey: startKey, SubscriptionChange: &bizbilling.SubscriptionChange{SubscriptionID: sub.ID, Action: "start_invoice"}})
}

func (s *BizOperationService) startInvoiceSubscription(ctx context.Context, command bizbilling.Command, event *bizbilling.JournalEvent, sub *bizbilling.BizSubscription) error {
	if sub.EffectiveCollectionMode() != bizbilling.CollectionInvoice || sub.NativeTransitionID != "" {
		return bizbilling.ErrAutomaticCollectionInProgress
	}
	if !sub.InitialAutomaticPeriod {
		return nil
	}
	price := sub.PriceSnapshot
	if _, err := validatedSnapshotTotals(price, command.Scope); err != nil {
		return err
	}
	if sub.Status == bizbilling.SubscriptionStatusTrialing {
		sub.InitialAutomaticPeriod, sub.InitialPriceSnapshot = false, price
		return nil
	}
	if sub.Status != bizbilling.SubscriptionStatusAwaitingAuthorization {
		return bizbilling.ErrInvalidMoney
	}
	start := event.CreatedAt
	end, err := bizbilling.NextAnchoredPeriod(start, price.BillingCycle, price.CycleDays, start.Day())
	if err != nil {
		return err
	}
	if err = s.buildPeriodInvoice(ctx, command, event, sub, price, start, end, ""); err != nil {
		return err
	}
	sub.Status, sub.CurrentPeriodStart, sub.CurrentPeriodEnd = bizbilling.SubscriptionStatusActive, start, end
	sub.AnchorDay, sub.InitialAutomaticPeriod = start.Day(), false
	return nil
}
