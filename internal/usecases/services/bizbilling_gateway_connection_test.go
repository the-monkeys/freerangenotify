package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/secrets"
	"go.uber.org/zap"
)

func merchantFixture(t *testing.T) (*BizGatewayConnectionService, *BizLifecycleGate, *elasticsearch.Client, *secrets.ConnectionCodec, string) {
	return merchantFaultFixture(t, nil)
}

func merchantFaultFixture(t *testing.T, fault *lostBillingResponse) (*BizGatewayConnectionService, *BizLifecycleGate, *elasticsearch.Client, *secrets.ConnectionCodec, string) {
	t.Helper()
	_, es := operationFixture(t, fault)
	templates := &database.IndexTemplates{}
	ensureBillingIndex(t, es, bizbillingrepo.GatewayConnectionIndex, templates.GetBizGatewayConnectionsTemplate())
	ensureBillingIndex(t, es, bizbillingrepo.LifecycleStateIndex, templates.GetBizLifecycleStatesTemplate())
	codec, err := secrets.NewConnectionCodec("fixture:" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{37}, 32)))
	require.NoError(t, err)
	gate := NewBizLifecycleGate(bizbillingrepo.NewLifecycleStateRepo(es))
	service := NewBizGatewayConnectionService(es, gate, codec, func(_ context.Context, credentials bizbilling.MerchantCredentials) error {
		if credentials.KeySecret == "rejected-fixture-secret" {
			return errors.New("provider must not leak " + credentials.KeySecret)
		}
		return nil
	}, zap.NewNop())
	return service, gate, es, codec, uuid.NewString()
}

func TestBizMerchantInterruptedSaveRecovery(t *testing.T) {
	for _, boundary := range []string{"credentials", "slot"} {
		t.Run(boundary, func(t *testing.T) {
			fault := &lostBillingResponse{match: func(r *http.Request, body []byte) bool {
				if boundary == "credentials" {
					return strings.Contains(r.URL.Path, bizbillingrepo.GatewayConnectionIndex+"/_doc/") && r.URL.Query().Get("op_type") == "create"
				}
				return strings.Contains(r.URL.Path, bizbillingrepo.LifecycleStateIndex+"/_doc/") && bytes.Contains(body, []byte(`"merchant_slot_revisions":{"test":1}`))
			}}
			service, _, _, _, app := merchantFaultFixture(t, fault)
			fault.armed.Store(true)
			saved, err := service.Save(context.Background(), app, bizbilling.GatewayTest, 0, merchantPatch("acc_fixture_a", "rzp_test_biz1", "owned-api-secret", "owned-hook-secret"))
			require.NoError(t, err, "a persisted write with a lost acknowledgment must be read back")
			require.False(t, fault.armed.Load(), "fault must hit the actual durable boundary")
			require.Equal(t, int64(1), saved.Revision)
			credentials, err := service.Resolve(context.Background(), merchantReference(saved))
			require.NoError(t, err)
			require.Equal(t, "owned-api-secret", credentials.KeySecret)
		})
	}
}

func TestBizMerchantExpiredMetadataFenceRecovery(t *testing.T) {
	service, gate, _, _, app := merchantFixture(t)
	ctx := context.Background()
	stale, release, err := gate.beginMerchantSwitch(ctx, app, bizbilling.GatewayTest, 0)
	require.NoError(t, err)
	defer release()
	require.NoError(t, gate.store.Mutate(ctx, app, func(state *bizbilling.LifecycleState) error {
		state.MerchantSwitch.LeaseUntil = time.Now().Add(-time.Minute)
		return nil
	}))
	saved, err := service.Save(ctx, app, bizbilling.GatewayTest, 0, merchantPatch("acc_fixture_a", "rzp_test_biz1", "owned-api-secret", "owned-hook-secret"))
	require.NoError(t, err, "an abandoned metadata-only fence must not permanently block billing")
	require.True(t, saved.Configured)
	require.ErrorIs(t, gate.publishMerchantSwitch(stale, app, "stale-connection"), bizbilling.ErrStaleMutationPermit)
	require.ErrorIs(t, gate.renewMerchantSwitch(stale, app), bizbilling.ErrStaleMutationPermit)
	_, err = gate.AcquireMutation(ctx, app)
	require.NoError(t, err, "normal billing admission must recover too")
}

func merchantPatch(account, key, secret, webhook string) bizbilling.MerchantPatch {
	return bizbilling.MerchantPatch{AccountID: account, KeyID: &key, KeySecret: &secret, WebhookSecret: &webhook}
}

func merchantReference(snapshot bizbilling.MerchantSnapshot) bizbilling.GatewayRef {
	return bizbilling.GatewayRef{ConnectionID: snapshot.ConnectionID, AccountID: snapshot.AccountID, Mode: snapshot.Mode, CredentialRevision: snapshot.Revision}
}

func TestBizMerchantEncryptionAndRBAC(t *testing.T) {
	service, _, es, codec, app := merchantFixture(t)
	ctx := context.Background()
	patch := merchantPatch("acc_fixture_a", "rzp_test_biz1", "owned-fixture-api-secret", "owned-fixture-hook-secret")
	first, err := service.Save(ctx, app, bizbilling.GatewayTest, 0, patch)
	require.NoError(t, err)
	require.True(t, first.Configured)
	require.True(t, first.APIAuthenticated)
	require.False(t, first.AccountVerified, "API authentication alone cannot attest the account ID")
	require.False(t, first.AutomaticReady)
	require.Equal(t, int64(1), first.Revision)
	ref := merchantReference(first)
	credentials, err := service.Resolve(ctx, ref)
	require.NoError(t, err)
	require.Equal(t, app, credentials.AppID)
	require.Equal(t, *patch.KeySecret, credentials.KeySecret)
	for _, value := range []any{first, credentials} {
		encoded, err := json.Marshal(value)
		require.NoError(t, err)
		for _, secret := range []string{*patch.KeyID, *patch.KeySecret, *patch.WebhookSecret} {
			require.NotContains(t, string(encoded), secret)
		}
	}
	stored, err := bizbillingrepo.NewGatewayConnectionRepo(es, zap.NewNop()).Get(ctx, ref)
	require.NoError(t, err)
	binding := merchantSecretBinding(ref, app, "key_secret")
	plaintext, err := codec.Open(stored.Secrets["key_secret"], binding)
	require.NoError(t, err)
	require.Equal(t, *patch.KeySecret, string(plaintext))
	binding.AppID = "different-app"
	_, err = codec.Open(stored.Secrets["key_secret"], binding)
	require.Error(t, err, "ciphertext must not be transferable to another application")
	raw, err := es.Get(bizbillingrepo.GatewayConnectionIndex, bizbillingrepo.GatewayConnectionRevisionID(ref))
	require.NoError(t, err)
	data, err := io.ReadAll(raw.Body)
	raw.Body.Close()
	require.NoError(t, err)
	require.NotContains(t, string(data), *patch.KeySecret)
	require.NotContains(t, string(data), *patch.WebhookSecret)
	_, err = service.Save(ctx, app, bizbilling.GatewayTest, 0, patch)
	require.ErrorIs(t, err, bizbilling.ErrConflict)
	other, err := service.Snapshot(ctx, uuid.NewString(), bizbilling.GatewayTest)
	require.NoError(t, err)
	require.False(t, other.Configured)
	for _, wrong := range []bizbilling.GatewayRef{
		{ConnectionID: ref.ConnectionID, AccountID: "acc_other", Mode: ref.Mode, CredentialRevision: ref.CredentialRevision},
		{ConnectionID: ref.ConnectionID, AccountID: ref.AccountID, Mode: bizbilling.GatewayLive, CredentialRevision: ref.CredentialRevision},
	} {
		_, err = service.Resolve(ctx, wrong)
		require.Error(t, err)
	}
	_, err = service.Save(ctx, app, bizbilling.GatewayLive, 0, patch)
	require.Error(t, err, "test credentials must not configure a live slot")
	bad := "rejected-fixture-secret"
	_, err = service.Save(ctx, app, bizbilling.GatewayTest, 1, bizbilling.MerchantPatch{KeySecret: &bad})
	require.Error(t, err)
	require.NotContains(t, err.Error(), bad)
	empty, err := secrets.NewConnectionCodec("")
	require.NoError(t, err)
	withoutKeyring := NewBizGatewayConnectionService(es, service.gate, empty, service.authenticate, zap.NewNop())
	_, err = withoutKeyring.Save(ctx, uuid.NewString(), bizbilling.GatewayTest, 0, patch)
	require.Error(t, err)
}

func TestBizMerchantSlotSwitchKeepsOldOrder(t *testing.T) {
	service, gate, _, _, app := merchantFixture(t)
	ctx := context.Background()
	first, err := service.Save(ctx, app, bizbilling.GatewayTest, 0, merchantPatch("acc_fixture_a", "rzp_test_biz1", "old-api-secret", "old-hook-secret"))
	require.NoError(t, err)
	oldOrder := merchantReference(first)
	permit, err := gate.AcquireMutation(ctx, app)
	require.NoError(t, err)
	_, err = service.Save(ctx, app, bizbilling.GatewayTest, first.Revision, merchantPatch("acc_fixture_b", "rzp_test_biz2", "new-api-secret", "new-hook-secret"))
	require.ErrorIs(t, err, bizbilling.ErrActiveLegacyWriter)
	require.NoError(t, gate.CheckMutation(ctx, permit))
	require.NoError(t, gate.ReleaseMutation(ctx, permit))
	next, err := service.Save(ctx, app, bizbilling.GatewayTest, first.Revision, merchantPatch("acc_fixture_b", "rzp_test_biz2", "new-api-secret", "new-hook-secret"))
	require.NoError(t, err)
	require.NotEqual(t, first.ConnectionID, next.ConnectionID)
	require.Greater(t, next.Revision, first.Revision)
	original, err := service.Resolve(ctx, oldOrder)
	require.NoError(t, err)
	require.Equal(t, "acc_fixture_a", original.Reference.AccountID)
	require.Equal(t, "old-api-secret", original.KeySecret)
	require.NoError(t, service.Disconnect(ctx, app, bizbilling.GatewayTest, next.Revision))
	snapshot, err := service.Snapshot(ctx, app, bizbilling.GatewayTest)
	require.NoError(t, err)
	require.False(t, snapshot.Configured)
	require.Greater(t, snapshot.Revision, next.Revision, "disconnect must not reset the slot revision")
	_, err = service.Resolve(ctx, oldOrder)
	require.NoError(t, err, "late old captures must retain their original account binding")
	_, err = service.Save(ctx, app, bizbilling.GatewayTest, first.Revision, merchantPatch("acc_fixture_a", "rzp_test_biz1", "old-api-secret", "old-hook-secret"))
	require.ErrorIs(t, err, bizbilling.ErrConflict, "an old slot revision must never become valid again")
}

func TestBizMerchantRotationKeepsCallbackSecret(t *testing.T) {
	service, _, es, _, app := merchantFixture(t)
	ctx := context.Background()
	first, err := service.Save(ctx, app, bizbilling.GatewayTest, 0, merchantPatch("acc_fixture_a", "rzp_test_biz1", "first-api-secret", "first-hook-secret"))
	require.NoError(t, err)
	key, apiSecret, hook := "rzp_test_biz2", "rotated-api-secret", "rotated-hook-secret"
	rotated, err := service.Save(ctx, app, bizbilling.GatewayTest, first.Revision, bizbilling.MerchantPatch{KeyID: &key, KeySecret: &apiSecret, WebhookSecret: &hook})
	require.NoError(t, err)
	require.Equal(t, first.ConnectionID, rotated.ConnectionID)
	old, err := service.Resolve(ctx, merchantReference(first))
	require.NoError(t, err)
	require.Equal(t, "first-hook-secret", old.WebhookSecret)
	current, err := service.Resolve(ctx, merchantReference(rotated))
	require.NoError(t, err)
	require.Equal(t, hook, current.WebhookSecret)
	callbacks, err := service.CallbackCredentials(ctx, first.ConnectionID)
	require.NoError(t, err)
	require.Len(t, callbacks, 2)
	newCodec, err := secrets.NewConnectionCodec("different:" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{42}, 32)))
	require.NoError(t, err)
	missingOldKey := NewBizGatewayConnectionService(es, service.gate, newCodec, service.authenticate, zap.NewNop())
	_, err = missingOldKey.Resolve(ctx, merchantReference(first))
	require.Error(t, err)
	require.False(t, strings.Contains(err.Error(), "first-api-secret"))
}

func TestBizManualBillingDoesNotRequireMerchant(t *testing.T) {
	lifecycle, _, _, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	for _, method := range []string{"cash", "upi", "bank_transfer"} {
		invoice := seedFinancialInvoice(t, es, scope, 1000)
		payment, err := lifecycle.RecordPayment(ctx, scope.AppID, invoice.ID, &bizbilling.RecordPaymentRequest{Method: method, AmountPaisa: 1000, Notes: "Staff confirmed external receipt"})
		require.NoError(t, err)
		require.Equal(t, method, payment.Method)
		require.Equal(t, bizbilling.PaymentStatusSuccess, payment.Status)
		saved, err := lifecycle.GetInvoice(ctx, scope.AppID, invoice.ID)
		require.NoError(t, err)
		require.Equal(t, bizbilling.InvoiceStatusPaid, saved.Status)
		require.Zero(t, saved.AmountDuePaisa)
	}
}
