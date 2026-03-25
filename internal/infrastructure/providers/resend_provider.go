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

const resendAPIURL = "https://api.resend.com/emails"

// ResendConfig holds Resend-specific configuration.
type ResendConfig struct {
	Config
	APIKey    string
	FromEmail string
	FromName  string
}

// ResendProvider implements the Provider interface for email via Resend.
type ResendProvider struct {
	config     ResendConfig
	httpClient *http.Client
	logger     *zap.Logger
}

// NewResendProvider creates a new Resend provider.
func NewResendProvider(config ResendConfig, logger *zap.Logger) (Provider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	return &ResendProvider{
		config:     config,
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
	}, nil
}

// Send sends an email via Resend.
func (p *ResendProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	start := time.Now()

	p.logger.Info("Sending Resend email",
		zap.String("notification_id", notif.NotificationID),
		zap.String("user_id", usr.UserID),
		zap.String("to_email", usr.Email))

	if usr.Email == "" {
		return NewErrorResult(
			fmt.Errorf("no email address for user %s", usr.UserID),
			ErrorTypeInvalid,
		), nil
	}

	from := p.config.FromEmail
	if p.config.FromName != "" {
		from = fmt.Sprintf("%s <%s>", p.config.FromName, p.config.FromEmail)
	}

	payload := map[string]interface{}{
		"from":    from,
		"to":      []string{usr.Email},
		"subject": notif.Content.Title,
		"html":    p.buildHTMLBody(notif),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to marshal Resend payload: %w", err), ErrorTypeInvalid), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendAPIURL, bytes.NewReader(body))
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to create Resend request: %w", err), ErrorTypeUnknown), nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewErrorResult(fmt.Errorf("Resend API request failed: %w", err), ErrorTypeNetwork), nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		p.logger.Error("Resend API error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("body", string(respBody)))
		return NewErrorResult(
			fmt.Errorf("Resend API error: %d - %s", resp.StatusCode, string(respBody)),
			ErrorTypeProviderAPI,
		), nil
	}

	var result struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(respBody, &result)

	deliveryTime := time.Since(start)
	p.logger.Info("Resend email sent successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.Duration("delivery_time", deliveryTime))

	res := NewResult("resend-"+result.ID, deliveryTime)
	res.Metadata["credential_source"] = CredSourceSystem
	res.Metadata["billing_channel"] = "email"
	res.Metadata["to_email"] = usr.Email
	res.Metadata["from_email"] = p.config.FromEmail
	return res, nil
}

func (p *ResendProvider) GetName() string                                { return "resend" }
func (p *ResendProvider) GetSupportedChannel() notification.Channel      { return notification.ChannelEmail }
func (p *ResendProvider) IsHealthy(_ context.Context) bool               { return p.config.APIKey != "" }
func (p *ResendProvider) Close() error                                   { return nil }

func (p *ResendProvider) buildHTMLBody(notif *notification.Notification) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><title>%s</title></head>
<body style="font-family: sans-serif; line-height: 1.6; color: #333;">
    <div style="max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #eee; border-radius: 5px;">
        <h1 style="color: #444; margin-top: 0;">%s</h1>
        <div style="margin-bottom: 20px;">%s</div>
        <hr style="border: 0; border-top: 1px solid #eee;" />
        <footer style="font-size: 12px; color: #888;">Sent by FreeRangeNotify</footer>
    </div>
</body>
</html>`, notif.Content.Title, notif.Content.Title, notif.Content.Body)
}

func init() {
	RegisterFactory("resend", func(cfg map[string]interface{}, logger *zap.Logger) (Provider, error) {
		enabled, _ := cfg["enabled"].(bool)
		if !enabled {
			return nil, fmt.Errorf("resend: provider disabled")
		}
		apiKey, _ := cfg["api_key"].(string)
		if apiKey == "" {
			return nil, fmt.Errorf("resend: api_key is required")
		}
		fromEmail, _ := cfg["from_email"].(string)
		fromName, _ := cfg["from_name"].(string)
		return NewResendProvider(ResendConfig{
			Config:    Config{Timeout: 15 * time.Second, MaxRetries: 3, RetryDelay: 1 * time.Second},
			APIKey:    apiKey,
			FromEmail: fromEmail,
			FromName:  fromName,
		}, logger)
	})
}
