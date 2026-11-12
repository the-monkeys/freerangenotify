package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
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

type refundGateway struct {
	*checkoutGateway
	refunds         map[string]bizbilling.GatewayRefund
	refundCalls     int
	refundStatus    string
	loseRefund      bool
	findUnavailable bool
	authRejected    bool
}

func (g *refundGateway) Refund(_ context.Context, intent bizbilling.RefundIntent) (bizbilling.GatewayRefund, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.refundCalls++
	if g.authRejected {
		return bizbilling.GatewayRefund{}, bizbilling.ErrMerchantAuthentication
	}
	refund := bizbilling.GatewayRefund{ID: "rfnd_" + strings.ReplaceAll(uuid.NewString(), "-", ""), Intent: intent, Status: g.refundStatus}
	g.refunds[intent.Receipt] = refund
	if g.loseRefund {
		return bizbilling.GatewayRefund{}, errors.New("fixture refund response lost")
	}
	return refund, nil
}
func (g *refundGateway) FindRefund(_ context.Context, intent bizbilling.RefundIntent) (*bizbilling.GatewayRefund, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.findUnavailable {
		return nil, errors.New("fixture refund lookup unavailable")
	}
	refund, ok := g.refunds[intent.Receipt]
	if !ok {
		return nil, nil
	}
	refund.Status = g.refundStatus
	return &refund, nil
}
func refundFixture(t *testing.T, faults ...*lostBillingResponse) (*BizRefundService, *refundGateway, *BizLifecycleService, *BizOperationService, *BizGatewayConnectionService, *elasticsearch.Client, bizbilling.Scope, *bizbilling.BizInvoice, *bizbilling.BizPayment) {
	t.Helper()
	_, merchants, base, lifecycle, ops, es, scope := checkoutFixture(t, faults...)
	ctx := context.Background()
	inv := seedCheckoutInvoice(t, es, scope, 1000)
	// Trusted capture journal boundary: the provider received1200 for a1000
	// invoice, producing a traceable200 excess lot. Checkout validation is intact.
	result, err := ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandGatewayCapture, Scope: scope, IdempotencyKey: "captured-fixture", ManualPayment: &bizbilling.ManualPayment{InvoiceID: inv.ID, AmountPaisa: 1200, Method: "gateway", GatewayPaymentID: "pay_refund", VerifiedCapture: true, GatewayBinding: &base.ref}})
	require.NoError(t, err)
	payment, err := lifecycle.GetPayment(ctx, scope.AppID, operationResource(t, result, "payment"))
	require.NoError(t, err)
	base.payment = bizbilling.CapturedPayment{ID: "pay_refund", OrderID: "order_refund", Gateway: base.ref, AmountPaisa: 1200, Currency: scope.Currency, Captured: true, Status: "captured"}
	gateway := &refundGateway{checkoutGateway: base, refunds: map[string]bizbilling.GatewayRefund{}, refundStatus: "pending"}
	factory := bizFactoryFunc(func(_ context.Context, ref bizbilling.GatewayRef) (bizbilling.BizGateway, error) {
		if !sameMerchantIdentity(base.ref, ref) {
			return nil, bizbilling.ErrMerchantUnavailable
		}
		return gateway, nil
	})
	service := NewBizRefundService(es, lifecycle, merchants, factory, zap.NewNop())
	lifecycle.SetRefundService(service)
	return service, gateway, lifecycle, ops, merchants, es, scope, inv, payment
}

func TestBizRefundOnlyAfterProviderConfirmation(t *testing.T) {
	service, gateway, lifecycle, _, _, _, scope, _, payment := refundFixture(t)
	ctx := context.Background()
	request := bizbilling.RefundPaymentRequest{AmountPaisa: 1000, Reason: "customer return"}
	pending, err := service.Request(ctx, scope.AppID, payment.ID, request, "refund-one")
	require.Error(t, err)
	require.Equal(t, bizbilling.OperationAccepted, pending.State)
	require.NotEmpty(t, pending.OperationID)
	held, err := lifecycle.GetPayment(ctx, scope.AppID, payment.ID)
	require.NoError(t, err)
	require.Zero(t, held.RefundedPaisa)
	require.Equal(t, int64(1000), held.RefundReservedPaisa)
	require.Equal(t, 1, gateway.refundCalls)
	gateway.refundStatus = "processed"
	done, err := service.Request(ctx, scope.AppID, payment.ID, request, "refund-one")
	require.NoError(t, err)
	require.Equal(t, bizbilling.OperationCompleted, done.State)
	saved, err := lifecycle.GetPayment(ctx, scope.AppID, payment.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1000), saved.RefundedPaisa)
	require.Zero(t, saved.RefundReservedPaisa)
	require.Equal(t, 1, gateway.refundCalls)
	again, err := service.Request(ctx, scope.AppID, payment.ID, request, "refund-one")
	require.NoError(t, err)
	require.Equal(t, done.OperationID, again.OperationID)
}

func TestBizRefundSpentCreditCap(t *testing.T) {
	service, gateway, lifecycle, ops, _, es, scope, _, payment := refundFixture(t)
	ctx := context.Background()
	other := seedCheckoutInvoice(t, es, scope, 200)
	_, err := ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandBalanceApplication, Scope: scope, IdempotencyKey: "spend-excess", BalanceApplication: &bizbilling.BalanceApplication{InvoiceID: other.ID}})
	require.NoError(t, err)
	_, err = service.Request(ctx, scope.AppID, payment.ID, bizbilling.RefundPaymentRequest{AmountPaisa: 1200, Reason: "return"}, "too-much")
	require.Error(t, err)
	require.Zero(t, gateway.refundCalls)
	gateway.refundStatus = "processed"
	_, err = service.Request(ctx, scope.AppID, payment.ID, bizbilling.RefundPaymentRequest{AmountPaisa: 1000, Reason: "principal"}, "principal")
	require.NoError(t, err)
	require.Equal(t, 1, gateway.refundCalls)
	saved, err := lifecycle.GetPayment(ctx, scope.AppID, payment.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1000), saved.RefundedPaisa)
	balance, err := ops.Balance(ctx, scope)
	require.NoError(t, err)
	require.Zero(t, balance.AvailablePaisa)
	require.Zero(t, balance.ReservedPaisa)
}

func TestBizRefundUnknownOutcomeAfterRotation(t *testing.T) {
	service, gateway, lifecycle, _, merchants, _, scope, _, payment := refundFixture(t)
	ctx := context.Background()
	request := bizbilling.RefundPaymentRequest{AmountPaisa: 1000, Reason: "return"}
	gateway.loseRefund = true
	gateway.findUnavailable = true
	first, err := service.Request(ctx, scope.AppID, payment.ID, request, "same-refund")
	require.Error(t, err)
	require.Equal(t, 1, gateway.refundCalls)
	key, secret := "rzp_test_biz2", "rotated-api-secret"
	_, err = merchants.Save(ctx, scope.AppID, bizbilling.GatewayTest, 1, bizbilling.MerchantPatch{KeyID: &key, KeySecret: &secret})
	require.NoError(t, err)
	held, err := lifecycle.GetPayment(ctx, scope.AppID, payment.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1000), held.RefundReservedPaisa)
	require.Zero(t, held.RefundedPaisa)
	gateway.findUnavailable = false
	gateway.refundStatus = "processed"
	done, err := service.Request(ctx, scope.AppID, payment.ID, request, "same-refund")
	require.NoError(t, err)
	require.Equal(t, bizbilling.OperationCompleted, done.State)
	require.Equal(t, 1, gateway.refundCalls)
	require.NotEmpty(t, first.OperationID)
}

func TestBizRefundDoesNotReopenInvoice(t *testing.T) {
	service, gateway, lifecycle, ops, _, _, scope, invoice, payment := refundFixture(t)
	ctx := context.Background()
	gateway.refundStatus = "processed"
	_, err := service.Request(ctx, scope.AppID, payment.ID, bizbilling.RefundPaymentRequest{Reason: "full return"}, "full")
	require.NoError(t, err)
	saved, err := lifecycle.GetInvoice(ctx, scope.AppID, invoice.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.InvoiceStatusPaid, saved.Status)
	require.Zero(t, saved.AmountDuePaisa)
	require.Equal(t, int64(1200), saved.CashRefundedPaisa)
	balance, err := ops.Balance(ctx, scope)
	require.NoError(t, err)
	require.Zero(t, balance.AvailablePaisa)
	require.Zero(t, balance.ReservedPaisa)
	gateway.payment.Status = "refunded"
	gateway.payment.RefundedPaisa = 1200
	again, err := service.Request(ctx, scope.AppID, payment.ID, bizbilling.RefundPaymentRequest{Reason: "again"}, "again")
	require.Error(t, err)
	require.Equal(t, bizbilling.OperationRejected, again.State, "a fully refunded payment is a definitive rejection, not uncertain new work")
	require.Equal(t, 1, gateway.refundCalls)
}

func TestBizRefundFailureReleasesReservations(t *testing.T) {
	service, gateway, lifecycle, ops, _, _, scope, _, payment := refundFixture(t)
	ctx := context.Background()
	request := bizbilling.RefundPaymentRequest{AmountPaisa: 1200, Reason: "return"}
	_, err := service.Request(ctx, scope.AppID, payment.ID, request, "failed")
	require.Error(t, err)
	balance, err := ops.Balance(ctx, scope)
	require.NoError(t, err)
	require.Zero(t, balance.AvailablePaisa)
	require.Equal(t, int64(200), balance.ReservedPaisa)
	gateway.refundStatus = "failed"
	_, err = service.Request(ctx, scope.AppID, payment.ID, request, "failed")
	require.Error(t, err)
	saved, err := lifecycle.GetPayment(ctx, scope.AppID, payment.ID)
	require.NoError(t, err)
	require.Zero(t, saved.RefundedPaisa)
	require.Zero(t, saved.RefundReservedPaisa)
	balance, err = ops.Balance(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, int64(200), balance.AvailablePaisa)
	require.Zero(t, balance.ReservedPaisa)
	_, err = service.Request(ctx, scope.AppID, payment.ID, request, "failed")
	require.Error(t, err)
	require.Equal(t, 1, gateway.refundCalls)
}

func TestBizRefundPartialRequestsAndKeyBinding(t *testing.T) {
	service, gateway, lifecycle, _, _, _, scope, _, payment := refundFixture(t)
	ctx := context.Background()
	gateway.refundStatus = "processed"
	first := bizbilling.RefundPaymentRequest{AmountPaisa: 300, Reason: "part one"}
	_, err := service.Request(ctx, scope.AppID, payment.ID, first, "partial-one")
	require.NoError(t, err)
	_, err = service.Request(ctx, scope.AppID, payment.ID, bizbilling.RefundPaymentRequest{AmountPaisa: 400, Reason: "part two"}, "partial-two")
	require.NoError(t, err)
	_, err = service.Request(ctx, scope.AppID, payment.ID, bizbilling.RefundPaymentRequest{AmountPaisa: 400, Reason: "changed"}, "partial-one")
	require.ErrorIs(t, err, bizbilling.ErrIdempotencyConflict)
	_, err = service.Request(ctx, scope.AppID, payment.ID, bizbilling.RefundPaymentRequest{AmountPaisa: 501}, "over-budget")
	require.Error(t, err)
	saved, err := lifecycle.GetPayment(ctx, scope.AppID, payment.ID)
	require.NoError(t, err)
	require.Equal(t, int64(700), saved.RefundedPaisa)
	require.Zero(t, saved.RefundReservedPaisa)
	require.Equal(t, 2, gateway.refundCalls)
	_, err = service.Request(ctx, scope.AppID, payment.ID, first, "")
	require.Error(t, err)
}

func TestBizRefundRestartAndDisconnectedCallback(t *testing.T) {
	service, gateway, lifecycle, _, merchants, es, scope, _, payment := refundFixture(t)
	ctx := context.Background()
	gateway.loseRefund = true
	gateway.findUnavailable = true
	request := bizbilling.RefundPaymentRequest{AmountPaisa: 1200, Reason: "return"}
	pending, err := service.Request(ctx, scope.AppID, payment.ID, request, "restart-refund")
	require.Error(t, err)
	require.NotEmpty(t, pending.OperationID)
	require.NoError(t, merchants.Disconnect(ctx, scope.AppID, bizbilling.GatewayTest, 1))
	gateway.findUnavailable = false
	gateway.refundStatus = "processed"
	restarted := NewBizRefundService(es, lifecycle, merchants, service.factory, zap.NewNop())
	_, _ = restarted.Reconcile(ctx, 200)
	effect, err := service.effects.Get(ctx, pending.OperationID)
	require.NoError(t, err)
	require.Equal(t, "completed", effect.State)
	ensureBillingIndex(t, es, bizbillingrepo.GatewayEventIndex, (&database.IndexTemplates{}).GetBizGatewayEventsTemplate())
	events := NewBizGatewayEventService(es, merchants, nil, zap.NewNop())
	events.SetRefundProcessor(restarted)
	raw, _ := json.Marshal(map[string]any{"account_id": gateway.ref.AccountID, "event": "refund.processed", "payload": map[string]any{"refund": map[string]any{"entity": map[string]any{"id": effect.Refund.ID, "payment_id": payment.GatewayPaymentID, "amount": 1200, "currency": scope.Currency}}}})
	for _, header := range []string{"refund-hook-one", "refund-hook-two"} {
		receipt, err := events.Accept(ctx, gateway.ref.ConnectionID, header, raw, gatewaySignature(raw, "owned-hook-secret"))
		require.NoError(t, err)
		result, err := events.Process(ctx, receipt.ID)
		require.NoError(t, err)
		require.Equal(t, bizbilling.OperationCompleted, result.State)
	}
	saved, err := lifecycle.GetPayment(ctx, scope.AppID, payment.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1200), saved.RefundedPaisa)
	require.Zero(t, saved.RefundReservedPaisa)
	require.Equal(t, 1, gateway.refundCalls)
	unknown := bytes.ReplaceAll(raw, []byte(effect.Refund.ID), []byte("rfnd_unknown"))
	receipt, err := events.Accept(ctx, gateway.ref.ConnectionID, "unknown-refund", unknown, gatewaySignature(unknown, "owned-hook-secret"))
	require.NoError(t, err)
	_, err = events.Process(ctx, receipt.ID)
	require.Error(t, err)
	stored, err := events.inbox.Get(ctx, receipt.ID)
	require.NoError(t, err)
	require.Equal(t, "unmatched", stored.State)
}

func TestBizRefundFacadeNeverReportsPendingAsRefunded(t *testing.T) {
	service, gateway, lifecycle, _, _, _, scope, _, payment := refundFixture(t)
	ctx := WithBizOperationKey(context.Background(), "facade-refund")
	_, err := lifecycle.RefundPayment(ctx, scope.AppID, payment.ID, &bizbilling.RefundPaymentRequest{AmountPaisa: 500})
	require.ErrorIs(t, err, bizbilling.ErrGatewayPending)
	require.Equal(t, 1, gateway.refundCalls)
	gateway.refundStatus = "processed"
	saved, err := lifecycle.RefundPayment(ctx, scope.AppID, payment.ID, &bizbilling.RefundPaymentRequest{AmountPaisa: 500})
	require.NoError(t, err)
	require.Equal(t, int64(500), saved.RefundedPaisa)
	require.Equal(t, 1, gateway.refundCalls)
	_, err = service.Request(ctx, uuid.NewString(), payment.ID, bizbilling.RefundPaymentRequest{AmountPaisa: 1}, "wrong-app")
	require.Error(t, err)
}

func TestBizRefundAndCreditSpendCannotBothConsumeExcess(t *testing.T) {
	service, gateway, lifecycle, ops, _, es, scope, _, payment := refundFixture(t)
	ctx := context.Background()
	invoice := seedCheckoutInvoice(t, es, scope, 200)
	start := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		_, _ = service.Request(ctx, scope.AppID, payment.ID, bizbilling.RefundPaymentRequest{AmountPaisa: 1200}, "race-refund")
	}()
	go func() {
		defer wait.Done()
		<-start
		_, _ = ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandBalanceApplication, Scope: scope, IdempotencyKey: "race-spend", BalanceApplication: &bizbilling.BalanceApplication{InvoiceID: invoice.ID}})
	}()
	close(start)
	wait.Wait()
	saved, err := lifecycle.GetPayment(ctx, scope.AppID, payment.ID)
	require.NoError(t, err)
	inv, err := lifecycle.GetInvoice(ctx, scope.AppID, invoice.ID)
	require.NoError(t, err)
	balance, err := ops.Balance(ctx, scope)
	require.NoError(t, err)
	require.GreaterOrEqual(t, balance.AvailablePaisa, int64(0))
	require.GreaterOrEqual(t, balance.ReservedPaisa, int64(0))
	require.LessOrEqual(t, saved.RefundReservedPaisa+saved.RefundedPaisa+inv.CreditsApplied, int64(1200))
	if gateway.refundCalls == 1 {
		require.Equal(t, int64(1200), saved.RefundReservedPaisa)
		require.Zero(t, inv.CreditsApplied)
	}
}

func TestBizRefundDurableBoundaryRecovery(t *testing.T) {
	for _, boundary := range []string{"intent", "result", "attempt"} {
		t.Run(boundary, func(t *testing.T) {
			fault := &lostBillingResponse{match: func(r *http.Request, body []byte) bool {
				if !strings.Contains(r.URL.Path, bizbillingrepo.GatewayEffectIndex+"/_doc/") {
					return false
				}
				switch boundary {
				case "intent":
					return bytes.Contains(body, []byte(`"refund_intent":`)) && !bytes.Contains(body, []byte(`"attempted":true`))
				case "result":
					return bytes.Contains(body, []byte(`"kind":"refund"`)) && bytes.Contains(body, []byte(`"state":"completed"`))
				default:
					return bytes.Contains(body, []byte(`"refund_intent":`)) && bytes.Contains(body, []byte(`"attempted":true`))
				}
			}}
			service, gateway, lifecycle, _, merchants, es, scope, _, payment := refundFixture(t, fault)
			ctx := context.Background()
			gateway.refundStatus = "processed"
			fault.armed.Store(true)
			request := bizbilling.RefundPaymentRequest{AmountPaisa: 1200, Reason: "durable"}
			result, err := service.Request(ctx, scope.AppID, payment.ID, request, "durable-refund")
			require.False(t, fault.armed.Load(), "must drop an actual persisted boundary response")
			restarted := NewBizRefundService(es, lifecycle, merchants, service.factory, zap.NewNop())
			if boundary == "attempt" {
				require.ErrorIs(t, err, bizbilling.ErrGatewayPending)
				require.NotEmpty(t, result.OperationID)
				_, _ = restarted.Reconcile(ctx, 200)
				saved, readErr := lifecycle.GetPayment(ctx, scope.AppID, payment.ID)
				require.NoError(t, readErr)
				require.Zero(t, saved.RefundedPaisa)
				require.Equal(t, int64(1200), saved.RefundReservedPaisa)
				require.Zero(t, gateway.refundCalls, "unknown attempt marker cannot safely cause a second POST")
			} else {
				require.NoError(t, err)
				replay, replayErr := restarted.Request(ctx, scope.AppID, payment.ID, request, "durable-refund")
				require.NoError(t, replayErr)
				require.Equal(t, result.OperationID, replay.OperationID)
				require.Equal(t, 1, gateway.refundCalls)
			}
		})
	}
}

func TestBizRefundLegacyBindingRequiresReviewAndCashStillWorks(t *testing.T) {
	_, _, lifecycle, _, _, es, scope, _, _ := refundFixture(t)
	ctx := context.Background()
	invoice := seedCheckoutInvoice(t, es, scope, 1000)
	cash, err := lifecycle.RecordPayment(WithBizOperationKey(ctx, "cash"), scope.AppID, invoice.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: 1000})
	require.NoError(t, err)
	refund, err := lifecycle.RefundPayment(WithBizOperationKey(ctx, "cash-return"), scope.AppID, cash.ID, &bizbilling.RefundPaymentRequest{AmountPaisa: 100})
	require.NoError(t, err)
	require.Equal(t, int64(100), refund.RefundedPaisa)
	legacy := *cash
	legacy.ID = uuid.NewString()
	legacy.Method = "gateway"
	legacy.Gateway = "razorpay"
	legacy.GatewayPaymentID = "pay_legacy"
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizPayment](es, "frn_biz_payments", zap.NewNop()).Create(ctx, legacy.ID, &legacy))
	_, err = lifecycle.RefundPayment(ctx, scope.AppID, legacy.ID, &bizbilling.RefundPaymentRequest{AmountPaisa: 100})
	require.ErrorIs(t, err, bizbilling.ErrLegacyReviewRequired)
}

func TestBizRefundSignedCallbackRequiresProviderConfirmation(t *testing.T) {
	service, gateway, lifecycle, _, merchants, es, scope, _, payment := refundFixture(t)
	ctx := context.Background()
	pending, err := service.Request(ctx, scope.AppID, payment.ID, bizbilling.RefundPaymentRequest{AmountPaisa: 1000}, "callback-refund")
	require.ErrorIs(t, err, bizbilling.ErrGatewayPending)
	effect, err := service.effects.Get(ctx, pending.OperationID)
	require.NoError(t, err)
	ensureBillingIndex(t, es, bizbillingrepo.GatewayEventIndex, (&database.IndexTemplates{}).GetBizGatewayEventsTemplate())
	events := NewBizGatewayEventService(es, merchants, nil, zap.NewNop())
	events.SetRefundProcessor(service)
	raw, _ := json.Marshal(map[string]any{"account_id": gateway.ref.AccountID, "event": "refund.processed", "payload": map[string]any{"refund": map[string]any{"entity": map[string]any{"id": effect.Refund.ID, "payment_id": payment.GatewayPaymentID, "amount": 1000, "currency": scope.Currency}}}})
	receipt, err := events.Accept(ctx, gateway.ref.ConnectionID, "processed-hook", raw, gatewaySignature(raw, "owned-hook-secret"))
	require.NoError(t, err)
	_, err = events.Process(ctx, receipt.ID)
	require.ErrorIs(t, err, bizbilling.ErrGatewayPending, "even a valid signed claim is checked against provider facts")
	held, err := lifecycle.GetPayment(ctx, scope.AppID, payment.ID)
	require.NoError(t, err)
	require.Zero(t, held.RefundedPaisa)
	gateway.refundStatus = "processed"
	for i := 0; i < 2; i++ {
		_, err = events.Process(ctx, receipt.ID)
		require.NoError(t, err)
	}
	saved, err := lifecycle.GetPayment(ctx, scope.AppID, payment.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1000), saved.RefundedPaisa)
	require.Zero(t, saved.RefundReservedPaisa)
	require.Equal(t, 1, gateway.refundCalls)
}

func TestBizRefundAuthenticationRejectionReleasesHold(t *testing.T) {
	service, gateway, lifecycle, ops, _, _, scope, _, payment := refundFixture(t)
	ctx := context.Background()
	gateway.authRejected = true
	for i := 0; i < 2; i++ {
		_, err := service.Request(ctx, scope.AppID, payment.ID, bizbilling.RefundPaymentRequest{AmountPaisa: 1200}, "auth-refund")
		require.Error(t, err)
	}
	saved, err := lifecycle.GetPayment(ctx, scope.AppID, payment.ID)
	require.NoError(t, err)
	require.Zero(t, saved.RefundedPaisa)
	require.Zero(t, saved.RefundReservedPaisa)
	balance, err := ops.Balance(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, int64(200), balance.AvailablePaisa)
	require.Zero(t, balance.ReservedPaisa)
	require.Equal(t, 1, gateway.refundCalls)
}
