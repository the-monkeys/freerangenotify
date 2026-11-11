package services

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
)

type financialReportRevision struct {
	appID           string
	epoch, revision int64
}

func beginFinancialReport(ctx context.Context, gate *BizLifecycleGate, appID string) (financialReportRevision, error) {
	zero := financialReportRevision{}
	if gate == nil {
		return zero, bizbilling.ErrReconciliationRequired
	}
	state, err := gate.State(ctx, appID)
	if err != nil {
		return zero, err
	}
	if state.LifecycleVersion != 2 || state.Status != "active" {
		return zero, bizbilling.ErrReconciliationRequired
	}
	if len(state.MutationPermits) != 0 {
		return zero, bizbilling.ErrConflict
	}
	return financialReportRevision{appID: appID, epoch: state.Epoch, revision: state.WriteRevision}, nil
}
func (r financialReportRevision) verify(ctx context.Context, gate *BizLifecycleGate) error {
	state, err := gate.State(ctx, r.appID)
	if err != nil {
		return err
	}
	if state.LifecycleVersion != 2 || state.Status != "active" || state.Epoch != r.epoch || state.WriteRevision != r.revision || len(state.MutationPermits) != 0 {
		return bizbilling.ErrConflict
	}
	return nil
}

// Stable keyword paging avoids both the legacy 100-row truncation and the
// Elasticsearch offset window. A surrounding report revision detects writers
// that complete between its queries, even when no writer remains at the end.
func walkFinancialRows[T bizbilling.FinancialSnapshot](ctx context.Context, store *bizbillingrepo.Store[T], gate *BizLifecycleGate, operations *BizOperationService, appID, userID string, visit func(*T) error) error {
	after := ""
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		query := bizbillingrepo.NewQuery(appID).Term("user_id", userID)

		body := query.Body("id", 200, 0)
		// The cursor is exclusive: gte would repeat the last document forever.
		if after != "" {
			body["query"] = map[string]interface{}{"bool": map[string]interface{}{"filter": []interface{}{query.Bool(), map[string]interface{}{"range": map[string]interface{}{"id": map[string]interface{}{"gt": after}}}}}}
		}
		body["sort"] = []interface{}{map[string]interface{}{"id": "asc"}}
		rows, _, err := store.Find(ctx, body)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		if err = guardFinancialRows(ctx, gate, operations, rows); err != nil {
			return err
		}
		for _, row := range rows {
			value := *row
			if value.FinancialID() == "" || value.FinancialID() <= after {
				return bizbilling.ErrReconciliationRequired
			}
			after = value.FinancialID()
			if err = visit(row); err != nil {
				return err
			}
		}
	}
}

func reportCurrencyMatches(actual, selected string) (bool, error) {
	if actual == "" {
		return false, bizbilling.ErrLegacyReviewRequired
	}
	return actual == selected, nil
}

func addReportAmount(target *int64, amount int64) error {
	if amount < 0 {
		return bizbilling.ErrReconciliationRequired
	}
	value, err := bizbilling.CheckedAdd(*target, amount)
	if err != nil {
		return err
	}
	*target = value
	return nil
}
