package contracts

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/environment"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	templateDomain "github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/domain/topic"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/handlers"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/middleware"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

func (r *exampleUsers) List(_ context.Context, f user.UserFilter) ([]*user.User, error) {
	out := []*user.User{}
	for _, u := range r.users {
		if u.AppID == f.AppID {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	if f.Offset >= len(out) {
		return []*user.User{}, nil
	}
	end := len(out)
	if f.Limit > 0 && f.Offset+f.Limit < end {
		end = f.Offset + f.Limit
	}
	return out[f.Offset:end], nil
}
func (r *exampleNotifications) Update(ctx context.Context, n *notification.Notification) error {
	return r.Create(ctx, n)
}
func (r *exampleNotifications) UpdateStatus(_ context.Context, id string, s notification.Status) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notifications[id].Status = s
	return nil
}
func (r *exampleNotifications) BulkUpdateStatus(ctx context.Context, ids []string, s notification.Status) error {
	for _, id := range ids {
		if err := r.UpdateStatus(ctx, id, s); err != nil {
			return err
		}
	}
	return nil
}
func (r *exampleNotifications) BulkCreate(ctx context.Context, ns []*notification.Notification) error {
	for _, n := range ns {
		if err := r.Create(ctx, n); err != nil {
			return err
		}
	}
	return nil
}
func (r *exampleNotifications) IncrementRetryCount(_ context.Context, id, _ string) error {
	r.notifications[id].RetryCount++
	return nil
}
func (r *exampleNotifications) UpdateSnooze(ctx context.Context, id string, s notification.Status, until *time.Time) error {
	n := r.notifications[id]
	n.SnoozedUntil = until
	return r.UpdateStatus(ctx, id, s)
}
func (r *exampleNotifications) BulkArchive(ctx context.Context, ids []string, _ time.Time) error {
	return r.BulkUpdateStatus(ctx, ids, notification.StatusArchived)
}
func (*exampleQueue) EnqueueBatch(context.Context, []queue.NotificationQueueItem) error { return nil }
func (*exampleQueue) EnqueueScheduled(context.Context, queue.NotificationQueueItem, time.Time) error {
	return nil
}
func (*exampleQueue) RemoveScheduledByID(context.Context, []string) error { return nil }
func cloneTemplate(t *templateDomain.Template) *templateDomain.Template {
	raw, _ := json.Marshal(t)
	var out templateDomain.Template
	_ = json.Unmarshal(raw, &out)
	return &out
}
func (r *exampleTemplates) Update(_ context.Context, t *templateDomain.Template) error {
	r.templates[t.ID] = cloneTemplate(t)
	return nil
}
func (r *exampleTemplates) GetVersions(_ context.Context, app, name, locale string) ([]*templateDomain.Template, error) {
	out := []*templateDomain.Template{}
	for _, t := range r.templates {
		if t.AppID == app && t.Name == name && t.Locale == locale {
			out = append(out, cloneTemplate(t))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}
func (r *exampleTemplates) GetByVersion(ctx context.Context, app, name, locale string, v int) (*templateDomain.Template, error) {
	all, _ := r.GetVersions(ctx, app, name, locale)
	for _, t := range all {
		if t.Version == v {
			return t, nil
		}
	}
	return nil, fmt.Errorf("not found")
}
func (r *exampleTemplates) CreateVersion(ctx context.Context, t *templateDomain.Template) error {
	all, _ := r.GetVersions(ctx, t.AppID, t.Name, t.Locale)
	t.Version = 1
	for _, old := range all {
		if old.Version >= t.Version {
			t.Version = old.Version + 1
		}
	}
	return r.Create(ctx, t)
}
func (r *exampleTemplates) List(_ context.Context, f templateDomain.Filter) ([]*templateDomain.Template, int64, error) {
	out := []*templateDomain.Template{}
	for _, t := range r.templates {
		if t.AppID == f.AppID && (f.EnvironmentID == "" || t.EnvironmentID == f.EnvironmentID) {
			out = append(out, cloneTemplate(t))
		}
	}
	return out, int64(len(out)), nil
}

type exampleTopics struct {
	topic.Repository
	topics      map[string]*topic.Topic
	subscribers map[string][]string
}

func (r *exampleTopics) Create(_ context.Context, t *topic.Topic) error {
	r.topics[t.ID] = t
	return nil
}
func (r *exampleTopics) GetByID(_ context.Context, id string) (*topic.Topic, error) {
	return r.topics[id], nil
}
func (r *exampleTopics) GetByKey(_ context.Context, app, key string) (*topic.Topic, error) {
	for _, t := range r.topics {
		if t.AppID == app && t.Key == key {
			return t, nil
		}
	}
	return nil, fmt.Errorf("not found")
}
func (r *exampleTopics) AddSubscribers(_ context.Context, id, _ string, users []string) error {
	r.subscribers[id] = append(r.subscribers[id], users...)
	return nil
}

type exampleWorkflows struct {
	workflow.Repository
	workflows  map[string]*workflow.Workflow
	executions map[string]*workflow.WorkflowExecution
}

func (r *exampleWorkflows) CreateWorkflow(_ context.Context, w *workflow.Workflow) error {
	r.workflows[w.ID] = w
	return nil
}
func (r *exampleWorkflows) GetWorkflowByTrigger(_ context.Context, app, trigger string) (*workflow.Workflow, error) {
	for _, w := range r.workflows {
		if w.AppID == app && w.TriggerID == trigger {
			return w, nil
		}
	}
	return nil, fmt.Errorf("not found")
}
func (r *exampleWorkflows) CreateExecution(_ context.Context, e *workflow.WorkflowExecution) error {
	r.executions[e.ID] = e
	return nil
}

type exampleWorkflowQueue struct{ queue.WorkflowQueue }

func (*exampleWorkflowQueue) EnqueueWorkflow(context.Context, queue.WorkflowQueueItem) error {
	return nil
}

type exampleEnvironments struct {
	environment.Repository
	environments map[string]*environment.Environment
}

func (r *exampleEnvironments) Create(_ context.Context, e *environment.Environment) error {
	r.environments[e.ID] = e
	return nil
}
func (r *exampleEnvironments) GetByID(_ context.Context, id string) (*environment.Environment, error) {
	return r.environments[id], nil
}
func (r *exampleEnvironments) ListByApp(_ context.Context, app string) ([]environment.Environment, error) {
	out := []environment.Environment{}
	for _, e := range r.environments {
		if e.AppID == app {
			out = append(out, *e)
		}
	}
	return out, nil
}

func lifecycleRoutes(app *fiber.App, ah *handlers.ApplicationHandler, th *handlers.TemplateHandler, nh *handlers.NotificationHandler, tr *exampleTemplates, logger *zap.Logger) func(fiber.Router) {
	er := &exampleEnvironments{environments: map[string]*environment.Environment{}}
	eh := handlers.NewEnvironmentHandler(usecases.NewEnvironmentService(er, tr, nil, logger), validator.New(), logger)
	eh.SetAppHandler(ah)
	app.Post("/v1/apps/:id/environments", middleware.JWTAuth(&exampleAuth{}, logger), eh.Create)
	app.Post("/v1/apps/:id/environments/promote", middleware.JWTAuth(&exampleAuth{}, logger), eh.Promote)
	topics := &exampleTopics{topics: map[string]*topic.Topic{}, subscribers: map[string][]string{}}
	topicHandler := handlers.NewTopicHandler(services.NewTopicService(topics, logger), validator.New(), logger)
	wr := &exampleWorkflows{workflows: map[string]*workflow.Workflow{}, executions: map[string]*workflow.WorkflowExecution{}}
	wh := handlers.NewWorkflowHandler(services.NewWorkflowService(wr, &exampleWorkflowQueue{}, logger), validator.New(), logger)
	return func(v1 fiber.Router) {
		v1.Put("/templates/:id", th.UpdateTemplate)
		v1.Post("/templates/:id/render", th.RenderTemplate)
		v1.Post("/templates/:id/rollback", th.RollbackTemplate)
		v1.Post("/templates/:app_id/:name/versions", th.CreateTemplateVersion)
		v1.Get("/templates/:app_id/:name/versions", th.GetTemplateVersions)
		v1.Post("/topics", topicHandler.Create)
		v1.Post("/topics/:id/subscribers", topicHandler.AddSubscribers)
		v1.Post("/workflows", wh.Create)
		v1.Post("/workflows/trigger", wh.Trigger)
		v1.Post("/notifications/bulk", nh.SendBulk)
		v1.Post("/notifications/batch", nh.SendBatch)
		v1.Post("/notifications/broadcast", nh.Broadcast)
		v1.Post("/notifications/read", nh.MarkRead)
		v1.Patch("/notifications/bulk/archive", nh.BulkArchive)
		v1.Post("/notifications/:id/snooze", nh.Snooze)
		v1.Post("/notifications/:id/unsnooze", nh.Unsnooze)
		v1.Put("/notifications/:id/status", nh.UpdateStatus)
		v1.Delete("/notifications/:id", nh.Cancel)
		v1.Post("/notifications/:id/retry", nh.Retry)
	}
}
