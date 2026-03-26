package providers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"
	"go.uber.org/zap"
)

// TwilioProvider implements the Provider interface for SMS via Twilio
type TwilioProvider struct {
	config Config
	logger *zap.Logger

	// Twilio configuration
	accountSID string
	authToken  string
	fromNumber string

	client *twilio.RestClient
}

// TwilioConfig holds Twilio-specific configuration
type TwilioConfig struct {
	Config

	// AccountSID is the Twilio account SID
	AccountSID string

	// AuthToken is the Twilio auth token
	AuthToken string

	// FromNumber is the Twilio phone number to send from
	FromNumber string
}

// NewTwilioProvider creates a new Twilio provider
func NewTwilioProvider(config TwilioConfig, logger *zap.Logger) (Provider, error) {
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: config.AccountSID,
		Password: config.AuthToken,
	})

	return &TwilioProvider{
		config:     config.Config,
		logger:     logger,
		accountSID: config.AccountSID,
		authToken:  config.AuthToken,
		fromNumber: config.FromNumber,
		client:     client,
	}, nil
}

// Send sends an SMS via Twilio
func (p *TwilioProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	startTime := time.Now()

	p.logger.Info("Sending Twilio SMS",
		zap.String("notification_id", notif.NotificationID),
		zap.String("user_id", usr.UserID),
		zap.String("to_phone", usr.Phone))

	if usr.Phone == "" {
		return NewErrorResult(
			fmt.Errorf("no phone number for user %s", usr.UserID),
			ErrorTypeInvalid,
		), nil
	}

	// Build SMS body
	smsBody := p.buildSMSBody(notif)

	// Per-app credential override (use local vars to avoid race conditions)
	accountSID := p.accountSID
	authToken := p.authToken
	fromNumber := p.fromNumber
	credSource := billing.CredSourceSystem

	if appCfg, ok := ctx.Value(SMSConfigKey).(*application.SMSAppConfig); ok && appCfg != nil {
		if appCfg.AccountSID != "" && appCfg.AuthToken != "" {
			accountSID = appCfg.AccountSID
			authToken = appCfg.AuthToken
			if appCfg.FromNumber != "" {
				fromNumber = appCfg.FromNumber
			}
			credSource = billing.CredSourceBYOC
			p.logger.Debug("Using per-app SMS config",
				zap.String("notification_id", notif.NotificationID),
				zap.String("from_number", fromNumber),
				zap.String("account_sid_prefix", accountSID[:min(6, len(accountSID))]+"..."))
		}
	}

	client := p.client
	if credSource == billing.CredSourceBYOC {
		// Instantiate a new client per request for BYOC to avoid mutating the shared provider client
		client = twilio.NewRestClientWithParams(twilio.ClientParams{
			Username: accountSID,
			Password: authToken,
		})
	}

	params := &openapi.CreateMessageParams{}
	params.SetTo(usr.Phone)
	params.SetFrom(fromNumber)
	params.SetBody(smsBody)

	resp, err := client.Api.CreateMessage(params)
	if err != nil {
		p.logger.Error("Twilio SMS failed", zap.Error(err))
		return p.handleError(err), nil
	}

	if resp.Status != nil && (*resp.Status == "failed" || *resp.Status == "undelivered") {
		return NewErrorResult(
			fmt.Errorf("Twilio message failed with status: %s", *resp.Status),
			ErrorTypeProviderAPI,
		), nil
	}

	deliveryTime := time.Since(startTime)
	msgID := ""
	if resp.Sid != nil {
		msgID = *resp.Sid
	}

	p.logger.Info("Twilio SMS sent successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.String("twilio_sid", msgID),
		zap.Duration("delivery_time", deliveryTime))

	result := NewResult(msgID, deliveryTime)
	result.Metadata["credential_source"] = credSource
	result.Metadata["billing_channel"] = "sms"
	result.Metadata["to_phone"] = usr.Phone
	result.Metadata["from_number"] = fromNumber
	result.Metadata["body_length"] = len(smsBody)

	return result, nil
}

// GetName returns the provider name
func (p *TwilioProvider) GetName() string {
	return "twilio"
}

// GetSupportedChannel returns the channel this provider supports
func (p *TwilioProvider) GetSupportedChannel() notification.Channel {
	return notification.ChannelSMS
}

// IsHealthy checks if Twilio is healthy
func (p *TwilioProvider) IsHealthy(ctx context.Context) bool {
	// TODO: Implement real health check
	// - Verify credentials are valid
	// - Test connectivity to Twilio API
	return true
}

// Close closes the provider
func (p *TwilioProvider) Close() error {
	return nil
}

// buildSMSBody builds SMS message body with variable substitution
func (p *TwilioProvider) buildSMSBody(notif *notification.Notification) string {
	// TODO: Implement template variable substitution
	body := notif.Content.Body

	// For now, just return the body
	// In full implementation:
	// - Parse template variables like {{var_name}}
	// - Substitute with values from notif.Content.Data
	// - Limit to 160 characters for single SMS or 1600 for concatenated

	return body
}

// handleError categorizes Twilio errors
func (p *TwilioProvider) handleError(err error) *Result {
	errMsg := err.Error()
	if strings.Contains(errMsg, "20003") || strings.Contains(errMsg, "Authentication Error") {
		return NewErrorResult(err, ErrorTypeAuth)
	}
	if strings.Contains(errMsg, "21211") || strings.Contains(errMsg, "Invalid phone number") {
		return NewErrorResult(err, ErrorTypeInvalid)
	}
	if strings.Contains(errMsg, "21608") || strings.Contains(errMsg, "unsubscribed") {
		return NewErrorResult(err, ErrorTypeInvalid)
	}
	if strings.Contains(errMsg, "21610") || strings.Contains(errMsg, "blacklisted") {
		return NewErrorResult(err, ErrorTypeInvalid)
	}
	if strings.Contains(errMsg, "20429") {
		return NewErrorResult(err, ErrorTypeRateLimit)
	}

	return NewErrorResult(err, ErrorTypeUnknown)
}

func init() {
	RegisterFactory("twilio", func(cfg map[string]interface{}, logger *zap.Logger) (Provider, error) {
		accountSID, _ := cfg["account_sid"].(string)
		authToken, _ := cfg["auth_token"].(string)
		fromNumber, _ := cfg["from_number"].(string)
		if accountSID == "" || authToken == "" {
			return nil, fmt.Errorf("twilio: account_sid and auth_token are required")
		}
		return NewTwilioProvider(TwilioConfig{
			Config:     Config{Timeout: 10 * time.Second, MaxRetries: 3, RetryDelay: 1 * time.Second},
			AccountSID: accountSID,
			AuthToken:  authToken,
			FromNumber: fromNumber,
		}, logger)
	})
}
