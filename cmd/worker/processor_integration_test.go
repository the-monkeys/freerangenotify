//go:build integration

package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"go.uber.org/zap"
)

// This test simulates downtime: a scheduled notification is due while the worker is "down".
// When the scheduler starts (after the scheduled time), it should enqueue the item and
// set its status to queued.
func TestIntegration_ScheduledNotificationCatchup(t *testing.T) {
	logger := zap.NewNop()
	now := time.Now().Add(-5 * time.Minute) // simulate it was due 5 minutes ago

	fq := &fakeQueue{
		scheduled: []queue.NotificationQueueItem{
			{NotificationID: "n1", AppID: "app1", Priority: notification.PriorityNormal, EnqueuedAt: now},
		},
	}
	frepo := &fakeNotificationRepo{
		items: map[string]*notification.Notification{
			"n1": {
				NotificationID: "n1",
				AppID:          "app1",
				Status:         notification.StatusPending,
				ScheduledAt:    &now,
				Priority:       notification.PriorityNormal,
			},
		},
	}

	processor := &NotificationProcessor{
		queue:            fq,
		notifRepo:        frepo,
		userRepo:         nil,
		appRepo:          nil,
		templateRepo:     nil,
		authService:      nil,
		licensingChecker: nil,
		providerManager:  nil,
		redisClient:      nil,
		logger:           logger,
		config: ProcessorConfig{
			PollInterval:   10 * time.Millisecond,
			ScheduledBatch: 10,
			LateThreshold:  1 * time.Second,
		},
		metrics:  nil,
		stopChan: make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		processor.scheduler(ctx)
	}()

	// Let the scheduler tick at least once
	time.Sleep(50 * time.Millisecond)
	cancel()
	close(processor.stopChan)
	wg.Wait()

	require.True(t, fq.enqueuedBatch, "scheduled item should be enqueued")
	require.Equal(t, notification.StatusQueued, frepo.updatedStatus["n1"])
}

// --- fakes for scheduler path only ---

type fakeQueue struct {
	scheduled     []queue.NotificationQueueItem
	enqueuedBatch bool
}

func (f *fakeQueue) Enqueue(ctx context.Context, item queue.NotificationQueueItem) error {
	return nil
}
func (f *fakeQueue) EnqueuePriority(ctx context.Context, item queue.NotificationQueueItem) error {
	return nil
}
func (f *fakeQueue) Dequeue(ctx context.Context) (*queue.NotificationQueueItem, error) {
	return nil, nil
}
func (f *fakeQueue) EnqueueBatch(ctx context.Context, items []queue.NotificationQueueItem) error {
	if len(items) > 0 {
		f.enqueuedBatch = true
	}
	return nil
}
func (f *fakeQueue) GetQueueDepth(ctx context.Context) (map[string]int64, error)     { return nil, nil }
func (f *fakeQueue) Peek(ctx context.Context) (*queue.NotificationQueueItem, error)  { return nil, nil }
func (f *fakeQueue) ListDLQ(ctx context.Context, limit int) ([]queue.DLQItem, error) { return nil, nil }
func (f *fakeQueue) ReplayDLQ(ctx context.Context, limit int) (int, error)           { return 0, nil }
func (f *fakeQueue) ReplayDLQForApps(ctx context.Context, limit int, allowedApps map[string]bool) (int, error) {
	return 0, nil
}
func (f *fakeQueue) EnqueueScheduled(ctx context.Context, item queue.NotificationQueueItem, scheduledAt time.Time) error {
	return nil
}
func (f *fakeQueue) GetScheduledItems(ctx context.Context, limit int64) ([]queue.NotificationQueueItem, error) {
	items := f.scheduled
	f.scheduled = nil
	return items, nil
}
func (f *fakeQueue) RemoveScheduledByID(ctx context.Context, notificationIDs []string) error {
	return nil
}
func (f *fakeQueue) Acknowledge(ctx context.Context, item queue.NotificationQueueItem) error {
	return nil
}
func (f *fakeQueue) RequeueExpiredProcessing(ctx context.Context) (int, error) { return 0, nil }
func (f *fakeQueue) Close() error                                              { return nil }

type fakeNotificationRepo struct {
	items         map[string]*notification.Notification
	updatedStatus map[string]notification.Status
}

func (f *fakeNotificationRepo) Create(ctx context.Context, n *notification.Notification) error {
	return nil
}
func (f *fakeNotificationRepo) GetByID(ctx context.Context, notificationID string) (*notification.Notification, error) {
	return f.items[notificationID], nil
}
func (f *fakeNotificationRepo) Update(ctx context.Context, n *notification.Notification) error {
	return nil
}
func (f *fakeNotificationRepo) UpdateStatus(ctx context.Context, notificationID string, status notification.Status) error {
	if f.updatedStatus == nil {
		f.updatedStatus = make(map[string]notification.Status)
	}
	f.updatedStatus[notificationID] = status
	return nil
}
func (f *fakeNotificationRepo) Delete(ctx context.Context, notificationID string) error { return nil }
func (f *fakeNotificationRepo) List(ctx context.Context, filter *notification.NotificationFilter) ([]*notification.Notification, error) {
	return nil, nil
}
func (f *fakeNotificationRepo) GetByUser(ctx context.Context, userID string, filter *notification.NotificationFilter) ([]*notification.Notification, error) {
	return nil, nil
}
func (f *fakeNotificationRepo) GetByApp(ctx context.Context, appID string, filter *notification.NotificationFilter) ([]*notification.Notification, error) {
	return nil, nil
}
func (f *fakeNotificationRepo) GetPending(ctx context.Context) ([]*notification.Notification, error) {
	return nil, nil
}
func (f *fakeNotificationRepo) GetRetryable(ctx context.Context, limit int) ([]*notification.Notification, error) {
	return nil, nil
}
func (f *fakeNotificationRepo) IncrementRetryCount(ctx context.Context, notificationID string, reason string) error {
	return nil
}
func (f *fakeNotificationRepo) BulkUpdateStatus(ctx context.Context, ids []string, status notification.Status) error {
	if f.updatedStatus == nil {
		f.updatedStatus = make(map[string]notification.Status)
	}
	for _, id := range ids {
		f.updatedStatus[id] = status
	}
	return nil
}
func (f *fakeNotificationRepo) BulkArchive(ctx context.Context, ids []string, archivedAt time.Time) error {
	return nil
}
func (f *fakeNotificationRepo) MarkAllRead(ctx context.Context, userID, appID, category string) (int, error) {
	return 0, nil
}
func (f *fakeNotificationRepo) BulkUpdate(ctx context.Context, notifications []*notification.Notification) error {
	return nil
}
func (f *fakeNotificationRepo) ListSnoozedDue(ctx context.Context, now time.Time) ([]*notification.Notification, error) {
	return nil, nil
}
func (f *fakeNotificationRepo) UpdateSnooze(ctx context.Context, notificationID string, status notification.Status, until *time.Time) error {
	return nil
}
func (f *fakeNotificationRepo) BulkArchiveByUser(ctx context.Context, userID, appID string, archivedAt time.Time) (int, error) {
	return 0, nil
}
func (f *fakeNotificationRepo) Count(ctx context.Context, filter *notification.NotificationFilter) (int64, error) {
	return 0, nil
}
func (f *fakeNotificationRepo) IncrementRetryCountWithReason(ctx context.Context, notificationID string, reason string) error {
	return nil
}
func (f *fakeNotificationRepo) UpdateStatusWithError(ctx context.Context, notificationID string, status notification.Status, errorMessage string) error {
	return nil
}
func (f *fakeNotificationRepo) BulkUpdateStatusByApp(ctx context.Context, appID string, status notification.Status, reason string, limit int) (int, error) {
	return 0, nil
}
func (f *fakeNotificationRepo) BulkUpdateStatusByUser(ctx context.Context, userID, appID string, status notification.Status) (int, error) {
	return 0, nil
}
func (f *fakeNotificationRepo) BulkUpdateStatusByAppAndCategory(ctx context.Context, appID, category string, status notification.Status) (int, error) {
	return 0, nil
}
func (f *fakeNotificationRepo) BulkUpdateStatusByIDs(ctx context.Context, ids []string, status notification.Status, reason string) (int, error) {
	return 0, nil
}
func (f *fakeNotificationRepo) ListByStatus(ctx context.Context, status notification.Status, limit int) ([]*notification.Notification, error) {
	return nil, nil
}
func (f *fakeNotificationRepo) ListByUserAndStatus(ctx context.Context, userID, appID string, status notification.Status, limit int) ([]*notification.Notification, error) {
	return nil, nil
}
func (f *fakeNotificationRepo) ListByUser(ctx context.Context, userID, appID string, limit int, before time.Time) ([]*notification.Notification, error) {
	return nil, nil
}
func (f *fakeNotificationRepo) GetUnreadCount(ctx context.Context, userID, appID string) (int64, error) {
	return 0, nil
}
func (f *fakeNotificationRepo) BulkUpdateDeliveryStatus(ctx context.Context, updates []notification.DeliveryStatusUpdate) error {
	return nil
}
func (f *fakeNotificationRepo) UpdateStatusForApp(ctx context.Context, appID string, from, to notification.Status, limit int) (int, error) {
	return 0, nil
}
func (f *fakeNotificationRepo) UpdateStatusByIDs(ctx context.Context, ids []string, status notification.Status) error {
	return nil
}
func (f *fakeNotificationRepo) BulkUpdateProviderResponse(ctx context.Context, updates []notification.ProviderResponseUpdate) error {
	return nil
}
func (f *fakeNotificationRepo) BulkUpdateStatusAndError(ctx context.Context, updates []notification.StatusErrorUpdate) error {
	return nil
}
func (f *fakeNotificationRepo) BulkUpdateStatusAndMetadata(ctx context.Context, updates []notification.StatusMetadataUpdate) error {
	return nil
}
func (f *fakeNotificationRepo) UpdateProviderResponse(ctx context.Context, notificationID string, response notification.ProviderResponse) error {
	return nil
}
func (f *fakeNotificationRepo) BulkUpdateSnooze(ctx context.Context, ids []string, status notification.Status, until *time.Time) error {
	return nil
}
func (f *fakeNotificationRepo) ListScheduled(ctx context.Context, limit int, now time.Time) ([]*notification.Notification, error) {
	return nil, nil
}
func (f *fakeNotificationRepo) ListDueRecurrence(ctx context.Context, now time.Time, limit int) ([]*notification.Notification, error) {
	return nil, nil
}
func (f *fakeNotificationRepo) BulkUpdateRecurrence(ctx context.Context, updates []notification.RecurrenceUpdate) error {
	return nil
}
func (f *fakeNotificationRepo) BulkUpdateMetadata(ctx context.Context, updates []notification.MetadataUpdate) error {
	return nil
}
func (f *fakeNotificationRepo) ListByIDs(ctx context.Context, ids []string) ([]*notification.Notification, error) {
	var res []*notification.Notification
	for _, id := range ids {
		if n := f.items[id]; n != nil {
			res = append(res, n)
		}
	}
	return res, nil
}

// Stub user repository to satisfy NotificationProcessor but unused here.
type fakeUserRepo struct{}

func (f *fakeUserRepo) Create(ctx context.Context, user *user.User) error { return nil }
func (f *fakeUserRepo) GetByID(ctx context.Context, id string) (*user.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) GetByExternalID(ctx context.Context, appID, externalID string) (*user.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) GetByEmail(ctx context.Context, appID, email string) (*user.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) Update(ctx context.Context, user *user.User) error { return nil }
func (f *fakeUserRepo) List(ctx context.Context, filter user.UserFilter) ([]*user.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) Delete(ctx context.Context, id string) error { return nil }
func (f *fakeUserRepo) AddDevice(ctx context.Context, userID string, device user.Device) error {
	return nil
}
func (f *fakeUserRepo) RemoveDevice(ctx context.Context, userID, deviceID string) error { return nil }
func (f *fakeUserRepo) UpdatePreferences(ctx context.Context, userID string, preferences user.Preferences) error {
	return nil
}
func (f *fakeUserRepo) Count(ctx context.Context, filter user.UserFilter) (int64, error) {
	return 0, nil
}
func (f *fakeUserRepo) BulkCreate(ctx context.Context, users []*user.User) error { return nil }
