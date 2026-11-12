package services

import (
	"context"
	"math/big"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

func (s *BizOperationService) buildSubscriptionChange(ctx context.Context, command bizbilling.Command, event *bizbilling.JournalEvent) error {
	input := command.SubscriptionChange
	sub, err := s.stores.Subscriptions.Get(ctx, input.SubscriptionID)
	if err != nil {
		return err
	}
	if err = financialScopeMatches(sub.FinancialScope(), command.Scope); err != nil {
		return err
	}
	// A provider-owned mandate must confirm a native transition first.
	if sub.EffectiveCollectionMode() == bizbilling.CollectionAutomatic {
		if err = s.checkNativeSubscriptionChange(ctx, command, sub); err != nil {
			return err
		}
	} else if input.NativeEffectID != "" || input.Action == "native_pending" {
		return bizbilling.ErrInvalidMoney
	}
	now := event.CreatedAt
	switch input.Action {
	case "start_invoice":
		if err = s.startInvoiceSubscription(ctx, command, event, sub); err != nil {
			return err
		}
	case "native_pending":
		sub.NativeTransitionID, sub.CollectionState = input.NativeEffectID, "native_change_pending"
	case "pause":
		if sub.Status != bizbilling.SubscriptionStatusActive && sub.Status != bizbilling.SubscriptionStatusPastDue {
			return bizbilling.ErrInvalidMoney
		}
		sub.Status, sub.PausedAt = bizbilling.SubscriptionStatusPaused, &now
	case "resume":
		if sub.Status != bizbilling.SubscriptionStatusPaused {
			return bizbilling.ErrInvalidMoney
		}
		sub.Status, sub.PausedAt = bizbilling.SubscriptionStatusActive, nil
	case "cancel":
		if input.AtPeriodEnd {
			sub.CancelAtPeriodEnd = true
		} else {
			sub.Status, sub.CanceledAt = bizbilling.SubscriptionStatusCanceled, &now
		}
	case "reactivate":
		if sub.Status != bizbilling.SubscriptionStatusCanceled {
			return bizbilling.ErrInvalidMoney
		}
		sub.Status, sub.CanceledAt, sub.CancelAtPeriodEnd, sub.NonRenewing = bizbilling.SubscriptionStatusActive, nil, false, false
	case "update":
		request := input.Update
		if sub.Status != bizbilling.SubscriptionStatusActive && sub.Status != bizbilling.SubscriptionStatusTrialing && sub.Status != bizbilling.SubscriptionStatusPastDue {
			return bizbilling.ErrInvalidMoney
		}
		if request.PlanID != nil || request.Quantity != nil || request.AddonIDs != nil {
			if err = s.buildSubscriptionPriceChange(ctx, command, event, sub); err != nil {
				return err
			}
		}
		if request.CancelAtPeriodEnd != nil {
			sub.CancelAtPeriodEnd = *request.CancelAtPeriodEnd
		}
		if request.NonRenewing != nil {
			sub.NonRenewing = *request.NonRenewing
		}
		if request.Metadata != nil {
			sub.Metadata = request.Metadata
		}
	default:
		return bizbilling.ErrInvalidMoney
	}
	if sub.EffectiveCollectionMode() == bizbilling.CollectionAutomatic && input.Action != "native_pending" {
		sub.NativeTransitionID = ""
		if input.Action == "pause" {
			sub.CollectionState = "paused"
		} else {
			sub.CollectionState = "active"
		}
	}
	sub.LastSequence, sub.UpdatedAt = event.Sequence, now
	event.SubscriptionAfter = sub
	event.Resources = append(event.Resources, bizbilling.ResourceRef{Kind: "subscription", ID: sub.ID})
	return nil
}

func (s *BizOperationService) buildSubscriptionPriceChange(ctx context.Context, command bizbilling.Command, event *bizbilling.JournalEvent, sub *bizbilling.BizSubscription) error {
	request := command.SubscriptionChange.Update
	oldPrice := sub.PriceSnapshot
	var err error
	if oldPrice == nil {
		oldPrice, err = s.subscriptionPrice(ctx, command.Scope, sub, false)
		if err != nil {
			return err
		}
	}
	target := *sub
	if request.PlanID != nil {
		target.PlanID = *request.PlanID
	}
	if request.Quantity != nil {
		target.Quantity = *request.Quantity
	}
	if request.AddonIDs != nil {
		target.AddonIDs = append([]string(nil), request.AddonIDs...)
	}
	plan, err := s.stores.Plans.Get(ctx, target.PlanID)
	if err != nil {
		return err
	}
	if plan.AppID != command.Scope.AppID || !plan.Active {
		return bizbilling.ErrInvalidMoney
	}
	price, err := s.subscriptionPrice(ctx, command.Scope, &target, false)
	if err != nil {
		return err
	}
	var first *bizbilling.SubscriptionPriceSnapshot
	if sub.Status == bizbilling.SubscriptionStatusTrialing {
		first, err = s.subscriptionPrice(ctx, command.Scope, &target, true)
		if err != nil {
			return err
		}
	}
	if request.ChangeTiming == "next_period" {
		sub.PendingChange = &bizbilling.PendingSubscriptionChange{InitialPrice: first, PlanID: target.PlanID, Quantity: target.Quantity, AddonIDs: target.AddonIDs, Price: *price}
		return nil
	}
	if sub.Status != bizbilling.SubscriptionStatusTrialing {
		credit, err := proratedMinorUnits(oldPrice.SubtotalPaisa, sub.CurrentPeriodStart, sub.CurrentPeriodEnd, event.CreatedAt)
		if err != nil {
			return err
		}
		charge, err := proratedMinorUnits(price.SubtotalPaisa, sub.CurrentPeriodStart, sub.CurrentPeriodEnd, event.CreatedAt)
		if err != nil {
			return err
		}
		if charge > 0 {
			draft := command
			draft.InvoiceDraft = &bizbilling.InvoiceDraft{SubscriptionID: sub.ID, InvoiceType: bizbilling.InvoiceTypeRecurring, Request: bizbilling.CreateInvoiceRequest{UserID: sub.UserID, Currency: sub.Currency, GatewayMode: sub.EffectiveGatewayMode(), LineItems: []bizbilling.InvoiceLineItem{{Description: price.PlanName + " (prorated)", Quantity: 1, UnitPrice: charge}}, PaymentTerms: price.PaymentTerms}}
			if err = s.buildFinancialInvoiceDraft(ctx, draft, event); err != nil {
				return err
			}
			if err = s.issueFinancialInvoice(ctx, event, event.InvoiceAfter); err != nil {
				return err
			}
		}
		if credit > 0 {
			number := "TEST-CN-" + event.OperationID
			if command.Scope.Mode != bizbilling.GatewayTest {
				number, err = s.config.NextNumber(ctx, sub.AppID, NumberKindCreditNote)
				if err != nil {
					return err
				}
			}
			event.CreditNoteAfter = &bizbilling.CreditNote{ID: event.OperationID + "-credit-note", AppID: sub.AppID, UserID: sub.UserID, Currency: sub.Currency, GatewayMode: sub.EffectiveGatewayMode(), LastSequence: event.Sequence, CreditNoteNumber: number, AmountPaisa: credit, BalancePaisa: credit, Status: bizbilling.CreditNoteStatusOpen, Reason: "Unused subscription period: " + sub.ID, CreatedAt: event.CreatedAt, UpdatedAt: event.CreatedAt}
			event.Resources = append(event.Resources, bizbilling.ResourceRef{Kind: "credit_note", ID: event.CreditNoteAfter.ID})
		}
	}
	sub.PlanID, sub.Quantity, sub.AddonIDs, sub.PriceSnapshot, sub.PendingChange = target.PlanID, target.Quantity, target.AddonIDs, price, nil
	sub.InitialPriceSnapshot = first
	return nil
}

// Integer arithmetic avoids float rounding and overflow for large invoices.
func proratedMinorUnits(amount int64, start, end, now time.Time) (int64, error) {
	if amount < 0 || !end.After(start) {
		return 0, bizbilling.ErrInvalidMoney
	}
	if !now.Before(end) {
		return 0, nil
	}
	if now.Before(start) {
		now = start
	}
	value := new(big.Int).Mul(big.NewInt(amount), big.NewInt(int64(end.Sub(now))))
	value.Quo(value, big.NewInt(int64(end.Sub(start))))
	if !value.IsInt64() {
		return 0, bizbilling.ErrMoneyOverflow
	}
	return value.Int64(), nil
}
