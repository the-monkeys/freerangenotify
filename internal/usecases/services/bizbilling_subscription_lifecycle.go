package services

import (
	"context"
	"github.com/google/uuid"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (s *BizLifecycleService) subscriptionV2(ctx context.Context, app string) (bool, error) {
	if s.gate == nil {
		return false, nil
	}
	state, err := s.gate.State(ctx, app)
	if err != nil {
		return false, err
	}
	if state.LifecycleVersion == 1 {
		return false, nil
	}
	if state.LifecycleVersion != 2 || state.Status != "active" || s.operations == nil || s.periods == nil {
		return true, bizbilling.ErrReconciliationRequired
	}
	return true, nil
}

func (s *BizLifecycleService) trySubscriptionCreate(ctx context.Context, app string, request *bizbilling.CreateSubscriptionRequest) (*bizbilling.BizSubscription, bool, error) {
	v2, err := s.subscriptionV2(ctx, app)
	if err != nil || !v2 {
		return nil, v2, err
	}
	if request == nil {
		return nil, true, bizbilling.ErrInvalidMoney
	}
	plan, err := s.stores.Plans.Get(ctx, request.PlanID)
	if err != nil {
		return nil, true, err
	}
	if plan.AppID != app {
		return nil, true, apperrors.NotFound("plan", request.PlanID)
	}
	scope := bizbilling.Scope{AppID: app, UserID: request.UserID, Currency: plan.Currency}
	if request.GatewayMode == bizbilling.GatewayTest {
		scope.Mode = bizbilling.GatewayTest
	}
	event, err := s.applyFinancialCommand(ctx, bizbilling.Command{Kind: bizbilling.CommandSubscriptionCreate, Scope: scope, SubscriptionCreate: &bizbilling.SubscriptionCreate{Request: *request}})
	if err != nil {
		return nil, true, err
	}
	return event.SubscriptionAfter, true, nil
}

func (s *BizLifecycleService) trySubscriptionChange(ctx context.Context, app, id string, input bizbilling.SubscriptionChange) (*bizbilling.BizSubscription, bool, error) {
	v2, err := s.subscriptionV2(ctx, app)
	if err != nil || !v2 {
		return nil, v2, err
	}
	sub, err := s.getOwnedSubscription(ctx, app, id)
	if err != nil {
		return nil, true, err
	}
	input.SubscriptionID = id
	if sub.EffectiveCollectionMode() == bizbilling.CollectionAutomatic {
		recurring := s.periods.recurring
		if recurring == nil {
			return nil, true, bizbilling.ErrAutomaticCollectionInProgress
		}
		key := bizOperationKey(ctx)
		if key == "" {
			key = uuid.NewString()
		}
		if input.Action == "pause" || input.Action == "resume" {
			updated, err := recurring.changeNative(ctx, sub, bizbilling.SubscriptionAction(input.Action), key)
			return updated, true, err
		}
		stopping := input.Action == "cancel"
		if input.Action == "update" && input.Update.PlanID == nil && input.Update.Quantity == nil && input.Update.AddonIDs == nil && input.Update.Metadata == nil {
			stopping = input.Update.CancelAtPeriodEnd != nil && *input.Update.CancelAtPeriodEnd || input.Update.NonRenewing != nil && *input.Update.NonRenewing
		}
		if !stopping {
			return nil, true, apperrors.Conflict("Stop automatic collection before changing this subscription, then request fresh customer authorization")
		}
		_, err = recurring.configureCollection(ctx, app, id, bizbilling.ChangeCollectionRequest{CollectionMode: bizbilling.CollectionInvoice}, gatewayEffectID("subscription_stop", app, key), false)
		if err != nil {
			return nil, true, err
		}
		sub, err = s.getOwnedSubscription(ctx, app, id)
		if err != nil {
			return nil, true, err
		}
	}
	event, err := s.applyFinancialCommand(ctx, bizbilling.Command{Kind: bizbilling.CommandSubscriptionChange, Scope: sub.FinancialScope(), SubscriptionChange: &input})
	if err != nil {
		return nil, true, err
	}
	return event.SubscriptionAfter, true, nil
}

// ChangePlanAt is additive. An omitted timing keeps the original immediate
// manual proration behavior; next_period freezes a future price explicitly.
func (s *BizLifecycleService) ChangePlanAt(ctx context.Context, app, id, plan string, quantity *int, timing string) (*bizbilling.BizSubscription, error) {
	request := bizbilling.UpdateSubscriptionRequest{PlanID: &plan, Quantity: quantity, ChangeTiming: timing}
	if sub, handled, err := s.trySubscriptionChange(ctx, app, id, bizbilling.SubscriptionChange{Action: "update", Update: request}); handled || err != nil {
		return sub, err
	}
	if timing != "" && timing != "immediate" {
		return nil, apperrors.Conflict("Scheduled changes require the reviewed billing lifecycle")
	}
	return s.ChangePlan(ctx, app, id, plan, quantity)
}
