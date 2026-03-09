package providers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

// MailgunConfig holds Mailgun-specific configuration.
type MailgunConfig struct {
	Config
	APIKey    string
	Domain    string
	FromEmail string
	FromName  string
}

// MailgunProvider implements the Provider interface for email via Mailgun.
type MailgunProvider struct {
	config     MailgunConfig
	httpClient *http.Client
	logger     *zap.Logger
}

// NewMailgunProvider creates a new Mailgun provider.
func NewMailgunProvider(config MailgunConfig, logger *zap.Logger) (Provider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	return &MailgunProvider{
		config:     config,
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
	}, nil
}

// Send sends an email via Mailgun.
func (p *MailgunProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	start := time.Now()

	p.logger.Info("Sending Mailgun email",
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

	apiURL := fmt.Sprintf("https://api.mailgun.net/v3/%s/messages", p.config.Domain)

	form := url.Values{}
	form.Set("from", from)
	form.Set("to", usr.Email)
	form.Set("subject", notif.Content.Title)
	form.Set("html", p.buildHTMLBody(notif))
	form.Set("text", notif.Content.Body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to create Mailgun request: %w", err), ErrorTypeUnknown), nil
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("api", p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewErrorResult(fmt.Errorf("Mailgun API request failed: %w", err), ErrorTypeNetwork), nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		p.logger.Error("Mailgun API error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("body", string(respBody)))
		return NewErrorResult(
			fmt.Errorf("Mailgun API error: %d - %s", resp.StatusCode, string(respBody)),
			ErrorTypeProviderAPI,
		), nil
	}

	deliveryTime := time.Since(start)
	p.logger.Info("Mailgun email sent successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.Duration("delivery_time", deliveryTime))

	res := NewResult("mailgun-"+notif.NotificationID, deliveryTime)
	res.Metadata["to_email"] = usr.Email
	res.Metadata["from_email"] = p.config.FromEmail
	return res, nil
}

func (p *MailgunProvider) GetName() string                                { return "mailgun" }
func (p *MailgunProvider) GetSupportedChannel() notification.Channel      { return notification.ChannelEmail }
func (p *MailgunProvider) IsHealthy(_ context.Context) bool               { return p.config.APIKey != "" && p.config.Domain != "" }
func (p *MailgunProvider) Close() error                                   { return nil }

func (p *MailgunProvider) buildHTMLBody(notif *notification.Notification) string {
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
	RegisterFactory("mailgun", func(cfg map[string]interface{}, logger *zap.Logger) (Provider, error) {
		enabled, _ := cfg["enabled"].(bool)
		if !enabled {
			return nil, fmt.Errorf("mailgun: provider disabled")
		}
		apiKey, _ := cfg["api_key"].(string)
		if apiKey == "" {
			return nil, fmt.Errorf("mailgun: api_key is required")
		}
		domain, _ := cfg["domain"].(string)
		if domain == "" {
			return nil, fmt.Errorf("mailgun: domain is required")
		}
		fromEmail, _ := cfg["from_email"].(string)
		fromName, _ := cfg["from_name"].(string)
		return NewMailgunProvider(MailgunConfig{
			Config:    Config{Timeout: 15 * time.Second, MaxRetries: 3, RetryDelay: 1 * time.Second},
			APIKey:    apiKey,
			Domain:    domain,
			FromEmail: fromEmail,
			FromName:  fromName,
		}, logger)
	})
}
