package services

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

// Balance exposes only a completed stream revision. A crash after a money
// projection but before commit produces reconciliation-pending, not spendable
// partial credit. Readers can still see the last completed unchanged revision.
func (s *BizOperationService) Balance(ctx context.Context, scope bizbilling.Scope) (bizbilling.CustomerBalance, error) {
	zero := bizbilling.CustomerBalance{Scope: scope}
	if err := scope.Validate(); err != nil {
		return zero, err
	}
	stream, streamErr := s.streams.Get(ctx, scope)
	balance, err := s.balances.Get(ctx, bizbillingrepo.ScopeID(scope))
	if streamErr != nil && !apperrors.IsNotFound(streamErr) {
		return zero, streamErr
	}
	if err != nil && !apperrors.IsNotFound(err) {
		return zero, err
	}
	if apperrors.IsNotFound(streamErr) {
		if balance != nil {
			return zero, bizbilling.ErrReconciliationRequired
		}
		return zero, nil
	}
	if balance == nil {
		if stream.CommittedSequence != 0 || stream.ActiveOperationID != "" {
			return zero, bizbilling.ErrReconciliationRequired
		}
		return zero, nil
	}
	if balance.Scope != scope || balance.LastSequence != stream.CommittedSequence {
		return zero, bizbilling.ErrReconciliationRequired
	}
	return *balance, nil
}

func (s *BizOperationService) buildFinancialEvent(ctx context.Context, command bizbilling.Command, claim bizbilling.StreamClaim) (bizbilling.JournalEvent, error) {
	event := bizbilling.JournalEvent{SchemaVersion: 1, ID: bizbillingrepo.JournalID(claim.Scope, claim.Sequence), AppID: claim.Scope.AppID, OperationID: claim.OperationID, Scope: claim.Scope, Sequence: claim.Sequence, Kind: command.Kind, CreatedAt: time.Now().UTC()}
	// The initial opening has no prior balance document. Every later command
	// must read exactly the last committed revision of this stream.
	var balance bizbilling.CustomerBalance
	if claim.Sequence == 1 && command.Kind == bizbilling.CommandOpeningBalance {
		_, err := s.balances.Get(ctx, bizbillingrepo.ScopeID(claim.Scope))
		if !apperrors.IsNotFound(err) {
			if err != nil {
				return event, err
			}
			return event, bizbilling.ErrReconciliationRequired
		}
		balance = bizbilling.CustomerBalance{Scope: claim.Scope, AvailablePaisa: command.OpeningBalance.AvailablePaisa, UnresolvedLegacy: command.OpeningBalance.UnresolvedLegacy}
	} else {
		previous, readErr := s.balances.Get(ctx, bizbillingrepo.ScopeID(claim.Scope))
		if apperrors.IsNotFound(readErr) && claim.Sequence == 1 {
			balance = bizbilling.CustomerBalance{Scope: claim.Scope}
		} else {
			if readErr != nil {
				return event, readErr
			}
			balance = *previous
		}
		if balance.Scope != claim.Scope {
			return event, bizbilling.ErrReconciliationRequired
		}
		if balance.LastSequence != claim.Sequence-1 {
			return event, bizbilling.ErrReconciliationRequired
		}
		switch command.Kind {
		case bizbilling.CommandBalanceAdjustment:
			if balance.UnresolvedLegacy {
				return event, bizbilling.ErrReconciliationRequired
			}
			next, err := bizbilling.CheckedAdd(balance.AvailablePaisa, command.BalanceAdjustment.DeltaPaisa)
			if err != nil {
				return event, err
			}
			if next < 0 {
				return event, bizbilling.ErrInvalidMoney
			}
			balance.AvailablePaisa = next
		case bizbilling.CommandOpeningBalance:
			return event, bizbilling.ErrInvalidMoney
		default:
			return event, bizbilling.ErrInvalidMoney
		}
	}
	balance.LastSequence = claim.Sequence
	event.BalanceAfter = &balance
	return event, nil
}

func (s *BizOperationService) applyFinancialProjection(ctx context.Context, claim bizbilling.StreamClaim, event bizbilling.JournalEvent) error {
	if err := s.streams.Renew(ctx, claim); err != nil {
		return err
	}
	if event.BalanceAfter == nil {
		return bizbilling.ErrReconciliationRequired
	}
	after := event.BalanceAfter
	if after.Scope != claim.Scope || after.LastSequence != claim.Sequence || after.AvailablePaisa < 0 || after.ReservedPaisa < 0 {
		return bizbilling.ErrReconciliationRequired
	}
	id := bizbillingrepo.ScopeID(claim.Scope)
	_, err := s.balances.Get(ctx, id)
	if apperrors.IsNotFound(err) {
		if claim.Sequence != 1 {
			return bizbilling.ErrReconciliationRequired
		}
		created, createErr := s.balances.CreateIfAbsent(ctx, id, after)
		if createErr != nil {
			return createErr
		}
		if created {
			return nil
		}
	} else if err != nil {
		return err
	}
	_, err = s.balances.Update(ctx, id, func(balance *bizbilling.CustomerBalance) error {
		if balance.Scope != claim.Scope {
			return bizbilling.ErrReconciliationRequired
		}
		if balance.LastSequence == claim.Sequence {
			if *balance != *after {
				return bizbilling.ErrReconciliationRequired
			}
			return nil
		}
		if balance.LastSequence != claim.Sequence-1 {
			return bizbilling.ErrReconciliationRequired
		}
		*balance = *after
		return nil
	})
	return err
}
