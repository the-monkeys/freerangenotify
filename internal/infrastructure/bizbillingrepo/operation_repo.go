package bizbillingrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

const OperationIndex = "frn_biz_operations"
const CustomerBalanceIndex = "frn_biz_customer_balances"

func NewCustomerBalanceStore(es *elasticsearch.Client, logger *zap.Logger) *Store[bizbilling.CustomerBalance] {
	store := NewStore[bizbilling.CustomerBalance](es, CustomerBalanceIndex, logger)
	store.refresh = repository.RefreshFalse
	return store
}

func digest(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func ScopeID(scope bizbilling.Scope) string { return digest(scope) }
func intentHash(command bizbilling.Command) string {
	command.IdempotencyKey = ""
	return digest(command)
}

type OperationRepo struct{ store *Store[bizbilling.Operation] }

func NewOperationRepo(es *elasticsearch.Client, logger *zap.Logger) *OperationRepo {
	store := NewStore[bizbilling.Operation](es, OperationIndex, logger)
	store.refresh = repository.RefreshFalse
	return &OperationRepo{store: store}
}
func (r *OperationRepo) Begin(ctx context.Context, command bizbilling.Command) (*bizbilling.Operation, bool, error) {
	if err := command.Validate(); err != nil {
		return nil, false, err
	}
	id := uuid.NewString()
	if command.IdempotencyKey != "" {
		id = digest(struct {
			Scope bizbilling.Scope
			Key   string
		}{command.Scope, command.IdempotencyKey})
	}
	now := time.Now().UTC()
	op := &bizbilling.Operation{SchemaVersion: bizbilling.FinancialOperationSchemaVersion, ID: id, AppID: command.Scope.AppID, Scope: command.Scope, IntentHash: intentHash(command), Command: command, State: bizbilling.OperationAccepted, CreatedAt: now, UpdatedAt: now}
	created, err := r.store.CreateIfAbsent(ctx, id, op)
	if err != nil {
		return op, false, err
	}
	if created {
		return op, true, nil
	}
	stored, err := r.Get(ctx, id)
	if err != nil {
		return nil, false, err
	}
	if stored.Scope != command.Scope || stored.IntentHash != op.IntentHash {
		return stored, false, bizbilling.ErrIdempotencyConflict
	}
	return stored, false, nil
}
func (r *OperationRepo) Get(ctx context.Context, id string) (*bizbilling.Operation, error) {
	op, err := r.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !bizbilling.SupportsFinancialSchema(op.SchemaVersion) || (op.Event != nil && op.Event.SchemaVersion != op.SchemaVersion) || op.ID != id || op.AppID != op.Scope.AppID || op.Command.Scope != op.Scope || op.Command.Validate() != nil || intentHash(op.Command) != op.IntentHash {
		return nil, bizbilling.ErrReconciliationRequired
	}
	switch op.State {
	case bizbilling.OperationAccepted:
		if op.Event != nil || op.HeadReleased {
			return nil, bizbilling.ErrReconciliationRequired
		}
	case bizbilling.OperationPrepared:
		if op.Event == nil || op.HeadReleased {
			return nil, bizbilling.ErrReconciliationRequired
		}
	case bizbilling.OperationCompleted:
		if op.Event == nil || !op.HeadReleased {
			return nil, bizbilling.ErrReconciliationRequired
		}
	case bizbilling.OperationRejected:
		if op.Event != nil {
			return nil, bizbilling.ErrReconciliationRequired
		}
	default:
		return nil, bizbilling.ErrReconciliationRequired
	}
	return op, nil
}
func (r *OperationRepo) Freeze(ctx context.Context, id string, event bizbilling.JournalEvent) (*bizbilling.Operation, error) {
	return r.store.Update(ctx, id, func(op *bizbilling.Operation) error {
		if !bizbilling.SupportsFinancialSchema(op.SchemaVersion) || event.SchemaVersion != bizbilling.FinancialOperationSchemaVersion || op.Scope != event.Scope || op.ID != event.OperationID || event.Sequence <= 0 || op.Command.Kind != event.Kind {
			return bizbilling.ErrReconciliationRequired
		}
		if op.Event != nil {
			return nil
		}
		if op.State != bizbilling.OperationAccepted {
			return bizbilling.ErrConflict
		}
		// Only an accepted intent without a frozen fact is promoted. Existing
		// prepared/completed facts keep their original schema and exact content.
		op.SchemaVersion = event.SchemaVersion
		op.Event = &event
		op.Resources = event.Resources
		op.State = bizbilling.OperationPrepared
		op.UpdatedAt = time.Now().UTC()
		return nil
	})
}
func (r *OperationRepo) Finish(ctx context.Context, id string) error {
	_, err := r.store.Update(ctx, id, func(op *bizbilling.Operation) error {
		if op.Event == nil || (op.State != bizbilling.OperationPrepared && op.State != bizbilling.OperationCompleted) {
			return bizbilling.ErrReconciliationRequired
		}
		op.State = bizbilling.OperationCompleted
		op.HeadReleased = true
		op.UpdatedAt = time.Now().UTC()
		return nil
	})
	return err
}
func (r *OperationRepo) Reject(ctx context.Context, id, code string) error {
	_, err := r.store.Update(ctx, id, func(op *bizbilling.Operation) error {
		if op.Event != nil || (op.State != bizbilling.OperationAccepted && op.State != bizbilling.OperationRejected) {
			return bizbilling.ErrConflict
		}
		op.State = bizbilling.OperationRejected
		op.ErrorCode = code
		op.UpdatedAt = time.Now().UTC()
		return nil
	})
	return err
}
func (r *OperationRepo) FinishRejected(ctx context.Context, id string) error {
	_, err := r.store.Update(ctx, id, func(op *bizbilling.Operation) error {
		if op.State != bizbilling.OperationRejected || op.Event != nil {
			return bizbilling.ErrConflict
		}
		op.HeadReleased = true
		op.UpdatedAt = time.Now().UTC()
		return nil
	})
	return err
}

// Pending uses a stable keyword cursor, never offset pagination over a shrinking
// result set. A new earlier ID is picked up on the next reconciliation pass.
func (r *OperationRepo) Pending(ctx context.Context, after string, size int) ([]*bizbilling.Operation, error) {
	if size <= 0 || size > 100 {
		size = 100
	}
	query := map[string]interface{}{"bool": map[string]interface{}{"filter": []interface{}{map[string]interface{}{"term": map[string]interface{}{"head_released": false}}}}}
	if after != "" {
		query["bool"].(map[string]interface{})["filter"] = append(query["bool"].(map[string]interface{})["filter"].([]interface{}), map[string]interface{}{"range": map[string]interface{}{"id": map[string]interface{}{"gt": after}}})
	}
	ops, _, err := r.store.Find(ctx, map[string]interface{}{"query": query, "size": size, "sort": []interface{}{map[string]interface{}{"id": "asc"}}})
	if err != nil {
		return nil, fmt.Errorf("billing reconciliation scan: %w", err)
	}
	return ops, nil
}

// PendingForApp is used only behind the app's migration fence. An unfinished
// rejected head still counts until recovery has released it.
func (r *OperationRepo) PendingForApp(ctx context.Context, appID string) ([]*bizbilling.Operation, error) {
	if err := r.store.Refresh(ctx); err != nil {
		return nil, err
	}
	result := []*bizbilling.Operation{}
	after := ""
	for {
		q := NewQuery(appID)
		body := q.Body("id", 200, 0)
		body["sort"] = []interface{}{map[string]interface{}{"id": "asc"}}
		filters := []interface{}{q.Bool(), map[string]interface{}{"bool": map[string]interface{}{"must_not": []interface{}{map[string]interface{}{"term": map[string]interface{}{"state": "completed"}}}}}}
		if after != "" {
			filters = append(filters, map[string]interface{}{"range": map[string]interface{}{"id": map[string]interface{}{"gt": after}}})
		}
		body["query"] = map[string]interface{}{"bool": map[string]interface{}{"filter": filters}}
		rows, _, err := r.store.Find(ctx, body)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return result, nil
		}
		for _, op := range rows {
			if op.AppID != appID || op.ID == "" || op.ID <= after {
				return nil, bizbilling.ErrReconciliationRequired
			}
			after = op.ID
			if op.State != bizbilling.OperationRejected || !op.HeadReleased {
				result = append(result, op)
			}
			if len(result) > 10000 {
				return nil, bizbilling.ErrReconciliationRequired
			}
		}
	}
}
