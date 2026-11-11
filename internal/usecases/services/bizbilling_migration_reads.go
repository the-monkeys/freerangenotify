package services

import (
	"context"
	"encoding/json"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

// A reviewed currency is an additive sidecar until a later authorized monetary
// action actually changes that resource. Merely upgrading never edits it.
func financialReadOverlay(gate *BizLifecycleGate, operations *BizOperationService) bizbillingrepo.FinancialReadOverlay {
	if gate == nil || operations == nil {
		return nil
	}
	return func(ctx context.Context, index, id string, source json.RawMessage) (json.RawMessage, error) {
		var header struct {
			AppID    string `json:"app_id"`
			Currency string `json:"currency"`
		}
		if err := json.Unmarshal(source, &header); err != nil {
			return nil, err
		}
		if header.Currency != "" {
			return source, nil
		}
		state, err := gate.State(ctx, header.AppID)
		if err != nil {
			return nil, err
		}
		if state.LifecycleVersion == 1 {
			return source, nil
		}
		if state.LifecycleVersion != 2 || state.Status != "active" {
			return nil, bizbilling.ErrReconciliationRequired
		}
		content := bizbillingrepo.SourceContentHash(source)
		key := bizbillingrepo.LegacyDecisionID(header.AppID, index, id, content)
		decision, err := operations.legacyReviews.Decisions.Get(ctx, key)
		if apperrors.IsNotFound(err) {
			return source, nil
		}
		if err != nil {
			return nil, err
		}
		if decision.AppID != header.AppID || decision.Index != index || decision.ResourceID != id || decision.SourceContentHash != content {
			return nil, bizbilling.ErrReconciliationRequired
		}
		currency := decision.Resolution.Currency
		if err := (bizbilling.Scope{AppID: header.AppID, UserID: "review", Currency: currency}).Validate(); err != nil {
			return nil, bizbilling.ErrReconciliationRequired
		}
		var fields map[string]json.RawMessage
		if err = json.Unmarshal(source, &fields); err != nil {
			return nil, err
		}
		fields["currency"], _ = json.Marshal(currency)
		return json.Marshal(fields)
	}
}
