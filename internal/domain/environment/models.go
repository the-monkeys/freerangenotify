package environment

import (
	"context"
	"time"
)

// Environment represents a deployment environment within an application.
// Each application can have multiple environments (development, staging, production),
// each with its own API key, allowing isolated notification pipelines.
type Environment struct {
	ID        string    `json:"id" es:"id"`
	AppID     string    `json:"app_id" es:"app_id"`
	Name      string    `json:"name" es:"name" validate:"required,oneof=development staging production"`
	Slug      string    `json:"slug" es:"slug"`
	APIKey    string    `json:"api_key" es:"api_key"`
	IsDefault bool      `json:"is_default" es:"is_default"`
	CreatedAt time.Time `json:"created_at" es:"created_at"`
	UpdatedAt time.Time `json:"updated_at" es:"updated_at"`
}

// CreateRequest is the input for creating a new environment.
type CreateRequest struct {
	AppID string `json:"app_id" validate:"required"`
	Name  string `json:"name" validate:"required,oneof=development staging production"`
}

// PromoteRequest defines which resources to copy between environments.
type PromoteRequest struct {
	SourceEnvID string   `json:"source_env_id" validate:"required"`
	TargetEnvID string   `json:"target_env_id" validate:"required"`
	Resources   []string `json:"resources" validate:"required,min=1"`
}

// PromoteResult reports how many resources were promoted.
type PromoteResult struct {
	TemplatesPromoted int `json:"templates_promoted"`
	WorkflowsPromoted int `json:"workflows_promoted"`
}

// Repository defines persistence operations for environments.
type Repository interface {
	Create(ctx context.Context, env *Environment) error
	GetByID(ctx context.Context, id string) (*Environment, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*Environment, error)
	ListByApp(ctx context.Context, appID string) ([]Environment, error)
	Delete(ctx context.Context, id string) error
}

// Service defines business operations for environments.
type Service interface {
	Create(ctx context.Context, req CreateRequest) (*Environment, error)
	Get(ctx context.Context, id string) (*Environment, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*Environment, error)
	ListByApp(ctx context.Context, appID string) ([]Environment, error)
	Delete(ctx context.Context, id string) error
	Promote(ctx context.Context, appID string, req PromoteRequest) (*PromoteResult, error)
}
