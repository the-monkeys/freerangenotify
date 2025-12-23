package providers

import (
	"context"
	"errors"
	"net/smtp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap/zaptest"
)

func TestSMTPProvider_Send(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := SMTPConfig{
		Config: Config{
			Timeout:    1 * time.Second,
			MaxRetries: 1,
			RetryDelay: 0,
		},
		Host:      "smtp.example.com",
		Port:      587,
		Username:  "user",
		Password:  "pass",
		FromEmail: "from@example.com",
		FromName:  "Test Sender",
	}

	t.Run("success", func(t *testing.T) {
		provider, err := NewSMTPProvider(config, logger)
		assert.NoError(t, err)

		smtpProvider := provider.(*SMTPProvider)

		// Mock sender
		sent := false
		smtpProvider.sender = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
			assert.Equal(t, "smtp.example.com:587", addr)
			assert.Equal(t, "from@example.com", from)
			assert.Equal(t, []string{"user@example.com"}, to)
			assert.Contains(t, string(msg), "Subject: Test Subject")
			assert.Contains(t, string(msg), "Test Body")
			sent = true
			return nil
		}

		notif := &notification.Notification{
			NotificationID: "123",
			Content: notification.Content{
				Title: "Test Subject",
				Body:  "Test Body",
			},
		}
		usr := &user.User{
			UserID: "user1",
			Email:  "user@example.com",
		}

		result, err := provider.Send(context.Background(), notif, usr)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.True(t, sent)
	})

	t.Run("retry logic", func(t *testing.T) {
		provider, err := NewSMTPProvider(config, logger)
		assert.NoError(t, err)

		smtpProvider := provider.(*SMTPProvider)

		attempts := 0
		smtpProvider.sender = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
			attempts++
			if attempts < 2 {
				return errors.New("temporary error")
			}
			return nil
		}

		notif := &notification.Notification{NotificationID: "123"}
		usr := &user.User{Email: "user@example.com"}

		result, err := provider.Send(context.Background(), notif, usr)
		assert.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 2, attempts)
	})
}
