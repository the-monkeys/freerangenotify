package providers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCustomProviderPollFallbackContract(t *testing.T) {
	for _, tc := range []struct {
		name, body    string
		status, calls int
	}{
		{"recognized", `{"proto_data":["poll"]}`, 400, 2},
		{"nested", `{"errors":{"proto_data":["poll"]}}`, 400, 2},
		{"unrelated text", `{"message":"proto_data contains another error; poll is unrelated"}`, 400, 1},
		{"non JSON", `proto_data poll`, 400, 1},
		{"wrong field", `{"proto_data":["not_poll"]}`, 400, 1},
		{"upstream failure", `{"proto_data":["poll"]}`, 500, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			requests := make(chan map[string]interface{}, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				requests <- body
				if calls.Add(1) == 1 {
					w.WriteHeader(tc.status)
					io.WriteString(w, tc.body)
				} else {
					w.WriteHeader(200)
				}
			}))
			defer server.Close()
			p := NewCustomProvider("fixture", "webhook", "discord", server.URL, "", "v1", nil, zap.NewNop())
			n := &notification.Notification{Content: notification.Content{Title: "Plan", Body: "Choose", Poll: &notification.Poll{Question: "When?", Choices: []notification.PollChoice{{Label: "Morning"}, {Label: "Afternoon"}}, DurationHours: 24}}}
			result, err := p.Send(context.Background(), n, nil)
			require.NoError(t, err)
			require.Equal(t, int32(tc.calls), calls.Load())
			first := <-requests
			require.Contains(t, first, "poll")
			if tc.calls == 2 {
				require.True(t, result.Success)
				second := <-requests
				require.NotContains(t, second, "poll")
				require.Contains(t, second, "embeds")
			} else {
				require.False(t, result.Success)
			}
		})
	}
}
func TestWebhookRetryPreservesBodyAndSignature(t *testing.T) {
	type request struct {
		body      []byte
		signature string
	}
	requests := make(chan request, 2)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests <- request{body, r.Header.Get("X-Webhook-Signature")}
		if calls.Add(1) == 1 {
			w.WriteHeader(503)
		} else {
			w.WriteHeader(204)
		}
	}))
	defer server.Close()
	p, err := NewWebhookProvider(WebhookConfig{Config: Config{Timeout: time.Second, MaxRetries: 1, RetryDelay: time.Millisecond}, Secret: "fixture-secret"}, zap.NewNop())
	require.NoError(t, err)
	n := &notification.Notification{NotificationID: "fixture", Content: notification.Content{Title: "Update", Body: "Ready"}, Metadata: map[string]interface{}{"webhook_url": server.URL}}
	result, err := p.Send(context.Background(), n, nil)
	require.NoError(t, err)
	require.True(t, result.Success, "%v", result.Error)
	require.Equal(t, int32(2), calls.Load())
	first, second := <-requests, <-requests
	require.Equal(t, first.body, second.body)
	require.NotEmpty(t, second.body)
	mac := hmac.New(sha256.New, []byte("fixture-secret"))
	mac.Write(second.body)
	require.Equal(t, hex.EncodeToString(mac.Sum(nil)), second.signature)
}
