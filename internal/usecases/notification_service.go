package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/metrics"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"go.uber.org/zap"
)

// NotificationService implements the notification service interface
type NotificationService struct {
	notificationRepo notification.Repository
	userRepo         user.Repository
	queue            queue.Queue
	logger           *zap.Logger
	maxRetries       int
	metrics          *metrics.NotificationMetrics
}

// NotificationServiceConfig holds configuration for the notification service
type NotificationServiceConfig struct {
	MaxRetries int
}

// NewNotificationService creates a new notification service
func NewNotificationService(
	notificationRepo notification.Repository,
	userRepo user.Repository,
	queue queue.Queue,
	logger *zap.Logger,
	config NotificationServiceConfig,
	metrics *metrics.NotificationMetrics,
) notification.Service {
	return &NotificationService{
		notificationRepo: notificationRepo,
		userRepo:         userRepo,
		queue:            queue,
		logger:           logger,
		maxRetries:       config.MaxRetries,
		metrics:          metrics,
	}
}

// Send sends a notification to a user
func (s *NotificationService) Send(ctx context.Context, req notification.SendRequest) (*notification.Notification, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Check if user exists
	u, err := s.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		s.logger.Error("Failed to get user", zap.String("user_id", req.UserID), zap.Error(err))
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Validate channel is enabled in user preferences
	if !s.isChannelEnabled(u, req.Channel) {
		return nil, fmt.Errorf("channel %s is not enabled for user %s", req.Channel, req.UserID)
	}

	// Check quiet hours
	if s.isQuietHours(u) && req.Priority != notification.PriorityCritical {
		return nil, fmt.Errorf("user is in quiet hours, only critical notifications allowed")
	}

	// Create notification entity
	notif := &notification.Notification{
		NotificationID: uuid.New().String(),
		AppID:          req.AppID,
		UserID:         req.UserID,
		Channel:        req.Channel,
		Priority:       req.Priority,
		Status:         notification.StatusPending,
		Content: notification.Content{
			Title: req.Title,
			Body:  req.Body,
			Data:  req.Data,
		},
		TemplateID:  req.TemplateID,
		ScheduledAt: req.ScheduledAt,
		RetryCount:  0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Validate notification entity
	if err := notif.Validate(); err != nil {
		return nil, fmt.Errorf("invalid notification: %w", err)
	}

	// Save to database
	if err := s.notificationRepo.Create(ctx, notif); err != nil {
		s.logger.Error("Failed to create notification", zap.Error(err))
		return nil, fmt.Errorf("failed to create notification: %w", err)
	}

	// If scheduled for future, don't enqueue yet
	if notif.IsScheduled() {
		s.logger.Info("Notification scheduled for future delivery",
			zap.String("notification_id", notif.NotificationID),
			zap.Time("scheduled_at", *notif.ScheduledAt))
		return notif, nil
	}

	// Enqueue for immediate processing
	queueItem := queue.NotificationQueueItem{
		NotificationID: notif.NotificationID,
		Priority:       notif.Priority,
		EnqueuedAt:     time.Now(),
	}

	if err := s.queue.Enqueue(ctx, queueItem); err != nil {
		s.logger.Error("Failed to enqueue notification", zap.Error(err))
		// Update status to failed
		s.notificationRepo.UpdateStatus(ctx, notif.NotificationID, notification.StatusFailed)
		return nil, fmt.Errorf("failed to enqueue notification: %w", err)
	}

	// Update status to queued
	if err := s.notificationRepo.UpdateStatus(ctx, notif.NotificationID, notification.StatusQueued); err != nil {
		s.logger.Error("Failed to update notification status", zap.Error(err))
	}

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSend(string(req.Channel), string(notification.StatusQueued), string(req.Priority))
	}

	s.logger.Info("Notification sent successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.String("user_id", req.UserID),
		zap.String("channel", string(req.Channel)))

	return notif, nil
}

// SendBulk sends notifications to multiple users
func (s *NotificationService) SendBulk(ctx context.Context, req notification.BulkSendRequest) ([]*notification.Notification, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	var notifications []*notification.Notification
	var queueItems []queue.NotificationQueueItem

	// Create notifications for each user
	for _, userID := range req.UserIDs {
		// Check if user exists
		u, err := s.userRepo.GetByID(ctx, userID)
		if err != nil {
			s.logger.Warn("User not found in bulk send, skipping",
				zap.String("user_id", userID), zap.Error(err))
			continue
		}

		// Check if channel is enabled
		if !s.isChannelEnabled(u, req.Channel) {
			s.logger.Warn("Channel not enabled for user, skipping",
				zap.String("user_id", userID), zap.String("channel", string(req.Channel)))
			continue
		}

		// Check quiet hours (skip non-critical during quiet hours)
		if s.isQuietHours(u) && req.Priority != notification.PriorityCritical {
			s.logger.Debug("User in quiet hours, skipping non-critical notification",
				zap.String("user_id", userID))
			continue
		}

		// Create notification
		notif := &notification.Notification{
			NotificationID: uuid.New().String(),
			AppID:          req.AppID,
			UserID:         userID,
			Channel:        req.Channel,
			Priority:       req.Priority,
			Status:         notification.StatusPending,
			Content: notification.Content{
				Title: req.Title,
				Body:  req.Body,
				Data:  req.Data,
			},
			TemplateID:  req.TemplateID,
			ScheduledAt: req.ScheduledAt,
			RetryCount:  0,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := notif.Validate(); err != nil {
			s.logger.Warn("Invalid notification in bulk send, skipping",
				zap.String("user_id", userID), zap.Error(err))
			continue
		}

		if err := s.notificationRepo.Create(ctx, notif); err != nil {
			s.logger.Error("Failed to create notification in bulk send",
				zap.String("user_id", userID), zap.Error(err))
			continue
		}

		notifications = append(notifications, notif)

		// Queue for immediate delivery if not scheduled
		if !notif.IsScheduled() {
			queueItems = append(queueItems, queue.NotificationQueueItem{
				NotificationID: notif.NotificationID,
				Priority:       notif.Priority,
				EnqueuedAt:     time.Now(),
			})
		}
	}

	// Bulk enqueue
	if len(queueItems) > 0 {
		if err := s.queue.EnqueueBatch(ctx, queueItems); err != nil {
			s.logger.Error("Failed to bulk enqueue notifications", zap.Error(err))
			return notifications, fmt.Errorf("failed to bulk enqueue: %w", err)
		}

		// Bulk update status to queued
		var notifIDs []string
		for _, item := range queueItems {
			notifIDs = append(notifIDs, item.NotificationID)
		}
		if err := s.notificationRepo.BulkUpdateStatus(ctx, notifIDs, notification.StatusQueued); err != nil {
			s.logger.Error("Failed to bulk update notification status", zap.Error(err))
		}
	}

	s.logger.Info("Bulk notifications sent",
		zap.Int("total", len(req.UserIDs)),
		zap.Int("sent", len(notifications)))

	return notifications, nil
}

// Get retrieves a notification by ID
func (s *NotificationService) Get(ctx context.Context, notificationID, appID string) (*notification.Notification, error) {
	notif, err := s.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		return nil, err
	}

	// Verify the notification belongs to the app
	if notif.AppID != appID {
		return nil, fmt.Errorf("notification not found")
	}

	return notif, nil
}

// List retrieves notifications with filtering
func (s *NotificationService) List(ctx context.Context, filter notification.NotificationFilter) ([]*notification.Notification, error) {
	// Validate filter
	if err := filter.Validate(); err != nil {
		return nil, err
	}

	return s.notificationRepo.List(ctx, &filter)
}

// UpdateStatus updates the status of a notification
func (s *NotificationService) UpdateStatus(ctx context.Context, notificationID string, status notification.Status, errorMessage string) error {
	// Validate status
	if !status.Valid() {
		return notification.ErrInvalidStatus
	}

	// Get existing notification
	notif, err := s.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		return err
	}

	// Validate status transition
	if notif.Status.IsFinal() {
		return fmt.Errorf("cannot update status of notification in final state %s", notif.Status)
	}

	// Update status in repository (repository handles timestamps)
	err = s.notificationRepo.UpdateStatus(ctx, notificationID, status)
	if err != nil {
		return err
	}

	// If there's an error message, update it separately
	if errorMessage != "" {
		notif.ErrorMessage = errorMessage
		return s.notificationRepo.Update(ctx, notif)
	}

	return nil
}

// Cancel cancels a scheduled notification
func (s *NotificationService) Cancel(ctx context.Context, notificationID, appID string) error {
	notif, err := s.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		return err
	}

	// Verify ownership
	if notif.AppID != appID {
		return fmt.Errorf("notification not found")
	}

	// Check if notification can be cancelled
	if notif.Status == notification.StatusSent || notif.Status == notification.StatusDelivered {
		return notification.ErrCannotCancelSent
	}

	if notif.Status.IsFinal() {
		return fmt.Errorf("cannot cancel notification in final state %s", notif.Status)
	}

	return s.notificationRepo.UpdateStatus(ctx, notificationID, notification.StatusCancelled)
}

// Retry retries a failed notification
func (s *NotificationService) Retry(ctx context.Context, notificationID, appID string) error {
	notif, err := s.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		return err
	}

	// Verify ownership
	if notif.AppID != appID {
		return fmt.Errorf("notification not found")
	}

	// Check if notification can be retried
	if !notif.CanRetry(s.maxRetries) {
		if notif.RetryCount >= s.maxRetries {
			return notification.ErrMaxRetriesExceeded
		}
		return notification.ErrCannotRetry
	}

	// Increment retry count
	if err := s.notificationRepo.IncrementRetryCount(ctx, notificationID, ""); err != nil {
		return fmt.Errorf("failed to increment retry count: %w", err)
	}

	// Re-enqueue
	queueItem := queue.NotificationQueueItem{
		NotificationID: notif.NotificationID,
		Priority:       notif.Priority,
		RetryCount:     notif.RetryCount + 1,
		EnqueuedAt:     time.Now(),
	}

	if err := s.queue.Enqueue(ctx, queueItem); err != nil {
		return fmt.Errorf("failed to re-enqueue notification: %w", err)
	}

	// Update status back to queued
	if err := s.notificationRepo.UpdateStatus(ctx, notificationID, notification.StatusQueued); err != nil {
		s.logger.Error("Failed to update status after retry", zap.Error(err))
	}

	s.logger.Info("Notification retried",
		zap.String("notification_id", notificationID),
		zap.Int("retry_count", notif.RetryCount+1))

	return nil
}

// isChannelEnabled checks if a channel is enabled for the user
func (s *NotificationService) isChannelEnabled(u *user.User, channel notification.Channel) bool {
	switch channel {
	case notification.ChannelEmail:
		return u.Preferences.EmailEnabled
	case notification.ChannelPush:
		return u.Preferences.PushEnabled
	case notification.ChannelSMS:
		return u.Preferences.SMSEnabled
	default:
		return true // Unknown channels default to enabled
	}
}

// isQuietHours checks if the user is in quiet hours
func (s *NotificationService) isQuietHours(u *user.User) bool {
	// If quiet hours not configured (empty strings), not in quiet hours
	if u.Preferences.QuietHours.Start == "" || u.Preferences.QuietHours.End == "" {
		return false
	}

	now := time.Now()
	if u.Timezone != "" {
		loc, err := time.LoadLocation(u.Timezone)
		if err != nil {
			s.logger.Warn("Invalid timezone for user", zap.String("user_id", u.UserID), zap.String("timezone", u.Timezone))
		} else {
			now = now.In(loc)
		}
	}

	currentTime := now.Format("15:04")
	start := u.Preferences.QuietHours.Start
	end := u.Preferences.QuietHours.End

	// Handle quiet hours spanning midnight
	if start < end {
		return currentTime >= start && currentTime < end
	}
	return currentTime >= start || currentTime < end
}
