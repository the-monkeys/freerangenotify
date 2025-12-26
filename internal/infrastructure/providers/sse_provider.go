package providers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/sse"
	"go.uber.org/zap"
)

// SSEProvider implements the Provider interface for Server-Sent Events
type SSEProvider struct {
	config Config
	logger *zap.Logger
	redis  *redis.Client
}

// SSEConfig holds SSE-specific configuration
type SSEConfig struct {
	Config
}

// NewSSEProvider creates a new SSE provider
func NewSSEProvider(config SSEConfig, redisClient *redis.Client, logger *zap.Logger) (Provider, error) {
	return &SSEProvider{
		config: config.Config,
		logger: logger,
		redis:  redisClient,
	}, nil
}

// Send sends a notification via SSE (Server-Sent Events)
func (p *SSEProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	startTime := time.Now()

	p.logger.Info("Sending SSE notification",
		zap.String("notification_id", notif.NotificationID),
		zap.String("user_id", usr.UserID))

	// Create SSE message
	sseMsg := &sse.SSEMessage{
		Type:         "notification",
		UserID:       usr.UserID,
		Notification: notif,
	}

	// Serialize message
	data, err := json.Marshal(sseMsg)
	if err != nil {
		p.logger.Error("Failed to marshal SSE message", zap.Error(err))
		return NewErrorResult(err, ErrorTypeInvalid), nil
	}

	// Publish to Redis for broadcasting
	p.logger.Debug("Publishing SSE message to Redis",
		zap.String("channel", "sse:notifications"),
		zap.String("payload", string(data)))

	if err := p.redis.Publish(ctx, "sse:notifications", data).Err(); err != nil {
		p.logger.Error("Failed to publish SSE message", zap.Error(err))
		return NewErrorResult(err, ErrorTypeProviderAPI), nil
	}

	processingTime := time.Since(startTime)
	p.logger.Info("SSE notification sent successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.String("user_id", usr.UserID),
		zap.Duration("processing_time", processingTime))

	return NewResult("", processingTime), nil
}

// GetName returns the provider name
func (p *SSEProvider) GetName() string {
	return "SSE"
}

// GetSupportedChannel returns the channel this provider supports
func (p *SSEProvider) GetSupportedChannel() notification.Channel {
	return notification.ChannelSSE
}

// IsHealthy checks if the provider is healthy
func (p *SSEProvider) IsHealthy(ctx context.Context) bool {
	// SSE provider is healthy if Redis is available
	return p.redis.Ping(ctx).Err() == nil
}

// Close closes the provider
func (p *SSEProvider) Close() error {
	// No resources to close
	return nil
}
