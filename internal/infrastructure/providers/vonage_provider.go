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

const vonageAPIURL = "https://rest.nexmo.com/sms/json"

// VonageConfig holds Vonage-specific configuration.
type VonageConfig struct {
	Config
	APIKey     string
	APISecret  string
	FromNumber string
}

// VonageProvider implements the Provider interface for SMS via Vonage/Nexmo.
type VonageProvider struct {
	config     VonageConfig
	httpClient *http.Client
	logger     *zap.Logger
}

// NewVonageProvider creates a new Vonage provider.
func NewVonageProvider(config VonageConfig, logger *zap.Logger) (Provider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &VonageProvider{
		config:     config,
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
	}, nil
}

// Send sends an SMS via Vonage.
func (p *VonageProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	start := time.Now()

	p.logger.Info("Sending Vonage SMS",
		zap.String("notification_id", notif.NotificationID),
		zap.String("user_id", usr.UserID),
		zap.String("to_phone", usr.Phone))

	if usr.Phone == "" {
		return NewErrorResult(
			fmt.Errorf("no phone number for user %s", usr.UserID),
			ErrorTypeInvalid,
		), nil
	}

	smsText := notif.Content.Body
	if notif.Content.Title != "" {
		smsText = notif.Content.Title + ": " + notif.Content.Body
	}

	payload := map[string]string{
		"api_key":    p.config.APIKey,
		"api_secret": p.config.APISecret,
		"from":       p.config.FromNumber,
		"to":         usr.Phone,
		"text":       smsText,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to marshal Vonage payload: %w", err), ErrorTypeInvalid), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, vonageAPIURL, bytes.NewReader(body))
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to create Vonage request: %w", err), ErrorTypeUnknown), nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewErrorResult(fmt.Errorf("Vonage API request failed: %w", err), ErrorTypeNetwork), nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		p.logger.Error("Vonage API HTTP error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("body", string(respBody)))
		return NewErrorResult(
			fmt.Errorf("Vonage API error: %d - %s", resp.StatusCode, string(respBody)),
			ErrorTypeProviderAPI,
		), nil
	}

	var vonageResp struct {
		Messages []struct {
			Status    string `json:"status"`
			MessageID string `json:"message-id"`
			ErrorText string `json:"error-text"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(respBody, &vonageResp); err != nil {
		return NewErrorResult(fmt.Errorf("failed to parse Vonage response: %w", err), ErrorTypeProviderAPI), nil
	}

	if len(vonageResp.Messages) > 0 && vonageResp.Messages[0].Status != "0" {
		return NewErrorResult(
			fmt.Errorf("Vonage send failed: status=%s error=%s", vonageResp.Messages[0].Status, vonageResp.Messages[0].ErrorText),
			ErrorTypeProviderAPI,
		), nil
	}

	messageID := ""
	if len(vonageResp.Messages) > 0 {
		messageID = vonageResp.Messages[0].MessageID
	}

	deliveryTime := time.Since(start)
	p.logger.Info("Vonage SMS sent successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.String("message_id", messageID),
		zap.Duration("delivery_time", deliveryTime))

	res := NewResult("vonage-"+messageID, deliveryTime)
	res.Metadata["credential_source"] = CredSourceSystem
	res.Metadata["billing_channel"] = "sms"
	res.Metadata["to_phone"] = usr.Phone
	res.Metadata["from_number"] = p.config.FromNumber
	return res, nil
}

func (p *VonageProvider) GetName() string                                { return "vonage" }
func (p *VonageProvider) GetSupportedChannel() notification.Channel      { return notification.ChannelSMS }
func (p *VonageProvider) IsHealthy(_ context.Context) bool               { return p.config.APIKey != "" && p.config.APISecret != "" }
func (p *VonageProvider) Close() error                                   { return nil }

func init() {
	RegisterFactory("vonage", func(cfg map[string]interface{}, logger *zap.Logger) (Provider, error) {
		enabled, _ := cfg["enabled"].(bool)
		if !enabled {
			return nil, fmt.Errorf("vonage: provider disabled")
		}
		apiKey, _ := cfg["api_key"].(string)
		if apiKey == "" {
			return nil, fmt.Errorf("vonage: api_key is required")
		}
		apiSecret, _ := cfg["api_secret"].(string)
		if apiSecret == "" {
			return nil, fmt.Errorf("vonage: api_secret is required")
		}
		fromNumber, _ := cfg["from_number"].(string)
		return NewVonageProvider(VonageConfig{
			Config:     Config{Timeout: 10 * time.Second, MaxRetries: 3, RetryDelay: 1 * time.Second},
			APIKey:     apiKey,
			APISecret:  apiSecret,
			FromNumber: fromNumber,
		}, logger)
	})
}
