package providers

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/attachment"
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
	sender    EmailSender
	tlsConfig *tls.Config
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
	// TLSConfig allows deployments using a private CA to provide trust roots.
	// The resolved SMTP host is always used for certificate verification.
	TLSConfig *tls.Config
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
		tlsConfig:  config.TLSConfig,
	}, nil
}

// Send sends an email via SMTP
func (p *SMTPProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	startTime := time.Now()
	cfg, credSource := p.resolveConfig(ctx)
	ctx, cancel := context.WithTimeout(ctx, p.operationTimeout())
	defer cancel()
	if err := validateSMTPConfig(cfg); err != nil {
		return smtpFailure(newDeliveryError("smtp", "configuration", credSource, ErrorTypeConfiguration, err)), nil
	}
	if err := ctx.Err(); err != nil {
		return smtpFailure(newDeliveryError("smtp", "send", credSource, "", err)), nil
	}

	p.logger.Info("Sending SMTP email",
		zap.String("notification_id", notif.NotificationID),
		zap.String("user_id", usr.UserID),
		zap.String("to_email", usr.Email))

	if usr.Email == "" {
		return smtpFailure(newDeliveryError("smtp", "send", credSource, ErrorTypeInvalid, fmt.Errorf("no email address for user %s", usr.UserID))), nil
	}
	addr := net.JoinHostPort(cfg.Host, fmt.Sprint(cfg.Port))
	var auth smtp.Auth
	if cfg.Username != "" && cfg.Password != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}

	// Resolve attachments (URL / inline base64 / file_id) via the
	// per-notification closure on ctx. The shared helper keeps this path
	// byte-identical across all six email providers.
	resolved, _, rErr := resolveEmailAttachments(ctx, notif, p.logger, "smtp")
	if rErr != nil {
		return smtpFailure(newDeliveryError("smtp", "send", credSource, emailAttachmentErrorType(rErr), rErr)), nil
	}
	if resolved != nil {
		defer attachment.CloseAll(resolved)
	}

	// Construct message
	to := []string{usr.Email}
	msg, mErr := buildSMTPMessage(smtpMessageOptions{
		From:        cfg.FromEmail,
		FromName:    cfg.FromName,
		To:          usr.Email,
		Subject:     notif.Content.Title,
		HTMLBody:    notif.Content.Body,
		Attachments: resolved,
	})
	if mErr != nil {
		p.logger.Error("Failed to build SMTP message",
			zap.String("notification_id", notif.NotificationID),
			zap.Error(mErr))
		category := ErrorTypeInvalid
		if errors.Is(mErr, ErrSMTPAttachmentReadFailed) {
			category = emailAttachmentErrorType(mErr)
		}
		return smtpFailure(newDeliveryError("smtp", "send", credSource, category, mErr)), nil
	}

	// Send email with retries
	var err error
	for i := 0; i <= p.maxRetries; i++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = ctxErr
			break
		}
		if i > 0 {
			timer := time.NewTimer(p.config.RetryDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				err = ctx.Err()
			case <-timer.C:
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				err = ctxErr
				break
			}
		}
		if p.sender != nil {
			err = p.sender(addr, auth, cfg.FromEmail, to, msg)
		} else {
			err = p.sendSMTP(ctx, cfg, credSource, auth, to, msg)
		}
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			err = ctx.Err()
			break
		}
		failure := newDeliveryError("smtp", "send", credSource, "", err)
		if !failure.Retryable {
			err = failure
			break
		}
		p.logger.Warn("SMTP send failed", zap.Int("attempt", i+1), zap.Error(failure))
	}

	if err != nil {
		failure := newDeliveryError("smtp", "send", credSource, "", err)
		p.logger.Error("Failed to send SMTP email", zap.Error(failure))
		return smtpFailure(failure), nil
	}

	deliveryTime := time.Since(startTime)

	p.logger.Info("SMTP email sent successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.Duration("delivery_time", deliveryTime))

	result := NewResult("smtp-"+notif.NotificationID, deliveryTime)
	result.Metadata["credential_source"] = credSource
	result.Metadata["billing_channel"] = "email"
	result.Metadata["to_email"] = usr.Email
	result.Metadata["from_email"] = cfg.FromEmail

	return result, nil
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
	return p.CheckHealth(ctx) == nil
}

func (p *SMTPProvider) CheckHealth(ctx context.Context) error {
	cfg, source := p.resolveConfig(ctx)
	if err := validateSMTPConfig(cfg); err != nil {
		return newDeliveryError("smtp", "configuration", source, ErrorTypeConfiguration, err)
	}
	ctx, cancel := context.WithTimeout(ctx, p.operationTimeout())
	defer cancel()
	_, closeConn, err := p.openSMTP(ctx, cfg, source)
	if err != nil {
		return err
	}
	closeConn()
	return nil
}

func (p *SMTPProvider) resolveConfig(ctx context.Context) (SMTPConfig, string) {
	cfg := SMTPConfig{Config: p.config, Host: p.host, Port: p.port, Username: p.username, Password: p.password, FromEmail: p.fromEmail, FromName: p.fromName, TLSConfig: p.tlsConfig}
	source := CredSourceSystem
	if app, ok := ctx.Value(EmailConfigKey).(*application.EmailConfig); ok && app != nil && app.ProviderType == "smtp" && app.SMTP != nil {
		custom := app.SMTP
		cfg.Host, cfg.Port, cfg.Username, cfg.Password = custom.Host, custom.Port, custom.Username, custom.Password
		cfg.FromEmail, cfg.FromName = custom.FromEmail, custom.FromName
		source = CredSourceBYOC
	}
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	return cfg, source
}

func (p *SMTPProvider) operationTimeout() time.Duration {
	if p.config.Timeout > 0 {
		return p.config.Timeout
	}
	return 30 * time.Second
}
func validateSMTPConfig(cfg SMTPConfig) error {
	if cfg.Host == "" {
		return fmt.Errorf("SMTP host is required")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("SMTP port is invalid")
	}
	if (cfg.Username == "") != (cfg.Password == "") {
		return fmt.Errorf("SMTP username and password must both be configured")
	}
	return nil
}
func smtpFailure(err *DeliveryError) *Result {
	result := NewErrorResult(err, err.ErrorType)
	result.Metadata = DeliveryErrorMetadata(err)
	return result
}

// Close closes the provider
func (p *SMTPProvider) Close() error {
	return nil
}

func init() {
	RegisterFactory("smtp", func(cfg map[string]interface{}, logger *zap.Logger) (Provider, error) {
		host, _ := cfg["host"].(string)
		if host == "" {
			return nil, fmt.Errorf("smtp: host is required")
		}
		port, _ := cfg["port"].(int)
		if port == 0 {
			if pf, ok := cfg["port"].(float64); ok {
				port = int(pf)
			}
		}
		username, _ := cfg["username"].(string)
		password, _ := cfg["password"].(string)
		fromEmail, _ := cfg["from_email"].(string)
		fromName, _ := cfg["from_name"].(string)
		return NewSMTPProvider(SMTPConfig{
			Config:    Config{Timeout: 30 * time.Second, MaxRetries: 3, RetryDelay: 1 * time.Second},
			Host:      host,
			Port:      port,
			Username:  username,
			Password:  password,
			FromEmail: fromEmail,
			FromName:  fromName,
		}, logger)
	})
}
