package providers

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

// SESConfig holds AWS SES-specific configuration.
type SESConfig struct {
	Config
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	FromEmail       string
	FromName        string
}

// SESProvider implements the Provider interface for email via AWS Simple Email Service.
type SESProvider struct {
	config SESConfig
	client *sesv2.Client
	logger *zap.Logger
}

// NewSESProvider creates a new AWS SES provider.
func NewSESProvider(cfg SESConfig, logger *zap.Logger) (Provider, error) {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	optFns := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		optFns = append(optFns, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "",
		)))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := sesv2.NewFromConfig(awsCfg)
	return &SESProvider{
		config: cfg,
		client: client,
		logger: logger,
	}, nil
}

// Send sends an email via AWS SES.
func (p *SESProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	start := time.Now()

	p.logger.Info("Sending AWS SES email",
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

	htmlBody := p.buildHTMLBody(notif)
	textBody := notif.Content.Body
	if textBody == "" {
		textBody = notif.Content.Title
	}

	input := &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(from),
		Destination: &types.Destination{
			ToAddresses: []string{usr.Email},
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data: aws.String(notif.Content.Title),
				},
				Body: &types.Body{
					Html: &types.Content{Data: aws.String(htmlBody)},
					Text: &types.Content{Data: aws.String(textBody)},
				},
			},
		},
	}

	resp, err := p.client.SendEmail(ctx, input)
	if err != nil {
		p.logger.Error("AWS SES send failed",
			zap.String("notification_id", notif.NotificationID),
			zap.Error(err))
		return NewErrorResult(fmt.Errorf("AWS SES send failed: %w", err), ErrorTypeProviderAPI), nil
	}

	deliveryTime := time.Since(start)
	p.logger.Info("AWS SES email sent successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.String("message_id", aws.ToString(resp.MessageId)),
		zap.Duration("delivery_time", deliveryTime))

	res := NewResult("ses-"+aws.ToString(resp.MessageId), deliveryTime)
	res.Metadata["credential_source"] = CredSourceSystem
	res.Metadata["billing_channel"] = "email"
	res.Metadata["to_email"] = usr.Email
	res.Metadata["from_email"] = p.config.FromEmail
	res.Metadata["message_id"] = aws.ToString(resp.MessageId)
	return res, nil
}

func (p *SESProvider) GetName() string                           { return "ses" }
func (p *SESProvider) GetSupportedChannel() notification.Channel { return notification.ChannelEmail }
func (p *SESProvider) IsHealthy(_ context.Context) bool          { return p.config.FromEmail != "" }
func (p *SESProvider) Close() error                               { return nil }

func (p *SESProvider) buildHTMLBody(notif *notification.Notification) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><title>%s</title></head>
<body style="font-family: sans-serif; line-height: 1.6; color: #333;">
    <div style="max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #eee; border-radius: 5px;">
        <h1 style="color: #444; margin-top: 0;">%s</h1>
        <div style="margin-bottom: 20px;">%s</div>
        <hr style="border: 0; border-top: 1px solid #eee;" />
        <footer style="font-size: 12px; color: #888;">Sent by FreeRangeNotify via AWS SES</footer>
    </div>
</body>
</html>`, notif.Content.Title, notif.Content.Title, notif.Content.Body)
}

func init() {
	RegisterFactory("ses", func(cfg map[string]interface{}, logger *zap.Logger) (Provider, error) {
		enabled, _ := cfg["enabled"].(bool)
		if !enabled {
			return nil, fmt.Errorf("ses: provider disabled")
		}
		region, _ := cfg["region"].(string)
		if region == "" {
			region = "us-east-1"
		}
		accessKeyID, _ := cfg["access_key_id"].(string)
		secretAccessKey, _ := cfg["secret_access_key"].(string)
		fromEmail, _ := cfg["from_email"].(string)
		if fromEmail == "" {
			return nil, fmt.Errorf("ses: from_email is required")
		}
		fromName, _ := cfg["from_name"].(string)

		return NewSESProvider(SESConfig{
			Config:          Config{Timeout: 15 * time.Second, MaxRetries: 3, RetryDelay: 1 * time.Second},
			Region:          region,
			AccessKeyID:     accessKeyID,
			SecretAccessKey: secretAccessKey,
			FromEmail:       fromEmail,
			FromName:        fromName,
		}, logger)
	})
}
