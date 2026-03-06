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

	// Initialize and register FCM provider
	fcmProvider, err := providers.NewFCMProvider(providers.FCMConfig{
		Config: providers.Config{
			Timeout:    10 * time.Second,
			MaxRetries: 3,
			RetryDelay: 1 * time.Second,
		},
		ProjectID:       cfg.Providers.FCM.ProjectID,
		CredentialsPath: cfg.Providers.FCM.CredentialsPath,
	}, logger)
	if err != nil {
		logger.Warn("Failed to initialize FCM provider", zap.Error(err))
	} else {
		providerManager.RegisterProvider(fcmProvider)
	}

	// Initialize and register APNS provider
	apnsProvider, err := providers.NewAPNSProvider(providers.APNSConfig{
		Config: providers.Config{
			Timeout:    10 * time.Second,
			MaxRetries: 3,
			RetryDelay: 1 * time.Second,
		},
		BundleID:   cfg.Providers.APNS.BundleID,
		TeamID:     cfg.Providers.APNS.TeamID,
		KeyID:      cfg.Providers.APNS.KeyID,
		KeyPath:    cfg.Providers.APNS.KeyPath,
		Production: cfg.Providers.APNS.Production,
	}, logger)
	if err != nil {
		logger.Warn("Failed to initialize APNS provider", zap.Error(err))
	} else {
		providerManager.RegisterProvider(apnsProvider)
	}

	// Initialize and register SMTP provider if configured
	logger.Debug("Checking SMTP configuration", zap.String("host", cfg.Providers.SMTP.Host))
	if cfg.Providers.SMTP.Host != "" {
		smtpProvider, err := providers.NewSMTPProvider(providers.SMTPConfig{
			Config: providers.Config{
				Timeout:    30 * time.Second,
				MaxRetries: 3,
				RetryDelay: 1 * time.Second,
			},
			Host:      cfg.Providers.SMTP.Host,
			Port:      cfg.Providers.SMTP.Port,
			Username:  cfg.Providers.SMTP.Username,
			Password:  cfg.Providers.SMTP.Password,
			FromEmail: cfg.Providers.SMTP.FromEmail,
			FromName:  cfg.Providers.SMTP.FromName,
		}, logger)

		if err != nil {
			logger.Warn("Failed to initialize SMTP provider", zap.Error(err))
		} else {
			if err := providerManager.RegisterProvider(smtpProvider); err != nil {
				logger.Warn("Failed to register SMTP provider", zap.Error(err))
			} else {
				logger.Info("Registered SMTP provider for email channel")
			}
		}
	} else {
		logger.Warn("SMTP provider not configured - host is empty")
	}

	// Initialize and register SendGrid provider if configured
	logger.Debug("Checking SendGrid configuration", zap.String("api_key", cfg.Providers.SendGrid.APIKey))
	if cfg.Providers.SendGrid.APIKey != "" {
		sendgridProvider, err := providers.NewSendGridProvider(providers.SendGridConfig{
			Config: providers.Config{
				Timeout:    15 * time.Second,
				MaxRetries: 3,
				RetryDelay: 1 * time.Second,
			},
			APIKey:    cfg.Providers.SendGrid.APIKey,
			FromEmail: cfg.Providers.SendGrid.FromEmail,
			FromName:  cfg.Providers.SendGrid.FromName,
		}, logger)
		if err != nil {
			logger.Warn("Failed to initialize SendGrid provider", zap.Error(err))
		} else {
			if err := providerManager.RegisterProvider(sendgridProvider); err != nil {
				logger.Warn("Failed to register SendGrid provider", zap.Error(err))
			} else {
				logger.Info("Registered SendGrid provider for email channel")
			}
		}
	}

	// Initialize and register Twilio provider
	twilioProvider, err := providers.NewTwilioProvider(providers.TwilioConfig{
		Config: providers.Config{
			Timeout:    10 * time.Second,
			MaxRetries: 3,
			RetryDelay: 1 * time.Second,
		},
		AccountSID: cfg.Providers.Twilio.AccountSID,
		AuthToken:  cfg.Providers.Twilio.AuthToken,
		FromNumber: cfg.Providers.Twilio.FromNumber,
	}, logger)
	if err != nil {
		logger.Warn("Failed to initialize Twilio provider", zap.Error(err))
	} else {
		providerManager.RegisterProvider(twilioProvider)
	}

	// Initialize and register Webhook provider
	webhookProvider, err := providers.NewWebhookProvider(providers.WebhookConfig{
		Config: providers.Config{
			Timeout:    time.Duration(cfg.Providers.Webhook.Timeout) * time.Second,
			MaxRetries: cfg.Providers.Webhook.MaxRetries,
			RetryDelay: 2 * time.Second,
		},
		Secret: cfg.Providers.Webhook.Secret,
	}, logger)
	if err != nil {
		logger.Warn("Failed to initialize Webhook provider", zap.Error(err))
	} else {
		if err := providerManager.RegisterProvider(webhookProvider); err != nil {
			logger.Warn("Failed to register Webhook provider", zap.Error(err))
		}
	}

	// Initialize and register SSE provider
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

	// ── Phase 3: Slack provider ──
	if cfg.Providers.Slack.Enabled {
		slackProvider, err := providers.NewSlackProvider(providers.SlackConfig{
			Config: providers.Config{
				Timeout:    time.Duration(cfg.Providers.Slack.Timeout) * time.Second,
				MaxRetries: cfg.Providers.Slack.MaxRetries,
				RetryDelay: 2 * time.Second,
			},
			DefaultWebhookURL: cfg.Providers.Slack.DefaultWebhookURL,
		}, logger)
		if err != nil {
			logger.Warn("Failed to initialize Slack provider", zap.Error(err))
		} else {
			if err := providerManager.RegisterProvider(slackProvider); err != nil {
				logger.Warn("Failed to register Slack provider", zap.Error(err))
			} else {
				logger.Info("Registered Slack provider")
			}
		}
	}

	// ── Phase 3: Discord provider ──
	if cfg.Providers.Discord.Enabled {
		discordProvider, err := providers.NewDiscordProvider(providers.DiscordConfig{
			Config: providers.Config{
				Timeout:    time.Duration(cfg.Providers.Discord.Timeout) * time.Second,
				MaxRetries: cfg.Providers.Discord.MaxRetries,
				RetryDelay: 2 * time.Second,
			},
			DefaultWebhookURL: cfg.Providers.Discord.DefaultWebhookURL,
		}, logger)
		if err != nil {
			logger.Warn("Failed to initialize Discord provider", zap.Error(err))
		} else {
			if err := providerManager.RegisterProvider(discordProvider); err != nil {
				logger.Warn("Failed to register Discord provider", zap.Error(err))
			} else {
				logger.Info("Registered Discord provider")
			}
		}
	}

	// ── Phase 3: WhatsApp provider (Twilio-backed) ──
	if cfg.Providers.WhatsApp.Enabled {
		whatsappProvider, err := providers.NewWhatsAppProvider(providers.WhatsAppConfig{
			Config: providers.Config{
				Timeout:    time.Duration(cfg.Providers.WhatsApp.Timeout) * time.Second,
				MaxRetries: cfg.Providers.WhatsApp.MaxRetries,
				RetryDelay: 2 * time.Second,
			},
			AccountSID: cfg.Providers.WhatsApp.AccountSID,
			AuthToken:  cfg.Providers.WhatsApp.AuthToken,
			FromNumber: cfg.Providers.WhatsApp.FromNumber,
		}, logger)
		if err != nil {
			logger.Warn("Failed to initialize WhatsApp provider", zap.Error(err))
		} else {
			if err := providerManager.RegisterProvider(whatsappProvider); err != nil {
				logger.Warn("Failed to register WhatsApp provider", zap.Error(err))
			} else {
				logger.Info("Registered WhatsApp provider")
			}
		}
	}

	logger.Info("Provider manager initialized",
		zap.Strings("channels", func() []string {
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
	}

	// ── Phase 1: Digest Manager (feature-gated) ──
	var digestManager *orchestrator.DigestManager
	if cfg.Features.DigestEnabled {
		digestRepo := repository.NewDigestRepository(c.DatabaseManager.Client.GetClient(), logger)
		digestManager = orchestrator.NewDigestManager(
			digestRepo,
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
