package services

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

func recurringPlanMatches(plan bizbilling.GatewayRecurringPlan, intent bizbilling.RecurringIntent) bool {
	return plan.ID != "" && plan.OperationID == intent.OperationID && plan.AmountPaisa == intent.AmountPaisa && plan.Currency == intent.Currency && plan.Period == intent.Period && plan.Interval == intent.Interval
}

func recurringSubscriptionMatches(remote bizbilling.GatewaySubscription, intent bizbilling.RecurringIntent) bool {
	actual := remote.Intent
	return remote.ID != "" && remote.PlanID == intent.PlanID && actual.PlanID == intent.PlanID && actual.OperationID == intent.OperationID && actual.SubscriptionID == intent.SubscriptionID && actual.Scope == intent.Scope && actual.Gateway == intent.Gateway && actual.AmountPaisa == intent.AmountPaisa && actual.Currency == intent.Currency && actual.Period == intent.Period && actual.Interval == intent.Interval && actual.TotalCount == intent.TotalCount && actual.StartOnAuthorization == intent.StartOnAuthorization && actual.StartAt.Equal(intent.StartAt) && actual.ExpireBy.Equal(intent.ExpireBy)
}

func (s *BizRecurringService) ensureRecurringPlan(ctx context.Context, effect *bizbilling.GatewayEffect, gateway bizbilling.RecurringGateway) (*bizbilling.GatewayEffect, error) {
	if effect.StopRequested {
		return nil, bizbilling.ErrAutomaticCollectionInProgress
	}
	if effect.RecurringIntent == nil {
		return nil, bizbilling.ErrReconciliationRequired
	}
	if effect.RecurringPlan != nil {
		if !recurringPlanMatches(*effect.RecurringPlan, *effect.RecurringIntent) || effect.RecurringIntent.PlanID != effect.RecurringPlan.ID {
			return nil, bizbilling.ErrReconciliationRequired
		}
		return effect, nil
	}
	won := false
	updated, err := s.effects.Update(ctx, effect.ID, func(current *bizbilling.GatewayEffect) error {
		won = false
		if current.StopRequested {
			return bizbilling.ErrAutomaticCollectionInProgress
		}
		if current.RecurringPlan != nil {
			return nil
		}
		if !current.PlanAttempted {
			current.PlanAttempted, won = true, true
		}
		current.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return nil, bizbilling.ErrGatewayPending
	}
	if updated.RecurringPlan != nil {
		return s.ensureRecurringPlan(ctx, updated, gateway)
	}
	intent := *updated.RecurringIntent
	var plan *bizbilling.GatewayRecurringPlan
	if won {
		created, createErr := gateway.CreateRecurringPlan(ctx, intent)
		if createErr != nil {
			return nil, createErr
		}
		plan = &created
	} else {
		plan, err = gateway.FindRecurringPlan(ctx, intent)
		if err != nil {
			return nil, err
		}
		if plan == nil {
			return nil, bizbilling.ErrGatewayPending
		}
	}
	if !recurringPlanMatches(*plan, intent) {
		return nil, bizbilling.ErrInvalidGatewayProof
	}
	updated, err = s.effects.Update(ctx, effect.ID, func(current *bizbilling.GatewayEffect) error {
		if current.RecurringPlan != nil && *current.RecurringPlan != *plan {
			return bizbilling.ErrConflict
		}
		current.RecurringPlan = plan
		current.RecurringIntent.PlanID = plan.ID
		current.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return nil, bizbilling.ErrGatewayPending
	}
	return updated, nil
}

func (s *BizRecurringService) ensureSubscription(ctx context.Context, effect *bizbilling.GatewayEffect, gateway bizbilling.RecurringGateway) (*bizbilling.GatewaySubscription, error) {
	current, err := s.effects.Get(ctx, effect.ID)
	if err != nil {
		return nil, err
	}
	if current.StopRequested {
		return nil, bizbilling.ErrAutomaticCollectionInProgress
	}
	current, err = s.ensureRecurringPlan(ctx, current, gateway)
	if err != nil {
		return nil, err
	}
	intent := *current.RecurringIntent
	var remote *bizbilling.GatewaySubscription
	if current.RecurringSubscription != nil {
		fetched, fetchErr := gateway.FetchSubscription(ctx, current.RecurringSubscription.ID)
		if fetchErr != nil {
			return nil, fetchErr
		}
		remote = &fetched
	} else {
		won := false
		current, err = s.effects.Update(ctx, current.ID, func(row *bizbilling.GatewayEffect) error {
			won = false
			if row.StopRequested {
				return bizbilling.ErrAutomaticCollectionInProgress
			}
			if row.RecurringSubscription != nil {
				return nil
			}
			if !row.Attempted {
				row.Attempted, won = true, true
			}
			row.UpdatedAt = time.Now().UTC()
			return nil
		})
		if err != nil {
			return nil, bizbilling.ErrGatewayPending
		}
		if current.RecurringSubscription != nil {
			return s.ensureSubscription(ctx, current, gateway)
		}
		if won {
			created, createErr := gateway.CreateSubscription(ctx, intent)
			if createErr != nil {
				return nil, createErr
			}
			remote = &created
		} else {
			remote, err = gateway.FindSubscription(ctx, intent)
			if err != nil {
				return nil, err
			}
			if remote == nil {
				return nil, bizbilling.ErrGatewayPending
			}
		}
	}
	if !recurringSubscriptionMatches(*remote, intent) {
		return nil, bizbilling.ErrInvalidGatewayProof
	}
	_, err = s.effects.Update(ctx, current.ID, func(row *bizbilling.GatewayEffect) error {
		if row.ProviderID != "" && row.ProviderID != remote.ID {
			return bizbilling.ErrConflict
		}
		row.RecurringSubscription, row.ProviderID, row.State = remote, remote.ID, "provider_created"
		row.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return nil, bizbilling.ErrGatewayPending
	}
	return remote, nil
}
