package services

import (
	"context"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"go.uber.org/zap"
	"testing"
	"time"
)

func TestChannelPlatformMetaSnapshot(t *testing.T) {
	s := connectionTestService()
	s.System = map[notification.Channel]*channelconnection.Snapshot{"whatsapp": {Provider: "twilio", AccountID: "twilio", Credentials: map[string]string{"auth_token": "fixture-wrong"}}, "meta_whatsapp": {Provider: "meta", AccountID: "waba", SenderID: "phone", Credentials: map[string]string{"access_token": "fixture-meta", "app_secret": "fixture-signing"}}}
	c := &channelconnection.Connection{ID: "c", AppID: "a", Channel: "whatsapp", Provider: "meta", CredentialSource: "system", AccountID: "waba", SenderID: "phone", Revision: 1}
	snap, err := s.snapshot(c)
	if err != nil || snap.Credentials["access_token"] != "fixture-meta" {
		t.Fatalf("platform Meta selection: %v", err)
	}
	c.AccountID = "foreign"
	if _, err = s.snapshot(c); err == nil {
		t.Fatal("foreign system identity accepted")
	}
}

type phase5WorkflowRepo struct {
	workflow.Repository
	env       string
	execution *workflow.WorkflowExecution
}

func (r *phase5WorkflowRepo) GetWorkflowByTrigger(ctx context.Context, app, trigger string) (*workflow.Workflow, error) {
	r.env = channelconnection.EnvironmentID(ctx)
	return &workflow.Workflow{ID: "wf", AppID: app, EnvironmentID: r.env, Status: workflow.WorkflowStatusActive, Steps: []workflow.Step{{ID: "step"}}}, nil
}
func (r *phase5WorkflowRepo) CreateExecution(_ context.Context, e *workflow.WorkflowExecution) error {
	r.execution = e
	return nil
}
func (r *phase5WorkflowRepo) GetActiveExecutions(context.Context, string, string) ([]*workflow.WorkflowExecution, error) {
	return nil, nil
}

type phase5WorkflowQueue struct{ queue.WorkflowQueue }

func (*phase5WorkflowQueue) EnqueueWorkflow(context.Context, queue.WorkflowQueueItem) error {
	return nil
}
func TestChannelInboundWorkflowEnvironment(t *testing.T) {
	events := &channelMemoryEvents{rows: map[string]*whatsapp.EventReceipt{}}
	messages := &channelMessages{EventRepository: events, rows: map[string]*whatsapp.InboundMessage{}}
	apps := &channelEventApps{app: &application.Application{AppID: "a", Settings: application.Settings{WhatsAppInbound: &whatsapp.InboundConfig{Enabled: true, RouteAction: whatsapp.RouteWorkflow, WorkflowTriggerID: "hello"}}}}
	repo := &phase5WorkflowRepo{}
	svc := NewWhatsAppService(messages, apps, nil, nil, NewWorkflowService(repo, &phase5WorkflowQueue{}, zap.NewNop()), nil, zap.NewNop()).(*whatsappService)
	receipt, err := svc.AcceptVerifiedEvent(context.Background(), &InboundSenderBinding{ID: "binding", AppID: "a", EnvironmentID: "stage", Provider: "meta"}, &whatsapp.VerifiedEvent{Kind: "inbound", Message: &whatsapp.InboundMessage{MetaMessageID: "incoming", ContactWAID: "123", EnvironmentID: "untrusted", Timestamp: time.Now()}})
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.ProcessEventReceipt(context.Background(), receipt.ID); err != nil {
		t.Fatal(err)
	}
	if repo.env != "stage" || repo.execution == nil || repo.execution.EnvironmentID != "stage" || messages.rows[receipt.ID].EnvironmentID != "stage" {
		t.Fatal("verified environment lost before workflow admission")
	}
}
