package handlers

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

func TestBizGatewayTestCreditCannotFundLiveInvoice(t *testing.T) {
	f := newPortalHTTPFixture(t)
	ctx := context.Background()
	logger := zap.NewNop()
	seed := func(mode bizbilling.GatewayMode, total int64) *bizbilling.BizInvoice {
		invoice := &bizbilling.BizInvoice{ID: uuid.NewString(), AppID: f.appID, UserID: f.userID, Currency: "INR", GatewayMode: mode, Status: bizbilling.InvoiceStatusOpen, InvoiceNumber: "MODE-FIXTURE", SubtotalPaisa: total, TotalPaisa: total, AmountDuePaisa: total}
		require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizInvoice](f.es, "frn_biz_invoices", logger).Create(ctx, invoice.ID, invoice))
		return invoice
	}
	testInvoice := seed(bizbilling.GatewayTest, 1000)
	order, err := f.checkout.Start(ctx, f.appID, f.userID, testInvoice.ID, bizbilling.GatewayTest, "test-capture")
	require.NoError(t, err)
	_, err = f.lifecycle.RecordPayment(ctx, f.appID, testInvoice.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: 1000})
	require.NoError(t, err)
	f.gateway.captured = true
	_, err = f.checkout.Verify(ctx, f.appID, f.userID, testInvoice.ID, bizbilling.CheckoutProof{OrderID: order.OrderID, PaymentID: "pay_fixture", Signature: "fixture-valid"})
	require.NoError(t, err)
	liveScope := bizbilling.Scope{AppID: f.appID, UserID: f.userID, Currency: "INR"}
	liveBalance, err := f.operations.Balance(ctx, liveScope)
	require.NoError(t, err)
	require.Zero(t, liveBalance.AvailablePaisa, "fake test-mode money must not enter the live customer wallet")
	testBalance, err := f.operations.Balance(ctx, testInvoice.FinancialScope())
	require.NoError(t, err)
	require.Equal(t, int64(1000), testBalance.AvailablePaisa)
	liveInvoice := seed(bizbilling.GatewayLive, 1000)
	_, err = f.operations.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandBalanceApplication, Scope: liveScope, IdempotencyKey: "cannot-spend-test-on-live", BalanceApplication: &bizbilling.BalanceApplication{InvoiceID: liveInvoice.ID}})
	require.Error(t, err)
	_, err = f.operations.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandBalanceAdjustment, Scope: liveScope, IdempotencyKey: "real-credit", BalanceAdjustment: &bizbilling.BalanceAdjustment{DeltaPaisa: 400, Reason: "real customer credit"}})
	require.NoError(t, err)
	_, err = f.operations.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandBalanceApplication, Scope: liveScope, IdempotencyKey: "spend-real-credit", BalanceApplication: &bizbilling.BalanceApplication{InvoiceID: liveInvoice.ID}})
	require.NoError(t, err)
	otherTest := seed(bizbilling.GatewayTest, 900)
	_, err = f.operations.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandBalanceApplication, Scope: otherTest.FinancialScope(), IdempotencyKey: "spend-only-test-credit", BalanceApplication: &bizbilling.BalanceApplication{InvoiceID: otherTest.ID}})
	require.NoError(t, err)
	testBalance, err = f.operations.Balance(ctx, testInvoice.FinancialScope())
	require.NoError(t, err)
	require.Equal(t, int64(100), testBalance.AvailablePaisa)
	stores := bizbillingrepo.NewStores(f.es, logger)
	credit := services.NewBizCreditService(stores, services.NewBizConfigService(bizbillingrepo.NewConfigRepo(f.es, logger), logger), nil, nil, logger)
	credit.SetOperationService(f.operations)
	credit.SetLifecycleGate(services.NewBizLifecycleGate(bizbillingrepo.NewLifecycleStateRepo(f.es)))
	statement, err := credit.StatementInCurrency(ctx, f.appID, f.userID, "INR")
	require.NoError(t, err)
	require.Equal(t, int64(1000), statement.TotalBilled)
	require.Zero(t, statement.TotalPaid)
	require.Equal(t, int64(600), statement.BalanceOwed)
}

func TestBizGatewayTestInvoiceDoesNotConsumeLiveNumber(t *testing.T) {
	f := newPortalHTTPFixture(t)
	ctx := context.Background()
	require.NoError(t, bizbillingrepo.NewStore[user.User](f.es, "users", zap.NewNop()).Create(ctx, f.userID, &user.User{UserID: f.userID, AppID: f.appID, Email: "numbering@example.invalid"}))
	request := bizbilling.CreateInvoiceRequest{UserID: f.userID, Currency: "INR", GatewayMode: bizbilling.GatewayTest, LineItems: []bizbilling.InvoiceLineItem{{Description: "Service", Quantity: 1, UnitPrice: 1000}}}
	first, err := f.lifecycle.CreateOneTimeInvoice(services.WithBizOperationKey(ctx, "test-one"), f.appID, &request)
	require.NoError(t, err)
	require.Contains(t, first.InvoiceNumber, "TEST-")
	second, err := f.lifecycle.CreateOneTimeInvoice(services.WithBizOperationKey(ctx, "test-two"), f.appID, &request)
	require.NoError(t, err)
	require.NotEqual(t, first.InvoiceNumber, second.InvoiceNumber)
	request.GatewayMode = bizbilling.GatewayLive
	live, err := f.lifecycle.CreateOneTimeInvoice(services.WithBizOperationKey(ctx, "live-one"), f.appID, &request)
	require.NoError(t, err)
	require.Equal(t, "INV-000001", live.InvoiceNumber)
}
