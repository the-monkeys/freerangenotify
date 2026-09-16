package services

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
)

// TwilioSMSSender sends plain SMS and WhatsApp text messages via the Twilio
// REST API, for platform-level notifications that aren't tied to any
// tenant/app (e.g. admin payment-success alerts). It reads credentials
// directly from the environment, matching the pattern already used for
// phone-OTP delivery in auth_service_impl.go.
type TwilioSMSSender struct {
	logger *zap.Logger
	client *http.Client
}

func NewTwilioSMSSender(logger *zap.Logger) *TwilioSMSSender {
	return &TwilioSMSSender{
		logger: logger,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// Send delivers a plain SMS to the given phone number.
func (s *TwilioSMSSender) Send(toNumber, body string) error {
	accountSID := os.Getenv("FREERANGE_PROVIDERS_TWILIO_ACCOUNT_SID")
	authToken := os.Getenv("FREERANGE_PROVIDERS_TWILIO_AUTH_TOKEN")
	fromNumber := os.Getenv("FREERANGE_PROVIDERS_TWILIO_FROM_NUMBER")

	if accountSID == "" || authToken == "" || fromNumber == "" || toNumber == "" {
		s.logger.Warn("Twilio SMS not configured, skipping notification SMS", zap.String("to", toNumber))
		return nil
	}

	return s.send(accountSID, authToken, fromNumber, toNumber, body)
}

// SendWhatsApp delivers a WhatsApp text message to the given phone number.
func (s *TwilioSMSSender) SendWhatsApp(toNumber, body string) error {
	accountSID := os.Getenv("FREERANGE_PROVIDERS_WHATSAPP_ACCOUNT_SID")
	authToken := os.Getenv("FREERANGE_PROVIDERS_WHATSAPP_AUTH_TOKEN")
	fromNumber := os.Getenv("FREERANGE_PROVIDERS_WHATSAPP_FROM_NUMBER")

	if accountSID == "" || authToken == "" || fromNumber == "" || toNumber == "" {
		s.logger.Warn("Twilio WhatsApp not configured, skipping notification WhatsApp message", zap.String("to", toNumber))
		return nil
	}

	if !strings.HasPrefix(fromNumber, "whatsapp:") {
		fromNumber = "whatsapp:" + fromNumber
	}
	if !strings.HasPrefix(toNumber, "whatsapp:") {
		toNumber = "whatsapp:" + toNumber
	}

	return s.send(accountSID, authToken, fromNumber, toNumber, body)
}

func (s *TwilioSMSSender) send(accountSID, authToken, fromNumber, toNumber, body string) error {
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", accountSID)
	data := fmt.Sprintf("To=%s&From=%s&Body=%s", toNumber, fromNumber, body)

	req, err := http.NewRequest(http.MethodPost, apiURL, strings.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create twilio request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(accountSID, authToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call twilio api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("twilio api error: status %d", resp.StatusCode)
	}

	s.logger.Info("Notification message sent via Twilio", zap.String("to", toNumber))
	return nil
}
