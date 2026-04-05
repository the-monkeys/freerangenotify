package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"go.uber.org/zap"
)

func TestDelayedPollerCatchesUpAfterDowntime(t *testing.T) {
	now := time.Now()
	q := &fakeWorkflowQueue{}

	due := queue.WorkflowQueueItem{ExecutionID: "exec-1", StepID: "step-1", EnqueuedAt: now}
	future := queue.WorkflowQueueItem{ExecutionID: "exec-2", StepID: "step-2", EnqueuedAt: now}

	require.NoError(t, q.EnqueueWorkflowDelayed(context.Background(), due, now.Add(-time.Hour)))
	require.NoError(t, q.EnqueueWorkflowDelayed(context.Background(), future, now.Add(time.Hour)))

	engine := NewEngine(nil, nil, q, nil, zap.NewNop(), nil, 0, 50)

	engine.processDelayedOnce(context.Background())

	require.Len(t, q.main, 1)
	require.Equal(t, "exec-1", q.main[0].ExecutionID)
	require.Len(t, q.delayed, 1, "future item should stay delayed")
}

// minimal in-memory WorkflowQueue implementation for tests
type fakeWorkflowQueue struct {
	main    []queue.WorkflowQueueItem
	delayed []delayedItem
}

type delayedItem struct {
	item      queue.WorkflowQueueItem
	executeAt time.Time
}

func (f *fakeWorkflowQueue) EnqueueWorkflow(ctx context.Context, item queue.WorkflowQueueItem) error {
	f.main = append(f.main, item)
	return nil
}

func (f *fakeWorkflowQueue) DequeueWorkflow(ctx context.Context) (*queue.WorkflowQueueItem, error) {
	if len(f.main) == 0 {
		return nil, nil
	}
	item := f.main[0]
	f.main = f.main[1:]
	return &item, nil
}

func (f *fakeWorkflowQueue) EnqueueWorkflowDelayed(ctx context.Context, item queue.WorkflowQueueItem, executeAt time.Time) error {
	f.delayed = append(f.delayed, delayedItem{item: item, executeAt: executeAt})
	return nil
}

func (f *fakeWorkflowQueue) GetDelayedWorkflowItems(ctx context.Context, limit int64) ([]queue.WorkflowQueueItem, error) {
	now := time.Now()
	var ready []queue.WorkflowQueueItem
	var remaining []delayedItem
	for _, di := range f.delayed {
		if di.executeAt.Before(now) || di.executeAt.Equal(now) {
			if int64(len(ready)) < limit {
				ready = append(ready, di.item)
				continue
			}
		}
		remaining = append(remaining, di)
	}
	f.delayed = remaining
	return ready, nil
}

// --- Queue (embedded) stubs ---
func (f *fakeWorkflowQueue) Enqueue(ctx context.Context, item queue.NotificationQueueItem) error {
	return nil
}
func (f *fakeWorkflowQueue) EnqueuePriority(ctx context.Context, item queue.NotificationQueueItem) error {
	return nil
}
func (f *fakeWorkflowQueue) Dequeue(ctx context.Context) (*queue.NotificationQueueItem, error) {
	return nil, nil
}
func (f *fakeWorkflowQueue) EnqueueBatch(ctx context.Context, items []queue.NotificationQueueItem) error {
	return nil
}
func (f *fakeWorkflowQueue) GetQueueDepth(ctx context.Context) (map[string]int64, error) {
	return nil, nil
}
func (f *fakeWorkflowQueue) Peek(ctx context.Context) (*queue.NotificationQueueItem, error) {
	return nil, nil
}
func (f *fakeWorkflowQueue) ListDLQ(ctx context.Context, limit int) ([]queue.DLQItem, error) {
	return nil, nil
}
func (f *fakeWorkflowQueue) ReplayDLQ(ctx context.Context, limit int) (int, error) { return 0, nil }
func (f *fakeWorkflowQueue) ReplayDLQForApps(ctx context.Context, limit int, allowedApps map[string]bool) (int, error) {
	return 0, nil
}
func (f *fakeWorkflowQueue) EnqueueScheduled(ctx context.Context, item queue.NotificationQueueItem, scheduledAt time.Time) error {
	return nil
}
func (f *fakeWorkflowQueue) GetScheduledItems(ctx context.Context, limit int64) ([]queue.NotificationQueueItem, error) {
	return nil, nil
}
func (f *fakeWorkflowQueue) RemoveScheduledByID(ctx context.Context, notificationIDs []string) error {
	return nil
}
func (f *fakeWorkflowQueue) Acknowledge(ctx context.Context, item queue.NotificationQueueItem) error {
	return nil
}
func (f *fakeWorkflowQueue) RequeueExpiredProcessing(ctx context.Context) (int, error) { return 0, nil }
func (f *fakeWorkflowQueue) Close() error                                              { return nil }
