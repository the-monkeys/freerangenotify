package freerangenotify

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// WorkflowsClient handles workflow operations.
type WorkflowsClient struct {
	client *Client
}

// Create creates a new notification workflow.
func (w *WorkflowsClient) Create(ctx context.Context, params CreateWorkflowParams) (*Workflow, error) {
	var result Workflow
	if err := w.client.do(ctx, "POST", "/workflows/", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Get retrieves a workflow by ID.
func (w *WorkflowsClient) Get(ctx context.Context, workflowID string) (*Workflow, error) {
	var result Workflow
	if err := w.client.do(ctx, "GET", "/workflows/"+workflowID, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Update modifies an existing workflow.
func (w *WorkflowsClient) Update(ctx context.Context, workflowID string, params UpdateWorkflowParams) (*Workflow, error) {
	var result Workflow
	if err := w.client.do(ctx, "PUT", "/workflows/"+workflowID, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Delete removes a workflow.
func (w *WorkflowsClient) Delete(ctx context.Context, workflowID string) error {
	return w.client.do(ctx, "DELETE", "/workflows/"+workflowID, nil, nil)
}

// List returns a paginated list of workflows.
func (w *WorkflowsClient) List(ctx context.Context, page, pageSize int) (*WorkflowListResponse, error) {
	q := url.Values{}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if pageSize > 0 {
		q.Set("page_size", strconv.Itoa(pageSize))
	}

	var result WorkflowListResponse
	if err := w.client.doWithQuery(ctx, "GET", "/workflows/", q, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Trigger starts a workflow execution for a specific user.
func (w *WorkflowsClient) Trigger(ctx context.Context, params TriggerWorkflowParams) (*WorkflowExecution, error) {
	var result WorkflowExecution
	if err := w.client.do(ctx, "POST", "/workflows/trigger", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetExecution retrieves a specific workflow execution by ID.
func (w *WorkflowsClient) GetExecution(ctx context.Context, executionID string) (*WorkflowExecution, error) {
	var result WorkflowExecution
	if err := w.client.do(ctx, "GET", "/workflows/executions/"+executionID, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListExecutions returns a paginated list of workflow executions.
func (w *WorkflowsClient) ListExecutions(ctx context.Context, page, pageSize int) (*ExecutionListResponse, error) {
	q := url.Values{}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if pageSize > 0 {
		q.Set("page_size", strconv.Itoa(pageSize))
	}

	var result ExecutionListResponse
	if err := w.client.doWithQuery(ctx, "GET", "/workflows/executions", q, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CancelExecution cancels a running workflow execution.
func (w *WorkflowsClient) CancelExecution(ctx context.Context, executionID string) error {
	return w.client.do(ctx, "POST", "/workflows/executions/"+executionID+"/cancel", nil, nil)
}

// ── Builder Integration ──

// CreateFromBuilder validates and creates a workflow from a WorkflowBuilder.
func (w *WorkflowsClient) CreateFromBuilder(ctx context.Context, wf *WorkflowBuilder) (*Workflow, error) {
	params, err := wf.Build()
	if err != nil {
		return nil, fmt.Errorf("workflow builder: %w", err)
	}
	return w.Create(ctx, *params)
}

// UpdateFromBuilder validates and updates a workflow from a WorkflowBuilder.
func (w *WorkflowsClient) UpdateFromBuilder(ctx context.Context, id string, wf *WorkflowBuilder) (*Workflow, error) {
	params, err := wf.Build()
	if err != nil {
		return nil, fmt.Errorf("workflow builder: %w", err)
	}
	return w.Update(ctx, id, UpdateWorkflowParams{
		Name:        params.Name,
		Description: params.Description,
		Steps:       params.Steps,
	})
}
