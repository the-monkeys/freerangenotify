package services

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/secrets"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

type billingDeliveryFake struct {
	mu        sync.Mutex
	calls     int
	failure   error
	uncertain bool
	rows      map[string]*notification.Notification
}

func (s *billingDeliveryFake) Dispatch(_ context.Context, intent bizbilling.NotificationIntent, id string, data map[string]interface{}) (*notification.Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.failure != nil {
		return nil, s.failure
	}
	n := &notification.Notification{NotificationID: id, AppID: intent.AppID, UserID: intent.UserID, Channel: notification.Channel(intent.Channel), Status: notification.StatusQueued, Content: notification.Content{Data: data}}
	s.rows[id] = n
	if s.uncertain {
		return nil, &notification.DispatchError{NotificationID: id, MayBeEnqueued: true, Cause: errors.New("lost queue acknowledgement")}
	}
	return n, nil
}
func (s *billingDeliveryFake) Get(_ context.Context, id, app string) (*notification.Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.rows[id]
	if n == nil || n.AppID != app {
		return nil, apperrors.NotFound("notification", id)
	}
	copy := *n
	return &copy, nil
}

func outboxFixture(t *testing.T) (*BizOutboxService, *billingDeliveryFake, *BizLifecycleService, *BizOperationService, bizbilling.Scope) {
	t.Helper()
	lifecycle, _, ops, es, scope := financialFacadesFixture(t)
	ensureBillingIndex(t, es, "frn_biz_notification_outbox", (&database.IndexTemplates{}).GetBizNotificationOutboxTemplate())
	codec, err := secrets.NewConnectionCodec("fixture:" + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))))
	require.NoError(t, err)
	sender := &billingDeliveryFake{rows: map[string]*notification.Notification{}}
	service := NewBizOutboxService(es, ops, sender, nil, codec, zap.NewNop())
	return service, sender, lifecycle, ops, scope
}

func outboxInvoice(t *testing.T, lifecycle *BizLifecycleService, scope bizbilling.Scope) *bizbilling.BizInvoice {
	t.Helper()
	ctx := context.Background()
	draft, err := lifecycle.CreateOneTimeInvoice(WithBizOperationKey(ctx, "draft"), scope.AppID, &bizbilling.CreateInvoiceRequest{UserID: scope.UserID, Currency: scope.Currency, LineItems: []bizbilling.InvoiceLineItem{{Description: "Service", Quantity: 1, UnitPrice: 1000}}})
	require.NoError(t, err)
	invoice, err := lifecycle.IssueInvoice(WithBizOperationKey(ctx, "issue"), scope.AppID, draft.ID)
	require.NoError(t, err)
	return invoice
}

func TestBizNotificationFailureDoesNotUndoPayment(t *testing.T) {
	service, sender, lifecycle, _, scope := outboxFixture(t)
	ctx := context.Background()
	invoice := outboxInvoice(t, lifecycle, scope)
	_, err := lifecycle.RecordPayment(WithBizOperationKey(ctx, "cash"), scope.AppID, invoice.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: invoice.AmountDuePaisa, Reference: "STAFF-CONFIRMED"})
	require.NoError(t, err)
	sender.failure = errors.New("insufficient notification credits")
	_, err = service.DispatchDueForApp(ctx, scope.AppID, 100)
	require.NoError(t, err, "a known admission failure is recorded, not a payment failure")
	stored, err := lifecycle.GetInvoice(ctx, scope.AppID, invoice.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.InvoiceStatusPaid, stored.Status)
	require.Equal(t, invoice.TotalPaisa, stored.AmountPaidPaisa)
	rows, err := service.List(ctx, scope.AppID, 100)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.NotEqual(t, "delivered", row.Status)
		require.NotEmpty(t, row.FailureCode)
	}
	_, count, err := lifecycle.ListPayments(ctx, bizbilling.PaymentFilter{AppID: scope.AppID, UserID: scope.UserID})
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestBizSameDayReminderChannels(t *testing.T) {
	service, _, lifecycle, ops, scope := outboxFixture(t)
	ctx := context.Background()
	invoice := outboxInvoice(t, lifecycle, scope)
	result, err := ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandInvoiceReminder, Scope: scope, IdempotencyKey: "two-channels", InvoiceReminder: &bizbilling.InvoiceReminder{InvoiceID: invoice.ID, DaysOverdue: 1, Steps: []bizbilling.DunningStep{{DayOffset: 0, Channel: "email", Template: "biz_reminder_gentle"}, {DayOffset: 0, Channel: "whatsapp", Template: "biz_reminder_whatsapp"}}}})
	require.NoError(t, err)
	event, err := ops.completedFinancialEvent(ctx, scope.AppID, result.OperationID)
	require.NoError(t, err)
	require.Len(t, event.Notifications, 2)
	require.NotEqual(t, event.Notifications[0].ID, event.Notifications[1].ID)
	require.NotEmpty(t, event.Notifications[0].StepID)
	rows, err := service.List(ctx, scope.AppID, 100)
	require.NoError(t, err)
	require.Len(t, rows, 3, "one issued invoice and both reminder channels")
}

func TestBizOutboxAdmissionRecovery(t *testing.T) {
	service, sender, lifecycle, ops, scope := outboxFixture(t)
	ctx := context.Background()
	outboxInvoice(t, lifecycle, scope)
	sender.uncertain = true
	_, err := service.DispatchDueForApp(ctx, scope.AppID, 100)
	require.NoError(t, err)
	rows, err := service.List(ctx, scope.AppID, 100)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "unknown", rows[0].Status)
	restarted := NewBizOutboxService(service.es, ops, sender, nil, service.codec, zap.NewNop())
	_, err = restarted.RefreshDeliveryForApp(ctx, scope.AppID, 100)
	require.NoError(t, err)
	_, err = restarted.DispatchDueForApp(ctx, scope.AppID, 100)
	require.NoError(t, err)
	require.Equal(t, 1, sender.calls, "uncertain admission must never blindly resend")
	sender.mu.Lock()
	sender.rows[rows[0].NotificationID].Status = notification.StatusSent
	sender.mu.Unlock()
	_, err = restarted.RefreshDeliveryForApp(ctx, scope.AppID, 100)
	require.NoError(t, err)
	rows, err = restarted.List(ctx, scope.AppID, 100)
	require.NoError(t, err)
	require.Equal(t, "accepted", rows[0].Status, "provider acceptance does not prove recipient delivery")
}

func TestBizAcceptedIsNotDelivered(t *testing.T) {
	service, sender, lifecycle, _, scope := outboxFixture(t)
	ctx := context.Background()
	outboxInvoice(t, lifecycle, scope)
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() { defer workers.Done(); _, _ = service.DispatchDueForApp(ctx, scope.AppID, 100) }()
	}
	workers.Wait()
	_, err := service.DispatchDueForApp(ctx, scope.AppID, 100)
	require.NoError(t, err)
	require.Equal(t, 1, sender.calls)
	rows, err := service.List(ctx, scope.AppID, 100)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "accepted", rows[0].Status)
	sender.mu.Lock()
	sender.rows[rows[0].NotificationID].Status = notification.StatusFailed
	sender.mu.Unlock()
	_, err = service.RefreshDeliveryForApp(ctx, scope.AppID, 100)
	require.NoError(t, err)
	failed, err := service.Get(ctx, scope.AppID, rows[0].ID)
	require.NoError(t, err)
	require.Equal(t, "unknown", failed.Status, "a provider failure needs review before any resend")
	_, err = service.Retry(ctx, scope.AppID, rows[0].ID)
	require.ErrorIs(t, err, bizbilling.ErrConflict)
	sender.mu.Lock()
	sender.rows[rows[0].NotificationID].Status = notification.StatusDelivered
	sender.mu.Unlock()
	_, err = service.RefreshDeliveryForApp(ctx, scope.AppID, 100)
	require.NoError(t, err)
	rows, err = service.List(ctx, scope.AppID, 100)
	require.NoError(t, err)
	require.Equal(t, "delivered", rows[0].Status)
	_, err = service.Get(ctx, "wrong-app", rows[0].ID)
	require.True(t, apperrors.IsNotFound(err))
}

func TestBizOutboxAdmissionRetriesAreBounded(t *testing.T) {
	service, sender, lifecycle, _, scope := outboxFixture(t)
	ctx := context.Background()
	outboxInvoice(t, lifecycle, scope)
	sender.failure = errors.New("channel not configured")
	for i := 0; i < 10; i++ {
		rows, err := service.List(ctx, scope.AppID, 100)
		require.NoError(t, err)
		for _, row := range rows {
			_, err = service.records.Update(ctx, row.ID, func(r *bizbilling.NotificationDelivery) error {
				r.NextAttemptAt = time.Now().Add(-time.Hour)
				return nil
			})
			require.NoError(t, err)
		}
		_, err = service.DispatchDueForApp(ctx, scope.AppID, 100)
		require.NoError(t, err)
	}
	rows, err := service.List(ctx, scope.AppID, 100)
	require.NoError(t, err)
	require.Equal(t, "failed", rows[0].Status)
	require.LessOrEqual(t, sender.calls, 6)
}
