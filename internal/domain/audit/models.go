package audit

import (
	"context"
	"time"
)

// AuditLog represents an immutable record of a state-changing operation.
// Audit logs are append-only — no updates or deletes are permitted.
type AuditLog struct {
	AuditID       string                 `json:"audit_id" es:"audit_id"`
	AppID         string                 `json:"app_id" es:"app_id"`
	EnvironmentID string                 `json:"environment_id,omitempty" es:"environment_id"`
	ActorID       string                 `json:"actor_id" es:"actor_id"`     // User or API key that performed the action
	ActorType     string                 `json:"actor_type" es:"actor_type"` // "user", "api_key", "system"
	Action        string                 `json:"action" es:"action"`         // "create", "update", "delete", "send"
	Resource      string                 `json:"resource" es:"resource"`     // "notification", "template", "user", "application", etc.
	ResourceID    string                 `json:"resource_id" es:"resource_id"`
	Changes       map[string]interface{} `json:"changes,omitempty" es:"changes"` // Diff or relevant payload snapshot
	IPAddress     string                 `json:"ip_address,omitempty" es:"ip_address"`
	UserAgent     string                 `json:"user_agent,omitempty" es:"user_agent"`
	CreatedAt     time.Time              `json:"created_at" es:"created_at"`
}

// Filter represents query parameters for listing audit logs.
type Filter struct {
	AppID         string   `json:"app_id,omitempty"`
	AppIDs        []string `json:"app_ids,omitempty"` // Multi-tenancy: restrict to these apps
	EnvironmentID string   `json:"environment_id,omitempty"`
	ActorID       string   `json:"actor_id,omitempty"`
	Action        string   `json:"action,omitempty"`
	Resource      string   `json:"resource,omitempty"`
	ResourceID    string   `json:"resource_id,omitempty"`
	Limit         int      `json:"limit,omitempty"`
	Offset        int      `json:"offset,omitempty"`
}

// DefaultFilter returns a filter with sensible defaults.
func DefaultFilter() Filter {
	return Filter{
		Limit:  50,
		Offset: 0,
	}
}

// Repository defines data access for audit logs (append-only).
type Repository interface {
	Create(ctx context.Context, log *AuditLog) error
	GetByID(ctx context.Context, id string) (*AuditLog, error)
	List(ctx context.Context, filter Filter) ([]*AuditLog, error)
	Count(ctx context.Context, filter Filter) (int64, error)
}

// Service defines the business logic interface for audit logging.
type Service interface {
	// Record creates a new audit log entry (fire-and-forget safe).
	Record(ctx context.Context, log *AuditLog) error
	// Get retrieves a single audit log by ID.
	Get(ctx context.Context, id string) (*AuditLog, error)
	// List returns audit logs matching the given filter.
	List(ctx context.Context, filter Filter) ([]*AuditLog, error)
}
