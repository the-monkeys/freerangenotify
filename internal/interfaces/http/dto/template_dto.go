package dto

import (
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

// CreateTemplateRequest represents a request to create a new template
type CreateTemplateRequest struct {
	AppID         string                 `json:"app_id" validate:"required"`
	Name          string                 `json:"name" validate:"required,min=3,max=100"`
	Description   string                 `json:"description"`
	Channel       string                 `json:"channel" validate:"required,oneof=push email sms webhook in_app sse"`
	WebhookTarget string                 `json:"webhook_target,omitempty"`
	Subject       string                 `json:"subject"`
	Body          string                 `json:"body" validate:"required"`
	Variables     []string               `json:"variables"`
	Metadata      map[string]interface{} `json:"metadata"`
	Locale        string                 `json:"locale" validate:"omitempty,min=2,max=10"`
	CreatedBy     string                 `json:"created_by"`
} // @name CreateTemplateRequest

// UpdateTemplateRequest represents a request to update an existing template
type UpdateTemplateRequest struct {
	Description   string                 `json:"description"`
	WebhookTarget string                 `json:"webhook_target,omitempty"`
	Subject       string                 `json:"subject"`
	Body          string                 `json:"body"`
	Variables     []string               `json:"variables"`
	Metadata      map[string]interface{} `json:"metadata"`
	Status        string                 `json:"status" validate:"omitempty,oneof=active inactive archived"`
	Locale        string                 `json:"locale" validate:"omitempty,min=2,max=10"`
	UpdatedBy     string                 `json:"updated_by"`
} // @name UpdateTemplateRequest

// TemplateResponse represents a template in API responses
type TemplateResponse struct {
	ID            string                 `json:"id"`
	AppID         string                 `json:"app_id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Channel       string                 `json:"channel"`
	WebhookTarget string                 `json:"webhook_target,omitempty"`
	Subject       string                 `json:"subject"`
	Body          string                 `json:"body"`
	Variables     []string               `json:"variables"`
	Metadata      map[string]interface{} `json:"metadata"`
	Version       int                    `json:"version"`
	Status        string                 `json:"status"`
	Locale        string                 `json:"locale"`
	CreatedBy     string                 `json:"created_by"`
	UpdatedBy     string                 `json:"updated_by"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
} // @name TemplateResponse

// ListTemplatesRequest represents query parameters for listing templates
type ListTemplatesRequest struct {
	AppID    string `query:"app_id"`
	Channel  string `query:"channel" validate:"omitempty,oneof=push email sms webhook in_app sse"`
	Name     string `query:"name"`
	Status   string `query:"status" validate:"omitempty,oneof=active inactive archived"`
	Locale   string `query:"locale"`
	FromDate string `query:"from_date"` // RFC3339 format
	ToDate   string `query:"to_date"`   // RFC3339 format
	Limit    int    `query:"limit" validate:"omitempty,min=1,max=100"`
	Offset   int    `query:"offset" validate:"omitempty,min=0"`
} // @name ListTemplatesRequest

// ListTemplatesResponse represents the response for list templates
type ListTemplatesResponse struct {
	Templates []*TemplateResponse `json:"templates"`
	Total     int                 `json:"total"`
	Limit     int                 `json:"limit"`
	Offset    int                 `json:"offset"`
} // @name ListTemplatesResponse

// RenderTemplateRequest represents a request to render a template
type RenderTemplateRequest struct {
	Data map[string]interface{} `json:"data" validate:"required"`
} // @name RenderTemplateRequest

// RenderTemplateResponse represents the response for template rendering
type RenderTemplateResponse struct {
	RenderedBody string `json:"rendered_body"`
} // @name RenderTemplateResponse

// CreateVersionRequest represents a request to create a new template version
type CreateVersionRequest struct {
	Description string                 `json:"description"`
	Subject     string                 `json:"subject"`
	Body        string                 `json:"body" validate:"required"`
	Variables   []string               `json:"variables"`
	Metadata    map[string]interface{} `json:"metadata"`
	Locale      string                 `json:"locale" validate:"omitempty,min=2,max=10"`
	CreatedBy   string                 `json:"created_by"`
} // @name CreateVersionRequest

// Helper functions to convert between domain and DTO

func ToTemplateResponse(tmpl interface{}) *TemplateResponse {
	// Type assertion with interface for template domain
	type Template struct {
		ID          string
		AppID       string
		Name        string
		Description string
		Channel     notification.Channel
		Subject     string
		Body        string
		Variables   []string
		Metadata    map[string]interface{}
		Version     int
		Status      interface{ String() string }
		Locale      string
		CreatedBy   string
		UpdatedBy   string
		CreatedAt   time.Time
		UpdatedAt   time.Time
	}

	t, ok := tmpl.(Template)
	if !ok {
		// Try pointer type
		tPtr, ok := tmpl.(*Template)
		if !ok {
			return nil
		}
		t = *tPtr
	}

	return &TemplateResponse{
		ID:          t.ID,
		AppID:       t.AppID,
		Name:        t.Name,
		Description: t.Description,
		Channel:     string(t.Channel),
		Subject:     t.Subject,
		Body:        t.Body,
		Variables:   t.Variables,
		Metadata:    t.Metadata,
		Version:     t.Version,
		Status:      t.Status.String(),
		Locale:      t.Locale,
		CreatedBy:   t.CreatedBy,
		UpdatedBy:   t.UpdatedBy,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

func ToTemplateListResponse(templates interface{}, limit, offset int) *ListTemplatesResponse {
	// Use type assertion for slice of templates
	var responses []*TemplateResponse

	// This will be populated by the handler with actual domain objects
	responses = make([]*TemplateResponse, 0)

	return &ListTemplatesResponse{
		Templates: responses,
		Total:     len(responses),
		Limit:     limit,
		Offset:    offset,
	}
}
