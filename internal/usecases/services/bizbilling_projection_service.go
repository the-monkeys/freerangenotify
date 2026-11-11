package services

import (
	"context"

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

func (s *BizOperationService) applyFinancialProjection(ctx context.Context, claim bizbilling.StreamClaim, event bizbilling.JournalEvent) error {
	ctx = bizbillingrepo.WithFinancialProjection(ctx, claim)
	if err := s.streams.Renew(ctx, claim); err != nil {
		return err
	}
	for i := range event.ExpensesAfter {
		if err := s.streams.Renew(ctx, claim); err != nil {
			return err
		}
		if err := bizbillingrepo.ProjectSnapshot(ctx, s.stores.Expenses, &event.ExpensesAfter[i], claim.Scope, claim.Sequence); err != nil {
			return err
		}
	}
	if event.BalanceAfter == nil {
		return bizbilling.ErrReconciliationRequired
	}
	if err := bizbillingrepo.ProjectSnapshot(ctx, s.stores.Estimates, event.EstimateAfter, claim.Scope, claim.Sequence); err != nil {
		return err
	}
	if err := bizbillingrepo.ProjectSnapshot(ctx, s.stores.CouponRedemptions, event.CouponRedemptionAfter, claim.Scope, claim.Sequence); err != nil {
		return err
	}
	if err := bizbillingrepo.ProjectSnapshot(ctx, s.stores.CreditNotes, event.CreditNoteAfter, claim.Scope, claim.Sequence); err != nil {
		return err
	}
	if err := bizbillingrepo.ProjectSnapshot(ctx, s.stores.Retainers, event.RetainerAfter, claim.Scope, claim.Sequence); err != nil {
		return err
	}
	if err := bizbillingrepo.ProjectSnapshot(ctx, s.stores.CreditApplications, event.CreditApplicationAfter, claim.Scope, claim.Sequence); err != nil {
		return err
	}
	if err := bizbillingrepo.ProjectSnapshot(ctx, s.stores.Invoices, event.InvoiceAfter, claim.Scope, claim.Sequence); err != nil {
		return err
	}
	if err := bizbillingrepo.ProjectSnapshot(ctx, s.stores.Payments, event.PaymentAfter, claim.Scope, claim.Sequence); err != nil {
		return err
	}
	for i := range event.CreditLotsAfter {
		if err := s.streams.Renew(ctx, claim); err != nil {
			return err
		}
		if err := bizbillingrepo.ProjectSnapshot(ctx, s.stores.CreditLots, &event.CreditLotsAfter[i], claim.Scope, claim.Sequence); err != nil {
			return err
		}
	}
	if err := bizbillingrepo.ProjectSnapshot(ctx, s.stores.RefundReservations, event.RefundReservationAfter, claim.Scope, claim.Sequence); err != nil {
		return err
	}
	after := event.BalanceAfter
	if after.Scope != claim.Scope || after.LastSequence != claim.Sequence || after.AvailablePaisa < 0 || after.ReservedPaisa < 0 {
		return bizbilling.ErrReconciliationRequired
	}
	if err := s.stores.ProjectUserBalanceMirror(ctx, event.UserBalanceAfter, claim.Scope, claim.Sequence); err != nil {
		return err
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
