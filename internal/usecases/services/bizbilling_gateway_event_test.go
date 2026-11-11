package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func gatewayRaw(event, payment, order, account, currency string, amount int64) []byte {
	raw, _ := json.Marshal(map[string]any{"entity": "event", "account_id": account, "event": event, "payload": map[string]any{"payment": map[string]any{"entity": map[string]any{"id": payment, "order_id": order, "amount": amount, "currency": currency, "status": "captured", "captured": true}}}})
	return raw
}
func gatewaySignature(raw []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(raw)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestBizGatewayReplayIdentity(t *testing.T) {
	checkout, merchants, gateway, lifecycle, ops, es, scope := checkoutFixture(t)
	ctx := context.Background()
	templates := &database.IndexTemplates{}
	ensureBillingIndex(t, es, bizbillingrepo.GatewayEventIndex, templates.GetBizGatewayEventsTemplate())
	svc := NewBizGatewayEventService(es, merchants, checkout, zap.NewNop())
	inv := seedCheckoutInvoice(t, es, scope, 1000)
	desc, err := checkout.Start(ctx, scope.AppID, scope.UserID, inv.ID, bizbilling.GatewayTest, "start")
	require.NoError(t, err)
	gateway.payment = bizbilling.CapturedPayment{ID: "pay_fixture", OrderID: desc.OrderID, Gateway: gateway.ref, AmountPaisa: 1000, Currency: scope.Currency, Captured: true, Status: "captured"}
	raw := gatewayRaw("payment.captured", "pay_fixture", desc.OrderID, gateway.ref.AccountID, scope.Currency, 1000)
	signature := gatewaySignature(raw, "owned-hook-secret")
	first, err := svc.Accept(ctx, gateway.ref.ConnectionID, "event-one", raw, signature)
	require.NoError(t, err)
	alias, err := svc.Accept(ctx, gateway.ref.ConnectionID, "changed-unsigned-header", raw, signature)
	require.NoError(t, err)
	require.Equal(t, first.ID, alias.ID)
	one, err := svc.Process(ctx, first.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.OperationCompleted, one.State)
	verified, err := merchants.Snapshot(ctx, scope.AppID, bizbilling.GatewayTest)
	require.NoError(t, err)
	require.True(t, verified.AccountVerified)
	require.False(t, verified.AutomaticReady, "a payment capture does not attest recurring capability")
	secondRaw := gatewayRaw("order.paid", "pay_fixture", desc.OrderID, gateway.ref.AccountID, scope.Currency, 1000)
	second, err := svc.Accept(ctx, gateway.ref.ConnectionID, "event-one", secondRaw, gatewaySignature(secondRaw, "owned-hook-secret"))
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)
	require.True(t, second.HeaderConflict, "header reuse must be reported without suppressing the second signed body")
	two, err := svc.Process(ctx, second.ID)
	require.NoError(t, err)
	require.Equal(t, one.OperationID, two.OperationID)
	saved, err := lifecycle.GetInvoice(ctx, scope.AppID, inv.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1000), saved.AmountPaidPaisa)
	require.NoError(t, ops.stores.Payments.Refresh(ctx))
	rows, _, err := ops.stores.Payments.Find(ctx, map[string]any{"query": map[string]any{"term": map[string]any{"app_id": scope.AppID}}, "size": 10})
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func TestBizGatewayOutOfOrder(t *testing.T) {
	checkout, merchants, gateway, lifecycle, _, es, scope := checkoutFixture(t)
	ctx := context.Background()
	ensureBillingIndex(t, es, bizbillingrepo.GatewayEventIndex, (&database.IndexTemplates{}).GetBizGatewayEventsTemplate())
	svc := NewBizGatewayEventService(es, merchants, checkout, zap.NewNop())
	inv := seedCheckoutInvoice(t, es, scope, 1000)
	desc, err := checkout.Start(ctx, scope.AppID, scope.UserID, inv.ID, bizbilling.GatewayTest, "start")
	require.NoError(t, err)
	gateway.payment = bizbilling.CapturedPayment{ID: "pay_fixture", OrderID: desc.OrderID, Gateway: gateway.ref, AmountPaisa: 1000, Currency: scope.Currency, Captured: true, Status: "captured"}
	var operation string
	for _, event := range []string{"payment.captured", "payment.failed", "payment.authorized", "order.paid"} {
		raw := gatewayRaw(event, "pay_fixture", desc.OrderID, gateway.ref.AccountID, scope.Currency, 1000)
		receipt, err := svc.Accept(ctx, gateway.ref.ConnectionID, event, raw, gatewaySignature(raw, "owned-hook-secret"))
		require.NoError(t, err)
		result, err := svc.Process(ctx, receipt.ID)
		require.NoError(t, err)
		if operation == "" {
			operation = result.OperationID
		}
		require.Equal(t, operation, result.OperationID)
	}
	saved, err := lifecycle.GetInvoice(ctx, scope.AppID, inv.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.InvoiceStatusPaid, saved.Status)
	require.Equal(t, int64(1000), saved.AmountPaidPaisa)
}

func TestBizGatewayWrongAccountOrMoney(t *testing.T) {
	checkout, merchants, gateway, lifecycle, _, es, scope := checkoutFixture(t)
	ctx := context.Background()
	ensureBillingIndex(t, es, bizbillingrepo.GatewayEventIndex, (&database.IndexTemplates{}).GetBizGatewayEventsTemplate())
	svc := NewBizGatewayEventService(es, merchants, checkout, zap.NewNop())
	inv := seedCheckoutInvoice(t, es, scope, 1000)
	desc, err := checkout.Start(ctx, scope.AppID, scope.UserID, inv.ID, bizbilling.GatewayTest, "start")
	require.NoError(t, err)
	gateway.payment = bizbilling.CapturedPayment{ID: "pay_fixture", OrderID: desc.OrderID, Gateway: gateway.ref, AmountPaisa: 1000, Currency: scope.Currency, Captured: true, Status: "captured"}
	badAccount := gatewayRaw("payment.captured", "pay_fixture", desc.OrderID, "acc_wrong", scope.Currency, 1000)
	_, err = svc.Accept(ctx, gateway.ref.ConnectionID, "wrong-account", badAccount, gatewaySignature(badAccount, "owned-hook-secret"))
	require.Error(t, err)
	for _, money := range []struct {
		currency string
		amount   int64
	}{{"USD", 1000}, {scope.Currency, 999}} {
		raw := gatewayRaw("payment.captured", "pay_fixture", desc.OrderID, gateway.ref.AccountID, money.currency, money.amount)
		receipt, err := svc.Accept(ctx, gateway.ref.ConnectionID, "wrong-money", raw, gatewaySignature(raw, "owned-hook-secret"))
		require.NoError(t, err)
		_, err = svc.Process(ctx, receipt.ID)
		require.Error(t, err)
	}
	raw := gatewayRaw("payment.captured", "pay_fixture", desc.OrderID, gateway.ref.AccountID, scope.Currency, 1000)
	_, err = svc.Accept(ctx, gateway.ref.ConnectionID, "tampered", append(raw, ' '), gatewaySignature(raw, "owned-hook-secret"))
	require.Error(t, err)
	saved, err := lifecycle.GetInvoice(ctx, scope.AppID, inv.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.InvoiceStatusOpen, saved.Status)
	require.Zero(t, saved.AmountPaidPaisa)
}

func TestBizGatewayUnmatchedAndRestartRecovery(t *testing.T) {
	checkout, merchants, gateway, lifecycle, _, es, scope := checkoutFixture(t)
	ctx := context.Background()
	ensureBillingIndex(t, es, bizbillingrepo.GatewayEventIndex, (&database.IndexTemplates{}).GetBizGatewayEventsTemplate())
	svc := NewBizGatewayEventService(es, merchants, checkout, zap.NewNop())
	unknown := gatewayRaw("payment.captured", "pay_unknown", "order_unknown", gateway.ref.AccountID, scope.Currency, 1000)
	receipt, err := svc.Accept(ctx, gateway.ref.ConnectionID, "unknown", unknown, gatewaySignature(unknown, "owned-hook-secret"))
	require.NoError(t, err)
	_, err = svc.Process(ctx, receipt.ID)
	require.Error(t, err)
	stored, err := svc.inbox.Get(ctx, receipt.ID)
	require.NoError(t, err)
	require.Equal(t, "unmatched", stored.State)
	inv := seedCheckoutInvoice(t, es, scope, 1000)
	desc, err := checkout.Start(ctx, scope.AppID, scope.UserID, inv.ID, bizbilling.GatewayTest, "start")
	require.NoError(t, err)
	gateway.payment = bizbilling.CapturedPayment{ID: "pay_fixture", OrderID: desc.OrderID, Gateway: gateway.ref, AmountPaisa: 1000, Currency: scope.Currency, Captured: true, Status: "captured"}
	raw := gatewayRaw("payment.captured", "pay_fixture", desc.OrderID, gateway.ref.AccountID, scope.Currency, 1000)
	accepted, err := svc.Accept(ctx, gateway.ref.ConnectionID, "before-restart", raw, gatewaySignature(raw, "owned-hook-secret"))
	require.NoError(t, err)
	require.NoError(t, merchants.Disconnect(ctx, scope.AppID, bizbilling.GatewayTest, 1))
	restarted := NewBizGatewayEventService(es, merchants, checkout, zap.NewNop())
	_, err = restarted.Process(ctx, accepted.ID)
	require.NoError(t, err)
	saved, err := lifecycle.GetInvoice(ctx, scope.AppID, inv.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.InvoiceStatusPaid, saved.Status)
}

func TestBizGatewayRotationAccountProof(t *testing.T) {
	checkout, merchants, gateway, _, _, es, scope := checkoutFixture(t)
	ctx := context.Background()
	ensureBillingIndex(t, es, bizbillingrepo.GatewayEventIndex, (&database.IndexTemplates{}).GetBizGatewayEventsTemplate())
	svc := NewBizGatewayEventService(es, merchants, checkout, zap.NewNop())
	inv := seedCheckoutInvoice(t, es, scope, 1000)
	desc, err := checkout.Start(ctx, scope.AppID, scope.UserID, inv.ID, bizbilling.GatewayTest, "start")
	require.NoError(t, err)
	original := gateway.ref
	key, secret, hook := "rzp_test_biz2", "rotated-api-secret", "rotated-hook-secret"
	_, err = merchants.Save(ctx, scope.AppID, bizbilling.GatewayTest, 1, bizbilling.MerchantPatch{KeyID: &key, KeySecret: &secret, WebhookSecret: &hook})
	require.NoError(t, err)
	before, err := merchants.ResolveAPI(ctx, original)
	require.NoError(t, err)
	require.Equal(t, original, before.Reference, "a new key alone must not retarget old orders")
	gateway.payment = bizbilling.CapturedPayment{ID: "pay_rotated", OrderID: desc.OrderID, Gateway: original, AmountPaisa: 1000, Currency: scope.Currency, Captured: true, Status: "captured"}
	raw := gatewayRaw("payment.captured", "pay_rotated", desc.OrderID, original.AccountID, scope.Currency, 1000)
	old, err := svc.Accept(ctx, original.ConnectionID, "old-secret-retry", raw, gatewaySignature(raw, "owned-hook-secret"))
	require.NoError(t, err)
	one, err := svc.Process(ctx, old.ID)
	require.NoError(t, err)
	current, err := merchants.Snapshot(ctx, scope.AppID, bizbilling.GatewayTest)
	require.NoError(t, err)
	require.False(t, current.AccountVerified, "old-key proof must not attest the new key")
	gateway.ref.CredentialRevision = 2
	gateway.payment.Gateway = gateway.ref
	newReceipt, err := svc.Accept(ctx, original.ConnectionID, "new-secret-replay", raw, gatewaySignature(raw, hook))
	require.NoError(t, err)
	require.Equal(t, old.ID, newReceipt.ID)
	two, err := svc.Process(ctx, newReceipt.ID)
	require.NoError(t, err)
	require.Equal(t, one.OperationID, two.OperationID)
	current, err = merchants.Snapshot(ctx, scope.AppID, bizbilling.GatewayTest)
	require.NoError(t, err)
	require.True(t, current.AccountVerified)
	require.False(t, current.AutomaticReady)
	lookup, err := merchants.ResolveAPI(ctx, original)
	require.NoError(t, err)
	require.Equal(t, int64(2), lookup.Reference.CredentialRevision)
	require.Equal(t, secret, lookup.KeySecret)
	require.NoError(t, merchants.Disconnect(ctx, scope.AppID, bizbilling.GatewayTest, 2))
	_, err = merchants.Save(ctx, scope.AppID, bizbilling.GatewayTest, 3, merchantPatch("acc_other", "rzp_test_other", "other-api-secret", "other-hook-secret"))
	require.NoError(t, err)
	lookup, err = merchants.ResolveAPI(ctx, original)
	require.NoError(t, err)
	require.Equal(t, original.AccountID, lookup.Reference.AccountID)
	require.Equal(t, original.ConnectionID, lookup.Reference.ConnectionID)
}

func TestBizGatewayConcurrentCapture(t *testing.T) {
	checkout, merchants, gateway, lifecycle, _, es, scope := checkoutFixture(t)
	ctx := context.Background()
	ensureBillingIndex(t, es, bizbillingrepo.GatewayEventIndex, (&database.IndexTemplates{}).GetBizGatewayEventsTemplate())
	svc := NewBizGatewayEventService(es, merchants, checkout, zap.NewNop())
	inv := seedCheckoutInvoice(t, es, scope, 1000)
	desc, err := checkout.Start(ctx, scope.AppID, scope.UserID, inv.ID, bizbilling.GatewayTest, "start")
	require.NoError(t, err)
	gateway.payment = bizbilling.CapturedPayment{ID: "pay_concurrent", OrderID: desc.OrderID, Gateway: gateway.ref, AmountPaisa: 1000, Currency: scope.Currency, Captured: true, Status: "captured"}
	ids := []string{}
	for _, name := range []string{"payment.captured", "order.paid"} {
		raw := gatewayRaw(name, gateway.payment.ID, desc.OrderID, gateway.ref.AccountID, scope.Currency, 1000)
		receipt, err := svc.Accept(ctx, gateway.ref.ConnectionID, name, raw, gatewaySignature(raw, "owned-hook-secret"))
		require.NoError(t, err)
		ids = append(ids, receipt.ID)
	}
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id string) { defer wg.Done(); _, _ = svc.Process(ctx, id) }(id)
	}
	wg.Wait()
	var operation string
	for _, id := range ids {
		result, err := svc.Process(ctx, id)
		require.NoError(t, err)
		if operation == "" {
			operation = result.OperationID
		}
		require.Equal(t, operation, result.OperationID)
	}
	saved, err := lifecycle.GetInvoice(ctx, scope.AppID, inv.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1000), saved.AmountPaidPaisa)
}

func TestBizInboxLostAcknowledgment(t *testing.T) {
	fault := &lostBillingResponse{match: func(r *http.Request, body []byte) bool {
		return strings.Contains(r.URL.Path, bizbillingrepo.GatewayEventIndex+"/_doc/") && bytes.Contains(body, []byte(`"kind":"receipt"`))
	}}
	checkout, merchants, gateway, _, _, es, scope := checkoutFixture(t, fault)
	ctx := context.Background()
	ensureBillingIndex(t, es, bizbillingrepo.GatewayEventIndex, (&database.IndexTemplates{}).GetBizGatewayEventsTemplate())
	svc := NewBizGatewayEventService(es, merchants, checkout, zap.NewNop())
	fault.armed.Store(true)
	raw := gatewayRaw("payment.captured", "pay_fixture", "order_fixture", gateway.ref.AccountID, scope.Currency, 1000)
	receipt, err := svc.Accept(ctx, gateway.ref.ConnectionID, "persisted-but-ack-lost", raw, gatewaySignature(raw, "owned-hook-secret"))
	require.NoError(t, err)
	require.False(t, fault.armed.Load())
	stored, err := svc.inbox.Get(ctx, receipt.ID)
	require.NoError(t, err)
	require.Equal(t, receipt.ID, stored.ID)
}
