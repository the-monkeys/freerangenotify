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

const postmarkAPIURL = "https://api.postmarkapp.com/email"

// PostmarkConfig holds Postmark-specific configuration.
type PostmarkConfig struct {
	Config
	ServerToken string
	FromEmail   string
	FromName    string
}

// PostmarkProvider implements the Provider interface for email via Postmark.
type PostmarkProvider struct {
	config     PostmarkConfig
	httpClient *http.Client
	logger     *zap.Logger
}

// NewPostmarkProvider creates a new Postmark provider.
func NewPostmarkProvider(config PostmarkConfig, logger *zap.Logger) (Provider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	return &PostmarkProvider{
		config:     config,
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
	}, nil
}

// Send sends an email via Postmark.
func (p *PostmarkProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	start := time.Now()

	p.logger.Info("Sending Postmark email",
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
		"From":     from,
		"To":       usr.Email,
		"Subject":  notif.Content.Title,
		"HtmlBody": p.buildHTMLBody(notif),
		"TextBody": notif.Content.Body,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to marshal Postmark payload: %w", err), ErrorTypeInvalid), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, postmarkAPIURL, bytes.NewReader(body))
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to create Postmark request: %w", err), ErrorTypeUnknown), nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Postmark-Server-Token", p.config.ServerToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewErrorResult(fmt.Errorf("Postmark API request failed: %w", err), ErrorTypeNetwork), nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		p.logger.Error("Postmark API error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("body", string(respBody)))
		return NewErrorResult(
			fmt.Errorf("Postmark API error: %d - %s", resp.StatusCode, string(respBody)),
			ErrorTypeProviderAPI,
		), nil
	}

	var result struct {
		MessageID string `json:"MessageID"`
	}
	_ = json.Unmarshal(respBody, &result)

	deliveryTime := time.Since(start)
	p.logger.Info("Postmark email sent successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.String("message_id", result.MessageID),
		zap.Duration("delivery_time", deliveryTime))

	res := NewResult("postmark-"+result.MessageID, deliveryTime)
	res.Metadata["credential_source"] = CredSourceSystem
	res.Metadata["billing_channel"] = "email"
	res.Metadata["to_email"] = usr.Email
	res.Metadata["from_email"] = p.config.FromEmail
	return res, nil
}

func (p *PostmarkProvider) GetName() string                                { return "postmark" }
func (p *PostmarkProvider) GetSupportedChannel() notification.Channel      { return notification.ChannelEmail }
func (p *PostmarkProvider) IsHealthy(_ context.Context) bool               { return p.config.ServerToken != "" }
func (p *PostmarkProvider) Close() error                                   { return nil }

func (p *PostmarkProvider) buildHTMLBody(notif *notification.Notification) string {
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
	RegisterFactory("postmark", func(cfg map[string]interface{}, logger *zap.Logger) (Provider, error) {
		enabled, _ := cfg["enabled"].(bool)
		if !enabled {
			return nil, fmt.Errorf("postmark: provider disabled")
		}
		serverToken, _ := cfg["server_token"].(string)
		if serverToken == "" {
			return nil, fmt.Errorf("postmark: server_token is required")
		}
		fromEmail, _ := cfg["from_email"].(string)
		fromName, _ := cfg["from_name"].(string)
		return NewPostmarkProvider(PostmarkConfig{
			Config:      Config{Timeout: 15 * time.Second, MaxRetries: 3, RetryDelay: 1 * time.Second},
			ServerToken: serverToken,
			FromEmail:   fromEmail,
			FromName:    fromName,
		}, logger)
	})
}
