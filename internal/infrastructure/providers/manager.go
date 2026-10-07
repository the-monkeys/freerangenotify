package providers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/metrics"
	"go.uber.org/zap"
)

// Manager manages multiple notification providers and routes notifications
type Manager struct {
	providers      map[notification.Channel]Provider
	namedProviders map[string]Provider
	breakers       map[string]*CircuitBreaker
	metrics        *metrics.NotificationMetrics
	presenceRepo   user.PresenceRepository
	logger         *zap.Logger
	mu             sync.RWMutex

	// Billing — both fields are nil-safe when billing is disabled.
	billingEnabled bool
	usageEmitter   billing.UsageEmitter
}

// NewManager creates a new provider manager
func NewManager(metrics *metrics.NotificationMetrics, presenceRepo user.PresenceRepository, logger *zap.Logger) *Manager {
	return &Manager{
		providers:      make(map[notification.Channel]Provider),
		namedProviders: make(map[string]Provider),
		breakers:       make(map[string]*CircuitBreaker),
		metrics:        metrics,
		presenceRepo:   presenceRepo,
		logger:         logger,
	}
}

// WithBillingEmitter configures the Manager to emit usage events after each
// successful Send(). This is a no-op when billingEnabled is false, preserving
// full backward compatibility for local development.
func (m *Manager) WithBillingEmitter(enabled bool, emitter billing.UsageEmitter) *Manager {
	m.billingEnabled = enabled
	m.usageEmitter = emitter
	return m
}

// RegisterProvider registers a provider for a specific channel
func (m *Manager) RegisterProvider(provider Provider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	channel := provider.GetSupportedChannel()
	name := provider.GetName()
	namedKey := fmt.Sprintf("%s-%s", name, channel)

	// Register as default for this channel if none exists
	if _, exists := m.providers[channel]; !exists {
		m.providers[channel] = provider
	}

	// Register by name
	m.namedProviders[namedKey] = provider
	m.breakers[namedKey] = NewCircuitBreaker(namedKey, 5, 30*time.Second, m.logger)

	m.logger.Info("Provider registered with circuit breaker",
		zap.String("provider", name),
		zap.String("channel", string(channel)),
		zap.String("key", namedKey))

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

	// 1. Smart Delivery: Check for dynamic routing presence (only if we have a user)
	if m.presenceRepo != nil && usr != nil {
		available, dynamicURL, err := m.presenceRepo.IsAvailable(ctx, usr.UserID)
		if err == nil && available && dynamicURL != "" {
			m.logger.Info("Smart Delivery: Overriding static webhook with dynamic URL",
				zap.String("user_id", usr.UserID),
				zap.String("static_url", usr.WebhookURL),
				zap.String("dynamic_url", dynamicURL))

			// Create a copy of the user to avoid side effects on other goroutines/cache
			userCopy := *usr
			userCopy.WebhookURL = dynamicURL
			usr = &userCopy
		}
	}

	// 2. Resolve provider
	provider, err := m.resolveProvider(ctx, notif.Channel, "")
	if err != nil {
		m.logger.Error("Failed to get provider",
			zap.String("channel", string(notif.Channel)),
			zap.Error(err))
		result, failure := deliveryFailure(nil, err, "provider", "configuration", "", ErrorTypeInvalid)
		return result, failure
	}

	m.logger.Info("Routing notification to provider",
		zap.String("notification_id", notif.NotificationID),
		zap.String("channel", string(notif.Channel)),
		zap.String("provider", provider.GetName()))

	result, err := m.sendProvider(ctx, provider, notif, usr)

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

	// Emit billing usage event — only when billing is enabled and emitter is wired.
	m.emitUsageEvent(notif, result, provider.GetName())

	return result, nil
}

// SendWithFallback tries an ordered list of providers for a channel,
// falling back to the next provider if the current one fails.
func (m *Manager) SendWithFallback(ctx context.Context, notif *notification.Notification, usr *user.User, providerNames []string) (*Result, error) {
	startTime := time.Now()

	// Smart Delivery: dynamic routing (same as Send)
	if m.presenceRepo != nil && usr != nil {
		available, dynamicURL, err := m.presenceRepo.IsAvailable(ctx, usr.UserID)
		if err == nil && available && dynamicURL != "" {
			m.logger.Info("Smart Delivery: Overriding static webhook with dynamic URL (fallback path)",
				zap.String("user_id", usr.UserID),
				zap.String("dynamic_url", dynamicURL))
			userCopy := *usr
			userCopy.WebhookURL = dynamicURL
			usr = &userCopy
		}
	}

	var lastErr error
	var lastResolutionErr error
	for i, providerName := range providerNames {
		if ctx.Err() != nil {
			lastErr = newDeliveryError(providerName, "send", "", "", ctx.Err())
			break
		}
		provider, resolveErr := m.resolveProvider(ctx, notif.Channel, providerName)
		if resolveErr != nil {
			lastResolutionErr = resolveErr
			m.logger.Warn("Fallback provider not registered, skipping",
				zap.String("provider", providerName),
				zap.String("channel", string(notif.Channel)))
			continue
		}

		result, err := m.sendProvider(ctx, provider, notif, usr)

		// Record metrics
		if m.metrics != nil {
			latency := time.Since(startTime).Seconds()
			m.metrics.RecordProviderLatency(providerName, string(notif.Channel), latency)
			if result != nil && result.Success {
				m.metrics.RecordDeliverySuccess(string(notif.Channel), providerName)
			} else if result != nil {
				m.metrics.RecordDeliveryFailure(string(notif.Channel), providerName, result.ErrorType)
			}
		}

		if err == nil && result != nil && result.Success {
			m.emitUsageEvent(notif, result, providerName)
			if i > 0 {
				m.logger.Info("Delivery succeeded via fallback provider",
					zap.String("notification_id", notif.NotificationID),
					zap.String("provider", providerName),
					zap.Int("attempt", i+1))
			}
			return result, nil
		}

		lastErr = err
		if result != nil && result.Error != nil {
			lastErr = result.Error
		}
		m.logger.Warn("Fallback provider failed, trying next",
			zap.String("notification_id", notif.NotificationID),
			zap.String("provider", providerName),
			zap.Int("attempt", i+1),
			zap.Error(lastErr))
	}

	if lastErr == nil {
		// Missing adapters were skipped, so their configuration errors must
		// not erase a failure from a provider whose delivery was attempted.
		lastErr = lastResolutionErr
	}
	if lastErr == nil {
		lastErr = newDeliveryError("provider", "configuration", "", ErrorTypeConfiguration, fmt.Errorf("no fallback providers configured"))
	}
	err := fmt.Errorf("all fallback providers failed: %w", lastErr)
	result, _ := deliveryFailure(nil, err, "provider", "send", "", ErrorTypeProviderAPI)
	// Preserve the fallback wrapper and its typed cause on both surfaces.
	result.Error = err
	return result, err
}

// resolveProvider handles explicit application SMTP before asking for a channel
// default. Request-created adapters never enter the manager's shared maps.
func (m *Manager) resolveProvider(ctx context.Context, channel notification.Channel, requested string) (Provider, error) {
	app, _ := ctx.Value(EmailConfigKey).(*application.EmailConfig)
	name := requested
	if name == "" && channel == notification.ChannelEmail && app != nil && app.ProviderType != "system" {
		name = app.ProviderType
	}
	m.mu.RLock()
	named := m.namedProviders[fmt.Sprintf("%s-%s", name, channel)]
	m.mu.RUnlock()
	if named != nil {
		return named, nil
	}
	if name == "smtp" && channel == notification.ChannelEmail && app != nil && app.ProviderType == "smtp" && app.SMTP != nil {
		cfg := app.SMTP
		factory := GetFactory("smtp")
		if factory == nil {
			return nil, newDeliveryError("smtp", "configuration", CredSourceBYOC, ErrorTypeConfiguration, fmt.Errorf("SMTP factory unavailable"))
		}
		provider, err := factory(map[string]interface{}{"host": cfg.Host, "port": cfg.Port, "username": cfg.Username, "password": cfg.Password, "from_email": cfg.FromEmail, "from_name": cfg.FromName}, m.logger)
		if err != nil {
			return nil, newDeliveryError("smtp", "configuration", CredSourceBYOC, ErrorTypeConfiguration, err)
		}
		return provider, nil
	}
	// A configured explicit choice must never silently become the channel
	// default. Legacy empty/incomplete SMTP and SendGrid choices keep their
	// previous default-provider behavior.
	if channel == notification.ChannelEmail && app != nil && name == app.ProviderType && name != "" && name != "system" {
		configured := true
		switch name {
		case "smtp":
			configured = app.SMTP != nil
		case "sendgrid":
			configured = app.SendGrid != nil
		}
		if configured {
			return nil, newDeliveryError(name, "configuration", CredSourceBYOC, ErrorTypeConfiguration, fmt.Errorf("explicit email provider is not available"))
		}
	}
	if requested != "" {
		return nil, newDeliveryError(requested, "configuration", "", ErrorTypeConfiguration, fmt.Errorf("fallback provider is not registered"))
	}
	return m.GetProvider(channel)
}

func credentialSource(ctx context.Context, provider Provider) string {
	// Custom adapters have tenant-supplied names rather than a fixed "custom"
	// provider name. Their requests must also stay outside shared breakers.
	if _, ok := provider.(*CustomProvider); ok {
		return CredSourceBYOC
	}
	name := provider.GetName()
	if provider.GetSupportedChannel() == notification.ChannelEmail {
		if app, ok := ctx.Value(EmailConfigKey).(*application.EmailConfig); ok && app != nil {
			if name == "smtp" && app.ProviderType == "smtp" && app.SMTP != nil {
				return CredSourceBYOC
			}
			if name == "sendgrid" && app.ProviderType == "sendgrid" && app.SendGrid != nil {
				return CredSourceBYOC
			}
		}
	}
	if provider.GetSupportedChannel() == notification.ChannelSMS && name == "twilio" {
		if app, ok := ctx.Value(SMSConfigKey).(*application.SMSAppConfig); ok && app != nil && app.AccountSID != "" && app.AuthToken != "" {
			return CredSourceBYOC
		}
	}
	if provider.GetSupportedChannel() == notification.ChannelWhatsApp {
		if app, ok := ctx.Value(WhatsAppConfigKey).(*application.WhatsAppAppConfig); ok && app != nil {
			if name == "meta_whatsapp" && app.Provider == "meta" && app.MetaPhoneNumberID != "" && app.MetaAccessToken != "" {
				return CredSourceBYOC
			}
			if name == "whatsapp" && app.AccountSID != "" && app.AuthToken != "" {
				return CredSourceBYOC
			}
		}
	}
	switch name {
	case "webhook", "slack", "discord", "teams", "whatsapp_self_hosted", "custom":
		return CredSourceBYOC
	case "inapp", "sse", "fcm", "apns":
		return CredSourcePlatform
	}
	return CredSourceSystem
}

func deliveryFailure(result *Result, cause error, provider, stage, source, category string) (*Result, error) {
	if cause == nil {
		cause = fmt.Errorf("provider returned an unsuccessful result without a cause")
	}
	failure := newDeliveryError(provider, stage, source, category, cause)
	if result == nil {
		result = NewErrorResult(failure, failure.ErrorType)
	}
	result.Success, result.Error, result.ErrorType = false, failure, failure.ErrorType
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}
	for key, value := range DeliveryErrorMetadata(failure) {
		result.Metadata[key] = value
	}
	return result, failure
}

func (m *Manager) sendProvider(ctx context.Context, provider Provider, notif *notification.Notification, usr *user.User) (*Result, error) {
	source := credentialSource(ctx, provider)
	if err := ctx.Err(); err != nil {
		return deliveryFailure(nil, err, provider.GetName(), "send", source, "")
	}
	var healthErr error
	if checker, ok := provider.(HealthChecker); ok {
		healthErr = checker.CheckHealth(ctx)
	} else if !provider.IsHealthy(ctx) {
		healthErr = fmt.Errorf("provider is unhealthy")
	}
	if healthErr != nil {
		return deliveryFailure(nil, healthErr, provider.GetName(), "health", source, ErrorTypeProviderAPI)
	}
	var result *Result
	send := func() error {
		var err error
		result, err = provider.Send(ctx, notif, usr)
		if err != nil || result == nil || !result.Success {
			category := ErrorTypeUnknown
			if result != nil {
				category = result.ErrorType
				if err == nil {
					err = result.Error
				}
			}
			result, err = deliveryFailure(result, err, provider.GetName(), "send", source, category)
		}
		return err
	}
	m.mu.RLock()
	breaker := m.breakers[fmt.Sprintf("%s-%s", provider.GetName(), notif.Channel)]
	m.mu.RUnlock()
	var err error
	if breaker == nil || source == CredSourceBYOC {
		err = send()
	} else {
		err = breaker.Execute(send)
	}
	if err != nil && result == nil {
		return deliveryFailure(nil, err, provider.GetName(), "breaker", source, ErrorTypeProviderAPI)
	}
	return result, err
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

// ProviderHealth represents the health status of a single provider.
type ProviderHealth struct {
	Name         string `json:"name"`
	Channel      string `json:"channel"`
	Healthy      bool   `json:"healthy"`
	BreakerState string `json:"breaker_state"` // closed, open, half-open
}

// HealthStatus returns per-channel health including circuit breaker state.
func (m *Manager) HealthStatus() map[string]ProviderHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]ProviderHealth)
	for channel, provider := range m.providers {
		breakerKey := fmt.Sprintf("%s-%s", provider.GetName(), channel)
		breakerState := "closed"
		if b, ok := m.breakers[breakerKey]; ok {
			breakerState = string(b.GetState())
		}
		result[string(channel)] = ProviderHealth{
			Name:         provider.GetName(),
			Channel:      string(channel),
			Healthy:      provider.IsHealthy(context.Background()),
			BreakerState: breakerState,
		}
	}
	return result
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

func (m *Manager) emitUsageEvent(notif *notification.Notification, result *Result, providerName string) {
	if !m.billingEnabled || m.usageEmitter == nil || notif == nil || result == nil {
		return
	}

	credSource := ""
	billingChannel := ""
	var creditsUsed int64
	rateCardVersion := ""

	if result.Metadata != nil {
		if v, ok := result.Metadata["credential_source"].(string); ok {
			credSource = v
		}
		if v, ok := result.Metadata["billing_channel"].(string); ok {
			billingChannel = v
		}
		if v, ok := result.Metadata["rate_card_version"].(string); ok {
			rateCardVersion = v
		}
		if v, ok := result.Metadata["credits_used"].(int64); ok {
			creditsUsed = v
		} else if v, ok := result.Metadata["credits_used"].(float64); ok {
			creditsUsed = int64(v)
		}
	}
	if notif.Metadata != nil {
		if credSource == "" {
			if v, ok := notif.Metadata["credential_source"].(string); ok {
				credSource = v
			}
		}
		if billingChannel == "" {
			if v, ok := notif.Metadata["billing_channel"].(string); ok {
				billingChannel = v
			}
		}
		if rateCardVersion == "" {
			if v, ok := notif.Metadata["rate_card_version"].(string); ok {
				rateCardVersion = v
			}
		}
		if creditsUsed == 0 {
			if v, ok := notif.Metadata["credits_used"].(int64); ok {
				creditsUsed = v
			} else if v, ok := notif.Metadata["credits_used"].(float64); ok {
				creditsUsed = int64(v)
			}
		}
	}

	if credSource == "" || billingChannel == "" {
		return
	}

	event := &billing.UsageEvent{
		TenantID:         notif.AppID,
		AppID:            notif.AppID,
		NotificationID:   notif.NotificationID,
		Channel:          billingChannel,
		Provider:         providerName,
		CredentialSource: credSource,
		MessageType:      string(notif.Category),
		CreditsUsed:      creditsUsed,
		RateCardVersion:  rateCardVersion,
		Currency:         "INR",
		Status:           "charged",
	}
	go func() {
		if err := m.usageEmitter.Emit(context.Background(), event); err != nil {
			m.logger.Error("billing: failed to emit usage event",
				zap.String("notification_id", notif.NotificationID),
				zap.Error(err))
		}
	}()
}
