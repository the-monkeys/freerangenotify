package bizbillingrepo

import (
	"context"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// ExpenseClaimRepo fences mutable source definitions before freezing a batch.
// It is private to financial coordination; ordinary stores cannot set claims.
type ExpenseClaimRepo struct {
	store   *Store[bizbilling.Expense]
	streams *StreamRepo
}

func NewExpenseClaimRepo(es *elasticsearch.Client, logger *zap.Logger) *ExpenseClaimRepo {
	return &ExpenseClaimRepo{store: NewStore[bizbilling.Expense](es, ExpenseIndex, logger, repository.RefreshFalse), streams: NewStreamRepo(es, logger)}
}

func (r *ExpenseClaimRepo) Claim(ctx context.Context, id string, claim bizbilling.StreamClaim) (*bizbilling.Expense, error) {
	return r.store.Update(ctx, id, func(e *bizbilling.Expense) error {
		if err := r.streams.Check(ctx, claim); err != nil {
			return err
		}
		if e.FinancialScope() != claim.Scope || e.InvoiceID != "" || e.LastSequence != 0 || !e.Billable || e.AmountPaisa <= 0 {
			return bizbilling.ErrInvalidMoney
		}
		if e.PendingOperationID != "" && e.PendingOperationID != claim.OperationID {
			return bizbilling.ErrConflict
		}
		if e.PendingGeneration > claim.Generation {
			return bizbilling.ErrConflict
		}
		e.PendingOperationID, e.PendingGeneration = claim.OperationID, claim.Generation
		return nil
	})
}

func (r *ExpenseClaimRepo) Release(ctx context.Context, id string, claim bizbilling.StreamClaim) error {
	_, err := r.store.Update(ctx, id, func(e *bizbilling.Expense) error {
		if err := r.streams.Check(ctx, claim); err != nil {
			return err
		}
		if e.PendingOperationID == "" || e.PendingOperationID != claim.OperationID {
			return nil
		}
		if e.InvoiceID != "" || e.LastSequence != 0 || e.PendingGeneration > claim.Generation {
			return bizbilling.ErrConflict
		}
		e.PendingOperationID, e.PendingGeneration = "", 0
		return nil
	})
	if apperrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (r *ExpenseClaimRepo) SetFinancialReadOverlay(reader FinancialReadOverlay) {
	r.store.SetFinancialReadOverlay(reader)
}
