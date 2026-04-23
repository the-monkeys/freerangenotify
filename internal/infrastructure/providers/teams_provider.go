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

// TeamsConfig holds Microsoft Teams-specific configuration.
type TeamsConfig struct {
	Config
	DefaultWebhookURL string
}

// TeamsProvider implements the Provider interface for Microsoft Teams via Incoming Webhooks.
type TeamsProvider struct {
	config     TeamsConfig
	httpClient *http.Client
	logger     *zap.Logger
}

// NewTeamsProvider creates a new Teams provider.
func NewTeamsProvider(config TeamsConfig, logger *zap.Logger) (Provider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &TeamsProvider{
		config:     config,
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
	}, nil
}

// Send delivers a notification to Microsoft Teams via an Incoming Webhook.
func (p *TeamsProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	start := time.Now()

	webhookURL := p.resolveWebhookURL(notif)
	if webhookURL == "" {
		return NewErrorResult(
			fmt.Errorf("no Teams webhook URL configured for notification %s", notif.NotificationID),
			ErrorTypeInvalid,
		), nil
	}

	payload := p.buildAdaptiveCard(notif, webhookURL)
	body, err := json.Marshal(payload)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to marshal Teams payload: %w", err), ErrorTypeInvalid), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to create Teams request: %w", err), ErrorTypeUnknown), nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewErrorResult(fmt.Errorf("Teams webhook request failed: %w", err), ErrorTypeNetwork), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return NewErrorResult(
			fmt.Errorf("Teams returned status %d: %s", resp.StatusCode, string(respBody)),
			ErrorTypeProviderAPI,
		), nil
	}

	p.logger.Info("Teams notification delivered",
		zap.String("notification_id", notif.NotificationID),
		zap.Duration("delivery_time", time.Since(start)))

	result := NewResult("teams-"+notif.NotificationID, time.Since(start))
	result.Metadata["credential_source"] = CredSourceBYOC
	result.Metadata["billing_channel"] = "teams"
	return result, nil
}

func (p *TeamsProvider) GetName() string                           { return "teams" }
func (p *TeamsProvider) GetSupportedChannel() notification.Channel { return notification.ChannelTeams }
func (p *TeamsProvider) IsHealthy(_ context.Context) bool          { return true }
func (p *TeamsProvider) Close() error                              { return nil }

// resolveWebhookURL determines the target Teams webhook.
// Priority: notification metadata > default config.
func (p *TeamsProvider) resolveWebhookURL(notif *notification.Notification) string {
	if notif.Metadata != nil {
		if url, ok := notif.Metadata["teams_webhook_url"].(string); ok && url != "" {
			return url
		}
	}
	return p.config.DefaultWebhookURL
}

// buildAdaptiveCard constructs a Teams Adaptive Card payload.
func (p *TeamsProvider) buildAdaptiveCard(notif *notification.Notification, webhookURL string) map[string]interface{} {
	return render.BuildTeamsPayload(notif, webhookURL)
}

func init() {
	RegisterFactory("teams", func(cfg map[string]interface{}, logger *zap.Logger) (Provider, error) {
		enabled, _ := cfg["enabled"].(bool)
		if !enabled {
			return nil, fmt.Errorf("teams: provider disabled")
		}
		webhookURL, _ := cfg["default_webhook_url"].(string)
		return NewTeamsProvider(TeamsConfig{
			Config:            Config{Timeout: 10 * time.Second, MaxRetries: 3, RetryDelay: 2 * time.Second},
			DefaultWebhookURL: webhookURL,
		}, logger)
	})
}
