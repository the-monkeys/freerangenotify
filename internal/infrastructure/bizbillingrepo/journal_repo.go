package bizbillingrepo

import (
	"context"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

const JournalIndex = "frn_biz_journal"

func JournalID(scope bizbilling.Scope, sequence int64) string {
	return fmt.Sprintf("%s-%020d", ScopeID(scope), sequence)
}

type JournalRepo struct {
	store      *Store[bizbilling.JournalEvent]
	operations *OperationRepo
	streams    *StreamRepo
}

func NewJournalRepo(es *elasticsearch.Client, logger *zap.Logger) *JournalRepo {
	store := NewStore[bizbilling.JournalEvent](es, JournalIndex, logger)
	store.refresh = repository.RefreshFalse
	return &JournalRepo{store: store, operations: NewOperationRepo(es, logger), streams: NewStreamRepo(es, logger)}
}
func (r *JournalRepo) Get(ctx context.Context, scope bizbilling.Scope, sequence int64) (*bizbilling.JournalEvent, error) {
	event, err := r.store.Get(ctx, JournalID(scope, sequence))
	if err != nil {
		return nil, err
	}
	if event.SchemaVersion != bizbilling.FinancialOperationSchemaVersion || event.Scope != scope || event.Sequence != sequence || event.ID != JournalID(scope, sequence) {
		return nil, bizbilling.ErrReconciliationRequired
	}
	op, err := r.operations.Get(ctx, event.OperationID)
	if err != nil {
		return nil, err
	}
	if op.Event == nil || digest(op.Event) != digest(event) {
		return nil, bizbilling.ErrReconciliationRequired
	}
	return event, nil
}
func (r *JournalRepo) Append(ctx context.Context, claim bizbilling.StreamClaim, event bizbilling.JournalEvent) error {
	if event.ID != JournalID(claim.Scope, claim.Sequence) || event.Scope != claim.Scope || event.AppID != claim.Scope.AppID || event.OperationID != claim.OperationID || event.Sequence != claim.Sequence {
		return bizbilling.ErrReconciliationRequired
	}
	if err := r.streams.Check(ctx, claim); err != nil {
		return err
	}
	op, err := r.operations.Get(ctx, claim.OperationID)
	if err != nil {
		return err
	}
	if op.Event == nil || digest(op.Event) != digest(event) {
		return bizbilling.ErrReconciliationRequired
	}
	created, err := r.store.CreateIfAbsent(ctx, event.ID, &event)
	if err != nil {
		return err
	}
	if created {
		return nil
	}
	stored, err := r.store.Get(ctx, event.ID)
	if err != nil {
		return err
	}
	if digest(stored) != digest(event) {
		return bizbilling.ErrReconciliationRequired
	}
	return nil
}
