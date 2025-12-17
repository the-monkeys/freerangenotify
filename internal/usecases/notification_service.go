package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/limiter"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/metrics"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"go.uber.org/zap"
)

// NotificationService implements the notification service interface
type NotificationService struct {
	notificationRepo notification.Repository
	userRepo         user.Repository
	appRepo          application.Repository
	queue            queue.Queue
	logger           *zap.Logger
	maxRetries       int
	metrics          *metrics.NotificationMetrics
	limiter          limiter.Limiter
}

// NotificationServiceConfig holds configuration for the notification service
type NotificationServiceConfig struct {
	MaxRetries int
}

// NewNotificationService creates a new notification service
func NewNotificationService(
	notificationRepo notification.Repository,
	userRepo user.Repository,
	appRepo application.Repository,
	queue queue.Queue,
	logger *zap.Logger,
	config NotificationServiceConfig,
	metrics *metrics.NotificationMetrics,
	l limiter.Limiter,
) notification.Service {
	return &NotificationService{
		notificationRepo: notificationRepo,
		userRepo:         userRepo,
		appRepo:          appRepo,
		queue:            queue,
		logger:           logger,
		maxRetries:       config.MaxRetries,
		metrics:          metrics,
		limiter:          l,
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

	// Check global DND
	if u.Preferences.DND && req.Priority != notification.PriorityCritical {
		return nil, notification.ErrDNDEnabled
	}

	// Validate channel is enabled in user preferences (honoring category overrides)
	if !s.isChannelEnabled(ctx, u, req.Channel, req.Category) {
		return nil, fmt.Errorf("channel %s is not enabled for user %s (category: %s)", req.Channel, req.UserID, req.Category)
	}

	// Check quiet hours
	if s.isQuietHours(u) && req.Priority != notification.PriorityCritical {
		return nil, fmt.Errorf("user is in quiet hours, only critical notifications allowed")
	}

	// Check daily limit
	if u.Preferences.DailyLimit > 0 {
		allowed, err := s.limiter.IncrementAndCheckDailyLimit(ctx, fmt.Sprintf("user:%s", req.UserID), u.Preferences.DailyLimit)
		if err != nil {
			s.logger.Error("Failed to check daily limit", zap.Error(err))
		} else if !allowed && req.Priority != notification.PriorityCritical {
			return nil, notification.ErrRateLimitExceeded
		}
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
		Category:    req.Category,
		TemplateID:  req.TemplateID,
		ScheduledAt: req.ScheduledAt,
		RetryCount:  0,
		Recurrence:  req.Recurrence,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Calculate initial schedule for recurring notifications if not provided
	if notif.Recurrence != nil && notif.ScheduledAt == nil {
		next, err := notif.Recurrence.CalculateNextRun(time.Now())
		if err == nil && !next.IsZero() {
			notif.ScheduledAt = &next
		} else if err != nil {
			s.logger.Warn("Failed to calculate initial recurrence", zap.Error(err))
		}
	}

	// If not scheduled, set initial status to Queued
	if !notif.IsScheduled() {
		notif.Status = notification.StatusQueued
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

	// If scheduled for future, enqueue in scheduled queue
	if notif.IsScheduled() {
		s.logger.Info("Notification scheduled for future delivery",
			zap.String("notification_id", notif.NotificationID),
			zap.Time("scheduled_at", *notif.ScheduledAt))

		queueItem := queue.NotificationQueueItem{
			NotificationID: notif.NotificationID,
			Priority:       notif.Priority,
			EnqueuedAt:     time.Now(),
		}

		if err := s.queue.EnqueueScheduled(ctx, queueItem, *notif.ScheduledAt); err != nil {
			s.logger.Error("Failed to enqueue scheduled notification", zap.Error(err))
			// Status remains pending in DB, ES scheduler will eventually pick it up as fallback
		}

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

		// Check global DND
		if u.Preferences.DND && req.Priority != notification.PriorityCritical {
			s.logger.Debug("User has DND enabled, skipping non-critical notification",
				zap.String("user_id", userID))
			continue
		}

		// Check if channel is enabled
		if !s.isChannelEnabled(ctx, u, req.Channel, req.Category) {
			s.logger.Warn("Channel not enabled for user, skipping",
				zap.String("user_id", userID), zap.String("channel", string(req.Channel)), zap.String("category", req.Category))
			continue
		}

		// Check quiet hours (skip non-critical during quiet hours)
		if s.isQuietHours(u) && req.Priority != notification.PriorityCritical {
			s.logger.Debug("User in quiet hours, skipping non-critical notification",
				zap.String("user_id", userID))
			continue
		}

		// Check daily limit
		if u.Preferences.DailyLimit > 0 {
			allowed, err := s.limiter.IncrementAndCheckDailyLimit(ctx, fmt.Sprintf("user:%s", userID), u.Preferences.DailyLimit)
			if err != nil {
				s.logger.Error("Failed to check daily limit", zap.Error(err))
			} else if !allowed && req.Priority != notification.PriorityCritical {
				s.logger.Debug("User exceeded daily limit, skipping", zap.String("user_id", userID))
				continue
			}
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
			Category:    req.Category,
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

		// Queue or schedule
		queueItem := queue.NotificationQueueItem{
			NotificationID: notif.NotificationID,
			Priority:       notif.Priority,
			EnqueuedAt:     time.Now(),
		}

		if notif.IsScheduled() {
			if err := s.queue.EnqueueScheduled(ctx, queueItem, *notif.ScheduledAt); err != nil {
				s.logger.Error("Failed to enqueue scheduled notification in bulk", zap.Error(err))
			}
		} else {
			queueItems = append(queueItems, queueItem)
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

// SendBatch sends multiple distinct notifications
func (s *NotificationService) SendBatch(ctx context.Context, requests []notification.SendRequest) ([]*notification.Notification, error) {
	var notifications []*notification.Notification

	// Process each request
	for _, req := range requests {
		// We call Send for each to reuse logic (validation, quota, etc.)
		// In a production system, this should likely be optimized to bulk fetch users/check quotas
		// but reuse the same careful logic.
		n, err := s.Send(ctx, req)
		if err != nil {
			// Log error but continue with others
			s.logger.Error("Failed to send notification in batch",
				zap.String("user_id", req.UserID),
				zap.Error(err))
			continue
		}
		notifications = append(notifications, n)
	}

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

// CancelBatch cancels multiple scheduled notifications
func (s *NotificationService) CancelBatch(ctx context.Context, notificationIDs []string, appID string) error {
	// 1. Verify ownership and status for each (or bulk).
	// For simplicity and correctness regarding ownership, we fetch them differently or trust UUIDs?
	// No, must check AppID.

	// We iterate for now because ES GetMany + Filter is not exposed in Repo yet.
	// Or we can assume UUIDs are hard to guess, but multi-tenant security requires AppID check.

	var validIDs []string
	for _, id := range notificationIDs {
		notif, err := s.notificationRepo.GetByID(ctx, id)
		if err != nil {
			continue // Skip not found
		}

		if notif.AppID != appID {
			continue // Skip not owned
		}

		if notif.Status.IsFinal() || notif.Status == notification.StatusSent {
			continue // Cannot cancel
		}

		validIDs = append(validIDs, id)
	}

	if len(validIDs) == 0 {
		return nil
	}

	return s.notificationRepo.BulkUpdateStatus(ctx, validIDs, notification.StatusCancelled)
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

// isChannelEnabled checks if a channel is enabled for the user, honoring overrides and defaults
func (s *NotificationService) isChannelEnabled(ctx context.Context, u *user.User, channel notification.Channel, category string) bool {
	// 1. Check category-specific overrides
	if category != "" && u.Preferences.Categories != nil {
		if catPref, exists := u.Preferences.Categories[category]; exists {
			// If category is disabled, notification is not allowed
			if !catPref.Enabled {
				return false
			}
			// If specific channels are enabled for this category, check if requested channel is in there
			if len(catPref.EnabledChannels) > 0 {
				for _, ec := range catPref.EnabledChannels {
					if notification.Channel(ec) == channel {
						return true
					}
				}
				return false // Requested channel not in category's enabled channels
			}
		}
	}

	// 2. Check User Preferences (Explicit overrides)
	var userPref *bool
	switch channel {
	case notification.ChannelEmail:
		userPref = u.Preferences.EmailEnabled
	case notification.ChannelPush:
		userPref = u.Preferences.PushEnabled
	case notification.ChannelSMS:
		userPref = u.Preferences.SMSEnabled
	}

	if userPref != nil {
		return *userPref
	}

	// 3. Check App Defaults
	// We need to fetch the application to get defaults.
	// Optimize: Add caching layer for Application or pass App down from caller if available.
	// For now, we fetch.
	app, err := s.appRepo.GetByID(ctx, u.AppID)
	if err == nil && app.Settings.DefaultPreferences != nil {
		var appPref *bool
		switch channel {
		case notification.ChannelEmail:
			appPref = app.Settings.DefaultPreferences.EmailEnabled
		case notification.ChannelPush:
			appPref = app.Settings.DefaultPreferences.PushEnabled
		case notification.ChannelSMS:
			appPref = app.Settings.DefaultPreferences.SMSEnabled
		}

		if appPref != nil {
			return *appPref
		}
	} else if err != nil {
		// Log but continue to system defaults
		s.logger.Debug("Failed to fetch application for defaults", zap.Error(err))
	}

	// 4. System Defaults (Enabled by default)
	return true
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
