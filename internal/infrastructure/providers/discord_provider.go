package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers/render"
	"go.uber.org/zap"
)

// DiscordConfig holds configuration for the Discord provider.
type DiscordConfig struct {
	Config                   // Common: Timeout, MaxRetries, RetryDelay
	DefaultWebhookURL string // App-level fallback webhook URL
}

// DiscordProvider delivers notifications to Discord via Incoming Webhooks.
type DiscordProvider struct {
	config     DiscordConfig
	httpClient *http.Client
	logger     *zap.Logger
}

// NewDiscordProvider creates a new DiscordProvider.
func NewDiscordProvider(config DiscordConfig, logger *zap.Logger) (*DiscordProvider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &DiscordProvider{
		config:     config,
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
	}, nil
}

// Send delivers a notification to Discord.
func (p *DiscordProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	start := time.Now()

	webhookURL := p.resolveWebhookURL(notif, usr)
	if webhookURL == "" {
		return NewErrorResult(
			fmt.Errorf("no Discord webhook URL configured for user %s", notif.UserID),
			ErrorTypeInvalid,
		), nil
	}

	payload := p.buildPayload(notif)
	body, err := json.Marshal(payload)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to marshal Discord payload: %w", err), ErrorTypeInvalid), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to create Discord request: %w", err), ErrorTypeUnknown), nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewErrorResult(fmt.Errorf("Discord webhook request failed: %w", err), ErrorTypeNetwork), nil
	}
	defer resp.Body.Close()

	// Discord returns 204 No Content on success for webhook execution
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return NewErrorResult(
			fmt.Errorf("Discord returned status %d: %s", resp.StatusCode, string(respBody)),
			ErrorTypeProviderAPI,
		), nil
	}

	p.logger.Info("Discord notification delivered",
		zap.String("notification_id", notif.NotificationID),
		zap.Duration("delivery_time", time.Since(start)))

	result := NewResult("discord-"+notif.NotificationID, time.Since(start))
	result.Metadata["credential_source"] = CredSourceBYOC
	result.Metadata["billing_channel"] = "discord"
	return result, nil
}

// GetName returns the provider name.
func (p *DiscordProvider) GetName() string { return "discord" }

// GetSupportedChannel returns the channel this provider supports.
func (p *DiscordProvider) GetSupportedChannel() notification.Channel {
	return notification.ChannelDiscord
}

// IsHealthy checks if the provider is healthy.
func (p *DiscordProvider) IsHealthy(_ context.Context) bool { return true }

// Close releases provider resources.
func (p *DiscordProvider) Close() error { return nil }

// resolveWebhookURL determines the target Discord webhook.
// Priority: notification metadata > user-level > app-level default.
func (p *DiscordProvider) resolveWebhookURL(notif *notification.Notification, usr *user.User) string {
	if notif.Metadata != nil {
		if url, ok := notif.Metadata["discord_webhook_url"].(string); ok && url != "" {
			return url
		}
	}
	if usr != nil && usr.DiscordWebhookURL != "" {
		return usr.DiscordWebhookURL
	}
	return p.config.DefaultWebhookURL
}

func init() {
	RegisterFactory("discord", func(cfg map[string]interface{}, logger *zap.Logger) (Provider, error) {
		enabled, _ := cfg["enabled"].(bool)
		if !enabled {
			return nil, fmt.Errorf("discord: provider disabled")
		}
		webhookURL, _ := cfg["default_webhook_url"].(string)
		timeout := 10
		if t, ok := cfg["timeout"].(float64); ok && t > 0 {
			timeout = int(t)
		}
		maxRetries := 3
		if r, ok := cfg["max_retries"].(float64); ok {
			maxRetries = int(r)
		}
		return NewDiscordProvider(DiscordConfig{
			Config:            Config{Timeout: time.Duration(timeout) * time.Second, MaxRetries: maxRetries, RetryDelay: 2 * time.Second},
			DefaultWebhookURL: webhookURL,
		}, logger)
	})
}

// buildPayload constructs a Discord webhook payload with embeds.
func (p *DiscordProvider) buildPayload(notif *notification.Notification) map[string]interface{} {
	return render.BuildDiscordPayload(notif)
}
