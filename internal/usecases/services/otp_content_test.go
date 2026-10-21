package services

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/otp"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"strings"
	"testing"
)

func TestOTPContentUsesDirectSendAndSafeLogs(t *testing.T) {
	s, _, sender, _, templates := newServiceForTest(t)
	core, logs := observer.New(zap.DebugLevel)
	s.logger = zap.New(core)
	out, err := s.Send(context.Background(), otp.SendInput{AppID: "app-a", Channel: otp.ChannelEmail, Recipient: "alice@example.test", TemplateBody: "Welcome {{code}} from {{app_name}}", TemplateData: map[string]interface{}{"app_name": "Acme", "code": "attacker-value"}})
	require.NoError(t, err)
	require.Empty(t, templates.calls)
	require.Len(t, sender.calls, 1)
	request := sender.calls[0]
	require.Empty(t, request.TemplateID)
	require.Contains(t, request.Body, "Welcome ")
	require.Equal(t, "Acme", request.Data["app_name"])
	code := request.Data["code"].(string)
	require.NotEqual(t, "attacker-value", code)
	require.Contains(t, request.Body, code)
	raw, _ := json.Marshal(out)
	require.NotContains(t, string(raw), code)
	for _, entry := range logs.All() {
		raw, _ := json.Marshal(entry.ContextMap())
		require.False(t, strings.Contains(string(raw), code))
	}
}
