package queue

import (
	"context"
	"time"
)

// WorkflowQueueItem represents a workflow execution step in the queue.
type WorkflowQueueItem struct {
	ExecutionID string    `json:"execution_id"`
	StepID      string    `json:"step_id"`
	EnqueuedAt  time.Time `json:"enqueued_at"`
}

// WorkflowQueue extends Queue with workflow-specific operations.
// Any Queue implementation that also supports workflow processing
// should implement this interface.
type WorkflowQueue interface {
	Queue // Embeds existing Queue — backward compatible

	EnqueueWorkflow(ctx context.Context, item WorkflowQueueItem) error
	DequeueWorkflow(ctx context.Context) (*WorkflowQueueItem, error)
	EnqueueWorkflowDelayed(ctx context.Context, item WorkflowQueueItem, executeAt time.Time) error
	GetDelayedWorkflowItems(ctx context.Context, limit int64) ([]WorkflowQueueItem, error)
}
