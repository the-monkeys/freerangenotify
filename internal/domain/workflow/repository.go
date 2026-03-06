package workflow

import (
	"context"
	"time"
)

// Repository defines data operations for workflows and their executions.
type Repository interface {
	// Workflow CRUD
	CreateWorkflow(ctx context.Context, wf *Workflow) error
	GetWorkflow(ctx context.Context, id string) (*Workflow, error)
	GetWorkflowByTrigger(ctx context.Context, appID, triggerID string) (*Workflow, error)
	UpdateWorkflow(ctx context.Context, wf *Workflow) error
	DeleteWorkflow(ctx context.Context, id string) error
	ListWorkflows(ctx context.Context, appID, environmentID string, limit, offset int) ([]*Workflow, int64, error)
	CountAll(ctx context.Context) (int64, error)
	CountByAppIDs(ctx context.Context, appIDs []string) (int64, error)

	// Execution CRUD
	CreateExecution(ctx context.Context, exec *WorkflowExecution) error
	GetExecution(ctx context.Context, id string) (*WorkflowExecution, error)
	UpdateExecution(ctx context.Context, exec *WorkflowExecution) error
	ListExecutions(ctx context.Context, workflowID string, limit, offset int) ([]*WorkflowExecution, int64, error)
	GetActiveExecutions(ctx context.Context, userID, workflowID string) ([]*WorkflowExecution, error)

	// Recovery: find executions stuck in "running" for too long
	GetStaleExecutions(ctx context.Context, staleSince time.Time, limit int) ([]*WorkflowExecution, error)
}
