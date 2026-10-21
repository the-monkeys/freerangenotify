package services

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/otp"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
)

type otpSenderFunc func(context.Context, notification.SendRequest) (*notification.Notification, error)

func (f otpSenderFunc) Send(ctx context.Context, r notification.SendRequest) (*notification.Notification, error) {
	return f(ctx, r)
}
func TestOTPDispatchStateBeforeEnqueue(t *testing.T) {
	svc, repo, sender, _, _ := newServiceForTest(t)
	repo.failOn = "Create"
	_, err := svc.Send(context.Background(), otp.SendInput{AppID: "app-a", Channel: otp.ChannelEmail, Recipient: "alice@example.test"})
	require.Error(t, err)
	require.Empty(t, sender.calls, "an unverifiable code must never leave the service")
}
func seedResend(t *testing.T, repo *fakeOTPRepo) *otp.Request {
	t.Helper()
	r := &otp.Request{RequestID: "otp-resend", AppID: "app-a", Channel: otp.ChannelEmail, Recipient: "alice@example.test", CodeHash: hashCode("123456", "salt"), Salt: "salt", Length: 6, MaxAttempts: 5, LastSentAt: time.Now().UTC().Add(-2 * time.Minute), ExpiresAt: time.Now().UTC().Add(time.Minute)}
	require.NoError(t, repo.Create(context.Background(), r))
	return r
}
func TestOTPResendConcurrentClaim(t *testing.T) {
	svc, repo, _, _, _ := newServiceForTest(t)
	r := seedResend(t, repo)
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	var calls atomic.Int32
	svc.notificationSvc = otpSenderFunc(func(ctx context.Context, _ notification.SendRequest) (*notification.Notification, error) {
		calls.Add(1)
		entered <- struct{}{}
		<-release
		id, _ := notification.DispatchIdentityFromContext(ctx)
		return &notification.Notification{NotificationID: id}, nil
	})
	first := make(chan error, 1)
	go func() { _, err := svc.Resend(context.Background(), r.RequestID); first <- err }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("first resend did not reach enqueue")
	}
	second := make(chan error, 1)
	go func() { _, err := svc.Resend(context.Background(), r.RequestID); second <- err }()
	select {
	case err := <-second:
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("second resend also entered delivery instead of rejecting the pending operation")
	}
	require.Equal(t, int32(1), calls.Load())
	unblock()
	require.NoError(t, <-first)
}

type staleOTPRead struct {
	*fakeOTPRepo
	hook   func()
	called atomic.Bool
}

func (r *staleOTPRead) Get(ctx context.Context, id string) (*otp.Request, error) {
	value, err := r.fakeOTPRepo.Get(ctx, id)
	if err == nil && r.called.CompareAndSwap(false, true) {
		r.hook()
	}
	return value, err
}
func TestOTPVerifyCannotBeResetByResend(t *testing.T) {
	svc, repo, sender, _, _ := newServiceForTest(t)
	r := seedResend(t, repo)
	wrapped := &staleOTPRead{fakeOTPRepo: repo}
	svc.repo = wrapped
	wrapped.hook = func() {
		result, err := svc.Verify(context.Background(), otp.VerifyInput{RequestID: r.RequestID, Code: "123456"})
		require.NoError(t, err)
		require.True(t, result.Verified)
	}
	_, err := svc.Resend(context.Background(), r.RequestID)
	require.ErrorIs(t, err, otp.ErrAlreadyVerified)
	after, err := repo.Get(context.Background(), r.RequestID)
	require.NoError(t, err)
	require.True(t, after.Verified)
	require.Empty(t, sender.calls)
}
func TestOTPResendPreservesRecipientAndTemplate(t *testing.T) {
	svc, repo, sender, users, _ := newServiceForTest(t)
	users.byID["original-user"] = &user.User{UserID: "original-user", AppID: "app-a", Phone: "+14155551234"}
	out, err := svc.Send(context.Background(), otp.SendInput{AppID: "app-a", UserID: "original-user", Channel: otp.ChannelSMS, TemplateBody: "Branded {{code}}", TemplateData: map[string]interface{}{"brand": "Acme"}})
	require.NoError(t, err)
	r, err := repo.Get(context.Background(), out.RequestID)
	require.NoError(t, err)
	r.LastSentAt = time.Now().UTC().Add(-2 * time.Minute)
	require.NoError(t, repo.Update(context.Background(), r))
	_, err = svc.Resend(context.Background(), out.RequestID)
	require.NoError(t, err)
	require.Len(t, sender.calls, 2)
	require.Equal(t, "original-user", sender.calls[1].UserID)
	require.Equal(t, "Acme", sender.calls[1].Data["brand"])
}
func TestOTPRecipientBudgetFailureDoesNotDispatch(t *testing.T) {
	svc, repo, sender, _, _ := newServiceForTest(t)
	svc.repo = &otpBudgetFailure{fakeOTPRepo: repo}
	_, err := svc.Send(context.Background(), otp.SendInput{AppID: "app-a", Channel: otp.ChannelEmail, Recipient: "alice@example.test"})
	require.Error(t, err)
	require.Empty(t, sender.calls)
}

type otpBudgetFailure struct{ *fakeOTPRepo }

func (*otpBudgetFailure) RecipientRateLimit(context.Context, string, string, int) (int, error) {
	return 0, errors.New("budget unavailable")
}

func TestOTPEnqueueAckLoss(t *testing.T) {
	svc, repo, _, _, _ := newServiceForTest(t)
	var requestID, operationID, notificationID string
	calls := 0
	svc.notificationSvc = otpSenderFunc(func(ctx context.Context, in notification.SendRequest) (*notification.Notification, error) {
		calls++
		requestID, _ = in.Metadata["otp_request_id"].(string)
		operationID, _ = in.Metadata["otp_operation_id"].(string)
		notificationID, _ = notification.DispatchIdentityFromContext(ctx)
		return nil, &notification.DispatchError{NotificationID: notificationID, MayBeEnqueued: true, Cause: errors.New("lost enqueue receipt")}
	})
	_, err := svc.Send(context.Background(), otp.SendInput{AppID: "app-a", Channel: otp.ChannelEmail, Recipient: "alice@example.test"})
	require.Error(t, err)
	stored, err := repo.Get(context.Background(), requestID)
	require.NoError(t, err, "authority must survive uncertain enqueue")
	require.NotNil(t, stored.Delivery)
	require.Equal(t, "uncertain", stored.Delivery.State)
	_, err = svc.Resend(context.Background(), requestID)
	require.ErrorIs(t, err, otp.ErrOperationPending)
	require.Equal(t, 1, calls)
	require.NoError(t, svc.AuthorizeDelivery(context.Background(), otp.Scope{AppID: "app-a", EnvironmentID: "default"}, requestID, operationID, notificationID))
	require.NoError(t, svc.AuthorizeDelivery(context.Background(), otp.Scope{AppID: "app-a", EnvironmentID: "default"}, requestID, operationID, notificationID))
	stored, err = repo.Get(context.Background(), requestID)
	require.NoError(t, err)
	require.Equal(t, "accepted", stored.Delivery.State)
	require.Equal(t, 1, calls)
}
