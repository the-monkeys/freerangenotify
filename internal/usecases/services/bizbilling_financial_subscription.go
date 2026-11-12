package services

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

func (s *BizOperationService) buildSubscriptionCollection(ctx context.Context, command bizbilling.Command, event *bizbilling.JournalEvent) error {
	input := command.SubscriptionCollection
	sub, err := s.stores.Subscriptions.Get(ctx, input.SubscriptionID)
	if err != nil {
		return err
	}
	if err = financialScopeMatches(sub.FinancialScope(), command.Scope); err != nil {
		return err
	}
	if input.Action != "request" && sub.AuthorizationGeneration != input.ExpectedGeneration {
		return bizbilling.ErrConflict
	}
	if sub.Status == bizbilling.SubscriptionStatusCanceled {
		return bizbilling.ErrInvalidMoney
	}
	switch input.Action {
	case "request":
		if input.AutomaticCycles != nil {
			if *input.AutomaticCycles < 1 || *input.AutomaticCycles > 120 {
				return bizbilling.ErrInvalidMoney
			}
			if sub.AuthorizationEffectID != "" && sub.CollectionState != "cancelled" && *input.AutomaticCycles != sub.AutomaticCycles {
				return bizbilling.ErrAutomaticCollectionInProgress
			}
			sub.AutomaticCycles = *input.AutomaticCycles
		}
		if input.Mode == bizbilling.CollectionInvoice && (sub.GatewaySubscriptionID != "" || sub.AuthorizationEffectID != "") && sub.CollectionState != "cancelled" {
			return bizbilling.ErrAutomaticCollectionInProgress
		}
		if input.Mode == bizbilling.CollectionAutomatic {
			if sub.AutomaticCycles < 1 || sub.AutomaticCycles > 120 {
				return bizbilling.ErrInvalidMoney
			}
			if sub.EffectiveCollectionMode() != bizbilling.CollectionAutomatic || sub.CollectionState == "cancelled" {
				sub.AuthorizationGeneration++
				sub.AuthorizationEffectID, sub.GatewaySubscriptionID, sub.GatewayBinding = "", "", nil
				sub.CollectionState = "awaiting_authorization"
			}
			if sub.CollectionState == "" {
				sub.CollectionState = "awaiting_authorization"
			}
		} else {
			sub.CollectionState = "invoice"
		}
		sub.CollectionMode = input.Mode
	case "bind":
		if sub.EffectiveCollectionMode() != bizbilling.CollectionAutomatic || input.Gateway == nil || input.EffectID == "" || input.Price == nil {
			return bizbilling.ErrInvalidMoney
		}
		if sub.AuthorizationEffectID != "" && sub.AuthorizationEffectID != input.EffectID {
			return bizbilling.ErrConflict
		}
		if input.Gateway.Mode != sub.EffectiveGatewayMode() {
			return bizbilling.ErrModeMismatch
		}
		sub.AuthorizationEffectID, sub.GatewayBinding, sub.PriceSnapshot = input.EffectID, input.Gateway, input.Price
		sub.CollectionState = "awaiting_authorization"
	case "confirm":
		if sub.CollectionState == "change_pending" {
			return bizbilling.ErrAutomaticCollectionInProgress
		}
		if sub.EffectiveCollectionMode() != bizbilling.CollectionAutomatic || sub.AuthorizationEffectID != input.EffectID || sub.GatewayBinding == nil || input.Gateway == nil || *sub.GatewayBinding != *input.Gateway || input.RemoteID == "" {
			return bizbilling.ErrInvalidGatewayProof
		}
		if sub.GatewaySubscriptionID != "" && sub.GatewaySubscriptionID != input.RemoteID {
			return bizbilling.ErrConflict
		}
		switch input.State {
		case "awaiting_authorization", "awaiting_account_verification", "authorized", "active", "pending", "halted", "paused", "completed", "expired":
		default:
			return bizbilling.ErrInvalidGatewayProof
		}
		sub.GatewaySubscriptionID, sub.CollectionState = input.RemoteID, input.State
	case "pending":
		if sub.EffectiveCollectionMode() != bizbilling.CollectionAutomatic {
			return bizbilling.ErrConflict
		}
		sub.CollectionState = "change_pending"
		if input.RemoteID != "" {
			if sub.AuthorizationEffectID != input.EffectID || sub.GatewayBinding == nil || input.Gateway == nil || *sub.GatewayBinding != *input.Gateway || sub.GatewaySubscriptionID != "" && sub.GatewaySubscriptionID != input.RemoteID {
				return bizbilling.ErrInvalidGatewayProof
			}
			sub.GatewaySubscriptionID = input.RemoteID
		}
	case "cancel_unattempted":
		if sub.GatewaySubscriptionID != "" || sub.AuthorizationEffectID != input.EffectID || sub.GatewayBinding == nil || input.Gateway == nil || *sub.GatewayBinding != *input.Gateway {
			return bizbilling.ErrInvalidGatewayProof
		}
		sub.CollectionMode, sub.CollectionState = bizbilling.CollectionInvoice, "cancelled"
	case "cancel":
		if sub.GatewaySubscriptionID != input.RemoteID || sub.GatewayBinding == nil || input.Gateway == nil || *sub.GatewayBinding != *input.Gateway {
			return bizbilling.ErrInvalidGatewayProof
		}
		sub.CollectionMode, sub.CollectionState = bizbilling.CollectionInvoice, "cancelled"
	default:
		return bizbilling.ErrInvalidMoney
	}
	sub.LastSequence, sub.UpdatedAt = event.Sequence, event.CreatedAt
	event.SubscriptionAfter = sub
	event.Resources = []bizbilling.ResourceRef{{Kind: "subscription", ID: sub.ID}}
	return nil
}
