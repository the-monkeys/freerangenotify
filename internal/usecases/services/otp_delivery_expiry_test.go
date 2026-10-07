package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/otp"
)

func TestOTPDispatch_CarriesExpiryToWorker(t *testing.T) {
	svc, _, sender, _, _ := newServiceForTest(t)
	result, err := svc.Send(context.Background(), otp.SendInput{
		AppID: "app-1", Channel: otp.ChannelEmail, Recipient: "alice@example.com", TTLSeconds: 60,
	})
	require.NoError(t, err)
	require.Len(t, sender.calls, 1)
	value, ok := sender.calls[0].Metadata["otp_expires_at"].(string)
	require.True(t, ok)
	deadline, err := time.Parse(time.RFC3339Nano, value)
	require.NoError(t, err)
	require.True(t, deadline.Equal(result.ExpiresAt))
}
