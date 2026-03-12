package workflow

import "context"

// Service defines the business logic interface for workflows.
type Service interface {
	// CRUD
	Create(ctx context.Context, appID string, req *CreateRequest) (*Workflow, error)
	Get(ctx context.Context, id, appID string) (*Workflow, error)
	Update(ctx context.Context, id, appID string, req *UpdateRequest) (*Workflow, error)
	Delete(ctx context.Context, id, appID string) error
	List(ctx context.Context, appID, environmentID string, limit, offset int) ([]*Workflow, int64, error)

	// Execution
	Trigger(ctx context.Context, appID string, req *TriggerRequest) (*WorkflowExecution, error)
	TriggerByTopic(ctx context.Context, appID string, req *TriggerByTopicRequest) (*TriggerByTopicResult, error)
	TriggerForUserIDs(ctx context.Context, appID, triggerID string, userIDs []string, payload map[string]any) (*TriggerForUserIDsResult, error)
	CancelExecution(ctx context.Context, executionID, appID string) error
	GetExecution(ctx context.Context, executionID, appID string) (*WorkflowExecution, error)
	ListExecutions(ctx context.Context, workflowID, appID string, limit, offset int) ([]*WorkflowExecution, int64, error)
}

// CreateRequest is the input for creating a workflow.
type CreateRequest struct {
	Name          string         `json:"name" validate:"required,min=3,max=100"`
	Description   string         `json:"description"`
	TriggerID     string         `json:"trigger_id" validate:"required,min=1,max=100"`
	Steps         []Step         `json:"steps" validate:"required,min=1,dive"`
	EnvironmentID string         `json:"environment_id,omitempty"`
	Status        WorkflowStatus `json:"status,omitempty"`
}

// UpdateRequest is the input for updating a workflow.
type UpdateRequest struct {
	Name        *string         `json:"name,omitempty" validate:"omitempty,min=3,max=100"`
	Description *string         `json:"description,omitempty"`
	Steps       []Step          `json:"steps,omitempty" validate:"omitempty,min=1,dive"`
	Status      *WorkflowStatus `json:"status,omitempty"`
}

// TriggerRequest is the input for triggering a workflow execution.
type TriggerRequest struct {
	TriggerID     string         `json:"trigger_id" validate:"required"`
	UserID        string         `json:"user_id" validate:"required"`
	Payload       map[string]any `json:"payload"`
	TransactionID string         `json:"transaction_id"`
	Overrides     map[string]any `json:"overrides"`
}

// TriggerByTopicRequest is the input for triggering a workflow for all topic subscribers.
type TriggerByTopicRequest struct {
	TriggerID string         `json:"trigger_id" validate:"required"`
	TopicID   string         `json:"topic_id" validate:"required"`
	Payload   map[string]any `json:"payload"`
}

// TriggerByTopicResult is the response from TriggerByTopic.
type TriggerByTopicResult struct {
	Triggered    int      `json:"triggered"`
	ExecutionIDs []string `json:"execution_ids"`
}

// TriggerForUserIDsResult is the response from TriggerForUserIDs (broadcast→workflow fan-out).
type TriggerForUserIDsResult struct {
	Triggered    int      `json:"triggered"`
	ExecutionIDs []string `json:"execution_ids"`
}
