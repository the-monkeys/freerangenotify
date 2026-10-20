package contracts

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	templateDomain "github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/handlers"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/middleware"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

type exampleApps struct {
	application.Repository
	apps map[string]*application.Application
}

func (r *exampleApps) Create(_ context.Context, a *application.Application) error {
	r.apps[a.AppID] = a
	return nil
}
func (r *exampleApps) GetByID(_ context.Context, id string) (*application.Application, error) {
	return r.apps[id], nil
}
func (r *exampleApps) GetByAPIKey(_ context.Context, key string) (*application.Application, error) {
	for _, a := range r.apps {
		if a.APIKey == key {
			return a, nil
		}
	}
	return nil, fmt.Errorf("invalid key")
}

type exampleUsers struct {
	user.Repository
	users map[string]*user.User
}

func (r *exampleUsers) Create(_ context.Context, u *user.User) error {
	r.users[u.UserID] = u
	return nil
}
func (r *exampleUsers) GetByID(_ context.Context, id string) (*user.User, error) {
	return r.users[id], nil
}
func (r *exampleUsers) GetByEmail(_ context.Context, app, email string) (*user.User, error) {
	for _, u := range r.users {
		if u.AppID == app && u.Email == email {
			return u, nil
		}
	}
	return nil, nil
}
func (r *exampleUsers) GetByExternalID(_ context.Context, app, id string) (*user.User, error) {
	for _, u := range r.users {
		if u.AppID == app && u.ExternalID == id {
			return u, nil
		}
	}
	return nil, nil
}
func (r *exampleUsers) Update(ctx context.Context, u *user.User) error { return r.Create(ctx, u) }
func (r *exampleUsers) Delete(_ context.Context, id string) error      { delete(r.users, id); return nil }

type exampleTemplates struct {
	templateDomain.Repository
	templates map[string]*templateDomain.Template
}

func (r *exampleTemplates) Create(_ context.Context, t *templateDomain.Template) error {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	t.CreatedAt = time.Now()
	r.templates[t.ID] = t
	return nil
}
func (r *exampleTemplates) GetByID(_ context.Context, id string) (*templateDomain.Template, error) {
	t := r.templates[id]
	if t == nil {
		return nil, fmt.Errorf("not found")
	}
	return t, nil
}
func (r *exampleTemplates) GetByAppAndName(_ context.Context, app, name, locale string) (*templateDomain.Template, error) {
	for _, t := range r.templates {
		if t.AppID == app && t.Name == name && t.Locale == locale {
			return t, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

type exampleNotifications struct {
	notification.Repository
	mu            sync.Mutex
	notifications map[string]*notification.Notification
}

func (r *exampleNotifications) Create(_ context.Context, n *notification.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notifications[n.NotificationID] = n
	return nil
}
func (r *exampleNotifications) GetByID(_ context.Context, id string) (*notification.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.notifications[id], nil
}

type exampleQueue struct{ queue.Queue }

func (*exampleQueue) Enqueue(context.Context, queue.NotificationQueueItem) error { return nil }

type exampleAuth struct{ auth.Service }

func (*exampleAuth) ValidateToken(_ context.Context, token string) (*auth.AdminUser, error) {
	if token != "fixture-dashboard-jwt" {
		return nil, fmt.Errorf("invalid JWT")
	}
	return &auth.AdminUser{UserID: "owner-fixture", Email: "owner@example.test"}, nil
}

func newExampleRouter(t *testing.T) *fiber.App {
	t.Helper()
	logger := zap.NewNop()
	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(logger)})
	ar := &exampleApps{apps: map[string]*application.Application{}}
	ur := &exampleUsers{users: map[string]*user.User{}}
	tr := &exampleTemplates{templates: map[string]*templateDomain.Template{}}
	nr := &exampleNotifications{notifications: map[string]*notification.Notification{}}
	as := services.NewApplicationService(ar, logger)
	ah := handlers.NewApplicationHandler(as, nil, nil, ar, validator.New(), logger)
	uh := handlers.NewUserHandler(services.NewUserService(ur, logger), validator.New(), logger)
	th := handlers.NewTemplateHandler(usecases.NewTemplateService(tr, logger), nil, logger)
	ns := usecases.NewNotificationService(nr, ur, ar, tr, &exampleQueue{}, logger, usecases.NotificationServiceConfig{MaxRetries: 3}, nil, nil)
	nh := handlers.NewNotificationHandler(ns, logger)
	app.Post("/v1/apps", middleware.JWTAuth(&exampleAuth{}, logger), ah.Create)
	v1 := app.Group("/v1", middleware.APIKeyAuth(as, logger))
	v1.Post("/users", uh.Create)
	v1.Get("/users/:id", uh.GetByID)
	v1.Put("/users/:id", uh.Update)
	v1.Delete("/users/:id", uh.Delete)
	v1.Post("/templates", th.CreateTemplate)
	v1.Post("/notifications", nh.Send)
	v1.Get("/notifications/:id", nh.Get)
	return app
}
