package bizbillingrepo

import (
	"context"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

const GatewayEventIndex = "frn_biz_gateway_events"

type GatewayEventRepo struct {
	receipts   *Store[bizbilling.GatewayEventReceipt]
	aliases    *Store[bizbilling.GatewayEventAlias]
	collisions *Store[bizbilling.GatewayHeaderCollision]
}

func NewGatewayEventRepo(es *elasticsearch.Client, logger *zap.Logger) *GatewayEventRepo {
	return &GatewayEventRepo{receipts: NewStore[bizbilling.GatewayEventReceipt](es, GatewayEventIndex, logger, repository.RefreshFalse), aliases: NewStore[bizbilling.GatewayEventAlias](es, GatewayEventIndex, logger, repository.RefreshFalse), collisions: NewStore[bizbilling.GatewayHeaderCollision](es, GatewayEventIndex, logger, repository.RefreshFalse)}
}
func (r *GatewayEventRepo) Get(ctx context.Context, id string) (*bizbilling.GatewayEventReceipt, error) {
	receipt, err := r.receipts.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if receipt.ID != id || receipt.Kind != "receipt" || (receipt.SchemaVersion != 1 && receipt.SchemaVersion != 2) || receipt.AppID == "" || receipt.Gateway.ConnectionID == "" {
		return nil, bizbilling.ErrReconciliationRequired
	}
	return receipt, nil
}
func (r *GatewayEventRepo) Accept(ctx context.Context, receipt *bizbilling.GatewayEventReceipt, aliasID, collisionID string) (*bizbilling.GatewayEventReceipt, error) {
	_, writeErr := r.receipts.CreateIfAbsent(ctx, receipt.ID, receipt)
	existing, err := r.Get(ctx, receipt.ID)
	if err != nil {
		return nil, bizbilling.ErrGatewayPending
	}
	if existing.AppID != receipt.AppID || existing.BodyDigest != receipt.BodyDigest || existing.Data != receipt.Data || existing.Gateway.ConnectionID != receipt.Gateway.ConnectionID || existing.Gateway.AccountID != receipt.Gateway.AccountID || existing.Gateway.Mode != receipt.Gateway.Mode {
		return nil, bizbilling.ErrReconciliationRequired
	}
	if writeErr != nil && existing.ID != receipt.ID {
		return nil, bizbilling.ErrGatewayPending
	}
	if receipt.Gateway.CredentialRevision > existing.Gateway.CredentialRevision {
		existing, err = r.receipts.Update(ctx, receipt.ID, func(current *bizbilling.GatewayEventReceipt) error {
			if current.Gateway.CredentialRevision < receipt.Gateway.CredentialRevision {
				current.Gateway = receipt.Gateway
				current.State = "pending"
				current.UpdatedAt = time.Now().UTC()
			}
			return nil
		})
		if err != nil {
			return nil, bizbilling.ErrGatewayPending
		}
	}
	alias := &bizbilling.GatewayEventAlias{ID: aliasID, Kind: "alias", AppID: receipt.AppID, ReceiptID: receipt.ID, BodyDigest: receipt.BodyDigest}
	_, _ = r.aliases.CreateIfAbsent(ctx, aliasID, alias)
	bound, err := r.aliases.Get(ctx, aliasID)
	if err != nil {
		return nil, bizbilling.ErrGatewayPending
	}
	if bound.ID != aliasID || bound.Kind != "alias" || bound.AppID != receipt.AppID {
		return nil, bizbilling.ErrReconciliationRequired
	}
	if bound.BodyDigest != receipt.BodyDigest {
		collision := &bizbilling.GatewayHeaderCollision{ID: collisionID, Kind: "collision", AppID: receipt.AppID, FirstReceiptID: bound.ReceiptID, SecondReceiptID: receipt.ID, CreatedAt: time.Now().UTC()}
		_, _ = r.collisions.CreateIfAbsent(ctx, collisionID, collision)
		confirmed, readErr := r.collisions.Get(ctx, collisionID)
		if readErr != nil || confirmed.FirstReceiptID != bound.ReceiptID || confirmed.SecondReceiptID != receipt.ID {
			return nil, bizbilling.ErrGatewayPending
		}
		existing, err = r.receipts.Update(ctx, receipt.ID, func(current *bizbilling.GatewayEventReceipt) error {
			current.HeaderConflict = true
			if current.State == "pending" {
				current.State = "quarantined"
			}
			current.UpdatedAt = time.Now().UTC()
			return nil
		})
		if err != nil {
			return nil, bizbilling.ErrGatewayPending
		}
	}
	return existing, nil
}
func (r *GatewayEventRepo) Update(ctx context.Context, id string, fn func(*bizbilling.GatewayEventReceipt) error) (*bizbilling.GatewayEventReceipt, error) {
	return r.receipts.Update(ctx, id, fn)
}
func (r *GatewayEventRepo) Pending(ctx context.Context, after string, limit int) ([]*bizbilling.GatewayEventReceipt, error) {
	if err := r.receipts.Refresh(ctx); err != nil {
		return nil, err
	}
	filters := []any{map[string]any{"term": map[string]any{"kind": "receipt"}}, map[string]any{"terms": map[string]any{"state": []string{"pending", "pending_capture", "unmatched", "quarantined", "retry"}}}}
	if after != "" {
		filters = append(filters, map[string]any{"range": map[string]any{"id": map[string]any{"gt": after}}})
	}
	rows, _, err := r.receipts.Find(ctx, map[string]any{"size": limit, "sort": []any{map[string]any{"id": "asc"}}, "query": map[string]any{"bool": map[string]any{"filter": filters}}})
	return rows, err
}

func (r *GatewayEventRepo) ListForApp(ctx context.Context, app, state, cursor string, limit int) ([]*bizbilling.GatewayEventReceipt, error) {
	if err := r.receipts.Refresh(ctx); err != nil {
		return nil, err
	}
	filters := []any{map[string]any{"term": map[string]any{"kind": "receipt"}}, map[string]any{"term": map[string]any{"app_id": app}}}
	if state != "" {
		filters = append(filters, map[string]any{"term": map[string]any{"state": state}})
	}
	if cursor != "" {
		filters = append(filters, map[string]any{"range": map[string]any{"id": map[string]any{"gt": cursor}}})
	}
	rows, _, err := r.receipts.Find(ctx, map[string]any{"size": limit, "sort": []any{map[string]any{"id": "asc"}}, "query": map[string]any{"bool": map[string]any{"filter": filters}}})
	return rows, err
}
