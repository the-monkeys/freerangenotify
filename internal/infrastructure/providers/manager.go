package providers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/metrics"
	"go.uber.org/zap"
)

// Manager manages multiple notification providers and routes notifications
type Manager struct {
	providers map[notification.Channel]Provider
	metrics   *metrics.NotificationMetrics
	logger    *zap.Logger
	mu        sync.RWMutex
}

// NewManager creates a new provider manager
func NewManager(metrics *metrics.NotificationMetrics, logger *zap.Logger) *Manager {
	return &Manager{
		providers: make(map[notification.Channel]Provider),
		metrics:   metrics,
		logger:    logger,
	}
}

// RegisterProvider registers a provider for a specific channel
func (m *Manager) RegisterProvider(provider Provider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	channel := provider.GetSupportedChannel()
	if _, exists := m.providers[channel]; exists {
		return fmt.Errorf("provider already registered for channel %s", channel)
	}

	m.providers[channel] = provider
	m.logger.Info("Provider registered",
		zap.String("provider", provider.GetName()),
		zap.String("channel", string(channel)))

	return nil
}

// GetProvider returns the provider for a specific channel
func (m *Manager) GetProvider(channel notification.Channel) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	provider, exists := m.providers[channel]
	if !exists {
		return nil, fmt.Errorf("no provider registered for channel %s", channel)
	}

	return provider, nil
}

// Send routes a notification to the appropriate provider and sends it
func (m *Manager) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	startTime := time.Now()

	// Get provider for channel
	provider, err := m.GetProvider(notif.Channel)
	if err != nil {
		m.logger.Error("Failed to get provider",
			zap.String("channel", string(notif.Channel)),
			zap.Error(err))
		return NewErrorResult(err, ErrorTypeInvalid), err
	}

	// Check provider health
	if !provider.IsHealthy(ctx) {
		err := fmt.Errorf("provider %s is unhealthy", provider.GetName())
		m.logger.Warn("Provider unhealthy",
			zap.String("provider", provider.GetName()),
			zap.String("notification_id", notif.NotificationID))
		return NewErrorResult(err, ErrorTypeProviderAPI), err
	}

	m.logger.Info("Routing notification to provider",
		zap.String("notification_id", notif.NotificationID),
		zap.String("channel", string(notif.Channel)),
		zap.String("provider", provider.GetName()))

	// Send notification
	result, err := provider.Send(ctx, notif, usr)

	// Record metrics
	if m.metrics != nil {
		providerLatency := time.Since(startTime).Seconds()
		m.metrics.RecordProviderLatency(provider.GetName(), string(notif.Channel), providerLatency)

		if result != nil && result.Success {
			m.metrics.RecordDeliverySuccess(string(notif.Channel), provider.GetName())
		} else if result != nil {
			m.metrics.RecordDeliveryFailure(string(notif.Channel), provider.GetName(), result.ErrorType)
		}
	}

	if err != nil {
		m.logger.Error("Provider failed to send notification",
			zap.String("notification_id", notif.NotificationID),
			zap.String("provider", provider.GetName()),
			zap.Error(err))
		return result, err
	}

	if result != nil && !result.Success {
		m.logger.Warn("Notification delivery failed",
			zap.String("notification_id", notif.NotificationID),
			zap.String("provider", provider.GetName()),
			zap.String("error_type", result.ErrorType),
			zap.Error(result.Error))
		return result, result.Error
	}

	m.logger.Info("Notification delivered successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.String("provider", provider.GetName()),
		zap.String("provider_message_id", result.ProviderMessageID),
		zap.Duration("delivery_time", result.DeliveryTime))

	return result, nil
}

// Close closes all registered providers
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error

	for channel, provider := range m.providers {
		if err := provider.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close provider for %s: %w", channel, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing providers: %v", errors)
	}

	return nil
}

// IsHealthy checks if all providers are healthy
func (m *Manager) IsHealthy(ctx context.Context) map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health := make(map[string]bool)

	for _, provider := range m.providers {
		health[provider.GetName()] = provider.IsHealthy(ctx)
	}

	return health
}

// GetSupportedChannels returns all channels that have providers registered
func (m *Manager) GetSupportedChannels() []notification.Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channels := make([]notification.Channel, 0, len(m.providers))
	for channel := range m.providers {
		channels = append(channels, channel)
	}

	return channels
}
