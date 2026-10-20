package notification

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWebhookTargetWithoutRecipient(t *testing.T) {
	n := &Notification{NotificationID: "n", AppID: "app", Channel: ChannelWebhook, Priority: PriorityNormal, Status: StatusQueued, Content: Content{Title: "Poll", Body: "Choose", Data: map[string]interface{}{"webhook_target": "registered-slack"}}}
	require.NoError(t, n.Validate(), "a registered target is a destination, not a recipient")
	for _, target := range []interface{}{"", 123, nil} {
		n.Content.Data["webhook_target"] = target
		require.ErrorIs(t, n.Validate(), ErrInvalidUserID)
	}
}
