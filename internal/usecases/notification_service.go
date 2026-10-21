package usecases

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	stdtemplate "text/template"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/domain/topic"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/limiter"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/metrics"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	templateutil "github.com/the-monkeys/freerangenotify/internal/usecases/template"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
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
	channelDelivery interface {
		PrepareChannelDelivery(context.Context, *notification.Notification, *user.User) (*channelconnection.PreparedDelivery, error)
	}
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

func (s *NotificationService) SetChannelDispatch(dispatch interface {
	PrepareChannelDelivery(context.Context, *notification.Notification, *user.User) (*channelconnection.PreparedDelivery, error)
}) {
	s.channelDelivery = dispatch
}

func (s *NotificationService) prepareMessaging(ctx context.Context, n *notification.Notification, u *user.User) error {
	if n.Metadata != nil {
		for _, key := range []string{channelconnection.DeliveryMetadataKey, "channel_destination", "worker_claim", "worker_claim_until", "channel_quote", "reservation_id", "credits_used", "rate_card_version", "tenant_id", "delivery_attempt_state", "delivery_reservation_id", "channel_content_prepared", "inbox_reply_phone", "inbox_reply_contact", "inbox_reply_text", "inbox_reply_template", "inbox_reply_key"} {
			delete(n.Metadata, key)
		}
	}
	if s.channelDelivery != nil && (n.Channel == notification.ChannelSMS || n.Channel == notification.ChannelWhatsApp) {
		if n.Content.Data != nil {
			b, err := json.Marshal(n.Content.Data)
			if err != nil {
				return fmt.Errorf("invalid message data")
			}
			var data map[string]interface{}
			if json.Unmarshal(b, &data) != nil {
				return fmt.Errorf("invalid message data")
			}
			n.Content.Data = data
		}
		if n.TemplateID != "" {
			tmpl, err := s.resolveTemplate(ctx, n.AppID, n.TemplateID)
			if err != nil {
				return err
			}
			appName := ""
			for _, variable := range tmpl.Variables {
				if variable == "product" {
					app, e := s.appRepo.GetByID(ctx, n.AppID)
					if e != nil || app == nil {
						return fmt.Errorf("application unavailable")
					}
					appName = app.AppName
					break
				}
			}
			n.Content.Data = templateutil.EnrichData(tmpl, n.Content.Data, u, appName, n.UserID, n.AppID)
			render := func(text string) (string, error) {
				t, err := stdtemplate.New("message").Parse(NormalizeTemplateBody(text))
				if err != nil {
					return "", fmt.Errorf("invalid messaging template")
				}
				var b bytes.Buffer
				if err := t.Execute(&b, n.Content.Data); err != nil {
					return "", fmt.Errorf("messaging template data missing")
				}
				return b.String(), nil
			}
			title, err := render(tmpl.Subject)
			if err != nil {
				return err
			}
			body, err := render(tmpl.Body)
			if err != nil {
				return err
			}
			n.Content.Title, n.Content.Body = title, body
		}
		if n.Metadata == nil {
			n.Metadata = map[string]interface{}{}
		}
		n.Metadata["channel_content_prepared"] = true
		_, err := s.channelDelivery.PrepareChannelDelivery(ctx, n, u)
		return err
	}
	return nil
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
		zap.String("priority", string(req.Priority)))

	// ── Phase 2: Topic fan-out ──
	// If TopicID is set, resolve subscribers and delegate to SendBulk.
	if req.TopicID != "" {
		return s.sendToTopic(ctx, req)
	}

	// 2.2: Resolve user_id if it's not a UUID (email/external ID → internal UUID)
	if req.UserID != "" {
		resolvedID, err := s.resolveUserID(ctx, req.AppID, req.UserID)
		if err != nil {
			return nil, err
		}
		req.UserID = resolvedID
	}

	var title, body string
	var tmpl *template.Template

	// 2.3: Resolve template by name or UUID if provided
	if req.TemplateID != "" {
		var err error
		tmpl, err = s.resolveTemplate(ctx, req.AppID, req.TemplateID)
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
			zap.String("template_name", tmpl.Name))
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

	// Fast-fail for webhook-like sends with no resolvable destination.
	// Otherwise, the worker will dequeue and fail later with "no webhook URL found",
	// which is slow and confusing for users.
	if req.UserID == "" && notification.IsWebhookLikeChannel(req.Channel) {
		hasURL := false
		hasTarget := false
		if req.Data != nil {
			if v, ok := req.Data["webhook_url"].(string); ok && strings.TrimSpace(v) != "" {
				hasURL = true
			}
			if v, ok := req.Data["webhook_target"].(string); ok && strings.TrimSpace(v) != "" {
				hasTarget = true
			}
		}

		// Template pinning: a webhook template may specify a default target.
		if !hasTarget && tmpl != nil && strings.TrimSpace(tmpl.WebhookTarget) != "" {
			hasTarget = true
		}

		// Application-level default webhook URL(s) are also acceptable.
		appHasDefaultWebhook := strings.TrimSpace(app.WebhookURL) != "" || (app.Webhooks != nil && len(app.Webhooks) > 0)

		if !hasURL && !hasTarget && !appHasDefaultWebhook {
			return nil, fmt.Errorf("webhook destination required: set webhook_target (recommended), webhook_url, template.webhook_target, or app webhook_url")
		}
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

	if req.UserID == "" && notification.IsWebhookLikeChannel(req.Channel) {
		// Log that we are processing an anonymous webhook
		s.logger.Debug("Processing anonymous webhook", zap.String("app_id", req.AppID), zap.String("channel", string(req.Channel)))
	} else {
		u, err = s.userRepo.GetByID(ctx, req.UserID)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, errors.NotFound("user", req.UserID)
			}
			s.logger.Error("Failed to get user", zap.String("user_id", req.UserID), zap.Error(err))
			return nil, fmt.Errorf("user not found: %w", err)
		}

		// Channel-specific contact validation (skip instead of error)
		switch req.Channel {
		case notification.ChannelSMS, notification.ChannelWhatsApp:
			if contact := notification.NormalizePhone(u.Phone); contact == "" {
				s.logger.Warn("Skipping send: missing phone for SMS/WhatsApp",
					zap.String("user_id", req.UserID), zap.String("channel", string(req.Channel)))
				return nil, nil
			}
		case notification.ChannelEmail:
			if contact := notification.NormalizeEmail(u.Email); contact == "" {
				s.logger.Warn("Skipping send: missing email",
					zap.String("user_id", req.UserID))
				return nil, nil
			}
		}

		// Check global DND
		if u.Preferences.DND && req.Priority != notification.PriorityCritical {
			s.logger.Warn("User has DND enabled, blocking non-critical notification",
				zap.String("user_id", req.UserID),
				zap.String("channel", string(req.Channel)),
				zap.String("priority", string(req.Priority)))
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
		EnvironmentID:  req.EnvironmentID,
		UserID:         req.UserID,
		Channel:        req.Channel,
		Priority:       req.Priority,
		Status:         notification.StatusPending,
		Content: notification.Content{
			Title:       title,
			Body:        body,
			Data:        req.Data,
			MediaURL:    req.MediaURL,
			Attachments: req.Attachments,
			Actions:     req.Actions,
			Fields:      req.Fields,
			Mentions:    req.Mentions,
			Poll:        req.Poll,
			Style:       req.Style,
		},
		Category:    req.Category,
		TemplateID:  req.TemplateID,
		ScheduledAt: req.ScheduledAt,
		RetryCount:  0,
		Recurrence:  req.Recurrence,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if identity, ok := notification.DispatchIdentityFromContext(ctx); ok {
		notif.NotificationID = identity
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
	if err := s.prepareMessaging(ctx, notif, u); err != nil {
		return nil, err
	}
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
		if _, trusted := notification.DispatchIdentityFromContext(ctx); trusted {
			return nil, &notification.DispatchError{NotificationID: notif.NotificationID, MayBeEnqueued: true, Cause: err}
		}
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
			_, err := s.workflowService.Trigger(channelconnection.WithEnvironment(context.Background(), req.EnvironmentID), req.AppID, &workflow.TriggerRequest{
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
		MediaURL:    req.MediaURL,
		Attachments: req.Attachments,
		Actions:     req.Actions,
		Fields:      req.Fields,
		Mentions:    req.Mentions,
		Poll:        req.Poll,
		Style:       req.Style,
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
	return s.sendBulk(ctx, req, nil)
}
func (s *NotificationService) sendBulk(ctx context.Context, req notification.BulkSendRequest, profiles map[string]*user.User, reports ...*notification.BroadcastResult) ([]*notification.Notification, error) {
	var report *notification.BroadcastResult
	if len(reports) > 0 {
		report = reports[0]
	}
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	var notifications []*notification.Notification
	if profiles == nil {
		if reader, ok := s.userRepo.(interface {
			GetByIDs(context.Context, []string) (map[string]*user.User, error)
		}); ok {
			var err error
			profiles, err = reader.GetByIDs(ctx, req.UserIDs)
			if err != nil {
				return nil, fmt.Errorf("failed to load bulk recipients: %w", err)
			}
		}
	}
	app, err := s.getCachedApp(ctx, req.AppID)
	if err != nil {
		return nil, err
	}
	seenContacts := make(map[string]struct{})

	// Create notifications for each user
	for _, userID := range req.UserIDs {
		// Check if user exists
		var u *user.User
		var err error
		if profiles != nil {
			u = profiles[userID]
			if u == nil {
				continue
			}
		} else {
			u, err = s.userRepo.GetByID(ctx, userID)
		}
		if err != nil || u == nil {
			s.logger.Warn("User not found in bulk send, skipping",
				zap.String("user_id", userID), zap.Error(err))
			continue
		}

		if u.AppID != req.AppID || (req.EnvironmentID != "" && req.EnvironmentID != "default" && u.EnvironmentID != req.EnvironmentID) {
			continue
		}
		// Normalize contact for channel and skip if missing
		var contactKey string
		switch req.Channel {
		case notification.ChannelSMS, notification.ChannelWhatsApp:
			phone := notification.NormalizePhone(u.Phone)
			if phone == "" {
				s.logger.Debug("Missing phone for SMS/WhatsApp, skipping",
					zap.String("user_id", userID))
				continue
			}
			contactKey = string(req.Channel) + ":" + phone
		case notification.ChannelEmail:
			email := notification.NormalizeEmail(u.Email)
			if email == "" {
				s.logger.Debug("Missing email for email channel, skipping",
					zap.String("user_id", userID))
				continue
			}
			contactKey = string(req.Channel) + ":" + email
		default:
			// other channels: dedup by user id
			contactKey = string(req.Channel) + ":" + u.UserID
		}

		// Deduplicate within this bulk request
		if _, exists := seenContacts[contactKey]; exists {
			continue
		}
		seenContacts[contactKey] = struct{}{}

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

		if req.Channel == notification.ChannelEmail && app.Settings.DailyEmailLimit > 0 {
			allowed, err := s.limiter.IncrementAndCheckDailyLimit(ctx, fmt.Sprintf("app_email_limit:%s", req.AppID), app.Settings.DailyEmailLimit)
			if err != nil {
				return nil, fmt.Errorf("application daily email limiter unavailable: %w", err)
			}
			if !allowed && req.Priority != notification.PriorityCritical {
				continue
			}
		}

		// Create notification
		notif := &notification.Notification{
			NotificationID: uuid.New().String(),
			AppID:          req.AppID,
			EnvironmentID:  req.EnvironmentID,
			UserID:         userID,
			Channel:        req.Channel,
			Priority:       req.Priority,
			Status:         notification.StatusPending,
			Content: notification.Content{
				Title:       req.Title,
				Body:        req.Body,
				Data:        req.Data,
				MediaURL:    req.MediaURL,
				Attachments: req.Attachments,
				Actions:     req.Actions,
				Fields:      req.Fields,
				Mentions:    req.Mentions,
				Poll:        req.Poll,
				Style:       req.Style,
			},
			Category:    req.Category,
			TemplateID:  req.TemplateID,
			ScheduledAt: req.ScheduledAt,
			RetryCount:  0,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		// Promote data.media_url → Content.MediaURL when not set explicitly
		if notif.Content.MediaURL == "" {
			if mu, ok := req.Data["media_url"].(string); ok && mu != "" {
				notif.Content.MediaURL = mu
			}
		}
		if req.Metadata != nil {
			notif.Metadata = make(map[string]interface{})
			for k, v := range req.Metadata {
				notif.Metadata[k] = v
			}
		}

		if !notif.IsScheduled() {
			notif.Status = notification.StatusQueued
		}
		if err := notif.Validate(); err != nil {
			s.logger.Warn("Invalid notification in bulk send, skipping",
				zap.String("user_id", userID), zap.Error(err))
			continue
		}

		if err := s.prepareMessaging(ctx, notif, u); err != nil {
			return nil, err
		}
		notifications = append(notifications, notif)
	}
	confirmed := make([]*notification.Notification, 0, len(notifications))
	if report != nil {
		report.Skipped = len(req.UserIDs) - len(notifications)
	}
	for start := 0; start < len(notifications); start += 500 {
		end := start + 500
		if end > len(notifications) {
			end = len(notifications)
		}
		batch := notifications[start:end]
		var outcomes []error
		if writer, ok := s.notificationRepo.(interface {
			BulkCreateResults(context.Context, []*notification.Notification) ([]error, error)
		}); ok {
			var err error
			outcomes, err = writer.BulkCreateResults(ctx, batch)
			if err != nil {
				recordUnknownBroadcastBatch(report, confirmed, batch, notifications[end:], "storage_unknown")
				return confirmed, fmt.Errorf("notification storage write outcome unknown: %w", err)
			}
			if len(outcomes) != len(batch) {
				recordUnknownBroadcastBatch(report, confirmed, batch, notifications[end:], "storage_unknown")
				return confirmed, fmt.Errorf("notification storage outcomes incomplete")
			}
		} else {
			outcomes = make([]error, len(batch))
			for i, n := range batch {
				outcomes[i] = s.notificationRepo.Create(ctx, n)
			}
		}
		items := make([]queue.NotificationQueueItem, 0, len(batch))
		for i, n := range batch {
			if outcomes[i] != nil {
				if report != nil {
					report.Rejected++
					appendBroadcastOutcome(report, n, "storage_rejected")
				}
				s.logger.Warn("Bulk notification storage rejected", zap.String("notification_id", n.NotificationID), zap.Error(outcomes[i]))
				continue
			}
			confirmed = append(confirmed, n)
			item := queue.NotificationQueueItem{NotificationID: n.NotificationID, AppID: n.AppID, Priority: n.Priority, EnqueuedAt: time.Now()}
			if n.IsScheduled() {
				if err := s.queue.EnqueueScheduled(ctx, item, *n.ScheduledAt); err != nil {
					s.logger.Error("Scheduled publication failed; persisted pending fallback retained", zap.String("notification_id", n.NotificationID), zap.Error(err))
				}
			} else {
				items = append(items, item)
			}
		}
		if len(items) > 0 {
			if err := s.queue.EnqueueBatch(ctx, items); err != nil {
				if report != nil {
					report.Accepted = len(confirmed) - len(items)
					report.Unknown = len(items)
					report.NotAttempted = len(notifications) - end
					report.Partial = true
					for _, n := range confirmed {
						outcome := "accepted"
						if n.IsScheduled() || !containsQueueNotification(items, n.NotificationID) {
							appendBroadcastOutcome(report, n, outcome)
						} else {
							appendBroadcastOutcome(report, n, "publication_unknown")
						}
					}
					for _, n := range notifications[end:] {
						appendBroadcastOutcome(report, n, "not_attempted")
					}
				}
				// Publication may have succeeded despite a lost response. Leave persisted
				// states intact; changing them here can race a worker or cause resends.
				return confirmed, fmt.Errorf("failed to bulk enqueue (publication outcome unknown): %w", err)
			}
		}
	}
	if report != nil {
		report.Accepted = len(confirmed)
		report.Partial = report.Rejected > 0
		if report.Partial {
			for _, n := range confirmed {
				appendBroadcastOutcome(report, n, "accepted")
			}
		}
	}
	s.logger.Info("Bulk notifications prepared", zap.Int("requested", len(req.UserIDs)), zap.Int("persisted", len(confirmed)))
	return confirmed, nil
}

// Broadcast sends a notification to all users of an application (or triggers workflows when WorkflowTriggerID is set).
func (s *NotificationService) Broadcast(ctx context.Context, req notification.BroadcastRequest) (*notification.BroadcastResult, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, err
	}

	var allUserIDs []string
	var profiles map[string]*user.User

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
		profiles = make(map[string]*user.User)
		limit := 1000
		offset := 0
		for {
			users, err := s.userRepo.List(ctx, user.UserFilter{
				AppID:         req.AppID,
				EnvironmentID: req.EnvironmentID,
				Limit:         limit,
				Offset:        offset,
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
				profiles[u.UserID] = u
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
		result, err := s.workflowService.TriggerForUserIDs(channelconnection.WithEnvironment(ctx, req.EnvironmentID), req.AppID, req.WorkflowTriggerID, allUserIDs, req.Data)
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
		AppID:         req.AppID,
		EnvironmentID: req.EnvironmentID,
		UserIDs:       allUserIDs,
		Channel:       req.Channel,
		Priority:      req.Priority,
		Title:         req.Title,
		Body:          req.Body,
		Data:          req.Data,
		TemplateID:    req.TemplateID,
		Category:      req.Category,
		ScheduledAt:   req.ScheduledAt,
		Metadata:      req.Metadata,
		Attachments:   req.Attachments,
		Actions:       req.Actions,
		Fields:        req.Fields,
		Mentions:      req.Mentions,
		Poll:          req.Poll,
		Style:         req.Style,
	}
	report := &notification.BroadcastResult{Total: len(allUserIDs)}
	notifications, err := s.sendBulk(ctx, bulkReq, profiles, report)
	report.Notifications = notifications
	if err != nil && len(notifications) == 0 && report.Unknown == 0 {
		report.NotAttempted = report.Total - report.Skipped - report.Rejected
	}
	return report, err
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

// Count returns the total number of notifications matching the filter.
func (s *NotificationService) Count(ctx context.Context, filter notification.NotificationFilter) (int64, error) {
	if err := filter.Validate(); err != nil {
		return 0, err
	}
	return s.notificationRepo.Count(ctx, &filter)
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

	return "", errors.NotFound("user", identifier).
		WithMetadata("hint", "use a valid user_id, email address, external_id, or internal UUID")
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
	case "whatsapp":
		userPref = u.Preferences.WhatsAppEnabled
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
		case "whatsapp":
			appPref = app.Settings.DefaultPreferences.WhatsAppEnabled
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
