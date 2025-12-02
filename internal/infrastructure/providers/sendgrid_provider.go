package providers

import (
	"context"
	"fmt"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

// SendGridProvider implements the Provider interface for email via SendGrid
type SendGridProvider struct {
	config Config
	logger *zap.Logger

	// SendGrid configuration
	apiKey    string
	fromEmail string
	fromName  string

	// TODO: Add SendGrid client
	// client *sendgrid.Client
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
	// TODO: Initialize SendGrid client
	// client := sendgrid.NewSendClient(config.APIKey)

	return &SendGridProvider{
		config:    config.Config,
		logger:    logger,
		apiKey:    config.APIKey,
		fromEmail: config.FromEmail,
		fromName:  config.FromName,
	}, nil
}

// Send sends an email via SendGrid
func (p *SendGridProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	startTime := time.Now()

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

	// TODO: Build SendGrid message
	// from := mail.NewEmail(p.fromName, p.fromEmail)
	// to := mail.NewEmail(usr.Email, usr.Email)
	// message := mail.NewSingleEmail(
	//     from,
	//     notif.Content.Title,
	//     to,
	//     notif.Content.Body,
	//     p.buildHTMLBody(notif),
	// )
	//
	// // Add custom data
	// if len(notif.Content.Data) > 0 {
	//     for key, value := range notif.Content.Data {
	//         message.AddCustomArg(key, fmt.Sprintf("%v", value))
	//     }
	// }
	//
	// // Send email
	// response, err := p.client.SendWithContext(ctx, message)
	// if err != nil {
	//     return p.handleError(err), nil
	// }
	//
	// if response.StatusCode >= 400 {
	//     return NewErrorResult(
	//         fmt.Errorf("SendGrid API error: %d", response.StatusCode),
	//         ErrorTypeProviderAPI,
	//     ), nil
	// }

	// Simulate sending (remove when real implementation is added)
	time.Sleep(100 * time.Millisecond)

	deliveryTime := time.Since(startTime)

	p.logger.Info("SendGrid email sent successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.Duration("delivery_time", deliveryTime))

	result := NewResult("sendgrid-simulated-"+notif.NotificationID, deliveryTime)
	result.Metadata["to_email"] = usr.Email
	result.Metadata["from_email"] = p.fromEmail

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
	// TODO: Implement real health check
	// - Verify API key is valid
	// - Test connectivity to SendGrid API
	return true
}

// Close closes the provider
func (p *SendGridProvider) Close() error {
	return nil
}

// buildHTMLBody builds HTML email body from notification content
func (p *SendGridProvider) buildHTMLBody(notif *notification.Notification) string {
	// TODO: Implement proper HTML template rendering
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>%s</title>
</head>
<body>
    <h1>%s</h1>
    <p>%s</p>
</body>
</html>
`, notif.Content.Title, notif.Content.Title, notif.Content.Body)
}

// handleError categorizes SendGrid errors
func (p *SendGridProvider) handleError(err error) *Result {
	// TODO: Implement proper error categorization
	return NewErrorResult(err, ErrorTypeUnknown)
}
