package providers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

// WebhookProvider implements the Provider interface for webhooks
type WebhookProvider struct {
	config Config
	logger *zap.Logger
	client *http.Client
	secret string
}

// WebhookConfig holds Webhook-specific configuration
type WebhookConfig struct {
	Config
	Secret string
}

// Custom HTTP Client Interface for testing
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// WebhookPayload represents the data sent to the webhook endpoint
type WebhookPayload struct {
	ID        string                 `json:"id"`
	AppID     string                 `json:"app_id"`
	UserID    string                 `json:"user_id"`
	Channel   string                 `json:"channel"`
	Priority  string                 `json:"priority"`
	Status    string                 `json:"status"`
	Content   notification.Content   `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// NewWebhookProvider creates a new Webhook provider
func NewWebhookProvider(config WebhookConfig, logger *zap.Logger) (Provider, error) {
	if config.Secret == "" {
		logger.Warn("Webhook provider initialized without a signing secret. Webhooks will not be signed.")
	}

	return &WebhookProvider{
		config: config.Config,
		logger: logger,
		secret: config.Secret,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}, nil
}

// Send sends a notification via Webhook (HTTP POST)
func (p *WebhookProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	startTime := time.Now()

	targetURL := usr.WebhookURL
	if targetURL == "" {
		// Fallback to metadata if provided (e.g. for one-off webhooks)
		if url, ok := notif.Metadata["webhook_url"].(string); ok {
			targetURL = url
		}
	}

	if targetURL == "" {
		return NewErrorResult(
			fmt.Errorf("no webhook URL for user %s", usr.UserID),
			ErrorTypeInvalid,
		), nil
	}

	p.logger.Info("Sending Webhook",
		zap.String("notification_id", notif.NotificationID),
		zap.String("user_id", usr.UserID),
		zap.String("url", targetURL))

	// Prepare Payload
	webhookPayload := WebhookPayload{
		ID:        notif.NotificationID,
		AppID:     notif.AppID,
		UserID:    usr.UserID,
		Channel:   string(notif.Channel),
		Priority:  string(notif.Priority),
		Status:    string(notif.Status),
		Content:   notif.Content,
		Metadata:  notif.Metadata,
		CreatedAt: notif.CreatedAt,
	}

	payload, err := json.Marshal(webhookPayload)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to marshal notification: %w", err), ErrorTypeInvalid), nil
	}

	// Create Request
	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewBuffer(payload))
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to create request: %w", err), ErrorTypeInvalid), nil
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "FreeRangeNotify-Webhook/1.0")
	req.Header.Set("X-Notification-ID", notif.NotificationID)

	// Add HMAC Signature if secret is configured
	if p.secret != "" {
		mac := hmac.New(sha256.New, []byte(p.secret))
		mac.Write(payload)
		signature := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Webhook-Signature", signature)
	}

	// Send Request with retries
	var resp *http.Response
	for i := 0; i <= p.config.MaxRetries; i++ {
		if i > 0 {
			time.Sleep(p.config.RetryDelay)
		}

		resp, err = p.client.Do(req)
		if err == nil {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				resp.Body.Close()
				break
			}
			// Treat non-2xx as error for retry
			err = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			resp.Body.Close()
		}

		p.logger.Warn("Webhook send failed, retrying",
			zap.Int("attempt", i+1),
			zap.String("url", targetURL),
			zap.Error(err))
	}

	if err != nil {
		p.logger.Error("Failed to send Webhook", zap.Error(err))
		return NewErrorResult(err, ErrorTypeProviderAPI), nil
	}

	deliveryTime := time.Since(startTime)

	p.logger.Info("Webhook sent successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.Duration("delivery_time", deliveryTime))

	result := NewResult("webhook-"+notif.NotificationID, deliveryTime)
	result.Metadata["url"] = targetURL

	return result, nil
}

// GetName returns the provider name
func (p *WebhookProvider) GetName() string {
	return "webhook"
}

// GetSupportedChannel returns the channel this provider supports
func (p *WebhookProvider) GetSupportedChannel() notification.Channel {
	return notification.ChannelWebhook
}

// IsHealthy checks if the provider is operational
func (p *WebhookProvider) IsHealthy(ctx context.Context) bool {
	// Webhook provider is client-side, essentially always healthy unless config is bad
	return true
}

// Close closes the provider
func (p *WebhookProvider) Close() error {
	p.client.CloseIdleConnections()
	return nil
}
