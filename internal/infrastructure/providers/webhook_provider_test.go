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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap/zaptest"
)

func TestWebhookProvider_Send(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("success with signature", func(t *testing.T) {
		// Mock receiver
		secret := "my-secret-key"
		received := false

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "FreeRangeNotify-Webhook/1.0", r.Header.Get("User-Agent"))
			assert.Equal(t, "123", r.Header.Get("X-Notification-ID"))

			// Verify Body
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			var payload notification.Notification
			err = json.Unmarshal(body, &payload)
			assert.NoError(t, err)
			assert.Equal(t, "123", payload.NotificationID)

			// Verify Signature
			signature := r.Header.Get("X-Webhook-Signature")
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(body)
			expectedSignature := hex.EncodeToString(mac.Sum(nil))
			assert.Equal(t, expectedSignature, signature)

			received = true
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		provider, err := NewWebhookProvider(WebhookConfig{
			Config: Config{
				Timeout:    1 * time.Second,
				MaxRetries: 1,
			},
			Secret: secret,
		}, logger)
		assert.NoError(t, err)

		notif := &notification.Notification{
			NotificationID: "123",
			Content: notification.Content{
				Title: "Test",
			},
		}
		usr := &user.User{
			UserID:     "user1",
			WebhookURL: server.URL,
		}

		result, err := provider.Send(context.Background(), notif, usr)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Equal(t, server.URL, result.Metadata["url"])
		assert.True(t, received)
	})

	t.Run("retry on failure", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		provider, err := NewWebhookProvider(WebhookConfig{
			Config: Config{
				Timeout:    1 * time.Second,
				MaxRetries: 2,
				RetryDelay: 10 * time.Millisecond,
			},
		}, logger)
		assert.NoError(t, err)

		notif := &notification.Notification{NotificationID: "retry-test"}
		usr := &user.User{WebhookURL: server.URL}

		result, err := provider.Send(context.Background(), notif, usr)
		assert.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 2, attempts)
	})

	t.Run("missing url", func(t *testing.T) {
		provider, err := NewWebhookProvider(WebhookConfig{}, logger)
		assert.NoError(t, err)

		notif := &notification.Notification{NotificationID: "123"}
		usr := &user.User{UserID: "user1"} // No WebhookURL

		result, err := provider.Send(context.Background(), notif, usr)
		assert.NoError(t, err) // Should not error, but return ErrorResult
		assert.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Equal(t, ErrorTypeInvalid, result.ErrorType)
	})
}
