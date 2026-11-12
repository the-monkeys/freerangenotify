package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func TestBizOutboxCommittedPaymentRecoversLostMaterializationResponse(t *testing.T) {
	s, _, lifecycle, ops, scope := outboxFixture(t)
	ctx := context.Background()
	inv := outboxInvoice(t, lifecycle, scope)
	fault := &lostBillingResponse{match: func(r *http.Request, _ []byte) bool {
		return strings.Contains(r.URL.Path, bizbillingrepo.NotificationOutboxIndex) && r.Method == http.MethodPut
	}}
	recovering, _ := operationFixture(t, fault)
	recovering.SetLifecycleGate(ops.gate)
	lifecycle.SetOperationService(recovering)
	fault.armed.Store(true)
	_, err := lifecycle.RecordPayment(WithBizOperationKey(ctx, "cash-lost-outbox"), scope.AppID, inv.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: inv.AmountDuePaisa, Reference: "STAFF"})
	require.Error(t, err)
	var pending *bizbilling.OperationFailure
	require.ErrorAs(t, err, &pending)
	saved, err := lifecycle.GetInvoice(ctx, scope.AppID, inv.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.InvoiceStatusPaid, saved.Status)
	_, err = recovering.Resume(ctx, scope.AppID, pending.Result.OperationID)
	require.NoError(t, err)
	rows, err := s.List(ctx, scope.AppID, 100)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	_, count, err := lifecycle.ListPayments(ctx, bizbilling.PaymentFilter{AppID: scope.AppID, UserID: scope.UserID})
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestBizOutboxSandboxSuppressedByDefault(t *testing.T) {
	s, sender, lifecycle, _, scope := outboxFixture(t)
	ctx := context.Background()
	outboxInvoice(t, lifecycle, scope)
	rows, err := s.List(ctx, scope.AppID, 100)
	require.NoError(t, err)
	_, err = s.records.Update(ctx, rows[0].ID, func(r *bizbilling.NotificationDelivery) error { r.Scope.Mode = bizbilling.GatewayTest; return nil })
	require.NoError(t, err)
	_, err = s.DispatchDueForApp(ctx, scope.AppID, 100)
	require.NoError(t, err)
	require.Zero(t, sender.calls)
	row, err := s.Get(ctx, scope.AppID, rows[0].ID)
	require.NoError(t, err)
	require.Equal(t, "test_mode_suppressed", row.FailureCode)
	_, err = s.Retry(ctx, scope.AppID, row.ID)
	require.ErrorIs(t, err, bizbilling.ErrConflict)
}

func TestBizOutboxRetryNeverRepeatsMoney(t *testing.T) {
	s, sender, lifecycle, _, scope := outboxFixture(t)
	ctx := context.Background()
	inv := outboxInvoice(t, lifecycle, scope)
	_, err := lifecycle.RecordPayment(WithBizOperationKey(ctx, "cash"), scope.AppID, inv.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: inv.AmountDuePaisa, Reference: "STAFF"})
	require.NoError(t, err)
	rows, err := s.List(ctx, scope.AppID, 100)
	require.NoError(t, err)
	id := rows[0].ID
	_, err = s.records.Update(ctx, id, func(r *bizbilling.NotificationDelivery) error {
		r.Status = "failed"
		r.Attempts = 6
		r.FailureCode = "admission_rejected"
		return nil
	})
	require.NoError(t, err)
	_, err = s.Retry(ctx, "wrong-app", id)
	require.True(t, apperrors.IsNotFound(err))
	retried, err := s.Retry(ctx, scope.AppID, id)
	require.NoError(t, err)
	require.Equal(t, "pending", retried.Status)
	_, err = s.DispatchDueForApp(ctx, scope.AppID, 100)
	require.NoError(t, err)
	_, err = s.Retry(ctx, scope.AppID, id)
	require.ErrorIs(t, err, bizbilling.ErrConflict)
	_, count, err := lifecycle.ListPayments(ctx, bizbilling.PaymentFilter{AppID: scope.AppID, UserID: scope.UserID})
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, 2, sender.calls)
}

func TestBizOutboxExpiredAttemptRequiresObservation(t *testing.T) {
	s, sender, lifecycle, _, scope := outboxFixture(t)
	ctx := context.Background()
	outboxInvoice(t, lifecycle, scope)
	rows, err := s.List(ctx, scope.AppID, 100)
	require.NoError(t, err)
	_, err = s.records.Update(ctx, rows[0].ID, func(r *bizbilling.NotificationDelivery) error {
		r.MayHaveDispatched = true
		r.Attempts = 1
		r.LeaseOwner = "crashed"
		r.LeaseUntil = time.Now().Add(-time.Hour)
		return nil
	})
	require.NoError(t, err)
	_, err = s.DispatchDueForApp(ctx, scope.AppID, 100)
	require.NoError(t, err)
	row, err := s.Get(ctx, scope.AppID, rows[0].ID)
	require.NoError(t, err)
	require.Equal(t, "unknown", row.Status)
	require.Zero(t, sender.calls)
	_, err = s.Retry(ctx, scope.AppID, row.ID)
	require.ErrorIs(t, err, bizbilling.ErrConflict)
}

func TestBizOutboxPendingLeaseDoesNotStarveReadyDelivery(t *testing.T) {
	s, sender, lifecycle, _, scope := outboxFixture(t)
	ctx := context.Background()
	outboxInvoice(t, lifecycle, scope)
	rows, err := s.List(ctx, scope.AppID, 100)
	require.NoError(t, err)
	blocked := *rows[0]
	blocked.ID = scope.AppID + "-000-leased"
	blocked.NotificationID = blocked.ID
	blocked.LeaseOwner = "another-worker"
	blocked.LeaseUntil = time.Now().Add(time.Hour)
	blocked.NextAttemptAt = time.Now().Add(-time.Hour)
	require.NoError(t, s.records.Create(ctx, blocked.ID, &blocked))
	_, err = s.DispatchDueForApp(ctx, scope.AppID, 1)
	require.NoError(t, err)
	require.Equal(t, 1, sender.calls)
}

func TestBizOutboxRefreshDoesNotStarveOlderUnknown(t *testing.T) {
	s, sender, lifecycle, _, scope := outboxFixture(t)
	ctx := context.Background()
	outboxInvoice(t, lifecycle, scope)
	rows, err := s.List(ctx, scope.AppID, 100)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		r := *rows[0]
		r.ID = fmt.Sprintf("%s-observe-%02d", scope.AppID, i)
		r.NotificationID = r.ID
		r.Status = "unknown"
		r.UpdatedAt = time.Now().Add(time.Duration(i-10) * time.Hour)
		require.NoError(t, s.records.Create(ctx, r.ID, &r))
		if i == 0 {
			sender.rows[r.NotificationID] = &notification.Notification{NotificationID: r.NotificationID, AppID: scope.AppID, UserID: scope.UserID, Status: notification.StatusDelivered}
		}
	}
	for i := 0; i < 3; i++ {
		_, err = s.RefreshDeliveryForApp(ctx, scope.AppID, 1)
		require.NoError(t, err)
	}
	row, err := s.Get(ctx, scope.AppID, scope.AppID+"-observe-00")
	require.NoError(t, err)
	require.Equal(t, "delivered", row.Status)
}

func TestBizSameDayReminderAddedChannelAndLegacyProgress(t *testing.T) {
	_, _, lifecycle, ops, scope := outboxFixture(t)
	ctx := context.Background()
	inv := outboxInvoice(t, lifecycle, scope)
	apply := func(key, channel string) *bizbilling.JournalEvent {
		r, err := ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandInvoiceReminder, Scope: scope, IdempotencyKey: key, InvoiceReminder: &bizbilling.InvoiceReminder{InvoiceID: inv.ID, DaysOverdue: 1, Steps: []bizbilling.DunningStep{{DayOffset: 0, Channel: channel}}}})
		require.NoError(t, err)
		e, err := ops.completedFinancialEvent(ctx, scope.AppID, r.OperationID)
		require.NoError(t, err)
		return e
	}
	require.Len(t, apply("email", "email").Notifications, 1)
	require.Len(t, apply("wa", "whatsapp").Notifications, 1)
	require.Empty(t, apply("email-again", "email").Notifications)
	legacy := &bizbilling.BizInvoice{Metadata: map[string]interface{}{"dunning_last_step": float64(7)}}
	require.True(t, dunningStepPlanned(legacy, bizbilling.DunningStep{DayOffset: 0, Channel: "whatsapp"}))
}

func TestBizOutboxPublicRecordExcludesSecretEnvelope(t *testing.T) {
	s, _, lifecycle, _, scope := outboxFixture(t)
	ctx := context.Background()
	outboxInvoice(t, lifecycle, scope)
	rows, err := s.List(ctx, scope.AppID, 100)
	require.NoError(t, err)
	sealed, err := s.codec.Seal([]byte(`{"payment_link":"https://fixture.invalid/?token=BEARER_FIXTURE"}`), deliverySecretBinding(rows[0]))
	require.NoError(t, err)
	_, err = s.records.Update(ctx, rows[0].ID, func(r *bizbilling.NotificationDelivery) error {
		r.SensitivePayload = &sealed
		r.LeaseOwner = "PRIVATE_LEASE"
		return nil
	})
	require.NoError(t, err)
	public, err := s.Get(ctx, scope.AppID, rows[0].ID)
	require.NoError(t, err)
	body, err := json.Marshal(public)
	require.NoError(t, err)
	require.NotContains(t, string(body), "sensitive_payload")
	require.NotContains(t, string(body), "BEARER_FIXTURE")
	require.NotContains(t, string(body), "PRIVATE_LEASE")
	stored, err := s.records.Get(ctx, rows[0].ID)
	require.NoError(t, err)
	data, err := s.preparePayload(ctx, stored)
	require.NoError(t, err)
	require.Contains(t, data["payment_link"], "BEARER_FIXTURE", "ordinary notification content intentionally receives the resolved link")
	sender := &billingDeliveryFake{rows: map[string]*notification.Notification{}}
	n, err := sender.Dispatch(ctx, stored.Intent, stored.NotificationID, data)
	require.NoError(t, err)
	ordinary, err := sender.Get(ctx, n.NotificationID, scope.AppID)
	require.NoError(t, err)
	encoded, err := json.Marshal(ordinary)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "BEARER_FIXTURE", "rendered notification content retains the link under the ordinary retention policy")
}

func TestBizSameDayLegacyCutoffSurvivesNewStepTracking(t *testing.T) {
	invoice := &bizbilling.BizInvoice{Metadata: map[string]interface{}{"dunning_last_step": float64(14), "dunning_steps_version": float64(2), "dunning_legacy_through_day": float64(7), "dunning_planned_steps": map[string]interface{}{dunningStepID(bizbilling.DunningStep{DayOffset: 14, Channel: "email"}): true}}}
	require.True(t, dunningStepPlanned(invoice, bizbilling.DunningStep{DayOffset: 0, Channel: "email"}), "switching to per-channel progress must not resend historical legacy steps")
	require.False(t, dunningStepPlanned(invoice, bizbilling.DunningStep{DayOffset: 10, Channel: "whatsapp"}))
}

func TestBizSameDayLegacyCutoffPersistsDuringReminderUpgrade(t *testing.T) {
	_, _, lifecycle, ops, scope := outboxFixture(t)
	ctx := context.Background()
	draft, err := lifecycle.CreateOneTimeInvoice(WithBizOperationKey(ctx, "legacy-draft"), scope.AppID, &bizbilling.CreateInvoiceRequest{UserID: scope.UserID, Currency: scope.Currency, Metadata: map[string]interface{}{"dunning_last_step": 7}, LineItems: []bizbilling.InvoiceLineItem{{Description: "Service", Quantity: 1, UnitPrice: 1000}}})
	require.NoError(t, err)
	inv, err := lifecycle.IssueInvoice(WithBizOperationKey(ctx, "legacy-issue"), scope.AppID, draft.ID)
	require.NoError(t, err)
	_, err = ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandInvoiceReminder, Scope: scope, IdempotencyKey: "new-step", InvoiceReminder: &bizbilling.InvoiceReminder{InvoiceID: inv.ID, DaysOverdue: 14, Steps: []bizbilling.DunningStep{{DayOffset: 14, Channel: "email"}}}})
	require.NoError(t, err)
	saved, err := lifecycle.GetInvoice(ctx, scope.AppID, inv.ID)
	require.NoError(t, err)
	require.True(t, dunningStepPlanned(saved, bizbilling.DunningStep{DayOffset: 0, Channel: "whatsapp"}))
	require.False(t, dunningStepPlanned(saved, bizbilling.DunningStep{DayOffset: 10, Channel: "whatsapp"}))
}
