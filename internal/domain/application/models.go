package application

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
)

// Application represents an application entity
type Application struct {
	AppID             string            `json:"app_id" es:"app_id"`
	AppName           string            `json:"app_name" es:"app_name"`
	Description       string            `json:"description" es:"description"`
	APIKey            string            `json:"api_key" es:"api_key"`
	APIKeyGeneratedAt time.Time         `json:"api_key_generated_at" es:"api_key_generated_at"`
	AdminUserID       string            `json:"admin_user_id" es:"admin_user_id"` // The admin user who owns this app
	TenantID          string            `json:"tenant_id" es:"tenant_id"`         // Optional: app belongs to tenant (C1)
	WebhookURL        string            `json:"webhook_url,omitempty" es:"webhook_url"`
	Webhooks          map[string]string `json:"webhooks,omitempty" es:"webhooks"`
	Settings          Settings          `json:"settings" es:"settings"`
	CreatedAt         time.Time         `json:"created_at" es:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at" es:"updated_at"`
}

// Settings represents application-specific settings
type Settings struct {
	RateLimit              int                                 `json:"rate_limit" es:"rate_limit"`             // requests per hour
	RetryAttempts          int                                 `json:"retry_attempts" es:"retry_attempts"`     // max retry attempts
	DefaultTemplate        string                              `json:"default_template" es:"default_template"` // default template ID
	EmailConfig            *EmailConfig                        `json:"email_config,omitempty" es:"email_config"`
	DailyEmailLimit        int                                 `json:"daily_email_limit" es:"daily_email_limit"`
	EnableWebhooks         bool                                `json:"enable_webhooks" es:"enable_webhooks"`             // webhook notifications
	EnableAnalytics        bool                                `json:"enable_analytics" es:"enable_analytics"`           // analytics tracking
	Slack                  *SlackAppConfig                     `json:"slack,omitempty" es:"slack"`                       // Phase 3
	Discord                *DiscordAppConfig                   `json:"discord,omitempty" es:"discord"`                   // Phase 3
	WhatsApp               *WhatsAppAppConfig                  `json:"whatsapp_config,omitempty" es:"whatsapp_config"`   // Phase 3
	SMS                    *SMSAppConfig                       `json:"sms_config,omitempty" es:"sms_config"`             // Billing: per-app SMS creds
	CustomProviders        []CustomProviderConfig              `json:"custom_providers,omitempty" es:"custom_providers"` // Phase 3
	ValidationURL          string                              `json:"validation_url,omitempty" es:"validation_url"`
	ValidationConfig       *ValidationConfig                   `json:"validation_config,omitempty" es:"validation_config"`
	DefaultPreferences     *DefaultPreferences                 `json:"default_preferences,omitempty" es:"default_preferences"`
	ProviderFallbacks      []ProviderFallback                  `json:"provider_fallbacks,omitempty" es:"provider_fallbacks"`
	SubscriberThrottle     map[string]SubscriberThrottleConfig `json:"subscriber_throttle,omitempty" es:"subscriber_throttle"`               // Phase 2
	OnUserCreatedTriggerID string                              `json:"on_user_created_trigger_id,omitempty" es:"on_user_created_trigger_id"` // Phase 5: workflow to trigger on user create
	InboundWebhookConfig   *InboundWebhookConfig               `json:"inbound_webhook_config,omitempty" es:"inbound_webhook_config"`         // Phase 7: inbound webhook config (secret, event mapping)
	WhatsAppInbound        *whatsapp.InboundConfig             `json:"whatsapp_inbound,omitempty" es:"whatsapp_inbound"`                     // WhatsApp Meta inbound config
}

// InboundWebhookConfig holds configuration for receiving inbound webhooks (Phase 7)
type InboundWebhookConfig struct {
	Secret       string            `json:"secret,omitempty" es:"secret"`
	EventMapping map[string]string `json:"event_mapping,omitempty" es:"event_mapping"` // event name -> workflow trigger_id
}

// SubscriberThrottleConfig defines app-level default throttle limits
// applied to every subscriber unless overridden at the user level.
type SubscriberThrottleConfig struct {
	MaxPerHour int `json:"max_per_hour" es:"max_per_hour"`
	MaxPerDay  int `json:"max_per_day" es:"max_per_day"`
}

// ProviderFallback defines an ordered list of providers to try for a channel.
// If the primary provider fails, the system will attempt the next provider in the list.
type ProviderFallback struct {
	Channel   string   `json:"channel" es:"channel"`     // e.g. "email", "push", "sms"
	Providers []string `json:"providers" es:"providers"` // Ordered: ["sendgrid", "smtp"]
}

type EmailConfig struct {
	ProviderType string          `json:"provider_type" es:"provider_type"` // "smtp", "sendgrid", "system"
	SMTP         *SMTPConfig     `json:"smtp,omitempty" es:"smtp"`
	SendGrid     *SendGridConfig `json:"sendgrid,omitempty" es:"sendgrid"`
}

type SMTPConfig struct {
	Host      string `json:"host" es:"host"`
	Port      int    `json:"port" es:"port"`
	Username  string `json:"username" es:"username"`
	Password  string `json:"password" es:"password"`
	FromEmail string `json:"from_email" es:"from_email"`
	FromName  string `json:"from_name" es:"from_name"`
}

type SendGridConfig struct {
	APIKey    string `json:"api_key" es:"api_key"`
	FromEmail string `json:"from_email" es:"from_email"`
	FromName  string `json:"from_name" es:"from_name"`
}

// ValidationConfig represents configuration for external token validation
type ValidationConfig struct {
	Method         string            `json:"method" es:"method"`                   // GET, POST
	TokenPlacement string            `json:"token_placement" es:"token_placement"` // header, cookie, query, body_json, body_form
	TokenKey       string            `json:"token_key" es:"token_key"`             // e.g. "Authorization", "access_token"
	StaticHeaders  map[string]string `json:"static_headers,omitempty" es:"static_headers"`
}

// DefaultPreferences represents default notification preferences for the app
type DefaultPreferences struct {
	EmailEnabled    *bool `json:"email_enabled" es:"email_enabled"`
	PushEnabled     *bool `json:"push_enabled" es:"push_enabled"`
	SMSEnabled      *bool `json:"sms_enabled" es:"sms_enabled"`
	SlackEnabled    *bool `json:"slack_enabled,omitempty" es:"slack_enabled"`       // Phase 3
	DiscordEnabled  *bool `json:"discord_enabled,omitempty" es:"discord_enabled"`   // Phase 3
	WhatsAppEnabled *bool `json:"whatsapp_enabled,omitempty" es:"whatsapp_enabled"` // Phase 3
}

// SlackAppConfig holds app-level Slack configuration (Phase 3)
type SlackAppConfig struct {
	WebhookURL string `json:"webhook_url,omitempty" es:"webhook_url"`
	BotToken   string `json:"bot_token,omitempty" es:"bot_token"`
}

// DiscordAppConfig holds app-level Discord configuration (Phase 3)
type DiscordAppConfig struct {
	WebhookURL string `json:"webhook_url,omitempty" es:"webhook_url"`
}

// WhatsAppAppConfig holds per-app WhatsApp credentials (Phase 3).
// Provider field selects "twilio" (default) or "meta".
type WhatsAppAppConfig struct {
	Provider string `json:"provider,omitempty" es:"provider"` // "twilio" (default) or "meta"

	// Twilio fields (used when Provider == "" or "twilio")
	AccountSID string `json:"account_sid,omitempty" es:"account_sid"`
	AuthToken  string `json:"auth_token,omitempty" es:"auth_token"`
	FromNumber string `json:"from_number,omitempty" es:"from_number"`

	// Meta Cloud API fields (used when Provider == "meta")
	MetaPhoneNumberID string `json:"meta_phone_number_id,omitempty" es:"meta_phone_number_id"`
	MetaWABAID        string `json:"meta_waba_id,omitempty" es:"meta_waba_id"`
	MetaAccessToken   string `json:"meta_access_token,omitempty" es:"meta_access_token"`

	// Embedded Signup metadata (populated during OAuth connect flow)
	MetaBusinessID     string `json:"meta_business_id,omitempty" es:"meta_business_id"`
	ConnectionStatus   string `json:"connection_status,omitempty" es:"connection_status"` // connected, disconnected, pending
	ConnectedAt        string `json:"connected_at,omitempty" es:"connected_at"`
	DisplayPhoneNumber string `json:"display_phone_number,omitempty" es:"display_phone_number"`
	QualityRating      string `json:"quality_rating,omitempty" es:"quality_rating"`
}

// SMSAppConfig holds per-app Twilio SMS credentials
type SMSAppConfig struct {
	AccountSID string `json:"account_sid" es:"account_sid"`
	AuthToken  string `json:"auth_token" es:"auth_token"`
	FromNumber string `json:"from_number" es:"from_number"`
}

// CustomProviderConfig defines a user-registered custom delivery channel (Phase 3).
// Stored in app Settings.CustomProviders.
//
// Phase 7 (Webhook channel expansion) adds:
//   - Kind: explicit rendering adapter (generic | discord | slack | teams).
//     Empty string means "infer from WebhookURL host" for backward compatibility
//     with rows created before the field existed.
//   - SignatureVersion: "v1" (body-only HMAC, legacy default) or "v2"
//     (timestamp + body HMAC with 5-min replay window). Empty == "v1".
type CustomProviderConfig struct {
	ProviderID       string            `json:"provider_id" es:"provider_id"`
	Name             string            `json:"name" es:"name"`
	Channel          string            `json:"channel" es:"channel"`
	Kind             string            `json:"kind,omitempty" es:"kind"`
	WebhookURL       string            `json:"webhook_url" es:"webhook_url"`
	Headers          map[string]string `json:"headers,omitempty" es:"headers"`
	SigningKey       string            `json:"signing_key" es:"signing_key"`
	SignatureVersion string            `json:"signature_version,omitempty" es:"signature_version"`
	Active           bool              `json:"active" es:"active"`
	CreatedAt        string            `json:"created_at,omitempty" es:"created_at"`
}

// ApplicationFilter represents query filters for applications
type ApplicationFilter struct {
	AppName     string   `json:"app_name,omitempty"`
	AdminUserID string   `json:"admin_user_id,omitempty"`
	TenantID    string   `json:"tenant_id,omitempty"`
	TenantIDs   []string `json:"tenant_ids,omitempty"` // List apps in any of these tenants
	Limit       int      `json:"limit,omitempty"`
	Offset      int      `json:"offset,omitempty"`
	Cursor      string   `json:"cursor,omitempty"`
}

// Repository defines the interface for application data operations
type Repository interface {
	Create(ctx context.Context, app *Application) error
	GetByID(ctx context.Context, id string) (*Application, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*Application, error)
	Update(ctx context.Context, app *Application) error
	List(ctx context.Context, filter ApplicationFilter) ([]*Application, error)
	Delete(ctx context.Context, id string) error
	RegenerateAPIKey(ctx context.Context, id string) (string, error)
}

// Service defines the business logic interface for applications
type Service interface {
	Create(ctx context.Context, req *CreateRequest) (*Application, error)
	GetByID(ctx context.Context, id string) (*Application, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*Application, error)
	Update(ctx context.Context, id string, req *UpdateRequest) (*Application, error)
	Delete(ctx context.Context, id string) error
	RegenerateAPIKey(ctx context.Context, id string) (string, error)
	List(ctx context.Context, filter *ApplicationFilter) ([]*Application, error)
}

// CreateRequest represents a request to create an application
type CreateRequest struct {
	Name       string            `json:"name" validate:"required,min=1,max=100"`
	TenantID   string            `json:"tenant_id,omitempty"`
	WebhookURL string            `json:"webhook_url,omitempty" validate:"omitempty,url"`
	Webhooks   map[string]string `json:"webhooks,omitempty"`
	Settings   Settings          `json:"settings"`
}

// UpdateRequest represents a request to update an application
type UpdateRequest struct {
	Name       *string            `json:"name,omitempty" validate:"omitempty,min=1,max=100"`
	WebhookURL *string            `json:"webhook_url,omitempty" validate:"omitempty,url"`
	Webhooks   *map[string]string `json:"webhooks,omitempty"`
	Settings   *Settings          `json:"settings,omitempty"`
}
