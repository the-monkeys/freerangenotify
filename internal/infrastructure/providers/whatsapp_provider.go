package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

// WhatsAppConfig holds configuration for the WhatsApp provider (Twilio-backed).
type WhatsAppConfig struct {
	Config                 // Common: Timeout, MaxRetries, RetryDelay
	AccountSID string     // Twilio Account SID
	AuthToken  string     // Twilio Auth Token
	FromNumber string     // WhatsApp sender number (e.g. whatsapp:+14155238886)
}

// WhatsAppProvider delivers notifications via WhatsApp using the Twilio Messages API.
type WhatsAppProvider struct {
	config     WhatsAppConfig
	httpClient *http.Client
	logger     *zap.Logger
}

// NewWhatsAppProvider creates a new WhatsAppProvider.
func NewWhatsAppProvider(config WhatsAppConfig, logger *zap.Logger) (*WhatsAppProvider, error) {
	if config.AccountSID == "" || config.AuthToken == "" {
		return nil, fmt.Errorf("WhatsApp provider requires Twilio AccountSID and AuthToken")
	}
	// Ensure from number has whatsapp: prefix
	if config.FromNumber != "" && !strings.HasPrefix(config.FromNumber, "whatsapp:") {
		config.FromNumber = "whatsapp:" + config.FromNumber
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}

	return &WhatsAppProvider{
		config:     config,
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
	}, nil
}

// Send delivers a notification via WhatsApp through the Twilio Messages API.
func (p *WhatsAppProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	start := time.Now()

	if usr == nil || usr.Phone == "" {
		return NewErrorResult(
			fmt.Errorf("user has no phone number for WhatsApp delivery"),
			ErrorTypeInvalid,
		), nil
	}

	// Build message body
	messageBody := notif.Content.Body
	if notif.Content.Title != "" {
		messageBody = fmt.Sprintf("*%s*\n\n%s", notif.Content.Title, notif.Content.Body)
	}

	// Twilio Messages API endpoint
	apiURL := fmt.Sprintf(
		"https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json",
		p.config.AccountSID,
	)

	// Ensure whatsapp: prefix on recipient
	toNumber := usr.Phone
	if !strings.HasPrefix(toNumber, "whatsapp:") {
		toNumber = "whatsapp:" + toNumber
	}

	data := url.Values{
		"To":   {toNumber},
		"From": {p.config.FromNumber},
		"Body": {messageBody},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to create WhatsApp request: %w", err), ErrorTypeUnknown), nil
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(p.config.AccountSID, p.config.AuthToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewErrorResult(fmt.Errorf("WhatsApp request failed: %w", err), ErrorTypeNetwork), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return NewErrorResult(
			fmt.Errorf("Twilio WhatsApp API returned status %d", resp.StatusCode),
			ErrorTypeProviderAPI,
		), nil
	}

	p.logger.Info("WhatsApp notification delivered",
		zap.String("notification_id", notif.NotificationID),
		zap.String("to", toNumber),
		zap.Duration("delivery_time", time.Since(start)))

	return NewResult("whatsapp-"+notif.NotificationID, time.Since(start)), nil
}

// GetName returns the provider name.
func (p *WhatsAppProvider) GetName() string { return "whatsapp" }

// GetSupportedChannel returns the channel this provider supports.
func (p *WhatsAppProvider) GetSupportedChannel() notification.Channel {
	return notification.ChannelWhatsApp
}

// IsHealthy checks if the provider is healthy.
func (p *WhatsAppProvider) IsHealthy(_ context.Context) bool { return true }

// Close releases provider resources.
func (p *WhatsAppProvider) Close() error { return nil }
