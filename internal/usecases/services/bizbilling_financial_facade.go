package services

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func financialScopeFor(ctx context.Context, gate *BizLifecycleGate, appID, userID, currency string) (bizbilling.Scope, bool, error) {
	scope := bizbilling.Scope{AppID: appID, UserID: userID, Currency: currency}
	if gate == nil {
		return scope, false, nil
	}
	state, err := gate.State(ctx, appID)
	if err != nil {
		return scope, false, err
	}
	if state.LifecycleVersion == 1 {
		return scope, false, nil
	}
	if state.LifecycleVersion != 2 || state.Status != "active" || state.PrimaryCurrency == "" {
		return scope, false, bizbilling.ErrReconciliationRequired
	}
	if scope.Currency == "" {
		scope.Currency = state.PrimaryCurrency
	}
	if err = scope.Validate(); err != nil {
		return scope, false, apperrors.BadRequest("valid app, user and currency are required")
	}
	return scope, true, nil
}

func (s *BizOperationService) SetLifecycleGate(gate *BizLifecycleGate) {
	s.gate = gate
	guard := financialWriteGuard(gate, s)
	s.stores.SetFinancialWriteGuard(guard)
	s.balances.SetFinancialWriteGuard(guard)
}
func (s *BizOperationService) beginOperationMutation(ctx context.Context, appID string) (context.Context, func() error, error) {
	ctx, done, err := BeginBizMutation(ctx, s.gate, appID)
	if err != nil {
		return ctx, done, err
	}
	if s.gate != nil {
		state, err := s.gate.State(ctx, appID)
		if err != nil {
			releaseBizMutation(s.logger, done)
			return ctx, func() error { return nil }, err
		}
		if state.LifecycleVersion != 2 || state.Status != "active" {
			releaseBizMutation(s.logger, done)
			return ctx, func() error { return nil }, bizbilling.ErrReconciliationRequired
		}
	}
	return ctx, done, nil
}

// completedFinancialEvent returns this action's frozen result, avoiding a race
// with a later action when compatible facades build their synchronous response.
func (s *BizOperationService) completedFinancialEvent(ctx context.Context, appID, id string) (*bizbilling.JournalEvent, error) {
	op, err := s.ownedOperation(ctx, appID, id)
	if err != nil {
		return nil, err
	}
	if op.State != bizbilling.OperationCompleted || op.Event == nil {
		return nil, bizbilling.ErrReconciliationRequired
	}
	return op.Event, nil
}

func (s *BizOperationService) finalizeFinancialEvent(ctx context.Context, event bizbilling.JournalEvent, balance bizbilling.CustomerBalance) (bizbilling.JournalEvent, error) {
	balance.LastSequence = event.Sequence
	event.BalanceAfter = &balance
	if s.gate != nil {
		state, err := s.gate.State(ctx, event.Scope.AppID)
		if err != nil {
			return event, err
		}
		if state.LifecycleVersion == 2 && state.Status == "active" && event.Scope.Currency == state.PrimaryCurrency {
			mirror, err := s.stores.UserBalances.Get(ctx, event.Scope.UserID)
			if err != nil && !apperrors.IsNotFound(err) {
				return event, err
			}
			// Subscriber deletion cannot discard a verified money fact. Balances
			// remain in their own customer/currency stream without a profile.
			if err == nil {
				if mirror.AppID != event.Scope.AppID || mirror.UserID != event.Scope.UserID || (mirror.Currency != "" && mirror.Currency != state.PrimaryCurrency) {
					return event, bizbilling.ErrReconciliationRequired
				}
				event.UserBalanceAfter = &bizbilling.UserBalanceMirror{UserID: mirror.UserID, AppID: mirror.AppID, Currency: state.PrimaryCurrency, BalancePaisa: balance.AvailablePaisa, LastSequence: event.Sequence}
			}
		}
	}
	return event, nil
}
