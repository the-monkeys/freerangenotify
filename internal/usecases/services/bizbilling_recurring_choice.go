package services

import (
	"context"
	"strconv"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

// Freeze the target generation before doing work. A late retry of an earlier
// cancellation must never cancel a newly authorized mandate on this subscription.
func (s *BizRecurringService) configureCollectionChoice(ctx context.Context, sub *bizbilling.BizSubscription, request bizbilling.ChangeCollectionRequest, key string) (bizbilling.OperationResult, error) {
	if sub.FinancialScope().Validate() != nil {
		return bizbilling.OperationResult{}, bizbilling.ErrLegacyReviewRequired
	}
	cycles := ""
	if request.AutomaticCycles != nil {
		if request.CollectionMode != bizbilling.CollectionAutomatic {
			return bizbilling.OperationResult{}, apperrors.BadRequest("Choose automatic cycle count only for automatic collection")
		}
		if *request.AutomaticCycles < 1 || *request.AutomaticCycles > 120 {
			return bizbilling.OperationResult{}, bizbilling.ErrInvalidMoney
		}
		cycles = strconv.Itoa(*request.AutomaticCycles)
	}
	id := gatewayEffectID("collection_choice_key", sub.AppID, key)
	hash := gatewayEffectID("collection_choice", sub.AppID, sub.UserID, sub.ID, string(request.CollectionMode), cycles)
	choice, err := s.effects.Get(ctx, id)
	if apperrors.IsNotFound(err) {
		now := time.Now().UTC()
		row := &bizbilling.GatewayEffect{ID: id, Kind: "collection_choice_key", Scope: sub.FinancialScope(), SubscriptionID: sub.ID, SubscriptionGeneration: sub.AuthorizationGeneration, RequestHash: hash, NextEffectID: sub.AuthorizationEffectID, State: "pending", CreatedAt: now, UpdatedAt: now}
		_, _ = s.effects.CreateIfAbsent(ctx, id, row)
		choice, err = s.effects.Get(ctx, id)
		if err != nil {
			return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
		}
	} else if err != nil {
		return bizbilling.OperationResult{}, err
	}
	if choice.Kind != "collection_choice_key" || choice.Scope != sub.FinancialScope() || choice.RequestHash != hash || choice.SubscriptionID != sub.ID {
		return bizbilling.OperationResult{}, bizbilling.ErrIdempotencyConflict
	}
	if choice.Result != nil {
		if choice.Result.State == bizbilling.OperationRejected {
			return *choice.Result, bizbilling.ErrInvalidMoney
		}
		return *choice.Result, nil
	}
	var result bizbilling.OperationResult
	if request.CollectionMode == bizbilling.CollectionInvoice && choice.NextEffectID != "" {
		target, loadErr := s.effects.Get(ctx, choice.NextEffectID)
		if loadErr != nil {
			return result, loadErr
		}
		result, err = s.stopAuthorization(ctx, sub, target, key)
	} else {
		// An accepted request may have finished just before its response or alias
		// update was lost. Resume its saved input rather than rebuilding it.
		op, loadErr := s.lifecycle.operations.operations.Get(ctx, bizbillingrepo.OperationIDForKey(choice.Scope, key))
		if loadErr == nil {
			input := op.Command.SubscriptionCollection
			if op.Command.Kind != bizbilling.CommandSubscriptionCollection || input == nil || input.Action != "request" || input.Mode != request.CollectionMode || input.SubscriptionID != sub.ID || !sameCycleChoice(input.AutomaticCycles, request.AutomaticCycles) {
				return result, bizbilling.ErrIdempotencyConflict
			}
			result, err = s.lifecycle.operations.Resume(ctx, sub.AppID, op.ID)
		} else if !apperrors.IsNotFound(loadErr) {
			return result, loadErr
		} else {
			if sub.AuthorizationGeneration != choice.SubscriptionGeneration {
				return result, bizbilling.ErrConflict
			}
			result, err = s.applyCollection(ctx, sub, bizbilling.SubscriptionCollection{Action: "request", Mode: request.CollectionMode, AutomaticCycles: request.AutomaticCycles}, key)
		}
	}
	if result.OperationID != "" && (result.State == bizbilling.OperationCompleted || result.State == bizbilling.OperationRejected) {
		_, saveErr := s.effects.Update(ctx, id, func(row *bizbilling.GatewayEffect) error {
			if row.Result != nil && row.Result.OperationID != result.OperationID {
				return bizbilling.ErrConflict
			}
			row.Result = &result
			row.State = "settled"
			row.UpdatedAt = time.Now().UTC()
			return nil
		})
		if saveErr != nil {
			return result, bizbilling.ErrGatewayPending
		}
	}
	if err != nil && result.OperationID == "" {
		result = bizbilling.OperationResult{OperationID: id, State: bizbilling.OperationAccepted}
	}
	return result, err
}

func sameCycleChoice(a, b *int) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }
