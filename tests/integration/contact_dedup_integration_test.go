//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"go.uber.org/zap"
)

// lightweight mocks (scoped to integration tests)
type (
	iUserRepo struct{ users map[string]*user.User }
	iNotifRepo struct{ created []*notification.Notification }
	iAppRepo struct{ app *application.Application }
	iTemplateRepo struct{}
	iQueue struct{ batch []queue.NotificationQueueItem }
	iLimiter struct{}
)

func (m *iUserRepo) Create(context.Context, *user.User) error                                 { return nil }
func (m *iUserRepo) GetByID(ctx context.Context, id string) (*user.User, error)               { if u, ok := m.users[id]; ok { return u, nil }; return nil, fmt.Errorf("not found") }
func (m *iUserRepo) GetByExternalID(context.Context, string, string) (*user.User, error)      { return nil, fmt.Errorf("not found") }
func (m *iUserRepo) GetByEmail(context.Context, string, string) (*user.User, error)           { return nil, fmt.Errorf("not found") }
func (m *iUserRepo) Update(context.Context, *user.User) error                                 { return nil }
func (m *iUserRepo) List(context.Context, user.UserFilter) ([]*user.User, error)              { return nil, nil }
func (m *iUserRepo) Delete(context.Context, string) error                                     { return nil }
func (m *iUserRepo) AddDevice(context.Context, string, user.Device) error                     { return nil }
func (m *iUserRepo) RemoveDevice(context.Context, string, string) error                       { return nil }
func (m *iUserRepo) UpdatePreferences(context.Context, string, user.Preferences) error        { return nil }
func (m *iUserRepo) Count(context.Context, user.UserFilter) (int64, error)                    { return 0, nil }
func (m *iUserRepo) BulkCreate(context.Context, []*user.User) error                           { return nil }

func (m *iNotifRepo) Create(context.Context, *notification.Notification) error                { return nil }
func (m *iNotifRepo) GetByID(context.Context, string) (*notification.Notification, error)     { return nil, nil }
func (m *iNotifRepo) Update(context.Context, *notification.Notification) error                { return nil }
func (m *iNotifRepo) List(context.Context, *notification.NotificationFilter) ([]*notification.Notification, error) {
	return nil, nil
}
func (m *iNotifRepo) Delete(context.Context, string) error                                    { return nil }
func (m *iNotifRepo) Count(context.Context, *notification.NotificationFilter) (int64, error)  { return 0, nil }
func (m *iNotifRepo) UpdateStatus(context.Context, string, notification.Status) error         { return nil }
func (m *iNotifRepo) BulkUpdateStatus(context.Context, []string, notification.Status) error   { return nil }
func (m *iNotifRepo) GetPending(context.Context) ([]*notification.Notification, error)        { return nil, nil }
func (m *iNotifRepo) GetRetryable(context.Context, int) ([]*notification.Notification, error) { return nil, nil }
func (m *iNotifRepo) IncrementRetryCount(context.Context, string, string) error               { return nil }
func (m *iNotifRepo) UpdateSnooze(context.Context, string, notification.Status, *time.Time) error {
	return nil
}
func (m *iNotifRepo) BulkArchive(context.Context, []string, time.Time) error                  { return nil }
func (m *iNotifRepo) MarkAllRead(context.Context, string, string, string) (int, error)        { return 0, nil }
func (m *iNotifRepo) ListSnoozedDue(context.Context, time.Time) ([]*notification.Notification, error) {
	return nil, nil
}
func (m *iNotifRepo) UpdateStatusWithError(context.Context, string, notification.Status, string, string) error {
	return nil
}

func (m *iAppRepo) Create(context.Context, *application.Application) error                    { return nil }
func (m *iAppRepo) GetByID(context.Context, string) (*application.Application, error)         { return m.app, nil }
func (m *iAppRepo) GetByAPIKey(context.Context, string) (*application.Application, error)     { return m.app, nil }
func (m *iAppRepo) Update(context.Context, *application.Application) error                    { return nil }
func (m *iAppRepo) List(context.Context, application.ApplicationFilter) ([]*application.Application, error) {
	return nil, nil
}
func (m *iAppRepo) Delete(context.Context, string) error                                      { return nil }
func (m *iAppRepo) RegenerateAPIKey(context.Context, string) (string, error)                  { return "", nil }

func (m *iTemplateRepo) Create(context.Context, *template.Template) error                     { return nil }
func (m *iTemplateRepo) GetByID(context.Context, string) (*template.Template, error)          { return nil, nil }
func (m *iTemplateRepo) GetByAppAndName(context.Context, string, string, string) (*template.Template, error) {
	return nil, fmt.Errorf("not found")
}
func (m *iTemplateRepo) Update(context.Context, *template.Template) error                     { return nil }
func (m *iTemplateRepo) List(context.Context, template.Filter) ([]*template.Template, int64, error) {
	return nil, 0, nil
}
func (m *iTemplateRepo) Count(context.Context) (int64, error)                                 { return 0, nil }
func (m *iTemplateRepo) CountByFilter(context.Context, template.Filter) (int64, error)        { return 0, nil }
func (m *iTemplateRepo) Delete(context.Context, string) error                                 { return nil }
func (m *iTemplateRepo) GetVersions(context.Context, string, string, string) ([]*template.Template, error) {
	return nil, nil
}
func (m *iTemplateRepo) GetByVersion(context.Context, string, string, string, int) (*template.Template, error) {
	return nil, nil
}
func (m *iTemplateRepo) CreateVersion(context.Context, *template.Template) error              { return nil }

func (m *iQueue) Enqueue(context.Context, queue.NotificationQueueItem) error                               { return nil }
func (m *iQueue) EnqueuePriority(context.Context, queue.NotificationQueueItem) error                         { return nil }
func (m *iQueue) Dequeue(context.Context) (*queue.NotificationQueueItem, error)                             { return nil, nil }
func (m *iQueue) EnqueueBatch(ctx context.Context, items []queue.NotificationQueueItem) error               { m.batch = append(m.batch, items...); return nil }
func (m *iQueue) GetQueueDepth(context.Context) (map[string]int64, error)                                  { return nil, nil }
func (m *iQueue) Peek(context.Context) (*queue.NotificationQueueItem, error)                               { return nil, nil }
func (m *iQueue) ListDLQ(context.Context, int) ([]queue.DLQItem, error)                                    { return nil, nil }
func (m *iQueue) ReplayDLQ(context.Context, int) (int, error)                                              { return 0, nil }
func (m *iQueue) ReplayDLQForApps(context.Context, int, map[string]bool) (int, error)                      { return 0, nil }
func (m *iQueue) EnqueueScheduled(context.Context, queue.NotificationQueueItem, time.Time) error           { return nil }
func (m *iQueue) GetScheduledItems(context.Context, int64) ([]queue.NotificationQueueItem, error)          { return nil, nil }
func (m *iQueue) RemoveScheduledByID(context.Context, []string) error                                      { return nil }
func (m *iQueue) Acknowledge(context.Context, queue.NotificationQueueItem) error                           { return nil }
func (m *iQueue) RequeueExpiredProcessing(context.Context) (int, error)                                    { return 0, nil }
func (m *iQueue) Close() error                                                                             { return nil }

func (m *iLimiter) Allow(context.Context, string, int, time.Duration) (bool, error)                        { return true, nil }
func (m *iLimiter) IncrementAndCheckDailyLimit(context.Context, string, int) (bool, error)                 { return true, nil }
func (m *iLimiter) ResetDailyLimit(context.Context, string) error                                          { return nil }

func newSvc(users map[string]*user.User) *usecases.NotificationService {
	app := &application.Application{AppID: "app-1", Settings: application.Settings{}}
	svc := usecases.NewNotificationService(
		&iNotifRepo{},
		&iUserRepo{users: users},
		&iAppRepo{app: app},
		&iTemplateRepo{},
		&iQueue{},
		zap.NewNop(),
		usecases.NotificationServiceConfig{MaxRetries: 3},
		nil,
		&iLimiter{},
	)
	return svc.(*usecases.NotificationService)
}

// Ensures bulk dedup/skip logic operates with real service wiring (no HTTP layer).
func TestIntegration_BulkSkipAndDedupContacts(t *testing.T) {
	users := map[string]*user.User{
		"a": {UserID: "a", AppID: "app-1", Phone: "+919876543210"},
		"b": {UserID: "b", AppID: "app-1", Phone: "  09876543210"}, // duplicates after normalize
		"c": {UserID: "c", AppID: "app-1", Phone: ""},
		"d": {UserID: "d", AppID: "app-1", Phone: "+14155552671"},
	}

	svc := newSvc(users)
	req := notification.BulkSendRequest{
		AppID:    "app-1",
		UserIDs:  []string{"a", "b", "c", "d"},
		Channel:  notification.ChannelSMS,
		Priority: notification.PriorityNormal,
		Title:    "hello",
		Body:     "world",
	}
	got, err := svc.SendBulk(context.Background(), req)
	require.NoError(t, err)
	// expect only two unique phones enqueued
	require.Len(t, got, 2)
}

