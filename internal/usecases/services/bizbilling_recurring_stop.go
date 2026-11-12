package services

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (s *BizRecurringService) stopAuthorization(ctx context.Context, sub *bizbilling.BizSubscription, target *bizbilling.GatewayEffect, key string) (bizbilling.OperationResult, error) {
	var empty bizbilling.OperationResult
	if target.Kind != "subscription_authorize" || target.Scope != sub.FinancialScope() || target.SubscriptionID != sub.ID || target.RecurringIntent == nil {
		return empty, bizbilling.ErrReconciliationRequired
	}
	if target.State == "stopped" && target.Result != nil {
		return *target.Result, nil
	}
	if previous, found, err := s.savedStopResult(ctx, target); found || err != nil {
		if err != nil {
			return previous, err
		}
		return s.finishStoppedAuthorization(ctx, target, previous)
	}
	if sub.AuthorizationGeneration != target.SubscriptionGeneration || sub.AuthorizationEffectID != target.ID {
		return empty, bizbilling.ErrConflict
	}
	current, err := s.effects.Update(ctx, target.ID, func(row *bizbilling.GatewayEffect) error {
		row.StopRequested = true
		row.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return empty, bizbilling.ErrGatewayPending
	}
	if !current.Attempted {
		// The same CAS consulted by every subscription POST now prevents any
		// future attempt. A spare provider plan cannot debit a customer.
		result, err := s.applyCollection(ctx, sub, bizbilling.SubscriptionCollection{Action: "cancel_unattempted", Mode: bizbilling.CollectionInvoice, EffectID: current.ID, Gateway: &current.Gateway, ExpectedGeneration: current.SubscriptionGeneration}, current.ID+":stop-unattempted")
		if err != nil {
			return result, err
		}
		return s.finishStoppedAuthorization(ctx, current, result)
	}
	gateway, err := s.gateway(ctx, current.Gateway)
	if err != nil {
		return empty, err
	}
	var remote *bizbilling.GatewaySubscription
	if current.ProviderID != "" {
		fetched, fetchErr := gateway.FetchSubscription(ctx, current.ProviderID)
		if fetchErr != nil {
			return empty, fetchErr
		}
		remote = &fetched
	} else {
		remote, err = gateway.FindSubscription(ctx, *current.RecurringIntent)
		if err != nil {
			return empty, err
		}
		if remote == nil {
			return empty, bizbilling.ErrGatewayPending
		}
	}
	if !recurringSubscriptionMatches(*remote, *current.RecurringIntent) {
		return empty, bizbilling.ErrInvalidGatewayProof
	}
	_, err = s.effects.Update(ctx, current.ID, func(row *bizbilling.GatewayEffect) error {
		if row.ProviderID != "" && row.ProviderID != remote.ID {
			return bizbilling.ErrConflict
		}
		row.ProviderID, row.RecurringSubscription = remote.ID, remote
		row.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return empty, bizbilling.ErrGatewayPending
	}
	_, err = s.applyCollection(ctx, sub, bizbilling.SubscriptionCollection{Action: "pending", Mode: bizbilling.CollectionAutomatic, EffectID: current.ID, RemoteID: remote.ID, Gateway: &current.Gateway, ExpectedGeneration: current.SubscriptionGeneration}, gatewayEffectID("stop_link", current.ID, remote.ID))
	if err != nil {
		return empty, err
	}
	sub, err = s.lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	if err != nil {
		return empty, err
	}
	result, err := s.cancelAutomatic(ctx, sub, key)
	if err != nil {
		return result, err
	}
	return s.finishStoppedAuthorization(ctx, current, result)
}

func (s *BizRecurringService) finishStoppedAuthorization(ctx context.Context, effect *bizbilling.GatewayEffect, result bizbilling.OperationResult) (bizbilling.OperationResult, error) {
	_, err := s.effects.Update(ctx, effect.ID, func(row *bizbilling.GatewayEffect) error {
		row.StopRequested, row.State, row.Result = true, "stopped", &result
		row.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return result, bizbilling.ErrGatewayPending
	}
	return result, nil
}

func (s *BizRecurringService) savedStopResult(ctx context.Context, effect *bizbilling.GatewayEffect) (bizbilling.OperationResult, bool, error) {
	keys := []string{effect.ID + ":stop-unattempted"}
	if effect.ProviderID != "" {
		keys = append(keys, gatewayEffectID("subscription_cancel", effect.Scope.AppID, effect.SubscriptionID, effect.ProviderID)+":settled")
	}
	for _, key := range keys {
		op, err := s.lifecycle.operations.operations.Get(ctx, bizbillingrepo.OperationIDForKey(effect.Scope, key))
		if apperrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return bizbilling.OperationResult{}, false, err
		}
		input := op.Command.SubscriptionCollection
		if op.Scope != effect.Scope || op.Command.Kind != bizbilling.CommandSubscriptionCollection || input == nil || (input.Action != "cancel" && input.Action != "cancel_unattempted") || input.SubscriptionID != effect.SubscriptionID || input.Mode != bizbilling.CollectionInvoice || input.ExpectedGeneration != effect.SubscriptionGeneration || input.Gateway == nil || *input.Gateway != effect.Gateway {
			return bizbilling.OperationResult{}, false, bizbilling.ErrReconciliationRequired
		}
		if op.State == bizbilling.OperationCompleted {
			return op.Result(), true, nil
		}
		result, err := s.lifecycle.operations.Resume(ctx, effect.Scope.AppID, op.ID)
		return result, err == nil, err
	}
	return bizbilling.OperationResult{}, false, nil
}
