package bizbillingrepo

import (
	"context"
	"encoding/json"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

// ProjectSnapshot writes a typed financial revision with its checkpoint in the
// same CAS. All business decisions were frozen in the journal before this call.
func ProjectSnapshot[T bizbilling.FinancialSnapshot](ctx context.Context, store *Store[T], snapshot *T, scope bizbilling.Scope, sequence int64) error {
	if snapshot == nil {
		return nil
	}
	value := *snapshot
	if value.FinancialID() == "" || value.FinancialScope() != scope || value.FinancialSequence() != sequence {
		return bizbilling.ErrReconciliationRequired
	}
	id := value.FinancialID()
	_, err := store.Get(ctx, id)
	if apperrors.IsNotFound(err) {
		created, createErr := store.CreateIfAbsent(ctx, id, snapshot)
		if createErr != nil {
			return createErr
		}
		if created {
			return nil
		}
	} else if err != nil {
		return err
	}
	_, err = store.Update(ctx, id, func(current *T) error {
		previous := *current
		if previous.FinancialID() != id || previous.FinancialScope() != scope || previous.FinancialSequence() > sequence {
			return bizbilling.ErrReconciliationRequired
		}
		if previous.FinancialSequence() == sequence {
			if !sameSnapshot(previous, value) {
				return bizbilling.ErrReconciliationRequired
			}
			return nil
		}
		*current = value
		return nil
	})
	return err
}
func sameSnapshot(a, b any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	return err == nil && jsonEqual(left, right)
}

func verifySnapshot[T bizbilling.FinancialSnapshot](ctx context.Context, store *Store[T], snapshot *T, scope bizbilling.Scope, sequence int64) error {
	if snapshot == nil {
		return nil
	}
	expected := *snapshot
	if expected.FinancialScope() != scope || expected.FinancialSequence() != sequence || expected.FinancialID() == "" {
		return bizbilling.ErrReconciliationRequired
	}
	actual, err := store.Get(ctx, expected.FinancialID())
	if apperrors.IsNotFound(err) {
		return bizbilling.ErrReconciliationRequired
	}
	if err != nil {
		return err
	}
	if !sameSnapshot(*actual, expected) {
		return bizbilling.ErrReconciliationRequired
	}
	return nil
}

func (s *Stores) VerifyFinancialProjections(ctx context.Context, event bizbilling.JournalEvent) error {
	if err := verifySnapshot(ctx, s.Periods, event.PeriodAfter, event.Scope, event.Sequence); err != nil {
		return err
	}
	if err := verifySnapshot(ctx, s.Subscriptions, event.SubscriptionAfter, event.Scope, event.Sequence); err != nil {
		return err
	}
	for i := range event.ExpensesAfter {
		if err := verifySnapshot(ctx, s.Expenses, &event.ExpensesAfter[i], event.Scope, event.Sequence); err != nil {
			return err
		}
	}
	if err := verifySnapshot(ctx, s.Estimates, event.EstimateAfter, event.Scope, event.Sequence); err != nil {
		return err
	}
	if err := verifySnapshot(ctx, s.CouponRedemptions, event.CouponRedemptionAfter, event.Scope, event.Sequence); err != nil {
		return err
	}
	if err := verifySnapshot(ctx, s.CreditNotes, event.CreditNoteAfter, event.Scope, event.Sequence); err != nil {
		return err
	}
	if err := verifySnapshot(ctx, s.Retainers, event.RetainerAfter, event.Scope, event.Sequence); err != nil {
		return err
	}
	if err := verifySnapshot(ctx, s.CreditApplications, event.CreditApplicationAfter, event.Scope, event.Sequence); err != nil {
		return err
	}
	if err := verifySnapshot(ctx, s.Invoices, event.InvoiceAfter, event.Scope, event.Sequence); err != nil {
		return err
	}
	if err := verifySnapshot(ctx, s.Payments, event.PaymentAfter, event.Scope, event.Sequence); err != nil {
		return err
	}
	for i := range event.CreditLotsAfter {
		if err := verifySnapshot(ctx, s.CreditLots, &event.CreditLotsAfter[i], event.Scope, event.Sequence); err != nil {
			return err
		}
	}
	if err := verifySnapshot(ctx, s.RefundReservations, event.RefundReservationAfter, event.Scope, event.Sequence); err != nil {
		return err
	}
	if event.UserBalanceAfter != nil {
		mirror, err := s.UserBalances.Get(ctx, event.UserBalanceAfter.UserID)
		if apperrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if *mirror != *event.UserBalanceAfter {
			return bizbilling.ErrReconciliationRequired
		}
	}
	return nil
}

// ProjectUserBalanceMirror changes a small typed view of an existing subscriber,
// preserving profile fields. Deleting a subscriber never deletes journal money.
func (s *Stores) ProjectUserBalanceMirror(ctx context.Context, snapshot *bizbilling.UserBalanceMirror, scope bizbilling.Scope, sequence int64) error {
	if snapshot == nil {
		return nil
	}
	if snapshot.AppID != scope.AppID || snapshot.UserID != scope.UserID || snapshot.Currency != scope.Currency || snapshot.LastSequence != sequence || snapshot.BalancePaisa < 0 {
		return bizbilling.ErrReconciliationRequired
	}
	_, err := s.UserBalances.Update(ctx, snapshot.UserID, func(current *bizbilling.UserBalanceMirror) error {
		if current.AppID != scope.AppID || current.UserID != scope.UserID || (current.Currency != "" && current.Currency != scope.Currency) || current.LastSequence > sequence {
			return bizbilling.ErrReconciliationRequired
		}
		*current = *snapshot
		return nil
	})
	if apperrors.IsNotFound(err) {
		return nil
	}
	return err
}
