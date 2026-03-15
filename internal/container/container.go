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
	"github.com/the-monkeys/freerangenotify/internal/domain/digest"
	"github.com/the-monkeys/freerangenotify/internal/domain/environment"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/resourcelink"
	"github.com/the-monkeys/freerangenotify/internal/domain/schedule"
	"github.com/the-monkeys/freerangenotify/internal/domain/tenant"
	"github.com/the-monkeys/freerangenotify/internal/domain/topic"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/idempotency"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/limiter"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/metrics"
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
	PlaygroundHandler            *handlers.PlaygroundHandler
	AnalyticsHandler             *handlers.AnalyticsHandler

	// Quick-Send
	QuickSendService *usecases.QuickSendService

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
	AuditService audit.Service
	AuditHandler *handlers.AuditHandler

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
		Config:    cfg,
		Logger:    logger,
		Validator: validator.New(),
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

	// Initialize auth repository and service
	authRepo := repository.NewAuthRepository(dbManager.Client.GetClient(), redisClient, logger)
	otpRepo := repository.NewOTPRepository(redisClient)
	otpSender := services.NewOTPEmailSender(cfg.Providers.SMTP, logger)
	container.AuthService = services.NewAuthService(authRepo, container.MembershipRepo, container.JWTManager, container.NotificationService, otpRepo, otpSender, logger)

	// Initialize tenant/organization support (C1)
	tenantRepo := repository.NewTenantRepository(dbManager.Client.GetClient(), logger)
	tenantMemberRepo := repository.NewTenantMemberRepository(dbManager.Client.GetClient(), logger)
	dashboardNotifier := infrastructure.NewDashboardNotifier(
		repos.DashboardNotification,
		container.SSEBroadcaster,
		redisClient,
		logger,
	)
	container.TenantService = services.NewTenantService(tenantRepo, tenantMemberRepo, authRepo, dashboardNotifier, logger)
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
		logger,
	)
	container.HealthHandler = handlers.NewHealthHandler(
		container.DatabaseManager,
		container.RedisClient,
		logger,
	)
	container.SSEHandler = handlers.NewSSEHandler(
		container.SSEBroadcaster,
		container.ApplicationService,
		container.NotificationService,
		repos.User,
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

	// ── Phase 2: RBAC (feature-gated) ──
	if cfg.Features.RBACEnabled {
		container.TeamService = services.NewTeamService(container.MembershipRepo, authRepo, logger)
		container.TeamHandler = handlers.NewTeamHandler(container.TeamService, repos.Application, logger)
		logger.Info("RBAC / Team management enabled")
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
	container.UserHandler.SetLinkRepo(container.ResourceLinkRepo)
	container.TemplateHandler.SetLinkRepo(container.ResourceLinkRepo)
	if container.WorkflowHandler != nil {
		container.WorkflowHandler.SetLinkRepo(container.ResourceLinkRepo)
	}
	if container.DigestHandler != nil {
		container.DigestHandler.SetLinkRepo(container.ResourceLinkRepo)
	}
	if container.TopicHandler != nil {
		container.TopicHandler.SetLinkRepo(container.ResourceLinkRepo)
	}
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
