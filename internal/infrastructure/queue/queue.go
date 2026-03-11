package queue

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

// NotificationQueueItem represents an item in the notification queue
type NotificationQueueItem struct {
	NotificationID string                `json:"notification_id"`
	AppID          string                `json:"app_id,omitempty"`
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

	// EnqueuePriority adds a notification to the front of the queue (RPush for BRPop)
	EnqueuePriority(ctx context.Context, item NotificationQueueItem) error

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

	// ReplayDLQForApps replays DLQ items only for the specified app IDs
	ReplayDLQForApps(ctx context.Context, limit int, allowedApps map[string]bool) (int, error)

	// EnqueueScheduled adds a notification to the scheduled queue (delayed)
	EnqueueScheduled(ctx context.Context, item NotificationQueueItem, scheduledAt time.Time) error

	// GetScheduledItems returns items from scheduled queue that are ready to be processed
	GetScheduledItems(ctx context.Context, limit int64) ([]NotificationQueueItem, error)

	// RemoveScheduledByID removes notifications from the scheduled queue by ID.
	// Used when cancelling or snoozing so they are not dequeued and sent when due.
	RemoveScheduledByID(ctx context.Context, notificationIDs []string) error

	// Acknowledge removes a processed item from the processing set
	Acknowledge(ctx context.Context, item NotificationQueueItem) error

	// RequeueExpiredProcessing moves timed-out items back to their priority queues
	RequeueExpiredProcessing(ctx context.Context) (int, error)

	// Close closes the queue connection
	Close() error
}
