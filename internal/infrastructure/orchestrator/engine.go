package orchestrator

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/metrics"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"go.uber.org/zap"
)

// Engine orchestrates workflow step execution.
type Engine struct {
	workflowRepo workflow.Repository
	notifService notification.Service
	wfQueue      queue.WorkflowQueue
	redisClient  *redis.Client
	logger       *zap.Logger
	metrics      *metrics.NotificationMetrics

	workerCount int
	wg          sync.WaitGroup
	stopChan    chan struct{}
}

// NewEngine creates a new workflow orchestrator engine.
func NewEngine(
	workflowRepo workflow.Repository,
	notifService notification.Service,
	wfQueue queue.WorkflowQueue,
	redisClient *redis.Client,
	logger *zap.Logger,
	metrics *metrics.NotificationMetrics,
	workerCount int,
) *Engine {
	return &Engine{
		workflowRepo: workflowRepo,
		notifService: notifService,
		wfQueue:      wfQueue,
		redisClient:  redisClient,
		logger:       logger,
		metrics:      metrics,
		workerCount:  workerCount,
		stopChan:     make(chan struct{}),
	}
}

// Start launches workflow workers and the delayed step poller.
func (e *Engine) Start(ctx context.Context) {
	for i := 0; i < e.workerCount; i++ {
		e.wg.Add(1)
		go e.worker(ctx, i)
	}

	// Delayed step poller — moves delayed items to the main workflow queue
	e.wg.Add(1)
	go e.delayedPoller(ctx)

	// Recovery — re-enqueues stale executions
	e.wg.Add(1)
	go e.recovery(ctx)

	e.logger.Info("Workflow engine started",
		zap.Int("worker_count", e.workerCount))
}

// Shutdown gracefully stops the engine.
func (e *Engine) Shutdown(ctx context.Context) error {
	close(e.stopChan)
	done := make(chan struct{})
	go func() { e.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *Engine) worker(ctx context.Context, id int) {
	defer e.wg.Done()
	logger := e.logger.With(zap.Int("wf_worker_id", id))

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopChan:
			return
		default:
			item, err := e.wfQueue.DequeueWorkflow(ctx)
			if err != nil || item == nil {
				time.Sleep(1 * time.Second)
				continue
			}
			e.executeStep(ctx, item, logger)
		}
	}
}

func (e *Engine) executeStep(ctx context.Context, item *queue.WorkflowQueueItem, logger *zap.Logger) {
	exec, err := e.workflowRepo.GetExecution(ctx, item.ExecutionID)
	if err != nil {
		logger.Error("Failed to get workflow execution", zap.Error(err))
		return
	}

	if exec.Status != workflow.ExecStatusRunning {
		logger.Info("Execution is not running, skipping",
			zap.String("execution_id", exec.ID),
			zap.String("status", string(exec.Status)))
		return
	}

	wf, err := e.workflowRepo.GetWorkflow(ctx, exec.WorkflowID)
	if err != nil {
		logger.Error("Failed to get workflow", zap.Error(err))
		return
	}

	// Find the current step
	var step *workflow.Step
	for i := range wf.Steps {
		if wf.Steps[i].ID == item.StepID {
			step = &wf.Steps[i]
			break
		}
	}

	if step == nil {
		logger.Error("Step not found in workflow",
			zap.String("step_id", item.StepID))
		e.failExecution(ctx, exec, "step not found: "+item.StepID)
		return
	}

	// Check skip_if condition
	if step.SkipIf != nil && e.evaluateCondition(step.SkipIf, exec) {
		logger.Info("Step skipped by condition",
			zap.String("step_id", step.ID))
		e.advanceToNext(ctx, exec, wf, step, workflow.StepStatusSkipped, logger)
		return
	}

	// Execute based on step type
	now := time.Now()
	result := workflow.StepResult{
		StepID:    step.ID,
		Status:    workflow.StepStatusRunning,
		StartedAt: &now,
	}

	switch step.Type {
	case workflow.StepTypeChannel:
		notifID, err := e.executeChannelStep(ctx, exec, step)
		if err != nil {
			result.Status = workflow.StepStatusFailed
			result.Error = err.Error()
		} else {
			result.Status = workflow.StepStatusCompleted
			result.NotificationID = notifID
		}

	case workflow.StepTypeDelay:
		e.executeDelayStep(ctx, exec, step, item, logger)
		return // Don't advance — the delayed item will re-enter the queue

	case workflow.StepTypeDigest:
		count, err := e.executeDigestStep(ctx, exec, step)
		if err != nil {
			result.Status = workflow.StepStatusFailed
			result.Error = err.Error()
		} else {
			result.Status = workflow.StepStatusCompleted
			result.DigestCount = count
		}

	case workflow.StepTypeCondition:
		// Evaluate condition and advance to either on_success or on_failure
		matched := e.evaluateCondition(step.Config.Condition, exec)
		result.Status = workflow.StepStatusCompleted
		completedAt := time.Now()
		result.CompletedAt = &completedAt
		exec.StepResults[step.ID] = result

		nextStepID := step.OnFailure
		if matched {
			nextStepID = step.OnSuccess
		}
		if nextStepID == "" {
			e.completeExecution(ctx, exec)
			return
		}
		exec.CurrentStepID = nextStepID
		exec.UpdatedAt = time.Now()
		e.workflowRepo.UpdateExecution(ctx, exec)
		e.wfQueue.EnqueueWorkflow(ctx, queue.WorkflowQueueItem{
			ExecutionID: exec.ID,
			StepID:      nextStepID,
			EnqueuedAt:  time.Now(),
		})
		return

	default:
		result.Status = workflow.StepStatusFailed
		result.Error = fmt.Sprintf("unknown step type: %s", step.Type)
	}

	completedAt := time.Now()
	result.CompletedAt = &completedAt
	exec.StepResults[step.ID] = result

	e.advanceToNext(ctx, exec, wf, step, result.Status, logger)
}

// executeChannelStep sends a notification using the existing notification.Service.
// This is the critical backward compatibility point — we reuse Send() which
// goes through the exact same validation, queue, and provider pipeline.
func (e *Engine) executeChannelStep(ctx context.Context, exec *workflow.WorkflowExecution, step *workflow.Step) (string, error) {
	req := notification.SendRequest{
		AppID:      exec.AppID,
		UserID:     exec.UserID,
		Channel:    notification.Channel(step.Config.Channel),
		Priority:   notification.PriorityNormal,
		TemplateID: step.Config.TemplateID,
		Data:       exec.Payload,
	}

	notif, err := e.notifService.Send(ctx, req)
	if err != nil {
		return "", err
	}
	return notif.NotificationID, nil
}

// executeDelayStep schedules re-execution after the configured duration.
func (e *Engine) executeDelayStep(ctx context.Context, exec *workflow.WorkflowExecution, step *workflow.Step, item *queue.WorkflowQueueItem, logger *zap.Logger) {
	d, err := time.ParseDuration(step.Config.Duration)
	if err != nil {
		logger.Error("Invalid delay duration", zap.String("duration", step.Config.Duration))
		e.failExecution(ctx, exec, "invalid delay duration: "+step.Config.Duration)
		return
	}

	// Record the step as running (it's "waiting")
	now := time.Now()
	exec.StepResults[step.ID] = workflow.StepResult{
		StepID:    step.ID,
		Status:    workflow.StepStatusRunning,
		StartedAt: &now,
	}
	exec.UpdatedAt = time.Now()
	e.workflowRepo.UpdateExecution(ctx, exec)

	// Find next step to enqueue after delay (on_success, or next step by order if unset)
	nextStepID := step.OnSuccess
	if nextStepID == "" {
		// BUG FIX: Previously used step.ID which re-enqueued the delay step itself, causing
		// an infinite loop. Now find the next step by order in the workflow.
		wf, wfErr := e.workflowRepo.GetWorkflow(ctx, exec.WorkflowID)
		if wfErr != nil {
			logger.Error("Failed to get workflow for delay step fallback", zap.Error(wfErr))
			e.failExecution(ctx, exec, "cannot resolve next step after delay")
			return
		}
		for i := range wf.Steps {
			if wf.Steps[i].ID == step.ID && i+1 < len(wf.Steps) {
				nextStepID = wf.Steps[i+1].ID
				logger.Info("Delay step on_success empty, using next step by order",
					zap.String("next_step_id", nextStepID))
				break
			}
		}
		if nextStepID == "" {
			logger.Info("Delay step is last step, execution will complete after delay")
			// No next step — after delay we'll complete. Enqueue self to trigger completion path.
			nextStepID = step.ID
		}
	}

	executeAt := time.Now().Add(d)
	e.wfQueue.EnqueueWorkflowDelayed(ctx, queue.WorkflowQueueItem{
		ExecutionID: exec.ID,
		StepID:      nextStepID,
		EnqueuedAt:  time.Now(),
	}, executeAt)

	logger.Info("Delay step scheduled",
		zap.String("execution_id", exec.ID),
		zap.String("step_id", step.ID),
		zap.Duration("delay", d),
		zap.Time("execute_at", executeAt))
}

// executeDigestStep is a placeholder for digest integration within workflows.
func (e *Engine) executeDigestStep(ctx context.Context, exec *workflow.WorkflowExecution, step *workflow.Step) (int, error) {
	// In workflow context, digest steps accumulate events.
	// The actual digest logic is handled by DigestManager in the notification pipeline.
	// Here, we log and advance — the workflow step represents the point
	// where accumulation was configured.
	e.logger.Info("Digest step executed in workflow",
		zap.String("execution_id", exec.ID),
		zap.String("step_id", step.ID),
		zap.String("digest_key", step.Config.DigestKey),
		zap.String("window", step.Config.Window))

	return 0, nil
}

func (e *Engine) advanceToNext(ctx context.Context, exec *workflow.WorkflowExecution, wf *workflow.Workflow, currentStep *workflow.Step, status workflow.StepStatus, logger *zap.Logger) {
	nextStepID := ""
	if status == workflow.StepStatusCompleted || status == workflow.StepStatusSkipped {
		nextStepID = currentStep.OnSuccess
	} else if status == workflow.StepStatusFailed {
		nextStepID = currentStep.OnFailure
	}

	// Fallback: if on_success/on_failure not set, use next step by order (UI often omits these)
	if nextStepID == "" && (status == workflow.StepStatusCompleted || status == workflow.StepStatusSkipped) {
		for i := range wf.Steps {
			if wf.Steps[i].ID == currentStep.ID && i+1 < len(wf.Steps) {
				nextStepID = wf.Steps[i+1].ID
				logger.Info("Step on_success empty, using next step by order",
					zap.String("current_step", currentStep.ID),
					zap.String("next_step_id", nextStepID))
				break
			}
		}
	}

	if nextStepID == "" {
		// No next step — workflow is done
		if status == workflow.StepStatusFailed {
			e.failExecution(ctx, exec, "step failed: "+currentStep.ID)
		} else {
			e.completeExecution(ctx, exec)
		}
		return
	}

	exec.CurrentStepID = nextStepID
	exec.UpdatedAt = time.Now()
	e.workflowRepo.UpdateExecution(ctx, exec)

	e.wfQueue.EnqueueWorkflow(ctx, queue.WorkflowQueueItem{
		ExecutionID: exec.ID,
		StepID:      nextStepID,
		EnqueuedAt:  time.Now(),
	})
}

func (e *Engine) completeExecution(ctx context.Context, exec *workflow.WorkflowExecution) {
	now := time.Now()
	exec.Status = workflow.ExecStatusCompleted
	exec.CompletedAt = &now
	exec.UpdatedAt = now
	e.workflowRepo.UpdateExecution(ctx, exec)
}

func (e *Engine) failExecution(ctx context.Context, exec *workflow.WorkflowExecution, reason string) {
	now := time.Now()
	exec.Status = workflow.ExecStatusFailed
	exec.CompletedAt = &now
	exec.UpdatedAt = now
	e.workflowRepo.UpdateExecution(ctx, exec)

	e.logger.Error("Workflow execution failed",
		zap.String("execution_id", exec.ID),
		zap.String("reason", reason))
}

func (e *Engine) evaluateCondition(cond *workflow.Condition, exec *workflow.WorkflowExecution) bool {
	if cond == nil {
		return false
	}

	// Resolve field value from payload or step results
	val := e.resolveField(cond.Field, exec)

	switch cond.Operator {
	case workflow.OpEquals:
		return fmt.Sprintf("%v", val) == fmt.Sprintf("%v", cond.Value)
	case workflow.OpNotEquals:
		return fmt.Sprintf("%v", val) != fmt.Sprintf("%v", cond.Value)
	case workflow.OpContains:
		return strings.Contains(fmt.Sprintf("%v", val), fmt.Sprintf("%v", cond.Value))
	case workflow.OpExists:
		return val != nil
	case workflow.OpGreaterThan:
		a, _ := strconv.ParseFloat(fmt.Sprintf("%v", val), 64)
		b, _ := strconv.ParseFloat(fmt.Sprintf("%v", cond.Value), 64)
		return a > b
	case workflow.OpLessThan:
		a, _ := strconv.ParseFloat(fmt.Sprintf("%v", val), 64)
		b, _ := strconv.ParseFloat(fmt.Sprintf("%v", cond.Value), 64)
		return a < b
	case workflow.OpNotRead:
		// Check if a previous step's notification was not read
		stepID := cond.Field
		if result, ok := exec.StepResults[stepID]; ok {
			return result.NotificationID != "" && result.Status == workflow.StepStatusCompleted
		}
		return true // Step not executed → treat as "not read"
	default:
		return false
	}
}

func (e *Engine) resolveField(field string, exec *workflow.WorkflowExecution) any {
	parts := strings.SplitN(field, ".", 2)
	if len(parts) < 2 {
		if v, ok := exec.Payload[field]; ok {
			return v
		}
		return nil
	}

	switch parts[0] {
	case "payload":
		return exec.Payload[parts[1]]
	case "steps":
		stepParts := strings.SplitN(parts[1], ".", 2)
		if result, ok := exec.StepResults[stepParts[0]]; ok {
			if len(stepParts) == 2 {
				switch stepParts[1] {
				case "status":
					return string(result.Status)
				case "notification_id":
					return result.NotificationID
				case "error":
					return result.Error
				}
			}
			return result
		}
	}
	return nil
}

// delayedPoller moves delayed workflow items to the main queue when their time arrives.
func (e *Engine) delayedPoller(ctx context.Context) {
	defer e.wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopChan:
			return
		case <-ticker.C:
			items, err := e.wfQueue.GetDelayedWorkflowItems(ctx, 100)
			if err != nil {
				e.logger.Error("Failed to get delayed workflow items", zap.Error(err))
				continue
			}
			for _, item := range items {
				if err := e.wfQueue.EnqueueWorkflow(ctx, item); err != nil {
					e.logger.Error("Failed to re-enqueue delayed workflow item", zap.Error(err))
				} else {
					e.logger.Info("Delayed workflow item moved to queue",
						zap.String("execution_id", item.ExecutionID),
						zap.String("step_id", item.StepID))
				}
			}
		}
	}
}

// recovery scans for stale executions and re-enqueues them.
func (e *Engine) recovery(ctx context.Context) {
	defer e.wg.Done()
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopChan:
			return
		case <-ticker.C:
			staleSince := time.Now().Add(-5 * time.Minute)
			stale, err := e.workflowRepo.GetStaleExecutions(ctx, staleSince, 50)
			if err != nil {
				e.logger.Error("Failed to get stale executions", zap.Error(err))
				continue
			}
			for _, exec := range stale {
				e.logger.Warn("Recovering stale workflow execution",
					zap.String("execution_id", exec.ID),
					zap.String("current_step", exec.CurrentStepID))
				e.wfQueue.EnqueueWorkflow(ctx, queue.WorkflowQueueItem{
					ExecutionID: exec.ID,
					StepID:      exec.CurrentStepID,
					EnqueuedAt:  time.Now(),
				})
			}
		}
	}
}
