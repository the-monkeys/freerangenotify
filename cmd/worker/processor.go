package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/metrics"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"github.com/the-monkeys/freerangenotify/pkg/utils"
	"go.uber.org/zap"
)

// ProcessorConfig holds configuration for the notification processor
type ProcessorConfig struct {
	WorkerCount     int
	PollInterval    time.Duration
	MaxRetries      int
	RetryDelay      time.Duration
	MaxRetryDelay   time.Duration
	ShutdownTimeout time.Duration
}

// NotificationProcessor processes notifications from the queue
type NotificationProcessor struct {
	queue           queue.Queue
	notifRepo       notification.Repository
	userRepo        user.Repository
	providerManager *providers.Manager
	logger          *zap.Logger
	config          ProcessorConfig
	metrics         *metrics.NotificationMetrics

	wg       sync.WaitGroup
	stopChan chan struct{}
}

// NewNotificationProcessor creates a new notification processor
func NewNotificationProcessor(
	q queue.Queue,
	notifRepo notification.Repository,
	userRepo user.Repository,
	providerManager *providers.Manager,
	logger *zap.Logger,
	config ProcessorConfig,
	metrics *metrics.NotificationMetrics,
) *NotificationProcessor {
	return &NotificationProcessor{
		queue:           q,
		notifRepo:       notifRepo,
		userRepo:        userRepo,
		providerManager: providerManager,
		logger:          logger,
		config:          config,
		metrics:         metrics,
		stopChan:        make(chan struct{}),
	}
}

// Start starts the notification processor with multiple workers
func (p *NotificationProcessor) Start(ctx context.Context) error {
	p.logger.Info("Starting notification processor",
		zap.Int("worker_count", p.config.WorkerCount))

	// Start worker goroutines
	for i := 0; i < p.config.WorkerCount; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}

	// Start scheduler for pending notifications
	p.wg.Add(1)
	go p.scheduler(ctx)

	// Start retry processor
	p.wg.Add(1)
	go p.retryProcessor(ctx)

	// Start metrics updater
	if p.metrics != nil {
		p.wg.Add(1)
		go p.metricsUpdater(ctx)
	}

	return nil
}

// Shutdown gracefully stops the processor
func (p *NotificationProcessor) Shutdown(ctx context.Context) error {
	p.logger.Info("Shutting down notification processor")

	close(p.stopChan)

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("All workers stopped gracefully")
		return nil
	case <-ctx.Done():
		p.logger.Warn("Shutdown timeout exceeded, forcing stop")
		return ctx.Err()
	}
}

// worker processes notifications from the queue
func (p *NotificationProcessor) worker(ctx context.Context, workerID int) {
	defer p.wg.Done()

	logger := p.logger.With(zap.Int("worker_id", workerID))
	logger.Info("Worker started")

	for {
		select {
		case <-ctx.Done():
			logger.Info("Worker stopping (context cancelled)")
			return
		case <-p.stopChan:
			logger.Info("Worker stopping (shutdown signal)")
			return
		default:
			// Try to dequeue a notification
			item, err := p.queue.Dequeue(ctx)
			if err != nil {
				logger.Error("Failed to dequeue notification", zap.Error(err))
				time.Sleep(p.config.PollInterval)
				continue
			}

			if item == nil {
				// No items available
				time.Sleep(p.config.PollInterval)
				continue
			}

			// Process the notification
			p.processNotification(ctx, item, logger)
		}
	}
}

// processNotification processes a single notification
func (p *NotificationProcessor) processNotification(ctx context.Context, item *queue.NotificationQueueItem, logger *zap.Logger) {
	startTime := time.Now()

	logger.Info("Processing notification",
		zap.String("notification_id", item.NotificationID),
		zap.String("priority", string(item.Priority)))

	// Record queue latency
	if p.metrics != nil {
		queueLatency := time.Since(item.EnqueuedAt).Seconds()
		p.metrics.RecordQueueLatency(string(item.Priority), queueLatency)
	}

	// Get notification from database
	notif, err := p.notifRepo.GetByID(ctx, item.NotificationID)
	if err != nil {
		logger.Error("Failed to get notification", zap.Error(err))
		return
	}

	// Update status to processing
	if err := p.notifRepo.UpdateStatus(ctx, notif.NotificationID, notification.StatusProcessing); err != nil {
		logger.Error("Failed to update status to processing", zap.Error(err))
	}

	// Get user details
	usr, err := p.userRepo.GetByID(ctx, notif.UserID)
	if err != nil {
		logger.Error("Failed to get user", zap.Error(err))
		p.handleFailure(ctx, notif, item, "user not found")
		return
	}

	// Check user preferences
	if !p.checkUserPreferences(usr, notif) {
		logger.Info("Notification blocked by user preferences")
		p.notifRepo.UpdateStatus(ctx, notif.NotificationID, notification.StatusCancelled)
		return
	}

	// TODO: Route to appropriate provider based on channel
	// For now, simulate sending
	err = p.sendNotification(ctx, notif, usr)
	if err != nil {
		logger.Error("Failed to send notification", zap.Error(err))
		// Record failure metrics
		if p.metrics != nil {
			p.metrics.RecordDeliveryFailure(string(notif.Channel), "default", "send_error")
		}
		p.handleFailure(ctx, notif, item, err.Error())
		return
	}

	// Update status to sent
	if err := p.notifRepo.UpdateStatus(ctx, notif.NotificationID, notification.StatusSent); err != nil {
		logger.Error("Failed to update status to sent", zap.Error(err))
	}

	// Record metrics
	if p.metrics != nil {
		processingDuration := time.Since(startTime).Seconds()
		p.metrics.RecordProcessingDuration(string(notif.Channel), string(notification.StatusSent), processingDuration)
		p.metrics.RecordDeliverySuccess(string(notif.Channel), "default")
	}

	logger.Info("Notification processed successfully",
		zap.String("notification_id", notif.NotificationID))

	// Handle Recurrence
	if notif.Recurrence != nil {
		p.handleRecurrence(ctx, notif)
	}
}

// sendNotification sends the notification via the appropriate provider
func (p *NotificationProcessor) sendNotification(ctx context.Context, notif *notification.Notification, usr *user.User) error {
	// Use provider manager to route and send
	if p.providerManager == nil {
		// Fallback: simulate sending if no provider manager (for testing)
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	// Send via provider manager
	result, err := p.providerManager.Send(ctx, notif, usr)
	if err != nil {
		return fmt.Errorf("provider send failed: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("provider delivery failed: %s", result.ErrorType)
	}

	return nil
}

// handleFailure handles notification send failure
func (p *NotificationProcessor) handleFailure(ctx context.Context, notif *notification.Notification, item *queue.NotificationQueueItem, errorMsg string) {
	// Record retry metric
	if p.metrics != nil {
		p.metrics.RecordRetry(string(notif.Channel), errorMsg)
	}

	// Increment retry count
	if err := p.notifRepo.IncrementRetryCount(ctx, notif.NotificationID, errorMsg); err != nil {
		p.logger.Error("Failed to increment retry count", zap.Error(err))
	}

	// Check if can retry
	if notif.RetryCount >= p.config.MaxRetries {
		// Move to dead letter queue
		redisQueue, ok := p.queue.(*queue.RedisQueue)
		if ok {
			if err := redisQueue.EnqueueDeadLetter(ctx, *item, fmt.Sprintf("max retries exceeded: %s", errorMsg)); err != nil {
				p.logger.Error("Failed to move to dead letter queue", zap.Error(err))
			}
		}

		// Update status to failed
		p.notifRepo.UpdateStatus(ctx, notif.NotificationID, notification.StatusFailed)
		// Update error message separately
		notif.ErrorMessage = errorMsg
		p.notifRepo.Update(ctx, notif)
		return
	}

	// Schedule retry with exponential backoff and jitter
	delay := utils.CalculateBackoff(p.config.RetryDelay, notif.RetryCount, p.config.MaxRetryDelay)
	redisQueue, ok := p.queue.(*queue.RedisQueue)
	if ok {
		if err := redisQueue.EnqueueRetry(ctx, *item, delay); err != nil {
			p.logger.Error("Failed to enqueue retry", zap.Error(err))
		}
	}
}

// checkUserPreferences checks if notification should be sent based on user preferences
func (p *NotificationProcessor) checkUserPreferences(usr *user.User, notif *notification.Notification) bool {

	// Check if channel is enabled
	switch notif.Channel {
	case notification.ChannelEmail:
		if !utils.BoolValue(usr.Preferences.EmailEnabled) {
			return false
		}
	case notification.ChannelPush:
		if !utils.BoolValue(usr.Preferences.PushEnabled) {
			return false
		}
	case notification.ChannelSMS:
		if !utils.BoolValue(usr.Preferences.SMSEnabled) {
			return false
		}
	}

	// Check quiet hours (except for critical notifications)
	if notif.Priority != notification.PriorityCritical {
		if p.isQuietHours(usr) {
			return false
		}
	}

	return true
}

// isQuietHours checks if user is in quiet hours
func (p *NotificationProcessor) isQuietHours(usr *user.User) bool {
	// If quiet hours not configured, not in quiet hours
	if usr.Preferences.QuietHours.Start == "" || usr.Preferences.QuietHours.End == "" {
		return false
	}

	now := time.Now()
	if usr.Timezone != "" {
		loc, err := time.LoadLocation(usr.Timezone)
		if err != nil {
			p.logger.Warn("Invalid timezone", zap.String("user_id", usr.UserID), zap.String("timezone", usr.Timezone))
		} else {
			now = now.In(loc)
		}
	}

	currentTime := now.Format("15:04")
	start := usr.Preferences.QuietHours.Start
	end := usr.Preferences.QuietHours.End

	// Handle quiet hours spanning midnight
	if start < end {
		return currentTime >= start && currentTime < end
	}
	return currentTime >= start || currentTime < end
}

// scheduler periodically checks for scheduled notifications that are ready to be sent
func (p *NotificationProcessor) scheduler(ctx context.Context) {
	defer p.wg.Done()

	p.logger.Info("Scheduler started")
	ticker := time.NewTicker(p.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Scheduler stopping")
			return
		case <-p.stopChan:
			p.logger.Info("Scheduler stopping")
			return
		case <-ticker.C:
			// 1. Try to get items from Redis scheduled queue (Optimized path)
			scheduledItems, err := p.queue.GetScheduledItems(ctx, 100)
			if err != nil {
				p.logger.Error("Failed to get scheduled items from Redis", zap.Error(err))
			} else if len(scheduledItems) > 0 {
				p.logger.Info("Found ready scheduled notifications in Redis", zap.Int("count", len(scheduledItems)))
				if err := p.queue.EnqueueBatch(ctx, scheduledItems); err != nil {
					p.logger.Error("Failed to enqueue scheduled items from Redis", zap.Error(err))
				}

				// Update statuses to queued in ES
				var ids []string
				for _, item := range scheduledItems {
					ids = append(ids, item.NotificationID)
				}
				if err := p.notifRepo.BulkUpdateStatus(ctx, ids, notification.StatusQueued); err != nil {
					p.logger.Error("Failed to bulk update status for Redis items", zap.Error(err))
				}
			}

			// 2. Fallback: Get pending notifications from ES ready to be sent
			// This catches items that might have missed Redis or were created directly in ES
			pending, err := p.notifRepo.GetPending(ctx)
			if err != nil {
				p.logger.Error("Failed to get pending notifications from ES", zap.Error(err))
				continue
			}

			if len(pending) == 0 {
				continue
			}

			p.logger.Info("Found pending notifications in ES (fallback/sync)", zap.Int("count", len(pending)))

			// Enqueue them
			var items []queue.NotificationQueueItem
			for _, notif := range pending {
				items = append(items, queue.NotificationQueueItem{
					NotificationID: notif.NotificationID,
					Priority:       notif.Priority,
					EnqueuedAt:     time.Now(),
				})
			}

			if err := p.queue.EnqueueBatch(ctx, items); err != nil {
				p.logger.Error("Failed to enqueue pending notifications from ES", zap.Error(err))
				continue
			}

			// Update statuses to queued
			var ids []string
			for _, notif := range pending {
				ids = append(ids, notif.NotificationID)
			}
			if err := p.notifRepo.BulkUpdateStatus(ctx, ids, notification.StatusQueued); err != nil {
				p.logger.Error("Failed to bulk update status from ES", zap.Error(err))
			}
		}
	}
}

// retryProcessor processes notifications from the retry queue
func (p *NotificationProcessor) retryProcessor(ctx context.Context) {
	defer p.wg.Done()

	p.logger.Info("Retry processor started")
	ticker := time.NewTicker(p.config.PollInterval)
	defer ticker.Stop()

	redisQueue, ok := p.queue.(*queue.RedisQueue)
	if !ok {
		p.logger.Warn("Queue is not RedisQueue, retry processor disabled")
		return
	}

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Retry processor stopping")
			return
		case <-p.stopChan:
			p.logger.Info("Retry processor stopping")
			return
		case <-ticker.C:
			// Get retryable items
			items, err := redisQueue.GetRetryableItems(ctx, 100)
			if err != nil {
				p.logger.Error("Failed to get retryable items", zap.Error(err))
				continue
			}

			if len(items) == 0 {
				continue
			}

			p.logger.Info("Found retryable notifications", zap.Int("count", len(items)))

			// Re-enqueue them to appropriate priority queue
			if err := p.queue.EnqueueBatch(ctx, items); err != nil {
				p.logger.Error("Failed to re-enqueue retryable notifications", zap.Error(err))
			}
		}
	}
}

// metricsUpdater periodically updates queue depth metrics
func (p *NotificationProcessor) metricsUpdater(ctx context.Context) {
	defer p.wg.Done()

	p.logger.Info("Metrics updater started")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Metrics updater stopping")
			return
		case <-p.stopChan:
			p.logger.Info("Metrics updater stopping")
			return
		case <-ticker.C:
			// Get queue depths
			depths, err := p.queue.GetQueueDepth(ctx)
			if err != nil {
				p.logger.Error("Failed to get queue depths", zap.Error(err))
				continue
			}

			// Update metrics for each priority
			for priority, depth := range depths {
				p.metrics.SetQueueDepth(priority, float64(depth))
			}
		}
	}
}

// handleRecurrence schedules the next instance of a recurring notification
func (p *NotificationProcessor) handleRecurrence(ctx context.Context, notif *notification.Notification) {
	// Calculate next run time
	lastRun := time.Now()
	if notif.ScheduledAt != nil {
		lastRun = *notif.ScheduledAt
	}

	nextRun, err := notif.Recurrence.CalculateNextRun(lastRun)
	if err != nil {
		p.logger.Error("Failed to calculate next run for recurring notification",
			zap.String("notification_id", notif.NotificationID),
			zap.Error(err))
		return
	}

	if nextRun.IsZero() {
		return // No more runs
	}

	// Create new notification
	newRecurrence := *notif.Recurrence
	newRecurrence.CurrentCount++

	newNotif := &notification.Notification{
		NotificationID: uuid.New().String(),
		AppID:          notif.AppID,
		UserID:         notif.UserID,
		Channel:        notif.Channel,
		Priority:       notif.Priority,
		Status:         notification.StatusPending,
		Content:        notif.Content,
		Category:       notif.Category,
		TemplateID:     notif.TemplateID,
		ScheduledAt:    &nextRun,
		Recurrence:     &newRecurrence,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		RetryCount:     0,
	}

	// Save new notification
	if err := p.notifRepo.Create(ctx, newNotif); err != nil {
		p.logger.Error("Failed to create next recurring notification", zap.Error(err))
		return
	}

	p.logger.Info("Scheduled next recurring notification",
		zap.String("original_id", notif.NotificationID),
		zap.String("new_id", newNotif.NotificationID),
		zap.Time("next_run", nextRun))

	// Enqueue in scheduled queue
	queueItem := queue.NotificationQueueItem{
		NotificationID: newNotif.NotificationID,
		Priority:       newNotif.Priority,
		EnqueuedAt:     time.Now(),
	}

	if err := p.queue.EnqueueScheduled(ctx, queueItem, nextRun); err != nil {
		p.logger.Error("Failed to enqueue next recurring notification", zap.Error(err))
		// Not a critical failure as scheduler will pick it up from DB eventually
	}
}
