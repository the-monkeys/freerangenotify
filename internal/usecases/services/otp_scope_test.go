package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/otp"
)

func TestOTPVerifiedRetryChecksCode(t *testing.T) {
	svc, repo, _, _, _ := newServiceForTest(t)
	now := time.Now().UTC()
	original := &otp.Request{RequestID: "otp-retry", AppID: "app-a", CodeHash: hashCode("123456", "salt"), Salt: "salt", Verified: true, VerifiedAt: &now, MaxAttempts: 5, Attempts: 1, ExpiresAt: now.Add(time.Minute)}
	require.NoError(t, repo.Create(context.Background(), original))
	result, err := svc.Verify(context.Background(), otp.VerifyInput{RequestID: original.RequestID, Code: "123456"})
	require.NoError(t, err)
	require.True(t, result.Verified)
	require.Equal(t, now, *result.VerifiedAt)
	_, err = svc.Verify(context.Background(), otp.VerifyInput{RequestID: original.RequestID, Code: "654321"})
	require.ErrorIs(t, err, otp.ErrInvalidCode)
	after, err := repo.Get(context.Background(), original.RequestID)
	require.NoError(t, err)
	require.Equal(t, 1, after.Attempts)
}

func TestOTPScopedOperations(t *testing.T) {
	for _, tc := range []struct {
		name, app, env, storedEnv string
		denied                    bool
	}{
		{"other app", "app-b", "default", "default", true},
		{"other environment", "app-a", "staging", "production", true},
		{"legacy same app", "app-a", "staging", "", false},
		{"same environment", "app-a", "production", "production", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, operation := range []string{"verify", "resend"} {
				t.Run(operation, func(t *testing.T) {
					svc, repo, sender, _, _ := newServiceForTest(t)
					original := &otp.Request{RequestID: "otp-scope", AppID: "app-a", EnvironmentID: tc.storedEnv, Channel: otp.ChannelEmail, Recipient: "alice@example.test", CodeHash: hashCode("123456", "salt"), Salt: "salt", MaxAttempts: 5, Length: 6, LastSentAt: time.Now().Add(-2 * time.Minute), ExpiresAt: time.Now().Add(time.Minute)}
					require.NoError(t, repo.Create(context.Background(), original))
					ctx := otp.WithScope(context.Background(), otp.Scope{AppID: tc.app, EnvironmentID: tc.env})
					var err error
					if operation == "verify" {
						_, err = svc.Verify(ctx, otp.VerifyInput{RequestID: original.RequestID, Code: "123456"})
					} else {
						_, err = svc.Resend(ctx, original.RequestID)
					}
					after, getErr := repo.Get(context.Background(), original.RequestID)
					require.NoError(t, getErr)
					require.Equal(t, original.ExpiresAt, after.ExpiresAt)
					if tc.denied {
						require.ErrorIs(t, err, otp.ErrNotFound)
						require.Equal(t, original, after)
						require.Empty(t, sender.calls)
					} else {
						require.NoError(t, err)
					}
				})
			}
		})
	}
}
