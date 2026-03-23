package providers

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

// InAppProvider implements Provider for the in_app channel.
// in_app notifications are already persisted to Elasticsearch by the API
// service before they are enqueued to the worker. The worker only needs to
// mark them as "sent" — there is no external delivery step.
type InAppProvider struct {
	logger *zap.Logger
}

// NewInAppProvider creates a new InAppProvider.
func NewInAppProvider(logger *zap.Logger) Provider {
	return &InAppProvider{logger: logger}
}

// Send is a no-op for in_app: the notification is already stored in ES.
// Returning a successful Result tells the worker to mark it as "sent".
func (p *InAppProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	p.logger.Debug("In-app notification acknowledged (already persisted)",
		zap.String("notification_id", notif.NotificationID),
		zap.String("user_id", usr.UserID))
	return NewResult("", time.Duration(0)), nil
}

// GetName returns the provider name.
func (p *InAppProvider) GetName() string {
	return "in_app"
}

// GetSupportedChannel returns ChannelInApp.
func (p *InAppProvider) GetSupportedChannel() notification.Channel {
	return notification.ChannelInApp
}

// IsHealthy always returns true — no external dependency.
func (p *InAppProvider) IsHealthy(ctx context.Context) bool {
	return true
}

// Close is a no-op.
func (p *InAppProvider) Close() error {
	return nil
}
