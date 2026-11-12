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
const GatewayEffectIndex = "frn_biz_gateway_effects"

func (r *GatewayConnectionRepo) RecurringCapability(ctx context.Context, ref bizbilling.GatewayRef) (*bizbilling.RecurringCapability, error) {
	store := NewStore[bizbilling.RecurringCapability](r.store.es, GatewayConnectionIndex, r.store.logger, repository.RefreshFalse)
	capability, err := store.Get(ctx, GatewayConnectionRevisionID(ref)+"_recurring_capability")
	if err != nil {
		return nil, err
	}
	if capability.Gateway != ref || capability.AppID == "" || capability.VerifiedAt.IsZero() {
		return nil, bizbilling.ErrReconciliationRequired
	}
	return capability, nil
}

func (r *GatewayConnectionRepo) RecordRecurringCapability(ctx context.Context, capability bizbilling.RecurringCapability) error {
	connection, err := r.Get(ctx, capability.Gateway)
	if err != nil {
		return err
	}
	if connection.AppID != capability.AppID || capability.VerifiedAt.IsZero() {
		return bizbilling.ErrReconciliationRequired
	}
	store := NewStore[bizbilling.RecurringCapability](r.store.es, GatewayConnectionIndex, r.store.logger, repository.RefreshFalse)
	_, _ = store.CreateIfAbsent(ctx, GatewayConnectionRevisionID(capability.Gateway)+"_recurring_capability", &capability)
	confirmed, err := r.RecurringCapability(ctx, capability.Gateway)
	if err != nil {
		return err
	}
	if confirmed.AppID != capability.AppID {
		return bizbilling.ErrReconciliationRequired
	}
	return nil
}

type GatewayConnectionRepo struct {
	store  *Store[bizbilling.MerchantConnection]
	proofs *Store[bizbilling.MerchantAccountProof]
}

func NewGatewayConnectionRepo(es *elasticsearch.Client, logger *zap.Logger) *GatewayConnectionRepo {
	return &GatewayConnectionRepo{store: NewStore[bizbilling.MerchantConnection](es, GatewayConnectionIndex, logger, repository.RefreshFalse), proofs: NewStore[bizbilling.MerchantAccountProof](es, GatewayConnectionIndex, logger, repository.RefreshFalse)}
}

func (r *GatewayConnectionRepo) AccountProof(ctx context.Context, ref bizbilling.GatewayRef) (*bizbilling.MerchantAccountProof, error) {
	proof, err := r.proofs.Get(ctx, GatewayConnectionRevisionID(ref)+"_account_proof")
	if err != nil {
		return nil, err
	}
	if proof.Gateway != ref || proof.AppID == "" || proof.EventDigest == "" || (proof.PaymentID == "" && proof.SubscriptionID == "") {
		return nil, bizbilling.ErrReconciliationRequired
	}
	return proof, nil
}
func (r *GatewayConnectionRepo) RecordAccountProof(ctx context.Context, proof *bizbilling.MerchantAccountProof) error {
	connection, err := r.Get(ctx, proof.Gateway)
	if err != nil {
		return err
	}
	if connection.AppID != proof.AppID || proof.EventDigest == "" || (proof.PaymentID == "" && proof.SubscriptionID == "") {
		return bizbilling.ErrReconciliationRequired
	}
	_, _ = r.proofs.CreateIfAbsent(ctx, GatewayConnectionRevisionID(proof.Gateway)+"_account_proof", proof)
	confirmed, err := r.AccountProof(ctx, proof.Gateway)
	if err != nil {
		return err
	}
	if confirmed.AppID != proof.AppID {
		return bizbilling.ErrReconciliationRequired
	}
	return nil
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
