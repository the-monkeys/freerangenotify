package bizbillingrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

const GatewayConnectionIndex = "frn_biz_gateway_connections"

type GatewayConnectionRepo struct {
	store *Store[bizbilling.MerchantConnection]
}

func NewGatewayConnectionRepo(es *elasticsearch.Client, logger *zap.Logger) *GatewayConnectionRepo {
	return &GatewayConnectionRepo{NewStore[bizbilling.MerchantConnection](es, GatewayConnectionIndex, logger, repository.RefreshFalse)}
}

func GatewayConnectionRevisionID(ref bizbilling.GatewayRef) string {
	hash := sha256.Sum256([]byte(ref.ConnectionID + "\x00" + strconv.FormatInt(ref.CredentialRevision, 10)))
	return hex.EncodeToString(hash[:])
}

func (r *GatewayConnectionRepo) Get(ctx context.Context, ref bizbilling.GatewayRef) (*bizbilling.MerchantConnection, error) {
	if ref.ConnectionID == "" || ref.AccountID == "" || !ref.Mode.Valid() || ref.CredentialRevision < 1 {
		return nil, bizbilling.ErrMerchantNotReady
	}
	connection, err := r.Revision(ctx, ref.ConnectionID, ref.CredentialRevision)
	if err != nil {
		return nil, err
	}
	if connection.ID != ref.ConnectionID || connection.AccountID != ref.AccountID || connection.Mode != ref.Mode || connection.Revision != ref.CredentialRevision || connection.AppID == "" {
		return nil, bizbilling.ErrReconciliationRequired
	}
	return connection, nil
}

func (r *GatewayConnectionRepo) Revision(ctx context.Context, id string, revision int64) (*bizbilling.MerchantConnection, error) {
	if id == "" || revision < 1 {
		return nil, bizbilling.ErrMerchantNotReady
	}
	connection, err := r.store.Get(ctx, GatewayConnectionRevisionID(bizbilling.GatewayRef{ConnectionID: id, CredentialRevision: revision}))
	if err != nil {
		return nil, err
	}
	if connection.ID != id || connection.Revision != revision || connection.AppID == "" || connection.AccountID == "" || !connection.Mode.Valid() {
		return nil, bizbilling.ErrReconciliationRequired
	}
	return connection, nil
}

// Create never overwrites a retained revision or exposes a destructive delete.
func (r *GatewayConnectionRepo) Create(ctx context.Context, connection *bizbilling.MerchantConnection) error {
	ref := bizbilling.GatewayRef{ConnectionID: connection.ID, AccountID: connection.AccountID, Mode: connection.Mode, CredentialRevision: connection.Revision}
	created, err := r.store.CreateIfAbsent(ctx, GatewayConnectionRevisionID(ref), connection)
	if err != nil {
		return err
	}
	if !created {
		return bizbilling.ErrConflict
	}
	return nil
}

func (r *GatewayConnectionRepo) Revisions(ctx context.Context, connectionID string) ([]*bizbilling.MerchantConnection, error) {
	if connectionID == "" {
		return nil, bizbilling.ErrMerchantNotReady
	}
	if err := r.store.Refresh(ctx); err != nil {
		return nil, err
	}
	result := []*bizbilling.MerchantConnection{}
	after := int64(0)
	for {
		rows, _, err := r.store.Find(ctx, map[string]interface{}{"size": 200, "sort": []interface{}{map[string]interface{}{"revision": "asc"}}, "query": map[string]interface{}{"bool": map[string]interface{}{"filter": []interface{}{map[string]interface{}{"term": map[string]interface{}{"id": connectionID}}, map[string]interface{}{"range": map[string]interface{}{"revision": map[string]interface{}{"gt": after}}}}}}})
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return result, nil
		}
		for _, row := range rows {
			if row.ID != connectionID || row.Revision <= after || len(result) >= 10000 {
				return nil, bizbilling.ErrReconciliationRequired
			}
			after = row.Revision
			result = append(result, row)
		}
	}
}
