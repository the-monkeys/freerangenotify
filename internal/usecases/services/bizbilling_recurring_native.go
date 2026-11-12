package services

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (s *BizOperationService) checkNativeSubscriptionChange(ctx context.Context, command bizbilling.Command, sub *bizbilling.BizSubscription) error {
	input := command.SubscriptionChange
	if input.NativeEffectID == "" || sub.GatewayBinding == nil || sub.GatewaySubscriptionID == "" {
		return bizbilling.ErrAutomaticCollectionInProgress
	}
	effect, err := s.gatewayEffects.Get(ctx, input.NativeEffectID)
	if err != nil {
		return err
	}
	if effect.Scope != command.Scope || effect.SubscriptionID != sub.ID || effect.Gateway != *sub.GatewayBinding || effect.ProviderID != sub.GatewaySubscriptionID || effect.SubscriptionGeneration != sub.AuthorizationGeneration || effect.Kind != "subscription_native_"+string(effect.SubscriptionAction) {
		return bizbilling.ErrInvalidGatewayProof
	}
	if effect.SubscriptionAction != bizbilling.SubscriptionPause && effect.SubscriptionAction != bizbilling.SubscriptionResume {
		return bizbilling.ErrInvalidGatewayProof
	}
	if sub.NativeTransitionID != "" && sub.NativeTransitionID != effect.ID {
		return bizbilling.ErrAutomaticCollectionInProgress
	}
	if input.Action == "native_pending" {
		original, err := s.gatewayEffects.Get(ctx, sub.AuthorizationEffectID)
		if err != nil {
			return err
		}
		if original.StopRequested || sub.CollectionState == "change_pending" {
			return bizbilling.ErrAutomaticCollectionInProgress
		}
		if effect.SubscriptionAction == bizbilling.SubscriptionPause && sub.Status != bizbilling.SubscriptionStatusActive && sub.Status != bizbilling.SubscriptionStatusPastDue {
			return bizbilling.ErrInvalidMoney
		}
		if effect.SubscriptionAction == bizbilling.SubscriptionResume && sub.Status != bizbilling.SubscriptionStatusPaused {
			return bizbilling.ErrInvalidMoney
		}

		if effect.State != "pending" && effect.State != "provider_confirmed" {
			return bizbilling.ErrConflict
		}
		return nil
	}
	if sub.NativeTransitionID != effect.ID || input.Action != string(effect.SubscriptionAction) || effect.State != "provider_confirmed" {
		return bizbilling.ErrAutomaticCollectionInProgress
	}
	return nil
}

func (s *BizRecurringService) changeNative(ctx context.Context, sub *bizbilling.BizSubscription, action bizbilling.SubscriptionAction, key string) (*bizbilling.BizSubscription, error) {
	ctx, done, err := BeginBizMutation(ctx, s.lifecycle.gate, sub.AppID)
	if err != nil {
		return nil, err
	}
	defer releaseBizMutation(s.logger, done)
	if !recurringKeyValid(key) {
		return nil, bizbilling.ErrInvalidMoney
	}
	id := gatewayEffectID("subscription_native_key", sub.AppID, key)
	hash := gatewayEffectID("subscription_native_request", sub.AppID, sub.UserID, sub.ID, string(action))
	effect, err := s.effects.Get(ctx, id)
	if apperrors.IsNotFound(err) {
		if sub.GatewayBinding == nil || sub.GatewaySubscriptionID == "" || sub.AuthorizationEffectID == "" || sub.NativeTransitionID != "" {
			return nil, bizbilling.ErrAutomaticCollectionInProgress
		}
		if action == bizbilling.SubscriptionPause && sub.Status != bizbilling.SubscriptionStatusActive && sub.Status != bizbilling.SubscriptionStatusPastDue {
			return nil, bizbilling.ErrInvalidMoney
		}
		if action == bizbilling.SubscriptionResume && sub.Status != bizbilling.SubscriptionStatusPaused {
			return nil, bizbilling.ErrInvalidMoney
		}
		original, originalErr := s.effects.Get(ctx, sub.AuthorizationEffectID)
		if originalErr != nil {
			return nil, originalErr
		}
		if original.Scope != sub.FinancialScope() || original.Gateway != *sub.GatewayBinding || original.RecurringIntent == nil {
			return nil, bizbilling.ErrInvalidGatewayProof
		}
		preflightGateway, preflightErr := s.gateway(ctx, original.Gateway)
		if preflightErr != nil {
			return nil, preflightErr
		}
		remote, preflightErr := preflightGateway.FetchSubscription(ctx, sub.GatewaySubscriptionID)
		if preflightErr != nil {
			return nil, preflightErr
		}
		if !recurringSubscriptionMatches(remote, *original.RecurringIntent) {
			return nil, bizbilling.ErrInvalidGatewayProof
		}
		// Razorpay only pauses active mandates; pausing an authenticated
		// future mandate would cancel it. Reject before persisting a fence.
		if action == bizbilling.SubscriptionPause && remote.Status != "active" && remote.Status != "paused" {
			return nil, apperrors.Conflict("Automatic billing has not started or cannot be paused; stop automatic collection to use invoice billing")
		}
		if action == bizbilling.SubscriptionResume && remote.Status != "paused" && remote.Status != "active" {
			return nil, apperrors.Conflict("This automatic mandate cannot be resumed; use invoice collection or request fresh authorization")
		}
		now := time.Now().UTC()
		row := &bizbilling.GatewayEffect{ID: id, Kind: "subscription_native_" + string(action), Scope: sub.FinancialScope(), Gateway: original.Gateway, RecurringIntent: original.RecurringIntent, SubscriptionID: sub.ID, ProviderID: sub.GatewaySubscriptionID, SubscriptionGeneration: sub.AuthorizationGeneration, SubscriptionAction: action, RequestHash: hash, State: "pending", CreatedAt: now, UpdatedAt: now}
		_, _ = s.effects.CreateIfAbsent(ctx, id, row)
		effect, err = s.effects.Get(ctx, id)
	}
	if err != nil {
		return nil, err
	}
	if effect.RequestHash != hash {
		return nil, bizbilling.ErrIdempotencyConflict
	}
	result, err := s.resumeNative(ctx, sub, effect)
	if err != nil {
		return nil, &bizbilling.OperationFailure{Result: result, Cause: err}
	}
	event, err := s.lifecycle.operations.completedFinancialEvent(ctx, sub.AppID, result.OperationID)
	if err != nil {
		return nil, err
	}
	return event.SubscriptionAfter, nil
}

func (s *BizRecurringService) resumeNative(ctx context.Context, sub *bizbilling.BizSubscription, effect *bizbilling.GatewayEffect) (bizbilling.OperationResult, error) {
	pending := bizbilling.OperationResult{OperationID: effect.ID, State: bizbilling.OperationAccepted}
	if effect.State == "settled" && effect.Result != nil {
		return *effect.Result, nil
	}
	if effect.State == "rejected" && effect.Result != nil {
		if effect.FailureCode == "mandate_ended" {
			return *effect.Result, bizbilling.ErrMerchantUnavailable
		}
		return *effect.Result, bizbilling.ErrInvalidMoney
	}
	if effect.Scope != sub.FinancialScope() || effect.SubscriptionID != sub.ID || effect.SubscriptionGeneration != sub.AuthorizationGeneration || sub.GatewayBinding == nil || effect.Gateway != *sub.GatewayBinding || effect.ProviderID != sub.GatewaySubscriptionID || effect.RecurringIntent == nil {
		return pending, bizbilling.ErrConflict
	}
	if effect.State == "pending" {
		bound, err := s.lifecycle.operations.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandSubscriptionChange, Scope: effect.Scope, IdempotencyKey: effect.ID + ":pending", SubscriptionChange: &bizbilling.SubscriptionChange{SubscriptionID: sub.ID, Action: "native_pending", NativeEffectID: effect.ID}})
		if err != nil {
			if bound.State == bizbilling.OperationRejected {
				_, _ = s.effects.Update(ctx, effect.ID, func(current *bizbilling.GatewayEffect) error {
					current.State, current.Result, current.FailureCode = "rejected", &bound, bound.ErrorCode
					return nil
				})
				return bound, err
			}
			return pending, err
		}
		gateway, err := s.gateway(ctx, effect.Gateway)
		if err != nil {
			return pending, err
		}
		remote, err := gateway.FetchSubscription(ctx, effect.ProviderID)
		if err != nil {
			return pending, err
		}
		if !recurringSubscriptionMatches(remote, *effect.RecurringIntent) {
			return pending, bizbilling.ErrInvalidGatewayProof
		}
		if automaticStopped(remote.Status) {
			// A customer/provider may end the mandate independently while a
			// pause/resume response is uncertain. Release the fence truthfully;
			// do not retry an impossible transition or claim it succeeded.
			result, stopErr := s.applyCollection(ctx, sub, bizbilling.SubscriptionCollection{Action: "cancel", Mode: bizbilling.CollectionInvoice, RemoteID: effect.ProviderID, Gateway: &effect.Gateway, ExpectedGeneration: effect.SubscriptionGeneration}, effect.ID+":mandate-ended")
			if stopErr != nil {
				return result, stopErr
			}
			_, saveErr := s.effects.Update(ctx, effect.ID, func(current *bizbilling.GatewayEffect) error {
				current.State, current.Result, current.FailureCode = "rejected", &result, "mandate_ended"
				return nil
			})
			if saveErr != nil {
				return result, bizbilling.ErrGatewayPending
			}
			return result, bizbilling.ErrMerchantUnavailable
		}
		target := "paused"
		if effect.SubscriptionAction == bizbilling.SubscriptionResume {
			target = "active"
		}
		if remote.Status != target {
			won := false
			_, err = s.effects.Update(ctx, effect.ID, func(current *bizbilling.GatewayEffect) error {
				won = false
				if !current.Attempted {
					current.Attempted, won = true, true
				}
				return nil
			})
			if err != nil {
				return pending, bizbilling.ErrGatewayPending
			}
			if !won {
				return pending, bizbilling.ErrGatewayPending
			}
			remote, err = gateway.ChangeSubscription(ctx, effect.ProviderID, effect.SubscriptionAction)
			if err != nil {
				return pending, err
			}
			if !recurringSubscriptionMatches(remote, *effect.RecurringIntent) || remote.Status != target {
				return pending, bizbilling.ErrGatewayPending
			}
		}
		_, err = s.effects.Update(ctx, effect.ID, func(current *bizbilling.GatewayEffect) error {
			current.State, current.UpdatedAt = "provider_confirmed", time.Now().UTC()
			return nil
		})
		if err != nil {
			return pending, bizbilling.ErrGatewayPending
		}
	}
	result, err := s.lifecycle.operations.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandSubscriptionChange, Scope: effect.Scope, IdempotencyKey: effect.ID + ":commit", SubscriptionChange: &bizbilling.SubscriptionChange{SubscriptionID: sub.ID, Action: string(effect.SubscriptionAction), NativeEffectID: effect.ID}})
	if err != nil {
		return result, err
	}
	_, err = s.effects.Update(ctx, effect.ID, func(current *bizbilling.GatewayEffect) error {
		current.Result, current.State, current.UpdatedAt = &result, "settled", time.Now().UTC()
		return nil
	})
	if err != nil {
		return result, &bizbilling.OperationFailure{Result: result, Cause: bizbilling.ErrGatewayPending}
	}
	return result, nil
}
