package usecases

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"testing"
)

func TestOTPDispatchQueueFailureIsUncertain(t *testing.T) {
	repo := &mockNotifRepoSend{}
	q := &mockQueueSend{enqueueErr: errors.New("ack lost")}
	svc := newSendServiceForTest(defaultUsers(), defaultApps(), repo, q, nil)
	_, err := svc.Send(notification.WithDispatchIdentity(context.Background(), "otp-notification"), notification.SendRequest{AppID: "app-1", UserID: "user-1", Channel: notification.ChannelEmail, Title: "Verification", Body: "Code", Priority: notification.PriorityHigh})
	var uncertain *notification.DispatchError
	require.ErrorAs(t, err, &uncertain)
	require.True(t, uncertain.MayBeEnqueued)
	require.Equal(t, "otp-notification", uncertain.NotificationID)
	require.Len(t, repo.created, 1)
	require.Equal(t, "otp-notification", repo.created[0].NotificationID)
	require.NotEqual(t, notification.StatusFailed, repo.statusUpdate["otp-notification"])
}
