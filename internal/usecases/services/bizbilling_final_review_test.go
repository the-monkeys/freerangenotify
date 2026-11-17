package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"go.uber.org/zap"
)

func TestBizReviewDefinitiveModeMismatchDoesNotBlockStream(t *testing.T) {
	code, rejected := financialRejection(bizbilling.ErrModeMismatch)
	require.True(t, rejected, "a deterministic mode mismatch must release the accepted head")
	require.ErrorIs(t, financialRejectedError(code), bizbilling.ErrModeMismatch)
}

type reviewRotatedRecurringFactory struct {
	merchants *BizGatewayConnectionService
	current   *recurringGateway
	retired   *reviewRetiredRecurring
}
type reviewRetiredRecurring struct {
	*recurringGateway
	reads int
}

func (g *reviewRetiredRecurring) FetchSubscription(context.Context, string) (bizbilling.GatewaySubscription, error) {
	g.reads++
	return bizbilling.GatewaySubscription{}, bizbilling.ErrMerchantAuthentication
}
func (f reviewRotatedRecurringFactory) For(ctx context.Context, ref bizbilling.GatewayRef) (bizbilling.BizGateway, error) {
	credentials, err := f.merchants.ResolveAPI(ctx, ref)
	if err != nil {
		return nil, err
	}
	if credentials.KeyID == "rzp_test_rotated" {
		return f.current, nil
	}
	return f.retired, nil
}
func (f reviewRotatedRecurringFactory) ForExact(ctx context.Context, ref bizbilling.GatewayRef) (bizbilling.BizGateway, error) {
	credentials, err := f.merchants.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	if credentials.KeyID == "rzp_test_rotated" {
		return f.current, nil
	}
	return f.retired, nil
}

func reviewRecurringEvent(t *testing.T, service *BizRecurringService, sub *bizbilling.BizSubscription, hook string) *bizbilling.GatewayEventReceipt {
	t.Helper()
	ctx := context.Background()
	ensureBillingIndex(t, service.es, bizbillingrepo.GatewayEventIndex, (&database.IndexTemplates{}).GetBizGatewayEventsTemplate())
	snapshot, err := service.merchants.Snapshot(ctx, sub.AppID, bizbilling.GatewayTest)
	require.NoError(t, err)
	raw, err := json.Marshal(map[string]any{"entity": "event", "account_id": snapshot.AccountID, "event": "subscription.authenticated", "payload": map[string]any{"subscription": map[string]any{"entity": map[string]any{"id": "sub_recurring_fixture"}}}})
	require.NoError(t, err)
	events := NewBizGatewayEventService(service.es, service.merchants, nil, zap.NewNop())
	receipt, err := events.Accept(ctx, snapshot.ConnectionID, "recurring-proof", raw, gatewaySignature(raw, hook))
	require.NoError(t, err)
	return &receipt
}

func TestBizReviewRecurringRotationProvesNewKeyBeforeRetiredRead(t *testing.T) {
	service, gateway, lifecycle, merchants, sub := recurringFixture(t, bizbilling.CollectionAutomatic)
	ctx := context.Background()
	_, err := service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "original-authorize")
	require.NoError(t, err)
	key, secret, hook := "rzp_test_rotated", "rotated-fixture-api", "rotated-fixture-hook"
	_, err = merchants.Save(ctx, sub.AppID, bizbilling.GatewayTest, 1, bizbilling.MerchantPatch{KeyID: &key, KeySecret: &secret, WebhookSecret: &hook})
	require.NoError(t, err)
	gateway.remote.Status = "authenticated"
	retired := &reviewRetiredRecurring{recurringGateway: gateway}
	service.factory = reviewRotatedRecurringFactory{merchants, gateway, retired}
	receipt := reviewRecurringEvent(t, service, sub, hook)
	result, err := service.ProcessEvent(ctx, receipt)
	require.NoError(t, err)
	require.Equal(t, bizbilling.OperationCompleted, result.State)
	snapshot, err := merchants.Snapshot(ctx, sub.AppID, bizbilling.GatewayTest)
	require.NoError(t, err)
	require.True(t, snapshot.AccountVerified)
	stored, err := lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, stored.GatewayBinding.CredentialRevision, "original mandate binding remains immutable")
	require.Zero(t, retired.reads, "proof of a replacement key must precede old-key API reads")
	require.Equal(t, 1, gateway.creates)
}

func TestBizReviewRecurringReceiptsRemainListable(t *testing.T) {
	service, _, _, _, sub := recurringFixture(t, bizbilling.CollectionAutomatic)
	receipt := reviewRecurringEvent(t, service, sub, "owned-hook-secret")
	require.Equal(t, 2, receipt.SchemaVersion)
	events := NewBizGatewayEventService(service.es, service.merchants, nil, zap.NewNop())
	rows, err := events.ListForApp(context.Background(), sub.AppID, "", "", 20)
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func TestBizReviewInvoiceResendIsDurableScopedAndIdempotent(t *testing.T) {
	for _, mode := range []bizbilling.GatewayMode{bizbilling.GatewayLive, bizbilling.GatewayTest} {
		t.Run(string(mode), func(t *testing.T) {
			outbox, sender, lifecycle, _, scope := outboxFixture(t)
			ctx := context.Background()
			draft, err := lifecycle.CreateOneTimeInvoice(WithBizOperationKey(ctx, "resend-draft"), scope.AppID, &bizbilling.CreateInvoiceRequest{UserID: scope.UserID, Currency: "INR", GatewayMode: mode, LineItems: []bizbilling.InvoiceLineItem{{Description: "Service", Quantity: 1, UnitPrice: 1000}}})
			require.NoError(t, err)
			invoice, err := lifecycle.IssueInvoice(WithBizOperationKey(ctx, "resend-issue"), scope.AppID, draft.ID)
			require.NoError(t, err)
			for i := 0; i < 2; i++ {
				_, err = lifecycle.SendInvoice(WithBizOperationKey(ctx, "same-resend"), scope.AppID, invoice.ID)
				require.NoError(t, err)
			}
			rows, err := outbox.List(ctx, scope.AppID, 20)
			require.NoError(t, err)
			require.Len(t, rows, 2, "issue and one deliberate resend, never a duplicate replay")
			_, err = outbox.DispatchDueForApp(ctx, scope.AppID, 20)
			require.NoError(t, err)
			if mode == bizbilling.GatewayTest {
				require.Zero(t, sender.calls)
			} else {
				require.Equal(t, 2, sender.calls)
			}
			stored, err := lifecycle.GetInvoice(ctx, scope.AppID, invoice.ID)
			require.NoError(t, err)
			require.Equal(t, invoice.TotalPaisa, stored.TotalPaisa)
			require.Zero(t, stored.AmountPaidPaisa)
		})
	}
}

func TestBizReviewOverdueScanReachesInvoicesBeyondFirstPage(t *testing.T) {
	lifecycle, _, _, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	_, err := lifecycle.configSvc.UpdateConfig(ctx, scope.AppID, bizbilling.ConfigTypeDunning, map[string]any{"enabled": false})
	require.NoError(t, err)
	due := time.Now().UTC().Add(-time.Hour)
	var bulk bytes.Buffer
	ids := []string{}
	for i := 0; i < 205; i++ {
		id := fmt.Sprintf("%s-review-overdue-%03d", scope.AppID, i)
		ids = append(ids, id)
		header, _ := json.Marshal(map[string]any{"index": map[string]any{"_index": "frn_biz_invoices", "_id": id}})
		source, _ := json.Marshal(bizbilling.BizInvoice{ID: id, AppID: scope.AppID, UserID: scope.UserID, Currency: "INR", Status: bizbilling.InvoiceStatusOpen, DueDate: &due, TotalPaisa: 100, AmountDuePaisa: 100})
		bulk.Write(header)
		bulk.WriteByte('\n')
		bulk.Write(source)
		bulk.WriteByte('\n')
	}
	response, err := es.Bulk(bytes.NewReader(bulk.Bytes()), es.Bulk.WithRefresh("true"))
	require.NoError(t, err)
	defer response.Body.Close()
	require.False(t, response.IsError())
	scheduler := &BizBillingScheduler{lifecycleSvc: lifecycle, configSvc: lifecycle.configSvc, stores: lifecycle.stores, logger: zap.NewNop()}
	scheduler.processOverdueInvoices(ctx)
	for _, id := range ids {
		invoice, err := lifecycle.GetInvoice(ctx, scope.AppID, id)
		require.NoError(t, err)
		require.Equal(t, bizbilling.InvoiceStatusOverdue, invoice.Status, id)
	}
}

type reviewUnpaidOrderGateway struct {
	*checkoutGateway
	unpaid    bool
	lookupErr error
}

func (g *reviewUnpaidOrderGateway) OrderIsUnpaid(context.Context, bizbilling.GatewayOrder) (bool, error) {
	return g.unpaid, g.lookupErr
}

func TestBizReviewChangedBalanceReconcilesObsoleteCheckout(t *testing.T) {
	checkout, _, gateway, lifecycle, _, es, scope := checkoutFixture(t)
	ctx := context.Background()
	safety := &reviewUnpaidOrderGateway{checkoutGateway: gateway, unpaid: true}
	checkout.factory = bizFactoryFunc(func(context.Context, bizbilling.GatewayRef) (bizbilling.BizGateway, error) { return safety, nil })
	invoice := seedCheckoutInvoice(t, es, scope, 1000)
	old, err := checkout.Start(ctx, scope.AppID, scope.UserID, invoice.ID, bizbilling.GatewayTest, "abandoned")
	require.NoError(t, err)
	_, err = lifecycle.RecordPayment(WithBizOperationKey(ctx, "bank"), scope.AppID, invoice.ID, &bizbilling.RecordPaymentRequest{Method: "bank_transfer", AmountPaisa: 400})
	require.NoError(t, err)
	next, err := checkout.Start(ctx, scope.AppID, scope.UserID, invoice.ID, bizbilling.GatewayTest, "remaining-balance")
	require.NoError(t, err)
	require.EqualValues(t, 600, next.AmountPaisa)
	require.NotEqual(t, old.OrderID, next.OrderID)
	again, err := checkout.Start(ctx, scope.AppID, scope.UserID, invoice.ID, bizbilling.GatewayTest, "remaining-balance")
	require.NoError(t, err)
	require.Equal(t, next.OrderID, again.OrderID)
	require.Equal(t, 2, gateway.creates)
	// A late, real capture on the retained old binding must still be accounted
	// for once, including source-tracked overpayment rather than losing money.
	gateway.payment = bizbilling.CapturedPayment{ID: "late-original", OrderID: old.OrderID, Gateway: gateway.ref, AmountPaisa: 1000, Currency: "INR", Captured: true, Status: "captured"}
	proof := bizbilling.CheckoutProof{OrderID: old.OrderID, PaymentID: "late-original", Signature: "fixture-valid"}
	paid, err := checkout.Verify(ctx, scope.AppID, scope.UserID, invoice.ID, proof)
	require.NoError(t, err)
	replay, err := checkout.Verify(ctx, scope.AppID, scope.UserID, invoice.ID, proof)
	require.NoError(t, err)
	require.Equal(t, paid.OperationID, replay.OperationID)
	stored, err := lifecycle.GetInvoice(ctx, scope.AppID, invoice.ID)
	require.NoError(t, err)
	require.Zero(t, stored.AmountDuePaisa)
}

func TestBizReviewUnknownOldPaymentNeverStartsReplacementOrder(t *testing.T) {
	checkout, _, gateway, lifecycle, _, es, scope := checkoutFixture(t)
	ctx := context.Background()
	safety := &reviewUnpaidOrderGateway{checkoutGateway: gateway, unpaid: false}
	checkout.factory = bizFactoryFunc(func(context.Context, bizbilling.GatewayRef) (bizbilling.BizGateway, error) { return safety, nil })
	invoice := seedCheckoutInvoice(t, es, scope, 1000)
	_, err := checkout.Start(ctx, scope.AppID, scope.UserID, invoice.ID, bizbilling.GatewayTest, "original")
	require.NoError(t, err)
	_, err = lifecycle.RecordPayment(ctx, scope.AppID, invoice.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: 400})
	require.NoError(t, err)
	_, err = checkout.Start(ctx, scope.AppID, scope.UserID, invoice.ID, bizbilling.GatewayTest, "replacement")
	require.Error(t, err)
	require.Equal(t, 1, gateway.creates)
}

func TestBizReviewCreditNoteKeepsReferencedSandboxScope(t *testing.T) {
	_, credit, ops, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	scope.Mode = bizbilling.GatewayTest
	invoice := seedCheckoutInvoice(t, es, scope, 1000)
	note, err := credit.CreateCreditNote(WithBizOperationKey(ctx, "sandbox-note"), scope.AppID, &bizbilling.CreateCreditNoteRequest{UserID: scope.UserID, InvoiceID: invoice.ID, Currency: "INR", AmountPaisa: 100, Reason: "Test credit"})
	require.NoError(t, err)
	require.Equal(t, bizbilling.GatewayTest, note.GatewayMode)
	live := scope
	live.Mode = "" // Live streams retain the historical omitted-mode identity.
	_, err = ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandOpeningBalance, Scope: live, IdempotencyKey: "live-after-test", OpeningBalance: &bizbilling.OpeningBalance{AvailablePaisa: 10}})
	require.NoError(t, err, "sandbox credit must not strand the live customer stream")
}

func TestBizReviewSandboxSubscriptionsDoNotInflateReports(t *testing.T) {
	lifecycle, _, _, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	templates := &database.IndexTemplates{}
	ensureBillingIndex(t, es, "frn_biz_plans", templates.GetBizPlansTemplate())
	ensureBillingIndex(t, es, "frn_biz_subscriptions", templates.GetBizSubscriptionsTemplate())
	plan := &bizbilling.Plan{ID: uuid.NewString(), AppID: scope.AppID, AmountPaisa: 1000, Currency: "INR", PricingModel: bizbilling.PricingModelFlat, BillingCycle: bizbilling.BillingCycleMonthly, Active: true}
	require.NoError(t, lifecycle.stores.Plans.Create(ctx, plan.ID, plan))
	for _, mode := range []bizbilling.GatewayMode{bizbilling.GatewayLive, bizbilling.GatewayTest} {
		sub := &bizbilling.BizSubscription{ID: uuid.NewString(), AppID: scope.AppID, UserID: scope.UserID, PlanID: plan.ID, Quantity: 1, Currency: "INR", Status: bizbilling.SubscriptionStatusActive, GatewayMode: mode}
		require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizSubscription](es, "frn_biz_subscriptions", zap.NewNop()).Create(ctx, sub.ID, sub))
	}
	now := time.Now().UTC()
	canceled := &bizbilling.BizSubscription{ID: uuid.NewString(), AppID: scope.AppID, UserID: scope.UserID, PlanID: plan.ID, Currency: "INR", Status: bizbilling.SubscriptionStatusCanceled, GatewayMode: bizbilling.GatewayTest, CanceledAt: &now}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizSubscription](es, "frn_biz_subscriptions", zap.NewNop()).Create(ctx, canceled.ID, canceled))
	require.NoError(t, lifecycle.stores.Plans.Refresh(ctx))
	require.NoError(t, lifecycle.stores.Subscriptions.Refresh(ctx))
	analytics := &BizAnalyticsService{stores: lifecycle.stores, logger: zap.NewNop()}
	revenue := map[string]interface{}{}
	require.NoError(t, analytics.financialSubscriptionMetrics(ctx, scope.AppID, "INR", "Revenue", revenue))
	require.EqualValues(t, 1000, revenue["mrr_paisa"])
	stats := map[string]interface{}{}
	require.NoError(t, analytics.financialSubscriptionMetrics(ctx, scope.AppID, "INR", "Subscriptions", stats))
	require.EqualValues(t, 0, stats["canceled_last_30d"])
	require.EqualValues(t, 1, stats["by_status"].(map[string]int64)["active"])
}
