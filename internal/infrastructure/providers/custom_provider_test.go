package providers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"go.uber.org/zap/zaptest"
)

func TestCustomProvider_UsesExplicitKind(t *testing.T) {
	logger := zaptest.NewLogger(t)
	var got map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		err = json.Unmarshal(body, &got)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewCustomProvider("cp", "webhook", "slack", server.URL, "", "v1", nil, logger)
	notif := &notification.Notification{
		NotificationID: "n1",
		Content: notification.Content{
			Title: "Title",
			Body:  "Body",
		},
	}

	result, err := p.Send(context.Background(), notif, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "Title\nBody", got["text"])
	_, hasStandardID := got["notification_id"]
	assert.False(t, hasStandardID)
}

func TestCustomProvider_UsesTeamsKindPayload(t *testing.T) {
	logger := zaptest.NewLogger(t)
	var got map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		err = json.Unmarshal(body, &got)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewCustomProvider("cp", "webhook", "teams", server.URL, "", "v1", nil, logger)
	notif := &notification.Notification{
		NotificationID: "n-teams",
		Content: notification.Content{
			Title: "Title",
			Body:  "Body",
		},
	}

	result, err := p.Send(context.Background(), notif, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "message", got["type"])
	_, hasAttachments := got["attachments"]
	assert.True(t, hasAttachments)
	_, hasStandardID := got["notification_id"]
	assert.False(t, hasStandardID)
}

func TestCustomProvider_SignatureHeaders(t *testing.T) {
	logger := zaptest.NewLogger(t)
	secret := "custom-secret"

	t.Run("v1 body hash", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)

			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(body)
			expected := hex.EncodeToString(mac.Sum(nil))

			assert.Equal(t, expected, r.Header.Get("X-Webhook-Signature"))
			assert.Equal(t, expected, r.Header.Get("X-FRN-Signature"))
			assert.Empty(t, r.Header.Get("X-Webhook-Timestamp"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		p := NewCustomProvider("cp", "webhook", "generic", server.URL, secret, "v1", nil, logger)
		_, err := p.Send(context.Background(), &notification.Notification{NotificationID: "n1"}, nil)
		assert.NoError(t, err)
	})

	t.Run("v2 timestamp and body hash", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)

			ts := r.Header.Get("X-Webhook-Timestamp")
			assert.NotEmpty(t, ts)
			_, parseErr := strconv.ParseInt(ts, 10, 64)
			assert.NoError(t, parseErr)

			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write([]byte(ts))
			mac.Write([]byte("."))
			mac.Write(body)
			expected := hex.EncodeToString(mac.Sum(nil))

			assert.Equal(t, expected, r.Header.Get("X-Webhook-Signature"))
			assert.Equal(t, expected, r.Header.Get("X-FRN-Signature"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		p := NewCustomProvider("cp", "webhook", "generic", server.URL, secret, "v2", nil, logger)
		_, err := p.Send(context.Background(), &notification.Notification{NotificationID: "n2"}, nil)
		assert.NoError(t, err)
	})
}

func TestInferProviderKindFromURL(t *testing.T) {
	cases := []struct {
		rawURL string
		want   string
	}{
		{rawURL: "https://discord.com/api/webhooks/123/abc", want: "discord"},
		{rawURL: "https://discordapp.com/api/webhooks/123/abc", want: "discord"},
		{rawURL: "https://hooks.slack.com/services/T/B/X", want: "slack"},
		{rawURL: "https://outlook.webhook.office.com/webhookb2/abc", want: "teams"},
		{rawURL: "https://prod-12.westus.logic.azure.com:443/workflows/abc/triggers/manual", want: "teams"},
		{rawURL: "https://example.com/hook", want: "generic"},
		{rawURL: "not-a-url", want: "generic"},
	}

	for _, tc := range cases {
		assert.Equal(t, tc.want, inferProviderKindFromURL(tc.rawURL))
	}
}
