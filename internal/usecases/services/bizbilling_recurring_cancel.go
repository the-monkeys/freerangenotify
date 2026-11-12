package services

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (s *BizRecurringService) cancelAutomatic(ctx context.Context, sub *bizbilling.BizSubscription, key string) (bizbilling.OperationResult, error) {
	aliasID := gatewayEffectID("recurring_cancel_key", sub.AppID, key)
	hash := gatewayEffectID("cancel_request", sub.AppID, sub.UserID, sub.ID)
	alias, err := s.effects.Get(ctx, aliasID)
	var effect *bizbilling.GatewayEffect
	if err == nil {
		if alias.RequestHash != hash || alias.Kind != "recurring_cancel_key" {
			return bizbilling.OperationResult{}, bizbilling.ErrIdempotencyConflict
		}
		effect, err = s.effects.Get(ctx, alias.NextEffectID)
		if err != nil {
			return bizbilling.OperationResult{}, err
		}
	} else {
		if !apperrors.IsNotFound(err) {
			return bizbilling.OperationResult{}, err
		}
		if sub.GatewayBinding == nil || sub.GatewaySubscriptionID == "" || sub.AuthorizationEffectID == "" {
			return bizbilling.OperationResult{}, bizbilling.ErrReconciliationRequired
		}
		original, err := s.effects.Get(ctx, sub.AuthorizationEffectID)
		if err != nil {
			return bizbilling.OperationResult{}, err
		}
		if original.Scope != sub.FinancialScope() || original.Gateway != *sub.GatewayBinding || original.RecurringIntent == nil {
			return bizbilling.OperationResult{}, bizbilling.ErrReconciliationRequired
		}
		id := gatewayEffectID("subscription_cancel", sub.AppID, sub.ID, sub.GatewaySubscriptionID)
		now := time.Now().UTC()
		row := &bizbilling.GatewayEffect{ID: id, Kind: "subscription_cancel", Scope: original.Scope, Gateway: original.Gateway, RecurringIntent: original.RecurringIntent, ProviderID: sub.GatewaySubscriptionID, SubscriptionID: sub.ID, SubscriptionGeneration: sub.AuthorizationGeneration, SubscriptionAction: bizbilling.SubscriptionCancel, State: "pending", CreatedAt: now, UpdatedAt: now}
		_, _ = s.effects.CreateIfAbsent(ctx, id, row)
		effect, err = s.effects.Get(ctx, id)
		if err != nil {
			return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
		}
		alias = &bizbilling.GatewayEffect{ID: aliasID, Kind: "recurring_cancel_key", Scope: sub.FinancialScope(), RequestHash: hash, NextEffectID: id}
		_, _ = s.effects.CreateIfAbsent(ctx, aliasID, alias)
		confirmed, err := s.effects.Get(ctx, aliasID)
		if err != nil {
			return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
		}
		if confirmed.RequestHash != hash || confirmed.NextEffectID != id {
			return bizbilling.OperationResult{}, bizbilling.ErrIdempotencyConflict
		}
	}
	result, err := s.resumeCancelEffect(ctx, sub, effect)
	if err != nil && result.OperationID == "" {
		result = bizbilling.OperationResult{OperationID: effect.ID, State: bizbilling.OperationAccepted}
	}
	return result, err
}

func (s *BizRecurringService) resumeCancelEffect(ctx context.Context, sub *bizbilling.BizSubscription, effect *bizbilling.GatewayEffect) (bizbilling.OperationResult, error) {
	var err error
	if effect.Scope != sub.FinancialScope() || effect.SubscriptionID != sub.ID || effect.Kind != "subscription_cancel" || effect.RecurringIntent == nil {
		return bizbilling.OperationResult{}, bizbilling.ErrReconciliationRequired
	}
	if effect.State == "settled" && effect.Result != nil {
		return *effect.Result, nil
	}
	if sub.AuthorizationGeneration != effect.SubscriptionGeneration || sub.GatewaySubscriptionID != effect.ProviderID || sub.GatewayBinding == nil || *sub.GatewayBinding != effect.Gateway {
		return bizbilling.OperationResult{}, bizbilling.ErrConflict
	}
	_, err = s.applyCollection(ctx, sub, bizbilling.SubscriptionCollection{Action: "pending", Mode: bizbilling.CollectionAutomatic, ExpectedGeneration: effect.SubscriptionGeneration}, effect.ID+":pending")
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	gateway, err := s.gateway(ctx, effect.Gateway)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	remote, err := gateway.FetchSubscription(ctx, effect.ProviderID)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	if !recurringSubscriptionMatches(remote, *effect.RecurringIntent) || remote.ID != effect.ProviderID {
		return bizbilling.OperationResult{}, bizbilling.ErrInvalidGatewayProof
	}
	if !automaticStopped(remote.Status) {
		won := false
		_, err = s.effects.Update(ctx, effect.ID, func(row *bizbilling.GatewayEffect) error {
			won = !row.Attempted
			row.Attempted = true
			row.UpdatedAt = time.Now().UTC()
			return nil
		})
		if err != nil {
			return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
		}
		if !won {
			return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
		}
		remote, err = gateway.ChangeSubscription(ctx, effect.ProviderID, bizbilling.SubscriptionCancel)
		if err != nil {
			return bizbilling.OperationResult{}, err
		}
		if !recurringSubscriptionMatches(remote, *effect.RecurringIntent) || remote.ID != effect.ProviderID {
			return bizbilling.OperationResult{}, bizbilling.ErrInvalidGatewayProof
		}
		if !automaticStopped(remote.Status) {
			return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
		}
	}
	result, err := s.applyCollection(ctx, sub, bizbilling.SubscriptionCollection{Action: "cancel", Mode: bizbilling.CollectionInvoice, RemoteID: effect.ProviderID, Gateway: &effect.Gateway, ExpectedGeneration: effect.SubscriptionGeneration}, effect.ID+":settled")
	if err != nil {
		return result, err
	}
	_, err = s.effects.Update(ctx, effect.ID, func(row *bizbilling.GatewayEffect) error {
		row.State, row.Result = "settled", &result
		row.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return result, bizbilling.ErrGatewayPending
	}
	return result, nil
}

func automaticStopped(status string) bool {
	return status == "cancelled" || status == "completed" || status == "expired"
}
