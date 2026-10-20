package usecases

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"testing"
)

func TestNotificationSensitiveLogging(t *testing.T) {
	s := newSendServiceForTest(defaultUsers(), defaultApps(), nil, nil, nil)
	core, logs := observer.New(zap.DebugLevel)
	s.logger = zap.New(core)
	_, err := s.Send(context.Background(), notification.SendRequest{AppID: "app-1", UserID: "user-1", Channel: notification.ChannelEmail, Title: "Verification", Body: "Secret 583917", Data: map[string]interface{}{"code": "583917"}})
	require.NoError(t, err)
	for _, entry := range logs.All() {
		raw, _ := json.Marshal(entry.ContextMap())
		require.NotContains(t, string(raw), "583917")
	}
}
