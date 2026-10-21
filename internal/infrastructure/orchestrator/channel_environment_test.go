package orchestrator

import (
	"context"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"go.uber.org/zap"
	"testing"
)

type phase5NotificationCapture struct {
	notification.Service
	request notification.SendRequest
	env     string
}

type phase5ExecutionRepo struct {
	workflow.Repository
	execution  *workflow.WorkflowExecution
	definition *workflow.Workflow
}

func (r *phase5ExecutionRepo) GetExecution(context.Context, string) (*workflow.WorkflowExecution, error) {
	return r.execution, nil
}
func (r *phase5ExecutionRepo) GetWorkflow(context.Context, string) (*workflow.Workflow, error) {
	return r.definition, nil
}
func (r *phase5ExecutionRepo) UpdateExecution(context.Context, *workflow.WorkflowExecution) error {
	return nil
}
func TestChannelWorkflowLegacyExecutionCannotChangeEnvironment(t *testing.T) {
	repo := &phase5ExecutionRepo{execution: &workflow.WorkflowExecution{ID: "e", AppID: "a", WorkflowID: "wf", Status: workflow.ExecStatusRunning, StepResults: map[string]workflow.StepResult{}}, definition: &workflow.Workflow{ID: "wf", AppID: "a", EnvironmentID: "stage", Steps: []workflow.Step{{ID: "s", Type: workflow.StepTypeChannel, Config: workflow.StepConfig{Channel: "sms"}}}}}
	sends := &phase5NotificationCapture{}
	e := &Engine{workflowRepo: repo, notifService: sends, logger: zap.NewNop()}
	e.executeStep(context.Background(), &queue.WorkflowQueueItem{ExecutionID: "e", StepID: "s"}, zap.NewNop())
	if repo.execution.Status != workflow.ExecStatusFailed || sends.request.AppID != "" {
		t.Fatal("legacy execution without admitted environment sent through default")
	}
}

func (s *phase5NotificationCapture) Send(ctx context.Context, r notification.SendRequest) (*notification.Notification, error) {
	s.request = r
	s.env = channelconnection.EnvironmentID(ctx)
	return &notification.Notification{NotificationID: "n"}, nil
}
func TestChannelWorkflowStepEnvironment(t *testing.T) {
	s := &phase5NotificationCapture{}
	e := &Engine{notifService: s}
	_, err := e.executeChannelStep(context.Background(), &workflow.WorkflowExecution{AppID: "a", EnvironmentID: "stage", UserID: "u"}, &workflow.Step{Config: workflow.StepConfig{Channel: "sms", TemplateID: "tpl"}})
	if err != nil {
		t.Fatal(err)
	}
	if s.request.EnvironmentID != "stage" || s.env != "stage" {
		t.Fatal("workflow sent through another environment")
	}
}
