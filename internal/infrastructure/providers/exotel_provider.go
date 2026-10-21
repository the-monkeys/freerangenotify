package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ExotelConfig struct {
	Config
	Snapshot       *channelconnection.Snapshot
	StatusCallback string
}
type ExotelProvider struct {
	config ExotelConfig
	client *http.Client
}

func ExotelHost(region string) (string, error) {
	switch region {
	case "mumbai":
		return "api.in.exotel.com", nil
	case "singapore":
		return "api.exotel.com", nil
	}
	return "", fmt.Errorf("invalid Exotel region")
}
func NewExotelProvider(c ExotelConfig, _ *zap.Logger) (Provider, error) {
	if c.Snapshot == nil || c.Snapshot.Exotel == nil || c.Snapshot.Provider != "exotel" || c.Snapshot.Channel != notification.ChannelSMS || c.Snapshot.CredentialSource != "byoc" || !regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`).MatchString(c.Snapshot.AccountID) || c.Snapshot.Credentials["api_key"] == "" || c.Snapshot.Credentials["api_token"] == "" {
		return nil, fmt.Errorf("Exotel account credentials required")
	}
	if _, e := ExotelHost(c.Snapshot.Exotel.Region); e != nil {
		return nil, e
	}
	client := c.HTTPClient
	if client == nil {
		timeout := c.Timeout
		if timeout <= 0 {
			timeout = 15 * time.Second
		}
		client = &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return &ExotelProvider{config: c, client: client}, nil
}
func ValidateExotelMessage(s *channelconnection.Snapshot, n *notification.Notification, u *user.User) (*channelconnection.DLTTemplate, int64, error) {
	if s == nil || s.Exotel == nil || n == nil || u == nil {
		return nil, 0, fmt.Errorf("Exotel delivery configuration required")
	}
	id, _ := n.Content.Data["dlt_template_id"].(string)
	t, e := s.Exotel.Match(s.SenderID, u.Phone, n.Content.Body, id)
	if e != nil {
		return nil, 0, e
	}
	_, units, e := billing.SMSUnits(n.Content.Body)
	if e != nil || units > s.Exotel.Limits.MaxSegments {
		return nil, 0, fmt.Errorf("SMS segment cap exceeded")
	}
	limits := s.Exotel.Limits
	if units > limits.UnitsPerMinute || units > limits.DailyUnits || units > limits.DailySpendMinor/limits.CostMinorPerUnit {
		return nil, 0, fmt.Errorf("SMS cannot fit sender unit or spend policy")
	}
	return t, units, nil
}
func (p *ExotelProvider) Send(ctx context.Context, n *notification.Notification, u *user.User) (*Result, error) {
	start := time.Now()
	s := p.config.Snapshot
	t, _, e := ValidateExotelMessage(s, n, u)
	if e != nil {
		return NewErrorResult(e, ErrorTypeInvalid), nil
	}
	host, _ := ExotelHost(s.Exotel.Region)
	values := url.Values{"From": {s.SenderID}, "To": {u.Phone}, "Body": {n.Content.Body}, "DltEntityId": {s.Exotel.EntityID}, "DltTemplateId": {t.ID}, "SmsType": {s.Exotel.SMSType}, "CustomField": {n.NotificationID}}
	if p.config.StatusCallback != "" {
		values.Set("StatusCallback", p.config.StatusCallback)
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+host+"/v1/Accounts/"+url.PathEscape(s.AccountID)+"/Sms/send.json", strings.NewReader(values.Encode()))
	if e != nil {
		return NewErrorResult(fmt.Errorf("Exotel request unavailable"), ErrorTypeConfiguration), nil
	}
	req.SetBasicAuth(s.Credentials["api_key"], s.Credentials["api_token"])
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	response, e := p.client.Do(req)
	if e != nil {
		return NewErrorResult(fmt.Errorf("Exotel acceptance unknown"), ErrorTypeNetwork), nil
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		kind := ErrorTypeProviderAPI
		if response.StatusCode == 401 || response.StatusCode == 403 {
			kind = ErrorTypeAuth
		}
		if response.StatusCode == 429 {
			kind = ErrorTypeRateLimit
		}
		result := NewErrorResult(fmt.Errorf("Exotel HTTP %d", response.StatusCode), kind)
		if response.StatusCode >= 400 && response.StatusCode < 500 {
			result.Metadata["acceptance_rejected"] = true
		}
		return result, nil
	}
	var out struct {
		Message ExotelSMSMessage `json:"SMSMessage"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&out) != nil || out.Message.Sid == "" || out.Message.AccountSid != s.AccountID {
		return NewErrorResult(fmt.Errorf("Exotel acceptance unconfirmed"), ErrorTypeUnknown), nil
	}
	switch out.Message.Status {
	case "queued", "sending", "submitted", "sent":
	default:
		return NewErrorResult(fmt.Errorf("Exotel acceptance unconfirmed"), ErrorTypeUnknown), nil
	}
	result := NewResult(out.Message.Sid, time.Since(start))
	result.Metadata["provider"] = "exotel"
	result.Metadata["credential_source"] = "byoc"
	result.Metadata["billing_channel"] = "sms"
	return result, nil
}

type ExotelSMSMessage struct {
	Sid         string `json:"Sid"`
	AccountSid  string `json:"AccountSid"`
	From        string `json:"From"`
	To          string `json:"To"`
	Body        string `json:"Body"`
	Status      string `json:"Status"`
	DateUpdated string `json:"DateUpdated"`
}

func ReadExotelSMS(ctx context.Context, client *http.Client, s *channelconnection.Snapshot, id string) (*ExotelSMSMessage, error) {
	if s == nil || s.Exotel == nil || !regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`).MatchString(id) || !regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`).MatchString(s.AccountID) {
		return nil, fmt.Errorf("invalid Exotel message identity")
	}
	host, e := ExotelHost(s.Exotel.Region)
	if e != nil {
		return nil, e
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	req, e := http.NewRequestWithContext(ctx, "GET", "https://"+host+"/v1/Accounts/"+url.PathEscape(s.AccountID)+"/SMS/Messages/"+url.PathEscape(id)+".json", nil)
	if e != nil {
		return nil, e
	}
	req.SetBasicAuth(s.Credentials["api_key"], s.Credentials["api_token"])
	response, e := client.Do(req)
	if e != nil {
		return nil, fmt.Errorf("Exotel status unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode == 429 {
		wait := time.Minute
		if seconds, e := strconv.ParseInt(response.Header.Get("Retry-After"), 10, 64); e == nil && seconds > 0 && seconds <= 3600 {
			wait = time.Duration(seconds) * time.Second
		}
		return nil, &ExotelReadThrottledError{RetryAfter: wait}
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("Exotel status unavailable (%d)", response.StatusCode)
	}
	var out struct {
		Message ExotelSMSMessage `json:"SMSMessage"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&out) != nil || out.Message.Sid != id || out.Message.AccountSid != s.AccountID {
		return nil, fmt.Errorf("Exotel status identity mismatch")
	}
	return &out.Message, nil
}
func (p *ExotelProvider) GetName() string                           { return "exotel" }
func (p *ExotelProvider) GetSupportedChannel() notification.Channel { return notification.ChannelSMS }
func (p *ExotelProvider) IsHealthy(context.Context) bool {
	return p.config.Snapshot.Exotel.Validate() == nil
}
func (p *ExotelProvider) Close() error { return nil }
func init() {
	RegisterFactory("exotel", func(c map[string]interface{}, logger *zap.Logger) (Provider, error) {
		if c["enabled"] != true {
			return nil, fmt.Errorf("Exotel disabled")
		}
		s, ok := c["snapshot"].(*channelconnection.Snapshot)
		if !ok {
			return nil, fmt.Errorf("Exotel requires an app-scoped connection")
		}
		return NewExotelProvider(ExotelConfig{Snapshot: s}, logger)
	})
}

type ExotelReadThrottledError struct{ RetryAfter time.Duration }

func (e *ExotelReadThrottledError) Error() string { return "Exotel status read throttled" }
