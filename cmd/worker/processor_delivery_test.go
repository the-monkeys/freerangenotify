package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

func TestProcessNotification_CreditEnforcementDisabled(t *testing.T) {
	repo := newStubNotifRepo()
	notif := &notification.Notification{
		NotificationID: "disabled-credit", AppID: "app-1", UserID: "user-1",
		Channel: notification.ChannelEmail, Status: notification.StatusQueued,
		Content: notification.Content{Body: "Hello"},
	}
	repo.notifications[notif.NotificationID] = notif
	proc := newProcessorForTest(repo, nil,
		map[string]*user.User{"user-1": makeUser()},
		map[string]*application.Application{"app-1": makeApp()}, nil, nil)
	proc.creditService = services.NewCreditService(nil, nil, nil, nil, nil, nil, nil, zap.NewNop(), false)
	require.NotPanics(t, func() {
		proc.processNotification(context.Background(), makeQueueItem(notif.NotificationID), proc.logger)
	})
	require.Equal(t, notification.StatusSent, notif.Status)
	require.NotContains(t, notif.Metadata, "reservation_id")
}

func TestHandleFailure_PersistsRetryCountAndTerminalTime(t *testing.T) {
	repo := newStubNotifRepo()
	notif := &notification.Notification{
		NotificationID: "terminal-retry", AppID: "app-1", Status: notification.StatusProcessing,
		RetryCount: 3,
	}
	repo.notifications[notif.NotificationID] = notif
	proc := newProcessorForTest(repo, nil, nil, map[string]*application.Application{"app-1": makeApp()}, nil, nil)
	proc.handleFailure(context.Background(), notif, makeQueueItem(notif.NotificationID), errors.New("temporary failure"), "temporary failure")
	require.Equal(t, 4, repo.notifications[notif.NotificationID].RetryCount)
	require.Equal(t, notification.StatusFailed, notif.Status)
	require.NotNil(t, notif.FailedAt)
	require.WithinDuration(t, time.Now(), *notif.FailedAt, time.Second)
}

func TestHandleFailure_PersistsRetryDiagnostics(t *testing.T) {
	repo := newStubNotifRepo()
	q := &stubQueue{}
	notif := &notification.Notification{NotificationID: "retry-pending", AppID: "app-1", Status: notification.StatusProcessing}
	repo.notifications[notif.NotificationID] = notif
	proc := newProcessorForTest(repo, q, nil, map[string]*application.Application{"app-1": makeApp()}, nil, nil)
	proc.handleFailure(context.Background(), notif, makeQueueItem(notif.NotificationID), errors.New("temporary failure"), "temporary failure")
	require.Equal(t, 1, notif.RetryCount)
	require.Equal(t, notification.StatusQueued, notif.Status)
	require.Equal(t, "temporary failure", notif.ErrorMessage)
	require.Len(t, q.scheduled, 1)
}
func TestProcessNotification_ExpiredOTPIsNotDelivered(t *testing.T) {
	repo := newStubNotifRepo()
	notif := &notification.Notification{
		NotificationID: "expired-otp", AppID: "app-1", UserID: "user-1",
		Channel: notification.ChannelSMS, Status: notification.StatusQueued,
		Content:  notification.Content{Body: "OTP"},
		Metadata: map[string]interface{}{"otp_expires_at": time.Now().Add(-time.Minute).Format(time.RFC3339Nano)},
	}
	repo.notifications[notif.NotificationID] = notif
	proc := newProcessorForTest(repo, nil,
		map[string]*user.User{"user-1": makeUser()},
		map[string]*application.Application{"app-1": makeApp()}, nil, nil)
	proc.processNotification(context.Background(), makeQueueItem(notif.NotificationID), proc.logger)
	require.Equal(t, notification.StatusFailed, notif.Status)
	require.Nil(t, notif.SentAt)
	require.NotNil(t, notif.FailedAt)
	require.Equal(t, "notification_expired", notif.Metadata["failure_code"])
}

func TestHandleFailure_DoesNotRetryBeyondOTPExpiry(t *testing.T) {
	repo := newStubNotifRepo()
	q := &stubQueue{}
	notif := &notification.Notification{
		NotificationID: "expiring-otp", AppID: "app-1", Status: notification.StatusProcessing,
		Metadata: map[string]interface{}{"otp_expires_at": time.Now().Add(time.Second).Format(time.RFC3339Nano)},
	}
	repo.notifications[notif.NotificationID] = notif
	proc := newProcessorForTest(repo, q, nil, map[string]*application.Application{"app-1": makeApp()}, nil, nil)
	proc.config.RetryDelay = time.Minute
	proc.config.MaxRetryDelay = time.Minute
	proc.handleFailure(context.Background(), notif, makeQueueItem(notif.NotificationID), errors.New("temporary failure"), "temporary failure")
	require.Equal(t, notification.StatusFailed, notif.Status)
	require.Empty(t, q.scheduled)
	require.Equal(t, "notification_expired", notif.Metadata["failure_code"])
}

type deliveryTestSubscriptionRepo struct {
	license.Repository
	sub *license.Subscription
}

func (r *deliveryTestSubscriptionRepo) GetActiveSubscription(context.Context, string, string, time.Time) (*license.Subscription, error) {
	return r.sub, nil
}

type deliveryTestBalanceRepo struct {
	billing.CreditBalanceRepository
	balance           *billing.CreditBalance
	reserveErr        error
	commitErr         error
	releaseContextErr error
	commits           int
	releases          int
}

func (r *deliveryTestBalanceRepo) GetByTenantID(context.Context, string) (*billing.CreditBalance, error) {
	copy := *r.balance
	return &copy, nil
}
func (r *deliveryTestBalanceRepo) ReserveCredits(context.Context, string, int64) (*billing.CreditBalance, error) {
	if r.reserveErr != nil {
		return nil, r.reserveErr
	}
	copy := *r.balance
	copy.CreditsReserved++
	return &copy, nil
}
func (r *deliveryTestBalanceRepo) CommitReservedCredits(context.Context, string, int64) (*billing.CreditBalance, error) {
	r.commits++
	if r.commitErr != nil {
		return nil, r.commitErr
	}
	copy := *r.balance
	copy.CreditsRemaining--
	return &copy, nil
}
func (r *deliveryTestBalanceRepo) ReleaseReservedCredits(ctx context.Context, _ string, _ int64) (*billing.CreditBalance, error) {
	r.releases++
	r.releaseContextErr = ctx.Err()
	if r.releaseContextErr != nil {
		return nil, r.releaseContextErr
	}
	copy := *r.balance
	return &copy, nil
}

type deliveryTestLedger struct {
	billing.CreditLedgerRepository
	appendErr error
}

func (r *deliveryTestLedger) Append(context.Context, *billing.CreditLedgerEntry) error {
	return r.appendErr
}

func withDeliveryTestCredits(proc *NotificationProcessor, balance *deliveryTestBalanceRepo) {
	sub := &license.Subscription{ID: "sub-1", TenantID: "app-1", Plan: "free", CreditsTotal: 1500, CreditsRemaining: 1500,
		Metadata: map[string]interface{}{"billing_model": billing.BillingModelCredits}}
	proc.creditService = services.NewCreditService(balance, &deliveryTestLedger{}, &deliveryTestSubscriptionRepo{sub: sub},
		nil, nil, nil, nil, zap.NewNop(), true)
}

func TestProcessNotification_CreditContentionRetriesButDepletionFails(t *testing.T) {
	for _, temporary := range []bool{true, false} {
		name := "exhausted"
		remaining := int64(700)
		reserved := int64(0)
		if temporary {
			name = "temporarily_reserved"
			remaining = 1500
			reserved = 800
		}
		t.Run(name, func(t *testing.T) {
			repo := newStubNotifRepo()
			q := &stubQueue{}
			notif := &notification.Notification{NotificationID: name, AppID: "app-1", UserID: "user-1",
				Channel: notification.ChannelSMS, Status: notification.StatusQueued, Content: notification.Content{Body: "Hello"}}
			repo.notifications[name] = notif
			proc := newProcessorForTest(repo, q, map[string]*user.User{"user-1": makeUser()},
				map[string]*application.Application{"app-1": makeApp()}, nil, nil)
			rejection := billing.NewCreditUnavailableError(&billing.CreditBalance{CreditsRemaining: remaining, CreditsReserved: reserved}, 800, "test-rates")
			balance := &deliveryTestBalanceRepo{balance: &billing.CreditBalance{CreditsTotal: 1500, CreditsRemaining: 1500}, reserveErr: rejection}
			withDeliveryTestCredits(proc, balance)
			proc.processNotification(context.Background(), makeQueueItem(name), proc.logger)
			require.Nil(t, notif.SentAt)
			require.Equal(t, int64(700), notif.Metadata["credits_available"])
			require.Equal(t, int64(800), notif.Metadata["credits_required"])
			require.Equal(t, "insufficient credits", notif.ErrorMessage)
			require.Zero(t, balance.commits)
			if temporary {
				require.Equal(t, notification.StatusQueued, notif.Status)
				require.Len(t, q.scheduled, 1)
				require.Equal(t, 1, notif.RetryCount)
			} else {
				require.Equal(t, notification.StatusFailed, notif.Status)
				require.Empty(t, q.scheduled)
				require.Zero(t, notif.RetryCount)
			}
		})
	}
}

type deliveryTestProvider struct{ sends int }

func (*deliveryTestProvider) GetName() string { return "smtp" }
func (*deliveryTestProvider) GetSupportedChannel() notification.Channel {
	return notification.ChannelEmail
}
func (*deliveryTestProvider) IsHealthy(context.Context) bool { return true }
func (*deliveryTestProvider) Close() error                   { return nil }
func (p *deliveryTestProvider) Send(context.Context, *notification.Notification, *user.User) (*providers.Result, error) {
	p.sends++
	return providers.NewErrorResult(errors.New("535 Authentication failed token=secretpassword"), providers.ErrorTypeAuth), nil
}

func TestProcessNotification_AuthenticationReleasesCreditsWithoutRetryOrBurn(t *testing.T) {
	repo := newStubNotifRepo()
	q := &stubQueue{}
	notif := &notification.Notification{NotificationID: "auth-failure", AppID: "app-1", UserID: "user-1",
		Channel: notification.ChannelEmail, Status: notification.StatusQueued, Content: notification.Content{Body: "Hello"}}
	repo.notifications[notif.NotificationID] = notif
	proc := newProcessorForTest(repo, q, map[string]*user.User{"user-1": makeUser()},
		map[string]*application.Application{"app-1": makeApp()}, nil, nil)
	balance := &deliveryTestBalanceRepo{balance: &billing.CreditBalance{CreditsTotal: 1500, CreditsRemaining: 1500}}
	withDeliveryTestCredits(proc, balance)
	adapter := &deliveryTestProvider{}
	proc.providerManager = providers.NewManager(nil, nil, zap.NewNop())
	require.NoError(t, proc.providerManager.RegisterProvider(adapter))
	proc.processNotification(context.Background(), makeQueueItem(notif.NotificationID), proc.logger)
	require.Equal(t, 1, adapter.sends)
	require.Equal(t, 1, balance.releases)
	require.Zero(t, balance.commits)
	require.Equal(t, notification.StatusFailed, notif.Status)
	require.NotNil(t, notif.FailedAt)
	require.Empty(t, q.scheduled)
	require.Equal(t, "authentication", notif.Metadata["failure_code"])
	require.Equal(t, false, notif.Metadata["retryable"])
	require.NotContains(t, notif.ErrorMessage, "secretpassword")
}

type retryFailureQueue struct {
	stubQueue
	ackCalls int
}

func (*retryFailureQueue) EnqueueScheduled(context.Context, queue.NotificationQueueItem, time.Time) error {
	return errors.New("retry queue unavailable")
}
func (q *retryFailureQueue) Acknowledge(context.Context, queue.NotificationQueueItem) error {
	q.ackCalls++
	return nil
}

func TestProcessNotification_RetryEnqueueFailureKeepsProcessingItem(t *testing.T) {
	repo := newStubNotifRepo()
	notif := &notification.Notification{NotificationID: "retry-store-down", AppID: "app-1", UserID: "user-1",
		Channel: notification.ChannelSMS, Status: notification.StatusQueued, Content: notification.Content{Body: "Hello"}}
	repo.notifications[notif.NotificationID] = notif
	proc := newProcessorForTest(repo, nil, map[string]*user.User{"user-1": makeUser()},
		map[string]*application.Application{"app-1": makeApp()}, nil, nil)
	failedQueue := &retryFailureQueue{}
	proc.queue = failedQueue
	rejection := billing.NewCreditUnavailableError(&billing.CreditBalance{CreditsRemaining: 1500, CreditsReserved: 800}, 800, "test-rates")
	withDeliveryTestCredits(proc, &deliveryTestBalanceRepo{balance: &billing.CreditBalance{CreditsTotal: 1500, CreditsRemaining: 1500}, reserveErr: rejection})
	proc.processNotification(context.Background(), makeQueueItem(notif.NotificationID), proc.logger)
	require.Zero(t, failedQueue.ackCalls)
	require.NotEqual(t, notification.StatusQueued, notif.Status)
}
func TestProcessNotification_NewProviderFailureDoesNotReuseOldCreditSnapshot(t *testing.T) {
	repo := newStubNotifRepo()
	q := &stubQueue{}
	notif := &notification.Notification{NotificationID: "new-attempt", AppID: "app-1", UserID: "user-1",
		Channel: notification.ChannelEmail, Status: notification.StatusQueued, Content: notification.Content{Body: "Hello"},
		ErrorMessage: "insufficient credits", Metadata: map[string]interface{}{
			"failure_code": "credits_temporarily_reserved", "credits_available": int64(700),
			"credits_reserved": int64(800), "credits_remaining": int64(1500), "credits_required": int64(800),
		}}
	repo.notifications[notif.NotificationID] = notif
	proc := newProcessorForTest(repo, q, map[string]*user.User{"user-1": makeUser()},
		map[string]*application.Application{"app-1": makeApp()}, nil, nil)
	proc.providerManager = providers.NewManager(nil, nil, zap.NewNop())
	require.NoError(t, proc.providerManager.RegisterProvider(&deliveryTestProvider{}))
	proc.processNotification(context.Background(), makeQueueItem(notif.NotificationID), proc.logger)
	require.Equal(t, "authentication", notif.Metadata["failure_code"])
	require.NotContains(t, notif.Metadata, "credits_available")
	require.NotContains(t, notif.Metadata, "credits_required")
}
func TestProcessNotification_CompletedOTPIsNotChangedByExpiry(t *testing.T) {
	for _, status := range []notification.Status{notification.StatusSent, notification.StatusDelivered, notification.StatusRead} {
		t.Run(string(status), func(t *testing.T) {
			repo := newStubNotifRepo()
			sent := time.Now().Add(-time.Minute)
			notif := &notification.Notification{NotificationID: "completed-otp", AppID: "app-1", Status: status, SentAt: &sent,
				Metadata: map[string]interface{}{"otp_expires_at": time.Now().Add(-time.Second).Format(time.RFC3339Nano)}}
			repo.notifications[notif.NotificationID] = notif
			proc := newProcessorForTest(repo, nil, nil, nil, nil, nil)
			proc.processNotification(context.Background(), makeQueueItem(notif.NotificationID), proc.logger)
			require.Equal(t, status, notif.Status)
			require.Nil(t, notif.FailedAt)
			require.Empty(t, notif.ErrorMessage)
		})
	}
}

func TestHandleFailure_TerminalAuthenticationPreservedForUnexpiredOTP(t *testing.T) {
	repo := newStubNotifRepo()
	notif := &notification.Notification{NotificationID: "unexpired-auth", AppID: "app-1", Status: notification.StatusProcessing,
		Metadata: map[string]interface{}{"otp_expires_at": time.Now().Add(time.Second).Format(time.RFC3339Nano)}}
	repo.notifications[notif.NotificationID] = notif
	proc := newProcessorForTest(repo, nil, nil, map[string]*application.Application{"app-1": makeApp()}, nil, nil)
	proc.config.RetryDelay = time.Minute
	proc.config.MaxRetryDelay = time.Minute
	failure := &providers.DeliveryError{Provider: "smtp", ErrorType: providers.ErrorTypeAuth, Stage: "auth", Retryable: false, Err: errors.New("535 auth failed")}
	proc.handleFailure(context.Background(), notif, makeQueueItem(notif.NotificationID), failure, failure.Error())
	require.Equal(t, "authentication", notif.Metadata["failure_code"])
	require.Equal(t, notification.StatusFailed, notif.Status)
	require.NotEqual(t, "notification expired", notif.ErrorMessage)
}

type delayedDeliveryUserRepo struct {
	stubUserRepo
	until time.Time
}

func (r *delayedDeliveryUserRepo) GetByID(ctx context.Context, id string) (*user.User, error) {
	time.Sleep(time.Until(r.until))
	return r.stubUserRepo.GetByID(ctx, id)
}
func TestProcessNotification_OTPExpiresDuringPreflight(t *testing.T) {
	repo := newStubNotifRepo()
	expires := time.Now().Add(80 * time.Millisecond)
	notif := &notification.Notification{NotificationID: "preflight-expiry", AppID: "app-1", UserID: "user-1",
		Channel: notification.ChannelEmail, Status: notification.StatusQueued, Content: notification.Content{Body: "OTP"},
		Metadata: map[string]interface{}{"otp_expires_at": expires.Format(time.RFC3339Nano)}}
	repo.notifications[notif.NotificationID] = notif
	proc := newProcessorForTest(repo, nil, nil, map[string]*application.Application{"app-1": makeApp()}, nil, nil)
	proc.userRepo = &delayedDeliveryUserRepo{stubUserRepo: stubUserRepo{users: map[string]*user.User{"user-1": makeUser()}}, until: expires.Add(40 * time.Millisecond)}
	adapter := &deliveryTestProvider{}
	proc.providerManager = providers.NewManager(nil, nil, zap.NewNop())
	require.NoError(t, proc.providerManager.RegisterProvider(adapter))
	proc.processNotification(context.Background(), makeQueueItem(notif.NotificationID), proc.logger)
	require.Zero(t, adapter.sends)
	require.Equal(t, "notification_expired", notif.Metadata["failure_code"])
}

type clearFailureRepo struct {
	*stubNotifRepo
	clearErr error
}

func (r *clearFailureRepo) ClearDeliveryFailure(ctx context.Context, id string) error {
	if r.clearErr != nil {
		return r.clearErr
	}
	clearNotificationFailure(r.notifications[id])
	return nil
}

func TestProcessNotification_DiagnosticClearFailureRetainsItemBeforeDelivery(t *testing.T) {
	stored := newStubNotifRepo()
	failedAt := time.Now().Add(-time.Minute)
	notif := &notification.Notification{NotificationID: "clear-down", AppID: "app-1", UserID: "user-1",
		Channel: notification.ChannelEmail, Status: notification.StatusQueued, Content: notification.Content{Body: "Hello"},
		ErrorMessage: "old failure", FailedAt: &failedAt, Metadata: map[string]interface{}{
			"failure_code": "credits_temporarily_reserved", "credits_available": int64(700), "customer_marker": "keep",
		}}
	stored.notifications[notif.NotificationID] = notif
	proc := newProcessorForTest(stored, nil, map[string]*user.User{"user-1": makeUser()},
		map[string]*application.Application{"app-1": makeApp()}, nil, nil)
	repo := &clearFailureRepo{stubNotifRepo: stored, clearErr: errors.New("ES unavailable")}
	proc.notifRepo = repo
	q := &retryFailureQueue{}
	proc.queue = q
	adapter := &deliveryTestProvider{}
	proc.providerManager = providers.NewManager(nil, nil, zap.NewNop())
	require.NoError(t, proc.providerManager.RegisterProvider(adapter))

	proc.processNotification(context.Background(), makeQueueItem(notif.NotificationID), proc.logger)
	require.Zero(t, adapter.sends)
	require.Zero(t, q.ackCalls)
	require.Empty(t, stored.statusUpdates)
	require.Equal(t, "old failure", notif.ErrorMessage)
	require.Equal(t, "credits_temporarily_reserved", notif.Metadata["failure_code"])

	// Visibility recovery may proceed only after the persisted removal succeeds.
	repo.clearErr = nil
	proc.processNotification(context.Background(), makeQueueItem(notif.NotificationID), proc.logger)
	require.Equal(t, 1, adapter.sends)
	require.Equal(t, 1, q.ackCalls)
	require.Equal(t, "authentication", notif.Metadata["failure_code"])
	require.NotContains(t, notif.Metadata, "credits_available")
	require.Equal(t, "keep", notif.Metadata["customer_marker"])
}

type deadlineDeliveryProvider struct{ deliveryTestProvider }

func (p *deadlineDeliveryProvider) Send(ctx context.Context, _ *notification.Notification, _ *user.User) (*providers.Result, error) {
	p.sends++
	<-ctx.Done()
	return providers.NewErrorResult(ctx.Err(), providers.ErrorTypeTimeout), ctx.Err()
}

type accountingContextQueue struct {
	stubQueue
	ackCalls      int
	ackContextErr error
	ackErr        error
}

func (q *accountingContextQueue) Acknowledge(ctx context.Context, _ queue.NotificationQueueItem) error {
	q.ackCalls++
	q.ackContextErr = ctx.Err()
	if q.ackErr != nil {
		return q.ackErr
	}
	return ctx.Err()
}
func TestProcessNotification_OTPDeadlineDoesNotCancelReleaseOrAcknowledgement(t *testing.T) {
	repo := newStubNotifRepo()
	notif := &notification.Notification{NotificationID: "provider-timeout-otp", AppID: "app-1", UserID: "user-1",
		Channel: notification.ChannelEmail, Status: notification.StatusQueued, Content: notification.Content{Body: "OTP"},
		Metadata: map[string]interface{}{"otp_expires_at": time.Now().Add(500 * time.Millisecond).Format(time.RFC3339Nano)}}
	repo.notifications[notif.NotificationID] = notif
	proc := newProcessorForTest(repo, nil, map[string]*user.User{"user-1": makeUser()}, map[string]*application.Application{"app-1": makeApp()}, nil, nil)
	q := &accountingContextQueue{}
	proc.queue = q
	balance := &deliveryTestBalanceRepo{balance: &billing.CreditBalance{ID: "sub-1", TenantID: "app-1", CreditsTotal: 1500, CreditsRemaining: 1500}}
	withDeliveryTestCredits(proc, balance)
	adapter := &deadlineDeliveryProvider{}
	proc.providerManager = providers.NewManager(nil, nil, zap.NewNop())
	require.NoError(t, proc.providerManager.RegisterProvider(adapter))
	proc.processNotification(context.Background(), makeQueueItem(notif.NotificationID), proc.logger)
	require.Equal(t, 1, adapter.sends)
	require.Equal(t, 1, balance.releases)
	require.NoError(t, balance.releaseContextErr)
	require.Zero(t, balance.commits)
	require.Equal(t, notification.StatusFailed, notif.Status)
	require.Nil(t, notif.SentAt)
	require.Equal(t, "notification_expired", notif.Metadata["failure_code"])
	require.Equal(t, 1, q.ackCalls)
	require.NoError(t, q.ackContextErr)
}

type successfulDeliveryProvider struct{ deliveryTestProvider }

func (p *successfulDeliveryProvider) Send(context.Context, *notification.Notification, *user.User) (*providers.Result, error) {
	p.sends++
	return providers.NewResult("accepted-message", time.Millisecond), nil
}
func TestProcessNotification_SuccessfulDeliveryAccountingOutageDoesNotResend(t *testing.T) {
	for _, failureStage := range []string{"commit_before_delta", "ledger_after_delta", "acknowledgement"} {
		t.Run(failureStage, func(t *testing.T) {
			repo := newStubNotifRepo()
			notif := &notification.Notification{NotificationID: "accepted-" + failureStage, AppID: "app-1", UserID: "user-1",
				Channel: notification.ChannelEmail, Status: notification.StatusQueued, Content: notification.Content{Body: "Hello"}}
			repo.notifications[notif.NotificationID] = notif
			proc := newProcessorForTest(repo, nil, map[string]*user.User{"user-1": makeUser()}, map[string]*application.Application{"app-1": makeApp()}, nil, nil)
			q := &accountingContextQueue{}
			proc.queue = q
			balance := &deliveryTestBalanceRepo{balance: &billing.CreditBalance{ID: "sub-1", TenantID: "app-1", CreditsTotal: 1500, CreditsRemaining: 1500}}
			ledger := &deliveryTestLedger{}
			if failureStage == "commit_before_delta" {
				balance.commitErr = errors.New("ES unavailable")
			}
			if failureStage == "ledger_after_delta" {
				ledger.appendErr = errors.New("ledger unavailable")
			}
			if failureStage == "acknowledgement" {
				q.ackErr = errors.New("Redis acknowledgement unavailable")
			}
			sub := &license.Subscription{ID: "sub-1", TenantID: "app-1", Plan: "free", CreditsTotal: 1500, CreditsRemaining: 1500,
				Metadata: map[string]interface{}{"billing_model": billing.BillingModelCredits}}
			proc.creditService = services.NewCreditService(balance, ledger, &deliveryTestSubscriptionRepo{sub: sub}, nil, nil, nil, nil, zap.NewNop(), true)
			adapter := &successfulDeliveryProvider{}
			proc.providerManager = providers.NewManager(nil, nil, zap.NewNop())
			require.NoError(t, proc.providerManager.RegisterProvider(adapter))

			proc.processNotification(context.Background(), makeQueueItem(notif.NotificationID), proc.logger)
			require.Equal(t, notification.StatusSent, notif.Status)
			require.NotNil(t, notif.SentAt)
			require.Equal(t, 1, balance.commits)
			require.Zero(t, balance.releases)
			require.Empty(t, q.scheduled)
			require.Equal(t, 1, q.ackCalls)
			proc.processNotification(context.Background(), makeQueueItem(notif.NotificationID), proc.logger)
			require.Equal(t, 1, adapter.sends, "recovery must not resend an accepted delivery")
			require.Equal(t, 1, balance.commits, "recovered item must not re-reserve or commit")
			require.Equal(t, 2, q.ackCalls)
			require.Empty(t, q.scheduled)
		})
	}
}

type atomicDeliveryRepo struct {
	*stubNotifRepo
	sentOnlyCalls int
}

func (r *atomicDeliveryRepo) UpdateStatus(ctx context.Context, id string, status notification.Status) error {
	if status == notification.StatusSent {
		r.sentOnlyCalls++
	}
	return r.stubNotifRepo.UpdateStatus(ctx, id, status)
}
func TestProcessNotification_SuccessPersistsReservationBindingWithStatus(t *testing.T) {
	stored := newStubNotifRepo()
	notif := &notification.Notification{NotificationID: "atomic-success-binding", AppID: "app-1", UserID: "user-1",
		Channel: notification.ChannelEmail, Status: notification.StatusQueued, Content: notification.Content{Body: "Hello"},
		Metadata: map[string]interface{}{"reservation_id": "prior-attempt"}}
	stored.notifications[notif.NotificationID] = notif
	proc := newProcessorForTest(stored, nil, map[string]*user.User{"user-1": makeUser()}, map[string]*application.Application{"app-1": makeApp()}, nil, nil)
	repo := &atomicDeliveryRepo{stubNotifRepo: stored}
	proc.notifRepo = repo
	withDeliveryTestCredits(proc, &deliveryTestBalanceRepo{balance: &billing.CreditBalance{ID: "sub-1", TenantID: "app-1", CreditsTotal: 1500, CreditsRemaining: 1500}})
	proc.providerManager = providers.NewManager(nil, nil, zap.NewNop())
	require.NoError(t, proc.providerManager.RegisterProvider(&successfulDeliveryProvider{}))
	proc.processNotification(context.Background(), makeQueueItem(notif.NotificationID), proc.logger)
	require.Zero(t, repo.sentOnlyCalls, "never publish success separately from its reservation binding")
	require.Equal(t, notification.StatusSent, stored.notifications[notif.NotificationID].Status)
	require.NotNil(t, stored.notifications[notif.NotificationID].SentAt)
	require.NotEqual(t, "prior-attempt", notif.Metadata["reservation_id"])
	require.Equal(t, notif.Metadata["reservation_id"], notif.Metadata["delivery_reservation_id"])
}

type deliveryEvidenceBalanceRepo struct {
	billing.CreditBalanceRepository
	billing.CreditReservationJournal
	receipt     *billing.CreditReservationReceipt
	recordCalls int
}

func (*deliveryEvidenceBalanceRepo) GetByTenantID(context.Context, string) (*billing.CreditBalance, error) {
	return &billing.CreditBalance{ID: "sub-1", TenantID: "app-1", JournalBacked: true, CreditsTotal: 1500, CreditsRemaining: 1500}, nil
}
func (r *deliveryEvidenceBalanceRepo) ReserveReservation(_ context.Context, res *billing.CreditReservation) (*billing.CreditTransitionOutcome, error) {
	r.receipt = &billing.CreditReservationReceipt{Reservation: *res, Balance: billing.CreditBalance{ID: "sub-1", TenantID: "app-1", CreditsRemaining: 1500, CreditsReserved: 1}}
	return &billing.CreditTransitionOutcome{Receipt: *r.receipt, Applied: true}, nil
}
func (r *deliveryEvidenceBalanceRepo) RecordReservationDeliveryFailure(context.Context, string) error {
	r.recordCalls++
	r.receipt.DeliveryOutcome = "failed"
	return nil
}
func (*deliveryEvidenceBalanceRepo) TransitionReservation(context.Context, string, billing.CreditReservationStatus, string) (*billing.CreditTransitionOutcome, error) {
	return nil, errors.New("release storage unavailable")
}
func (r *deliveryEvidenceBalanceRepo) GetReservationReceipt(context.Context, string) (*billing.CreditReservationReceipt, error) {
	return r.receipt, nil
}

type uncertainDeliveryProvider struct{ deliveryTestProvider }

func (p *uncertainDeliveryProvider) Send(context.Context, *notification.Notification, *user.User) (*providers.Result, error) {
	p.sends++
	err := context.DeadlineExceeded
	return providers.NewErrorResult(err, providers.ErrorTypeTimeout), err
}
func TestProcessNotification_ReleaseOutageRetainsDefiniteAttemptEvidence(t *testing.T) {
	for _, definite := range []bool{true, false} {
		name := "authentication_rejected"
		if !definite {
			name = "unknown_send_timeout"
		}
		t.Run(name, func(t *testing.T) {
			repo := newStubNotifRepo()
			notif := &notification.Notification{NotificationID: name, AppID: "app-1", UserID: "user-1",
				Channel: notification.ChannelEmail, Status: notification.StatusQueued, Content: notification.Content{Body: "Hello"}}
			repo.notifications[name] = notif
			proc := newProcessorForTest(repo, nil, map[string]*user.User{"user-1": makeUser()}, map[string]*application.Application{"app-1": makeApp()}, nil, nil)
			balance := &deliveryEvidenceBalanceRepo{}
			sub := &license.Subscription{ID: "sub-1", TenantID: "app-1", Plan: "free", CreditsTotal: 1500, CreditsRemaining: 1500,
				Metadata: map[string]interface{}{"billing_model": billing.BillingModelCredits}}
			proc.creditService = services.NewCreditService(balance, &deliveryTestLedger{}, &deliveryTestSubscriptionRepo{sub: sub}, nil, nil, nil, nil, zap.NewNop(), true)
			proc.creditService.EnableReservationJournal(true)
			proc.providerManager = providers.NewManager(nil, nil, zap.NewNop())
			if definite {
				require.NoError(t, proc.providerManager.RegisterProvider(&deliveryTestProvider{}))
			} else {
				require.NoError(t, proc.providerManager.RegisterProvider(&uncertainDeliveryProvider{}))
			}
			proc.processNotification(context.Background(), makeQueueItem(name), proc.logger)
			require.NotNil(t, balance.receipt)
			require.Equal(t, billing.CreditReservationReserved, balance.receipt.Reservation.Status)
			require.Nil(t, notif.SentAt)
			if definite {
				require.Equal(t, 1, balance.recordCalls)
				require.Equal(t, "failed", balance.receipt.DeliveryOutcome)
				require.Equal(t, notification.StatusFailed, notif.Status)
			} else {
				require.Zero(t, balance.recordCalls)
				require.Empty(t, balance.receipt.DeliveryOutcome, "do not invent evidence for an uncertain provider outcome")
				require.Equal(t, notification.StatusQueued, notif.Status)
			}
		})
	}
}
