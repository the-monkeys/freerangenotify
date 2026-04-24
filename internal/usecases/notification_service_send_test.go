package usecases

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	notifqueue "github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"go.uber.org/zap"
)

// ─── Additional mocks for NotificationService.Send tests ──────────────────

// mockNotifRepoSend extends mockNotificationRepo with configurable error behavior.
type mockNotifRepoSend struct {
	mockNotificationRepo
	createErr    error
	updateErr    error
	statusUpdate map[string]notification.Status
}

func (m *mockNotifRepoSend) Create(ctx context.Context, n *notification.Notification) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = append(m.created, n)
	return nil
}

func (m *mockNotifRepoSend) UpdateStatus(ctx context.Context, id string, status notification.Status) error {
	if m.statusUpdate == nil {
		m.statusUpdate = make(map[string]notification.Status)
	}
	m.statusUpdate[id] = status
	return nil
}

// mockQueueSend extends mockQueue with configurable errors.
type mockQueueSend struct {
	mockQueue
	enqueueErr          error
	enqueueScheduledErr error
	scheduled           []struct {
		Item notifqueue.NotificationQueueItem
		At   time.Time
	}
}

func (m *mockQueueSend) Enqueue(ctx context.Context, item notifqueue.NotificationQueueItem) error {
	if m.enqueueErr != nil {
		return m.enqueueErr
	}
	m.batch = append(m.batch, item)
	return nil
}

func (m *mockQueueSend) EnqueueScheduled(ctx context.Context, item notifqueue.NotificationQueueItem, at time.Time) error {
	if m.enqueueScheduledErr != nil {
		return m.enqueueScheduledErr
	}
	m.scheduled = append(m.scheduled, struct {
		Item notifqueue.NotificationQueueItem
		At   time.Time
	}{item, at})
	return nil
}

// mockUserRepoSend extends mockUserRepo with GetByEmail support.
type mockUserRepoSend struct {
	mockUserRepo
}

func (m *mockUserRepoSend) GetByEmail(ctx context.Context, appID, email string) (*user.User, error) {
	for _, u := range m.users {
		if u.AppID == appID && u.Email == email {
			return u, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

// mockAppRepoSend allows per-ID lookup and configurable missing-app error.
type mockAppRepoSend struct {
	apps map[string]*application.Application
}

func (m *mockAppRepoSend) Create(ctx context.Context, a *application.Application) error { return nil }
func (m *mockAppRepoSend) GetByID(ctx context.Context, id string) (*application.Application, error) {
	if app, ok := m.apps[id]; ok {
		return app, nil
	}
	return nil, fmt.Errorf("application not found")
}
func (m *mockAppRepoSend) Update(ctx context.Context, a *application.Application) error { return nil }
func (m *mockAppRepoSend) Delete(ctx context.Context, id string) error                  { return nil }
func (m *mockAppRepoSend) List(ctx context.Context, f application.ApplicationFilter) ([]*application.Application, error) {
	return nil, nil
}
func (m *mockAppRepoSend) GetByAPIKey(ctx context.Context, apiKey string) (*application.Application, error) {
	return nil, nil
}
func (m *mockAppRepoSend) RegenerateAPIKey(ctx context.Context, id string) (string, error) {
	return "", nil
}

// mockLimiterSend allows configurable limit responses.
type mockLimiterSend struct {
	allowed          bool
	incrementAndResp map[string]bool // key → allowed
}

func (m *mockLimiterSend) IncrementAndCheckDailyLimit(ctx context.Context, key string, limit int) (bool, error) {
	if m.incrementAndResp != nil {
		if v, ok := m.incrementAndResp[key]; ok {
			return v, nil
		}
	}
	return m.allowed, nil
}
func (m *mockLimiterSend) ResetDailyLimit(ctx context.Context, key string) error { return nil }
func (m *mockLimiterSend) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	return m.allowed, nil
}

// ─── Helper ────────────────────────────────────────────────────────────────

func newSendServiceForTest(
	users map[string]*user.User,
	apps map[string]*application.Application,
	nrepo *mockNotifRepoSend,
	q *mockQueueSend,
	lim *mockLimiterSend,
) *NotificationService {
	if nrepo == nil {
		nrepo = &mockNotifRepoSend{}
	}
	if q == nil {
		q = &mockQueueSend{}
	}
	if lim == nil {
		lim = &mockLimiterSend{allowed: true}
	}
	svc := NewNotificationService(
		nrepo,
		&mockUserRepoSend{mockUserRepo{users: users}},
		&mockAppRepoSend{apps: apps},
		&mockTemplateRepoNSC{},
		q,
		zap.NewNop(),
		NotificationServiceConfig{MaxRetries: 3},
		nil,
		lim,
	).(*NotificationService)
	return svc
}

func defaultApp() *application.Application {
	return &application.Application{
		AppID:    "app-1",
		Settings: application.Settings{DailyEmailLimit: 0},
	}
}

func defaultUser() *user.User {
	return &user.User{
		UserID: "user-1",
		AppID:  "app-1",
		Email:  "alice@example.com",
		Phone:  "+15551234567",
		Preferences: user.Preferences{
			EmailEnabled: boolPtr(true),
			PushEnabled:  boolPtr(true),
			SMSEnabled:   boolPtr(true),
		},
	}
}

func defaultApps() map[string]*application.Application {
	a := defaultApp()
	return map[string]*application.Application{a.AppID: a}
}

func defaultUsers() map[string]*user.User {
	u := defaultUser()
	return map[string]*user.User{u.UserID: u}
}

// ─── Tests ────────────────────────────────────────────────────────────────

func TestNotificationService_Send(t *testing.T) {
	t.Run("successful email send", func(t *testing.T) {
		nrepo := &mockNotifRepoSend{}
		q := &mockQueueSend{}
		svc := newSendServiceForTest(defaultUsers(), defaultApps(), nrepo, q, nil)

		notif, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			UserID:   "user-1",
			Channel:  notification.ChannelEmail,
			Priority: notification.PriorityNormal,
			Title:    "Hello",
			Body:     "World",
		})

		require.NoError(t, err)
		require.NotNil(t, notif)
		assert.Equal(t, notification.StatusQueued, notif.Status)
		assert.Equal(t, "app-1", notif.AppID)
		assert.Equal(t, "user-1", notif.UserID)
		assert.Equal(t, notification.ChannelEmail, notif.Channel)
		assert.Len(t, nrepo.created, 1)
		assert.Len(t, q.batch, 1)
		assert.Equal(t, notif.NotificationID, q.batch[0].NotificationID)
	})

	t.Run("webhook-like channel with empty UserID succeeds", func(t *testing.T) {
		nrepo := &mockNotifRepoSend{}
		q := &mockQueueSend{}
		svc := newSendServiceForTest(defaultUsers(), defaultApps(), nrepo, q, nil)

		notif, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			UserID:   "",
			Channel:  notification.ChannelDiscord,
			Priority: notification.PriorityNormal,
			Title:    "Alert",
			Body:     "Server down",
			Data:     map[string]interface{}{"webhook_url": "https://discord.com/api/webhooks/123/abc"},
		})

		require.NoError(t, err)
		require.NotNil(t, notif)
		assert.Equal(t, "", notif.UserID)
		assert.Equal(t, notification.ChannelDiscord, notif.Channel)
		// webhook_url moved from Data to Metadata
		assert.Equal(t, "https://discord.com/api/webhooks/123/abc", notif.Metadata["webhook_url"])
	})

	t.Run("slack webhook-like channel without user", func(t *testing.T) {
		nrepo := &mockNotifRepoSend{}
		q := &mockQueueSend{}
		svc := newSendServiceForTest(defaultUsers(), defaultApps(), nrepo, q, nil)

		notif, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			Channel:  notification.ChannelSlack,
			Priority: notification.PriorityNormal,
			Title:    "Build failed",
			Body:     "Pipeline #42",
			Data:     map[string]interface{}{"webhook_url": "https://hooks.slack.com/xxx"},
		})

		require.NoError(t, err)
		require.NotNil(t, notif)
		assert.Equal(t, notification.ChannelSlack, notif.Channel)
	})

	t.Run("teams webhook-like channel without user", func(t *testing.T) {
		nrepo := &mockNotifRepoSend{}
		q := &mockQueueSend{}
		svc := newSendServiceForTest(defaultUsers(), defaultApps(), nrepo, q, nil)

		notif, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			Channel:  notification.ChannelTeams,
			Priority: notification.PriorityNormal,
			Title:    "Incident",
			Body:     "P1 alert",
			Data:     map[string]interface{}{"webhook_url": "https://teams.microsoft.com/l/xxx"},
		})

		require.NoError(t, err)
		require.NotNil(t, notif)
		assert.Equal(t, notification.ChannelTeams, notif.Channel)
	})

	t.Run("user not found returns error", func(t *testing.T) {
		svc := newSendServiceForTest(defaultUsers(), defaultApps(), nil, nil, nil)

		_, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			UserID:   "nonexistent",
			Channel:  notification.ChannelEmail,
			Priority: notification.PriorityNormal,
			Title:    "Hello",
			Body:     "World",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "user not found")
	})

	t.Run("missing phone for SMS returns nil (skip)", func(t *testing.T) {
		u := defaultUser()
		u.Phone = "" // no phone
		users := map[string]*user.User{u.UserID: u}
		svc := newSendServiceForTest(users, defaultApps(), nil, nil, nil)

		notif, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			UserID:   "user-1",
			Channel:  notification.ChannelSMS,
			Priority: notification.PriorityNormal,
			Title:    "Code",
			Body:     "Your code is 1234",
		})

		assert.NoError(t, err)
		assert.Nil(t, notif) // skipped, not an error
	})

	t.Run("missing email for email channel returns nil (skip)", func(t *testing.T) {
		u := defaultUser()
		u.Email = "" // no email
		users := map[string]*user.User{u.UserID: u}
		svc := newSendServiceForTest(users, defaultApps(), nil, nil, nil)

		notif, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			UserID:   "user-1",
			Channel:  notification.ChannelEmail,
			Priority: notification.PriorityNormal,
			Title:    "Hello",
			Body:     "World",
		})

		assert.NoError(t, err)
		assert.Nil(t, notif)
	})

	t.Run("DND enabled blocks non-critical", func(t *testing.T) {
		u := defaultUser()
		u.Preferences.DND = true
		users := map[string]*user.User{u.UserID: u}
		svc := newSendServiceForTest(users, defaultApps(), nil, nil, nil)

		_, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			UserID:   "user-1",
			Channel:  notification.ChannelEmail,
			Priority: notification.PriorityNormal,
			Title:    "Hello",
			Body:     "World",
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrDNDEnabled)
	})

	t.Run("DND enabled allows critical priority", func(t *testing.T) {
		u := defaultUser()
		u.Preferences.DND = true
		users := map[string]*user.User{u.UserID: u}
		svc := newSendServiceForTest(users, defaultApps(), nil, nil, nil)

		notif, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			UserID:   "user-1",
			Channel:  notification.ChannelEmail,
			Priority: notification.PriorityCritical,
			Title:    "URGENT",
			Body:     "System down",
		})

		require.NoError(t, err)
		require.NotNil(t, notif)
		assert.Equal(t, notification.PriorityCritical, notif.Priority)
	})

	t.Run("channel disabled in user preferences", func(t *testing.T) {
		u := defaultUser()
		u.Preferences.EmailEnabled = boolPtr(false)
		users := map[string]*user.User{u.UserID: u}
		svc := newSendServiceForTest(users, defaultApps(), nil, nil, nil)

		_, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			UserID:   "user-1",
			Channel:  notification.ChannelEmail,
			Priority: notification.PriorityNormal,
			Title:    "Hi",
			Body:     "Body",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "channel email is not enabled")
	})

	t.Run("user daily limit exceeded blocks non-critical", func(t *testing.T) {
		u := defaultUser()
		u.Preferences.DailyLimit = 10
		users := map[string]*user.User{u.UserID: u}
		lim := &mockLimiterSend{
			allowed:          true,
			incrementAndResp: map[string]bool{"user:user-1": false},
		}
		svc := newSendServiceForTest(users, defaultApps(), nil, nil, lim)

		_, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			UserID:   "user-1",
			Channel:  notification.ChannelEmail,
			Priority: notification.PriorityNormal,
			Title:    "Hi",
			Body:     "Body",
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrRateLimitExceeded)
	})

	t.Run("user daily limit exceeded allows critical", func(t *testing.T) {
		u := defaultUser()
		u.Preferences.DailyLimit = 10
		users := map[string]*user.User{u.UserID: u}
		lim := &mockLimiterSend{
			allowed:          true,
			incrementAndResp: map[string]bool{"user:user-1": false},
		}
		svc := newSendServiceForTest(users, defaultApps(), nil, nil, lim)

		notif, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			UserID:   "user-1",
			Channel:  notification.ChannelEmail,
			Priority: notification.PriorityCritical,
			Title:    "URGENT",
			Body:     "System down",
		})

		require.NoError(t, err)
		require.NotNil(t, notif)
	})

	t.Run("app daily email limit exceeded blocks non-critical", func(t *testing.T) {
		app := &application.Application{
			AppID:    "app-1",
			Settings: application.Settings{DailyEmailLimit: 100},
		}
		apps := map[string]*application.Application{app.AppID: app}
		lim := &mockLimiterSend{
			allowed:          true,
			incrementAndResp: map[string]bool{"app_email_limit:app-1": false},
		}
		svc := newSendServiceForTest(defaultUsers(), apps, nil, nil, lim)

		_, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			UserID:   "user-1",
			Channel:  notification.ChannelEmail,
			Priority: notification.PriorityNormal,
			Title:    "Hi",
			Body:     "Body",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "application daily email limit exceeded")
	})

	t.Run("app daily email limit allows critical", func(t *testing.T) {
		app := &application.Application{
			AppID:    "app-1",
			Settings: application.Settings{DailyEmailLimit: 100},
		}
		apps := map[string]*application.Application{app.AppID: app}
		lim := &mockLimiterSend{
			allowed:          true,
			incrementAndResp: map[string]bool{"app_email_limit:app-1": false},
		}
		svc := newSendServiceForTest(defaultUsers(), apps, nil, nil, lim)

		notif, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			UserID:   "user-1",
			Channel:  notification.ChannelEmail,
			Priority: notification.PriorityCritical,
			Title:    "URGENT",
			Body:     "Fire",
		})

		require.NoError(t, err)
		require.NotNil(t, notif)
	})

	t.Run("webhook_url moved from Data to Metadata", func(t *testing.T) {
		nrepo := &mockNotifRepoSend{}
		svc := newSendServiceForTest(defaultUsers(), defaultApps(), nrepo, nil, nil)

		notif, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			Channel:  notification.ChannelWebhook,
			Priority: notification.PriorityNormal,
			Title:    "Event",
			Body:     "Payload",
			Data:     map[string]interface{}{"webhook_url": "https://example.com/hook", "key": "value"},
		})

		require.NoError(t, err)
		require.NotNil(t, notif)
		assert.Equal(t, "https://example.com/hook", notif.Metadata["webhook_url"])
		// webhook_url removed from content data
		_, exists := notif.Content.Data["webhook_url"]
		assert.False(t, exists)
		// Other data preserved
		assert.Equal(t, "value", notif.Content.Data["key"])
	})

	t.Run("metadata digest_key copied to notification", func(t *testing.T) {
		nrepo := &mockNotifRepoSend{}
		svc := newSendServiceForTest(defaultUsers(), defaultApps(), nrepo, nil, nil)

		notif, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			UserID:   "user-1",
			Channel:  notification.ChannelEmail,
			Priority: notification.PriorityNormal,
			Title:    "Digest",
			Body:     "Summary",
			Metadata: map[string]interface{}{"digest_key": "daily_report"},
		})

		require.NoError(t, err)
		require.NotNil(t, notif.Metadata)
		assert.Equal(t, "daily_report", notif.Metadata["digest_key"])
	})

	t.Run("scheduled notification enqueued to scheduled queue", func(t *testing.T) {
		nrepo := &mockNotifRepoSend{}
		q := &mockQueueSend{}
		svc := newSendServiceForTest(defaultUsers(), defaultApps(), nrepo, q, nil)

		future := time.Now().Add(2 * time.Hour)
		notif, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:       "app-1",
			UserID:      "user-1",
			Channel:     notification.ChannelEmail,
			Priority:    notification.PriorityNormal,
			Title:       "Later",
			Body:        "Scheduled",
			ScheduledAt: &future,
		})

		require.NoError(t, err)
		require.NotNil(t, notif)
		assert.Equal(t, notification.StatusPending, notif.Status) // pending, not queued
		assert.Len(t, q.batch, 0)                                 // NOT in regular queue
		assert.Len(t, q.scheduled, 1)                             // in scheduled queue
	})

	t.Run("queue failure updates status to failed", func(t *testing.T) {
		nrepo := &mockNotifRepoSend{}
		q := &mockQueueSend{enqueueErr: fmt.Errorf("redis unavailable")}
		svc := newSendServiceForTest(defaultUsers(), defaultApps(), nrepo, q, nil)

		_, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			UserID:   "user-1",
			Channel:  notification.ChannelEmail,
			Priority: notification.PriorityNormal,
			Title:    "Hello",
			Body:     "World",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to enqueue")
		// Notification was created in repo
		assert.Len(t, nrepo.created, 1)
		// Status updated to failed
		assert.Equal(t, notification.StatusFailed, nrepo.statusUpdate[nrepo.created[0].NotificationID])
	})

	t.Run("application not found returns error", func(t *testing.T) {
		// Use webhook channel (no user resolution) so we reach the app lookup
		svc := newSendServiceForTest(defaultUsers(), map[string]*application.Application{}, nil, nil, nil)

		_, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-missing",
			Channel:  notification.ChannelWebhook,
			Priority: notification.PriorityNormal,
			Title:    "Hello",
			Body:     "World",
			Data:     map[string]interface{}{"webhook_url": "https://example.com/hook"},
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "application not found")
	})

	t.Run("invalid SendRequest fails validation", func(t *testing.T) {
		svc := newSendServiceForTest(defaultUsers(), defaultApps(), nil, nil, nil)

		_, err := svc.Send(context.Background(), notification.SendRequest{
			AppID: "", // missing
		})

		require.Error(t, err)
	})

	t.Run("push notification with user", func(t *testing.T) {
		nrepo := &mockNotifRepoSend{}
		q := &mockQueueSend{}
		svc := newSendServiceForTest(defaultUsers(), defaultApps(), nrepo, q, nil)

		notif, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			UserID:   "user-1",
			Channel:  notification.ChannelPush,
			Priority: notification.PriorityHigh,
			Title:    "Alert",
			Body:     "Check your account",
		})

		require.NoError(t, err)
		require.NotNil(t, notif)
		assert.Equal(t, notification.ChannelPush, notif.Channel)
		assert.Equal(t, notification.PriorityHigh, notif.Priority)
	})

	t.Run("content preserved in notification entity", func(t *testing.T) {
		nrepo := &mockNotifRepoSend{}
		svc := newSendServiceForTest(defaultUsers(), defaultApps(), nrepo, nil, nil)

		notif, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			UserID:   "user-1",
			Channel:  notification.ChannelEmail,
			Priority: notification.PriorityNormal,
			Title:    "My Title",
			Body:     "My Body",
			Data:     map[string]interface{}{"foo": "bar"},
			MediaURL: "https://example.com/img.png",
			Category: "alerts",
		})

		require.NoError(t, err)
		assert.Equal(t, "My Title", notif.Content.Title)
		assert.Equal(t, "My Body", notif.Content.Body)
		assert.Equal(t, "bar", notif.Content.Data["foo"])
		assert.Equal(t, "https://example.com/img.png", notif.Content.MediaURL)
		assert.Equal(t, "alerts", notif.Category)
	})

	t.Run("repo create failure returns error", func(t *testing.T) {
		nrepo := &mockNotifRepoSend{createErr: fmt.Errorf("es unavailable")}
		svc := newSendServiceForTest(defaultUsers(), defaultApps(), nrepo, nil, nil)

		_, err := svc.Send(context.Background(), notification.SendRequest{
			AppID:    "app-1",
			UserID:   "user-1",
			Channel:  notification.ChannelEmail,
			Priority: notification.PriorityNormal,
			Title:    "Hi",
			Body:     "Body",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create notification")
	})
}
