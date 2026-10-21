package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/sse"
	"go.uber.org/zap"
	"testing"
	"time"
)

type channelMessages struct {
	whatsapp.Repository
	whatsapp.EventRepository
	rows map[string]*whatsapp.InboundMessage
}
type channelStatusRepo struct {
	notification.Repository
	row *notification.Notification
}

func (r *channelStatusRepo) List(context.Context, *notification.NotificationFilter) ([]*notification.Notification, error) {
	return []*notification.Notification{r.row}, nil
}
func (r *channelStatusRepo) GetByID(context.Context, string) (*notification.Notification, error) {
	copy := *r.row
	return &copy, nil
}
func (r *channelStatusRepo) ApplyProviderDeliveryStatus(_ context.Context, id, appID, providerID string, state notification.Status, at time.Time) (bool, error) {
	changed := r.row.Status != state
	r.row.Status = state
	return changed, nil
}

type channelSignalPublisher struct {
	fail  bool
	calls int
}

func (p *channelSignalPublisher) PublishMessage(*sse.SSEMessage) error {
	p.calls++
	if p.fail {
		return fmt.Errorf("Redis unavailable")
	}
	return nil
}
func TestChannelEventRecoveryStatusRefresh(t *testing.T) {
	for _, crashAfterTransition := range []bool{false, true} {
		t.Run(fmt.Sprint(crashAfterTransition), func(t *testing.T) {
			events := &channelMemoryEvents{rows: map[string]*whatsapp.EventReceipt{}}
			state := notification.StatusSent
			if crashAfterTransition {
				state = notification.StatusRead
			}
			notifs := &channelStatusRepo{row: &notification.Notification{NotificationID: "n", AppID: "a", Status: state, Metadata: map[string]interface{}{"provider_message_id": "upstream"}}}
			publisher := &channelSignalPublisher{fail: !crashAfterTransition}
			svc := &whatsappService{events: events, notifRepo: notifs, sseBroadc: publisher, logger: zap.NewNop()}
			receipt, err := svc.AcceptVerifiedEvent(context.Background(), &InboundSenderBinding{ID: "binding", AppID: "a", Provider: "meta"}, &whatsapp.VerifiedEvent{Kind: "status", Status: &whatsapp.DeliveryStatus{AppID: "a", MetaMessageID: "upstream", Status: "read", Timestamp: time.Now()}})
			if err != nil {
				t.Fatal(err)
			}
			err = svc.ProcessEventReceipt(context.Background(), receipt.ID)
			if crashAfterTransition {
				if err != nil || publisher.calls != 1 || events.rows[receipt.ID].State != "done" {
					t.Fatal("already-applied transition lost refresh after crash")
				}
			} else {
				if err == nil || events.rows[receipt.ID].State != "pending" {
					t.Fatal("failed refresh was acknowledged as done")
				}
				publisher.fail = false
				events.rows[receipt.ID].NextAttempt = time.Time{}
				if err = svc.ProcessEventReceipt(context.Background(), receipt.ID); err != nil || publisher.calls != 2 || events.rows[receipt.ID].State != "done" {
					t.Fatal("refresh did not recover")
				}
			}
		})
	}
}

type channelWorkflowRepo struct {
	workflow.Repository
	creates int
}

func (*channelWorkflowRepo) GetWorkflowByTrigger(context.Context, string, string) (*workflow.Workflow, error) {
	return nil, fmt.Errorf("temporary lookup failure")
}
func (r *channelWorkflowRepo) CreateExecution(context.Context, *workflow.WorkflowExecution) error {
	r.creates++
	return nil
}
func TestChannelEventRecoveryWorkflowBeforeDispatchRetries(t *testing.T) {
	events := &channelMemoryEvents{rows: map[string]*whatsapp.EventReceipt{}}
	repo := &channelMessages{EventRepository: events, rows: map[string]*whatsapp.InboundMessage{}}
	apps := &channelEventApps{app: &application.Application{AppID: "a", Settings: application.Settings{WhatsAppInbound: &whatsapp.InboundConfig{Enabled: true, RouteAction: whatsapp.RouteWorkflow, WorkflowTriggerID: "workflow"}}}}
	wfRepo := &channelWorkflowRepo{}
	wfSvc := NewWorkflowService(wfRepo, nil, zap.NewNop())
	svc := NewWhatsAppService(repo, apps, nil, nil, wfSvc, nil, zap.NewNop()).(*whatsappService)
	receipt, err := svc.AcceptVerifiedEvent(context.Background(), &InboundSenderBinding{ID: "binding", AppID: "a", Provider: "meta"}, &whatsapp.VerifiedEvent{Kind: "inbound", Message: &whatsapp.InboundMessage{MetaMessageID: "upstream", ContactWAID: "123", Timestamp: time.Now()}})
	if err != nil {
		t.Fatal(err)
	}
	if svc.ProcessEventReceipt(context.Background(), receipt.ID) == nil {
		t.Fatal("lookup failure hidden")
	}
	if events.rows[receipt.ID].State != "pending" || wfRepo.creates != 0 {
		t.Fatal("definite pre-dispatch failure stranded as unknown")
	}
}

func (r *channelMessages) StoreMessage(_ context.Context, m *whatsapp.InboundMessage) error {
	copy := *m
	r.rows[m.ID] = &copy
	return nil
}

type channelEventApps struct {
	application.Repository
	app *application.Application
}

func (r *channelEventApps) GetByID(context.Context, string) (*application.Application, error) {
	if r.app != nil {
		return r.app, nil
	}
	return &application.Application{AppID: "a"}, nil
}
func TestChannelEventRecoveryRestartAndUncertainAction(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Idempotency-Key") == "" {
			t.Error("missing side effect identity")
		}
		calls.Add(1)
		w.WriteHeader(200)
	}))
	defer server.Close()
	apps := &channelEventApps{app: &application.Application{AppID: "a", Settings: application.Settings{WhatsAppInbound: &whatsapp.InboundConfig{Enabled: true, RouteAction: whatsapp.RouteWebhookForward, WebhookForwardURL: server.URL}}}}
	for _, ambiguous := range []bool{false, true} {
		events := &channelMemoryEvents{rows: map[string]*whatsapp.EventReceipt{}}
		repo := &channelMessages{EventRepository: events, rows: map[string]*whatsapp.InboundMessage{}}
		svc := NewWhatsAppService(repo, apps, nil, nil, nil, nil, zap.NewNop()).(*whatsappService)
		msg := &whatsapp.InboundMessage{MetaMessageID: "upstream", Timestamp: time.Now()}
		receipt, err := svc.AcceptVerifiedEvent(context.Background(), &InboundSenderBinding{ID: "binding", AppID: "a", Provider: "meta"}, &whatsapp.VerifiedEvent{Kind: "inbound", Message: msg})
		if err != nil {
			t.Fatal(err)
		}
		// Simulate an exited processor after storage, or in an uncertain external action.
		repo.rows[receipt.ID] = msg
		events.rows[receipt.ID].Steps["stored"] = "done"
		before := calls.Load()
		if ambiguous {
			events.rows[receipt.ID].Steps["action"] = "dispatching"
		}
		restarted := NewWhatsAppService(repo, apps, nil, nil, nil, nil, zap.NewNop()).(whatsapp.EventProcessor)
		if err := restarted.ProcessEventReceipt(context.Background(), receipt.ID); err != nil {
			t.Fatal(err)
		}
		if err := restarted.ProcessEventReceipt(context.Background(), receipt.ID); err != nil {
			t.Fatal(err)
		}
		if ambiguous {
			if calls.Load() != before || events.rows[receipt.ID].State != "unknown" {
				t.Fatal("uncertain action replayed")
			}
		} else if calls.Load() != before+1 || events.rows[receipt.ID].State != "done" {
			t.Fatal("restart failed to complete exactly one action")
		}
	}
}
func TestChannelEventRecoveryBoundedRetry(t *testing.T) {
	events := &channelMemoryEvents{rows: map[string]*whatsapp.EventReceipt{}}
	svc := &whatsappService{events: events, logger: zap.NewNop(), applyTemplate: func(context.Context, *InboundSenderBinding, *whatsapp.TemplateStatusEvent) error {
		return fmt.Errorf("unavailable")
	}}
	receipt, err := svc.AcceptVerifiedEvent(context.Background(), &InboundSenderBinding{ID: "binding", AppID: "a", Provider: "twilio"}, &whatsapp.VerifiedEvent{Kind: "template", Template: &whatsapp.TemplateStatusEvent{TemplateID: "HX", Status: "approved"}})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		events.rows[receipt.ID].NextAttempt = time.Time{}
		if svc.ProcessEventReceipt(context.Background(), receipt.ID) == nil {
			t.Fatal("failure hidden")
		}
	}
	if events.rows[receipt.ID].State != "dead_letter" || events.rows[receipt.ID].Attempts != 5 {
		t.Fatal("unbounded callback retry")
	}
}
func TestChannelEventRecoveryDuplicateInbound(t *testing.T) {
	events := &channelMemoryEvents{rows: map[string]*whatsapp.EventReceipt{}}
	repo := &channelMessages{EventRepository: events, rows: map[string]*whatsapp.InboundMessage{}}
	svc := NewWhatsAppService(repo, &channelEventApps{}, nil, nil, nil, nil, zap.NewNop()).(*whatsappService)
	for i := 0; i < 2; i++ {
		err := svc.HandleInbound(context.Background(), "a", &whatsapp.InboundMessage{MetaMessageID: "upstream-one", ContactWAID: "123", TextBody: "private", Timestamp: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)})
		if err != nil {
			t.Fatal(err)
		}
		for id := range events.rows {
			if err = svc.ProcessEventReceipt(context.Background(), id); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(repo.rows) != 1 {
		t.Fatalf("callback replay stored %d rows, want 1", len(repo.rows))
	}
}

func TestChannelEventRecoveryMissingRedisFailsClosed(t *testing.T) {
	svc := &whatsappService{}
	if svc.IsCSWOpen(context.Background(), "a", "contact") {
		t.Fatal("missing window store granted a service window")
	}
}

type channelMemoryEvents struct {
	mu       sync.Mutex
	rows     map[string]*whatsapp.EventReceipt
	failSave bool
}

func copyChannelReceipt(in *whatsapp.EventReceipt) *whatsapp.EventReceipt {
	raw, _ := json.Marshal(in)
	out := &whatsapp.EventReceipt{}
	json.Unmarshal(raw, out)
	out.Sequence = in.Sequence
	return out
}
func (r *channelMemoryEvents) AcceptEvent(_ context.Context, e *whatsapp.EventReceipt) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rows[e.ID] == nil {
		r.rows[e.ID] = copyChannelReceipt(e)
	}
	return nil
}
func (r *channelMemoryEvents) SaveEvent(_ context.Context, e *whatsapp.EventReceipt) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failSave {
		return fmt.Errorf("storage failed")
	}
	if r.rows[e.ID].Sequence != e.Sequence {
		return fmt.Errorf("conflict")
	}
	e.Sequence++
	r.rows[e.ID] = copyChannelReceipt(e)
	return nil
}
func (r *channelMemoryEvents) ClaimEvent(_ context.Context, id string, now time.Time) (*whatsapp.EventReceipt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := copyChannelReceipt(r.rows[id])
	if e.State == "done" || e.State == "unknown" || e.State == "dead_letter" || e.LeaseUntil.After(now) || e.NextAttempt.After(now) {
		return nil, nil
	}
	for _, state := range e.Steps {
		if state == "dispatching" {
			e.State = "unknown"
			r.rows[id] = e
			return nil, nil
		}
	}
	e.State = "processing"
	e.LeaseUntil = now.Add(time.Minute)
	e.Attempts++
	e.Sequence++
	r.rows[id] = copyChannelReceipt(e)
	return e, nil
}
func (r *channelMemoryEvents) PendingEvents(context.Context, time.Time, int) ([]string, error) {
	ids := []string{}
	for id := range r.rows {
		ids = append(ids, id)
	}
	return ids, nil
}
