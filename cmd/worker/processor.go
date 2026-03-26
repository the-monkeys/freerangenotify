package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	stdtemplate "text/template"
	"time"
	"unicode"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/limiter"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/metrics"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/orchestrator"
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
	queue            queue.Queue
	notifRepo        notification.Repository
	userRepo         user.Repository
	appRepo          application.Repository
	templateRepo     template.Repository
	authService      auth.Service
	licensingChecker license.Checker
	providerManager  *providers.Manager
	redisClient      *redis.Client
	logger           *zap.Logger
	config           ProcessorConfig
	metrics          *metrics.NotificationMetrics
	digestManager    *orchestrator.DigestManager // Phase 1: optional digest support
	throttle         *limiter.SubscriberThrottle // Phase 2: optional per-subscriber throttle

	wg       sync.WaitGroup
	stopChan chan struct{}
}

// SetDigestManager injects the optional digest manager (Phase 1).
// Uses setter injection to maintain backward compatibility.
func (p *NotificationProcessor) SetDigestManager(dm *orchestrator.DigestManager) {
	p.digestManager = dm
}

// SetSubscriberThrottle injects the optional subscriber throttle (Phase 2).
func (p *NotificationProcessor) SetSubscriberThrottle(t *limiter.SubscriberThrottle) {
	p.throttle = t
}

// NewNotificationProcessor creates a new notification processor
func NewNotificationProcessor(
	q queue.Queue,
	notifRepo notification.Repository,
	userRepo user.Repository,
	appRepo application.Repository,
	templateRepo template.Repository,
	authService auth.Service,
	licensingChecker license.Checker,
	providerManager *providers.Manager,
	redisClient *redis.Client,
	logger *zap.Logger,
	config ProcessorConfig,
	metrics *metrics.NotificationMetrics,
) *NotificationProcessor {
	return &NotificationProcessor{
		queue:            q,
		notifRepo:        notifRepo,
		userRepo:         userRepo,
		appRepo:          appRepo,
		templateRepo:     templateRepo,
		authService:      authService,
		licensingChecker: licensingChecker,
		providerManager:  providerManager,
		redisClient:      redisClient,
		logger:           logger,
		config:           config,
		metrics:          metrics,
		stopChan:         make(chan struct{}),
	}
}

// publishActivity publishes a notification status event to Redis pub/sub
// for the admin activity feed. Fire-and-forget — errors are logged but not returned.
func (p *NotificationProcessor) publishActivity(ctx context.Context, notificationID string, channel string, status string) {
	if p.redisClient == nil {
		return
	}
	event := map[string]string{
		"notification_id": notificationID,
		"channel":         channel,
		"status":          status,
		"timestamp":       time.Now().Format(time.RFC3339),
	}
	data, _ := json.Marshal(event)
	if err := p.redisClient.Publish(ctx, "notification:activity", string(data)).Err(); err != nil {
		p.logger.Debug("Failed to publish activity event", zap.Error(err))
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

	// Phase 5: Start un-snooze loop
	p.wg.Add(1)
	go p.unsnoozeLoop(ctx)

	// Start processing reaper for at-least-once delivery
	p.wg.Add(1)
	go p.processingReaper(ctx)

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
	defer func() {
		if rq, ok := p.queue.(*queue.RedisQueue); ok {
			rq.Acknowledge(ctx, *item)
		}
	}()

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

	// Skip cancelled — do not send
	if notif.Status == notification.StatusCancelled {
		logger.Info("Skipping cancelled notification",
			zap.String("notification_id", notif.NotificationID))
		return
	}

	// Skip snoozed — re-enqueue for snoozed_until so it will be processed when due
	if notif.Status == notification.StatusSnoozed && notif.SnoozedUntil != nil {
		if time.Now().Before(*notif.SnoozedUntil) {
			logger.Info("Skipping snoozed notification, re-queueing for later",
				zap.String("notification_id", notif.NotificationID),
				zap.Time("snoozed_until", *notif.SnoozedUntil))
			if err := p.queue.EnqueueScheduled(ctx, *item, *notif.SnoozedUntil); err != nil {
				logger.Error("Failed to re-enqueue snoozed notification",
					zap.String("notification_id", notif.NotificationID),
					zap.Error(err))
				// Fall through? No — we'd send it. Better to re-enqueue to main queue
				// so it gets another chance. Or leave in processing for requeue. For now just log.
			}
			return
		}
		// snoozed_until has passed — treat as ready, fall through to process
	}

	// ── Phase 1: Digest check (nil-guarded for backward compat) ──
	if p.digestManager != nil {
		rule, digestKeyValue := p.digestManager.MatchesDigestRule(ctx, notif)
		if rule != nil {
			// Notification matches a digest rule — accumulate and skip immediate delivery
			if accErr := p.digestManager.Accumulate(ctx, notif, rule, digestKeyValue); accErr != nil {
				logger.Error("Failed to accumulate digest, falling back to normal delivery",
					zap.Error(accErr))
			} else {
				logger.Info("Notification accumulated for digest",
					zap.String("notification_id", notif.NotificationID),
					zap.String("digest_key", rule.DigestKey),
					zap.String("window", rule.Window))
				return
			}
		}
	}

	// Update status to processing
	if err := p.notifRepo.UpdateStatus(ctx, notif.NotificationID, notification.StatusProcessing); err != nil {
		logger.Error("Failed to update status to processing", zap.Error(err))
	}
	p.publishActivity(ctx, notif.NotificationID, string(notif.Channel), "processing")

	// Get user details (only if UserID is present)
	var usr *user.User
	if notif.UserID != "" {
		usr, err = p.userRepo.GetByID(ctx, notif.UserID)
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

		// ── Phase 2: Per-subscriber throttle check (nil-guarded) ──
		if p.throttle != nil {
			throttleCfg := p.resolveThrottleConfig(ctx, usr, notif)
			allowed, err := p.throttle.Allow(ctx, usr.UserID, string(notif.Channel), throttleCfg)
			if err != nil {
				logger.Warn("Throttle check error, allowing notification", zap.Error(err))
			} else if !allowed {
				logger.Info("Notification throttled for subscriber",
					zap.String("user_id", usr.UserID),
					zap.String("channel", string(notif.Channel)))
				p.notifRepo.UpdateStatus(ctx, notif.NotificationID, notification.StatusCancelled)
				return
			}
		}
	} else if notif.Channel == notification.ChannelWebhook {
		// Anonymous webhook, continue without user
		logger.Debug("Processing anonymous webhook without user record")
	} else {
		// No user ID and not a webhook? Should have been caught by validation, but fail safe here.
		logger.Error("Missing user ID for non-webhook channel")
		p.handleFailure(ctx, notif, item, "missing user id")
		return
	}

	// Fetch template details if template_id is set to enrich content only (avoid duplicating metadata)
	if notif.TemplateID != "" {
		var tmpl *template.Template

		// Try by UUID first; if that fails and it's not a valid UUID, resolve by name
		if _, parseErr := uuid.Parse(notif.TemplateID); parseErr == nil {
			tmpl, err = p.templateRepo.GetByID(ctx, notif.TemplateID)
		} else {
			// template_id is a name, not a UUID — resolve via app+name
			tmpl, err = p.templateRepo.GetByAppAndName(ctx, notif.AppID, notif.TemplateID, "en")
			if err != nil {
				// Fallback: try with empty locale
				tmpl, err = p.templateRepo.GetByAppAndName(ctx, notif.AppID, notif.TemplateID, "")
			}
		}

		if err != nil {
			logger.Warn("Failed to fetch template, continuing without template details",
				zap.String("template_id", notif.TemplateID),
				zap.Error(err))
		} else {
			// Keep template context inside content; do not mirror it into metadata to avoid redundant storage
			if tmpl != nil {
				// Phase 6: Merge control defaults → saved values → payload
				if len(tmpl.Controls) > 0 || len(tmpl.ControlValues) > 0 {
					notif.Content.Data = p.mergeTemplateData(tmpl, notif.Content.Data)
				}

				// Phase 4: Auto-inject unsubscribe_url for newsletter-category templates
				if tmpl.Metadata != nil {
					if cat, ok := tmpl.Metadata["category"].(string); ok && cat == "newsletter" {
						if notif.Content.Data == nil {
							notif.Content.Data = make(map[string]interface{})
						}
						if _, exists := notif.Content.Data["unsubscribe_url"]; !exists {
							notif.Content.Data["unsubscribe_url"] = fmt.Sprintf(
								"https://notify.example.com/v1/unsubscribe?user=%s&app=%s",
								notif.UserID, notif.AppID,
							)
							logger.Debug("Auto-injected unsubscribe_url for newsletter template",
								zap.String("notification_id", notif.NotificationID))
						}
					}
				}

				// Auto-inject name from user email when not specified
				if notif.Content.Data == nil {
					notif.Content.Data = make(map[string]interface{})
				}
				if needNameFromEmail(notif.Content.Data) {
					emailForName := ""
					if usr != nil {
						emailForName = usr.Email
						if emailForName == "" && strings.Contains(usr.ExternalID, "@") {
							emailForName = usr.ExternalID
						}
					}
					if emailForName != "" {
						notif.Content.Data["name"] = nameFromEmail(emailForName)
						logger.Debug("Auto-injected name from email for template",
							zap.String("notification_id", notif.NotificationID),
							zap.String("name", notif.Content.Data["name"].(string)))
					} else {
						notif.Content.Data["name"] = "there"
						logger.Info("Auto-injected fallback name (no user email/external_id)",
							zap.String("notification_id", notif.NotificationID),
							zap.String("user_id", notif.UserID))
					}
				}

				// Auto-inject product and cta_url for welcome_email and similar templates when missing
				hasProductVar := containsString(tmpl.Variables, "product")
				hasCtaURLVar := containsString(tmpl.Variables, "cta_url")
				needProduct := hasProductVar && needTemplateVar(notif.Content.Data, "product")
				needCtaURL := hasCtaURLVar && needTemplateVar(notif.Content.Data, "cta_url")
				if needProduct || needCtaURL {
					frontendURL := os.Getenv("FREERANGE_FRONTEND_URL")
					if frontendURL == "" {
						frontendURL = "http://localhost:3000"
					}
					if needProduct {
						product := ""
						if app, err := p.appRepo.GetByID(ctx, notif.AppID); err == nil && app != nil && app.AppName != "" {
							product = app.AppName
						}
						if product == "" {
							product = "our platform"
						}
						notif.Content.Data["product"] = product
						logger.Debug("Auto-injected product for template",
							zap.String("notification_id", notif.NotificationID),
							zap.String("product", product))
					}
					if needCtaURL {
						ctaURL := frontendURL + "/login"
						notif.Content.Data["cta_url"] = ctaURL
						logger.Debug("Auto-injected cta_url for template",
							zap.String("notification_id", notif.NotificationID),
							zap.String("cta_url", ctaURL))
					}
				}

				// Render template content
				title, err := p.renderTemplate(tmpl.Subject, notif.Content.Data)
				if err != nil {
					logger.Warn("Failed to render template title", zap.Error(err))
					title = tmpl.Subject // Fallback to raw
				}

				body, err := p.renderTemplate(tmpl.Body, notif.Content.Data)
				if err != nil {
					logger.Warn("Failed to render template body", zap.Error(err))
					body = tmpl.Body // Fallback to raw
				}

				notif.Content.Title = title
				notif.Content.Body = body

				// Always populate RenderedNotification for client convenience (e.g. debugging or alternative display)
				// This field is transient and won't be saved to DB/ES due to es:"-" tag
				notif.RenderedNotification = &notification.Content{
					Title: title,
					Body:  body,
					Data:  notif.Content.Data,
				}

				logger.Debug("Template applied",
					zap.String("notification_id", notif.NotificationID),
					zap.String("template_id", notif.TemplateID),
					zap.String("template_name", tmpl.Name),
					zap.String("title", notif.Content.Title),
					zap.String("body", notif.Content.Body))
			} else {
				logger.Warn("Template is NIL in processor, skipping template enrichment",
					zap.String("notification_id", notif.NotificationID),
					zap.String("template_id", notif.TemplateID))
			}

			// Webhook Routing Logic: Resolve target from Application config if Template specifies it
			// Resolve webhook target URL if applicable
			target := ""
			if tmpl != nil && tmpl.WebhookTarget != "" {
				target = tmpl.WebhookTarget
			}

			// Allow override or manual target from data
			if notif.Content.Data != nil {
				if val, ok := notif.Content.Data["webhook_target"].(string); ok && val != "" {
					target = val
				}
			}

			if notif.Channel == notification.ChannelWebhook && target != "" {
				app, err := p.appRepo.GetByID(ctx, notif.AppID)
				if err != nil {
					logger.Error("Failed to fetch application for webhook routing", zap.Error(err))
				} else if app != nil {
					resolved := false

					// 1. Try legacy app.Webhooks map first
					if app.Webhooks != nil {
						if url, ok := app.Webhooks[target]; ok {
							if notif.Metadata == nil {
								notif.Metadata = make(map[string]interface{})
							}
							if _, exists := notif.Metadata["webhook_url"]; !exists {
								notif.Metadata["webhook_url"] = url
								logger.Info("Resolved webhook target from application webhooks map",
									zap.String("notification_id", notif.NotificationID),
									zap.String("target_name", target),
									zap.String("url", url))
								resolved = true
							}
						}
					}

					// 2. Try custom providers (the current storage mechanism)
					if !resolved {
						for _, cp := range app.Settings.CustomProviders {
							if cp.Name == target && cp.Channel == string(notification.ChannelWebhook) && cp.Active {
								if notif.Metadata == nil {
									notif.Metadata = make(map[string]interface{})
								}
								notif.Metadata["webhook_url"] = cp.WebhookURL
								logger.Info("Resolved webhook target from custom provider",
									zap.String("notification_id", notif.NotificationID),
									zap.String("target_name", target),
									zap.String("url", cp.WebhookURL))
								resolved = true
								break
							}
						}
					}

					if !resolved {
						logger.Error("Webhook target not found in application config or custom providers",
							zap.String("notification_id", notif.NotificationID),
							zap.String("target_name", target),
							zap.Int("custom_providers_count", len(app.Settings.CustomProviders)))
					}
				} else {
					logger.Error("Application not found for webhook routing",
						zap.String("notification_id", notif.NotificationID),
						zap.String("app_id", notif.AppID))
				}
			}
		}
	}

	if p.licensingChecker != nil && p.licensingChecker.Enabled() {
		appForCheck, appErr := p.appRepo.GetByID(ctx, notif.AppID)
		if appErr != nil || appForCheck == nil {
			logger.Error("Failed to load app for licensing check",
				zap.String("notification_id", notif.NotificationID),
				zap.String("app_id", notif.AppID),
				zap.Error(appErr))
			p.handleFailure(ctx, notif, item, "license check failed: app not found")
			return
		}

		decision, checkErr := p.licensingChecker.Check(ctx, appForCheck)
		if checkErr != nil {
			logger.Error("Licensing check errored",
				zap.String("notification_id", notif.NotificationID),
				zap.String("app_id", notif.AppID),
				zap.Error(checkErr))
			p.handleFailure(ctx, notif, item, "license check failed")
			return
		}

		if !decision.Allowed {
			errorReason := decision.Reason
			if errorReason == "" {
				errorReason = "license_required"
			}

			logger.Warn("Notification blocked by licensing",
				zap.String("notification_id", notif.NotificationID),
				zap.String("app_id", notif.AppID),
				zap.String("mode", string(decision.Mode)),
				zap.String("state", string(decision.State)),
				zap.String("reason", errorReason),
				zap.String("source", decision.Source))

			p.handleLicenseBlocked(ctx, notif, errorReason)
			return
		}
	}

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

	// Update in-memory status BEFORE persisting the full object,
	// otherwise the subsequent Update() overwrites "sent" back to "queued".
	notif.Status = notification.StatusSent
	now := time.Now()
	notif.SentAt = &now

	// Update status to sent
	if err := p.notifRepo.UpdateStatus(ctx, notif.NotificationID, notification.StatusSent); err != nil {
		logger.Error("Failed to update status to sent", zap.Error(err))
	}
	// Persist rendered content (title/body) so Notification History displays resolved text, not {{.var}}
	if err := p.notifRepo.Update(ctx, notif); err != nil {
		logger.Warn("Failed to persist rendered content after send", zap.Error(err))
	}
	p.publishActivity(ctx, notif.NotificationID, string(notif.Channel), "sent")

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

func (p *NotificationProcessor) handleLicenseBlocked(ctx context.Context, notif *notification.Notification, reason string) {
	notif.Status = notification.StatusFailed
	notif.ErrorMessage = reason
	now := time.Now()
	notif.FailedAt = &now

	if err := p.notifRepo.UpdateStatus(ctx, notif.NotificationID, notification.StatusFailed); err != nil {
		p.logger.Error("Failed to update status to failed for license block",
			zap.String("notification_id", notif.NotificationID),
			zap.Error(err))
	}
	if err := p.notifRepo.Update(ctx, notif); err != nil {
		p.logger.Error("Failed to persist license-blocked notification state",
			zap.String("notification_id", notif.NotificationID),
			zap.Error(err))
	}
	p.publishActivity(ctx, notif.NotificationID, string(notif.Channel), "failed")

	if p.metrics != nil {
		p.metrics.RecordDeliveryFailure(string(notif.Channel), "licensing", reason)
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
	// Send via provider manager
	p.logger.Info("Routing notification to provider",
		zap.String("notification_id", notif.NotificationID),
		zap.String("channel", string(notif.Channel)),
		zap.String("user_id", notif.UserID))

	// If it's an email, inject app-specific credentials only when the app has explicitly
	// configured valid credentials. Otherwise we use default .env SMTP.
	if notif.Channel == notification.ChannelEmail {
		app, err := p.appRepo.GetByID(ctx, notif.AppID)
		if err == nil && app != nil && app.Settings.EmailConfig != nil {
			ec := app.Settings.EmailConfig
			if hasValidEmailConfig(ec) {
				ctx = context.WithValue(ctx, providers.EmailConfigKey, ec)
				p.logger.Debug("Using app email config",
					zap.String("app_id", notif.AppID),
					zap.String("provider_type", ec.ProviderType))
			} else {
				p.logger.Debug("App email config incomplete or system default, using .env SMTP",
					zap.String("app_id", notif.AppID),
					zap.String("provider_type", ec.ProviderType))
			}
		}
	}

	// If it's WhatsApp, inject app-specific Twilio credentials when configured.
	if notif.Channel == notification.ChannelWhatsApp {
		app, err := p.appRepo.GetByID(ctx, notif.AppID)
		isByoc := false
		if err == nil && app != nil && app.Settings.WhatsApp != nil {
			waCfg := app.Settings.WhatsApp
			if waCfg.AccountSID != "" && waCfg.AuthToken != "" {
				ctx = context.WithValue(ctx, providers.WhatsAppConfigKey, waCfg)
				p.logger.Debug("Using app WhatsApp config",
					zap.String("app_id", notif.AppID))
				isByoc = true
			}
		}

		// Phase 3: Phone verification gate for system credentials
		if !isByoc && app != nil {
			adminUser, err := p.authService.GetCurrentUser(ctx, app.AdminUserID)
			if err != nil {
				p.logger.Error("Failed to fetch admin user for WhatsApp phone verification check",
					zap.String("admin_id", app.AdminUserID), zap.Error(err))
				return fmt.Errorf("phone verification check failed")
			}
			if !adminUser.PhoneVerified {
				p.logger.Warn("Blocked system WhatsApp send due to unverified phone",
					zap.String("app_id", app.AppID),
					zap.String("admin_id", app.AdminUserID))
				return fmt.Errorf("phone_verification_required")
			}
		}
	}

	// If it's SMS, inject app-specific Twilio credentials when configured.
	if notif.Channel == notification.ChannelSMS {
		app, err := p.appRepo.GetByID(ctx, notif.AppID)
		isByoc := false
		if err == nil && app != nil && app.Settings.SMS != nil {
			smsCfg := app.Settings.SMS
			if smsCfg.AccountSID != "" && smsCfg.AuthToken != "" {
				ctx = context.WithValue(ctx, providers.SMSConfigKey, smsCfg)
				p.logger.Debug("Using app SMS config",
					zap.String("app_id", notif.AppID))
				isByoc = true
			}
		}

		// Phase 3 block for SMS: Phone verification gate for system credentials
		if !isByoc && app != nil {
			adminUser, err := p.authService.GetCurrentUser(ctx, app.AdminUserID)
			if err != nil {
				p.logger.Error("Failed to fetch admin user for SMS phone verification check",
					zap.String("admin_id", app.AdminUserID), zap.Error(err))
				return fmt.Errorf("phone verification check failed")
			}
			if !adminUser.PhoneVerified {
				p.logger.Warn("Blocked system SMS send due to unverified phone",
					zap.String("app_id", app.AppID),
					zap.String("admin_id", app.AdminUserID))
				return fmt.Errorf("phone_verification_required")
			}
		}
	}

	// Check for provider fallback chains
	app, err := p.appRepo.GetByID(ctx, notif.AppID)
	if err == nil && app != nil && len(app.Settings.ProviderFallbacks) > 0 {
		for _, fb := range app.Settings.ProviderFallbacks {
			if fb.Channel == string(notif.Channel) && len(fb.Providers) > 0 {
				p.logger.Info("Using provider fallback chain",
					zap.String("notification_id", notif.NotificationID),
					zap.String("channel", fb.Channel),
					zap.Strings("providers", fb.Providers))
				result, fbErr := p.providerManager.SendWithFallback(ctx, notif, usr, fb.Providers)
				if fbErr != nil {
					return fmt.Errorf("all fallback providers failed: %w", fbErr)
				}
				if !result.Success {
					return fmt.Errorf("fallback delivery failed: %s", result.ErrorType)
				}
				return nil
			}
		}
	}

	result, err := p.providerManager.Send(ctx, notif, usr)
	if err != nil {
		// Phase 3: If no built-in provider found, try custom providers on the app
		if app != nil && len(app.Settings.CustomProviders) > 0 {
			for _, cp := range app.Settings.CustomProviders {
				if cp.Channel == string(notif.Channel) && cp.Active {
					p.logger.Info("Routing to custom provider",
						zap.String("notification_id", notif.NotificationID),
						zap.String("provider", cp.Name),
						zap.String("channel", cp.Channel))
					customProvider := providers.NewCustomProvider(
						cp.Name, cp.Channel, cp.WebhookURL, cp.SigningKey, cp.Headers, p.logger,
					)
					customResult, customErr := customProvider.Send(ctx, notif, usr)
					if customErr == nil && customResult != nil && customResult.Success {
						return nil
					}
					if customErr != nil {
						p.logger.Warn("Custom provider failed",
							zap.String("provider", cp.Name),
							zap.Error(customErr))
					} else if customResult != nil && !customResult.Success {
						p.logger.Warn("Custom provider delivery unsuccessful",
							zap.String("provider", cp.Name),
							zap.String("error_type", customResult.ErrorType),
							zap.Error(customResult.Error))
					}
				}
			}
		}

		p.logger.Error("Provider manager send failed",
			zap.String("notification_id", notif.NotificationID),
			zap.Error(err))
		return fmt.Errorf("provider send failed: %w", err)
	}

	p.logger.Info("Provider manager send result",
		zap.String("notification_id", notif.NotificationID),
		zap.Bool("success", result.Success),
		zap.String("provider_message_id", result.ProviderMessageID))

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
	maxRetries := p.config.MaxRetries
	// Attempt to fetch app-specific retry limit
	app, err := p.appRepo.GetByID(ctx, notif.AppID)
	if err == nil && app.Settings.RetryAttempts > 0 {
		maxRetries = app.Settings.RetryAttempts
	}

	if notif.RetryCount >= maxRetries {
		// Move to dead letter queue
		redisQueue, ok := p.queue.(*queue.RedisQueue)
		if ok {
			if err := redisQueue.EnqueueDeadLetter(ctx, *item, fmt.Sprintf("max retries exceeded: %s", errorMsg)); err != nil {
				p.logger.Error("Failed to move to dead letter queue", zap.Error(err))
			}
		}

		// Update status to failed
		notif.Status = notification.StatusFailed
		p.notifRepo.UpdateStatus(ctx, notif.NotificationID, notification.StatusFailed)
		p.publishActivity(ctx, notif.NotificationID, string(notif.Channel), "failed")
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
		} else {
			// Update status to queued to reflect it's waiting for retry (and not stuck in processing)
			// This allows visibility that it's active but pending attempt.
			if err := p.notifRepo.UpdateStatus(ctx, notif.NotificationID, notification.StatusQueued); err != nil {
				p.logger.Error("Failed to update status to queued after scheduling retry", zap.Error(err))
			}
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
	// Phase 3: New channels default to enabled (opt-out model).
	// nil means "enabled" — only block if explicitly set to false.
	case notification.ChannelSlack:
		if usr.Preferences.SlackEnabled != nil && !*usr.Preferences.SlackEnabled {
			return false
		}
	case notification.ChannelDiscord:
		if usr.Preferences.DiscordEnabled != nil && !*usr.Preferences.DiscordEnabled {
			return false
		}
	case notification.ChannelWhatsApp:
		if usr.Preferences.WhatsAppEnabled != nil && !*usr.Preferences.WhatsAppEnabled {
			return false
		}
	}

	// Check quiet hours (except for critical notifications)
	if notif.Priority != notification.PriorityCritical {
		if utils.IsQuietHours(usr) {
			return false
		}
	}

	return true
}

// resolveThrottleConfig returns the effective ThrottleConfig for a user+channel.
// User-level overrides take precedence; otherwise fall back to app-level defaults.
func (p *NotificationProcessor) resolveThrottleConfig(ctx context.Context, usr *user.User, notif *notification.Notification) limiter.ThrottleConfig {
	ch := string(notif.Channel)

	// User-level override
	if usr.Preferences.Throttle != nil {
		if tc, ok := usr.Preferences.Throttle[ch]; ok {
			return limiter.ThrottleConfig{MaxPerHour: tc.MaxPerHour, MaxPerDay: tc.MaxPerDay}
		}
	}

	// App-level default
	app, err := p.appRepo.GetByID(ctx, notif.AppID)
	if err == nil && app != nil && app.Settings.SubscriberThrottle != nil {
		if ac, ok := app.Settings.SubscriberThrottle[ch]; ok {
			return limiter.ThrottleConfig{MaxPerHour: ac.MaxPerHour, MaxPerDay: ac.MaxPerDay}
		}
	}

	return limiter.ThrottleConfig{} // no throttle
}

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

			pending, err := p.notifRepo.GetPending(ctx)
			if err != nil {
				p.logger.Error("Failed to get pending notifications from ES", zap.Error(err))
				continue
			}

			if len(pending) == 0 {
				continue
			}

			p.logger.Info("Found pending notifications in ES (fallback/sync)", zap.Int("count", len(pending)))

			// Enqueue
			var items []queue.NotificationQueueItem
			for _, notif := range pending {
				items = append(items, queue.NotificationQueueItem{
					NotificationID: notif.NotificationID,
					AppID:          notif.AppID,
					Priority:       notif.Priority,
					EnqueuedAt:     time.Now(),
				})
			}

			if err := p.queue.EnqueueBatch(ctx, items); err != nil {
				p.logger.Error("Failed to enqueue pending notifications from ES", zap.Error(err))
				continue
			}

			// Update statuses
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
				// DLQ items are NOT auto-replayed — they stay in the DLQ
				// until an admin manually triggers replay via the admin API.
				// Auto-replay causes infinite loops when the underlying issue
				// (e.g. unreachable webhook URL) is not fixed.
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
		AppID:          newNotif.AppID,
		Priority:       newNotif.Priority,
		EnqueuedAt:     time.Now(),
	}

	if err := p.queue.EnqueueScheduled(ctx, queueItem, nextRun); err != nil {
		p.logger.Error("Failed to enqueue next recurring notification", zap.Error(err))
		// Not a critical failure as scheduler will pick it up from DB eventually
	}
}

// renderTemplate renders a template string with data
func (p *NotificationProcessor) renderTemplate(tmplStr string, data map[string]interface{}) (string, error) {
	if tmplStr == "" {
		return "", nil
	}

	p.logger.Debug("Rendering template",
		zap.String("template_string", tmplStr),
		zap.Any("rendering_data", data))

	tmpl, err := stdtemplate.New("notification").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		p.logger.Error("Template execution failed", zap.Error(err))
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	result := buf.String()
	p.logger.Debug("Template rendered successfully",
		zap.String("result", result))

	return result, nil
}

// nameFromEmail derives a display name from an email address (e.g. "john.doe@example.com" -> "John Doe").
// Used to auto-fill {{.name}} in templates like welcome_email when the caller does not provide it.
func nameFromEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return "there"
	}
	at := strings.Index(email, "@")
	local := email
	if at > 0 {
		local = email[:at]
	}
	local = strings.ReplaceAll(local, ".", " ")
	local = strings.ReplaceAll(local, "_", " ")
	local = strings.TrimSpace(local)
	if local == "" {
		return "there"
	}
	words := strings.Fields(local)
	for i, w := range words {
		r := []rune(w)
		if len(r) > 0 {
			r[0] = unicode.ToUpper(r[0])
			words[i] = string(r)
		}
	}
	return strings.Join(words, " ")
}

// hasValidEmailConfig returns true if the app has explicitly configured and complete
// credentials for its chosen email provider. "system" or incomplete config means use default .env SMTP.
func hasValidEmailConfig(ec *application.EmailConfig) bool {
	if ec == nil {
		return false
	}
	switch ec.ProviderType {
	case "", "system":
		return false
	case "smtp":
		return ec.SMTP != nil && strings.TrimSpace(ec.SMTP.Host) != ""
	case "sendgrid":
		return ec.SendGrid != nil && strings.TrimSpace(ec.SendGrid.APIKey) != ""
	default:
		return false
	}
}

// needNameFromEmail returns true if data does not contain a non-empty "name" value.
func needNameFromEmail(data map[string]interface{}) bool {
	return needTemplateVar(data, "name")
}

// needTemplateVar returns true if data does not contain a non-empty value for the given key.
func needTemplateVar(data map[string]interface{}, key string) bool {
	v, ok := data[key]
	if !ok {
		return true
	}
	s, _ := v.(string)
	return strings.TrimSpace(s) == ""
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// mergeTemplateData merges control defaults, saved control values, and user payload
// into a single data map for template rendering. Priority (lowest to highest):
//  1. Control defaults (from template control schema)
//  2. Saved control values (editor overrides)
//  3. User payload (API caller runtime overrides)
func (p *NotificationProcessor) mergeTemplateData(tmpl *template.Template, payload map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{})

	// 1. Apply control defaults
	for _, ctrl := range tmpl.Controls {
		if ctrl.Default != "" {
			merged[ctrl.Key] = ctrl.Default
		}
	}

	// 2. Apply saved control values (override defaults)
	for k, v := range tmpl.ControlValues {
		merged[k] = v
	}

	// 3. Apply user payload (highest priority)
	for k, v := range payload {
		merged[k] = v
	}

	return merged
}

// ── Phase 5: Un-snooze loop ─────────────────────────────────────

// unsnoozeLoop periodically checks for snoozed notifications that are due
// and transitions them back to sent so they reappear in the user's inbox.
func (p *NotificationProcessor) unsnoozeLoop(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	p.logger.Info("Un-snooze loop started (30s interval)")

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Un-snooze loop shutting down (context cancelled)")
			return
		case <-p.stopChan:
			p.logger.Info("Un-snooze loop shutting down (stop signal)")
			return
		case <-ticker.C:
			due, err := p.notifRepo.ListSnoozedDue(ctx, time.Now())
			if err != nil {
				p.logger.Error("Failed to fetch snoozed-due notifications", zap.Error(err))
				continue
			}
			if len(due) == 0 {
				continue
			}

			p.logger.Info("Un-snoozing notifications", zap.Int("count", len(due)))

			for _, notif := range due {
				// Set status to queued (not sent) so worker will actually deliver
				if err := p.notifRepo.UpdateSnooze(ctx, notif.NotificationID, notification.StatusQueued, nil); err != nil {
					p.logger.Error("Failed to un-snooze notification",
						zap.String("notification_id", notif.NotificationID),
						zap.Error(err))
					continue
				}

				// Re-enqueue for delivery — worker will pick up and send
				queueItem := queue.NotificationQueueItem{
					NotificationID: notif.NotificationID,
					AppID:          notif.AppID,
					Priority:       notif.Priority,
					RetryCount:     0,
					EnqueuedAt:     time.Now(),
				}
				if err := p.queue.Enqueue(ctx, queueItem); err != nil {
					p.logger.Error("Failed to re-enqueue un-snoozed notification",
						zap.String("notification_id", notif.NotificationID),
						zap.Error(err))
					// Revert status? Could leave as queued; processing reaper might help. For now log.
					continue
				}

				// Publish to SSE via Redis pub/sub so browser clients see activity
				if p.redisClient != nil {
					event := map[string]string{
						"notification_id": notif.NotificationID,
						"channel":         string(notif.Channel),
						"status":          string(notification.StatusQueued),
						"timestamp":       time.Now().Format(time.RFC3339),
					}
					data, _ := json.Marshal(event)
					_ = p.redisClient.Publish(ctx, "notification:activity", string(data)).Err()
				}

				p.logger.Debug("Un-snoozed and re-enqueued notification",
					zap.String("notification_id", notif.NotificationID))
			}
		}
	}
}

// processingReaper periodically checks for items stuck in the processing set
// (visibility timeout expired) and requeues them for at-least-once delivery.
func (p *NotificationProcessor) processingReaper(ctx context.Context) {
	defer p.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	p.logger.Info("Processing reaper started (30s interval)")

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopChan:
			return
		case <-ticker.C:
			if rq, ok := p.queue.(*queue.RedisQueue); ok {
				count, err := rq.RequeueExpiredProcessing(ctx)
				if err != nil {
					p.logger.Error("Processing reaper error", zap.Error(err))
				}
				if count > 0 {
					p.logger.Info("Processing reaper requeued items", zap.Int("count", count))
				}
			}
		}
	}
}
