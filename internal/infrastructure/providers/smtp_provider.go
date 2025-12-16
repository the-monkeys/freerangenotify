package providers

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

// EmailSender defines the function signature for sending emails
type EmailSender func(addr string, a smtp.Auth, from string, to []string, msg []byte) error

// SMTPProvider implements the Provider interface for email via SMTP
type SMTPProvider struct {
	config Config
	logger *zap.Logger

	// SMTP configuration
	host       string
	port       int
	username   string
	password   string
	fromEmail  string
	fromName   string
	maxRetries int

	// sender is the function used to send emails (replaceable for testing)
	sender EmailSender
}

// SMTPConfig holds SMTP-specific configuration
type SMTPConfig struct {
	Config

	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

// NewSMTPProvider creates a new SMTP provider
func NewSMTPProvider(config SMTPConfig, logger *zap.Logger) (Provider, error) {
	if config.Host == "" {
		return nil, fmt.Errorf("SMTP host is required")
	}
	if config.Port == 0 {
		config.Port = 587 // Default to submission port
	}

	return &SMTPProvider{
		config:     config.Config,
		logger:     logger,
		host:       config.Host,
		port:       config.Port,
		username:   config.Username,
		password:   config.Password,
		fromEmail:  config.FromEmail,
		fromName:   config.FromName,
		maxRetries: config.MaxRetries,
		sender:     smtp.SendMail,
	}, nil
}

// Send sends an email via SMTP
func (p *SMTPProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	startTime := time.Now()

	p.logger.Info("Sending SMTP email",
		zap.String("notification_id", notif.NotificationID),
		zap.String("user_id", usr.UserID),
		zap.String("to_email", usr.Email))

	if usr.Email == "" {
		return NewErrorResult(
			fmt.Errorf("no email address for user %s", usr.UserID),
			ErrorTypeInvalid,
		), nil
	}

	addr := fmt.Sprintf("%s:%d", p.host, p.port)
	var auth smtp.Auth
	if p.username != "" && p.password != "" {
		auth = smtp.PlainAuth("", p.username, p.password, p.host)
	}

	// Construct message
	to := []string{usr.Email}
	msg := p.buildMessage(usr.Email, notif.Content.Title, notif.Content.Body)

	// Send email with retries
	var err error
	for i := 0; i <= p.maxRetries; i++ {
		if i > 0 {
			time.Sleep(p.config.RetryDelay)
		}

		// Note: net/smtp.SendMail does not support context cancellation directly,
		// but we can wrap the connection establishment if we implemented the client manually.
		// For simplicity/standard lib usage, we use SendMail which blocks.
		// In a production "world-class" system, we might want a pool of workers or a better client.
		// Use the sender function
		err = p.sender(addr, auth, p.fromEmail, to, msg)
		if err == nil {
			break
		}
		p.logger.Warn("SMTP send failed, retrying", zap.Int("attempt", i+1), zap.Error(err))
	}

	if err != nil {
		p.logger.Error("Failed to send SMTP email", zap.Error(err))
		return NewErrorResult(err, ErrorTypeProviderAPI), nil
	}

	deliveryTime := time.Since(startTime)

	p.logger.Info("SMTP email sent successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.Duration("delivery_time", deliveryTime))

	result := NewResult("smtp-"+notif.NotificationID, deliveryTime)
	result.Metadata["to_email"] = usr.Email
	result.Metadata["from_email"] = p.fromEmail

	return result, nil
}

// buildMessage constructs a MIME message
func (p *SMTPProvider) buildMessage(to, subject, body string) []byte {
	// Basic MIME headers
	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("%s <%s>", p.fromName, p.fromEmail)
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=\"UTF-8\""

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	return []byte(message)
}

// GetName returns the provider name
func (p *SMTPProvider) GetName() string {
	return "smtp"
}

// GetSupportedChannel returns the channel this provider supports
func (p *SMTPProvider) GetSupportedChannel() notification.Channel {
	return notification.ChannelEmail
}

// IsHealthy checks if SMTP server is reachable
func (p *SMTPProvider) IsHealthy(ctx context.Context) bool {
	// Try to connect to the server
	addr := fmt.Sprintf("%s:%d", p.host, p.port)
	conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		// Try non-TLS if TLS fails (fallback check)
		c, err := smtp.Dial(addr)
		if err != nil {
			p.logger.Error("SMTP health check failed", zap.Error(err))
			return false
		}
		c.Close()
		return true
	}
	conn.Close()
	return true
}

// Close closes the provider
func (p *SMTPProvider) Close() error {
	return nil
}
