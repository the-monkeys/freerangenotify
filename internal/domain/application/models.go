package application

import (
	"context"
	"time"
)

// Application represents an application entity
type Application struct {
	AppID             string            `json:"app_id" es:"app_id"`
	AppName           string            `json:"app_name" es:"app_name"`
	Description       string            `json:"description" es:"description"`
	APIKey            string            `json:"api_key" es:"api_key"`
	APIKeyGeneratedAt time.Time         `json:"api_key_generated_at" es:"api_key_generated_at"`
	WebhookURL        string            `json:"webhook_url,omitempty" es:"webhook_url"`
	Webhooks          map[string]string `json:"webhooks,omitempty" es:"webhooks"`
	Settings          Settings          `json:"settings" es:"settings"`
	CreatedAt         time.Time         `json:"created_at" es:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at" es:"updated_at"`
}

// Settings represents application-specific settings
type Settings struct {
	RateLimit          int                 `json:"rate_limit" es:"rate_limit"`             // requests per hour
	RetryAttempts      int                 `json:"retry_attempts" es:"retry_attempts"`     // max retry attempts
	DefaultTemplate    string              `json:"default_template" es:"default_template"` // default template ID
	EmailConfig        *EmailConfig        `json:"email_config,omitempty" es:"email_config"`
	DailyEmailLimit    int                 `json:"daily_email_limit" es:"daily_email_limit"`
	EnableWebhooks     bool                `json:"enable_webhooks" es:"enable_webhooks"`   // webhook notifications
	EnableAnalytics    bool                `json:"enable_analytics" es:"enable_analytics"` // analytics tracking
	ValidationURL      string              `json:"validation_url,omitempty" es:"validation_url"`
	ValidationConfig   *ValidationConfig   `json:"validation_config,omitempty" es:"validation_config"`
	DefaultPreferences *DefaultPreferences `json:"default_preferences,omitempty" es:"default_preferences"`
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
	EmailEnabled *bool `json:"email_enabled" es:"email_enabled"`
	PushEnabled  *bool `json:"push_enabled" es:"push_enabled"`
	SMSEnabled   *bool `json:"sms_enabled" es:"sms_enabled"`
}

// ApplicationFilter represents query filters for applications
type ApplicationFilter struct {
	AppName string `json:"app_name,omitempty"`
	Limit   int    `json:"limit,omitempty"`
	Offset  int    `json:"offset,omitempty"`
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
