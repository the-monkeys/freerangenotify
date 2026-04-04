package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/topic"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

type workflowService struct {
	repo     workflow.Repository
	queue    queue.WorkflowQueue
	topicSvc topic.Service // optional, for TriggerByTopic
	logger   *zap.Logger
}

// NewWorkflowService creates a new workflow service.
func NewWorkflowService(
	repo workflow.Repository,
	wq queue.WorkflowQueue,
	logger *zap.Logger,
) workflow.Service {
	return &workflowService{
		repo:   repo,
		queue:  wq,
		logger: logger,
	}
}

// SetTopicService injects the topic service for TriggerByTopic (called after container init).
func (s *workflowService) SetTopicService(ts topic.Service) { s.topicSvc = ts }

func (s *workflowService) Create(ctx context.Context, appID string, req *workflow.CreateRequest) (*workflow.Workflow, error) {
	s.logger.Info("Creating workflow",
		zap.String("app_id", appID),
		zap.String("name", req.Name),
		zap.String("trigger_id", req.TriggerID))

	// Check for duplicate trigger_id within the same app
	existing, _ := s.repo.GetWorkflowByTrigger(ctx, appID, req.TriggerID)
	if existing != nil {
		return nil, errors.BadRequest(fmt.Sprintf("trigger_id '%s' already exists in this app", req.TriggerID))
	}

	// Assign IDs to steps if not provided
	for i := range req.Steps {
		if req.Steps[i].ID == "" {
			req.Steps[i].ID = uuid.New().String()
		}
		if req.Steps[i].Order == 0 {
			req.Steps[i].Order = i + 1
		}
	}

	wf := &workflow.Workflow{
		ID:            uuid.New().String(),
		AppID:         appID,
		EnvironmentID: req.EnvironmentID,
		Name:          req.Name,
		Description:   req.Description,
		TriggerID:     req.TriggerID,
		Steps:         req.Steps,
		Status: func() workflow.WorkflowStatus {
			if req.Status != "" {
				return req.Status
			}
			return workflow.WorkflowStatusDraft
		}(),
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.CreateWorkflow(ctx, wf); err != nil {
		return nil, fmt.Errorf("failed to create workflow: %w", err)
	}

	return wf, nil
}

func (s *workflowService) Get(ctx context.Context, id, appID string) (*workflow.Workflow, error) {
	wf, err := s.repo.GetWorkflow(ctx, id)
	if err != nil {
		return nil, errors.NotFound("workflow", id)
	}
	if wf.AppID != appID {
		return nil, errors.NotFound("workflow", id)
	}
	return wf, nil
}

func (s *workflowService) Update(ctx context.Context, id, appID string, req *workflow.UpdateRequest) (*workflow.Workflow, error) {
	wf, err := s.repo.GetWorkflow(ctx, id)
	if err != nil {
		return nil, errors.NotFound("workflow", id)
	}
	if wf.AppID != appID {
		return nil, errors.NotFound("workflow", id)
	}

	if req.Name != nil {
		wf.Name = *req.Name
	}
	if req.Description != nil {
		wf.Description = *req.Description
	}
	if req.Steps != nil {
		// Assign IDs to new steps if not provided
		for i := range req.Steps {
			if req.Steps[i].ID == "" {
				req.Steps[i].ID = uuid.New().String()
			}
			if req.Steps[i].Order == 0 {
				req.Steps[i].Order = i + 1
			}
		}
		wf.Steps = req.Steps
		wf.Version++
	}
	if req.Status != nil {
		wf.Status = *req.Status
	}

	wf.UpdatedAt = time.Now()

	if err := s.repo.UpdateWorkflow(ctx, wf); err != nil {
		return nil, fmt.Errorf("failed to update workflow: %w", err)
	}

	return wf, nil
}

func (s *workflowService) Delete(ctx context.Context, id, appID string) error {
	wf, err := s.repo.GetWorkflow(ctx, id)
	if err != nil {
		return errors.NotFound("workflow", id)
	}
	if wf.AppID != appID {
		return errors.NotFound("workflow", id)
	}

	return s.repo.DeleteWorkflow(ctx, id)
}

func (s *workflowService) List(ctx context.Context, appID, environmentID string, linkedIDs []string, limit, offset int) ([]*workflow.Workflow, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListWorkflows(ctx, appID, environmentID, linkedIDs, limit, offset)
}

func (s *workflowService) Trigger(ctx context.Context, appID string, req *workflow.TriggerRequest) (*workflow.WorkflowExecution, error) {
	// 1. Resolve workflow by trigger_id
	wf, err := s.repo.GetWorkflowByTrigger(ctx, appID, req.TriggerID)
	if err != nil {
		return nil, errors.NotFound("workflow", req.TriggerID)
	}

	if wf.Status != workflow.WorkflowStatusActive {
		return nil, errors.BadRequest(fmt.Sprintf("workflow '%s' is not active (status: %s)", wf.Name, wf.Status))
	}

	// 2. Idempotency check
	if req.TransactionID != "" {
		existing, _ := s.repo.GetActiveExecutions(ctx, req.UserID, wf.ID)
		for _, exec := range existing {
			if exec.TransactionID == req.TransactionID {
				return exec, nil // Already running, return existing
			}
		}
	}

	// 3. Create execution (first step ID from the sorted steps)
	firstStepID := ""
	if len(wf.Steps) > 0 {
		firstStepID = wf.Steps[0].ID
	}

	exec := &workflow.WorkflowExecution{
		ID:            uuid.New().String(),
		WorkflowID:    wf.ID,
		AppID:         appID,
		UserID:        req.UserID,
		TransactionID: req.TransactionID,
		CurrentStepID: firstStepID,
		Status:        workflow.ExecStatusRunning,
		Payload:       req.Payload,
		StepResults:   make(map[string]workflow.StepResult),
		StartedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.repo.CreateExecution(ctx, exec); err != nil {
		return nil, fmt.Errorf("failed to create execution: %w", err)
	}

	// 4. Enqueue first step
	if err := s.queue.EnqueueWorkflow(ctx, queue.WorkflowQueueItem{
		ExecutionID: exec.ID,
		StepID:      firstStepID,
		EnqueuedAt:  time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("failed to enqueue workflow: %w", err)
	}

	s.logger.Info("Workflow triggered",
		zap.String("workflow_id", wf.ID),
		zap.String("execution_id", exec.ID),
		zap.String("trigger_id", req.TriggerID))

	return exec, nil
}

// triggerForUsers triggers a workflow for each user. Returns execution IDs and continues on per-user errors.
func (s *workflowService) triggerForUsers(ctx context.Context, appID, triggerID string, userIDs []string, payload map[string]any) ([]string, error) {
	if len(userIDs) == 0 {
		return []string{}, nil
	}
	var execIDs []string
	for _, userID := range userIDs {
		req := &workflow.TriggerRequest{
			TriggerID: triggerID,
			UserID:    userID,
			Payload:   payload,
		}
		exec, err := s.Trigger(ctx, appID, req)
		if err != nil {
			s.logger.Warn("Failed to trigger workflow for user, skipping",
				zap.String("user_id", userID),
				zap.String("trigger_id", triggerID),
				zap.Error(err))
			continue
		}
		execIDs = append(execIDs, exec.ID)
	}
	return execIDs, nil
}

func (s *workflowService) TriggerByTopic(ctx context.Context, appID string, req *workflow.TriggerByTopicRequest) (*workflow.TriggerByTopicResult, error) {
	if s.topicSvc == nil {
		return nil, errors.BadRequest("topics feature is not enabled")
	}
	userIDs, err := s.topicSvc.GetSubscriberUserIDs(ctx, req.TopicID, appID)
	if err != nil {
		return nil, err
	}
	execIDs, _ := s.triggerForUsers(ctx, appID, req.TriggerID, userIDs, req.Payload)
	return &workflow.TriggerByTopicResult{
		Triggered:    len(execIDs),
		ExecutionIDs: execIDs,
	}, nil
}

func (s *workflowService) TriggerForUserIDs(ctx context.Context, appID, triggerID string, userIDs []string, payload map[string]any) (*workflow.TriggerForUserIDsResult, error) {
	if len(userIDs) == 0 {
		return &workflow.TriggerForUserIDsResult{Triggered: 0, ExecutionIDs: nil}, nil
	}
	execIDs, _ := s.triggerForUsers(ctx, appID, triggerID, userIDs, payload)
	return &workflow.TriggerForUserIDsResult{
		Triggered:    len(execIDs),
		ExecutionIDs: execIDs,
	}, nil
}

func (s *workflowService) CancelExecution(ctx context.Context, executionID, appID string) error {
	exec, err := s.repo.GetExecution(ctx, executionID)
	if err != nil {
		return errors.NotFound("execution", executionID)
	}
	if exec.AppID != appID {
		return errors.NotFound("execution", executionID)
	}
	if exec.Status != workflow.ExecStatusRunning && exec.Status != workflow.ExecStatusPaused {
		return errors.BadRequest("execution is not in a cancellable state")
	}

	now := time.Now()
	exec.Status = workflow.ExecStatusCancelled
	exec.CompletedAt = &now
	exec.UpdatedAt = now

	return s.repo.UpdateExecution(ctx, exec)
}

func (s *workflowService) GetExecution(ctx context.Context, executionID, appID string) (*workflow.WorkflowExecution, error) {
	exec, err := s.repo.GetExecution(ctx, executionID)
	if err != nil {
		return nil, errors.NotFound("execution", executionID)
	}
	if exec.AppID != appID {
		return nil, errors.NotFound("execution", executionID)
	}
	return exec, nil
}

func (s *workflowService) ListExecutions(ctx context.Context, workflowID, appID string, limit, offset int) ([]*workflow.WorkflowExecution, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Verify workflow belongs to app
	if workflowID != "" {
		wf, err := s.repo.GetWorkflow(ctx, workflowID)
		if err != nil {
			return nil, 0, errors.NotFound("workflow", workflowID)
		}
		if wf.AppID != appID {
			return nil, 0, errors.NotFound("workflow", workflowID)
		}
	}

	return s.repo.ListExecutions(ctx, workflowID, limit, offset)
}
