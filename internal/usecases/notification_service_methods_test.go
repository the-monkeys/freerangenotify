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

// ─── Enhanced mocks for method-level testing ──────────────────────────────

// fullMockNotifRepo provides configurable behavior for all repo methods.
type fullMockNotifRepo struct {
	mockNotificationRepo
	notifications     map[string]*notification.Notification
	createErr         error
	updateStatusErr   error
	updateErr         error
	bulkUpdateErr     error
	bulkArchiveErr    error
	markAllReadErr    error
	markAllReadCount  int
	updateSnoozeErr   error
	incrementRetryErr error
	listResult        []*notification.Notification
	listErr           error
	countResult       int64
	countErr          error
	snoozedDueResult  []*notification.Notification

	// Track calls
	statusUpdates       map[string]notification.Status
	bulkStatusIDs       []string
	bulkStatusTarget    notification.Status
	bulkArchiveIDs      []string
	snoozeUpdates       map[string]notification.Status
	incrementedRetryIDs []string
}

func newFullMockNotifRepo() *fullMockNotifRepo {
	return &fullMockNotifRepo{
		notifications: make(map[string]*notification.Notification),
		statusUpdates: make(map[string]notification.Status),
		snoozeUpdates: make(map[string]notification.Status),
	}
}

func (m *fullMockNotifRepo) Create(ctx context.Context, n *notification.Notification) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = append(m.created, n)
	m.notifications[n.NotificationID] = n
	return nil
}

func (m *fullMockNotifRepo) GetByID(ctx context.Context, id string) (*notification.Notification, error) {
	if n, ok := m.notifications[id]; ok {
		return n, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *fullMockNotifRepo) UpdateStatus(ctx context.Context, id string, status notification.Status) error {
	if m.updateStatusErr != nil {
		return m.updateStatusErr
	}
	m.statusUpdates[id] = status
	return nil
}

func (m *fullMockNotifRepo) Update(ctx context.Context, n *notification.Notification) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.notifications[n.NotificationID] = n
	return nil
}

func (m *fullMockNotifRepo) BulkUpdateStatus(ctx context.Context, ids []string, status notification.Status) error {
	if m.bulkUpdateErr != nil {
		return m.bulkUpdateErr
	}
	m.bulkStatusIDs = ids
	m.bulkStatusTarget = status
	return nil
}

func (m *fullMockNotifRepo) BulkArchive(ctx context.Context, ids []string, at time.Time) error {
	if m.bulkArchiveErr != nil {
		return m.bulkArchiveErr
	}
	m.bulkArchiveIDs = ids
	return nil
}

func (m *fullMockNotifRepo) MarkAllRead(ctx context.Context, userID, appID, category string) (int, error) {
	if m.markAllReadErr != nil {
		return 0, m.markAllReadErr
	}
	return m.markAllReadCount, nil
}

func (m *fullMockNotifRepo) UpdateSnooze(ctx context.Context, id string, status notification.Status, t *time.Time) error {
	if m.updateSnoozeErr != nil {
		return m.updateSnoozeErr
	}
	m.snoozeUpdates[id] = status
	return nil
}

func (m *fullMockNotifRepo) IncrementRetryCount(ctx context.Context, id, errorMessage string) error {
	if m.incrementRetryErr != nil {
		return m.incrementRetryErr
	}
	m.incrementedRetryIDs = append(m.incrementedRetryIDs, id)
	return nil
}

func (m *fullMockNotifRepo) List(ctx context.Context, f *notification.NotificationFilter) ([]*notification.Notification, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.listResult, nil
}

func (m *fullMockNotifRepo) Count(ctx context.Context, f *notification.NotificationFilter) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.countResult, nil
}

func (m *fullMockNotifRepo) ListSnoozedDue(ctx context.Context, now time.Time) ([]*notification.Notification, error) {
	return m.snoozedDueResult, nil
}

// fullMockQueue tracks all queue operations.
type fullMockQueue struct {
	mockQueue
	enqueueErr         error
	enqueuePriorityErr error
	enqueuedItems      []notifqueue.NotificationQueueItem
	priorityItems      []notifqueue.NotificationQueueItem
	removedIDs         []string
}

func (m *fullMockQueue) Enqueue(ctx context.Context, item notifqueue.NotificationQueueItem) error {
	if m.enqueueErr != nil {
		return m.enqueueErr
	}
	m.enqueuedItems = append(m.enqueuedItems, item)
	return nil
}

func (m *fullMockQueue) EnqueuePriority(ctx context.Context, item notifqueue.NotificationQueueItem) error {
	if m.enqueuePriorityErr != nil {
		return m.enqueuePriorityErr
	}
	m.priorityItems = append(m.priorityItems, item)
	return nil
}

func (m *fullMockQueue) RemoveScheduledByID(ctx context.Context, ids []string) error {
	m.removedIDs = append(m.removedIDs, ids...)
	return nil
}

// newMethodTestService creates a service suitable for method-level tests
// that don't go through the full Send() path.
func newMethodTestService(
	nrepo *fullMockNotifRepo,
	q *fullMockQueue,
) *NotificationService {
	if nrepo == nil {
		nrepo = newFullMockNotifRepo()
	}
	if q == nil {
		q = &fullMockQueue{}
	}
	return NewNotificationService(
		nrepo,
		&mockUserRepo{users: map[string]*user.User{}},
		&mockAppRepoSend{apps: defaultApps()},
		&mockTemplateRepoNSC{},
		q,
		zap.NewNop(),
		NotificationServiceConfig{MaxRetries: 3},
		nil,
		&mockLimiterSend{allowed: true},
	).(*NotificationService)
}

// helper to seed a notification in the mock repo
func seedNotification(repo *fullMockNotifRepo, id, appID, userID string, status notification.Status) *notification.Notification {
	n := &notification.Notification{
		NotificationID: id,
		AppID:          appID,
		UserID:         userID,
		Channel:        notification.ChannelEmail,
		Priority:       notification.PriorityNormal,
		Status:         status,
		Content:        notification.Content{Title: "Test", Body: "Body"},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	repo.notifications[id] = n
	return n
}

// ─── Get ──────────────────────────────────────────────────────────────────

func TestNotificationService_Get(t *testing.T) {
	t.Run("found and app matches", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusQueued)
		svc := newMethodTestService(repo, nil)

		n, err := svc.Get(context.Background(), "n1", "app-1")
		require.NoError(t, err)
		assert.Equal(t, "n1", n.NotificationID)
	})

	t.Run("not found returns error", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)

		_, err := svc.Get(context.Background(), "missing", "app-1")
		require.Error(t, err)
	})

	t.Run("wrong app returns error", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusQueued)
		svc := newMethodTestService(repo, nil)

		_, err := svc.Get(context.Background(), "n1", "app-OTHER")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "notification not found")
	})
}

// ─── List ─────────────────────────────────────────────────────────────────

func TestNotificationService_List(t *testing.T) {
	t.Run("returns repo results", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		repo.listResult = []*notification.Notification{
			{NotificationID: "n1"},
			{NotificationID: "n2"},
		}
		svc := newMethodTestService(repo, nil)

		results, err := svc.List(context.Background(), notification.NotificationFilter{
			AppID: "app-1", Page: 1, PageSize: 50,
		})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("invalid filter returns error", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)

		_, err := svc.List(context.Background(), notification.NotificationFilter{
			Channel: notification.Channel("bogus"),
		})
		require.Error(t, err)
	})
}

// ─── Count ────────────────────────────────────────────────────────────────

func TestNotificationService_Count(t *testing.T) {
	t.Run("returns repo count", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		repo.countResult = 42
		svc := newMethodTestService(repo, nil)

		count, err := svc.Count(context.Background(), notification.NotificationFilter{
			AppID: "app-1", Page: 1, PageSize: 50,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(42), count)
	})

	t.Run("invalid filter returns error", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)

		_, err := svc.Count(context.Background(), notification.NotificationFilter{
			Channel: notification.Channel("bogus"),
		})
		require.Error(t, err)
	})
}

// ─── UpdateStatus ─────────────────────────────────────────────────────────

func TestNotificationService_UpdateStatus(t *testing.T) {
	t.Run("successful status update", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusQueued)
		svc := newMethodTestService(repo, nil)

		err := svc.UpdateStatus(context.Background(), "n1", notification.StatusProcessing, "", "app-1")
		require.NoError(t, err)
		assert.Equal(t, notification.StatusProcessing, repo.statusUpdates["n1"])
	})

	t.Run("status update with error message", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusQueued)
		svc := newMethodTestService(repo, nil)

		err := svc.UpdateStatus(context.Background(), "n1", notification.StatusFailed, "provider timeout", "app-1")
		require.NoError(t, err)
		assert.Equal(t, "provider timeout", repo.notifications["n1"].ErrorMessage)
	})

	t.Run("invalid status returns error", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)

		err := svc.UpdateStatus(context.Background(), "n1", notification.Status("bogus"), "", "app-1")
		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrInvalidStatus)
	})

	t.Run("notification not found returns error", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)

		err := svc.UpdateStatus(context.Background(), "missing", notification.StatusSent, "", "app-1")
		require.Error(t, err)
	})

	t.Run("wrong app returns error", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusQueued)
		svc := newMethodTestService(repo, nil)

		err := svc.UpdateStatus(context.Background(), "n1", notification.StatusSent, "", "app-OTHER")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "notification not found")
	})

	t.Run("final state cannot be updated", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusDelivered)
		svc := newMethodTestService(repo, nil)

		err := svc.UpdateStatus(context.Background(), "n1", notification.StatusRead, "", "app-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "final state")
	})
}

// ─── Cancel ───────────────────────────────────────────────────────────────

func TestNotificationService_Cancel(t *testing.T) {
	t.Run("successful cancel", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		q := &fullMockQueue{}
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusQueued)
		svc := newMethodTestService(repo, q)

		err := svc.Cancel(context.Background(), "n1", "app-1")
		require.NoError(t, err)
		assert.Equal(t, notification.StatusCancelled, repo.statusUpdates["n1"])
		assert.Contains(t, q.removedIDs, "n1")
	})

	t.Run("not found returns error", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)

		err := svc.Cancel(context.Background(), "missing", "app-1")
		require.Error(t, err)
	})

	t.Run("wrong app returns error", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusQueued)
		svc := newMethodTestService(repo, nil)

		err := svc.Cancel(context.Background(), "n1", "app-OTHER")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "notification not found")
	})

	t.Run("sent notification cannot be cancelled", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusSent)
		svc := newMethodTestService(repo, nil)

		err := svc.Cancel(context.Background(), "n1", "app-1")
		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrCannotCancelSent)
	})

	t.Run("delivered notification cannot be cancelled", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusDelivered)
		svc := newMethodTestService(repo, nil)

		err := svc.Cancel(context.Background(), "n1", "app-1")
		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrCannotCancelSent)
	})

	t.Run("already failed notification cannot be cancelled", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusFailed)
		svc := newMethodTestService(repo, nil)

		err := svc.Cancel(context.Background(), "n1", "app-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "final state")
	})
}

// ─── CancelBatch ──────────────────────────────────────────────────────────

func TestNotificationService_CancelBatch(t *testing.T) {
	t.Run("cancels eligible notifications", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		q := &fullMockQueue{}
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusQueued)
		seedNotification(repo, "n2", "app-1", "user-1", notification.StatusPending)
		svc := newMethodTestService(repo, q)

		err := svc.CancelBatch(context.Background(), []string{"n1", "n2"}, "app-1")
		require.NoError(t, err)
		assert.Len(t, repo.bulkStatusIDs, 2)
		assert.Equal(t, notification.StatusCancelled, repo.bulkStatusTarget)
		assert.Len(t, q.removedIDs, 2)
	})

	t.Run("skips wrong app", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusQueued)
		seedNotification(repo, "n2", "app-OTHER", "user-1", notification.StatusQueued)
		svc := newMethodTestService(repo, nil)

		err := svc.CancelBatch(context.Background(), []string{"n1", "n2"}, "app-1")
		require.NoError(t, err)
		assert.Len(t, repo.bulkStatusIDs, 1)
		assert.Equal(t, "n1", repo.bulkStatusIDs[0])
	})

	t.Run("skips final-state notifications", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusQueued)
		seedNotification(repo, "n2", "app-1", "user-1", notification.StatusDelivered)
		seedNotification(repo, "n3", "app-1", "user-1", notification.StatusSent)
		svc := newMethodTestService(repo, nil)

		err := svc.CancelBatch(context.Background(), []string{"n1", "n2", "n3"}, "app-1")
		require.NoError(t, err)
		assert.Len(t, repo.bulkStatusIDs, 1)
		assert.Equal(t, "n1", repo.bulkStatusIDs[0])
	})

	t.Run("skips not found", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusQueued)
		svc := newMethodTestService(repo, nil)

		err := svc.CancelBatch(context.Background(), []string{"n1", "missing"}, "app-1")
		require.NoError(t, err)
		assert.Len(t, repo.bulkStatusIDs, 1)
	})

	t.Run("all invalid returns nil", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)

		err := svc.CancelBatch(context.Background(), []string{"missing1", "missing2"}, "app-1")
		require.NoError(t, err)
	})
}

// ─── Retry ────────────────────────────────────────────────────────────────

func TestNotificationService_Retry(t *testing.T) {
	t.Run("successful retry", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		q := &fullMockQueue{}
		n := seedNotification(repo, "n1", "app-1", "user-1", notification.StatusFailed)
		n.RetryCount = 0
		svc := newMethodTestService(repo, q)

		err := svc.Retry(context.Background(), "n1", "app-1")
		require.NoError(t, err)
		assert.Contains(t, repo.incrementedRetryIDs, "n1")
		assert.Len(t, q.enqueuedItems, 1)
		assert.Equal(t, "n1", q.enqueuedItems[0].NotificationID)
		assert.Equal(t, notification.StatusQueued, repo.statusUpdates["n1"])
	})

	t.Run("not found returns error", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)

		err := svc.Retry(context.Background(), "missing", "app-1")
		require.Error(t, err)
	})

	t.Run("wrong app returns error", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusFailed)
		svc := newMethodTestService(repo, nil)

		err := svc.Retry(context.Background(), "n1", "app-OTHER")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "notification not found")
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		n := seedNotification(repo, "n1", "app-1", "user-1", notification.StatusFailed)
		n.RetryCount = 3 // equals maxRetries
		svc := newMethodTestService(repo, nil)

		err := svc.Retry(context.Background(), "n1", "app-1")
		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrMaxRetriesExceeded)
	})

	t.Run("non-failed notification cannot be retried", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusQueued)
		svc := newMethodTestService(repo, nil)

		err := svc.Retry(context.Background(), "n1", "app-1")
		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrCannotRetry)
	})

	t.Run("app retry limit overrides default", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		n := seedNotification(repo, "n1", "app-1", "user-1", notification.StatusFailed)
		n.RetryCount = 1
		q := &fullMockQueue{}

		// App has RetryAttempts=5, so retryCount=1 < 5 should succeed
		apps := map[string]*application.Application{
			"app-1": {AppID: "app-1", Settings: application.Settings{RetryAttempts: 5}},
		}
		svc := NewNotificationService(
			repo,
			&mockUserRepo{users: map[string]*user.User{}},
			&mockAppRepoSend{apps: apps},
			&mockTemplateRepoNSC{},
			q,
			zap.NewNop(),
			NotificationServiceConfig{MaxRetries: 1}, // default would block at retryCount=1
			nil,
			&mockLimiterSend{allowed: true},
		).(*NotificationService)

		err := svc.Retry(context.Background(), "n1", "app-1")
		require.NoError(t, err)
	})
}

// ─── FlushQueued ──────────────────────────────────────────────────────────

func TestNotificationService_FlushQueued(t *testing.T) {
	t.Run("flushes queued notifications", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		q := &fullMockQueue{}
		repo.listResult = []*notification.Notification{
			{NotificationID: "n1", AppID: "app-1", Priority: notification.PriorityNormal},
			{NotificationID: "n2", AppID: "app-1", Priority: notification.PriorityHigh},
		}
		svc := newMethodTestService(repo, q)

		err := svc.FlushQueued(context.Background(), "user-1")
		require.NoError(t, err)
		assert.Len(t, q.priorityItems, 2)
		assert.Equal(t, "n1", q.priorityItems[0].NotificationID)
		assert.Equal(t, "n2", q.priorityItems[1].NotificationID)
	})

	t.Run("no queued notifications is no-op", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		repo.listResult = nil
		q := &fullMockQueue{}
		svc := newMethodTestService(repo, q)

		err := svc.FlushQueued(context.Background(), "user-1")
		require.NoError(t, err)
		assert.Len(t, q.priorityItems, 0)
	})

	t.Run("list error returns error", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		repo.listErr = fmt.Errorf("es down")
		svc := newMethodTestService(repo, nil)

		err := svc.FlushQueued(context.Background(), "user-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to list queued notifications")
	})
}

// ─── GetUnreadCount ───────────────────────────────────────────────────────

func TestNotificationService_GetUnreadCount(t *testing.T) {
	t.Run("returns count from repo", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		repo.countResult = 7
		svc := newMethodTestService(repo, nil)

		count, err := svc.GetUnreadCount(context.Background(), "user-1", "app-1")
		require.NoError(t, err)
		assert.Equal(t, int64(7), count)
	})

	t.Run("repo error propagates", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		repo.countErr = fmt.Errorf("es down")
		svc := newMethodTestService(repo, nil)

		_, err := svc.GetUnreadCount(context.Background(), "user-1", "app-1")
		require.Error(t, err)
	})
}

// ─── MarkRead ─────────────────────────────────────────────────────────────

func TestNotificationService_MarkRead(t *testing.T) {
	t.Run("marks owned notifications as read", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusSent)
		seedNotification(repo, "n2", "app-1", "user-1", notification.StatusSent)
		svc := newMethodTestService(repo, nil)

		err := svc.MarkRead(context.Background(), []string{"n1", "n2"}, "app-1", "user-1")
		require.NoError(t, err)
		assert.Equal(t, notification.StatusRead, repo.bulkStatusTarget)
		assert.Len(t, repo.bulkStatusIDs, 2)
	})

	t.Run("wrong user returns error", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusSent)
		svc := newMethodTestService(repo, nil)

		err := svc.MarkRead(context.Background(), []string{"n1"}, "app-1", "user-OTHER")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not belong to user")
	})

	t.Run("wrong app returns error", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusSent)
		svc := newMethodTestService(repo, nil)

		err := svc.MarkRead(context.Background(), []string{"n1"}, "app-OTHER", "user-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not belong to user")
	})

	t.Run("not found notification is skipped", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusSent)
		svc := newMethodTestService(repo, nil)

		// "missing" is not found → skipped, "n1" passes ownership check
		err := svc.MarkRead(context.Background(), []string{"missing", "n1"}, "app-1", "user-1")
		require.NoError(t, err)
	})
}

// ─── ListUnread ───────────────────────────────────────────────────────────

func TestNotificationService_ListUnread(t *testing.T) {
	t.Run("returns unread notifications", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		repo.listResult = []*notification.Notification{
			{NotificationID: "n1", Status: notification.StatusSent},
		}
		svc := newMethodTestService(repo, nil)

		results, err := svc.ListUnread(context.Background(), "user-1", "app-1")
		require.NoError(t, err)
		assert.Len(t, results, 1)
	})
}

// ─── Snooze ───────────────────────────────────────────────────────────────

func TestNotificationService_Snooze(t *testing.T) {
	t.Run("successful snooze", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		q := &fullMockQueue{}
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusQueued)
		svc := newMethodTestService(repo, q)

		until := time.Now().Add(1 * time.Hour)
		err := svc.Snooze(context.Background(), "n1", "app-1", until)
		require.NoError(t, err)
		assert.Equal(t, notification.StatusSnoozed, repo.snoozeUpdates["n1"])
		assert.Contains(t, q.removedIDs, "n1")
	})

	t.Run("not found returns error", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)

		err := svc.Snooze(context.Background(), "missing", "app-1", time.Now().Add(time.Hour))
		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrNotificationNotFound)
	})

	t.Run("wrong app returns not found", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusQueued)
		svc := newMethodTestService(repo, nil)

		err := svc.Snooze(context.Background(), "n1", "app-OTHER", time.Now().Add(time.Hour))
		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrNotificationNotFound)
	})

	t.Run("already snoozed returns error", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusSnoozed)
		svc := newMethodTestService(repo, nil)

		err := svc.Snooze(context.Background(), "n1", "app-1", time.Now().Add(time.Hour))
		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrCannotSnooze)
	})

	t.Run("final state cannot be snoozed", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusDelivered)
		svc := newMethodTestService(repo, nil)

		err := svc.Snooze(context.Background(), "n1", "app-1", time.Now().Add(time.Hour))
		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrCannotSnooze)
	})

	t.Run("past time returns error", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusQueued)
		svc := newMethodTestService(repo, nil)

		err := svc.Snooze(context.Background(), "n1", "app-1", time.Now().Add(-1*time.Hour))
		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrInvalidSnoozeDuration)
	})
}

// ─── Unsnooze ─────────────────────────────────────────────────────────────

func TestNotificationService_Unsnooze(t *testing.T) {
	t.Run("successful unsnooze", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		q := &fullMockQueue{}
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusSnoozed)
		svc := newMethodTestService(repo, q)

		err := svc.Unsnooze(context.Background(), "n1", "app-1")
		require.NoError(t, err)
		assert.Equal(t, notification.StatusQueued, repo.snoozeUpdates["n1"])
		assert.Len(t, q.enqueuedItems, 1)
		assert.Equal(t, "n1", q.enqueuedItems[0].NotificationID)
	})

	t.Run("not found returns error", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)

		err := svc.Unsnooze(context.Background(), "missing", "app-1")
		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrNotificationNotFound)
	})

	t.Run("wrong app returns not found", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusSnoozed)
		svc := newMethodTestService(repo, nil)

		err := svc.Unsnooze(context.Background(), "n1", "app-OTHER")
		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrNotificationNotFound)
	})

	t.Run("non-snoozed notification returns error", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusQueued)
		svc := newMethodTestService(repo, nil)

		err := svc.Unsnooze(context.Background(), "n1", "app-1")
		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrCannotSnooze)
	})
}

// ─── Archive ──────────────────────────────────────────────────────────────

func TestNotificationService_Archive(t *testing.T) {
	t.Run("archives owned notifications", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusSent)
		seedNotification(repo, "n2", "app-1", "user-1", notification.StatusRead)
		svc := newMethodTestService(repo, nil)

		err := svc.Archive(context.Background(), []string{"n1", "n2"}, "app-1", "user-1")
		require.NoError(t, err)
		assert.Len(t, repo.bulkArchiveIDs, 2)
	})

	t.Run("not found returns error", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)

		err := svc.Archive(context.Background(), []string{"missing"}, "app-1", "user-1")
		require.Error(t, err)
		assert.ErrorIs(t, err, notification.ErrNotificationNotFound)
	})

	t.Run("wrong user returns error", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusSent)
		svc := newMethodTestService(repo, nil)

		err := svc.Archive(context.Background(), []string{"n1"}, "app-1", "user-OTHER")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not belong to user")
	})

	t.Run("wrong app returns error", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusSent)
		svc := newMethodTestService(repo, nil)

		err := svc.Archive(context.Background(), []string{"n1"}, "app-OTHER", "user-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not belong to user")
	})

	t.Run("already archived is allowed", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		seedNotification(repo, "n1", "app-1", "user-1", notification.StatusArchived)
		svc := newMethodTestService(repo, nil)

		err := svc.Archive(context.Background(), []string{"n1"}, "app-1", "user-1")
		require.NoError(t, err)
	})
}

// ─── MarkAllRead ──────────────────────────────────────────────────────────

func TestNotificationService_MarkAllRead(t *testing.T) {
	t.Run("successful mark all read", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		repo.markAllReadCount = 5
		svc := newMethodTestService(repo, nil)

		err := svc.MarkAllRead(context.Background(), "user-1", "app-1", "")
		require.NoError(t, err)
	})

	t.Run("repo error propagates", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		repo.markAllReadErr = fmt.Errorf("es down")
		svc := newMethodTestService(repo, nil)

		err := svc.MarkAllRead(context.Background(), "user-1", "app-1", "")
		require.Error(t, err)
	})
}

// ─── ListSnoozedDue ──────────────────────────────────────────────────────

func TestNotificationService_ListSnoozedDue(t *testing.T) {
	t.Run("returns snoozed due from repo", func(t *testing.T) {
		repo := newFullMockNotifRepo()
		repo.snoozedDueResult = []*notification.Notification{
			{NotificationID: "n1", Status: notification.StatusSnoozed},
		}
		svc := newMethodTestService(repo, nil)

		results, err := svc.ListSnoozedDue(context.Background())
		require.NoError(t, err)
		assert.Len(t, results, 1)
	})

	t.Run("returns empty when none due", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)

		results, err := svc.ListSnoozedDue(context.Background())
		require.NoError(t, err)
		assert.Len(t, results, 0)
	})
}

// ─── SendBatch ────────────────────────────────────────────────────────────

func TestNotificationService_SendBatch(t *testing.T) {
	t.Run("processes multiple requests", func(t *testing.T) {
		nrepo := &mockNotifRepoSend{}
		q := &mockQueueSend{}
		svc := newSendServiceForTest(defaultUsers(), defaultApps(), nrepo, q, nil)

		results, err := svc.SendBatch(context.Background(), []notification.SendRequest{
			{
				AppID: "app-1", UserID: "user-1",
				Channel: notification.ChannelEmail, Priority: notification.PriorityNormal,
				Title: "First", Body: "Body1",
			},
			{
				AppID: "app-1", UserID: "user-1",
				Channel: notification.ChannelEmail, Priority: notification.PriorityNormal,
				Title: "Second", Body: "Body2",
			},
		})

		require.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, notification.StatusQueued, results[0].Status)
		assert.Equal(t, notification.StatusQueued, results[1].Status)
	})

	t.Run("failed request returns failed notification item", func(t *testing.T) {
		svc := newSendServiceForTest(defaultUsers(), defaultApps(), nil, nil, nil)

		results, err := svc.SendBatch(context.Background(), []notification.SendRequest{
			{
				AppID: "app-1", UserID: "user-1",
				Channel: notification.ChannelEmail, Priority: notification.PriorityNormal,
				Title: "OK", Body: "Body",
			},
			{
				AppID: "", // invalid
			},
		})

		require.NoError(t, err) // SendBatch itself never errors
		assert.Len(t, results, 2)
		assert.Equal(t, notification.StatusQueued, results[0].Status)
		assert.Equal(t, notification.StatusFailed, results[1].Status)
		assert.NotEmpty(t, results[1].ErrorMessage)
	})
}

// ─── isChannelEnabled ─────────────────────────────────────────────────────

func TestNotificationService_isChannelEnabled(t *testing.T) {
	t.Run("email enabled by user preference", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)
		u := &user.User{
			AppID:       "app-1",
			Preferences: user.Preferences{EmailEnabled: boolPtr(true)},
		}

		assert.True(t, svc.isChannelEnabled(context.Background(), u, notification.ChannelEmail, ""))
	})

	t.Run("email disabled by user preference", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)
		u := &user.User{
			AppID:       "app-1",
			Preferences: user.Preferences{EmailEnabled: boolPtr(false)},
		}

		assert.False(t, svc.isChannelEnabled(context.Background(), u, notification.ChannelEmail, ""))
	})

	t.Run("defaults to enabled when no preference set", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)
		u := &user.User{
			AppID:       "app-1",
			Preferences: user.Preferences{},
		}

		assert.True(t, svc.isChannelEnabled(context.Background(), u, notification.ChannelPush, ""))
	})

	t.Run("category disabled blocks notification", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)
		u := &user.User{
			AppID: "app-1",
			Preferences: user.Preferences{
				EmailEnabled: boolPtr(true),
				Categories: map[string]user.CategoryPreference{
					"marketing": {Enabled: false},
				},
			},
		}

		assert.False(t, svc.isChannelEnabled(context.Background(), u, notification.ChannelEmail, "marketing"))
	})

	t.Run("category enabled channels override user preferences", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)
		u := &user.User{
			AppID: "app-1",
			Preferences: user.Preferences{
				EmailEnabled: boolPtr(true),
				SMSEnabled:   boolPtr(true),
				Categories: map[string]user.CategoryPreference{
					"alerts": {Enabled: true, EnabledChannels: []string{"sms"}},
				},
			},
		}

		// Email is NOT in category's enabled channels
		assert.False(t, svc.isChannelEnabled(context.Background(), u, notification.ChannelEmail, "alerts"))
		// SMS IS in category's enabled channels
		assert.True(t, svc.isChannelEnabled(context.Background(), u, notification.ChannelSMS, "alerts"))
	})

	t.Run("whatsapp preference respected", func(t *testing.T) {
		svc := newMethodTestService(nil, nil)
		u := &user.User{
			AppID:       "app-1",
			Preferences: user.Preferences{WhatsAppEnabled: boolPtr(false)},
		}

		assert.False(t, svc.isChannelEnabled(context.Background(), u, notification.ChannelWhatsApp, ""))
	})
}
