package bizbillingrepo

import (
	"context"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

const StreamIndex = "frn_biz_streams"
const streamLease = 30 * time.Second

type StreamRepo struct {
	store      *Store[bizbilling.Stream]
	operations *OperationRepo
	facts      *Store[bizbilling.JournalEvent]
	balances   *Store[bizbilling.CustomerBalance]
}

func NewStreamRepo(es *elasticsearch.Client, logger *zap.Logger) *StreamRepo {
	store := NewStore[bizbilling.Stream](es, StreamIndex, logger)
	store.refresh = repository.RefreshFalse
	return &StreamRepo{store: store, operations: NewOperationRepo(es, logger), facts: NewStore[bizbilling.JournalEvent](es, JournalIndex, logger), balances: NewCustomerBalanceStore(es, logger)}
}
func (r *StreamRepo) Get(ctx context.Context, scope bizbilling.Scope) (*bizbilling.Stream, error) {
	s, err := r.store.Get(ctx, ScopeID(scope))
	if err != nil {
		return nil, err
	}
	if s.Scope != scope || s.ID != ScopeID(scope) || s.CommittedSequence < 0 {
		return nil, bizbilling.ErrReconciliationRequired
	}
	return s, nil
}
func (r *StreamRepo) Claim(ctx context.Context, scope bizbilling.Scope, operationID string) (*bizbilling.StreamClaim, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if operationID == "" {
		return nil, bizbilling.ErrInvalidMoney
	}
	_, err := r.Get(ctx, scope)
	if apperrors.IsNotFound(err) {
		_, err = r.store.CreateIfAbsent(ctx, ScopeID(scope), &bizbilling.Stream{ID: ScopeID(scope), Scope: scope})
	}
	if err != nil {
		return nil, err
	}
	owner := uuid.NewString()
	now := time.Now().UTC()
	stream, err := r.store.Update(ctx, ScopeID(scope), func(s *bizbilling.Stream) error {
		if s.Scope != scope {
			return bizbilling.ErrReconciliationRequired
		}
		// Expiry only transfers recovery of the same head. It never authorizes
		// a different operation to skip an accepted, potentially frozen fact.
		if s.ActiveOperationID != "" && s.ActiveOperationID != operationID {
			return bizbilling.ErrReconciliationRequired
		}
		if s.ActiveOperationID == operationID && s.LeaseUntil.After(now) {
			return bizbilling.ErrConflict
		}
		generation, err := bizbilling.CheckedAdd(s.Generation, 1)
		if err != nil {
			return err
		}
		if _, err := bizbilling.CheckedAdd(s.CommittedSequence, 1); err != nil {
			return err
		}
		s.Generation = generation
		s.ActiveOperationID = operationID
		s.Owner = owner
		s.LeaseUntil = now.Add(streamLease)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &bizbilling.StreamClaim{Scope: scope, Sequence: stream.CommittedSequence + 1, OperationID: operationID, Owner: owner, Generation: stream.Generation}, nil
}
func matchesClaim(s *bizbilling.Stream, c bizbilling.StreamClaim) bool {
	return s.Scope == c.Scope && s.ActiveOperationID == c.OperationID && s.Owner == c.Owner && s.Generation == c.Generation && s.CommittedSequence == c.Sequence-1
}
func (r *StreamRepo) Check(ctx context.Context, c bizbilling.StreamClaim) error {
	s, err := r.Get(ctx, c.Scope)
	if err != nil {
		return err
	}
	if !matchesClaim(s, c) || !s.LeaseUntil.After(time.Now().UTC()) {
		return bizbilling.ErrConflict
	}
	return nil
}
func (r *StreamRepo) Renew(ctx context.Context, c bizbilling.StreamClaim) error {
	now := time.Now().UTC()
	_, err := r.store.Update(ctx, ScopeID(c.Scope), func(s *bizbilling.Stream) error {
		if !matchesClaim(s, c) || !s.LeaseUntil.After(now) {
			return bizbilling.ErrConflict
		}
		s.LeaseUntil = now.Add(streamLease)
		return nil
	})
	return err
}
func (r *StreamRepo) Complete(ctx context.Context, c bizbilling.StreamClaim) error {
	op, err := r.operations.Get(ctx, c.OperationID)
	if err != nil {
		return err
	}
	if op.Event == nil || op.Event.Scope != c.Scope || op.Event.Sequence != c.Sequence {
		return bizbilling.ErrReconciliationRequired
	}
	fact, err := r.facts.Get(ctx, JournalID(c.Scope, c.Sequence))
	if apperrors.IsNotFound(err) {
		return bizbilling.ErrReconciliationRequired
	}
	if err != nil {
		return err
	}
	if digest(fact) != digest(op.Event) {
		return bizbilling.ErrReconciliationRequired
	}
	if fact.BalanceAfter == nil {
		return bizbilling.ErrReconciliationRequired
	}
	balance, err := r.balances.Get(ctx, ScopeID(c.Scope))
	if apperrors.IsNotFound(err) {
		return bizbilling.ErrReconciliationRequired
	}
	if err != nil {
		return err
	}
	if *balance != *fact.BalanceAfter {
		return bizbilling.ErrReconciliationRequired
	}
	_, err = r.store.Update(ctx, ScopeID(c.Scope), func(s *bizbilling.Stream) error {
		if !matchesClaim(s, c) || !s.LeaseUntil.After(time.Now().UTC()) {
			return bizbilling.ErrConflict
		}
		s.CommittedSequence = c.Sequence
		s.ActiveOperationID = ""
		s.Owner = ""
		s.LeaseUntil = time.Time{}
		return nil
	})
	return err
}
func (r *StreamRepo) Abort(ctx context.Context, c bizbilling.StreamClaim) error {
	op, err := r.operations.Get(ctx, c.OperationID)
	if err != nil {
		return err
	}
	if op.State != bizbilling.OperationRejected || op.Event != nil {
		return bizbilling.ErrConflict
	}
	_, err = r.store.Update(ctx, ScopeID(c.Scope), func(s *bizbilling.Stream) error {
		if !matchesClaim(s, c) || !s.LeaseUntil.After(time.Now().UTC()) {
			return bizbilling.ErrConflict
		}
		s.ActiveOperationID = ""
		s.Owner = ""
		s.LeaseUntil = time.Time{}
		return nil
	})
	return err
}
