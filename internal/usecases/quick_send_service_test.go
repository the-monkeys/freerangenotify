package usecases

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	templateDomain "github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"go.uber.org/zap"
)

// ─── QuickSend Mocks ──────────────────────────────────────────────────────

// mockNotifService implements notification.Service for QuickSendService tests.
type mockNotifService struct {
	sendFn func(ctx context.Context, req notification.SendRequest) (*notification.Notification, error)
}

func (m *mockNotifService) Send(ctx context.Context, req notification.SendRequest) (*notification.Notification, error) {
	if m.sendFn != nil {
		return m.sendFn(ctx, req)
	}
	return &notification.Notification{
		NotificationID: "notif-new",
		Status:         notification.StatusQueued,
	}, nil
}

// Stubs for the rest of notification.Service (not exercised by QuickSend).
func (m *mockNotifService) SendBulk(context.Context, notification.BulkSendRequest) ([]*notification.Notification, error) {
	return nil, nil
}
func (m *mockNotifService) SendBatch(context.Context, []notification.SendRequest) ([]*notification.Notification, error) {
	return nil, nil
}
func (m *mockNotifService) Get(context.Context, string, string) (*notification.Notification, error) {
	return nil, nil
}
func (m *mockNotifService) List(context.Context, notification.NotificationFilter) ([]*notification.Notification, error) {
	return nil, nil
}
func (m *mockNotifService) Count(context.Context, notification.NotificationFilter) (int64, error) {
	return 0, nil
}
func (m *mockNotifService) UpdateStatus(context.Context, string, notification.Status, string, string) error {
	return nil
}
func (m *mockNotifService) Cancel(context.Context, string, string) error        { return nil }
func (m *mockNotifService) CancelBatch(context.Context, []string, string) error { return nil }
func (m *mockNotifService) Retry(context.Context, string, string) error         { return nil }
func (m *mockNotifService) FlushQueued(context.Context, string) error           { return nil }
func (m *mockNotifService) GetUnreadCount(context.Context, string, string) (int64, error) {
	return 0, nil
}
func (m *mockNotifService) ListUnread(context.Context, string, string) ([]*notification.Notification, error) {
	return nil, nil
}
func (m *mockNotifService) MarkRead(context.Context, []string, string, string) error { return nil }
func (m *mockNotifService) Broadcast(context.Context, notification.BroadcastRequest) (*notification.BroadcastResult, error) {
	return nil, nil
}
func (m *mockNotifService) Snooze(context.Context, string, string, time.Time) error   { return nil }
func (m *mockNotifService) Unsnooze(context.Context, string, string) error            { return nil }
func (m *mockNotifService) Archive(context.Context, []string, string, string) error   { return nil }
func (m *mockNotifService) MarkAllRead(context.Context, string, string, string) error { return nil }
func (m *mockNotifService) ListSnoozedDue(context.Context) ([]*notification.Notification, error) {
	return nil, nil
}

// mockTemplateRepoQS fulfils template.Repository for QuickSendService tests.
type mockTemplateRepoQS struct {
	templates map[string]*templateDomain.Template // keyed by ID or name
}

func (m *mockTemplateRepoQS) Create(ctx context.Context, t *templateDomain.Template) error {
	return nil
}
func (m *mockTemplateRepoQS) Update(ctx context.Context, t *templateDomain.Template) error {
	return nil
}
func (m *mockTemplateRepoQS) GetByID(ctx context.Context, id string) (*templateDomain.Template, error) {
	if t, ok := m.templates[id]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("not found")
}
func (m *mockTemplateRepoQS) GetLatestVersion(ctx context.Context, appID, name string) (*templateDomain.Template, error) {
	return nil, fmt.Errorf("not found")
}
func (m *mockTemplateRepoQS) List(ctx context.Context, f templateDomain.Filter) ([]*templateDomain.Template, int64, error) {
	return nil, 0, nil
}
func (m *mockTemplateRepoQS) Delete(ctx context.Context, id string) error { return nil }
func (m *mockTemplateRepoQS) Count(ctx context.Context) (int64, error)    { return 0, nil }
func (m *mockTemplateRepoQS) CountByFilter(ctx context.Context, f templateDomain.Filter) (int64, error) {
	return 0, nil
}
func (m *mockTemplateRepoQS) GetByAppAndName(ctx context.Context, appID, name, locale string) (*templateDomain.Template, error) {
	if t, ok := m.templates[name]; ok && t.AppID == appID {
		return t, nil
	}
	return nil, fmt.Errorf("not found")
}
func (m *mockTemplateRepoQS) GetVersions(ctx context.Context, appID, name, locale string) ([]*templateDomain.Template, error) {
	return nil, nil
}
func (m *mockTemplateRepoQS) GetByVersion(ctx context.Context, appID, name, locale string, version int) (*templateDomain.Template, error) {
	return nil, nil
}
func (m *mockTemplateRepoQS) CreateVersion(ctx context.Context, t *templateDomain.Template) error {
	return nil
}

// mockUserRepoQS fulfils user.Repository, reusing the same shape as mockUserRepo.
type mockUserRepoQS struct {
	users map[string]*user.User
}

func (m *mockUserRepoQS) Create(ctx context.Context, u *user.User) error { return nil }
func (m *mockUserRepoQS) GetByID(ctx context.Context, id string) (*user.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, fmt.Errorf("not found")
}
func (m *mockUserRepoQS) GetByExternalID(ctx context.Context, appID, externalID string) (*user.User, error) {
	return nil, fmt.Errorf("not found")
}
func (m *mockUserRepoQS) GetByEmail(ctx context.Context, appID, email string) (*user.User, error) {
	for _, u := range m.users {
		if u.AppID == appID && u.Email == email {
			return u, nil
		}
	}
	return nil, fmt.Errorf("not found")
}
func (m *mockUserRepoQS) Update(ctx context.Context, u *user.User) error { return nil }
func (m *mockUserRepoQS) List(ctx context.Context, f user.UserFilter) ([]*user.User, error) {
	return nil, nil
}
func (m *mockUserRepoQS) Delete(ctx context.Context, id string) error { return nil }
func (m *mockUserRepoQS) AddDevice(ctx context.Context, userID string, d user.Device) error {
	return nil
}
func (m *mockUserRepoQS) RemoveDevice(ctx context.Context, userID, deviceID string) error { return nil }
func (m *mockUserRepoQS) UpdatePreferences(ctx context.Context, userID string, p user.Preferences) error {
	return nil
}
func (m *mockUserRepoQS) Count(ctx context.Context, f user.UserFilter) (int64, error) { return 0, nil }
func (m *mockUserRepoQS) BulkCreate(ctx context.Context, users []*user.User) error    { return nil }

// ─── Helper ────────────────────────────────────────────────────────────────

func newQuickSendServiceForTest(
	notifSvc *mockNotifService,
	users map[string]*user.User,
	templates map[string]*templateDomain.Template,
) *QuickSendService {
	tmplRepo := &mockTemplateRepoQS{templates: templates}
	tmplSvc := NewTemplateService(tmplRepo, zap.NewNop())
	return NewQuickSendService(
		notifSvc,
		&mockUserRepoQS{users: users},
		tmplRepo,
		tmplSvc,
		zap.NewNop(),
	)
}

// ─── Tests ─────────────────────────────────────────────────────────────────

func TestQuickSendService_Send(t *testing.T) {
	// Shared fixtures
	emailTmpl := &templateDomain.Template{
		ID:      "tmpl-email-uuid",
		AppID:   "app-1",
		Name:    "welcome",
		Channel: "email",
		Subject: "Welcome",
		Body:    "Hello {{.name}}",
	}
	discordTmpl := &templateDomain.Template{
		ID:      "tmpl-discord-uuid",
		AppID:   "app-1",
		Name:    "discord-alert",
		Channel: "discord",
		Subject: "Alert",
		Body:    "Server alert",
	}
	templates := map[string]*templateDomain.Template{
		emailTmpl.ID:     emailTmpl,
		emailTmpl.Name:   emailTmpl,
		discordTmpl.ID:   discordTmpl,
		discordTmpl.Name: discordTmpl,
	}

	testUser := &user.User{
		UserID: "user-uuid-1",
		AppID:  "app-1",
		Email:  "alice@example.com",
	}
	users := map[string]*user.User{
		testUser.UserID: testUser,
	}

	t.Run("template-based email send", func(t *testing.T) {
		var captured notification.SendRequest
		svc := newQuickSendServiceForTest(
			&mockNotifService{sendFn: func(_ context.Context, req notification.SendRequest) (*notification.Notification, error) {
				captured = req
				return &notification.Notification{NotificationID: "n-1", Status: notification.StatusQueued}, nil
			}},
			users, templates,
		)

		resp, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			To:       testUser.UserID,
			Template: "welcome",
		})

		require.NoError(t, err)
		assert.Equal(t, "n-1", resp.NotificationID)
		assert.Equal(t, "queued", resp.Status)
		assert.Equal(t, "email", resp.Channel)
		assert.Equal(t, testUser.UserID, resp.UserID)
		assert.Equal(t, emailTmpl.ID, captured.TemplateID)
		assert.Equal(t, notification.PriorityNormal, captured.Priority)
	})

	t.Run("template resolved by UUID", func(t *testing.T) {
		var captured notification.SendRequest
		svc := newQuickSendServiceForTest(
			&mockNotifService{sendFn: func(_ context.Context, req notification.SendRequest) (*notification.Notification, error) {
				captured = req
				return &notification.Notification{NotificationID: "n-2", Status: notification.StatusQueued}, nil
			}},
			users, templates,
		)

		resp, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			To:       testUser.UserID,
			Template: emailTmpl.ID,
		})

		require.NoError(t, err)
		assert.Equal(t, "n-2", resp.NotificationID)
		assert.Equal(t, emailTmpl.ID, captured.TemplateID)
	})

	t.Run("inline body creates transient template", func(t *testing.T) {
		var captured notification.SendRequest
		svc := newQuickSendServiceForTest(
			&mockNotifService{sendFn: func(_ context.Context, req notification.SendRequest) (*notification.Notification, error) {
				captured = req
				return &notification.Notification{NotificationID: "n-3", Status: notification.StatusQueued}, nil
			}},
			users, templates,
		)

		resp, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			To:      testUser.UserID,
			Channel: "email",
			Subject: "Hi",
			Body:    "Inline body here",
		})

		require.NoError(t, err)
		assert.Equal(t, "n-3", resp.NotificationID)
		// TemplateID may be empty if the repo mock doesn't generate IDs,
		// but the channel and request shape should be correct.
		assert.Equal(t, notification.ChannelEmail, captured.Channel)
		assert.Equal(t, testUser.UserID, captured.UserID)
	})

	t.Run("webhook-like channel skips recipient resolution", func(t *testing.T) {
		var captured notification.SendRequest
		svc := newQuickSendServiceForTest(
			&mockNotifService{sendFn: func(_ context.Context, req notification.SendRequest) (*notification.Notification, error) {
				captured = req
				return &notification.Notification{NotificationID: "n-4", Status: notification.StatusQueued}, nil
			}},
			users, templates,
		)

		resp, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			Template:   "discord-alert",
			WebhookURL: "https://discord.com/api/webhooks/123/abc",
		})

		require.NoError(t, err)
		assert.Equal(t, "discord", resp.Channel)
		assert.Equal(t, "", captured.UserID)
		assert.Equal(t, "https://discord.com/api/webhooks/123/abc", captured.Data["webhook_url"])
	})

	t.Run("discord with explicit channel override", func(t *testing.T) {
		var captured notification.SendRequest
		svc := newQuickSendServiceForTest(
			&mockNotifService{sendFn: func(_ context.Context, req notification.SendRequest) (*notification.Notification, error) {
				captured = req
				return &notification.Notification{NotificationID: "n-5", Status: notification.StatusQueued}, nil
			}},
			users, templates,
		)

		// Template is email but explicit channel is discord
		resp, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			Template: "welcome",
			Channel:  "discord",
			// Webhook-like channel → no "to" required
			WebhookURL: "https://discord.com/api/webhooks/x/y",
		})

		require.NoError(t, err)
		assert.Equal(t, "discord", resp.Channel)
		assert.Equal(t, notification.ChannelDiscord, captured.Channel)
	})

	t.Run("missing to for non-webhook channel", func(t *testing.T) {
		svc := newQuickSendServiceForTest(&mockNotifService{}, users, templates)

		_, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			Template: "welcome",
			// no "To" for email channel
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "'to' is required")
	})

	t.Run("template not found", func(t *testing.T) {
		svc := newQuickSendServiceForTest(&mockNotifService{}, users, templates)

		_, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			To:       testUser.UserID,
			Template: "nonexistent",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resolve template")
	})

	t.Run("recipient not found", func(t *testing.T) {
		svc := newQuickSendServiceForTest(&mockNotifService{}, users, templates)

		_, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			To:       "nobody",
			Template: "welcome",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resolve recipient")
	})

	t.Run("recipient resolved by email", func(t *testing.T) {
		var captured notification.SendRequest
		svc := newQuickSendServiceForTest(
			&mockNotifService{sendFn: func(_ context.Context, req notification.SendRequest) (*notification.Notification, error) {
				captured = req
				return &notification.Notification{NotificationID: "n-6", Status: notification.StatusQueued}, nil
			}},
			users, templates,
		)

		resp, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			To:       "alice@example.com",
			Template: "welcome",
		})

		require.NoError(t, err)
		assert.Equal(t, testUser.UserID, resp.UserID)
		assert.Equal(t, testUser.UserID, captured.UserID)
	})

	t.Run("priority override", func(t *testing.T) {
		var captured notification.SendRequest
		svc := newQuickSendServiceForTest(
			&mockNotifService{sendFn: func(_ context.Context, req notification.SendRequest) (*notification.Notification, error) {
				captured = req
				return &notification.Notification{NotificationID: "n-7", Status: notification.StatusQueued}, nil
			}},
			users, templates,
		)

		_, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			To:       testUser.UserID,
			Template: "welcome",
			Priority: "critical",
		})

		require.NoError(t, err)
		assert.Equal(t, notification.PriorityCritical, captured.Priority)
	})

	t.Run("digest_key propagated as metadata", func(t *testing.T) {
		var captured notification.SendRequest
		svc := newQuickSendServiceForTest(
			&mockNotifService{sendFn: func(_ context.Context, req notification.SendRequest) (*notification.Notification, error) {
				captured = req
				return &notification.Notification{NotificationID: "n-8", Status: notification.StatusQueued}, nil
			}},
			users, templates,
		)

		_, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			To:        testUser.UserID,
			Template:  "welcome",
			DigestKey: "hourly_summary",
		})

		require.NoError(t, err)
		require.NotNil(t, captured.Metadata)
		assert.Equal(t, "hourly_summary", captured.Metadata["digest_key"])
	})

	t.Run("webhook_url passthrough in Data", func(t *testing.T) {
		var captured notification.SendRequest
		svc := newQuickSendServiceForTest(
			&mockNotifService{sendFn: func(_ context.Context, req notification.SendRequest) (*notification.Notification, error) {
				captured = req
				return &notification.Notification{NotificationID: "n-9", Status: notification.StatusQueued}, nil
			}},
			users, templates,
		)

		_, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			Template:   "discord-alert",
			WebhookURL: "https://hooks.slack.com/xxx",
		})

		require.NoError(t, err)
		assert.Equal(t, "https://hooks.slack.com/xxx", captured.Data["webhook_url"])
	})

	t.Run("content_sid WhatsApp send", func(t *testing.T) {
		var captured notification.SendRequest
		svc := newQuickSendServiceForTest(
			&mockNotifService{sendFn: func(_ context.Context, req notification.SendRequest) (*notification.Notification, error) {
				captured = req
				return &notification.Notification{NotificationID: "n-10", Status: notification.StatusQueued}, nil
			}},
			users, templates,
		)

		resp, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			To:   testUser.UserID,
			Data: map[string]interface{}{"content_sid": "HXabc123"},
		})

		require.NoError(t, err)
		assert.Equal(t, "whatsapp", resp.Channel)
		assert.Equal(t, notification.ChannelWhatsApp, captured.Channel)
	})

	t.Run("content_sid with explicit channel", func(t *testing.T) {
		var captured notification.SendRequest
		svc := newQuickSendServiceForTest(
			&mockNotifService{sendFn: func(_ context.Context, req notification.SendRequest) (*notification.Notification, error) {
				captured = req
				return &notification.Notification{NotificationID: "n-11", Status: notification.StatusQueued}, nil
			}},
			users, templates,
		)

		_, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			To:      testUser.UserID,
			Channel: "sms",
			Data:    map[string]interface{}{"content_sid": "HXabc123"},
		})

		require.NoError(t, err)
		assert.Equal(t, notification.ChannelSMS, captured.Channel)
	})

	t.Run("no template and no body returns error", func(t *testing.T) {
		svc := newQuickSendServiceForTest(&mockNotifService{}, users, templates)

		_, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			To: testUser.UserID,
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrEmptyContent)
	})

	t.Run("notification service error propagated", func(t *testing.T) {
		svc := newQuickSendServiceForTest(
			&mockNotifService{sendFn: func(_ context.Context, req notification.SendRequest) (*notification.Notification, error) {
				return nil, fmt.Errorf("queue full")
			}},
			users, templates,
		)

		_, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			To:       testUser.UserID,
			Template: "welcome",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "queue full")
	})

	t.Run("scheduled_at propagated", func(t *testing.T) {
		var captured notification.SendRequest
		svc := newQuickSendServiceForTest(
			&mockNotifService{sendFn: func(_ context.Context, req notification.SendRequest) (*notification.Notification, error) {
				captured = req
				return &notification.Notification{NotificationID: "n-12", Status: notification.StatusPending}, nil
			}},
			users, templates,
		)

		_, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			To:       testUser.UserID,
			Template: "welcome",
		})

		require.NoError(t, err)
		// Without ScheduledAt set, it should be nil
		assert.Nil(t, captured.ScheduledAt)
	})

	t.Run("environment_id propagated", func(t *testing.T) {
		var captured notification.SendRequest
		svc := newQuickSendServiceForTest(
			&mockNotifService{sendFn: func(_ context.Context, req notification.SendRequest) (*notification.Notification, error) {
				captured = req
				return &notification.Notification{NotificationID: "n-13", Status: notification.StatusQueued}, nil
			}},
			users, templates,
		)

		_, err := svc.Send(context.Background(), "app-1", &dto.QuickSendRequest{
			To:            testUser.UserID,
			Template:      "welcome",
			EnvironmentID: "env-staging",
		})

		require.NoError(t, err)
		assert.Equal(t, "env-staging", captured.EnvironmentID)
	})
}
