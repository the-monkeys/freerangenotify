package queue

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

// NotificationQueueItem represents an item in the notification queue
type NotificationQueueItem struct {
	NotificationID string                `json:"notification_id"`
	Priority       notification.Priority `json:"priority"`
	RetryCount     int                   `json:"retry_count"`
	EnqueuedAt     time.Time             `json:"enqueued_at"`
}

// DLQItem represents an item in the dead letter queue with failure reason
type DLQItem struct {
	NotificationQueueItem
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

// Queue defines the interface for notification queueing operations
type Queue interface {
	// Enqueue adds a notification to the queue
	Enqueue(ctx context.Context, item NotificationQueueItem) error

	// Dequeue removes and returns the next notification from the queue
	// Blocks until an item is available or context is canceled
	Dequeue(ctx context.Context) (*NotificationQueueItem, error)

	// EnqueueBatch adds multiple notifications to the queue
	EnqueueBatch(ctx context.Context, items []NotificationQueueItem) error

	// GetQueueDepth returns the number of items in each priority queue
	GetQueueDepth(ctx context.Context) (map[string]int64, error)

	// Peek returns the next item without removing it
	Peek(ctx context.Context) (*NotificationQueueItem, error)

	// ListDLQ returns items from the dead letter queue
	ListDLQ(ctx context.Context, limit int) ([]DLQItem, error)

	// ReplayDLQ moves items from DLQ back to their original priority queues
	ReplayDLQ(ctx context.Context, limit int) (int, error)

	// EnqueueScheduled adds a notification to the scheduled queue (delayed)
	EnqueueScheduled(ctx context.Context, item NotificationQueueItem, scheduledAt time.Time) error

	// GetScheduledItems returns items from scheduled queue that are ready to be processed
	GetScheduledItems(ctx context.Context, limit int64) ([]NotificationQueueItem, error)

	// Close closes the queue connection
	Close() error
}
