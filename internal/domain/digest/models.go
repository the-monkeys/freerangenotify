package digest

import (
	"context"
	"time"
)

// DigestRule defines how notifications should be batched before delivery.
type DigestRule struct {
	ID            string    `json:"id"`
	AppID         string    `json:"app_id"`
	EnvironmentID string    `json:"environment_id,omitempty"`
	Name          string    `json:"name"`
	DigestKey     string    `json:"digest_key"`
	Window        string    `json:"window"`
	Channel       string    `json:"channel"`
	TemplateID    string    `json:"template_id"`
	MaxBatch      int       `json:"max_batch"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Repository for digest rules.
type Repository interface {
	Create(ctx context.Context, rule *DigestRule) error
	GetByID(ctx context.Context, id string) (*DigestRule, error)
	GetActiveByKey(ctx context.Context, appID, digestKey string) (*DigestRule, error)
	List(ctx context.Context, appID, environmentID string, linkedIDs []string, limit, offset int) ([]*DigestRule, int64, error)
	Update(ctx context.Context, rule *DigestRule) error
	Delete(ctx context.Context, id string) error
}

// Service for digest operations.
type Service interface {
	Create(ctx context.Context, appID string, req *CreateRequest) (*DigestRule, error)
	Get(ctx context.Context, id, appID string) (*DigestRule, error)
	List(ctx context.Context, appID, environmentID string, linkedIDs []string, limit, offset int) ([]*DigestRule, int64, error)
	Update(ctx context.Context, id, appID string, req *UpdateRequest) (*DigestRule, error)
	Delete(ctx context.Context, id, appID string) error
}

// CreateRequest is the input for creating a digest rule.
type CreateRequest struct {
	Name          string `json:"name" validate:"required,min=3,max=100"`
	DigestKey     string `json:"digest_key" validate:"required,min=1,max=100"`
	Window        string `json:"window" validate:"required"`
	Channel       string `json:"channel" validate:"required,oneof=push email sms webhook in_app sse"`
	TemplateID    string `json:"template_id" validate:"required"`
	MaxBatch      int    `json:"max_batch,omitempty"`
	EnvironmentID string `json:"environment_id,omitempty"`
}

// UpdateRequest is the input for updating a digest rule.
type UpdateRequest struct {
	Name       *string `json:"name,omitempty" validate:"omitempty,min=3,max=100"`
	DigestKey  *string `json:"digest_key,omitempty" validate:"omitempty,min=1,max=100"`
	Window     *string `json:"window,omitempty"`
	Channel    *string `json:"channel,omitempty" validate:"omitempty,oneof=push email sms webhook in_app sse"`
	TemplateID *string `json:"template_id,omitempty"`
	MaxBatch   *int    `json:"max_batch,omitempty"`
	Status     *string `json:"status,omitempty" validate:"omitempty,oneof=active inactive"`
}
