package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"go.uber.org/zap"
)

type periodGateway struct {
	*recurringGateway
	invoices []bizbilling.GatewayRecurringInvoice
}

func providerPeriodFixture(t *testing.T) (*BizRecurringService, *periodGateway, *BizLifecycleService, *bizbilling.BizSubscription) {
	t.Helper()
	recurring, original, lifecycle, _, sub := recurringFixture(t, bizbilling.CollectionAutomatic)
	ensureBillingIndex(t, recurring.es, bizbillingrepo.BillingPeriodIndex, (&database.IndexTemplates{}).GetBizBillingPeriodsTemplate())
	lifecycle.SetPeriodService(NewBizPeriodService(recurring.es, lifecycle, recurring, zap.NewNop()))
	gateway := &periodGateway{recurringGateway: original}
	recurring.factory = bizFactoryFunc(func(context.Context, bizbilling.GatewayRef) (bizbilling.BizGateway, error) { return gateway, nil })
	return recurring, gateway, lifecycle, sub
}

func (g *periodGateway) FetchRecurringInvoice(_ context.Context, id string) (bizbilling.GatewayRecurringInvoice, error) {
	for _, invoice := range g.invoices {
		if invoice.ID == id {
			return invoice, nil
		}
	}
	return bizbilling.GatewayRecurringInvoice{}, bizbilling.ErrGatewayPending
}
func (g *periodGateway) ListRecurringInvoices(_ context.Context, sub string, offset, limit int) ([]bizbilling.GatewayRecurringInvoice, error) {
	if offset >= len(g.invoices) {
		return nil, nil
	}
	return g.invoices[offset:min(offset+limit, len(g.invoices))], nil
}

func TestBizProviderInvoiceInitialAutomaticPeriod(t *testing.T) {
	recurring, original, lifecycle, _, seed := recurringFixture(t, bizbilling.CollectionAutomatic)
	ctx := context.Background()
	ensureBillingIndex(t, recurring.es, bizbillingrepo.BillingPeriodIndex, (&database.IndexTemplates{}).GetBizBillingPeriodsTemplate())
	periods := NewBizPeriodService(recurring.es, lifecycle, recurring, zap.NewNop())
	lifecycle.SetPeriodService(periods)
	gateway := &periodGateway{recurringGateway: original}
	recurring.factory = bizFactoryFunc(func(context.Context, bizbilling.GatewayRef) (bizbilling.BizGateway, error) { return gateway, nil })
	sub, err := lifecycle.CreateSubscription(WithBizOperationKey(ctx, "new-auto"), seed.AppID, &bizbilling.CreateSubscriptionRequest{UserID: seed.UserID, PlanID: seed.PlanID, Quantity: 1, CollectionMode: bizbilling.CollectionAutomatic, AutomaticCycles: 12, GatewayMode: bizbilling.GatewayTest})
	require.NoError(t, err)
	require.Equal(t, bizbilling.SubscriptionStatusAwaitingAuthorization, sub.Status)
	require.True(t, sub.CurrentPeriodEnd.IsZero())
	_, total, err := lifecycle.ListInvoices(ctx, bizbilling.InvoiceFilter{AppID: seed.AppID, SubscriptionID: sub.ID})
	require.NoError(t, err)
	require.Zero(t, total, "no manual invoice or free first cycle before authorization")
	// A later catalogue edit must not reprice this accepted subscription.
	_, err = lifecycle.stores.Plans.Update(ctx, sub.PlanID, func(p *bizbilling.Plan) error { p.AmountPaisa = 2000; return nil })
	require.NoError(t, err)
	_, err = recurring.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "authorize")
	require.NoError(t, err)
	start := time.Now().UTC().Truncate(time.Second)
	end, err := bizbilling.NextAnchoredPeriod(start, bizbilling.BillingCycleMonthly, 0, start.Day())
	require.NoError(t, err)
	gateway.remote.Status, gateway.remote.CurrentStart, gateway.remote.CurrentEnd = "active", start, end
	invoice := bizbilling.GatewayRecurringInvoice{ID: "inv_initial", SubscriptionID: gateway.remote.ID, Gateway: gateway.ref, OrderID: "order_initial", PaymentID: "pay_initial", Status: "paid", AmountPaisa: 1180, PaidPaisa: 1180, Currency: "INR", Start: start, End: end}
	gateway.invoices = []bizbilling.GatewayRecurringInvoice{invoice}
	gateway.payment = bizbilling.CapturedPayment{ID: invoice.PaymentID, OrderID: invoice.OrderID, Gateway: gateway.ref, AmountPaisa: 1180, Currency: "INR", Status: "captured", Captured: true}
	for i := 0; i < 2; i++ {
		_, err = recurring.SyncProviderSubscription(ctx, sub.AppID, sub.ID)
		require.NoError(t, err)
	}
	rows, total, err := lifecycle.ListInvoices(ctx, bizbilling.InvoiceFilter{AppID: sub.AppID, SubscriptionID: sub.ID})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, int64(1180), rows[0].AmountPaidPaisa)
	require.Zero(t, rows[0].AmountDuePaisa)
	_, total, err = lifecycle.ListPayments(ctx, bizbilling.PaymentFilter{AppID: sub.AppID, UserID: sub.UserID})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, 1, gateway.creates, "sync cannot create a second provider subscription")
	require.Zero(t, gateway.checkoutGateway.creates, "provider owns the charge; never mint a checkout order")
	stored, err := lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.True(t, stored.CurrentPeriodStart.Equal(start))
	require.True(t, stored.CurrentPeriodEnd.Equal(end))
	require.False(t, stored.InitialAutomaticPeriod)
}

func TestBizProviderInvoiceNativePauseWaitsForConfirmation(t *testing.T) {
	recurring, gateway, lifecycle, _, sub := recurringFixture(t, bizbilling.CollectionAutomatic)
	lifecycle.SetPeriodService(NewBizPeriodService(recurring.es, lifecycle, recurring, zap.NewNop()))
	ctx := context.Background()
	_, err := recurring.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "authorize")
	require.NoError(t, err)
	gateway.remote.Status = "active"
	gateway.cancelPending = true
	_, err = lifecycle.PauseSubscription(WithBizOperationKey(ctx, "pause"), sub.AppID, sub.ID)
	require.ErrorIs(t, err, bizbilling.ErrGatewayPending)
	require.Equal(t, 1, gateway.changes, "one native request must precede the pending result")
	_, err = recurring.ChangeCollection(ctx, sub.AppID, sub.ID, bizbilling.CollectionInvoice, "switch-while-native-pending")
	require.ErrorIs(t, err, bizbilling.ErrAutomaticCollectionInProgress, "a concurrent mode switch must wait for the native transition")
	require.Equal(t, 1, gateway.changes)
	stored, err := lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.SubscriptionStatusActive, stored.Status, "an uncertain provider outcome must not claim a local pause")
	// The provider completed the original request but its response was lost.
	gateway.cancelPending, gateway.remote.Status = false, "paused"
	paused, err := lifecycle.PauseSubscription(WithBizOperationKey(ctx, "pause"), sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.SubscriptionStatusPaused, paused.Status)
	require.Equal(t, 1, gateway.changes, "unknown outcomes are looked up, never blindly retried")
	resumed, err := lifecycle.ResumeSubscription(WithBizOperationKey(ctx, "resume"), sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.SubscriptionStatusActive, resumed.Status)
	require.Equal(t, "active", gateway.remote.Status)
	require.Equal(t, 2, gateway.changes)
}

func TestBizProviderInvoiceManualAfterConfirmedStop(t *testing.T) {
	recurring, gateway, lifecycle, sub := providerPeriodFixture(t)
	ctx := context.Background()
	_, err := recurring.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "authorize")
	require.NoError(t, err)
	gateway.remote.Status = "active"
	start := sub.CurrentPeriodEnd.Truncate(time.Second).Add(time.Second)
	end, err := bizbilling.NextAnchoredPeriod(start, bizbilling.BillingCycleMonthly, 0, start.Day())
	require.NoError(t, err)
	gateway.invoices = []bizbilling.GatewayRecurringInvoice{{ID: "inv_manual_after_stop", SubscriptionID: gateway.remote.ID, Gateway: gateway.ref, OrderID: "order_manual_after_stop", Status: "issued", AmountPaisa: 1180, DuePaisa: 1180, Currency: "INR", Start: start, End: end}}
	_, err = recurring.SyncProviderSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	rows, total, err := lifecycle.ListInvoices(ctx, bizbilling.InvoiceFilter{AppID: sub.AppID, SubscriptionID: sub.ID})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	request := &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: 1180, Reference: "STAFF-CONFIRMED"}
	_, err = lifecycle.RecordPayment(WithBizOperationKey(ctx, "blocked-cash"), sub.AppID, rows[0].ID, request)
	require.ErrorIs(t, err, bizbilling.ErrAutomaticCollectionInProgress)
	_, err = recurring.ChangeCollection(ctx, sub.AppID, sub.ID, bizbilling.CollectionInvoice, "stop-auto")
	require.NoError(t, err)
	_, err = lifecycle.RecordPayment(WithBizOperationKey(ctx, "cash-after-stop"), sub.AppID, rows[0].ID, request)
	require.NoError(t, err, "confirmed provider cancellation must release the old invoice for cash or UPI collection")
	paid, err := lifecycle.GetInvoice(ctx, sub.AppID, rows[0].ID)
	require.NoError(t, err)
	require.Equal(t, int64(1180), paid.AmountPaidPaisa)
	require.Zero(t, paid.AmountDuePaisa)
	require.Equal(t, 1, gateway.changes)
}

func TestBizProviderInvoiceSwitchBeforeInitialAuthorization(t *testing.T) {
	recurring, gateway, lifecycle, seed := providerPeriodFixture(t)
	ctx := context.Background()
	sub, err := lifecycle.CreateSubscription(WithBizOperationKey(ctx, "new-auto"), seed.AppID, &bizbilling.CreateSubscriptionRequest{UserID: seed.UserID, PlanID: seed.PlanID, Quantity: 1, CollectionMode: bizbilling.CollectionAutomatic, AutomaticCycles: 12, GatewayMode: bizbilling.GatewayTest})
	require.NoError(t, err)
	first, err := recurring.ChangeCollection(ctx, sub.AppID, sub.ID, bizbilling.CollectionInvoice, "choose-cash")
	require.NoError(t, err)
	retry, err := recurring.ChangeCollection(ctx, sub.AppID, sub.ID, bizbilling.CollectionInvoice, "choose-cash")
	require.NoError(t, err)
	require.Equal(t, first.OperationID, retry.OperationID)
	stored, err := lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.SubscriptionStatusActive, stored.Status)
	require.False(t, stored.InitialAutomaticPeriod)
	require.False(t, stored.CurrentPeriodEnd.IsZero())
	rows, total, err := lifecycle.ListInvoices(ctx, bizbilling.InvoiceFilter{AppID: sub.AppID, SubscriptionID: sub.ID})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, int64(1180), rows[0].AmountDuePaisa)
	require.Zero(t, rows[0].AmountPaidPaisa)
	require.Zero(t, gateway.creates)
}

func TestBizProviderInvoiceCancelUnstartedDoesNotBill(t *testing.T) {
	_, gateway, lifecycle, seed := providerPeriodFixture(t)
	ctx := context.Background()
	sub, err := lifecycle.CreateSubscription(WithBizOperationKey(ctx, "new-auto"), seed.AppID, &bizbilling.CreateSubscriptionRequest{UserID: seed.UserID, PlanID: seed.PlanID, Quantity: 1, CollectionMode: bizbilling.CollectionAutomatic, AutomaticCycles: 12, GatewayMode: bizbilling.GatewayTest})
	require.NoError(t, err)
	stopped, err := lifecycle.CancelSubscription(WithBizOperationKey(ctx, "cancel-unstarted"), sub.AppID, sub.ID, false)
	require.NoError(t, err)
	require.Equal(t, bizbilling.SubscriptionStatusCanceled, stopped.Status)
	_, total, err := lifecycle.ListInvoices(ctx, bizbilling.InvoiceFilter{AppID: sub.AppID, SubscriptionID: sub.ID})
	require.NoError(t, err)
	require.Zero(t, total, "cancelling an unstarted subscription must never issue a bill")
	require.Zero(t, gateway.creates)
}

func TestBizProviderInvoiceNativeTerminalOutcomeDoesNotStayPending(t *testing.T) {
	recurring, gateway, lifecycle, sub := providerPeriodFixture(t)
	ctx := context.Background()
	_, err := recurring.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "authorize")
	require.NoError(t, err)
	gateway.remote.Status = "active"
	gateway.cancelPending = true
	_, err = lifecycle.PauseSubscription(WithBizOperationKey(ctx, "pause"), sub.AppID, sub.ID)
	require.ErrorIs(t, err, bizbilling.ErrGatewayPending)
	// The customer ended the mandate independently while the pause response
	// was uncertain. A terminal remote result must release the pending fence.
	gateway.cancelPending, gateway.remote.Status = false, "cancelled"
	_, err = lifecycle.PauseSubscription(WithBizOperationKey(ctx, "pause"), sub.AppID, sub.ID)
	require.ErrorIs(t, err, bizbilling.ErrMerchantUnavailable)
	stored, err := lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Empty(t, stored.NativeTransitionID)
	require.Equal(t, bizbilling.CollectionInvoice, stored.CollectionMode)
	require.Equal(t, "cancelled", stored.CollectionState)
	require.Equal(t, 1, gateway.changes, "never re-submit an uncertain native request")
}

func TestBizProviderInvoicePauseBeforeMandateStartsHasNoFence(t *testing.T) {
	recurring, gateway, lifecycle, sub := providerPeriodFixture(t)
	ctx := context.Background()
	_, err := recurring.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "authorize")
	require.NoError(t, err)
	// Razorpay cannot pause a mandate that has not started. Reject before
	// claiming a native transition, so invoice collection remains available.
	gateway.cancelPending = true
	_, err = lifecycle.PauseSubscription(WithBizOperationKey(ctx, "pause-created"), sub.AppID, sub.ID)
	require.Error(t, err)
	require.Zero(t, gateway.changes)
	stored, err := lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Empty(t, stored.NativeTransitionID)
	gateway.cancelPending = false
	_, err = recurring.ChangeCollection(ctx, sub.AppID, sub.ID, bizbilling.CollectionInvoice, "choose-invoice")
	require.NoError(t, err)
}
