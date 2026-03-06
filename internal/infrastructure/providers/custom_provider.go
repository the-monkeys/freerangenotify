package providers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

// CustomProvider delivers notifications to a user-registered webhook endpoint.
// It acts as a generic relay, signing payloads with HMAC-SHA256 for security.
type CustomProvider struct {
	name       string
	channel    notification.Channel
	webhookURL string
	headers    map[string]string
	signingKey string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewCustomProvider creates a custom webhook-based provider.
func NewCustomProvider(name, channel, webhookURL, signingKey string, headers map[string]string, logger *zap.Logger) *CustomProvider {
	return &CustomProvider{
		name:       name,
		channel:    notification.Channel(channel),
		webhookURL: webhookURL,
		headers:    headers,
		signingKey: signingKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logger,
	}
}

// Send delivers a notification to the custom webhook endpoint.
func (p *CustomProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	start := time.Now()

	// Build standardized payload
	payload := map[string]interface{}{
		"notification_id": notif.NotificationID,
		"app_id":          notif.AppID,
		"user_id":         notif.UserID,
		"channel":         string(p.channel),
		"content":         notif.Content,
		"metadata":        notif.Metadata,
		"priority":        string(notif.Priority),
		"category":        notif.Category,
		"created_at":      notif.CreatedAt,
	}

	// Include user context if available
	if usr != nil {
		payload["user"] = map[string]interface{}{
			"email":       usr.Email,
			"phone":       usr.Phone,
			"external_id": usr.ExternalID,
			"timezone":    usr.Timezone,
			"language":    usr.Language,
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to marshal custom payload: %w", err), ErrorTypeInvalid), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.webhookURL, bytes.NewReader(body))
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to create custom provider request: %w", err), ErrorTypeUnknown), nil
	}

	// Set standard headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "FreeRangeNotify/1.0")

	// Set custom headers
	for k, v := range p.headers {
		req.Header.Set(k, v)
	}

	// HMAC-SHA256 signature
	if p.signingKey != "" {
		mac := hmac.New(sha256.New, []byte(p.signingKey))
		mac.Write(body)
		signature := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-FRN-Signature", signature)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewErrorResult(fmt.Errorf("custom provider request failed: %w", err), ErrorTypeNetwork), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return NewErrorResult(
			fmt.Errorf("custom provider %s returned status %d: %s", p.name, resp.StatusCode, string(respBody)),
			ErrorTypeProviderAPI,
		), nil
	}

	p.logger.Info("Custom provider notification delivered",
		zap.String("provider", p.name),
		zap.String("notification_id", notif.NotificationID),
		zap.Duration("delivery_time", time.Since(start)))

	return NewResult(fmt.Sprintf("custom-%s-%s", p.name, notif.NotificationID), time.Since(start)), nil
}

// GetName returns the provider name.
func (p *CustomProvider) GetName() string { return p.name }

// GetSupportedChannel returns the channel this provider supports.
func (p *CustomProvider) GetSupportedChannel() notification.Channel { return p.channel }

// IsHealthy checks if the provider is healthy.
func (p *CustomProvider) IsHealthy(_ context.Context) bool { return true }

// Close releases provider resources.
func (p *CustomProvider) Close() error { return nil }
