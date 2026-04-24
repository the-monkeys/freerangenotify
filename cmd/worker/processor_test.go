package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	templateDomain "github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"go.uber.org/zap"
)

// ─── Fakes for processNotification unit tests ─────────────────────────────

type stubQueue struct {
	items     []*queue.NotificationQueueItem
	enqueued  []queue.NotificationQueueItem
	scheduled []queue.NotificationQueueItem
}

func (s *stubQueue) Enqueue(_ context.Context, item queue.NotificationQueueItem) error {
	s.enqueued = append(s.enqueued, item)
	return nil
}
func (s *stubQueue) EnqueuePriority(_ context.Context, item queue.NotificationQueueItem) error {
	return nil
}
func (s *stubQueue) Dequeue(_ context.Context) (*queue.NotificationQueueItem, error) {
	if len(s.items) == 0 {
		return nil, nil
	}
	it := s.items[0]
	s.items = s.items[1:]
	return it, nil
}
func (s *stubQueue) EnqueueBatch(_ context.Context, items []queue.NotificationQueueItem) error {
	s.enqueued = append(s.enqueued, items...)
	return nil
}
func (s *stubQueue) GetQueueDepth(_ context.Context) (map[string]int64, error) { return nil, nil }
func (s *stubQueue) Peek(_ context.Context) (*queue.NotificationQueueItem, error) {
	return nil, nil
}
func (s *stubQueue) ListDLQ(_ context.Context, limit int) ([]queue.DLQItem, error) {
	return nil, nil
}
func (s *stubQueue) ReplayDLQ(_ context.Context, limit int) (int, error) { return 0, nil }
func (s *stubQueue) ReplayDLQForApps(_ context.Context, limit int, allowedApps map[string]bool) (int, error) {
	return 0, nil
}
func (s *stubQueue) EnqueueScheduled(_ context.Context, item queue.NotificationQueueItem, at time.Time) error {
	s.scheduled = append(s.scheduled, item)
	return nil
}
func (s *stubQueue) GetScheduledItems(_ context.Context, limit int64) ([]queue.NotificationQueueItem, error) {
	return nil, nil
}
func (s *stubQueue) RemoveScheduledByID(_ context.Context, ids []string) error { return nil }
func (s *stubQueue) Acknowledge(_ context.Context, item queue.NotificationQueueItem) error {
	return nil
}
func (s *stubQueue) RequeueExpiredProcessing(_ context.Context) (int, error) { return 0, nil }
func (s *stubQueue) Close() error                                            { return nil }

type stubNotifRepo struct {
	notifications map[string]*notification.Notification
	created       []*notification.Notification
	statusUpdates map[string]notification.Status
	retryIncr     map[string]int
}

func newStubNotifRepo() *stubNotifRepo {
	return &stubNotifRepo{
		notifications: make(map[string]*notification.Notification),
		statusUpdates: make(map[string]notification.Status),
		retryIncr:     make(map[string]int),
	}
}

func (s *stubNotifRepo) Create(_ context.Context, n *notification.Notification) error {
	s.created = append(s.created, n)
	s.notifications[n.NotificationID] = n
	return nil
}
func (s *stubNotifRepo) GetByID(_ context.Context, id string) (*notification.Notification, error) {
	if n, ok := s.notifications[id]; ok {
		return n, nil
	}
	return nil, fmt.Errorf("notification not found: %s", id)
}
func (s *stubNotifRepo) Update(_ context.Context, n *notification.Notification) error {
	s.notifications[n.NotificationID] = n
	return nil
}
func (s *stubNotifRepo) List(_ context.Context, f *notification.NotificationFilter) ([]*notification.Notification, error) {
	return nil, nil
}
func (s *stubNotifRepo) Delete(_ context.Context, id string) error { return nil }
func (s *stubNotifRepo) Count(_ context.Context, f *notification.NotificationFilter) (int64, error) {
	return 0, nil
}
func (s *stubNotifRepo) UpdateStatus(_ context.Context, id string, status notification.Status) error {
	s.statusUpdates[id] = status
	return nil
}
func (s *stubNotifRepo) BulkUpdateStatus(_ context.Context, ids []string, status notification.Status) error {
	return nil
}
func (s *stubNotifRepo) GetPending(_ context.Context) ([]*notification.Notification, error) {
	return nil, nil
}
func (s *stubNotifRepo) GetRetryable(_ context.Context, maxRetries int) ([]*notification.Notification, error) {
	return nil, nil
}
func (s *stubNotifRepo) IncrementRetryCount(_ context.Context, id string, _ string) error {
	s.retryIncr[id]++
	return nil
}
func (s *stubNotifRepo) UpdateSnooze(_ context.Context, id string, status notification.Status, t *time.Time) error {
	return nil
}
func (s *stubNotifRepo) BulkArchive(_ context.Context, ids []string, archivedAt time.Time) error {
	return nil
}
func (s *stubNotifRepo) MarkAllRead(_ context.Context, userID, appID, category string) (int, error) {
	return 0, nil
}
func (s *stubNotifRepo) ListSnoozedDue(_ context.Context, now time.Time) ([]*notification.Notification, error) {
	return nil, nil
}

type stubUserRepo struct {
	users map[string]*user.User
}

func (s *stubUserRepo) Create(_ context.Context, u *user.User) error { return nil }
func (s *stubUserRepo) GetByID(_ context.Context, id string) (*user.User, error) {
	if u, ok := s.users[id]; ok {
		return u, nil
	}
	return nil, fmt.Errorf("user not found: %s", id)
}
func (s *stubUserRepo) GetByExternalID(_ context.Context, appID, externalID string) (*user.User, error) {
	return nil, fmt.Errorf("not found")
}
func (s *stubUserRepo) GetByEmail(_ context.Context, appID, email string) (*user.User, error) {
	return nil, fmt.Errorf("not found")
}
func (s *stubUserRepo) Update(_ context.Context, u *user.User) error { return nil }
func (s *stubUserRepo) List(_ context.Context, f user.UserFilter) ([]*user.User, error) {
	return nil, nil
}
func (s *stubUserRepo) Delete(_ context.Context, id string) error                       { return nil }
func (s *stubUserRepo) AddDevice(_ context.Context, userID string, d user.Device) error { return nil }
func (s *stubUserRepo) RemoveDevice(_ context.Context, userID, deviceID string) error   { return nil }
func (s *stubUserRepo) UpdatePreferences(_ context.Context, userID string, p user.Preferences) error {
	return nil
}
func (s *stubUserRepo) Count(_ context.Context, f user.UserFilter) (int64, error) { return 0, nil }
func (s *stubUserRepo) BulkCreate(_ context.Context, users []*user.User) error    { return nil }

type stubAppRepo struct {
	apps map[string]*application.Application
}

func (s *stubAppRepo) Create(_ context.Context, a *application.Application) error { return nil }
func (s *stubAppRepo) GetByID(_ context.Context, id string) (*application.Application, error) {
	if a, ok := s.apps[id]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("app not found: %s", id)
}
func (s *stubAppRepo) GetByAPIKey(_ context.Context, key string) (*application.Application, error) {
	return nil, nil
}
func (s *stubAppRepo) Update(_ context.Context, a *application.Application) error { return nil }
func (s *stubAppRepo) List(_ context.Context, f application.ApplicationFilter) ([]*application.Application, error) {
	return nil, nil
}
func (s *stubAppRepo) Delete(_ context.Context, id string) error                     { return nil }
func (s *stubAppRepo) RegenerateAPIKey(_ context.Context, id string) (string, error) { return "", nil }

type stubTemplateRepo struct {
	templates map[string]*templateDomain.Template
}

func (s *stubTemplateRepo) Create(_ context.Context, t *templateDomain.Template) error { return nil }
func (s *stubTemplateRepo) GetByID(_ context.Context, id string) (*templateDomain.Template, error) {
	if t, ok := s.templates[id]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("template not found")
}
func (s *stubTemplateRepo) GetByAppAndName(_ context.Context, appID, name, locale string) (*templateDomain.Template, error) {
	for _, t := range s.templates {
		if t.AppID == appID && t.Name == name {
			return t, nil
		}
	}
	return nil, fmt.Errorf("template not found")
}
func (s *stubTemplateRepo) Update(_ context.Context, t *templateDomain.Template) error { return nil }
func (s *stubTemplateRepo) List(_ context.Context, f templateDomain.Filter) ([]*templateDomain.Template, int64, error) {
	return nil, 0, nil
}
func (s *stubTemplateRepo) Count(_ context.Context) (int64, error) { return 0, nil }
func (s *stubTemplateRepo) CountByFilter(_ context.Context, f templateDomain.Filter) (int64, error) {
	return 0, nil
}
func (s *stubTemplateRepo) Delete(_ context.Context, id string) error { return nil }
func (s *stubTemplateRepo) GetVersions(_ context.Context, appID, name, locale string) ([]*templateDomain.Template, error) {
	return nil, nil
}
func (s *stubTemplateRepo) GetByVersion(_ context.Context, appID, name, locale string, version int) (*templateDomain.Template, error) {
	return nil, nil
}
func (s *stubTemplateRepo) CreateVersion(_ context.Context, t *templateDomain.Template) error {
	return nil
}

type stubAuthService struct{}

func (s *stubAuthService) Register(_ context.Context, _ *auth.RegisterRequest) (*auth.AuthResponse, error) {
	return nil, nil
}
func (s *stubAuthService) Login(_ context.Context, _ *auth.LoginRequest) (*auth.AuthResponse, error) {
	return nil, nil
}
func (s *stubAuthService) RefreshAccessToken(_ context.Context, _ string) (*auth.TokenPair, error) {
	return nil, nil
}
func (s *stubAuthService) Logout(_ context.Context, _ string) error { return nil }
func (s *stubAuthService) ForgotPassword(_ context.Context, _ *auth.ForgotPasswordRequest) error {
	return nil
}
func (s *stubAuthService) ResetPassword(_ context.Context, _ *auth.ResetPasswordRequest) error {
	return nil
}
func (s *stubAuthService) ChangePassword(_ context.Context, _ string, _ *auth.ChangePasswordRequest) error {
	return nil
}
func (s *stubAuthService) DeleteOwnAccount(_ context.Context, _ string, _ *auth.DeleteAccountRequest) error {
	return nil
}
func (s *stubAuthService) DeleteAccountByAdmin(_ context.Context, _, _ string) error { return nil }
func (s *stubAuthService) GetCurrentUser(_ context.Context, userID string) (*auth.AdminUser, error) {
	return nil, nil
}
func (s *stubAuthService) ValidateToken(_ context.Context, _ string) (*auth.AdminUser, error) {
	return nil, nil
}
func (s *stubAuthService) SSOLogin(_ context.Context, _, _ string) (*auth.AuthResponse, error) {
	return nil, nil
}
func (s *stubAuthService) VerifyRegistrationOTP(_ context.Context, _ *auth.VerifyOTPRequest) (*auth.AuthResponse, error) {
	return nil, nil
}
func (s *stubAuthService) ResendRegistrationOTP(_ context.Context, _ *auth.ResendOTPRequest) error {
	return nil
}
func (s *stubAuthService) SendPhoneOTP(_ context.Context, _ string, _ *auth.PhoneOTPRequest) error {
	return nil
}
func (s *stubAuthService) VerifyPhoneOTP(_ context.Context, _ string, _ *auth.PhoneVerifyRequest) error {
	return nil
}

type stubLicenseChecker struct {
	enabled bool
	allowed bool
	mode    license.Mode
}

func (s *stubLicenseChecker) Enabled() bool      { return s.enabled }
func (s *stubLicenseChecker) Mode() license.Mode { return s.mode }
func (s *stubLicenseChecker) Check(_ context.Context, _ *application.Application) (license.Decision, error) {
	return license.Decision{
		Allowed:   s.allowed,
		Mode:      s.mode,
		State:     license.StateActive,
		Reason:    "test",
		CheckedAt: time.Now(),
	}, nil
}

// ─── Helper ────────────────────────────────────────────────────────────────

func newProcessorForTest(
	nrepo *stubNotifRepo,
	q *stubQueue,
	users map[string]*user.User,
	apps map[string]*application.Application,
	templates map[string]*templateDomain.Template,
	lc *stubLicenseChecker,
) *NotificationProcessor {
	if nrepo == nil {
		nrepo = newStubNotifRepo()
	}
	if q == nil {
		q = &stubQueue{}
	}
	if users == nil {
		users = map[string]*user.User{}
	}
	if apps == nil {
		apps = map[string]*application.Application{}
	}
	if templates == nil {
		templates = map[string]*templateDomain.Template{}
	}

	np := &NotificationProcessor{
		queue:           q,
		notifRepo:       nrepo,
		userRepo:        &stubUserRepo{users: users},
		appRepo:         &stubAppRepo{apps: apps},
		templateRepo:    &stubTemplateRepo{templates: templates},
		authService:     &stubAuthService{},
		providerManager: nil, // nil → simulated send (100ms sleep)
		redisClient:     nil,
		logger:          zap.NewNop(),
		config: ProcessorConfig{
			WorkerCount:   1,
			PollInterval:  10 * time.Millisecond,
			MaxRetries:    3,
			RetryDelay:    100 * time.Millisecond,
			MaxRetryDelay: 1 * time.Second,
		},
		metrics:  nil,
		stopChan: make(chan struct{}),
	}
	if lc != nil {
		np.licensingChecker = lc
	}
	return np
}

func makeQueueItem(notifID string) *queue.NotificationQueueItem {
	return &queue.NotificationQueueItem{
		NotificationID: notifID,
		AppID:          "app-1",
		Priority:       notification.PriorityNormal,
		EnqueuedAt:     time.Now(),
	}
}

func makeApp() *application.Application {
	return &application.Application{
		AppID:    "app-1",
		AppName:  "TestApp",
		Settings: application.Settings{},
	}
}

func makeUser() *user.User {
	return &user.User{
		UserID: "user-1",
		AppID:  "app-1",
		Email:  "alice@example.com",
		Phone:  "+15551234567",
		Preferences: user.Preferences{
			EmailEnabled: ptrBool(true),
			PushEnabled:  ptrBool(true),
			SMSEnabled:   ptrBool(true),
		},
	}
}

func ptrBool(b bool) *bool { return &b }

// ─── Tests ────────────────────────────────────────────────────────────────

func TestProcessNotification(t *testing.T) {
	t.Run("successful send — email with user and template", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		notif := &notification.Notification{
			NotificationID: "n-1",
			AppID:          "app-1",
			UserID:         "user-1",
			Channel:        notification.ChannelEmail,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusQueued,
			TemplateID:     "tmpl-1",
			Content: notification.Content{
				Title: "Hello",
				Body:  "World",
			},
		}
		nrepo.notifications["n-1"] = notif

		tmpl := &templateDomain.Template{
			ID:      "tmpl-1",
			AppID:   "app-1",
			Name:    "welcome",
			Channel: "email",
			Subject: "Welcome {{.name}}",
			Body:    "Hello {{.name}}, welcome!",
		}

		proc := newProcessorForTest(nrepo, nil,
			map[string]*user.User{"user-1": makeUser()},
			map[string]*application.Application{"app-1": makeApp()},
			map[string]*templateDomain.Template{"tmpl-1": tmpl},
			nil,
		)

		proc.processNotification(context.Background(), makeQueueItem("n-1"), zap.NewNop())

		assert.Equal(t, notification.StatusSent, nrepo.statusUpdates["n-1"])
	})

	t.Run("cancelled notification is skipped", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		nrepo.notifications["n-2"] = &notification.Notification{
			NotificationID: "n-2",
			AppID:          "app-1",
			UserID:         "user-1",
			Channel:        notification.ChannelEmail,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusCancelled,
			Content:        notification.Content{Title: "T", Body: "B"},
		}

		proc := newProcessorForTest(nrepo, nil, nil, nil, nil, nil)

		proc.processNotification(context.Background(), makeQueueItem("n-2"), zap.NewNop())

		// Status should NOT be updated to processing or sent
		_, updated := nrepo.statusUpdates["n-2"]
		assert.False(t, updated, "cancelled notification should not have status updated")
	})

	t.Run("snoozed notification is re-queued", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		snoozedUntil := time.Now().Add(1 * time.Hour) // still in the future
		nrepo.notifications["n-3"] = &notification.Notification{
			NotificationID: "n-3",
			AppID:          "app-1",
			UserID:         "user-1",
			Channel:        notification.ChannelEmail,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusSnoozed,
			SnoozedUntil:   &snoozedUntil,
			Content:        notification.Content{Title: "T", Body: "B"},
		}

		q := &stubQueue{}
		proc := newProcessorForTest(nrepo, q, nil, nil, nil, nil)

		proc.processNotification(context.Background(), makeQueueItem("n-3"), zap.NewNop())

		require.Len(t, q.scheduled, 1)
		assert.Equal(t, "n-3", q.scheduled[0].NotificationID)
		// Should NOT be marked as sent
		_, sentUpdated := nrepo.statusUpdates["n-3"]
		assert.False(t, sentUpdated)
	})

	t.Run("snoozed notification past due is processed normally", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		pastSnooze := time.Now().Add(-1 * time.Hour) // already past
		nrepo.notifications["n-3b"] = &notification.Notification{
			NotificationID: "n-3b",
			AppID:          "app-1",
			UserID:         "user-1",
			Channel:        notification.ChannelEmail,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusSnoozed,
			SnoozedUntil:   &pastSnooze,
			Content:        notification.Content{Title: "T", Body: "B"},
		}

		proc := newProcessorForTest(nrepo, nil,
			map[string]*user.User{"user-1": makeUser()},
			map[string]*application.Application{"app-1": makeApp()},
			nil, nil,
		)

		proc.processNotification(context.Background(), makeQueueItem("n-3b"), zap.NewNop())

		assert.Equal(t, notification.StatusSent, nrepo.statusUpdates["n-3b"])
	})

	t.Run("webhook-like channel — discord without user", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		nrepo.notifications["n-4"] = &notification.Notification{
			NotificationID: "n-4",
			AppID:          "app-1",
			UserID:         "", // no user for webhook-like channel
			Channel:        notification.ChannelDiscord,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusQueued,
			Content:        notification.Content{Title: "Alert", Body: "Server down"},
			Metadata:       map[string]interface{}{"webhook_url": "https://discord.com/api/webhooks/123/abc"},
		}

		proc := newProcessorForTest(nrepo, nil,
			nil,
			map[string]*application.Application{"app-1": makeApp()},
			nil, nil,
		)

		proc.processNotification(context.Background(), makeQueueItem("n-4"), zap.NewNop())

		assert.Equal(t, notification.StatusSent, nrepo.statusUpdates["n-4"])
	})

	t.Run("slack webhook-like without user", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		nrepo.notifications["n-4s"] = &notification.Notification{
			NotificationID: "n-4s",
			AppID:          "app-1",
			Channel:        notification.ChannelSlack,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusQueued,
			Content:        notification.Content{Title: "Build", Body: "Failed"},
			Metadata:       map[string]interface{}{"webhook_url": "https://hooks.slack.com/xxx"},
		}

		proc := newProcessorForTest(nrepo, nil, nil,
			map[string]*application.Application{"app-1": makeApp()},
			nil, nil,
		)

		proc.processNotification(context.Background(), makeQueueItem("n-4s"), zap.NewNop())

		assert.Equal(t, notification.StatusSent, nrepo.statusUpdates["n-4s"])
	})

	t.Run("teams webhook-like without user", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		nrepo.notifications["n-4t"] = &notification.Notification{
			NotificationID: "n-4t",
			AppID:          "app-1",
			Channel:        notification.ChannelTeams,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusQueued,
			Content:        notification.Content{Title: "Incident", Body: "P1"},
			Metadata:       map[string]interface{}{"webhook_url": "https://teams.microsoft.com/l/xxx"},
		}

		proc := newProcessorForTest(nrepo, nil, nil,
			map[string]*application.Application{"app-1": makeApp()},
			nil, nil,
		)

		proc.processNotification(context.Background(), makeQueueItem("n-4t"), zap.NewNop())

		assert.Equal(t, notification.StatusSent, nrepo.statusUpdates["n-4t"])
	})

	t.Run("missing user ID for non-webhook channel → failure", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		nrepo.notifications["n-5"] = &notification.Notification{
			NotificationID: "n-5",
			AppID:          "app-1",
			UserID:         "", // empty — not webhook-like
			Channel:        notification.ChannelEmail,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusQueued,
			Content:        notification.Content{Title: "Hi", Body: "World"},
		}

		proc := newProcessorForTest(nrepo, nil, nil,
			map[string]*application.Application{"app-1": makeApp()},
			nil, nil,
		)

		proc.processNotification(context.Background(), makeQueueItem("n-5"), zap.NewNop())

		// Should be handled as failure, retry count incremented
		assert.Greater(t, nrepo.retryIncr["n-5"], 0)
	})

	t.Run("user not found → failure", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		nrepo.notifications["n-6"] = &notification.Notification{
			NotificationID: "n-6",
			AppID:          "app-1",
			UserID:         "nonexistent",
			Channel:        notification.ChannelEmail,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusQueued,
			Content:        notification.Content{Title: "T", Body: "B"},
		}

		proc := newProcessorForTest(nrepo, nil,
			map[string]*user.User{}, // empty user map
			map[string]*application.Application{"app-1": makeApp()},
			nil, nil,
		)

		proc.processNotification(context.Background(), makeQueueItem("n-6"), zap.NewNop())

		assert.Greater(t, nrepo.retryIncr["n-6"], 0)
	})

	t.Run("user preferences block email", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		nrepo.notifications["n-7"] = &notification.Notification{
			NotificationID: "n-7",
			AppID:          "app-1",
			UserID:         "user-1",
			Channel:        notification.ChannelEmail,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusQueued,
			Content:        notification.Content{Title: "T", Body: "B"},
		}

		u := makeUser()
		u.Preferences.EmailEnabled = ptrBool(false) // email disabled

		proc := newProcessorForTest(nrepo, nil,
			map[string]*user.User{"user-1": u},
			map[string]*application.Application{"app-1": makeApp()},
			nil, nil,
		)

		proc.processNotification(context.Background(), makeQueueItem("n-7"), zap.NewNop())

		// Should be cancelled, not sent
		assert.Equal(t, notification.StatusCancelled, nrepo.statusUpdates["n-7"])
	})

	t.Run("notification not found in repo → returns early", func(t *testing.T) {
		nrepo := newStubNotifRepo() // empty — no notifications

		proc := newProcessorForTest(nrepo, nil, nil, nil, nil, nil)

		// Should not panic
		proc.processNotification(context.Background(), makeQueueItem("nonexistent"), zap.NewNop())

		_, updated := nrepo.statusUpdates["nonexistent"]
		assert.False(t, updated)
	})

	t.Run("licensing check blocks notification", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		nrepo.notifications["n-8"] = &notification.Notification{
			NotificationID: "n-8",
			AppID:          "app-1",
			UserID:         "user-1",
			Channel:        notification.ChannelEmail,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusQueued,
			Content:        notification.Content{Title: "T", Body: "B"},
		}

		lc := &stubLicenseChecker{
			enabled: true,
			allowed: false,
			mode:    license.ModeHosted,
		}

		proc := newProcessorForTest(nrepo, nil,
			map[string]*user.User{"user-1": makeUser()},
			map[string]*application.Application{"app-1": makeApp()},
			nil, lc,
		)

		proc.processNotification(context.Background(), makeQueueItem("n-8"), zap.NewNop())

		// Should be failed due to licensing
		assert.Equal(t, notification.StatusFailed, nrepo.statusUpdates["n-8"])
	})

	t.Run("licensing check passes → notification sent", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		nrepo.notifications["n-9"] = &notification.Notification{
			NotificationID: "n-9",
			AppID:          "app-1",
			UserID:         "user-1",
			Channel:        notification.ChannelEmail,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusQueued,
			Content:        notification.Content{Title: "T", Body: "B"},
		}

		lc := &stubLicenseChecker{
			enabled: true,
			allowed: true,
			mode:    license.ModeHosted,
		}

		proc := newProcessorForTest(nrepo, nil,
			map[string]*user.User{"user-1": makeUser()},
			map[string]*application.Application{"app-1": makeApp()},
			nil, lc,
		)

		proc.processNotification(context.Background(), makeQueueItem("n-9"), zap.NewNop())

		assert.Equal(t, notification.StatusSent, nrepo.statusUpdates["n-9"])
	})

	t.Run("template rendering populates content", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		tmplID := "00000000-0000-4000-a000-000000000010"
		nrepo.notifications["n-10"] = &notification.Notification{
			NotificationID: "n-10",
			AppID:          "app-1",
			UserID:         "user-1",
			Channel:        notification.ChannelEmail,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusQueued,
			TemplateID:     tmplID,
			Content: notification.Content{
				Data: map[string]interface{}{"name": "Alice"},
			},
		}

		tmpl := &templateDomain.Template{
			ID:      tmplID,
			AppID:   "app-1",
			Name:    "greeting",
			Channel: "email",
			Subject: "Hi {{.name}}",
			Body:    "Welcome {{.name}}!",
		}

		proc := newProcessorForTest(nrepo, nil,
			map[string]*user.User{"user-1": makeUser()},
			map[string]*application.Application{"app-1": makeApp()},
			map[string]*templateDomain.Template{tmplID: tmpl},
			nil,
		)

		proc.processNotification(context.Background(), makeQueueItem("n-10"), zap.NewNop())

		assert.Equal(t, notification.StatusSent, nrepo.statusUpdates["n-10"])
		// Check that template was rendered into the notification
		n := nrepo.notifications["n-10"]
		assert.Equal(t, "Hi Alice", n.Content.Title)
		assert.Equal(t, "Welcome Alice!", n.Content.Body)
	})

	t.Run("webhook target resolved from custom provider", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		tmplID := "00000000-0000-4000-a000-000000000011"
		nrepo.notifications["n-11"] = &notification.Notification{
			NotificationID: "n-11",
			AppID:          "app-1",
			Channel:        notification.ChannelDiscord,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusQueued,
			TemplateID:     tmplID,
			Content:        notification.Content{Title: "T", Body: "B"},
		}

		tmpl := &templateDomain.Template{
			ID:            tmplID,
			AppID:         "app-1",
			Name:          "discord-hook",
			Channel:       "discord",
			Subject:       "Alert",
			Body:          "Alert body",
			WebhookTarget: "my-discord-hook",
		}

		app := makeApp()
		app.Settings.CustomProviders = []application.CustomProviderConfig{
			{
				Name:       "my-discord-hook",
				Channel:    "discord",
				Active:     true,
				WebhookURL: "https://discord.com/api/webhooks/resolved/url",
			},
		}

		proc := newProcessorForTest(nrepo, nil,
			nil,
			map[string]*application.Application{"app-1": app},
			map[string]*templateDomain.Template{tmplID: tmpl},
			nil,
		)

		proc.processNotification(context.Background(), makeQueueItem("n-11"), zap.NewNop())

		assert.Equal(t, notification.StatusSent, nrepo.statusUpdates["n-11"])
		n := nrepo.notifications["n-11"]
		assert.Equal(t, "https://discord.com/api/webhooks/resolved/url", n.Metadata["webhook_url"])
	})

	t.Run("webhook channel (classic) without user succeeds", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		nrepo.notifications["n-12"] = &notification.Notification{
			NotificationID: "n-12",
			AppID:          "app-1",
			Channel:        notification.ChannelWebhook,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusQueued,
			Content:        notification.Content{Title: "Event", Body: "Payload"},
			Metadata:       map[string]interface{}{"webhook_url": "https://example.com/hook"},
		}

		proc := newProcessorForTest(nrepo, nil, nil,
			map[string]*application.Application{"app-1": makeApp()},
			nil, nil,
		)

		proc.processNotification(context.Background(), makeQueueItem("n-12"), zap.NewNop())

		assert.Equal(t, notification.StatusSent, nrepo.statusUpdates["n-12"])
	})

	t.Run("push notification with user preferences blocks push", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		nrepo.notifications["n-13"] = &notification.Notification{
			NotificationID: "n-13",
			AppID:          "app-1",
			UserID:         "user-1",
			Channel:        notification.ChannelPush,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusQueued,
			Content:        notification.Content{Title: "T", Body: "B"},
		}

		u := makeUser()
		u.Preferences.PushEnabled = ptrBool(false)

		proc := newProcessorForTest(nrepo, nil,
			map[string]*user.User{"user-1": u},
			map[string]*application.Application{"app-1": makeApp()},
			nil, nil,
		)

		proc.processNotification(context.Background(), makeQueueItem("n-13"), zap.NewNop())

		assert.Equal(t, notification.StatusCancelled, nrepo.statusUpdates["n-13"])
	})

	t.Run("licensing checker nil → no licensing check", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		nrepo.notifications["n-14"] = &notification.Notification{
			NotificationID: "n-14",
			AppID:          "app-1",
			UserID:         "user-1",
			Channel:        notification.ChannelEmail,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusQueued,
			Content:        notification.Content{Title: "T", Body: "B"},
		}

		proc := newProcessorForTest(nrepo, nil,
			map[string]*user.User{"user-1": makeUser()},
			map[string]*application.Application{"app-1": makeApp()},
			nil,
			nil, // no license checker
		)

		proc.processNotification(context.Background(), makeQueueItem("n-14"), zap.NewNop())

		assert.Equal(t, notification.StatusSent, nrepo.statusUpdates["n-14"])
	})

	t.Run("licensing checker disabled → no licensing check", func(t *testing.T) {
		nrepo := newStubNotifRepo()
		nrepo.notifications["n-15"] = &notification.Notification{
			NotificationID: "n-15",
			AppID:          "app-1",
			UserID:         "user-1",
			Channel:        notification.ChannelEmail,
			Priority:       notification.PriorityNormal,
			Status:         notification.StatusQueued,
			Content:        notification.Content{Title: "T", Body: "B"},
		}

		lc := &stubLicenseChecker{enabled: false}

		proc := newProcessorForTest(nrepo, nil,
			map[string]*user.User{"user-1": makeUser()},
			map[string]*application.Application{"app-1": makeApp()},
			nil, lc,
		)

		proc.processNotification(context.Background(), makeQueueItem("n-15"), zap.NewNop())

		assert.Equal(t, notification.StatusSent, nrepo.statusUpdates["n-15"])
	})
}
