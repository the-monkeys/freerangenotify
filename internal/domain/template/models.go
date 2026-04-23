package template

import (
	"context"
	"time"
)

// AttributeVar represents a template variable found inside an HTML attribute
// (e.g. src=, href=) that cannot be edited inline via contenteditable.
type AttributeVar struct {
	Name string `json:"name"`
	Type string `json:"type"` // "image", "url", "attribute"
}

// TemplateControl defines a single editable field for non-technical users.
// Controls are stored on the template and rendered as a form by the dashboard.
type TemplateControl struct {
	Key         string   `json:"key" es:"key"`                                                                                 // Variable name (e.g., "button_text")
	Label       string   `json:"label" es:"label"`                                                                             // UI label (e.g., "Call to Action Text")
	Type        string   `json:"type" es:"type" validate:"required,oneof=text textarea url color image number boolean select"` // Control type
	Default     string   `json:"default,omitempty" es:"default"`                                                               // Default value
	Placeholder string   `json:"placeholder,omitempty" es:"placeholder"`                                                       // Input placeholder
	Required    bool     `json:"required,omitempty" es:"required"`                                                             // Whether the field is required
	Options     []string `json:"options,omitempty" es:"options"`                                                               // For "select" type
	Group       string   `json:"group,omitempty" es:"group"`                                                                   // UI grouping (e.g., "Hero", "CTA", "Footer")
	HelpText    string   `json:"help_text,omitempty" es:"help_text"`                                                           // Tooltip/description
}

// ControlValues holds the user-edited values for template controls.
type ControlValues map[string]interface{}

// Template represents a notification template entity
type Template struct {
	ID            string                 `json:"id" es:"template_id"`
	AppID         string                 `json:"app_id" es:"app_id"`
	EnvironmentID string                 `json:"environment_id,omitempty" es:"environment_id"`
	Name          string                 `json:"name" es:"name"`
	Description   string                 `json:"description" es:"description"`
	Channel       string                 `json:"channel" es:"channel"`                         // push, email, sms, webhook, slack, discord, teams, whatsapp, sse
	WebhookTarget string                 `json:"webhook_target,omitempty" es:"webhook_target"` // Specific webhook target name
	PayloadKind   string                 `json:"payload_kind,omitempty" es:"payload_kind"`     // Phase 7: generic|discord|slack|teams — forces renderer for channel=webhook
	Subject       string                 `json:"subject,omitempty" es:"subject"`               // for email
	Body          string                 `json:"body" es:"body"`
	Variables     []string               `json:"variables" es:"variables"`                     // template variables like {{user_name}}
	Metadata      map[string]interface{} `json:"metadata" es:"metadata"`                       // Additional template metadata
	Controls      []TemplateControl      `json:"controls,omitempty" es:"controls"`             // Phase 6: Content control definitions
	ControlValues ControlValues          `json:"control_values,omitempty" es:"control_values"` // Phase 6: Content control values
	Version       int                    `json:"version" es:"version"`                         // Template version number
	Status        string                 `json:"status" es:"status"`                           // active, inactive, archived
	Locale        string                 `json:"locale" es:"locale"`                           // Language/locale code (e.g., "en", "es", "fr")
	CreatedBy     string                 `json:"created_by" es:"created_by"`                   // User who created the template
	UpdatedBy     string                 `json:"updated_by" es:"updated_by"`                   // User who last updated the template
	CreatedAt     time.Time              `json:"created_at" es:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at" es:"updated_at"`
}

// Filter represents query filters for templates
type Filter struct {
	AppID         string     `json:"app_id,omitempty"`
	AppIDs        []string   `json:"app_ids,omitempty"`
	LinkedIDs     []string   `json:"linked_ids,omitempty"`
	EnvironmentID string     `json:"environment_id,omitempty"`
	Channel       string     `json:"channel,omitempty"`
	Name          string     `json:"name,omitempty"`
	Status        string     `json:"status,omitempty"`
	Locale        string     `json:"locale,omitempty"`
	FromDate      *time.Time `json:"from_date,omitempty"`
	ToDate        *time.Time `json:"to_date,omitempty"`
	Limit         int        `json:"limit,omitempty"`
	Offset        int        `json:"offset,omitempty"`
	Cursor        string     `json:"cursor,omitempty"`
}

// Repository defines the interface for template data operations
type Repository interface {
	Create(ctx context.Context, template *Template) error
	GetByID(ctx context.Context, id string) (*Template, error)
	GetByAppAndName(ctx context.Context, appID, name, locale string) (*Template, error)
	Update(ctx context.Context, template *Template) error
	List(ctx context.Context, filter Filter) ([]*Template, int64, error)
	Count(ctx context.Context) (int64, error)
	CountByFilter(ctx context.Context, filter Filter) (int64, error)
	Delete(ctx context.Context, id string) error
	GetVersions(ctx context.Context, appID, name, locale string) ([]*Template, error)
	GetByVersion(ctx context.Context, appID, name, locale string, version int) (*Template, error)
	CreateVersion(ctx context.Context, template *Template) error
}

// TemplateDiff represents the difference between two template versions
type TemplateDiff struct {
	FromVersion int           `json:"from_version"`
	ToVersion   int           `json:"to_version"`
	Changes     []FieldChange `json:"changes"`
}

// FieldChange represents a single field-level change between versions
type FieldChange struct {
	Field string      `json:"field"`
	From  interface{} `json:"from"`
	To    interface{} `json:"to"`
}

// Service defines the business logic interface for templates
type Service interface {
	Create(ctx context.Context, req *CreateRequest) (*Template, error)
	GetByID(ctx context.Context, id, appID string) (*Template, error)
	GetByName(ctx context.Context, appID, name, locale string) (*Template, error)
	Update(ctx context.Context, id, appID string, req *UpdateRequest) (*Template, error)
	Delete(ctx context.Context, id, appID string) error
	List(ctx context.Context, filter Filter) ([]*Template, int64, error)
	Render(ctx context.Context, templateID, appID string, variables map[string]interface{}, editable bool) (string, []AttributeVar, error)
	CreateVersion(ctx context.Context, templateID, appID, updatedBy string) (*Template, error)
	GetVersions(ctx context.Context, appID, name, locale string) ([]*Template, error)
	GetByVersion(ctx context.Context, appID, name, locale string, version int) (*Template, error)
	Rollback(ctx context.Context, templateID, appID string, targetVersion int, updatedBy string) (*Template, error)
	Diff(ctx context.Context, appID, name, locale string, fromVersion, toVersion int) (*TemplateDiff, error)
}

// CreateRequest represents a request to create a template
type CreateRequest struct {
	AppID         string                 `json:"app_id" validate:"required"`
	Name          string                 `json:"name" validate:"required,min=1,max=100"`
	Description   string                 `json:"description"`
	Channel       string                 `json:"channel" validate:"required,oneof=push email sms webhook sse slack discord whatsapp teams"`
	WebhookTarget string                 `json:"webhook_target,omitempty"`
	PayloadKind   string                 `json:"payload_kind,omitempty" validate:"omitempty,oneof=generic discord slack teams"`
	Subject       string                 `json:"subject,omitempty"`
	Body          string                 `json:"body" validate:"required"`
	Variables     []string               `json:"variables,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Locale        string                 `json:"locale" validate:"required"`
	CreatedBy     string                 `json:"created_by"`
	EnvironmentID string                 `json:"environment_id,omitempty"`
}

// UpdateRequest represents a request to update a template
type UpdateRequest struct {
	Name          *string                `json:"name,omitempty" validate:"omitempty,min=1,max=100"`
	Description   *string                `json:"description,omitempty"`
	WebhookTarget *string                `json:"webhook_target,omitempty"`
	PayloadKind   *string                `json:"payload_kind,omitempty" validate:"omitempty,oneof=generic discord slack teams"`
	Subject       *string                `json:"subject,omitempty"`
	Body          *string                `json:"body,omitempty"`
	Variables     *[]string              `json:"variables,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Controls      *[]TemplateControl     `json:"controls,omitempty"`       // Phase 6: update control definitions
	ControlValues ControlValues          `json:"control_values,omitempty"` // Phase 6: update control values
	Status        *string                `json:"status,omitempty" validate:"omitempty,oneof=active inactive archived"`
	UpdatedBy     string                 `json:"updated_by"`
}
