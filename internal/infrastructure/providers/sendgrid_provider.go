package providers

import (
	"context"
	"fmt"
	"time"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

// SendGridProvider implements the Provider interface for email via SendGrid
type SendGridProvider struct {
	config Config
	logger *zap.Logger

	// Default SendGrid configuration
	apiKey    string
	fromEmail string
	fromName  string

	client *sendgrid.Client
}

// SendGridConfig holds SendGrid-specific configuration
type SendGridConfig struct {
	Config

	// APIKey is the SendGrid API key
	APIKey string

	// FromEmail is the default sender email address
	FromEmail string

	// FromName is the default sender name
	FromName string
}

// NewSendGridProvider creates a new SendGrid provider
func NewSendGridProvider(config SendGridConfig, logger *zap.Logger) (Provider, error) {
	client := sendgrid.NewSendClient(config.APIKey)

	return &SendGridProvider{
		config:    config.Config,
		logger:    logger,
		apiKey:    config.APIKey,
		fromEmail: config.FromEmail,
		fromName:  config.FromName,
		client:    client,
	}, nil
}

// Send sends an email via SendGrid
func (p *SendGridProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	startTime := time.Now()

	apiKey := p.apiKey
	fromEmail := p.fromEmail
	fromName := p.fromName
	client := p.client

	// Check for dynamic config override in context
	if cfg, ok := ctx.Value(EmailConfigKey).(*application.EmailConfig); ok && cfg != nil {
		if cfg.ProviderType == "sendgrid" && cfg.SendGrid != nil {
			apiKey = cfg.SendGrid.APIKey
			fromEmail = cfg.SendGrid.FromEmail
			fromName = cfg.SendGrid.FromName
			client = sendgrid.NewSendClient(apiKey)
			p.logger.Debug("Using dynamic SendGrid configuration", zap.String("notification_id", notif.NotificationID))
		}
	}

	p.logger.Info("Sending SendGrid email",
		zap.String("notification_id", notif.NotificationID),
		zap.String("user_id", usr.UserID),
		zap.String("to_email", usr.Email))

	if usr.Email == "" {
		return NewErrorResult(
			fmt.Errorf("no email address for user %s", usr.UserID),
			ErrorTypeInvalid,
		), nil
	}

	from := mail.NewEmail(fromName, fromEmail)
	to := mail.NewEmail(usr.Email, usr.Email)
	message := mail.NewSingleEmail(
		from,
		notif.Content.Title,
		to,
		notif.Content.Body,
		p.buildHTMLBody(notif),
	)

	// Add custom data
	if len(notif.Content.Data) > 0 {
		customArgs := make(map[string]string)
		for key, value := range notif.Content.Data {
			customArgs[key] = fmt.Sprintf("%v", value)
		}
		message.Personalizations[0].CustomArgs = customArgs
	}

	// Send email
	response, err := client.SendWithContext(ctx, message)
	if err != nil {
		p.logger.Error("Failed to send SendGrid email", zap.Error(err))
		return NewErrorResult(err, ErrorTypeProviderAPI), nil
	}

	if response.StatusCode >= 400 {
		p.logger.Error("SendGrid API error", zap.Int("status_code", response.StatusCode), zap.String("body", response.Body))
		return NewErrorResult(
			fmt.Errorf("SendGrid API error: %d - %s", response.StatusCode, response.Body),
			ErrorTypeProviderAPI,
		), nil
	}

	deliveryTime := time.Since(startTime)

	p.logger.Info("SendGrid email sent successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.Duration("delivery_time", deliveryTime))

	result := NewResult("sendgrid-"+notif.NotificationID, deliveryTime)
	result.Metadata["to_email"] = usr.Email
	result.Metadata["from_email"] = fromEmail

	return result, nil
}

// GetName returns the provider name
func (p *SendGridProvider) GetName() string {
	return "sendgrid"
}

// GetSupportedChannel returns the channel this provider supports
func (p *SendGridProvider) GetSupportedChannel() notification.Channel {
	return notification.ChannelEmail
}

// IsHealthy checks if SendGrid is healthy
func (p *SendGridProvider) IsHealthy(ctx context.Context) bool {
	// Simple sanity check: we have an API key and client
	return p.apiKey != "" && p.client != nil
}

// Close closes the provider
func (p *SendGridProvider) Close() error {
	return nil
}

// buildHTMLBody builds HTML email body from notification content
func (p *SendGridProvider) buildHTMLBody(notif *notification.Notification) string {
	// If the body already looks like HTML, use it as is
	// Otherwise wrap it in a basic template
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>%s</title>
</head>
<body style="font-family: sans-serif; line-height: 1.6; color: #333;">
    <div style="max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #eee; border-radius: 5px;">
        <h1 style="color: #444; margin-top: 0;">%s</h1>
        <div style="margin-bottom: 20px;">
            %s
        </div>
        <hr style="border: 0; border-top: 1px solid #eee;" />
        <footer style="font-size: 12px; color: #888;">
            Sent by FreeRangeNotify
        </footer>
    </div>
</body>
</html>
`, notif.Content.Title, notif.Content.Title, notif.Content.Body)
}

// handleError categorizes SendGrid errors
func (p *SendGridProvider) handleError(err error) *Result {
	// TODO: Implement proper error categorization
	return NewErrorResult(err, ErrorTypeUnknown)
}
