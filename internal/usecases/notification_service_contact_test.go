package usecases

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
	"go.uber.org/zap"
)

// ─── Mocks ─────────────────────────────────────────────────────────────────

type mockUserRepo struct {
	users map[string]*user.User
}

func (m *mockUserRepo) Create(ctx context.Context, u *user.User) error                 { return nil }
func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*user.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, fmt.Errorf("not found")
}
func (m *mockUserRepo) GetByExternalID(ctx context.Context, appID, externalID string) (*user.User, error) {
	return nil, fmt.Errorf("not found")
}
func (m *mockUserRepo) GetByEmail(ctx context.Context, appID, email string) (*user.User, error) {
	return nil, fmt.Errorf("not found")
}
func (m *mockUserRepo) Update(ctx context.Context, u *user.User) error                       { return nil }
func (m *mockUserRepo) List(ctx context.Context, f user.UserFilter) ([]*user.User, error)    { return nil, nil }
func (m *mockUserRepo) Delete(ctx context.Context, id string) error                          { return nil }
func (m *mockUserRepo) AddDevice(ctx context.Context, userID string, d user.Device) error    { return nil }
func (m *mockUserRepo) RemoveDevice(ctx context.Context, userID, deviceID string) error      { return nil }
func (m *mockUserRepo) UpdatePreferences(ctx context.Context, userID string, p user.Preferences) error {
	return nil
}
func (m *mockUserRepo) Count(ctx context.Context, f user.UserFilter) (int64, error) { return 0, nil }
func (m *mockUserRepo) BulkCreate(ctx context.Context, users []*user.User) error    { return nil }

type mockNotificationRepo struct {
	created []*notification.Notification
}

func (m *mockNotificationRepo) Create(ctx context.Context, n *notification.Notification) error {
	m.created = append(m.created, n)
	return nil
}
func (m *mockNotificationRepo) GetByID(ctx context.Context, id string) (*notification.Notification, error) {
	return nil, nil
}
func (m *mockNotificationRepo) List(ctx context.Context, f *notification.NotificationFilter) ([]*notification.Notification, error) {
	return nil, nil
}
func (m *mockNotificationRepo) UpdateStatus(ctx context.Context, id string, status notification.Status) error {
	return nil
}
func (m *mockNotificationRepo) BulkUpdateStatus(ctx context.Context, ids []string, status notification.Status) error {
	return nil
}
func (m *mockNotificationRepo) UpdateSnooze(ctx context.Context, id string, status notification.Status, t *time.Time) error {
	return nil
}
func (m *mockNotificationRepo) UpdateStatusWithError(ctx context.Context, notificationID string, status notification.Status, errorMessage string, appID string) error {
	return nil
}
func (m *mockNotificationRepo) Count(ctx context.Context, f *notification.NotificationFilter) (int64, error) {
	return 0, nil
}
func (m *mockNotificationRepo) Update(ctx context.Context, notification *notification.Notification) error {
	return nil
}
func (m *mockNotificationRepo) Delete(ctx context.Context, id string) error { return nil }
func (m *mockNotificationRepo) GetPending(ctx context.Context) ([]*notification.Notification, error) {
	return nil, nil
}
func (m *mockNotificationRepo) GetRetryable(ctx context.Context, maxRetries int) ([]*notification.Notification, error) {
	return nil, nil
}
func (m *mockNotificationRepo) IncrementRetryCount(ctx context.Context, id string, errorMessage string) error {
	return nil
}
func (m *mockNotificationRepo) BulkArchive(ctx context.Context, ids []string, archivedAt time.Time) error {
	return nil
}
func (m *mockNotificationRepo) MarkAllRead(ctx context.Context, userID, appID, category string) (int, error) {
	return 0, nil
}
func (m *mockNotificationRepo) ListSnoozedDue(ctx context.Context, now time.Time) ([]*notification.Notification, error) {
	return nil, nil
}

type mockAppRepo struct{ app *application.Application }

func (m *mockAppRepo) Create(ctx context.Context, a *application.Application) error                 { return nil }
func (m *mockAppRepo) GetByID(ctx context.Context, id string) (*application.Application, error)     { return m.app, nil }
func (m *mockAppRepo) Update(ctx context.Context, a *application.Application) error                 { return nil }
func (m *mockAppRepo) Delete(ctx context.Context, id string) error                                  { return nil }
func (m *mockAppRepo) List(ctx context.Context, f application.ApplicationFilter) ([]*application.Application, error) {
	return nil, nil
}
func (m *mockAppRepo) GetByAPIKey(ctx context.Context, apiKey string) (*application.Application, error) {
	return m.app, nil
}
func (m *mockAppRepo) RegenerateAPIKey(ctx context.Context, id string) (string, error) { return "", nil }

type mockTemplateRepoNSC struct{}

func (m *mockTemplateRepoNSC) Create(ctx context.Context, t *template.Template) error                           { return nil }
func (m *mockTemplateRepoNSC) Update(ctx context.Context, t *template.Template) error                           { return nil }
func (m *mockTemplateRepoNSC) GetByID(ctx context.Context, id string) (*template.Template, error)               { return nil, nil }
func (m *mockTemplateRepoNSC) GetLatestVersion(ctx context.Context, appID, name string) (*template.Template, error) {
	return nil, nil
}
func (m *mockTemplateRepoNSC) List(ctx context.Context, f template.Filter) ([]*template.Template, int64, error) {
	return nil, 0, nil
}
func (m *mockTemplateRepoNSC) Delete(ctx context.Context, id string) error { return nil }
func (m *mockTemplateRepoNSC) Count(ctx context.Context) (int64, error) { return 0, nil }
func (m *mockTemplateRepoNSC) CountByFilter(ctx context.Context, f template.Filter) (int64, error) {
	return 0, nil
}
func (m *mockTemplateRepoNSC) GetByAppAndName(ctx context.Context, appID, name, locale string) (*template.Template, error) {
	return nil, nil
}
func (m *mockTemplateRepoNSC) GetVersions(ctx context.Context, appID, name, locale string) ([]*template.Template, error) {
	return nil, nil
}
func (m *mockTemplateRepoNSC) GetByVersion(ctx context.Context, appID, name, locale string, version int) (*template.Template, error) {
	return nil, nil
}
func (m *mockTemplateRepoNSC) CreateVersion(ctx context.Context, t *template.Template) error { return nil }

type mockQueue struct {
	batch []queue.NotificationQueueItem
}

func (m *mockQueue) Enqueue(ctx context.Context, item queue.NotificationQueueItem) error                      { return nil }
func (m *mockQueue) EnqueuePriority(ctx context.Context, item queue.NotificationQueueItem) error              { return nil }
func (m *mockQueue) Dequeue(ctx context.Context) (*queue.NotificationQueueItem, error)                        { return nil, nil }
func (m *mockQueue) EnqueueBatch(ctx context.Context, items []queue.NotificationQueueItem) error              { m.batch = append(m.batch, items...); return nil }
func (m *mockQueue) GetQueueDepth(ctx context.Context) (map[string]int64, error)                              { return nil, nil }
func (m *mockQueue) Peek(ctx context.Context) (*queue.NotificationQueueItem, error)                           { return nil, nil }
func (m *mockQueue) ListDLQ(ctx context.Context, limit int) ([]queue.DLQItem, error)                          { return nil, nil }
func (m *mockQueue) ReplayDLQ(ctx context.Context, limit int) (int, error)                                    { return 0, nil }
func (m *mockQueue) ReplayDLQForApps(ctx context.Context, limit int, allowedApps map[string]bool) (int, error) { return 0, nil }
func (m *mockQueue) EnqueueScheduled(ctx context.Context, item queue.NotificationQueueItem, scheduledAt time.Time) error {
	return nil
}
func (m *mockQueue) GetScheduledItems(ctx context.Context, limit int64) ([]queue.NotificationQueueItem, error) { return nil, nil }
func (m *mockQueue) RemoveScheduledByID(ctx context.Context, ids []string) error                               { return nil }
func (m *mockQueue) Acknowledge(ctx context.Context, item queue.NotificationQueueItem) error                   { return nil }
func (m *mockQueue) RequeueExpiredProcessing(ctx context.Context) (int, error)                                 { return 0, nil }
func (m *mockQueue) Close() error                                                                              { return nil }

type mockLimiter struct{}

func (m *mockLimiter) IncrementAndCheckDailyLimit(ctx context.Context, key string, limit int) (bool, error) { return true, nil }
func (m *mockLimiter) ResetDailyLimit(ctx context.Context, key string) error                                 { return nil }
func (m *mockLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)  { return true, nil }

// ─── Tests ────────────────────────────────────────────────────────────────

func newServiceForTest(users map[string]*user.User) (*NotificationService, *mockNotificationRepo, *mockQueue) {
	nrepo := &mockNotificationRepo{}
	q := &mockQueue{}
	app := &application.Application{AppID: "app-1", Settings: application.Settings{DailyEmailLimit: 0}}
	svc := NewNotificationService(
		nrepo,
		&mockUserRepo{users: users},
		&mockAppRepo{app: app},
		&mockTemplateRepoNSC{},
		q,
		zap.NewNop(),
		NotificationServiceConfig{MaxRetries: 3},
		nil,
		&mockLimiter{},
	).(*NotificationService)
	return svc, nrepo, q
}

func TestSendBulk_DedupAndMissingContact(t *testing.T) {
	users := map[string]*user.User{
		"u1": {UserID: "u1", AppID: "app-1", Phone: "+1 222-333-4444"},
		"u2": {UserID: "u2", AppID: "app-1", Phone: "  +12223334444  "}, // duplicate after normalize
		"u3": {UserID: "u3", AppID: "app-1", Phone: ""},                 // missing
		"u4": {UserID: "u4", AppID: "app-1", Phone: "+15550001111"},
	}

	svc, nrepo, q := newServiceForTest(users)

	req := notification.BulkSendRequest{
		AppID:    "app-1",
		UserIDs:  []string{"u1", "u2", "u3", "u4"},
		Channel:  notification.ChannelSMS,
		Priority: notification.PriorityNormal,
		Title:    "hi",
		Body:     "body",
	}

	ctx := context.Background()
	res, err := svc.SendBulk(ctx, req)
	require.NoError(t, err)

	// u1 and u2 dedup to one; u3 skipped; u4 sent => 2 notifications
	require.Len(t, res, 2)
	require.Len(t, nrepo.created, 2)
	require.Len(t, q.batch, 2) // enqueued two items
}

func TestSend_SkipMissingEmail(t *testing.T) {
	users := map[string]*user.User{
		"u1": {UserID: "u1", AppID: "app-1", Email: ""},
	}
	svc, nrepo, q := newServiceForTest(users)

	req := notification.SendRequest{
		AppID:    "app-1",
		UserID:   "u1",
		Channel:  notification.ChannelEmail,
		Priority: notification.PriorityNormal,
		Title:    "t",
		Body:     "b",
	}

	ctx := context.Background()
	resp, err := svc.Send(ctx, req)
	require.NoError(t, err)
	require.Nil(t, resp)
	require.Len(t, nrepo.created, 0)
	require.Len(t, q.batch, 0)
}
