package template

import (
	"context"
	"time"
)

// Template represents a notification template entity
type Template struct {
	ID        string    `json:"id" es:"template_id"`
	AppID     string    `json:"app_id" es:"app_id"`
	Name      string    `json:"name" es:"name"`
	Channel   string    `json:"channel" es:"channel"`           // push, email, sms, webhook
	Subject   string    `json:"subject,omitempty" es:"subject"` // for email
	Body      string    `json:"body" es:"body"`
	Variables []string  `json:"variables" es:"variables"` // template variables like {{user_name}}
	CreatedAt time.Time `json:"created_at" es:"created_at"`
	UpdatedAt time.Time `json:"updated_at" es:"updated_at"`
}

// Filter represents query filters for templates
type Filter struct {
	AppID   string `json:"app_id,omitempty"`
	Channel string `json:"channel,omitempty"`
	Name    string `json:"name,omitempty"`
	Limit   int    `json:"limit,omitempty"`
	Offset  int    `json:"offset,omitempty"`
}

// Repository defines the interface for template data operations
type Repository interface {
	Create(ctx context.Context, template *Template) error
	GetByID(ctx context.Context, id string) (*Template, error)
	Update(ctx context.Context, template *Template) error
	List(ctx context.Context, filter Filter) ([]*Template, error)
	Delete(ctx context.Context, id string) error
}

// Service defines the business logic interface for templates
type Service interface {
	Create(ctx context.Context, req *CreateRequest) (*Template, error)
	GetByID(ctx context.Context, id string) (*Template, error)
	Update(ctx context.Context, id string, req *UpdateRequest) (*Template, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter Filter) ([]*Template, error)
	Render(ctx context.Context, templateID string, variables map[string]interface{}) (string, error)
}

// CreateRequest represents a request to create a template
type CreateRequest struct {
	AppID     string   `json:"app_id" validate:"required"`
	Name      string   `json:"name" validate:"required,min=1,max=100"`
	Channel   string   `json:"channel" validate:"required,oneof=push email sms webhook"`
	Subject   string   `json:"subject,omitempty"`
	Body      string   `json:"body" validate:"required"`
	Variables []string `json:"variables,omitempty"`
}

// UpdateRequest represents a request to update a template
type UpdateRequest struct {
	Name      *string   `json:"name,omitempty" validate:"omitempty,min=1,max=100"`
	Subject   *string   `json:"subject,omitempty"`
	Body      *string   `json:"body,omitempty"`
	Variables *[]string `json:"variables,omitempty"`
}
