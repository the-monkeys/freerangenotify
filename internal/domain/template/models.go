package template

import (
	"context"
	"time"
)

// Template represents a notification template entity
type Template struct {
	ID            string                 `json:"id" es:"template_id"`
	AppID         string                 `json:"app_id" es:"app_id"`
	Name          string                 `json:"name" es:"name"`
	Description   string                 `json:"description" es:"description"`
	Channel       string                 `json:"channel" es:"channel"`                         // push, email, sms, webhook
	WebhookTarget string                 `json:"webhook_target,omitempty" es:"webhook_target"` // Specific webhook target name
	Subject       string                 `json:"subject,omitempty" es:"subject"`               // for email
	Body          string                 `json:"body" es:"body"`
	Variables     []string               `json:"variables" es:"variables"`   // template variables like {{user_name}}
	Metadata      map[string]interface{} `json:"metadata" es:"metadata"`     // Additional template metadata
	Version       int                    `json:"version" es:"version"`       // Template version number
	Status        string                 `json:"status" es:"status"`         // active, inactive, archived
	Locale        string                 `json:"locale" es:"locale"`         // Language/locale code (e.g., "en", "es", "fr")
	CreatedBy     string                 `json:"created_by" es:"created_by"` // User who created the template
	UpdatedBy     string                 `json:"updated_by" es:"updated_by"` // User who last updated the template
	CreatedAt     time.Time              `json:"created_at" es:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at" es:"updated_at"`
}

// Filter represents query filters for templates
type Filter struct {
	AppID    string     `json:"app_id,omitempty"`
	Channel  string     `json:"channel,omitempty"`
	Name     string     `json:"name,omitempty"`
	Status   string     `json:"status,omitempty"`
	Locale   string     `json:"locale,omitempty"`
	FromDate *time.Time `json:"from_date,omitempty"`
	ToDate   *time.Time `json:"to_date,omitempty"`
	Limit    int        `json:"limit,omitempty"`
	Offset   int        `json:"offset,omitempty"`
}

// Repository defines the interface for template data operations
type Repository interface {
	Create(ctx context.Context, template *Template) error
	GetByID(ctx context.Context, id string) (*Template, error)
	GetByAppAndName(ctx context.Context, appID, name, locale string) (*Template, error)
	Update(ctx context.Context, template *Template) error
	List(ctx context.Context, filter Filter) ([]*Template, error)
	Delete(ctx context.Context, id string) error
	GetVersions(ctx context.Context, appID, name, locale string) ([]*Template, error)
	CreateVersion(ctx context.Context, template *Template) error
}

// Service defines the business logic interface for templates
type Service interface {
	Create(ctx context.Context, req *CreateRequest) (*Template, error)
	GetByID(ctx context.Context, id, appID string) (*Template, error)
	GetByName(ctx context.Context, appID, name, locale string) (*Template, error)
	Update(ctx context.Context, id, appID string, req *UpdateRequest) (*Template, error)
	Delete(ctx context.Context, id, appID string) error
	List(ctx context.Context, filter Filter) ([]*Template, error)
	Render(ctx context.Context, templateID, appID string, variables map[string]interface{}) (string, error)
	CreateVersion(ctx context.Context, templateID, appID, updatedBy string) (*Template, error)
	GetVersions(ctx context.Context, appID, name, locale string) ([]*Template, error)
}

// CreateRequest represents a request to create a template
type CreateRequest struct {
	AppID         string                 `json:"app_id" validate:"required"`
	Name          string                 `json:"name" validate:"required,min=1,max=100"`
	Description   string                 `json:"description"`
	Channel       string                 `json:"channel" validate:"required,oneof=push email sms webhook"`
	WebhookTarget string                 `json:"webhook_target,omitempty"`
	Subject       string                 `json:"subject,omitempty"`
	Body          string                 `json:"body" validate:"required"`
	Variables     []string               `json:"variables,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Locale        string                 `json:"locale" validate:"required"`
	CreatedBy     string                 `json:"created_by"`
}

// UpdateRequest represents a request to update a template
type UpdateRequest struct {
	Name          *string                `json:"name,omitempty" validate:"omitempty,min=1,max=100"`
	Description   *string                `json:"description,omitempty"`
	WebhookTarget *string                `json:"webhook_target,omitempty"`
	Subject       *string                `json:"subject,omitempty"`
	Body          *string                `json:"body,omitempty"`
	Variables     *[]string              `json:"variables,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Status        *string                `json:"status,omitempty" validate:"omitempty,oneof=active inactive archived"`
	UpdatedBy     string                 `json:"updated_by"`
}
