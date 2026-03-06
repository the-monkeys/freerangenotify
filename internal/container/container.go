package container

import (
	"fmt"
	"time"

	"context"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/config"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/limiter"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/metrics"
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
	UserHandler         *handlers.UserHandler
	ApplicationHandler  *handlers.ApplicationHandler
	NotificationHandler *handlers.NotificationHandler
	TemplateHandler     *handlers.TemplateHandler
	PresenceHandler     *handlers.PresenceHandler
	AdminHandler        *handlers.AdminHandler
	HealthHandler       *handlers.HealthHandler
	SSEHandler          *handlers.SSEHandler
	AuthHandler         *handlers.AuthHandler
	QuickSendHandler    *handlers.QuickSendHandler
	PlaygroundHandler   *handlers.PlaygroundHandler
	AnalyticsHandler    *handlers.AnalyticsHandler

	// Quick-Send
	QuickSendService *usecases.QuickSendService

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

	// Get repositories from database manager
	repos := dbManager.GetRepositories()

	// Initialize SSE broadcaster
	container.SSEBroadcaster = sse.NewBroadcaster(repos.Notification, logger)
	container.SSEBroadcaster.SetRedis(redisClient)

	// Initialize services
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

	// Initialize auth repository and service
	authRepo := repository.NewAuthRepository(dbManager.Client.GetClient(), logger)
	container.AuthService = services.NewAuthService(authRepo, container.JWTManager, container.NotificationService, logger)

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
		container.Validator,
		logger,
	)
	container.UserHandler = handlers.NewUserHandler(
		container.UserService,
		container.Validator,
		logger,
	)
	container.NotificationHandler = handlers.NewNotificationHandler(
		container.NotificationService,
		logger,
	)
	container.TemplateHandler = handlers.NewTemplateHandler(
		container.TemplateService,
		logger,
	)
	container.PresenceHandler = handlers.NewPresenceHandler(
		container.PresenceService,
		logger,
	)
	container.AdminHandler = handlers.NewAdminHandler(
		container.Queue,
		nil, // Provider manager is only available in worker process
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

	// Playground handler
	playgroundBaseURL := fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port)
	container.PlaygroundHandler = handlers.NewPlaygroundHandler(
		container.RedisClient,
		playgroundBaseURL,
		logger,
	)

	// Analytics handler
	container.AnalyticsHandler = handlers.NewAnalyticsHandler(
		repos.Notification,
		logger,
	)

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
