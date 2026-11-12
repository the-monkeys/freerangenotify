package services

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"go.uber.org/zap"
)

type bizFactoryFunc func(context.Context, bizbilling.GatewayRef) (bizbilling.BizGateway, error)

func (f bizFactoryFunc) For(ctx context.Context, ref bizbilling.GatewayRef) (bizbilling.BizGateway, error) {
	return f(ctx, ref)
}

type checkoutGateway struct {
	mu          sync.Mutex
	ref         bizbilling.GatewayRef
	orders      map[string]bizbilling.GatewayOrder
	payment     bizbilling.CapturedPayment
	creates     int
	loseCreate  bool
	lookupError bool
}

func (g *checkoutGateway) Refund(context.Context, bizbilling.RefundIntent) (bizbilling.GatewayRefund, error) {
	return bizbilling.GatewayRefund{}, bizbilling.ErrMerchantNotReady
}
func (g *checkoutGateway) FindRefund(context.Context, bizbilling.RefundIntent) (*bizbilling.GatewayRefund, error) {
	return nil, bizbilling.ErrMerchantNotReady
}

func (g *checkoutGateway) CreateOrder(_ context.Context, intent bizbilling.OrderIntent) (bizbilling.GatewayOrder, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.creates++
	order := bizbilling.GatewayOrder{ID: "order_" + uuid.NewString(), Intent: intent}
	g.orders[intent.Receipt] = order
	if g.loseCreate {
		return bizbilling.GatewayOrder{}, errors.New("fixture lost provider response")
	}
	return order, nil
}
func (g *checkoutGateway) FindOrder(_ context.Context, intent bizbilling.OrderIntent) (*bizbilling.GatewayOrder, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.lookupError {
		return nil, errors.New("fixture unavailable")
	}
	order, ok := g.orders[intent.Receipt]
	if !ok {
		return nil, nil
	}
	return &order, nil
}
func (g *checkoutGateway) FetchPayment(_ context.Context, id string) (bizbilling.CapturedPayment, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.payment.ID != id {
		return bizbilling.CapturedPayment{}, errors.New("fixture missing payment")
	}
	return g.payment, nil
}
func (g *checkoutGateway) VerifyCheckout(order, payment, signature string) error {
	if signature != "fixture-valid" {
		return bizbilling.ErrInvalidGatewayProof
	}
	return nil
}

func checkoutFixture(t *testing.T, faults ...*lostBillingResponse) (*BizCheckoutService, *BizGatewayConnectionService, *checkoutGateway, *BizLifecycleService, *BizOperationService, *elasticsearch.Client, bizbilling.Scope) {
	t.Helper()
	lifecycle, credit, ops, es, scope := financialFacadesFixture(t, faults...)
	scope.Mode = bizbilling.GatewayTest
	templates := &database.IndexTemplates{}
	ensureBillingIndex(t, es, bizbillingrepo.LifecycleStateIndex, templates.GetBizLifecycleStatesTemplate())
	ensureBillingIndex(t, es, bizbillingrepo.GatewayConnectionIndex, templates.GetBizGatewayConnectionsTemplate())
	ensureBillingIndex(t, es, bizbillingrepo.GatewayEffectIndex, templates.GetBizGatewayEffectsTemplate())
	state := bizbilling.DefaultLifecycleState(scope.AppID)
	state.LifecycleVersion = 2
	state.Status = "active"
	state.PrimaryCurrency = scope.Currency
	stateRepo := bizbillingrepo.NewLifecycleStateRepo(es)
	require.NoError(t, stateRepo.Mutate(context.Background(), scope.AppID, func(s *bizbilling.LifecycleState) error { *s = state; return nil }))
	gate := NewBizLifecycleGate(stateRepo)
	ops.SetLifecycleGate(gate)
	credit.SetLifecycleGate(gate)
	lifecycle.SetLifecycleGate(gate)
	_, _, _, codec, _ := merchantFixture(t)
	merchant := NewBizGatewayConnectionService(es, gate, codec, func(context.Context, bizbilling.MerchantCredentials) error { return nil }, zap.NewNop())
	snapshot, err := merchant.Save(context.Background(), scope.AppID, bizbilling.GatewayTest, 0, merchantPatch("acc_fixture_a", "rzp_test_biz1", "owned-api-secret", "owned-hook-secret"))
	require.NoError(t, err)
	gateway := &checkoutGateway{ref: merchantReference(snapshot), orders: map[string]bizbilling.GatewayOrder{}}
	factory := bizFactoryFunc(func(_ context.Context, ref bizbilling.GatewayRef) (bizbilling.BizGateway, error) {
		require.Equal(t, gateway.ref.ConnectionID, ref.ConnectionID, "must use the invoice's own business account")
		require.Equal(t, gateway.ref.AccountID, ref.AccountID)
		require.Equal(t, gateway.ref.Mode, ref.Mode)
		return gateway, nil
	})
	checkout := NewBizCheckoutService(es, lifecycle, merchant, factory, zap.NewNop())
	checkout.SetNewCollectionEnabled(true)
	return checkout, merchant, gateway, lifecycle, ops, es, scope
}

func TestBizCheckoutMerchantIsolation(t *testing.T) {
	a, _, ga, la, _, ea, sa := checkoutFixture(t)
	b, _, gb, _, _, eb, sb := checkoutFixture(t)
	ctx := context.Background()
	ia := seedCheckoutInvoice(t, ea, sa, 1000)
	ib := seedCheckoutInvoice(t, eb, sb, 2500)
	_, err := la.RecordPayment(ctx, sa.AppID, ia.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: 300})
	require.NoError(t, err)
	da, err := a.Start(ctx, sa.AppID, sa.UserID, ia.ID, bizbilling.GatewayTest, "checkout-a")
	require.NoError(t, err)
	require.Equal(t, int64(700), da.AmountPaisa)
	db, err := b.Start(ctx, sb.AppID, sb.UserID, ib.ID, bizbilling.GatewayTest, "checkout-b")
	require.NoError(t, err)
	require.Equal(t, int64(2500), db.AmountPaisa)
	require.NotEqual(t, da.OrderID, db.OrderID)
	require.Equal(t, 1, ga.creates)
	require.Equal(t, 1, gb.creates)
	_, err = b.Start(ctx, sb.AppID, sb.UserID, ia.ID, bizbilling.GatewayTest, "wrong-app")
	require.Error(t, err)
	_, err = a.Start(ctx, sa.AppID, uuid.NewString(), ia.ID, bizbilling.GatewayTest, "wrong-user")
	require.Error(t, err)
	_, err = a.Start(ctx, sa.AppID, sa.UserID, ia.ID, bizbilling.GatewayLive, "wrong-mode")
	require.Error(t, err)
	_, err = a.Start(ctx, sa.AppID, sa.UserID, seedCheckoutInvoice(t, ea, sa, 500).ID, bizbilling.GatewayTest, "checkout-a")
	require.ErrorIs(t, err, bizbilling.ErrIdempotencyConflict)
	require.Equal(t, 1, ga.creates)
}

func TestBizCheckoutRejectsClosedOrAutomaticInvoice(t *testing.T) {
	checkout, _, gateway, _, _, es, scope := checkoutFixture(t)
	ctx := context.Background()
	for _, status := range []bizbilling.InvoiceStatus{bizbilling.InvoiceStatusPaid, bizbilling.InvoiceStatusVoid, bizbilling.InvoiceStatusOpen} {
		inv := seedCheckoutInvoice(t, es, scope, 1000)
		inv.Status = status
		if status == bizbilling.InvoiceStatusOpen {
			inv.CollectionOwner = "automatic-existing"
		}
		require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizInvoice](es, "frn_biz_invoices", zap.NewNop()).Replace(ctx, inv.ID, inv))
		_, err := checkout.Start(ctx, scope.AppID, scope.UserID, inv.ID, bizbilling.GatewayTest, uuid.NewString())
		require.Error(t, err)
	}
	require.Zero(t, gateway.creates)
}

func TestBizCheckoutVerifyServerFacts(t *testing.T) {
	for _, wrong := range []string{"signature", "authorized", "amount", "currency", "order", "account", "payment", "valid"} {
		t.Run(wrong, func(t *testing.T) {
			checkout, _, gateway, lifecycle, _, es, scope := checkoutFixture(t)
			ctx := context.Background()
			inv := seedCheckoutInvoice(t, es, scope, 1000)
			desc, err := checkout.Start(ctx, scope.AppID, scope.UserID, inv.ID, bizbilling.GatewayTest, "start")
			require.NoError(t, err)
			gateway.payment = bizbilling.CapturedPayment{ID: "pay_fixture", OrderID: desc.OrderID, Gateway: gateway.ref, AmountPaisa: 1000, Currency: scope.Currency, Captured: true, Status: "captured"}
			proof := bizbilling.CheckoutProof{OrderID: desc.OrderID, PaymentID: "pay_fixture", Signature: "fixture-valid"}
			switch wrong {
			case "signature":
				proof.Signature = "tampered"
			case "authorized":
				gateway.payment.Captured = false
				gateway.payment.Status = "authorized"
			case "amount":
				gateway.payment.AmountPaisa = 999
			case "currency":
				gateway.payment.Currency = "USD"
			case "order":
				gateway.payment.OrderID = "order_other"
			case "account":
				gateway.payment.Gateway.AccountID = "acc_other"
			case "payment":
				proof.PaymentID = "pay_other"
			}
			result, err := checkout.Verify(ctx, scope.AppID, scope.UserID, inv.ID, proof)
			if wrong == "valid" {
				require.NoError(t, err)
				require.Equal(t, bizbilling.OperationCompleted, result.State)
				replay, err := checkout.Verify(ctx, scope.AppID, scope.UserID, inv.ID, proof)
				require.NoError(t, err)
				require.Equal(t, result.OperationID, replay.OperationID)
				payment, err := lifecycle.GetPayment(ctx, scope.AppID, operationResource(t, result, "payment"))
				require.NoError(t, err)
				require.Equal(t, gateway.ref, *payment.GatewayBinding)
			} else {
				require.Error(t, err)
			}
			saved, err := lifecycle.GetInvoice(ctx, scope.AppID, inv.ID)
			require.NoError(t, err)
			if wrong == "valid" {
				require.Equal(t, bizbilling.InvoiceStatusPaid, saved.Status)
			} else {
				require.Equal(t, bizbilling.InvoiceStatusOpen, saved.Status)
				require.Zero(t, saved.AmountPaidPaisa)
			}
		})
	}
}

func TestBizOrderUnknownOutcomeDoesNotRecreate(t *testing.T) {
	checkout, merchant, gateway, _, _, es, scope := checkoutFixture(t)
	ctx := context.Background()
	inv := seedCheckoutInvoice(t, es, scope, 1000)
	gateway.loseCreate = true
	gateway.lookupError = true
	first, err := checkout.Start(ctx, scope.AppID, scope.UserID, inv.ID, bizbilling.GatewayTest, "start")
	require.Error(t, err)
	require.NotEmpty(t, first.OperationID)
	require.Equal(t, 1, gateway.creates)
	_, err = checkout.Start(ctx, scope.AppID, scope.UserID, inv.ID, bizbilling.GatewayTest, "different-key")
	require.Error(t, err)
	require.Equal(t, 1, gateway.creates)
	rotated := "rotated-api-secret"
	_, err = merchant.Save(ctx, scope.AppID, bizbilling.GatewayTest, 1, bizbilling.MerchantPatch{KeySecret: &rotated})
	require.NoError(t, err)
	gateway.lookupError = false
	checkout.SetNewCollectionEnabled(false)
	recovered, err := checkout.Start(ctx, scope.AppID, scope.UserID, inv.ID, bizbilling.GatewayTest, "start")
	require.NoError(t, err)
	require.NotEmpty(t, recovered.OrderID)
	require.Equal(t, first.OperationID, recovered.OperationID)
	require.Equal(t, 1, gateway.creates)
	_, err = checkout.Start(ctx, scope.AppID, scope.UserID, seedCheckoutInvoice(t, es, scope, 500).ID, bizbilling.GatewayTest, "new-after-disable")
	require.Error(t, err)
	require.Equal(t, 1, gateway.creates)
}

func seedCheckoutInvoice(t *testing.T, es *elasticsearch.Client, scope bizbilling.Scope, amount int64) *bizbilling.BizInvoice {
	t.Helper()
	inv := seedFinancialInvoice(t, es, scope, amount)
	inv.GatewayMode = bizbilling.GatewayTest
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizInvoice](es, "frn_biz_invoices", zap.NewNop()).Replace(context.Background(), inv.ID, inv))
	return inv
}
func TestBizCheckoutTestPaymentCannotSettleLiveInvoice(t *testing.T) {
	checkout, _, gateway, _, _, es, scope := checkoutFixture(t)
	inv := seedFinancialInvoice(t, es, scope, 1000)
	_, err := checkout.Start(context.Background(), scope.AppID, scope.UserID, inv.ID, bizbilling.GatewayTest, "test-for-live")
	require.Error(t, err)
	require.Zero(t, gateway.creates)
}

func TestBizOrderPersistedResponseRecovery(t *testing.T) {
	fault := &lostBillingResponse{match: func(r *http.Request, body []byte) bool {
		return strings.Contains(r.URL.Path, bizbillingrepo.GatewayEffectIndex+"/_doc/") && bytes.Contains(body, []byte(`"provider_id":"order_`))
	}}
	checkout, _, gateway, _, _, es, scope := checkoutFixture(t, fault)
	inv := seedCheckoutInvoice(t, es, scope, 1000)
	fault.armed.Store(true)
	desc, err := checkout.Start(context.Background(), scope.AppID, scope.UserID, inv.ID, bizbilling.GatewayTest, "start")
	require.NoError(t, err)
	require.NotEmpty(t, desc.OrderID)
	require.False(t, fault.armed.Load())
	require.Equal(t, 1, gateway.creates)
	restarted := NewBizCheckoutService(es, checkout.lifecycle, checkout.merchants, checkout.factory, zap.NewNop())
	replayed, err := restarted.Start(context.Background(), scope.AppID, scope.UserID, inv.ID, bizbilling.GatewayTest, "start")
	require.NoError(t, err)
	require.Equal(t, desc, replayed)
	require.Equal(t, 1, gateway.creates)
}

func TestBizCheckoutLegacyModeNotSilentlyIgnored(t *testing.T) {
	service := &BizLifecycleService{logger: zap.NewNop()}
	require.NotPanics(t, func() {
		_, err := service.CreateOneTimeInvoice(context.Background(), "legacy", &bizbilling.CreateInvoiceRequest{GatewayMode: bizbilling.GatewayTest, UserID: "user", Currency: "INR", LineItems: []bizbilling.InvoiceLineItem{{Description: "Service", Quantity: 1, UnitPrice: 1000}}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "upgrade")
	})
}

func TestBizCheckoutRemainingBalanceAfterEarlierCapture(t *testing.T) {
	checkout, _, gateway, lifecycle, ops, es, scope := checkoutFixture(t)
	ctx := context.Background()
	inv := seedCheckoutInvoice(t, es, scope, 1000)
	due := time.Now().Add(-48 * time.Hour)
	inv.DueDate = &due
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizInvoice](es, "frn_biz_invoices", zap.NewNop()).Replace(ctx, inv.ID, inv))
	first, err := checkout.Start(ctx, scope.AppID, scope.UserID, inv.ID, bizbilling.GatewayTest, "first")
	require.NoError(t, err)
	_, err = ops.config.UpdateConfig(ctx, scope.AppID, bizbilling.ConfigTypeLateFees, map[string]interface{}{"enabled": true, "type": "fixed", "value": 100, "grace_days": 0})
	require.NoError(t, err)
	_, err = ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandInvoiceChange, Scope: scope, IdempotencyKey: "late-fee", InvoiceChange: &bizbilling.InvoiceChange{InvoiceID: inv.ID, Action: "late_fee"}})
	require.NoError(t, err)
	gateway.payment = bizbilling.CapturedPayment{ID: "pay_first", OrderID: first.OrderID, Gateway: gateway.ref, AmountPaisa: 1000, Currency: scope.Currency, Captured: true, Status: "captured"}
	_, err = checkout.Verify(ctx, scope.AppID, scope.UserID, inv.ID, bizbilling.CheckoutProof{OrderID: first.OrderID, PaymentID: "pay_first", Signature: "fixture-valid"})
	require.NoError(t, err)
	second, err := checkout.Start(ctx, scope.AppID, scope.UserID, inv.ID, bizbilling.GatewayTest, "remainder")
	require.NoError(t, err, "a settled old order must not permanently prevent paying the remaining balance")
	require.Equal(t, int64(100), second.AmountPaisa)
	require.NotEqual(t, first.OrderID, second.OrderID)
	require.Equal(t, 2, gateway.creates)
	gateway.payment = bizbilling.CapturedPayment{ID: "pay_second", OrderID: second.OrderID, Gateway: gateway.ref, AmountPaisa: 100, Currency: scope.Currency, Captured: true, Status: "captured"}
	_, err = checkout.Verify(ctx, scope.AppID, scope.UserID, inv.ID, bizbilling.CheckoutProof{OrderID: second.OrderID, PaymentID: "pay_second", Signature: "fixture-valid"})
	require.NoError(t, err)
	saved, err := lifecycle.GetInvoice(ctx, scope.AppID, inv.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.InvoiceStatusPaid, saved.Status)
	require.Equal(t, int64(1100), saved.AmountPaidPaisa)
}
