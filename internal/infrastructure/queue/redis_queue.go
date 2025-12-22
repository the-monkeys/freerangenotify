package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"go.uber.org/zap"
)

const (
	// Queue names by priority
	QueueHigh   = "frn:queue:high"
	QueueNormal = "frn:queue:normal"
	QueueLow    = "frn:queue:low"

	// Special queues
	QueueRetry      = "frn:queue:retry"
	QueueDeadLetter = "frn:queue:dlq"
	QueueScheduled  = "frn:queue:scheduled"

	// Timeout for blocking operations
	BlockTimeout = 5 * time.Second
)

// RedisQueue implements the Queue interface using Redis Lists
type RedisQueue struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRedisQueue creates a new Redis queue
func NewRedisQueue(client *redis.Client, logger *zap.Logger) Queue {
	return &RedisQueue{
		client: client,
		logger: logger,
	}
}

// Enqueue adds a notification to the appropriate priority queue
func (q *RedisQueue) Enqueue(ctx context.Context, item NotificationQueueItem) error {
	queueName := q.getQueueName(item.Priority)

	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal queue item: %w", err)
	}

	// LPUSH adds to the head of the list
	if err := q.client.LPush(ctx, queueName, data).Err(); err != nil {
		return fmt.Errorf("failed to enqueue item: %w", err)
	}

	q.logger.Debug("Item enqueued",
		zap.String("notification_id", item.NotificationID),
		zap.String("queue", queueName),
		zap.String("priority", string(item.Priority)))

	return nil
}

// EnqueuePriority adds a notification to the tail of the list (next to be RPOP'd)
func (q *RedisQueue) EnqueuePriority(ctx context.Context, item NotificationQueueItem) error {
	queueName := q.getQueueName(item.Priority)

	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal queue item: %w", err)
	}

	// RPUSH adds to the tail of the list. Since we use BRPOP (from tail), this item will be next.
	if err := q.client.RPush(ctx, queueName, data).Err(); err != nil {
		return fmt.Errorf("failed to enqueue priority item: %w", err)
	}

	q.logger.Info("Priority item enqueued (Jump the line)",
		zap.String("notification_id", item.NotificationID),
		zap.String("queue", queueName))

	return nil
}

// Dequeue removes and returns the next notification from the queues
// Priority order: high -> normal -> low
func (q *RedisQueue) Dequeue(ctx context.Context) (*NotificationQueueItem, error) {
	// Try queues in priority order
	queues := []string{QueueHigh, QueueNormal, QueueLow}

	// BRPOP blocks until an item is available or timeout
	result, err := q.client.BRPop(ctx, BlockTimeout, queues...).Result()
	if err != nil {
		if err == redis.Nil {
			// No items available (timeout)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to dequeue item: %w", err)
	}

	// result is [queue_name, data]
	if len(result) < 2 {
		return nil, fmt.Errorf("invalid dequeue result")
	}

	var item NotificationQueueItem
	if err := json.Unmarshal([]byte(result[1]), &item); err != nil {
		return nil, fmt.Errorf("failed to unmarshal queue item: %w", err)
	}

	q.logger.Debug("Item dequeued",
		zap.String("notification_id", item.NotificationID),
		zap.String("queue", result[0]))

	return &item, nil
}

// EnqueueBatch adds multiple notifications to the queue
func (q *RedisQueue) EnqueueBatch(ctx context.Context, items []NotificationQueueItem) error {
	if len(items) == 0 {
		return nil
	}

	// Group items by priority
	queueItems := make(map[string][]interface{})

	for _, item := range items {
		queueName := q.getQueueName(item.Priority)

		data, err := json.Marshal(item)
		if err != nil {
			q.logger.Error("Failed to marshal item in batch",
				zap.String("notification_id", item.NotificationID),
				zap.Error(err))
			continue
		}

		queueItems[queueName] = append(queueItems[queueName], data)
	}

	// Use pipeline for efficiency
	pipe := q.client.Pipeline()

	for queueName, items := range queueItems {
		pipe.LPush(ctx, queueName, items...)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to enqueue batch: %w", err)
	}

	q.logger.Info("Batch enqueued",
		zap.Int("total", len(items)))

	return nil
}

// GetQueueDepth returns the number of items in each queue
func (q *RedisQueue) GetQueueDepth(ctx context.Context) (map[string]int64, error) {
	queues := []string{QueueHigh, QueueNormal, QueueLow, QueueRetry, QueueDeadLetter}

	depths := make(map[string]int64)

	for _, queue := range queues {
		var length int64
		var err error

		if queue == QueueRetry || queue == QueueScheduled {
			length, err = q.client.ZCard(ctx, queue).Result()
		} else {
			length, err = q.client.LLen(ctx, queue).Result()
		}

		if err != nil {
			return nil, fmt.Errorf("failed to get queue depth for %s: %w", queue, err)
		}
		depths[queue] = length
	}

	return depths, nil
}

// Peek returns the next item without removing it
func (q *RedisQueue) Peek(ctx context.Context) (*NotificationQueueItem, error) {
	// Try queues in priority order
	queues := []string{QueueHigh, QueueNormal, QueueLow}

	for _, queueName := range queues {
		// LINDEX gets element at index (0 = head for RPOP direction, -1 = tail for LPUSH direction)
		data, err := q.client.LIndex(ctx, queueName, -1).Result()
		if err != nil {
			if err == redis.Nil {
				// Queue is empty, try next
				continue
			}
			return nil, fmt.Errorf("failed to peek queue %s: %w", queueName, err)
		}

		var item NotificationQueueItem
		if err := json.Unmarshal([]byte(data), &item); err != nil {
			return nil, fmt.Errorf("failed to unmarshal queue item: %w", err)
		}

		return &item, nil
	}

	// All queues are empty
	return nil, nil
}

// EnqueueRetry adds a notification to the retry queue with delay
func (q *RedisQueue) EnqueueRetry(ctx context.Context, item NotificationQueueItem, delay time.Duration) error {
	// Add retry timestamp to item
	item.EnqueuedAt = time.Now().Add(delay)

	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal retry item: %w", err)
	}

	// Use sorted set for delayed retry (score = timestamp)
	score := float64(item.EnqueuedAt.Unix())
	if err := q.client.ZAdd(ctx, QueueRetry, &redis.Z{
		Score:  score,
		Member: data,
	}).Err(); err != nil {
		return fmt.Errorf("failed to enqueue retry item: %w", err)
	}

	q.logger.Info("Item enqueued for retry",
		zap.String("notification_id", item.NotificationID),
		zap.Duration("delay", delay))

	return nil
}

// GetRetryableItems returns items from retry queue that are ready to be retried
func (q *RedisQueue) GetRetryableItems(ctx context.Context, limit int64) ([]NotificationQueueItem, error) {
	now := float64(time.Now().Unix())

	// Get items with score <= now (ready to retry)
	results, err := q.client.ZRangeByScore(ctx, QueueRetry, &redis.ZRangeBy{
		Min:   "0",
		Max:   fmt.Sprintf("%f", now),
		Count: limit,
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to get retryable items: %w", err)
	}

	var items []NotificationQueueItem
	for _, data := range results {
		var item NotificationQueueItem
		if err := json.Unmarshal([]byte(data), &item); err != nil {
			q.logger.Error("Failed to unmarshal retry item", zap.Error(err))
			continue
		}
		items = append(items, item)
	}

	// Remove processed items from retry queue
	if len(results) > 0 {
		if err := q.client.ZRem(ctx, QueueRetry, results).Err(); err != nil {
			q.logger.Error("Failed to remove retry items", zap.Error(err))
		}
	}

	return items, nil
}

// ListDLQ returns items from the dead letter queue
func (q *RedisQueue) ListDLQ(ctx context.Context, limit int) ([]DLQItem, error) {
	// LRANGE gets elements from list (-limit to -1 gets the most recent ones if added with LPUSH)
	// Actually LPUSH adds to head, so 0 to limit-1 gets the most recent
	results, err := q.client.LRange(ctx, QueueDeadLetter, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list DLQ items: %w", err)
	}

	var items []DLQItem
	for _, data := range results {
		var item DLQItem
		if err := json.Unmarshal([]byte(data), &item); err != nil {
			q.logger.Error("Failed to unmarshal DLQ item", zap.Error(err))
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

// ReplayDLQ moves items from DLQ back to their original priority queues
func (q *RedisQueue) ReplayDLQ(ctx context.Context, limit int) (int, error) {
	replayedCount := 0

	for i := 0; i < limit; i++ {
		// RPOP gets the oldest item from DLQ
		data, err := q.client.RPop(ctx, QueueDeadLetter).Result()
		if err != nil {
			if err == redis.Nil {
				break // Queue empty
			}
			return replayedCount, fmt.Errorf("failed to pop from DLQ: %w", err)
		}

		var dlqItem DLQItem
		if err := json.Unmarshal([]byte(data), &dlqItem); err != nil {
			q.logger.Error("Failed to unmarshal DLQ item for replay", zap.Error(err))
			continue
		}

		// Prepare for re-enqueueing (reset retry count?)
		// Usually we keep the count but it's now back in the main flow
		dlqItem.NotificationQueueItem.EnqueuedAt = time.Now()

		// Enqueue back to appropriate priority queue
		if err := q.Enqueue(ctx, dlqItem.NotificationQueueItem); err != nil {
			q.logger.Error("Failed to re-enqueue item from DLQ",
				zap.String("notification_id", dlqItem.NotificationID),
				zap.Error(err))
			// Optionally put it back in DLQ? For now just log
			continue
		}

		replayedCount++
	}

	if replayedCount > 0 {
		q.logger.Info("Replayed items from DLQ", zap.Int("count", replayedCount))
	}

	return replayedCount, nil
}

// EnqueueDeadLetter adds a notification to the dead letter queue
func (q *RedisQueue) EnqueueDeadLetter(ctx context.Context, item NotificationQueueItem, reason string) error {
	dlqItem := DLQItem{
		NotificationQueueItem: item,
		Reason:                reason,
		Timestamp:             time.Now(),
	}

	data, err := json.Marshal(dlqItem)
	if err != nil {
		return fmt.Errorf("failed to marshal DLQ item: %w", err)
	}

	if err := q.client.LPush(ctx, QueueDeadLetter, data).Err(); err != nil {
		return fmt.Errorf("failed to enqueue DLQ item: %w", err)
	}

	q.logger.Warn("Item moved to dead letter queue",
		zap.String("notification_id", item.NotificationID),
		zap.String("reason", reason))

	return nil
}

// Close closes the Redis client connection
func (q *RedisQueue) Close() error {
	return q.client.Close()
}

// EnqueueScheduled adds a notification to the scheduled queue (delayed)
func (q *RedisQueue) EnqueueScheduled(ctx context.Context, item NotificationQueueItem, scheduledAt time.Time) error {
	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal scheduled item: %w", err)
	}

	// Use sorted set for scheduled items (score = scheduled timestamp)
	score := float64(scheduledAt.Unix())
	if err := q.client.ZAdd(ctx, QueueScheduled, &redis.Z{
		Score:  score,
		Member: data,
	}).Err(); err != nil {
		return fmt.Errorf("failed to enqueue scheduled item: %w", err)
	}

	q.logger.Info("Item scheduled in Redis",
		zap.String("notification_id", item.NotificationID),
		zap.Time("scheduled_at", scheduledAt))

	return nil
}

// GetScheduledItems returns items from scheduled queue that are ready to be processed
func (q *RedisQueue) GetScheduledItems(ctx context.Context, limit int64) ([]NotificationQueueItem, error) {
	now := float64(time.Now().Unix())

	// Get items with score <= now (ready to process)
	results, err := q.client.ZRangeByScore(ctx, QueueScheduled, &redis.ZRangeBy{
		Min:   "0",
		Max:   fmt.Sprintf("%f", now),
		Count: limit,
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled items: %w", err)
	}

	var items []NotificationQueueItem
	for _, data := range results {
		var item NotificationQueueItem
		if err := json.Unmarshal([]byte(data), &item); err != nil {
			q.logger.Error("Failed to unmarshal scheduled item", zap.Error(err))
			continue
		}
		items = append(items, item)
	}

	// Remove processed items from scheduled queue
	if len(results) > 0 {
		if err := q.client.ZRem(ctx, QueueScheduled, results).Err(); err != nil {
			q.logger.Error("Failed to remove scheduled items", zap.Error(err))
		}
	}

	return items, nil
}

// getQueueName returns the queue name for a given priority
func (q *RedisQueue) getQueueName(priority notification.Priority) string {
	switch priority {
	case notification.PriorityCritical, notification.PriorityHigh:
		return QueueHigh
	case notification.PriorityLow:
		return QueueLow
	default:
		return QueueNormal
	}
}
