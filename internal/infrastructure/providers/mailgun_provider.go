package providers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/attachment"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

// mailgunAPIBase is the Mailgun API base URL. Var (not const) so tests can
// point it at a stub server. Treat as read-only outside of test code.
var mailgunAPIBase = "https://api.mailgun.net"

// MailgunConfig holds Mailgun-specific configuration.
type MailgunConfig struct {
	Config
	APIKey    string
	Domain    string
	FromEmail string
	FromName  string
}

// MailgunProvider implements the Provider interface for email via Mailgun.
type MailgunProvider struct {
	config     MailgunConfig
	httpClient *http.Client
	logger     *zap.Logger
}

// NewMailgunProvider creates a new Mailgun provider.
func NewMailgunProvider(config MailgunConfig, logger *zap.Logger) (Provider, error) {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	return &MailgunProvider{
		config:     config,
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
	}, nil
}

// Send sends an email via Mailgun.
func (p *MailgunProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	start := time.Now()

	p.logger.Info("Sending Mailgun email",
		zap.String("notification_id", notif.NotificationID),
		zap.String("user_id", usr.UserID),
		zap.String("to_email", usr.Email))

	if usr.Email == "" {
		return NewErrorResult(
			fmt.Errorf("no email address for user %s", usr.UserID),
			ErrorTypeInvalid,
		), nil
	}

	from := p.config.FromEmail
	if p.config.FromName != "" {
		from = fmt.Sprintf("%s <%s>", p.config.FromName, p.config.FromEmail)
	}

	apiURL := fmt.Sprintf("%s/v3/%s/messages", mailgunAPIBase, p.config.Domain)

	// Resolve attachments. When any are present we must POST multipart/form-data
	// so the binary parts ride alongside the text fields; otherwise we keep
	// the historical x-www-form-urlencoded fast path for byte-stable behaviour.
	resolved, _, rErr := resolveEmailAttachments(ctx, notif, p.logger, "mailgun")
	if rErr != nil {
		return NewErrorResult(rErr, ErrorTypeInvalid), nil
	}
	if resolved != nil {
		defer attachment.CloseAll(resolved)
	}

	var req *http.Request
	if len(resolved) == 0 {
		form := url.Values{}
		form.Set("from", from)
		form.Set("to", usr.Email)
		form.Set("subject", notif.Content.Title)
		form.Set("html", p.buildHTMLBody(notif))
		form.Set("text", notif.Content.Body)

		var rerr error
		req, rerr = http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
		if rerr != nil {
			return NewErrorResult(fmt.Errorf("failed to create Mailgun request: %w", rerr), ErrorTypeUnknown), nil
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		buf := &bytes.Buffer{}
		mw := multipart.NewWriter(buf)
		_ = mw.WriteField("from", from)
		_ = mw.WriteField("to", usr.Email)
		_ = mw.WriteField("subject", notif.Content.Title)
		_ = mw.WriteField("html", p.buildHTMLBody(notif))
		_ = mw.WriteField("text", notif.Content.Body)

		for _, ra := range resolved {
			raw, bErr := readResolvedBytes(ra)
			if bErr != nil {
				_ = mw.Close()
				return NewErrorResult(bErr, ErrorTypeInvalid), nil
			}
			// Mailgun: `attachment` field for regular, `inline` field for
			// HTML-embedded parts (the cid: reference is just the filename).
			fieldName := "attachment"
			if ra.Disposition == "inline" && ra.ContentID != "" {
				fieldName = "inline"
			}
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, fieldName, coalesceFilename(ra.Filename)))
			h.Set("Content-Type", coalesceMIME(ra.MIMEType))
			part, pErr := mw.CreatePart(h)
			if pErr != nil {
				_ = mw.Close()
				return NewErrorResult(fmt.Errorf("mailgun: create part: %w", pErr), ErrorTypeUnknown), nil
			}
			if _, wErr := part.Write(raw); wErr != nil {
				_ = mw.Close()
				return NewErrorResult(fmt.Errorf("mailgun: write part: %w", wErr), ErrorTypeUnknown), nil
			}
		}
		if cErr := mw.Close(); cErr != nil {
			return NewErrorResult(fmt.Errorf("mailgun: close multipart: %w", cErr), ErrorTypeUnknown), nil
		}

		var rerr error
		req, rerr = http.NewRequestWithContext(ctx, http.MethodPost, apiURL, buf)
		if rerr != nil {
			return NewErrorResult(fmt.Errorf("failed to create Mailgun request: %w", rerr), ErrorTypeUnknown), nil
		}
		req.Header.Set("Content-Type", mw.FormDataContentType())
	}
	req.SetBasicAuth("api", p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewErrorResult(fmt.Errorf("Mailgun API request failed: %w", err), ErrorTypeNetwork), nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		p.logger.Error("Mailgun API error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("body", string(respBody)))
		return NewErrorResult(
			fmt.Errorf("Mailgun API error: %d - %s", resp.StatusCode, string(respBody)),
			ErrorTypeProviderAPI,
		), nil
	}

	deliveryTime := time.Since(start)
	p.logger.Info("Mailgun email sent successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.Duration("delivery_time", deliveryTime))

	res := NewResult("mailgun-"+notif.NotificationID, deliveryTime)
	res.Metadata["credential_source"] = CredSourceSystem
	res.Metadata["billing_channel"] = "email"
	res.Metadata["to_email"] = usr.Email
	res.Metadata["from_email"] = p.config.FromEmail
	return res, nil
}

func (p *MailgunProvider) GetName() string                                { return "mailgun" }
func (p *MailgunProvider) GetSupportedChannel() notification.Channel      { return notification.ChannelEmail }
func (p *MailgunProvider) IsHealthy(_ context.Context) bool               { return p.config.APIKey != "" && p.config.Domain != "" }
func (p *MailgunProvider) Close() error                                   { return nil }

func (p *MailgunProvider) buildHTMLBody(notif *notification.Notification) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><title>%s</title></head>
<body style="font-family: sans-serif; line-height: 1.6; color: #333;">
    <div style="max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #eee; border-radius: 5px;">
        <h1 style="color: #444; margin-top: 0;">%s</h1>
        <div style="margin-bottom: 20px;">%s</div>
        <hr style="border: 0; border-top: 1px solid #eee;" />
        <footer style="font-size: 12px; color: #888;">Sent by FreeRangeNotify</footer>
    </div>
</body>
</html>`, notif.Content.Title, notif.Content.Title, notif.Content.Body)
}

func init() {
	RegisterFactory("mailgun", func(cfg map[string]interface{}, logger *zap.Logger) (Provider, error) {
		enabled, _ := cfg["enabled"].(bool)
		if !enabled {
			return nil, fmt.Errorf("mailgun: provider disabled")
		}
		apiKey, _ := cfg["api_key"].(string)
		if apiKey == "" {
			return nil, fmt.Errorf("mailgun: api_key is required")
		}
		domain, _ := cfg["domain"].(string)
		if domain == "" {
			return nil, fmt.Errorf("mailgun: domain is required")
		}
		fromEmail, _ := cfg["from_email"].(string)
		fromName, _ := cfg["from_name"].(string)
		return NewMailgunProvider(MailgunConfig{
			Config:    Config{Timeout: 15 * time.Second, MaxRetries: 3, RetryDelay: 1 * time.Second},
			APIKey:    apiKey,
			Domain:    domain,
			FromEmail: fromEmail,
			FromName:  fromName,
		}, logger)
	})
}
