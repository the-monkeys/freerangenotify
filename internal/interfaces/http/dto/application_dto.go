package dto

import "github.com/the-monkeys/freerangenotify/internal/domain/application"

// CreateApplicationRequest represents a request to create a new application
type CreateApplicationRequest struct {
	AppName    string                `json:"app_name" validate:"required,min=3,max=100"`
	WebhookURL string                `json:"webhook_url" validate:"omitempty,url"`
	Webhooks   map[string]string     `json:"webhooks,omitempty"`
	Settings   *application.Settings `json:"settings,omitempty"`
}

// UpdateApplicationRequest represents a request to update an application
type UpdateApplicationRequest struct {
	AppName    string                `json:"app_name" validate:"omitempty,min=3,max=100"`
	WebhookURL string                `json:"webhook_url" validate:"omitempty,url"`
	Webhooks   map[string]string     `json:"webhooks,omitempty"`
	Settings   *application.Settings `json:"settings,omitempty"`
}

// UpdateSettingsRequest represents a request to update application settings
type UpdateSettingsRequest struct {
	RateLimit          *int                          `json:"rate_limit" validate:"omitempty,min=1"`
	RetryAttempts      *int                          `json:"retry_attempts" validate:"omitempty,min=0,max=10"`
	DefaultTemplate    *string                       `json:"default_template" validate:"omitempty"`
	EmailConfig        *application.EmailConfig      `json:"email_config"`
	DailyEmailLimit    *int                          `json:"daily_email_limit" validate:"omitempty,min=0"`
	EnableWebhooks     *bool                         `json:"enable_webhooks"`
	EnableAnalytics    *bool                         `json:"enable_analytics"`
	ValidationURL      *string                       `json:"validation_url"`
	ValidationConfig   *application.ValidationConfig `json:"validation_config"`
	DefaultPreferences *DefaultPreferencesDTO        `json:"default_preferences"`
}

type DefaultPreferencesDTO struct {
	EmailEnabled *bool `json:"email_enabled"`
	PushEnabled  *bool `json:"push_enabled"`
	SMSEnabled   *bool `json:"sms_enabled"`
}

// ApplicationResponse represents an application response
type ApplicationResponse struct {
	AppID             string               `json:"app_id"`
	AppName           string               `json:"app_name"`
	APIKey            string               `json:"api_key"`
	WebhookURL        string               `json:"webhook_url,omitempty"`
	Webhooks          map[string]string    `json:"webhooks,omitempty"`
	Settings          application.Settings `json:"settings"`
	APIKeyGeneratedAt string               `json:"api_key_generated_at"`
	CreatedAt         string               `json:"created_at"`
	UpdatedAt         string               `json:"updated_at"`
}

// ListApplicationsResponse represents a paginated list of applications
type ListApplicationsResponse struct {
	Applications []ApplicationResponse `json:"applications"`
	TotalCount   int64                 `json:"total_count"`
	Page         int                   `json:"page"`
	PageSize     int                   `json:"page_size"`
}

// RegenerateAPIKeyResponse represents the response after regenerating an API key
type RegenerateAPIKeyResponse struct {
	APIKey  string `json:"api_key"`
	Message string `json:"message"`
}

// ToApplicationResponse converts an application entity to a response DTO
func ToApplicationResponse(app *application.Application) ApplicationResponse {
	return ApplicationResponse{
		AppID:             app.AppID,
		AppName:           app.AppName,
		APIKey:            app.APIKey,
		WebhookURL:        app.WebhookURL,
		Webhooks:          app.Webhooks,
		Settings:          app.Settings,
		APIKeyGeneratedAt: app.APIKeyGeneratedAt.Format("2006-01-02T15:04:05Z07:00"),
		CreatedAt:         app.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:         app.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
