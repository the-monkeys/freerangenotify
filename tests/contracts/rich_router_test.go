package contracts

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/otp"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/handlers"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

type exampleOTP struct {
	otp.Repository
	mu       sync.Mutex
	requests map[string]*otp.Request
	budgets  map[string]int
}

func copyOTP(r *otp.Request) *otp.Request {
	raw, _ := json.Marshal(r)
	var out otp.Request
	_ = json.Unmarshal(raw, &out)
	return &out
}
func (r *exampleOTP) Create(_ context.Context, request *otp.Request) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests[request.RequestID] = copyOTP(request)
	return nil
}
func (r *exampleOTP) Get(_ context.Context, id string) (*otp.Request, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	request := r.requests[id]
	if request == nil {
		return nil, otp.ErrNotFound
	}
	return copyOTP(request), nil
}
func (r *exampleOTP) CompareAndSwap(_ context.Context, old, next *otp.Request) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current := r.requests[old.RequestID]
	if current == nil {
		return otp.ErrNotFound
	}
	if current.Revision != old.Revision {
		return otp.ErrConflict
	}
	next.Revision++
	r.requests[next.RequestID] = copyOTP(next)
	return nil
}
func (r *exampleOTP) RecipientRateLimit(_ context.Context, app, recipient string, _ int) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := app + ":" + recipient
	r.budgets[key]++
	return r.budgets[key], nil
}

type noStoredConnections struct{ channelconnection.Repository }

func (noStoredConnections) Get(context.Context, string) (*channelconnection.Connection, error) {
	return nil, channelconnection.ErrNotFound
}
func (r *exampleApps) List(context.Context, application.ApplicationFilter) ([]*application.Application, error) {
	out := []*application.Application{}
	for _, app := range r.apps {
		out = append(out, app)
	}
	return out, nil
}

func richRoutes(v1 fiber.Router, ns notification.Service, apps *exampleApps, users *exampleUsers, logger *zap.Logger) {
	ns.(*usecases.NotificationService).SetChannelDispatch(&services.ChannelDispatchService{Connections: &services.ChannelConnectionService{Apps: apps, Repo: noStoredConnections{}}, Logger: logger})
	state := &exampleOTP{requests: map[string]*otp.Request{}, budgets: map[string]int{}}
	handler := handlers.NewOTPHandler(services.NewOTPService(state, ns, users, nil, logger), validator.New(), logger)
	v1.Post("/otp/send", handler.Send)
	v1.Post("/otp/verify", handler.Verify)
	v1.Post("/otp/resend", handler.Resend)
}
