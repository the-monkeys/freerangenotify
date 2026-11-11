package services

import (
	"context"
	"crypto/subtle"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/secrets"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

type BizGatewayConnectionService struct {
	connections  *bizbillingrepo.GatewayConnectionRepo
	gate         *BizLifecycleGate
	codec        *secrets.ConnectionCodec
	authenticate func(context.Context, bizbilling.MerchantCredentials) error
}

func NewBizGatewayConnectionService(es *elasticsearch.Client, gate *BizLifecycleGate, codec *secrets.ConnectionCodec, authenticate func(context.Context, bizbilling.MerchantCredentials) error, logger *zap.Logger) *BizGatewayConnectionService {
	return &BizGatewayConnectionService{connections: bizbillingrepo.NewGatewayConnectionRepo(es, logger), gate: gate, codec: codec, authenticate: authenticate}
}

func merchantRef(connection *bizbilling.MerchantConnection) bizbilling.GatewayRef {
	return bizbilling.GatewayRef{ConnectionID: connection.ID, AccountID: connection.AccountID, Mode: connection.Mode, CredentialRevision: connection.Revision}
}

func merchantSecretBinding(ref bizbilling.GatewayRef, app, field string) secrets.SecretBinding {
	return secrets.SecretBinding{ConnectionID: ref.ConnectionID, AppID: app, EnvironmentID: string(ref.Mode), Provider: "razorpay", Field: field + ":" + strconv.FormatInt(ref.CredentialRevision, 10) + ":" + ref.AccountID}
}

func (s *BizGatewayConnectionService) Resolve(ctx context.Context, ref bizbilling.GatewayRef) (bizbilling.MerchantCredentials, error) {
	result := bizbilling.MerchantCredentials{Reference: ref}
	connection, err := s.connections.Get(ctx, ref)
	if err != nil {
		return result, err
	}
	if s.codec == nil {
		return result, bizbilling.ErrMerchantUnavailable
	}
	result.AppID = connection.AppID
	for field, target := range map[string]*string{"key_id": &result.KeyID, "key_secret": &result.KeySecret, "webhook_secret": &result.WebhookSecret} {
		plaintext, err := s.codec.Open(connection.Secrets[field], merchantSecretBinding(ref, connection.AppID, field))
		if err != nil {
			return bizbilling.MerchantCredentials{}, bizbilling.ErrMerchantUnavailable
		}
		*target = string(plaintext)
	}
	return result, nil
}

// ResolveAPI may recover a retired API key only through a newer, separately
// verified revision of this same immutable account. Checkout signatures still
// resolve the exact historical secret; today's active slot is never consulted.
func (s *BizGatewayConnectionService) ResolveAPI(ctx context.Context, ref bizbilling.GatewayRef) (bizbilling.MerchantCredentials, error) {
	original, err := s.Resolve(ctx, ref)
	if err != nil {
		return original, err
	}
	refs, err := s.verifiedSuccessors(ctx, ref, original.AppID)
	if err != nil {
		return bizbilling.MerchantCredentials{}, err
	}
	if len(refs) > 0 {
		return s.Resolve(ctx, refs[len(refs)-1])
	}
	return original, nil
}

// Only revisions whose account linkage has already been proved may verify a
// newer checkout result for a historical order. This returns references only.
func (s *BizGatewayConnectionService) VerifiedCheckoutRevisions(ctx context.Context, ref bizbilling.GatewayRef) ([]bizbilling.GatewayRef, error) {
	original, err := s.connections.Get(ctx, ref)
	if err != nil {
		return nil, err
	}
	return s.verifiedSuccessors(ctx, ref, original.AppID)
}
func (s *BizGatewayConnectionService) verifiedSuccessors(ctx context.Context, ref bizbilling.GatewayRef, app string) ([]bizbilling.GatewayRef, error) {
	revisions, err := s.connections.Revisions(ctx, ref.ConnectionID)
	if err != nil {
		return nil, bizbilling.ErrMerchantUnavailable
	}
	refs := []bizbilling.GatewayRef{}
	for _, connection := range revisions {
		if connection.Revision <= ref.CredentialRevision {
			continue
		}
		if connection.AppID != app || !sameMerchantIdentity(merchantRef(connection), ref) {
			return nil, bizbilling.ErrReconciliationRequired
		}
		proof, err := s.connections.AccountProof(ctx, merchantRef(connection))
		if apperrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, bizbilling.ErrMerchantUnavailable
		}
		if proof.AppID != app {
			return nil, bizbilling.ErrReconciliationRequired
		}
		refs = append(refs, merchantRef(connection))
	}
	return refs, nil
}

func (s *BizGatewayConnectionService) Snapshot(ctx context.Context, app string, mode bizbilling.GatewayMode) (bizbilling.MerchantSnapshot, error) {
	snapshot := bizbilling.MerchantSnapshot{AppID: app, Mode: mode, Readiness: "not_configured"}
	if app == "" || !mode.Valid() {
		return snapshot, apperrors.BadRequest("Choose a valid payment account mode")
	}
	state, err := s.gate.State(ctx, app)
	if err != nil {
		return snapshot, err
	}
	if state.Status != "legacy" && state.Status != "active" {
		return snapshot, bizbilling.ErrActiveLegacyWriter
	}
	snapshot.Revision = state.MerchantSlotRevisions[mode]
	id := state.MerchantSlots[mode]
	if id == "" {
		if snapshot.Revision > 0 {
			snapshot.Readiness = "disconnected"
		}
		return snapshot, nil
	}
	connection, err := s.connections.Revision(ctx, id, snapshot.Revision)
	if err != nil {
		return snapshot, err
	}
	if connection.AppID != app || connection.Mode != mode {
		return snapshot, bizbilling.ErrReconciliationRequired
	}
	proof, proofErr := s.connections.AccountProof(ctx, merchantRef(connection))
	if proofErr != nil && !apperrors.IsNotFound(proofErr) {
		return snapshot, proofErr
	}
	if proofErr == nil {
		if proof.AppID != app {
			return snapshot, bizbilling.ErrReconciliationRequired
		}
		connection.AccountVerified = true
		connection.Readiness = "verified_for_checkout"
	}
	return bizbilling.MerchantSnapshot{Configured: true, ConnectionID: id, AppID: app, AccountID: connection.AccountID, Mode: mode, Revision: connection.Revision, Readiness: connection.Readiness, APIAuthenticated: connection.APIAuthenticated, AccountVerified: connection.AccountVerified, AutomaticReady: connection.AutomaticReady}, nil
}

var merchantAccountID = regexp.MustCompile(`^acc_[A-Za-z0-9_]{1,128}$`)

func validateMerchantCredentials(credentials bizbilling.MerchantCredentials) error {
	prefix := "rzp_" + string(credentials.Reference.Mode) + "_"
	if !merchantAccountID.MatchString(credentials.Reference.AccountID) || !credentials.Reference.Mode.Valid() || !strings.HasPrefix(credentials.KeyID, prefix) || len(credentials.KeyID) <= len(prefix) || len(credentials.KeyID) > 256 {
		return apperrors.BadRequest("The payment account and API key must match the selected mode")
	}
	for _, value := range []string{credentials.KeySecret, credentials.WebhookSecret} {
		if len(value) < 8 || len(value) > 512 || strings.ContainsRune(value, 0) {
			return apperrors.BadRequest("Provide valid payment API and webhook secrets")
		}
	}
	return nil
}

func (s *BizGatewayConnectionService) Save(ctx context.Context, app string, mode bizbilling.GatewayMode, expectedRevision int64, patch bizbilling.MerchantPatch) (bizbilling.MerchantSnapshot, error) {
	current, err := s.Snapshot(ctx, app, mode)
	if err != nil {
		return current, err
	}
	if expectedRevision < 0 || current.Revision != expectedRevision {
		return current, bizbilling.ErrConflict
	}
	account := patch.AccountID
	if account == "" {
		account = current.AccountID
	}
	credentials := bizbilling.MerchantCredentials{AppID: app}
	id := "bizm_" + uuid.NewString()
	if current.Configured && account == current.AccountID {
		id = current.ConnectionID
		credentials, err = s.Resolve(ctx, bizbilling.GatewayRef{ConnectionID: id, AccountID: account, Mode: mode, CredentialRevision: current.Revision})
		if err != nil {
			return current, err
		}
	} else if patch.KeyID == nil || patch.KeySecret == nil || patch.WebhookSecret == nil {
		return current, apperrors.BadRequest("A new payment account requires its own API and webhook credentials")
	}
	next, err := bizbilling.CheckedAdd(expectedRevision, 1)
	if err != nil {
		return current, err
	}
	credentials.Reference = bizbilling.GatewayRef{ConnectionID: id, AccountID: account, Mode: mode, CredentialRevision: next}
	if patch.KeyID != nil {
		credentials.KeyID = *patch.KeyID
	}
	if patch.KeySecret != nil {
		credentials.KeySecret = *patch.KeySecret
	}
	if patch.WebhookSecret != nil {
		credentials.WebhookSecret = *patch.WebhookSecret
	}
	if err = validateMerchantCredentials(credentials); err != nil {
		return current, err
	}
	if s.codec == nil || s.authenticate == nil {
		return current, bizbilling.ErrMerchantUnavailable
	}
	connection := &bizbilling.MerchantConnection{ID: id, AppID: app, AccountID: account, Mode: mode, Revision: next, Readiness: "awaiting_account_verification", APIAuthenticated: true, Secrets: map[string]channelconnection.EncryptedSecret{}, CreatedAt: time.Now().UTC()}
	for field, value := range map[string]string{"key_id": credentials.KeyID, "key_secret": credentials.KeySecret, "webhook_secret": credentials.WebhookSecret} {
		sealed, err := s.codec.Seal([]byte(value), merchantSecretBinding(credentials.Reference, app, field))
		if err != nil {
			return current, bizbilling.ErrMerchantUnavailable
		}
		connection.Secrets[field] = sealed
	}
	if err = s.authenticate(ctx, credentials); err != nil {
		if errors.Is(err, bizbilling.ErrMerchantAuthentication) {
			return current, bizbilling.ErrMerchantAuthentication
		}
		return current, bizbilling.ErrMerchantUnavailable
	}
	ctx, release, err := s.gate.beginMerchantSwitch(ctx, app, mode, expectedRevision)
	if err != nil {
		return current, err
	}
	defer release()
	stopRenewal := keepBillingLease(ctx, 20*time.Second, func(renewCtx context.Context) error { return s.gate.renewMerchantSwitch(renewCtx, app) })
	defer stopRenewal()
	if err = s.connections.Create(ctx, connection); err != nil {
		// An interrupted metadata write can be reused only when the original
		// immutable revision decrypts to exactly this authenticated intent.
		existing, readErr := s.Resolve(ctx, credentials.Reference)
		if readErr != nil || !sameMerchantCredentials(existing, credentials) {
			return current, err
		}
	}
	if err = s.gate.publishMerchantSwitch(ctx, app, id); err != nil {
		confirmed, readErr := s.Snapshot(ctx, app, mode)
		if readErr != nil || confirmed.ConnectionID != id || confirmed.Revision != next {
			return current, err
		}
		return confirmed, nil
	}
	return s.Snapshot(ctx, app, mode)
}

func sameMerchantCredentials(a, b bizbilling.MerchantCredentials) bool {
	return a.AppID == b.AppID && a.Reference == b.Reference && a.KeyID == b.KeyID && subtle.ConstantTimeCompare([]byte(a.KeySecret), []byte(b.KeySecret)) == 1 && subtle.ConstantTimeCompare([]byte(a.WebhookSecret), []byte(b.WebhookSecret)) == 1
}

func (s *BizGatewayConnectionService) Disconnect(ctx context.Context, app string, mode bizbilling.GatewayMode, expectedRevision int64) error {
	if app == "" || !mode.Valid() || expectedRevision < 0 {
		return apperrors.BadRequest("Choose a valid payment account and revision")
	}
	ctx, release, err := s.gate.beginMerchantSwitch(ctx, app, mode, expectedRevision)
	if err != nil {
		return err
	}
	defer release()
	return s.gate.publishMerchantSwitch(ctx, app, "")
}

// CallbackCredentials is internal-only. It retains verification material for
// old events without selecting a different merchant or exposing secrets to UI.
func (s *BizGatewayConnectionService) CallbackCredentials(ctx context.Context, connectionID string) ([]bizbilling.MerchantCredentials, error) {
	revisions, err := s.connections.Revisions(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	result := make([]bizbilling.MerchantCredentials, 0, len(revisions))
	for _, connection := range revisions {
		if len(result) > 0 && (connection.AppID != result[0].AppID || connection.AccountID != result[0].Reference.AccountID || connection.Mode != result[0].Reference.Mode) {
			return nil, bizbilling.ErrReconciliationRequired
		}
		credentials, err := s.Resolve(ctx, merchantRef(connection))
		if err != nil {
			return nil, err
		}
		result = append(result, credentials)
	}
	if len(result) == 0 {
		return nil, bizbilling.ErrMerchantNotReady
	}
	return result, nil
}
