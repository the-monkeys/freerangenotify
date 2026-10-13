package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
)

type scheduleClaimRepo struct {
	*stubNotifRepo
	mu       sync.Mutex
	row      *notification.Notification
	fallback bool
}

func (r *scheduleClaimRepo) GetPending(context.Context) ([]*notification.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.fallback {
		return nil, nil
	}
	copy := *r.row
	copy.Status = notification.StatusPending
	return []*notification.Notification{&copy}, nil // deliberately stale search snapshot
}
func (r *scheduleClaimRepo) GetByID(context.Context, string) (*notification.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := *r.row
	return &copy, nil
}
func (r *scheduleClaimRepo) ClaimScheduledNotification(context.Context, string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.row.Status != notification.StatusPending {
		return false, nil
	}
	r.row.Status = notification.StatusQueued
	return true, nil
}
func (r *scheduleClaimRepo) BulkUpdateStatus(_ context.Context, _ []string, status notification.Status) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.row.Status = status
	return nil
}

type scheduledConsumer struct {
	stubQueue
	repo       *scheduleClaimRepo
	first      atomic.Bool
	dispatches atomic.Int64
}

func (q *scheduledConsumer) GetScheduledItems(context.Context, int64) ([]queue.NotificationQueueItem, error) {
	if q.repo.fallback || q.first.Swap(true) {
		return nil, nil
	}
	return []queue.NotificationQueueItem{{NotificationID: q.repo.row.NotificationID}}, nil
}
func (q *scheduledConsumer) EnqueueBatch(_ context.Context, items []queue.NotificationQueueItem) error {
	q.repo.mu.Lock()
	defer q.repo.mu.Unlock()
	q.dispatches.Add(int64(len(items)))
	q.repo.row.Status = notification.StatusSent
	return nil
}
func TestSchedulerClaimsBeforePublicationAndPreservesFastDelivery(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		t.Run(map[bool]string{false: "redis", true: "fallback"}[fallback], func(t *testing.T) {
			repo := &scheduleClaimRepo{stubNotifRepo: newStubNotifRepo(), row: &notification.Notification{NotificationID: "scheduled", Status: notification.StatusPending}, fallback: fallback}
			q := &scheduledConsumer{repo: repo}
			p := newProcessorForTest(repo.stubNotifRepo, nil, nil, nil, nil, nil)
			p.notifRepo = repo
			p.queue = q
			p.config.PollInterval = time.Millisecond * 5
			ctx, cancel := context.WithCancel(context.Background())
			p.wg.Add(2)
			go p.scheduler(ctx)
			go p.scheduler(ctx)
			require.Eventually(t, func() bool { return q.dispatches.Load() > 0 }, time.Second, time.Millisecond)
			time.Sleep(time.Millisecond * 40)
			cancel()
			p.wg.Wait()
			require.Equal(t, int64(1), q.dispatches.Load(), "two schedulers must not republish stale pending records")
			require.Equal(t, notification.StatusSent, repo.row.Status, "scheduler must not overwrite a fast worker's success")
		})
	}
}
