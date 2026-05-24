package container

import (
	"fmt"
	"time"

	"context"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/config"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/audit"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/digest"
	"github.com/the-monkeys/freerangenotify/internal/domain/environment"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/resourcelink"
	"github.com/the-monkeys/freerangenotify/internal/domain/schedule"
	"github.com/the-monkeys/freerangenotify/internal/domain/tenant"
	"github.com/the-monkeys/freerangenotify/internal/domain/topic"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/billingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/idempotency"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/limiter"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/metrics"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/payment"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/sse"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/handlers"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"github.com/the-monkeys/freerangenotify/pkg/jwt"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// Container holds all dependencies for the application
type Container struct {
	// Configuration
	Config *config.Config
	Logger *zap.Logger

	// Database
	DatabaseManager *database.DatabaseManager

	// Queue
	RedisClient *redis.Client
	Queue       queue.Queue

	// Idempotency (Redis-backed for Idempotency-Key header)
	IdempotencyStore *idempotency.Store

	// Metrics
	Metrics *metrics.NotificationMetrics

	// Validator
	Validator *validator.Validator
	Limiter   limiter.Limiter

	// Services
	UserService         usecases.UserService
	ApplicationService  usecases.ApplicationService
	NotificationService notification.Service
	TemplateService     *usecases.TemplateService
	PresenceService     usecases.PresenceService
	PresenceRepository  user.PresenceRepository
	LicensingChecker    license.Checker
	AuthService         auth.Service

	// JWT
	JWTManager *jwt.Manager

	// Handlers
	UserHandler                  *handlers.UserHandler
	ApplicationHandler           *handlers.ApplicationHandler
	NotificationHandler          *handlers.NotificationHandler
	TemplateHandler              *handlers.TemplateHandler
	PresenceHandler              *handlers.PresenceHandler
	AdminHandler                 *handlers.AdminHandler
	DashboardNotificationHandler *handlers.DashboardNotificationHandler
	HealthHandler                *handlers.HealthHandler
	SSEHandler                   *handlers.SSEHandler
	AuthHandler                  *handlers.AuthHandler
	QuickSendHandler             *handlers.QuickSendHandler
	OTPHandler                   *handlers.OTPHandler
	PlaygroundHandler            *handlers.PlaygroundHandler
	AnalyticsHandler             *handlers.AnalyticsHandler
	LicensingHandler             *handlers.LicensingHandler
	OpsHandler                   *handlers.OpsHandler
	BillingHandler               *handlers.BillingHandler
	PaymentHandler               *handlers.PaymentHandler
	RenewalHandler               *handlers.RenewalHandler

	// Billing Metering and Payment
	UsageRepo         billing.UsageRepository
	PaymentProvider   billing.Provider
	RateCardService   *services.RateCardService
	CreditService     *services.CreditService
	rateCardSvcCancel context.CancelFunc

	// Quick-Send
	QuickSendService *usecases.QuickSendService

	// OTP-as-a-service (public API for customers to send/verify OTPs)
	OTPService *services.OTPService

	// Workflow Engine (Phase 1 — feature-gated)
	WorkflowService workflow.Service
	WorkflowHandler *handlers.WorkflowHandler

	// Phase 6: Schedules (feature-gated with Workflow)
	ScheduleService schedule.Service
	ScheduleHandler *handlers.ScheduleHandler

	// Digest Engine (Phase 1 — feature-gated)
	DigestService digest.Service
	DigestHandler *handlers.DigestHandler

	// Topics (Phase 2 — feature-gated)
	TopicService topic.Service
	TopicHandler *handlers.TopicHandler

	// Audit Logs (Phase 2 — feature-gated)
	AuditService  audit.Service
	AuditHandler  *handlers.AuditHandler
	PublicHandler *handlers.PublicHandler

	// RBAC / Team Management (Phase 2 — feature-gated)
	TeamService    auth.TeamService
	TeamHandler    *handlers.TeamHandler
	MembershipRepo auth.MembershipRepository
	AppRepo        application.Repository

	// Media upload
	MediaHandler *handlers.MediaHandler

	// Custom Providers (Phase 3)
	CustomProviderHandler *handlers.CustomProviderHandler

	// Cross-App Resource Linking
	ResourceLinkRepo resourcelink.Repository
	ImportHandler    *handlers.ImportHandler

	// Multi-Environment (Phase 6 — feature-gated)
	EnvironmentService environment.Service
	EnvironmentHandler *handlers.EnvironmentHandler

	// Phase 7: Inbound Webhooks (feature-gated with Workflow)
	InboundWebhookHandler *handlers.InboundWebhookHandler

	// WhatsApp Meta Tech Provider (feature-gated)
	WhatsAppService             whatsapp.Service
	MetaWebhookHandler          *handlers.MetaWebhookHandler
	WhatsAppAdminHandler        *handlers.WhatsAppAdminHandler
	WhatsAppTemplateHandler     *handlers.WhatsAppTemplateHandler
	WhatsAppConversationHandler *handlers.WhatsAppConversationHandler
	// Phase 1 of WHATSAPP_RICH_INTERACTIVE_PLAN.md — typed rich templates.
	WhatsAppRichTemplateService services.WhatsAppRichTemplateService
	WhatsAppRichTemplateHandler *handlers.WhatsAppRichTemplateHandler
	// Phase 2 — Twilio Content API approval webhook.
	TwilioContentStatusHandler *handlers.TwilioContentStatusHandler
	// Phase 6 — Twilio inbound WhatsApp (replies / button taps) webhook.
	TwilioInboundWebhookHandler *handlers.TwilioInboundWebhookHandler
	// Phase 3 — click attribution.
	ClickSigner          *whatsapp.ClickSigner
	ClickRedirectHandler *handlers.ClickRedirectHandler

	// Twilio Content Templates
	TwilioTemplateHandler *handlers.TwilioTemplateHandler

	// Tenant/Organization (Phase C1)
	TenantService tenant.Service
	TenantHandler *handlers.TenantHandler

	// SSE
	SSEBroadcaster *sse.Broadcaster

	// OIDC
	OIDCProvider *oidc.Provider
	OAuth2Config *oauth2.Config
	OIDCVerifier *oidc.IDTokenVerifier
}

// NewContainer creates a new dependency injection container
func NewContainer(cfg *config.Config, logger *zap.Logger) (*Container, error) {
	container := &Container{
		Config:           cfg,
		Logger:           logger,
		Validator:        validator.New(),
		LicensingChecker: license.NewNoopChecker(),
	}

	// Initialize database
	dbManager, err := database.NewDatabaseManager(cfg, logger)
	if err != nil {
		return nil, err
	}
	container.DatabaseManager = dbManager

	// Initialize Redis client
	redisAddr := fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port)
	redisClient := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MaxRetries:   cfg.Redis.MaxRetries,
		MinIdleConns: cfg.Redis.MinIdleConn,
	})
	container.RedisClient = redisClient

	// Initialize queue
	container.Queue = queue.NewRedisQueue(redisClient, logger)

	// Initialize metrics
	container.Metrics = metrics.NewNotificationMetrics()

	// Initialize limiter
	container.Limiter = limiter.NewRedisLimiter(redisClient, logger)

	// Initialize idempotency store
	container.IdempotencyStore = idempotency.NewStore(redisClient, logger)

	// Get repositories from database manager
	repos := dbManager.GetRepositories()

	// Initialize licensing checker
	if cfg.Licensing.Enabled {
		cacheTTL := time.Duration(cfg.Licensing.CacheTTLSeconds) * time.Second
		grace := time.Duration(cfg.Licensing.GraceWindowSeconds) * time.Second

		switch cfg.Licensing.DeploymentMode {
		case string(license.ModeSelfHosted):
			checker, checkerErr := license.NewSelfHostedChecker(license.SelfHostedOptions{
				CacheTTL:         cacheTTL,
				GraceWindow:      grace,
				FailMode:         cfg.Licensing.FailMode,
				LicenseKey:       cfg.Licensing.SelfHosted.LicenseKey,
				PublicKeyPEM:     cfg.Licensing.SelfHosted.PublicKeyPEM,
				LicenseServerURL: cfg.Licensing.SelfHosted.LicenseServerURL,
				VerifyInterval:   time.Duration(cfg.Licensing.SelfHosted.VerifyIntervalSeconds) * time.Second,
			})
			if checkerErr != nil {
				return nil, fmt.Errorf("failed to initialize self-hosted licensing checker: %w", checkerErr)
			}
			container.LicensingChecker = checker
			logger.Info("Self-hosted licensing checker initialized")

		case string(license.ModeHosted):
			checker, checkerErr := license.NewHostedChecker(repos.Subscription, license.HostedOptions{
				CacheTTL:    cacheTTL,
				GraceWindow: grace,
				FailMode:    cfg.Licensing.FailMode,
			})
			if checkerErr != nil {
				return nil, fmt.Errorf("failed to initialize hosted licensing checker: %w", checkerErr)
			}
			container.LicensingChecker = checker
			logger.Info("Hosted licensing checker initialized")

		default:
			return nil, fmt.Errorf("unsupported licensing deployment mode: %s", cfg.Licensing.DeploymentMode)
		}
	}

	// Initialize SSE broadcaster
	container.SSEBroadcaster = sse.NewBroadcaster(repos.Notification, logger)
	container.SSEBroadcaster.SetRedis(redisClient)

	// Initialize services
	container.AppRepo = repos.Application
	container.ApplicationService = services.NewApplicationService(repos.Application, logger)
	container.UserService = services.NewUserService(repos.User, logger)
	container.NotificationService = usecases.NewNotificationService(
		repos.Notification,
		repos.User,
		repos.Application,
		repos.Template,
		container.Queue,
		logger,
		usecases.NotificationServiceConfig{
			MaxRetries: 3,
		},
		container.Metrics,
		container.Limiter,
	)

	// Initialize template service
	container.TemplateService = usecases.NewTemplateService(
		repos.Template,
		logger,
	)

	// Initialize presence repository
	container.PresenceRepository = repository.NewRedisPresenceRepository(redisClient)

	// Initialize presence service
	container.PresenceService = services.NewPresenceService(
		container.PresenceRepository,
		container.NotificationService,
		logger,
	)

	// Initialize JWT manager
	accessTokenDuration := time.Duration(cfg.Security.JWTAccessExpiration) * time.Minute
	refreshTokenDuration := time.Duration(cfg.Security.JWTRefreshExpiration) * time.Minute
	container.JWTManager = jwt.NewManager(cfg.Security.JWTSecret, accessTokenDuration, refreshTokenDuration)

	// Initialize membership repo early — needed by AuthService to claim
	// pending team invitations on login/register regardless of RBAC toggle.
	container.MembershipRepo = repository.NewMembershipRepository(dbManager.Client.GetClient(), logger)
	tenantRepo := repository.NewTenantRepository(dbManager.Client.GetClient(), logger)
	tenantMemberRepo := repository.NewTenantMemberRepository(dbManager.Client.GetClient(), logger)

	// Initialize auth repository and service
	authRepo := repository.NewAuthRepository(dbManager.Client.GetClient(), redisClient, logger)
	otpRepo := repository.NewOTPRepository(redisClient)
	otpSender := services.NewOTPEmailSender(cfg.Providers.SMTP, logger)
	dashboardNotifier := infrastructure.NewDashboardNotifier(
		repos.DashboardNotification,
		container.SSEBroadcaster,
		redisClient,
		logger,
	)
	container.AuthService = services.NewAuthService(
		authRepo,
		container.MembershipRepo,
		container.JWTManager,
		container.NotificationService,
		otpRepo,
		otpSender,
		dashboardNotifier,
		repos.Subscription,
		repos.Application,
		tenantRepo,
		tenantMemberRepo,
		dbManager.Client.GetClient(),
		logger,
	)

	// Initialize tenant/organization support (C1)
	container.TenantService = services.NewTenantService(tenantRepo, tenantMemberRepo, authRepo, repos.Subscription, dashboardNotifier, logger)
	container.TenantHandler = handlers.NewTenantHandler(container.TenantService, container.Validator, logger)
	container.DashboardNotificationHandler = handlers.NewDashboardNotificationHandler(repos.DashboardNotification, logger)

	// Initialize OIDC
	if cfg.OIDC.Enabled {
		if cfg.OIDC.ClientID == "" {
			logger.Warn("OIDC is enabled but client_id is empty. SSO routes will not be registered. " +
				"Register a client in Monkeys Identity and set FREERANGE_OIDC_CLIENT_ID and FREERANGE_OIDC_CLIENT_SECRET.")
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			provider, err := oidc.NewProvider(ctx, cfg.OIDC.Issuer)
			if err != nil {
				logger.Error("Failed to initialize OIDC provider — SSO routes will not be registered. "+
					"Verify that the issuer URL is reachable and has a valid /.well-known/openid-configuration",
					zap.String("issuer", cfg.OIDC.Issuer),
					zap.Error(err),
				)
			} else {
				container.OIDCProvider = provider
				container.OAuth2Config = &oauth2.Config{
					ClientID:     cfg.OIDC.ClientID,
					ClientSecret: cfg.OIDC.ClientSecret,
					RedirectURL:  cfg.OIDC.RedirectURL,
					Endpoint:     provider.Endpoint(),
					Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
				}
				container.OIDCVerifier = provider.Verifier(&oidc.Config{ClientID: cfg.OIDC.ClientID})
				logger.Info("OIDC provider initialized successfully",
					zap.String("issuer", cfg.OIDC.Issuer),
					zap.String("client_id", cfg.OIDC.ClientID),
					zap.String("redirect_url", cfg.OIDC.RedirectURL),
				)
			}
		}
	}

	// Initialize handlers
	container.ApplicationHandler = handlers.NewApplicationHandler(
		container.ApplicationService,
		container.MembershipRepo,
		container.TenantService,
		repos.Application,
		container.Validator,
		logger,
	)
	container.UserHandler = handlers.NewUserHandler(
		container.UserService,
		container.Validator,
		logger,
	)
	// LinkRepo wired after import handler init (below)
	container.NotificationHandler = handlers.NewNotificationHandler(
		container.NotificationService,
		logger,
	)
	container.NotificationHandler.SetIdempotencyStore(container.IdempotencyStore)
	container.NotificationHandler.SetUserRepo(repos.User)
	// Create SMTP provider for template test-send (best-effort — nil if not configured)
	var smtpProvider providers.Provider
	if cfg.Providers.SMTP.Host != "" {
		sp, err := providers.NewSMTPProvider(providers.SMTPConfig{
			Host:      cfg.Providers.SMTP.Host,
			Port:      cfg.Providers.SMTP.Port,
			Username:  cfg.Providers.SMTP.Username,
			Password:  cfg.Providers.SMTP.Password,
			FromEmail: cfg.Providers.SMTP.FromEmail,
			FromName:  cfg.Providers.SMTP.FromName,
		}, logger)
		if err != nil {
			logger.Warn("SMTP provider not available for template test-send", zap.Error(err))
		} else {
			smtpProvider = sp
		}
	}
	container.TemplateHandler = handlers.NewTemplateHandler(
		container.TemplateService,
		smtpProvider,
		logger,
	)
	container.PresenceHandler = handlers.NewPresenceHandler(
		container.PresenceService,
		logger,
	)
	container.AdminHandler = handlers.NewAdminHandler(
		container.Queue,
		nil, // Provider manager is only available in worker process
		repos.Application,
		repos.Notification,
		logger,
	)
	container.AuthHandler = handlers.NewAuthHandler(
		container.AuthService,
		container.Validator,
		cfg,
		logger,
	)
	container.HealthHandler = handlers.NewHealthHandler(
		container.DatabaseManager,
		container.RedisClient,
		logger,
	)
	container.LicensingHandler = handlers.NewLicensingHandler(
		container.LicensingChecker,
		repos.Subscription,
		logger,
	)
	rateCard := billing.DefaultRates()
	container.BillingHandler = handlers.NewBillingHandler(repos.Subscription, repos.Application, rateCard, logger)

	// Initialize Payment Provider
	if cfg.Payment.Provider == "razorpay" {
		container.PaymentProvider = payment.NewRazorpayProvider(
			cfg.Payment.Razorpay.KeyID,
			cfg.Payment.Razorpay.KeySecret,
			cfg.Payment.Razorpay.WebhookSecret,
			cfg.Payment.Razorpay.Currency,
			logger,
		)
		logger.Info("Razorpay payment provider initialized")
	} else {
		container.PaymentProvider = payment.NewMockProvider()
		logger.Info("Mock payment provider initialized (set FREERANGE_PAYMENT_PROVIDER=razorpay for production)")
	}

	container.PaymentHandler = handlers.NewPaymentHandler(
		container.PaymentProvider,
		repos.Subscription,
		repos.Application,
		rateCard,
		cfg.Features.BillingEnabled,
		logger,
	)

	container.RenewalHandler = handlers.NewRenewalHandler(repos.Subscription, repos.Application, rateCard, logger)

	container.SSEHandler = handlers.NewSSEHandler(
		container.SSEBroadcaster,
		container.ApplicationService,
		container.NotificationService,
		repos.User,
		container.MembershipRepo,
		container.RedisClient,
		logger,
	)

	// Quick-Send service and handler
	container.QuickSendService = usecases.NewQuickSendService(
		container.NotificationService,
		repos.User,
		repos.Template,
		container.TemplateService,
		logger,
	)
	container.QuickSendHandler = handlers.NewQuickSendHandler(
		container.QuickSendService,
		container.Validator,
		logger,
	)
	container.QuickSendHandler.SetIdempotencyStore(container.IdempotencyStore)

	// OTP-as-a-service: customer-facing API to send and verify codes via
	// SMS, WhatsApp, or email. Code dispatch flows through the standard
	// NotificationService so credit metering, audit, and retries all apply.
	otpAPIRepo := repository.NewOTPAPIRepository(container.RedisClient)
	container.OTPService = services.NewOTPService(
		otpAPIRepo,
		container.NotificationService,
		repos.User,
		container.TemplateService,
		logger,
	)
	container.OTPHandler = handlers.NewOTPHandler(
		container.OTPService,
		container.Validator,
		logger,
	)

	// Playground handler
	var playgroundBaseURL string
	if cfg.Server.PublicURL != "" {
		playgroundBaseURL = cfg.Server.PublicURL
	} else {
		host := cfg.Server.Host
		if host == "0.0.0.0" || host == "" {
			host = "localhost"
		}
		playgroundBaseURL = fmt.Sprintf("http://%s:%d", host, cfg.Server.Port)
	}
	container.PlaygroundHandler = handlers.NewPlaygroundHandler(
		container.RedisClient,
		playgroundBaseURL,
		logger,
	)
	container.PlaygroundHandler.SetBroadcaster(container.SSEBroadcaster)

	// Media upload handler
	container.MediaHandler = handlers.NewMediaHandler(playgroundBaseURL, logger)

	// Analytics handler (workflow repo wired below after feature-gate check)
	container.AnalyticsHandler = handlers.NewAnalyticsHandler(
		repos.Notification,
		repos.User,
		repos.Template,
		nil, // workflow repo set below if enabled
		repos.Application,
		logger,
	)

	// ── Phase 3: Custom Provider Handler ──
	container.CustomProviderHandler = handlers.NewCustomProviderHandler(
		container.ApplicationService,
		container.MembershipRepo,
		logger,
	)

	// ── Phase 1: Workflow Engine (feature-gated) ──
	if cfg.Features.WorkflowEnabled {
		wfQueue, ok := container.Queue.(queue.WorkflowQueue)
		if !ok {
			logger.Error("Queue does not implement WorkflowQueue — workflow engine disabled")
		} else {
			wfRepo := repository.NewWorkflowRepository(dbManager.Client.GetClient(), logger)
			container.WorkflowService = services.NewWorkflowService(wfRepo, wfQueue, logger)
			container.WorkflowHandler = handlers.NewWorkflowHandler(
				container.WorkflowService,
				container.Validator,
				logger,
			)
			// Wire workflow repo into analytics handler for dashboard counts
			container.AnalyticsHandler.SetWorkflowRepo(wfRepo)
			// Wire workflow service into notification service for broadcast→workflow
			if ns, ok := container.NotificationService.(*usecases.NotificationService); ok {
				ns.SetWorkflowService(container.WorkflowService)
			}
			// Wire app repo + workflow service into user service for on-user-created trigger
			if us, ok := container.UserService.(*services.UserServiceImpl); ok {
				us.SetAppRepo(repos.Application)
				us.SetWorkflowService(container.WorkflowService)
			}
			// Phase 6: Schedule service + handler (workflow schedules)
			scheduleRepo := repository.NewScheduleRepository(dbManager.Client.GetClient(), logger)
			container.ScheduleService = services.NewScheduleService(
				scheduleRepo,
				container.WorkflowService,
				repos.User,
				logger,
			)
			container.ScheduleHandler = handlers.NewScheduleHandler(container.ScheduleService, container.Validator, logger)
			container.InboundWebhookHandler = handlers.NewInboundWebhookHandler(
				container.ApplicationService,
				container.UserService,
				container.WorkflowService,
				logger,
			)
			logger.Info("Workflow engine enabled")
		}
	}

	// ── Phase 1: Digest Engine (feature-gated) ──
	if cfg.Features.DigestEnabled {
		digestRepo := repository.NewDigestRepository(dbManager.Client.GetClient(), logger)
		container.DigestService = services.NewDigestService(digestRepo, logger)
		container.DigestHandler = handlers.NewDigestHandler(
			container.DigestService,
			container.Validator,
			logger,
		)
		logger.Info("Digest engine enabled")
	}

	// ── Phase 1: SSE HMAC enforcement (feature-gated) ──
	if cfg.Features.SSEHMACEnforced {
		container.SSEHandler.SetHMACEnforced(true)
		logger.Info("SSE HMAC subscriber authentication enforced")
	}

	// ── Phase 2: Topics (feature-gated) ──
	if cfg.Features.TopicsEnabled {
		topicRepo := repository.NewTopicRepository(dbManager.Client.GetClient(), logger)
		container.TopicService = services.NewTopicService(topicRepo, logger)
		container.TopicHandler = handlers.NewTopicHandler(container.TopicService, container.Validator, logger)
		container.TopicHandler.SetUserRepo(repos.User)

		// Wire topic service into notification service for fan-out
		if ns, ok := container.NotificationService.(*usecases.NotificationService); ok {
			ns.SetTopicService(container.TopicService)
		}
		// Wire topic service into workflow service for trigger-by-topic
		if ws, ok := container.WorkflowService.(interface{ SetTopicService(topic.Service) }); ok {
			ws.SetTopicService(container.TopicService)
		}
		// Wire workflow service into topic service for on-subscribe trigger
		if ts, ok := container.TopicService.(interface{ SetWorkflowService(workflow.Service) }); ok && container.WorkflowService != nil {
			ts.SetWorkflowService(container.WorkflowService)
		}
		// Wire topic service into schedule service for target_type=topic
		if ss, ok := container.ScheduleService.(interface{ SetTopicService(topic.Service) }); ok {
			ss.SetTopicService(container.TopicService)
		}
		logger.Info("Topics feature enabled")
	}

	// ── Phase 2: Audit Logs (feature-gated) ──
	if cfg.Features.AuditEnabled {
		auditRepo := repository.NewAuditRepository(dbManager.Client.GetClient(), logger)
		container.AuditService = services.NewAuditService(auditRepo, logger)
		container.AuditHandler = handlers.NewAuditHandler(container.AuditService, repos.Application, logger)
		logger.Info("Audit logging enabled")
	}

	// Public handler (aggregate, no auth)
	container.PublicHandler = handlers.NewPublicHandler(authRepo)

	// ── Phase 2: RBAC (feature-gated) ──
	if cfg.Features.RBACEnabled {
		container.TeamService = services.NewTeamService(container.MembershipRepo, authRepo, logger)
		container.TeamHandler = handlers.NewTeamHandler(container.TeamService, repos.Application, logger)
		logger.Info("RBAC / Team management enabled")
	}

	// ── Billing: Usage Metering (feature-gated) ──
	if cfg.Features.BillingEnabled {
		usageRepo := billingrepo.NewESUsageRepo(dbManager.Client.GetClient(), logger)
		if err := usageRepo.EnsureIndex(context.Background()); err != nil {
			logger.Warn("billing: failed to ensure usage index — metering may not work", zap.Error(err))
		}

		rateCardRepo := billingrepo.NewESRateCardRepo(dbManager.Client.GetClient(), logger)
		rateCardService := services.NewRateCardService(
			rateCardRepo,
			redisClient,
			logger,
			services.RateCardServiceConfig{
				RefreshInterval: time.Duration(cfg.Billing.RateCardRefreshSeconds) * time.Second,
				PubSubChannel:   cfg.Billing.RateCardPubSubChannel,
			},
		)
		rcCtx, rcCancel := context.WithCancel(context.Background())
		rateCardService.Start(rcCtx)
		container.rateCardSvcCancel = rcCancel
		container.RateCardService = rateCardService

		creditBalanceRepo := billingrepo.NewSubscriptionCreditBalanceRepo(repos.Subscription, logger)
		creditLedgerRepo := billingrepo.NewESCreditLedgerRepo(dbManager.Client.GetClient(), logger)
		container.CreditService = services.NewCreditService(
			creditBalanceRepo,
			creditLedgerRepo,
			repos.Subscription,
			usageRepo,
			repos.Application,
			rateCardService,
			redisClient,
			logger,
			cfg.Billing.EnforceCreditChecks,
		)

		container.BillingHandler.SetUsageRepo(usageRepo, true)
		container.BillingHandler.SetRateCardManager(rateCardService)
		container.PaymentHandler.SetUsageRepo(usageRepo, true)
		container.RenewalHandler.SetUsageRepo(usageRepo, true)
		container.UsageRepo = usageRepo
		logger.Info("Billing metering enabled", zap.String("index", "usage_events"))
	} else {
		logger.Warn("Billing metering is DISABLED. Set FREERANGE_FEATURES_BILLING_ENABLED=true for production.")
	}

	container.OpsHandler = handlers.NewOpsHandler(
		container.AuthService,
		repos.Subscription,
		repos.Application,
		container.CreditService,
		dashboardNotifier,
		cfg.Providers.SMTP,
		logger,
	)

	// ── WhatsApp Meta Tech Provider (feature-gated) ──
	if cfg.Features.WhatsAppMetaEnabled {
		waRepo := repository.NewWhatsAppRepository(dbManager.Client.GetClient(), logger)
		container.WhatsAppService = services.NewWhatsAppService(
			waRepo,
			repos.Application,
			repos.Notification,
			container.SSEBroadcaster,
			container.WorkflowService,
			redisClient,
			logger,
		)
		container.MetaWebhookHandler = handlers.NewMetaWebhookHandler(
			container.WhatsAppService,
			repos.Application,
			cfg.Providers.MetaWhatsApp.AppSecret,
			cfg.Providers.MetaWhatsApp.WebhookVerify,
			logger,
		)

		webhookURL := ""
		if cfg.Server.PublicURL != "" {
			webhookURL = cfg.Server.PublicURL + "/v1/webhooks/meta/whatsapp"
		}
		container.WhatsAppAdminHandler = handlers.NewWhatsAppAdminHandler(
			container.ApplicationService,
			container.MembershipRepo,
			repos.Application,
			cfg.Providers.MetaWhatsApp.MetaAppID,
			cfg.Providers.MetaWhatsApp.MetaAppSecret,
			cfg.Providers.MetaWhatsApp.APIVersion,
			webhookURL,
			logger,
		)
		container.WhatsAppTemplateHandler = handlers.NewWhatsAppTemplateHandler(
			repos.Application,
			container.SSEBroadcaster,
			cfg.Providers.MetaWhatsApp.APIVersion,
			logger,
		)
		container.MetaWebhookHandler.SetTemplateHandler(container.WhatsAppTemplateHandler)
		container.WhatsAppConversationHandler = handlers.NewWhatsAppConversationHandler(
			container.WhatsAppService,
			logger,
		)
		// Rich-template authoring (carousel, coupon, cta_url, quick_reply, list)
		richTplRepo := repository.NewWhatsAppRichTemplateRepository(dbManager.Client.GetClient(), logger)
		container.WhatsAppRichTemplateService = services.NewWhatsAppRichTemplateService(
			richTplRepo,
			repos.Application,
			nil, // default *http.Client
			cfg.Providers.MetaWhatsApp.APIVersion,
			cfg.Providers.MetaWhatsApp.MetaAppID,
			cfg.Providers.MetaWhatsApp.MetaAppSecret,
			logger,
		)
		container.WhatsAppRichTemplateHandler = handlers.NewWhatsAppRichTemplateHandler(
			container.WhatsAppRichTemplateService,
			logger,
		)
		container.MetaWebhookHandler.SetRichTemplateService(container.WhatsAppRichTemplateService)
		container.TwilioContentStatusHandler = handlers.NewTwilioContentStatusHandler(
			container.WhatsAppRichTemplateService,
			logger,
		)
		// Inbound Twilio WhatsApp webhook (replies, button taps).
		// Signature validation uses the system-level Twilio auth token; per-app
		// auth tokens are not currently surfaced to the inbound webhook because
		// Twilio signs with the account that owns the configured webhook URL,
		// which in self-hosted FRN is always the system account.
		if container.WhatsAppService != nil {
			container.TwilioInboundWebhookHandler = handlers.NewTwilioInboundWebhookHandler(
				container.WhatsAppService,
				repos.Application,
				cfg.Providers.WhatsApp.AuthToken,
				logger,
			)
		}
		// Click attribution signer + redirect handler (Phase 3). Falls back
		// to JWTSecret if no dedicated click-signing key is configured.
		clickKey := cfg.Security.JWTSecret
		signer, err := whatsapp.NewClickSigner(clickKey)
		if err != nil {
			logger.Warn("Click attribution disabled: signer key is empty", zap.Error(err))
		} else {
			container.ClickSigner = signer
			analyticsRepo := repository.NewAnalyticsEventRepository(dbManager.Client.GetClient(), logger)
			container.ClickRedirectHandler = handlers.NewClickRedirectHandler(signer, analyticsRepo, logger)
			// Enable click-attribution wrapping in the rich template
			// service. publicURL is required: log a warning if missing so
			// operators know why tracked URLs degrade to passthrough.
			if cfg.Server.PublicURL == "" {
				logger.Warn("Click attribution: server.public_url is empty; URL buttons with track_clicks will degrade to passthrough")
			} else if container.WhatsAppRichTemplateService != nil {
				container.WhatsAppRichTemplateService.EnableClickTracking(signer, cfg.Server.PublicURL)
			}
		}
		logger.Info("WhatsApp Meta integration enabled (webhook receiver, inbound messaging, embedded signup, template management, inbox)")
	}

	// ── Twilio Content Templates ──
	if cfg.Providers.Twilio.AccountSID != "" && cfg.Providers.Twilio.AuthToken != "" {
		container.TwilioTemplateHandler = handlers.NewTwilioTemplateHandler(
			container.SSEBroadcaster,
			cfg.Providers.Twilio.AccountSID,
			cfg.Providers.Twilio.AuthToken,
			logger,
		)
		logger.Info("Twilio Content Template management enabled")
	}

	// ── Phase 6: Multi-Environment (feature-gated) ──
	if cfg.Features.MultiEnvironmentEnabled {
		envRepo := repository.NewEnvironmentRepository(dbManager.Client.GetClient(), logger)
		// wfRepo may be nil if workflow engine is not enabled
		var wfRepo workflow.Repository
		if container.WorkflowService != nil {
			wfRepoInst := repository.NewWorkflowRepository(dbManager.Client.GetClient(), logger)
			wfRepo = wfRepoInst
		}
		container.EnvironmentService = usecases.NewEnvironmentService(envRepo, repos.Template, wfRepo, logger)
		container.EnvironmentHandler = handlers.NewEnvironmentHandler(
			container.EnvironmentService,
			container.Validator,
			logger,
		)
		logger.Info("Multi-Environment feature enabled")
	}

	// ── Cross-App Resource Linking (after all feature-gated services) ──
	container.ResourceLinkRepo = repository.NewResourceLinkRepository(dbManager.Client.GetClient(), logger)
	cascadeDeleter := services.NewCascadeDeleter(container.ResourceLinkRepo, dbManager.Client.GetClient(), repos.Application, logger)
	container.ApplicationService.(*services.ApplicationServiceImpl).SetCascadeDeleter(cascadeDeleter)
	services.SetAuthCascadeDeleter(container.AuthService, cascadeDeleter)
	container.UserHandler.SetLinkRepo(container.ResourceLinkRepo)
	container.UserHandler.SetUserRepo(repos.User)
	container.TemplateHandler.SetLinkRepo(container.ResourceLinkRepo)
	container.TemplateHandler.SetTemplateRepo(repos.Template)
	if container.WorkflowHandler != nil {
		container.WorkflowHandler.SetLinkRepo(container.ResourceLinkRepo)
		container.WorkflowHandler.SetWorkflowRepo(repository.NewWorkflowRepository(dbManager.Client.GetClient(), logger))
	}
	if container.DigestHandler != nil {
		container.DigestHandler.SetLinkRepo(container.ResourceLinkRepo)
		container.DigestHandler.SetDigestRepo(repository.NewDigestRepository(dbManager.Client.GetClient(), logger))
	}
	if container.TopicHandler != nil {
		container.TopicHandler.SetLinkRepo(container.ResourceLinkRepo)
		container.TopicHandler.SetTopicRepo(repository.NewTopicRepository(dbManager.Client.GetClient(), logger))
	}
	container.CustomProviderHandler.SetLinkRepo(container.ResourceLinkRepo)
	container.ImportHandler = handlers.NewImportHandler(
		container.ResourceLinkRepo,
		container.ApplicationService,
		container.MembershipRepo,
		repos.Application,
		repos.User,
		logger,
	)
	container.ImportHandler.SetTemplateService(container.TemplateService)
	if container.WorkflowService != nil {
		container.ImportHandler.SetWorkflowService(container.WorkflowService)
	}
	if container.DigestService != nil {
		container.ImportHandler.SetDigestService(container.DigestService)
	}
	if container.TopicService != nil {
		container.ImportHandler.SetTopicService(container.TopicService)
	}

	return container, nil
}

// Close cleans up all resources
func (c *Container) Close() error {
	if c.rateCardSvcCancel != nil {
		c.rateCardSvcCancel()
	}
	if c.RateCardService != nil {
		c.RateCardService.Stop()
	}
	if c.Queue != nil {
		c.Queue.Close()
	}
	if c.RedisClient != nil {
		c.RedisClient.Close()
	}
	if c.SSEBroadcaster != nil {
		c.SSEBroadcaster.Close()
	}
	if c.DatabaseManager != nil {
		return c.DatabaseManager.Close()
	}
	return nil
}
