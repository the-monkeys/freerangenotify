package services

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"go.uber.org/zap"
)

func TestBizRenewalStatusTracksOutstandingDebt(t *testing.T) {
	service, lifecycle, ops, _, sub := periodFixture(t)
	ctx := context.Background()
	start, end := renewalBounds(t, sub)
	_, err := service.EnsurePeriod(ctx, sub.AppID, sub.ID, start, end)
	require.NoError(t, err)
	rows, total, err := lifecycle.ListInvoices(ctx, bizbilling.InvoiceFilter{AppID: sub.AppID, SubscriptionID: sub.ID})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	first := rows[0]
	_, err = lifecycle.MarkOverdue(WithBizOperationKey(ctx, "first-overdue"), first)
	require.NoError(t, err)
	stored, err := lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.SubscriptionStatusPastDue, stored.Status)
	secondResult, err := ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandInvoiceDraft, Scope: sub.FinancialScope(), IdempotencyKey: "another-invoice", InvoiceDraft: &bizbilling.InvoiceDraft{SubscriptionID: sub.ID, InvoiceType: bizbilling.InvoiceTypeRecurring, Request: bizbilling.CreateInvoiceRequest{UserID: sub.UserID, Currency: sub.Currency, GatewayMode: sub.GatewayMode, LineItems: []bizbilling.InvoiceLineItem{{Description: "Additional service", Quantity: 1, UnitPrice: 1000}}}}})
	require.NoError(t, err)
	second, err := lifecycle.IssueInvoice(WithBizOperationKey(ctx, "issue-second"), sub.AppID, operationResource(t, secondResult, "invoice"))
	require.NoError(t, err)
	_, err = lifecycle.MarkOverdue(WithBizOperationKey(ctx, "second-overdue"), second)
	require.NoError(t, err)
	_, err = lifecycle.RecordPayment(WithBizOperationKey(ctx, "pay-first"), sub.AppID, first.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: first.AmountDuePaisa})
	require.NoError(t, err)
	stored, err = lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.SubscriptionStatusPastDue, stored.Status, "other overdue debt must still keep the subscription past due")
	_, err = lifecycle.RecordPayment(WithBizOperationKey(ctx, "pay-second"), sub.AppID, second.ID, &bizbilling.RecordPaymentRequest{Method: "upi", AmountPaisa: second.AmountDuePaisa, Reference: "STAFF-CONFIRMED"})
	require.NoError(t, err)
	stored, err = lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.SubscriptionStatusActive, stored.Status)
}

func TestBizRenewalSubscriptionListHidesPartialRevision(t *testing.T) {
	fault := &lostBillingResponse{match: func(r *http.Request, body []byte) bool {
		return r.Method == http.MethodPut && strings.Contains(r.URL.Path, "frn_biz_subscriptions/") && bytes.Contains(body, []byte(`"status":"paused"`))
	}}
	_, lifecycle, ops, es, sub := periodFixture(t, fault)
	lifecycle.planSvc = NewBizPlanService(bizbillingrepo.NewPlanRepo(es, zap.NewNop()), nil, nil, zap.NewNop())
	ctx := context.Background()
	fault.armed.Store(true)
	_, err := lifecycle.PauseSubscription(WithBizOperationKey(ctx, "pause"), sub.AppID, sub.ID)
	require.Error(t, err)
	require.False(t, fault.armed.Load())
	var failure *bizbilling.OperationFailure
	require.ErrorAs(t, err, &failure)
	require.NoError(t, lifecycle.stores.Subscriptions.Refresh(ctx))
	_, _, err = lifecycle.ListSubscriptions(ctx, bizbilling.SubscriptionFilter{AppID: sub.AppID})
	require.ErrorIs(t, err, bizbilling.ErrReconciliationRequired, "a projected status ahead of the committed money stream is not safe to show")
	expireOperationLease(t, es, sub.FinancialScope())
	_, err = ops.Resume(ctx, sub.AppID, failure.Result.OperationID)
	require.NoError(t, err)
	rows, total, err := lifecycle.ListSubscriptions(ctx, bizbilling.SubscriptionFilter{AppID: sub.AppID})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, bizbilling.SubscriptionStatusPaused, rows[0].Status)
}

func TestBizLegacyChangeTimingRequiresReview(t *testing.T) {
	_, lifecycle, ops, _, sub := periodFixture(t)
	legacy := bizbilling.DefaultLifecycleState(sub.AppID)
	gate := NewBizLifecycleGate(&gateMemoryStore{states: map[string]bizbilling.LifecycleState{sub.AppID: legacy}})
	ops.SetLifecycleGate(gate)
	lifecycle.SetLifecycleGate(gate)
	quantity := 2
	_, err := lifecycle.UpdateSubscription(context.Background(), sub.AppID, sub.ID, &bizbilling.UpdateSubscriptionRequest{Quantity: &quantity, ChangeTiming: "next_period"})
	require.Error(t, err, "a legacy caller's explicit next-period choice cannot silently become an immediate change")
	stored, err := lifecycle.GetSubscription(context.Background(), sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, 1, stored.Quantity)
}
