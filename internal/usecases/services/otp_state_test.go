package services

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/otp"
	"testing"
)

func TestOTPDefiniteFailurePreservesCommittedCode(t *testing.T) {
	s, repo, sender, _, _ := newServiceForTest(t)
	before := seedResend(t, repo)
	sender.failNext = true
	_, err := s.Resend(context.Background(), before.RequestID)
	require.Error(t, err)
	after, err := repo.Get(context.Background(), before.RequestID)
	require.NoError(t, err)
	require.Equal(t, before.CodeHash, after.CodeHash)
	require.Equal(t, before.ExpiresAt, after.ExpiresAt)
	result, err := s.Verify(context.Background(), otp.VerifyInput{RequestID: before.RequestID, Code: "123456"})
	require.NoError(t, err)
	require.True(t, result.Verified)
}
func TestOTPSuppressedDispatchDoesNotSucceed(t *testing.T) {
	s, repo, _, _, _ := newServiceForTest(t)
	s.notificationSvc = otpSenderFunc(func(context.Context, notification.SendRequest) (*notification.Notification, error) { return nil, nil })
	_, err := s.Send(context.Background(), otp.SendInput{AppID: "app-a", Channel: otp.ChannelEmail, Recipient: "alice@example.test"})
	require.Error(t, err)
	for _, r := range repo.store {
		require.Empty(t, r.CodeHash)
		require.Nil(t, r.Delivery)
	}
}

type lostCASReceipt struct {
	*fakeOTPRepo
	lost bool
}

func (r *lostCASReceipt) CompareAndSwap(ctx context.Context, old, next *otp.Request) error {
	err := r.fakeOTPRepo.CompareAndSwap(ctx, old, next)
	if err == nil && !r.lost {
		r.lost = true
		return errors.New("receipt lost")
	}
	return err
}
func TestOTPClaimAckLossDoesNotDispatch(t *testing.T) {
	s, repo, sender, _, _ := newServiceForTest(t)
	before := seedResend(t, repo)
	s.repo = &lostCASReceipt{fakeOTPRepo: repo}
	_, err := s.Resend(context.Background(), before.RequestID)
	require.ErrorIs(t, err, otp.ErrStateUnavailable)
	require.Empty(t, sender.calls)
	_, err = s.Resend(context.Background(), before.RequestID)
	require.ErrorIs(t, err, otp.ErrOperationPending)
	after, err := repo.Get(context.Background(), before.RequestID)
	require.NoError(t, err)
	require.Equal(t, before.ExpiresAt, after.ExpiresAt)
}
