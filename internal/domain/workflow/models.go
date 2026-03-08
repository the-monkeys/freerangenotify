package workflow

import (
	"time"
)

// StepType defines the kind of action a workflow step performs.
type StepType string

const (
	StepTypeChannel   StepType = "channel"   // Deliver via a channel
	StepTypeDelay     StepType = "delay"     // Wait for a duration
	StepTypeDigest    StepType = "digest"    // Aggregate events over a window
	StepTypeCondition StepType = "condition" // Branch based on a condition
)

// WorkflowStatus represents the lifecycle state of a workflow definition.
type WorkflowStatus string

const (
	WorkflowStatusDraft    WorkflowStatus = "draft"
	WorkflowStatusActive   WorkflowStatus = "active"
	WorkflowStatusInactive WorkflowStatus = "inactive"
)

// ExecutionStatus represents the lifecycle state of a single workflow run.
type ExecutionStatus string

const (
	ExecStatusRunning   ExecutionStatus = "running"
	ExecStatusPaused    ExecutionStatus = "paused"
	ExecStatusCompleted ExecutionStatus = "completed"
	ExecStatusFailed    ExecutionStatus = "failed"
	ExecStatusCancelled ExecutionStatus = "cancelled"
)

// StepStatus represents the outcome of one step within an execution.
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
)

// ConditionOperator for conditional steps.
type ConditionOperator string

const (
	OpEquals      ConditionOperator = "eq"
	OpNotEquals   ConditionOperator = "neq"
	OpContains    ConditionOperator = "contains"
	OpGreaterThan ConditionOperator = "gt"
	OpLessThan    ConditionOperator = "lt"
	OpExists      ConditionOperator = "exists"
	OpNotRead     ConditionOperator = "not_read"
)

// Workflow defines a multi-step notification pipeline.
type Workflow struct {
	ID            string         `json:"id"`
	AppID         string         `json:"app_id"`
	EnvironmentID string         `json:"environment_id,omitempty"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	TriggerID     string         `json:"trigger_id"`
	Steps         []Step         `json:"steps"`
	Status        WorkflowStatus `json:"status"`
	Version       int            `json:"version"`
	CreatedBy     string         `json:"created_by"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// Step is one node in the workflow pipeline.
type Step struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Type      StepType       `json:"type"`
	Order     int            `json:"order"`
	Config    StepConfig     `json:"config"`
	OnSuccess string         `json:"on_success,omitempty"`
	OnFailure string         `json:"on_failure,omitempty"`
	SkipIf    *Condition     `json:"skip_if,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// StepConfig holds type-specific configuration.
type StepConfig struct {
	// Channel step
	Channel    string `json:"channel,omitempty"`
	TemplateID string `json:"template_id,omitempty"`
	Provider   string `json:"provider,omitempty"`

	// Delay step
	Duration string `json:"duration,omitempty"`

	// Digest step
	DigestKey string `json:"digest_key,omitempty"`
	Window    string `json:"window,omitempty"`
	MaxBatch  int    `json:"max_batch,omitempty"`

	// Condition step
	Condition *Condition `json:"condition,omitempty"`
}

// Condition defines a conditional expression.
type Condition struct {
	Field    string            `json:"field"`
	Operator ConditionOperator `json:"operator"`
	Value    any               `json:"value"`
}

// WorkflowExecution tracks a single run of a workflow for one subscriber.
type WorkflowExecution struct {
	ID            string                `json:"id"`
	WorkflowID    string                `json:"workflow_id"`
	AppID         string                `json:"app_id"`
	EnvironmentID string                `json:"environment_id,omitempty"`
	UserID        string                `json:"user_id"`
	TransactionID string                `json:"transaction_id"`
	CurrentStepID string                `json:"current_step_id"`
	Status        ExecutionStatus       `json:"status"`
	Payload       map[string]any        `json:"payload"`
	StepResults   map[string]StepResult `json:"step_results"`
	StartedAt     time.Time             `json:"started_at"`
	CompletedAt   *time.Time            `json:"completed_at,omitempty"`
	UpdatedAt     time.Time             `json:"updated_at"`
}

// StepResult records the outcome of one step execution.
type StepResult struct {
	StepID         string     `json:"step_id"`
	Status         StepStatus `json:"status"`
	NotificationID string     `json:"notification_id,omitempty"`
	DigestCount    int        `json:"digest_count,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	Error          string     `json:"error,omitempty"`
}
