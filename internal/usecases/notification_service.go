package usecases

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/domain/topic"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/limiter"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/metrics"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"github.com/the-monkeys/freerangenotify/pkg/utils"
	"go.uber.org/zap"
)

// appCacheEntry stores a cached application with expiry time.
type appCacheEntry struct {
	app       *application.Application
	expiresAt time.Time
}

// NotificationService implements the notification service interface
type NotificationService struct {
	notificationRepo notification.Repository
	userRepo         user.Repository
	appRepo          application.Repository
	templateRepo     template.Repository
	queue            queue.Queue
	logger           *zap.Logger
	maxRetries       int
	metrics          *metrics.NotificationMetrics
	limiter          limiter.Limiter
	appCache         map[string]*appCacheEntry
	appCacheMu       sync.RWMutex
	topicService     topic.Service    // Phase 2: optional, set via SetTopicService
	workflowService  workflow.Service // Phase 2: optional, set via SetWorkflowService
}

// SetTopicService injects the optional topic service for fan-out support.
// Uses setter injection to avoid circular dependency (TopicService is optional).
func (s *NotificationService) SetTopicService(ts topic.Service) {
	s.topicService = ts
}

// SetWorkflowService injects the optional workflow service for broadcast→workflow.
func (s *NotificationService) SetWorkflowService(ws workflow.Service) {
	s.workflowService = ws
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
	templateRepo template.Repository,
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
		templateRepo:     templateRepo,
		queue:            queue,
		logger:           logger,
		maxRetries:       config.MaxRetries,
		metrics:          metrics,
		limiter:          l,
		appCache:         make(map[string]*appCacheEntry),
	}
}

// Send sends a notification to a user
func (s *NotificationService) Send(ctx context.Context, req notification.SendRequest) (*notification.Notification, error) {
	// Debug: Log incoming send request
	s.logger.Debug("Notification send request received",
		zap.String("app_id", req.AppID),
		zap.String("user_id", req.UserID),
		zap.String("template_id", req.TemplateID),
		zap.String("channel", string(req.Channel)),
		zap.String("priority", string(req.Priority)),
		zap.Any("data", req.Data))

	// ── Phase 2: Topic fan-out ──
	// If TopicID is set, resolve subscribers and delegate to SendBulk.
	if req.TopicID != "" {
		return s.sendToTopic(ctx, req)
	}

	// 2.2: Resolve user_id if it's not a UUID (email/external ID → internal UUID)
	if req.UserID != "" {
		resolvedID, err := s.resolveUserID(ctx, req.AppID, req.UserID)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve user: %w", err)
		}
		req.UserID = resolvedID
	}

	var title, body string

	// 2.3: Resolve template by name or UUID if provided
	if req.TemplateID != "" {
		tmpl, err := s.resolveTemplate(ctx, req.AppID, req.TemplateID)
		if err != nil {
			s.logger.Error("Template not found", zap.String("template_id", req.TemplateID), zap.Error(err))
			return nil, notification.ErrTemplateNotFound
		}
		// Update TemplateID to resolved UUID for downstream consistency
		req.TemplateID = tmpl.ID

		// 2.4: Infer channel from template if not explicitly set
		if req.Channel == "" {
			req.Channel = notification.Channel(tmpl.Channel)
			s.logger.Debug("Inferred channel from template",
				zap.String("template_id", tmpl.ID),
				zap.String("channel", string(req.Channel)))
		}

		title = tmpl.Subject
		body = tmpl.Body

		s.logger.Debug("Template loaded for notification",
			zap.String("template_id", req.TemplateID),
			zap.String("template_name", tmpl.Name),
			zap.String("title", title),
			zap.String("body", body))
	} else {
		title = req.Title
		body = req.Body
	}

	// Validate request (after resolution so channel/user/template are populated)
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Use req.Title/req.Body when both provided (e.g. digest flush with fixed text); otherwise use template
	if req.Title != "" && req.Body != "" {
		title = req.Title
		body = req.Body
	}

	// Check if user exists (only if UserID is present)
	var u *user.User

	// Fetch application to check limits and settings
	app, err := s.appRepo.GetByID(ctx, req.AppID)
	if err != nil {
		s.logger.Error("Failed to fetch application", zap.String("app_id", req.AppID), zap.Error(err))
		return nil, fmt.Errorf("application not found: %w", err)
	}

	// Check application daily email limit
	if req.Channel == notification.ChannelEmail && app.Settings.DailyEmailLimit > 0 {
		allowed, err := s.limiter.IncrementAndCheckDailyLimit(ctx, fmt.Sprintf("app_email_limit:%s", req.AppID), app.Settings.DailyEmailLimit)
		if err != nil {
			s.logger.Error("Failed to check application daily email limit", zap.String("app_id", req.AppID), zap.Error(err))
		} else if !allowed && req.Priority != notification.PriorityCritical {
			return nil, fmt.Errorf("application daily email limit exceeded")
		}
	}

	if req.UserID == "" && req.Channel == notification.ChannelWebhook {
		// Log that we are processing an anonymous webhook
		s.logger.Debug("Processing anonymous webhook", zap.String("app_id", req.AppID))
	} else {
		u, err = s.userRepo.GetByID(ctx, req.UserID)
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
		if utils.IsQuietHours(u) && req.Priority != notification.PriorityCritical {
			return nil, notification.ErrQuietHours
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
			Title:    title,
			Body:     body,
			Data:     req.Data,
			MediaURL: req.MediaURL,
		},
		Category:    req.Category,
		TemplateID:  req.TemplateID,
		ScheduledAt: req.ScheduledAt,
		RetryCount:  0,
		Recurrence:  req.Recurrence,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Move webhook_url from Data to Metadata if present
	if req.Data != nil {
		if url, ok := req.Data["webhook_url"].(string); ok {
			if notif.Metadata == nil {
				notif.Metadata = make(map[string]interface{})
			}
			notif.Metadata["webhook_url"] = url
			// Remove from Content.Data to keep payload clean
			delete(notif.Content.Data, "webhook_url")
		}
	}
	// Copy request metadata (e.g. digest_key for digest batching)
	if req.Metadata != nil {
		if notif.Metadata == nil {
			notif.Metadata = make(map[string]interface{})
		}
		for k, v := range req.Metadata {
			notif.Metadata[k] = v
		}
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
			AppID:          notif.AppID,
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
		AppID:          notif.AppID,
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

	// Phase 3: After-send workflow trigger (fire-and-forget, single user only)
	if req.WorkflowTriggerID != "" && req.UserID != "" && s.workflowService != nil {
		go func() {
			payload := req.Data
			if payload == nil {
				payload = make(map[string]interface{})
			}
			payload["notification_id"] = notif.NotificationID
			payload["channel"] = string(req.Channel)
			payload["template_id"] = req.TemplateID
			_, err := s.workflowService.Trigger(context.Background(), req.AppID, &workflow.TriggerRequest{
				TriggerID: req.WorkflowTriggerID,
				UserID:    req.UserID,
				Payload:   payload,
			})
			if err != nil {
				s.logger.Warn("Workflow trigger after send failed (non-fatal)",
					zap.String("trigger_id", req.WorkflowTriggerID),
					zap.String("user_id", req.UserID),
					zap.Error(err))
			}
		}()
	}

	s.logger.Info("Notification sent successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.String("user_id", req.UserID),
		zap.String("channel", string(req.Channel)))

	return notif, nil
}

// sendToTopic resolves topic subscribers and fans out via SendBulk.
func (s *NotificationService) sendToTopic(ctx context.Context, req notification.SendRequest) (*notification.Notification, error) {
	if s.topicService == nil {
		return nil, fmt.Errorf("topics feature is not enabled")
	}

	userIDs, err := s.topicService.GetSubscriberUserIDs(ctx, req.TopicID, req.AppID)
	if err != nil {
		return nil, fmt.Errorf("failed to get topic subscribers: %w", err)
	}
	if len(userIDs) == 0 {
		return nil, fmt.Errorf("topic has no subscribers")
	}

	s.logger.Info("Fanning out notification to topic subscribers",
		zap.String("topic_id", req.TopicID),
		zap.String("app_id", req.AppID),
		zap.Int("subscriber_count", len(userIDs)))

	bulkReq := notification.BulkSendRequest{
		AppID:       req.AppID,
		UserIDs:     userIDs,
		Channel:     req.Channel,
		Priority:    req.Priority,
		Title:       req.Title,
		Body:        req.Body,
		Data:        req.Data,
		TemplateID:  req.TemplateID,
		Category:    req.Category,
		ScheduledAt: req.ScheduledAt,
		Metadata:    req.Metadata,
	}

	results, err := s.SendBulk(ctx, bulkReq)
	if err != nil {
		return nil, err
	}

	// Return the first notification as representative
	if len(results) > 0 {
		return results[0], nil
	}
	return nil, nil
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
		if utils.IsQuietHours(u) && req.Priority != notification.PriorityCritical {
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
		if req.Metadata != nil {
			notif.Metadata = make(map[string]interface{})
			for k, v := range req.Metadata {
				notif.Metadata[k] = v
			}
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
			AppID:          notif.AppID,
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

// Broadcast sends a notification to all users of an application (or triggers workflows when WorkflowTriggerID is set).
func (s *NotificationService) Broadcast(ctx context.Context, req notification.BroadcastRequest) (*notification.BroadcastResult, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	var allUserIDs []string

	if req.TopicKey != "" {
		// Resolve recipients from topic subscribers
		if s.topicService == nil {
			return nil, fmt.Errorf("topics feature is not enabled")
		}
		topicObj, err := s.topicService.GetByKey(ctx, req.AppID, req.TopicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to get topic by key %q: %w", req.TopicKey, err)
		}
		allUserIDs, err = s.topicService.GetSubscriberUserIDs(ctx, topicObj.ID, req.AppID)
		if err != nil {
			return nil, fmt.Errorf("failed to get topic subscribers: %w", err)
		}
		if len(allUserIDs) == 0 {
			return nil, fmt.Errorf("topic %q has no subscribers", req.TopicKey)
		}
		s.logger.Info("Broadcast targeting topic subscribers",
			zap.String("app_id", req.AppID),
			zap.String("topic_key", req.TopicKey),
			zap.Int("user_count", len(allUserIDs)))
	} else {
		// Fetch all users for the application
		limit := 100
		offset := 0
		for {
			users, err := s.userRepo.List(ctx, user.UserFilter{
				AppID:  req.AppID,
				Limit:  limit,
				Offset: offset,
			})
			if err != nil {
				s.logger.Error("Failed to list users for broadcast", zap.Error(err))
				return nil, fmt.Errorf("failed to list users: %w", err)
			}
			if len(users) == 0 {
				break
			}
			for _, u := range users {
				allUserIDs = append(allUserIDs, u.UserID)
			}
			if len(users) < limit {
				break
			}
			offset += limit
		}
		if len(allUserIDs) == 0 {
			return nil, fmt.Errorf("no users found for application %s", req.AppID)
		}
		s.logger.Info("Starting broadcast to all users",
			zap.String("app_id", req.AppID),
			zap.Int("user_count", len(allUserIDs)))
	}

	// Workflow trigger path
	if req.WorkflowTriggerID != "" {
		if s.workflowService == nil {
			return nil, fmt.Errorf("workflows feature is not enabled")
		}
		result, err := s.workflowService.TriggerForUserIDs(ctx, req.AppID, req.WorkflowTriggerID, allUserIDs, req.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to trigger workflows: %w", err)
		}
		s.logger.Info("Broadcast triggered workflows",
			zap.String("app_id", req.AppID),
			zap.String("trigger_id", req.WorkflowTriggerID),
			zap.Int("triggered", result.Triggered))
		return &notification.BroadcastResult{Triggered: result.Triggered}, nil
	}

	// Standard notification broadcast
	bulkReq := notification.BulkSendRequest{
		AppID:       req.AppID,
		UserIDs:     allUserIDs,
		Channel:     req.Channel,
		Priority:    req.Priority,
		Title:       req.Title,
		Body:        req.Body,
		Data:        req.Data,
		TemplateID:  req.TemplateID,
		Category:    req.Category,
		ScheduledAt: req.ScheduledAt,
		Metadata:    req.Metadata,
	}
	notifications, err := s.SendBulk(ctx, bulkReq)
	if err != nil {
		return nil, err
	}
	return &notification.BroadcastResult{Notifications: notifications}, nil
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
			// Log error and return a failed notification item
			s.logger.Error("Failed to send notification in batch",
				zap.String("user_id", req.UserID),
				zap.Error(err))

			n = &notification.Notification{
				AppID:        req.AppID,
				UserID:       req.UserID,
				Channel:      req.Channel,
				Priority:     req.Priority,
				Status:       notification.StatusFailed,
				ErrorMessage: err.Error(),
			}
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
func (s *NotificationService) UpdateStatus(ctx context.Context, notificationID string, status notification.Status, errorMessage string, appID string) error {
	// Validate status
	if !status.Valid() {
		return notification.ErrInvalidStatus
	}

	// Get existing notification
	notif, err := s.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		return err
	}

	// Verify ownership
	if notif.AppID != appID {
		return fmt.Errorf("notification not found")
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

	if err := s.notificationRepo.UpdateStatus(ctx, notificationID, notification.StatusCancelled); err != nil {
		return err
	}
	// Remove from scheduled queue so it won't be sent when due
	_ = s.queue.RemoveScheduledByID(ctx, []string{notificationID})
	return nil
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

	if err := s.notificationRepo.BulkUpdateStatus(ctx, validIDs, notification.StatusCancelled); err != nil {
		return err
	}
	// Remove from scheduled queue so they won't be sent when due
	_ = s.queue.RemoveScheduledByID(ctx, validIDs)
	return nil
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

	// Fetch Application to check retry limit settings
	app, err := s.appRepo.GetByID(ctx, notif.AppID)
	retryLimit := s.maxRetries
	if err == nil {
		if app.Settings.RetryAttempts > 0 {
			retryLimit = app.Settings.RetryAttempts
		}
	} else {
		s.logger.Warn("Failed to fetch application config for retry limit, using default", zap.Error(err))
	}

	// Check if notification can be retried
	if !notif.CanRetry(retryLimit) {
		if notif.RetryCount >= retryLimit {
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
		AppID:          notif.AppID,
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

// FlushQueued re-enqueues all queued notifications for a user for immediate processing
func (s *NotificationService) FlushQueued(ctx context.Context, userID string) error {
	filter := notification.NotificationFilter{
		UserID:   userID,
		Status:   notification.StatusQueued,
		PageSize: 100, // Reasonable batch size
	}

	// Important: We need a way to find StatusQueued across all sources.
	// NotificationRepository.List with these filters will look into Elasticsearch.
	notifications, err := s.notificationRepo.List(ctx, &filter)
	if err != nil {
		return fmt.Errorf("failed to list queued notifications for flush: %w", err)
	}

	if len(notifications) == 0 {
		return nil
	}

	s.logger.Info("Flushing queued notifications for user",
		zap.String("user_id", userID),
		zap.Int("count", len(notifications)))

	for _, notif := range notifications {
		queueItem := queue.NotificationQueueItem{
			NotificationID: notif.NotificationID,
			AppID:          notif.AppID,
			Priority:       notif.Priority,
			RetryCount:     notif.RetryCount,
			EnqueuedAt:     time.Now(),
		}

		// Use EnqueuePriority to jump to the front of the queue
		if err := s.queue.EnqueuePriority(ctx, queueItem); err != nil {
			s.logger.Error("Failed to re-enqueue notification for flush",
				zap.String("notification_id", notif.NotificationID),
				zap.Error(err))
		}
	}

	return nil
}

// resolveUserID converts an email, external user_id, or internal UUID to an internal user ID.
// Priority: UUID (passthrough) → email lookup → external_id lookup → user_id lookup (ES document ID).
func (s *NotificationService) resolveUserID(ctx context.Context, appID, identifier string) (string, error) {
	// If it parses as a UUID, use directly
	if _, err := uuid.Parse(identifier); err == nil {
		return identifier, nil
	}

	// Try email lookup
	if strings.Contains(identifier, "@") {
		u, err := s.userRepo.GetByEmail(ctx, appID, identifier)
		if err == nil {
			return u.UserID, nil
		}
	}

	// Try external_id lookup
	u, err := s.userRepo.GetByExternalID(ctx, appID, identifier)
	if err == nil {
		return u.UserID, nil
	}

	// Try direct lookup by user_id (external identifier stored as ES document ID)
	u, err = s.userRepo.GetByID(ctx, identifier)
	if err == nil && u.AppID == appID {
		return u.UserID, nil
	}

	return "", fmt.Errorf("user %q not found; use a valid user_id, email address, external_id, or internal UUID", identifier)
}

// resolveTemplate resolves a template reference by name or UUID.
// It tries UUID first, then name with locale "en", then name with empty locale.
func (s *NotificationService) resolveTemplate(ctx context.Context, appID, ref string) (*template.Template, error) {
	if ref == "" {
		return nil, notification.ErrTemplateNotFound
	}

	// Try UUID first
	if _, err := uuid.Parse(ref); err == nil {
		tmpl, err := s.templateRepo.GetByID(ctx, ref)
		if err == nil {
			return tmpl, nil
		}
	}

	// Try by name with default locale "en"
	tmpl, err := s.templateRepo.GetByAppAndName(ctx, appID, ref, "en")
	if err == nil {
		return tmpl, nil
	}

	// Try by name with empty locale (any locale)
	tmpl, err = s.templateRepo.GetByAppAndName(ctx, appID, ref, "")
	if err == nil {
		return tmpl, nil
	}

	return nil, notification.ErrTemplateNotFound
}

// getCachedApp fetches an application by ID with a short in-memory cache (30s TTL)
// to avoid repeated Elasticsearch queries during bulk operations.
func (s *NotificationService) getCachedApp(ctx context.Context, appID string) (*application.Application, error) {
	s.appCacheMu.RLock()
	if entry, ok := s.appCache[appID]; ok && time.Now().Before(entry.expiresAt) {
		s.appCacheMu.RUnlock()
		return entry.app, nil
	}
	s.appCacheMu.RUnlock()

	app, err := s.appRepo.GetByID(ctx, appID)
	if err != nil {
		return nil, err
	}

	s.appCacheMu.Lock()
	s.appCache[appID] = &appCacheEntry{app: app, expiresAt: time.Now().Add(30 * time.Second)}
	s.appCacheMu.Unlock()

	return app, nil
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

	// 3. Check App Defaults (with in-memory cache to avoid repeated ES queries)
	app, err := s.getCachedApp(ctx, u.AppID)
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

// GetUnreadCount returns the number of unread notifications for a user
func (s *NotificationService) GetUnreadCount(ctx context.Context, userID, appID string) (int64, error) {
	filter := notification.NotificationFilter{
		UserID: userID,
		AppID:  appID,
		Status: notification.StatusSent, // We consider "sent" (delivered to SSE provider) as unread
	}
	return s.notificationRepo.Count(ctx, &filter)
}

// MarkRead marks multiple notifications as read after verifying ownership.
func (s *NotificationService) MarkRead(ctx context.Context, notificationIDs []string, appID, userID string) error {
	for _, id := range notificationIDs {
		notif, err := s.notificationRepo.GetByID(ctx, id)
		if err != nil {
			s.logger.Warn("MarkRead: notification not found, skipping",
				zap.String("notification_id", id), zap.Error(err))
			continue
		}
		if notif.AppID != appID || notif.UserID != userID {
			s.logger.Warn("MarkRead ownership check failed",
				zap.String("notification_id", id),
				zap.String("claimed_user", userID),
				zap.String("actual_user", notif.UserID))
			return fmt.Errorf("notification %s does not belong to user %s", id, userID)
		}
	}
	return s.notificationRepo.BulkUpdateStatus(ctx, notificationIDs, notification.StatusRead)
}

// ListUnread returns unread notifications for a user
func (s *NotificationService) ListUnread(ctx context.Context, userID, appID string) ([]*notification.Notification, error) {
	filter := notification.NotificationFilter{
		UserID:   userID,
		AppID:    appID,
		Status:   notification.StatusSent,
		Page:     1,
		PageSize: 100, // Reasonable limit for unread
	}
	return s.notificationRepo.List(ctx, &filter)
}

// ── Phase 5: Snooze, Archive, Mark-All-Read ────────────────────────

// Snooze defers a notification until the specified time.
func (s *NotificationService) Snooze(ctx context.Context, notificationID, appID string, until time.Time) error {
	notif, err := s.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		return notification.ErrNotificationNotFound
	}
	if notif.AppID != appID {
		return notification.ErrNotificationNotFound
	}
	if notif.Status.IsFinal() || notif.Status == notification.StatusSnoozed {
		return notification.ErrCannotSnooze
	}
	if until.Before(time.Now()) {
		return notification.ErrInvalidSnoozeDuration
	}

	if err := s.notificationRepo.UpdateSnooze(ctx, notificationID, notification.StatusSnoozed, &until); err != nil {
		return err
	}
	// Remove from scheduled queue so it won't fire at scheduled_at
	_ = s.queue.RemoveScheduledByID(ctx, []string{notificationID})
	return nil
}

// Unsnooze immediately removes snooze from a notification.
func (s *NotificationService) Unsnooze(ctx context.Context, notificationID, appID string) error {
	notif, err := s.notificationRepo.GetByID(ctx, notificationID)
	if err != nil {
		return notification.ErrNotificationNotFound
	}
	if notif.AppID != appID {
		return notification.ErrNotificationNotFound
	}
	if notif.Status != notification.StatusSnoozed {
		return notification.ErrCannotSnooze
	}

	// Set status to queued and re-enqueue for delivery
	if err := s.notificationRepo.UpdateSnooze(ctx, notificationID, notification.StatusQueued, nil); err != nil {
		return err
	}
	queueItem := queue.NotificationQueueItem{
		NotificationID: notif.NotificationID,
		AppID:          notif.AppID,
		Priority:       notif.Priority,
		RetryCount:     0,
		EnqueuedAt:     time.Now(),
	}
	return s.queue.Enqueue(ctx, queueItem)
}

// Archive marks notifications as archived for a user.
func (s *NotificationService) Archive(ctx context.Context, notificationIDs []string, appID, userID string) error {
	// Verify ownership of all notifications
	for _, id := range notificationIDs {
		notif, err := s.notificationRepo.GetByID(ctx, id)
		if err != nil {
			s.logger.Warn("Archive: notification not found", zap.String("id", id), zap.Error(err))
			return notification.ErrNotificationNotFound
		}
		if notif.AppID != appID || notif.UserID != userID {
			return fmt.Errorf("notification %s does not belong to user %s", id, userID)
		}
		if notif.Status == notification.StatusArchived {
			continue // already archived, skip
		}
	}
	return s.notificationRepo.BulkArchive(ctx, notificationIDs, time.Now())
}

// MarkAllRead marks all unread notifications as read for a user.
func (s *NotificationService) MarkAllRead(ctx context.Context, userID, appID, category string) error {
	count, err := s.notificationRepo.MarkAllRead(ctx, userID, appID, category)
	if err != nil {
		return err
	}
	s.logger.Info("Marked all notifications as read",
		zap.String("user_id", userID),
		zap.String("app_id", appID),
		zap.Int("count", count))
	return nil
}

// ListSnoozedDue returns notifications whose snooze period has expired.
func (s *NotificationService) ListSnoozedDue(ctx context.Context) ([]*notification.Notification, error) {
	return s.notificationRepo.ListSnoozedDue(ctx, time.Now())
}
