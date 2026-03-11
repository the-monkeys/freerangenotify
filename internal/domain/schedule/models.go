package schedule

import (
	"context"
	"time"
)

// TargetType defines who receives the workflow trigger
type TargetType string

const (
	TargetAll   TargetType = "all"   // All app users
	TargetTopic TargetType = "topic" // Topic subscribers
)

// WorkflowSchedule represents a scheduled workflow run
type WorkflowSchedule struct {
	ID                string         `json:"id"`
	AppID             string         `json:"app_id"`
	EnvironmentID     string         `json:"environment_id,omitempty"`
	Name              string         `json:"name"`
	WorkflowTriggerID string         `json:"workflow_trigger_id"`
	Cron              string         `json:"cron"` // Standard 5-field cron: "0 9 * * 1" = Monday 9am
	Timezone          string         `json:"timezone,omitempty"` // IANA timezone, e.g. "America/New_York"; empty = UTC
	TargetType        TargetType     `json:"target_type"`
	TopicID           string         `json:"topic_id,omitempty"` // When target_type=topic
	Payload           map[string]any `json:"payload,omitempty"`
	Status            string         `json:"status"` // active, inactive
	LastRunAt         *time.Time     `json:"last_run_at,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// Repository defines data access for workflow schedules
type Repository interface {
	Create(ctx context.Context, s *WorkflowSchedule) error
	GetByID(ctx context.Context, id string) (*WorkflowSchedule, error)
	List(ctx context.Context, appID, environmentID string, limit, offset int) ([]*WorkflowSchedule, int64, error)
	ListDue(ctx context.Context, at time.Time) ([]*WorkflowSchedule, error) // Schedules whose cron matches at (minute granularity)
	Update(ctx context.Context, s *WorkflowSchedule) error
	Delete(ctx context.Context, id string) error
}

// CreateRequest is the input for creating a schedule
type CreateRequest struct {
	Name              string         `json:"name" validate:"required,min=1,max=100"`
	WorkflowTriggerID string         `json:"workflow_trigger_id" validate:"required"`
	Cron              string         `json:"cron" validate:"required"`
	Timezone          string         `json:"timezone,omitempty"` // IANA timezone; empty = UTC
	TargetType        TargetType     `json:"target_type" validate:"required,oneof=all topic"`
	TopicID           string         `json:"topic_id,omitempty"` // Required when target_type=topic
	Payload           map[string]any `json:"payload,omitempty"`
	EnvironmentID     string         `json:"environment_id,omitempty"`
}

// UpdateRequest is the input for updating a schedule
type UpdateRequest struct {
	Name              *string        `json:"name,omitempty" validate:"omitempty,min=1,max=100"`
	WorkflowTriggerID *string        `json:"workflow_trigger_id,omitempty"`
	Cron              *string        `json:"cron,omitempty"`
	Timezone          *string        `json:"timezone,omitempty"`
	TargetType        *TargetType    `json:"target_type,omitempty"`
	TopicID           *string        `json:"topic_id,omitempty"`
	Payload           map[string]any `json:"payload,omitempty"`
	Status            *string        `json:"status,omitempty" validate:"omitempty,oneof=active inactive"`
}

// Service defines business logic for workflow schedules
type Service interface {
	Create(ctx context.Context, appID string, req *CreateRequest) (*WorkflowSchedule, error)
	Get(ctx context.Context, id, appID string) (*WorkflowSchedule, error)
	List(ctx context.Context, appID, environmentID string, limit, offset int) ([]*WorkflowSchedule, int64, error)
	Update(ctx context.Context, id, appID string, req *UpdateRequest) (*WorkflowSchedule, error)
	Delete(ctx context.Context, id, appID string) error
}
