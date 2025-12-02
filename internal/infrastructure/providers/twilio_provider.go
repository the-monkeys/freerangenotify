package providers

import (
	"context"
	"fmt"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
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

	// TODO: Add Twilio client
	// client *twilio.RestClient
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
	// TODO: Initialize Twilio client
	// client := twilio.NewRestClientWithParams(twilio.ClientParams{
	//     Username: config.AccountSID,
	//     Password: config.AuthToken,
	// })

	return &TwilioProvider{
		config:     config.Config,
		logger:     logger,
		accountSID: config.AccountSID,
		authToken:  config.AuthToken,
		fromNumber: config.FromNumber,
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

	// TODO: Send SMS via Twilio
	// params := &api.CreateMessageParams{}
	// params.SetTo(usr.Phone)
	// params.SetFrom(p.fromNumber)
	// params.SetBody(smsBody)
	//
	// resp, err := p.client.Api.CreateMessage(params)
	// if err != nil {
	//     return p.handleError(err), nil
	// }
	//
	// if resp.Status != nil && *resp.Status == "failed" {
	//     return NewErrorResult(
	//         fmt.Errorf("Twilio message failed"),
	//         ErrorTypeProviderAPI,
	//     ), nil
	// }

	// Simulate sending (remove when real implementation is added)
	time.Sleep(80 * time.Millisecond)

	deliveryTime := time.Since(startTime)

	p.logger.Info("Twilio SMS sent successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.Duration("delivery_time", deliveryTime))

	result := NewResult("twilio-simulated-"+notif.NotificationID, deliveryTime)
	result.Metadata["to_phone"] = usr.Phone
	result.Metadata["from_number"] = p.fromNumber
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
	// TODO: Implement proper error categorization based on Twilio error codes
	// - 20003: Authentication Error -> ErrorTypeAuth
	// - 21211: Invalid Phone Number -> ErrorTypeInvalid
	// - 21608: Unsubscribed -> ErrorTypeInvalid
	// - 21610: Blacklisted -> ErrorTypeInvalid
	// - 20429: Too Many Requests -> ErrorTypeRateLimit

	return NewErrorResult(err, ErrorTypeUnknown)
}
