package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"go.uber.org/zap"
)

// WorkflowRepository implements workflow.Repository using Elasticsearch.
type WorkflowRepository struct {
	workflows  *BaseRepository
	executions *BaseRepository
}

// NewWorkflowRepository creates a new workflow repository.
func NewWorkflowRepository(client *elasticsearch.Client, logger *zap.Logger) workflow.Repository {
	return &WorkflowRepository{
		workflows:  NewBaseRepository(client, "workflows", logger, RefreshWaitFor),
		executions: NewBaseRepository(client, "workflow_executions", logger),
	}
}

// --- Workflow CRUD ---

func (r *WorkflowRepository) CreateWorkflow(ctx context.Context, wf *workflow.Workflow) error {
	wf.CreatedAt = time.Now()
	wf.UpdatedAt = time.Now()
	return r.workflows.Create(ctx, wf.ID, wf)
}

func (r *WorkflowRepository) GetWorkflow(ctx context.Context, id string) (*workflow.Workflow, error) {
	doc, err := r.workflows.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var wf workflow.Workflow
	if err := mapToStruct(doc, &wf); err != nil {
		return nil, fmt.Errorf("failed to map document to workflow: %w", err)
	}
	return &wf, nil
}

func (r *WorkflowRepository) GetWorkflowByTrigger(ctx context.Context, appID, triggerID string) (*workflow.Workflow, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"app_id": appID}},
					{"term": map[string]interface{}{"trigger_id": triggerID}},
				},
			},
		},
		"size": 1,
	}

	result, err := r.workflows.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	if result.Total == 0 {
		return nil, fmt.Errorf("workflow not found for trigger %s in app %s", triggerID, appID)
	}

	var wf workflow.Workflow
	if err := mapToStruct(result.Hits[0], &wf); err != nil {
		return nil, fmt.Errorf("failed to map document to workflow: %w", err)
	}
	return &wf, nil
}

func (r *WorkflowRepository) UpdateWorkflow(ctx context.Context, wf *workflow.Workflow) error {
	wf.UpdatedAt = time.Now()
	return r.workflows.Update(ctx, wf.ID, wf)
}

func (r *WorkflowRepository) DeleteWorkflow(ctx context.Context, id string) error {
	return r.workflows.Delete(ctx, id)
}

func (r *WorkflowRepository) ListWorkflows(ctx context.Context, appID, environmentID string, limit, offset int) ([]*workflow.Workflow, int64, error) {
	filters := []map[string]interface{}{
		{"term": map[string]interface{}{"app_id": appID}},
	}
	if environmentID != "" && environmentID != "default" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{"environment_id": environmentID},
		})
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": filters,
			},
		},
		"sort": []map[string]interface{}{
			{"created_at": map[string]interface{}{"order": "desc"}},
		},
		"from": offset,
		"size": limit,
	}

	result, err := r.workflows.Search(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	var workflows []*workflow.Workflow
	for _, hit := range result.Hits {
		var wf workflow.Workflow
		if err := mapToStruct(hit, &wf); err != nil {
			r.workflows.logger.Error("Failed to map document to workflow", zap.Error(err))
			continue
		}
		workflows = append(workflows, &wf)
	}

	return workflows, result.Total, nil
}

// CountAll returns the total number of workflows across all apps.
func (r *WorkflowRepository) CountAll(ctx context.Context) (int64, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
	}
	return r.workflows.Count(ctx, query)
}

// CountByAppIDs returns the count of workflows belonging to the specified apps.
func (r *WorkflowRepository) CountByAppIDs(ctx context.Context, appIDs []string) (int64, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"terms": map[string]interface{}{
				"app_id": appIDs,
			},
		},
	}
	return r.workflows.Count(ctx, query)
}

// --- Execution CRUD ---

func (r *WorkflowRepository) CreateExecution(ctx context.Context, exec *workflow.WorkflowExecution) error {
	exec.StartedAt = time.Now()
	exec.UpdatedAt = time.Now()
	return r.executions.Create(ctx, exec.ID, exec)
}

func (r *WorkflowRepository) GetExecution(ctx context.Context, id string) (*workflow.WorkflowExecution, error) {
	doc, err := r.executions.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var exec workflow.WorkflowExecution
	if err := mapToStruct(doc, &exec); err != nil {
		return nil, fmt.Errorf("failed to map document to workflow execution: %w", err)
	}
	return &exec, nil
}

func (r *WorkflowRepository) UpdateExecution(ctx context.Context, exec *workflow.WorkflowExecution) error {
	exec.UpdatedAt = time.Now()
	return r.executions.Update(ctx, exec.ID, exec)
}

func (r *WorkflowRepository) ListExecutions(ctx context.Context, workflowID string, limit, offset int) ([]*workflow.WorkflowExecution, int64, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"workflow_id": workflowID,
			},
		},
		"sort": []map[string]interface{}{
			{"started_at": map[string]interface{}{"order": "desc"}},
		},
		"from": offset,
		"size": limit,
	}

	result, err := r.executions.Search(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	var executions []*workflow.WorkflowExecution
	for _, hit := range result.Hits {
		var exec workflow.WorkflowExecution
		if err := mapToStruct(hit, &exec); err != nil {
			r.executions.logger.Error("Failed to map document to workflow execution", zap.Error(err))
			continue
		}
		executions = append(executions, &exec)
	}

	return executions, result.Total, nil
}

func (r *WorkflowRepository) GetActiveExecutions(ctx context.Context, userID, workflowID string) ([]*workflow.WorkflowExecution, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"user_id": userID}},
					{"term": map[string]interface{}{"workflow_id": workflowID}},
					{"term": map[string]interface{}{"status": string(workflow.ExecStatusRunning)}},
				},
			},
		},
		"size": 100,
	}

	result, err := r.executions.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	var executions []*workflow.WorkflowExecution
	for _, hit := range result.Hits {
		var exec workflow.WorkflowExecution
		if err := mapToStruct(hit, &exec); err != nil {
			r.executions.logger.Error("Failed to map document to workflow execution", zap.Error(err))
			continue
		}
		executions = append(executions, &exec)
	}

	return executions, nil
}

func (r *WorkflowRepository) GetStaleExecutions(ctx context.Context, staleSince time.Time, limit int) ([]*workflow.WorkflowExecution, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"status": string(workflow.ExecStatusRunning)}},
					{"range": map[string]interface{}{
						"updated_at": map[string]interface{}{
							"lt": staleSince.Format(time.RFC3339),
						},
					}},
				},
			},
		},
		"size": limit,
	}

	result, err := r.executions.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	var executions []*workflow.WorkflowExecution
	for _, hit := range result.Hits {
		var exec workflow.WorkflowExecution
		if err := mapToStruct(hit, &exec); err != nil {
			r.executions.logger.Error("Failed to map document to workflow execution", zap.Error(err))
			continue
		}
		executions = append(executions, &exec)
	}

	return executions, nil
}
