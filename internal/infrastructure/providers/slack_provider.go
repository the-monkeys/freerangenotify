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
	"go.uber.org/zap"
)

// SlackConfig holds configuration for the Slack provider.
type SlackConfig struct {
	Config                     // Common: Timeout, MaxRetries, RetryDelay
	DefaultWebhookURL string  // App-level fallback webhook URL
}

// SlackProvider delivers notifications to Slack via Incoming Webhooks.
type SlackProvider struct {
	config     SlackConfig
	httpClient *http.Client
	logger     *zap.Logger
}

// NewSlackProvider creates a new SlackProvider.
func NewSlackProvider(config SlackConfig, logger *zap.Logger) (*SlackProvider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &SlackProvider{
		config:     config,
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
	}, nil
}

// Send delivers a notification to Slack.
func (p *SlackProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	start := time.Now()

	webhookURL := p.resolveWebhookURL(notif, usr)
	if webhookURL == "" {
		return NewErrorResult(
			fmt.Errorf("no Slack webhook URL configured for user %s", notif.UserID),
			ErrorTypeInvalid,
		), nil
	}

	payload := p.buildPayload(notif)
	body, err := json.Marshal(payload)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to marshal Slack payload: %w", err), ErrorTypeInvalid), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to create Slack request: %w", err), ErrorTypeUnknown), nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewErrorResult(fmt.Errorf("Slack webhook request failed: %w", err), ErrorTypeNetwork), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return NewErrorResult(
			fmt.Errorf("Slack returned status %d: %s", resp.StatusCode, string(respBody)),
			ErrorTypeProviderAPI,
		), nil
	}

	p.logger.Info("Slack notification delivered",
		zap.String("notification_id", notif.NotificationID),
		zap.Duration("delivery_time", time.Since(start)))

	return NewResult("slack-"+notif.NotificationID, time.Since(start)), nil
}

// GetName returns the provider name.
func (p *SlackProvider) GetName() string { return "slack" }

// GetSupportedChannel returns the channel this provider supports.
func (p *SlackProvider) GetSupportedChannel() notification.Channel { return notification.ChannelSlack }

// IsHealthy checks if the provider is healthy.
func (p *SlackProvider) IsHealthy(_ context.Context) bool { return true }

// Close releases provider resources.
func (p *SlackProvider) Close() error { return nil }

// resolveWebhookURL determines the target Slack webhook.
// Priority: notification metadata > user-level > app-level default.
func (p *SlackProvider) resolveWebhookURL(notif *notification.Notification, usr *user.User) string {
	if notif.Metadata != nil {
		if url, ok := notif.Metadata["slack_webhook_url"].(string); ok && url != "" {
			return url
		}
	}
	if usr != nil && usr.SlackWebhookURL != "" {
		return usr.SlackWebhookURL
	}
	return p.config.DefaultWebhookURL
}

// buildPayload constructs a Slack Block Kit message.
func (p *SlackProvider) buildPayload(notif *notification.Notification) map[string]interface{} {
	blocks := []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*%s*\n%s", notif.Content.Title, notif.Content.Body),
			},
		},
	}

	// Add action URL button if present in data
	if notif.Content.Data != nil {
		if actionURL, ok := notif.Content.Data["action_url"].(string); ok && actionURL != "" {
			actionLabel := "View"
			if label, ok := notif.Content.Data["action_label"].(string); ok && label != "" {
				actionLabel = label
			}
			blocks = append(blocks, map[string]interface{}{
				"type": "actions",
				"elements": []map[string]interface{}{
					{
						"type": "button",
						"text": map[string]interface{}{
							"type": "plain_text",
							"text": actionLabel,
						},
						"url": actionURL,
					},
				},
			})
		}
	}

	return map[string]interface{}{
		"text":   notif.Content.Title, // Fallback for notifications/accessibility
		"blocks": blocks,
	}
}
