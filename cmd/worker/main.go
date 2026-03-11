package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/config"
	"github.com/the-monkeys/freerangenotify/internal/container"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/limiter"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/orchestrator"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	var logger *zap.Logger
	if cfg.App.Debug || os.Getenv("DEBUG") == "true" {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting notification worker")

	// Create container with dependencies
	c, err := container.NewContainer(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to create container", zap.Error(err))
	}
	defer c.Close()

	// Initialize database
	ctx := context.Background()
	if err := c.DatabaseManager.Initialize(ctx); err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}

	// Create provider manager and register providers
	providerManager := providers.NewManager(c.Metrics, c.PresenceRepository, logger)

	// Build provider config map from application config
	providerConfigs := map[string]map[string]interface{}{
		"fcm": {
			"project_id":       cfg.Providers.FCM.ProjectID,
			"credentials_path": cfg.Providers.FCM.CredentialsPath,
		},
		"apns": {
			"bundle_id":  cfg.Providers.APNS.BundleID,
			"team_id":    cfg.Providers.APNS.TeamID,
			"key_id":     cfg.Providers.APNS.KeyID,
			"key_path":   cfg.Providers.APNS.KeyPath,
			"production": cfg.Providers.APNS.Production,
		},
		"smtp": {
			"host":       cfg.Providers.SMTP.Host,
			"port":       cfg.Providers.SMTP.Port,
			"username":   cfg.Providers.SMTP.Username,
			"password":   cfg.Providers.SMTP.Password,
			"from_email": cfg.Providers.SMTP.FromEmail,
			"from_name":  cfg.Providers.SMTP.FromName,
		},
		"sendgrid": {
			"api_key":    cfg.Providers.SendGrid.APIKey,
			"from_email": cfg.Providers.SendGrid.FromEmail,
			"from_name":  cfg.Providers.SendGrid.FromName,
		},
		"twilio": {
			"account_sid": cfg.Providers.Twilio.AccountSID,
			"auth_token":  cfg.Providers.Twilio.AuthToken,
			"from_number": cfg.Providers.Twilio.FromNumber,
		},
		"webhook": {
			"secret":      cfg.Providers.Webhook.Secret,
			"timeout":     float64(cfg.Providers.Webhook.Timeout),
			"max_retries": float64(cfg.Providers.Webhook.MaxRetries),
		},
		"slack": {
			"enabled":             cfg.Providers.Slack.Enabled,
			"default_webhook_url": cfg.Providers.Slack.DefaultWebhookURL,
			"timeout":             float64(cfg.Providers.Slack.Timeout),
			"max_retries":         float64(cfg.Providers.Slack.MaxRetries),
		},
		"discord": {
			"enabled":             cfg.Providers.Discord.Enabled,
			"default_webhook_url": cfg.Providers.Discord.DefaultWebhookURL,
			"timeout":             float64(cfg.Providers.Discord.Timeout),
			"max_retries":         float64(cfg.Providers.Discord.MaxRetries),
		},
		"whatsapp": {
			"enabled":     cfg.Providers.WhatsApp.Enabled,
			"account_sid": cfg.Providers.WhatsApp.AccountSID,
			"auth_token":  cfg.Providers.WhatsApp.AuthToken,
			"from_number": cfg.Providers.WhatsApp.FromNumber,
			"timeout":     float64(cfg.Providers.WhatsApp.Timeout),
			"max_retries": float64(cfg.Providers.WhatsApp.MaxRetries),
		},
		"resend": {
			"enabled":    cfg.Providers.Resend.Enabled,
			"api_key":    cfg.Providers.Resend.APIKey,
			"from_email": cfg.Providers.Resend.FromEmail,
			"from_name":  cfg.Providers.Resend.FromName,
		},
		"postmark": {
			"enabled":      cfg.Providers.Postmark.Enabled,
			"server_token": cfg.Providers.Postmark.ServerToken,
			"from_email":   cfg.Providers.Postmark.FromEmail,
			"from_name":    cfg.Providers.Postmark.FromName,
		},
		"mailgun": {
			"enabled":    cfg.Providers.Mailgun.Enabled,
			"api_key":    cfg.Providers.Mailgun.APIKey,
			"domain":     cfg.Providers.Mailgun.Domain,
			"from_email": cfg.Providers.Mailgun.FromEmail,
			"from_name":  cfg.Providers.Mailgun.FromName,
		},
		"ses": {
			"enabled":          cfg.Providers.SES.Enabled,
			"region":           cfg.Providers.SES.Region,
			"access_key_id":    cfg.Providers.SES.AccessKeyID,
			"secret_access_key": cfg.Providers.SES.SecretAccessKey,
			"from_email":       cfg.Providers.SES.FromEmail,
			"from_name":        cfg.Providers.SES.FromName,
		},
		"vonage": {
			"enabled":     cfg.Providers.Vonage.Enabled,
			"api_key":     cfg.Providers.Vonage.APIKey,
			"api_secret":  cfg.Providers.Vonage.APISecret,
			"from_number": cfg.Providers.Vonage.FromNumber,
		},
		"teams": {
			"enabled":             cfg.Providers.Teams.Enabled,
			"default_webhook_url": cfg.Providers.Teams.DefaultWebhookURL,
		},
	}

	// Auto-instantiate all registered providers
	registeredProviders := providers.InstantiateAll(providerConfigs, logger)
	byName := make(map[string]providers.Provider)
	for _, p := range registeredProviders {
		byName[p.GetName()] = p
	}
	// Register email providers in preferred order: SMTP first (default .env), then SendGrid.
	// When app has no/invalid email config, we use the default for the channel = first registered.
	emailOrder := []string{"smtp", "sendgrid", "resend", "postmark", "mailgun", "ses"}
	for _, name := range emailOrder {
		if p, ok := byName[name]; ok {
			if err := providerManager.RegisterProvider(p); err != nil {
				logger.Warn("Failed to register provider", zap.String("provider", name), zap.Error(err))
			}
			delete(byName, name)
		}
	}
	// Register remaining providers
	for _, p := range byName {
		if err := providerManager.RegisterProvider(p); err != nil {
			logger.Warn("Failed to register provider", zap.String("provider", p.GetName()), zap.Error(err))
		}
	}

	// SSE provider requires Redis client — wire manually
	sseProvider, err := providers.NewSSEProvider(providers.SSEConfig{
		Config: providers.Config{
			Timeout:    5 * time.Second,
			MaxRetries: 3,
			RetryDelay: 1 * time.Second,
		},
	}, c.RedisClient, logger)
	if err != nil {
		logger.Warn("Failed to initialize SSE provider", zap.Error(err))
	} else {
		if err := providerManager.RegisterProvider(sseProvider); err != nil {
			logger.Warn("Failed to register SSE provider", zap.Error(err))
		}
	}

	logger.Info("Provider registry initialized",
		zap.Strings("registered_factories", providers.RegisteredProviders()),
		zap.Strings("active_channels", func() []string {
			channels := providerManager.GetSupportedChannels()
			result := make([]string, len(channels))
			for i, ch := range channels {
				result[i] = string(ch)
			}
			return result
		}()))

	// Create notification processor
	workerCount := cfg.Queue.Workers
	if workerCount == 0 {
		workerCount = 5 // Default
	}

	processor := NewNotificationProcessor(
		c.Queue,
		c.DatabaseManager.Repositories.Notification,
		c.DatabaseManager.Repositories.User,
		c.DatabaseManager.Repositories.Application,
		c.DatabaseManager.Repositories.Template,
		providerManager,
		c.RedisClient,
		logger,
		ProcessorConfig{
			WorkerCount:     workerCount,
			PollInterval:    5 * time.Second,
			MaxRetries:      cfg.Queue.MaxRetries,
			RetryDelay:      5 * time.Second,
			MaxRetryDelay:   5 * time.Minute,
			ShutdownTimeout: 30 * time.Second,
		},
		c.Metrics,
	)

	// Start processor
	processorCtx, processorCancel := context.WithCancel(ctx)
	defer processorCancel()

	if err := processor.Start(processorCtx); err != nil {
		logger.Fatal("Failed to start processor", zap.Error(err))
	}

	logger.Info("Notification worker started successfully",
		zap.Int("worker_count", workerCount))

	// ── Phase 1: Workflow Engine (feature-gated) ──
	var workflowEngine *orchestrator.Engine
	var schedulePoller *orchestrator.SchedulePoller
	if cfg.Features.WorkflowEnabled {
		wfQueue, ok := c.Queue.(queue.WorkflowQueue)
		if !ok {
			logger.Fatal("Queue does not implement WorkflowQueue interface; cannot start workflow engine")
		}
		wfRepo := repository.NewWorkflowRepository(c.DatabaseManager.Client.GetClient(), logger)
		workflowEngine = orchestrator.NewEngine(
			wfRepo,
			c.NotificationService,
			wfQueue,
			c.RedisClient,
			logger,
			c.Metrics,
			cfg.Queue.Workers,
		)
		workflowEngine.Start(processorCtx)
		logger.Info("Workflow engine started")

		// Phase 6: Schedule poller (runs scheduled workflows every minute)
		scheduleRepo := repository.NewScheduleRepository(c.DatabaseManager.Client.GetClient(), logger)
		schedulePoller := orchestrator.NewSchedulePoller(
			scheduleRepo,
			c.WorkflowService,
			c.TopicService,
			c.DatabaseManager.Repositories.User,
			logger,
		)
		schedulePoller.Start(processorCtx)
		logger.Info("Schedule poller started")
	}

	// ── Phase 1: Digest Manager (feature-gated) ──
	var digestManager *orchestrator.DigestManager
	if cfg.Features.DigestEnabled {
		digestRepo := repository.NewDigestRepository(c.DatabaseManager.Client.GetClient(), logger)
		digestManager = orchestrator.NewDigestManager(
			digestRepo,
			c.DatabaseManager.Repositories.Notification,
			c.NotificationService,
			c.RedisClient,
			logger,
		)
		processor.SetDigestManager(digestManager)
		digestManager.StartFlushPoller(processorCtx)
		logger.Info("Digest manager started")
	}

	// ── Phase 2: Per-Subscriber Throttle (feature-gated) ──
	if cfg.Features.ThrottleEnabled {
		subscriberThrottle := limiter.NewSubscriberThrottle(c.RedisClient, logger)
		processor.SetSubscriberThrottle(subscriberThrottle)
		logger.Info("Subscriber throttle enabled")
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	logger.Info("Shutdown signal received, stopping worker...")

	// Cancel processor context
	processorCancel()

	// Shutdown workflow engine
	if workflowEngine != nil {
		shutdownEngineCtx, shutdownEngineCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownEngineCancel()
		if err := workflowEngine.Shutdown(shutdownEngineCtx); err != nil {
			logger.Error("Error shutting down workflow engine", zap.Error(err))
		}
	}

	// Shutdown schedule poller
	if schedulePoller != nil {
		schedulePoller.Shutdown()
	}

	// Shutdown digest manager
	if digestManager != nil {
		digestManager.Shutdown()
	}

	// Wait for processor to finish with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := processor.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error during shutdown", zap.Error(err))
	}

	logger.Info("Notification worker stopped")
}
