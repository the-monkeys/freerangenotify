package services

import (
	"context"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"time"
)

func (s *BizOperationService) buildFinancialEvent(ctx context.Context, command bizbilling.Command, claim bizbilling.StreamClaim) (bizbilling.JournalEvent, error) {
	event := bizbilling.JournalEvent{SchemaVersion: bizbilling.FinancialOperationSchemaVersion, ID: bizbillingrepo.JournalID(claim.Scope, claim.Sequence), AppID: claim.Scope.AppID, OperationID: claim.OperationID, Scope: claim.Scope, Sequence: claim.Sequence, Kind: command.Kind, CreatedAt: time.Now().UTC()}
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
		balance = bizbilling.CustomerBalance{Scope: claim.Scope, AvailablePaisa: command.OpeningBalance.AvailablePaisa, NonRefundablePaisa: command.OpeningBalance.AvailablePaisa, UnresolvedLegacy: command.OpeningBalance.UnresolvedLegacy}
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
		case bizbilling.CommandBalanceAdjustment, bizbilling.CommandBalanceSet, bizbilling.CommandBalanceConsume:
			delta := int64(0)
			if command.Kind == bizbilling.CommandBalanceSet {
				if command.BalanceSet.Initialize {
					if claim.Sequence != 1 || balance.AvailablePaisa != 0 || balance.ReservedPaisa != 0 || balance.UnresolvedLegacy {
						return event, apperrors.Conflict("This customer already has a billing history")
					}
					_, err := s.profiles.Get(ctx, command.Scope.UserID)
					if err == nil {
						return event, apperrors.Conflict("This customer already exists")
					}
					if !apperrors.IsNotFound(err) {
						return event, err
					}
				}
				delta = command.BalanceSet.TargetPaisa - balance.AvailablePaisa
				event.Resources = []bizbilling.ResourceRef{{Kind: "customer", ID: command.Scope.UserID}}
			} else if command.Kind == bizbilling.CommandBalanceConsume {
				event.BalanceConsumedPaisa = min(balance.AvailablePaisa, command.BalanceConsume.MaxPaisa)
				delta = -event.BalanceConsumedPaisa
			} else {
				delta = command.BalanceAdjustment.DeltaPaisa
			}
			if balance.UnresolvedLegacy {
				return event, bizbilling.ErrLegacyReviewRequired
			}
			next, err := bizbilling.CheckedAdd(balance.AvailablePaisa, delta)
			if err != nil {
				return event, err
			}
			if next < 0 {
				return event, bizbilling.ErrInvalidMoney
			}
			if delta >= 0 {
				value, err := bizbilling.CheckedAdd(balance.NonRefundablePaisa, delta)
				if err != nil {
					return event, err
				}
				balance.NonRefundablePaisa = value
			} else {
				amount, err := bizbilling.CheckedMul(delta, -1)
				if err != nil {
					return event, err
				}
				if err = s.consumeFinancialCredit(ctx, &event, &balance, amount); err != nil {
					return event, err
				}
			}
			balance.AvailablePaisa = next
		case bizbilling.CommandOpeningBalance:
			return event, bizbilling.ErrInvalidMoney
		default:
			return s.buildResourceFinancialEvent(ctx, command, claim, event, balance)
		}
	}
	return s.finalizeFinancialEvent(ctx, event, balance)
}
