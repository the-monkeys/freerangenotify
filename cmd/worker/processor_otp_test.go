package main

import (
	"context"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/otp"
	"testing"
)

type otpAuthorizerFunc func(context.Context, otp.Scope, string, string, string) error

func (f otpAuthorizerFunc) AuthorizeDelivery(c context.Context, s otp.Scope, r, o, n string) error {
	return f(c, s, r, o, n)
}

func TestOTPWorkerAuthorization(t *testing.T) {
	p := &NotificationProcessor{}
	legacy := &notification.Notification{AppID: "app-a"}
	require.NoError(t, p.authorizeOTPDelivery(context.Background(), legacy))
	n := &notification.Notification{AppID: "app-a", EnvironmentID: "staging", NotificationID: "n", Metadata: map[string]interface{}{"otp_request_id": "r", "otp_operation_id": "o"}}
	require.ErrorIs(t, p.authorizeOTPDelivery(context.Background(), n), otp.ErrStateUnavailable)
	calls := 0
	p.SetOTPDeliveryAuthorizer(otpAuthorizerFunc(func(_ context.Context, s otp.Scope, r, o, id string) error {
		calls++
		require.Equal(t, otp.Scope{AppID: "app-a", EnvironmentID: "staging"}, s)
		require.Equal(t, "r", r)
		require.Equal(t, "o", o)
		require.Equal(t, "n", id)
		return otp.ErrNotFound
	}))
	require.ErrorIs(t, p.authorizeOTPDelivery(context.Background(), n), otp.ErrNotFound)
	require.Equal(t, 1, calls)
}
