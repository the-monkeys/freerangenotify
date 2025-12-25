package container

import (
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/config"
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
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
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

	// Handlers
	UserHandler         *handlers.UserHandler
	ApplicationHandler  *handlers.ApplicationHandler
	NotificationHandler *handlers.NotificationHandler
	TemplateHandler     *handlers.TemplateHandler
	PresenceHandler     *handlers.PresenceHandler
	AdminHandler        *handlers.AdminHandler
	HealthHandler       *handlers.HealthHandler
	SSEHandler          *handlers.SSEHandler

	// SSE
	SSEBroadcaster *sse.Broadcaster
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

	// Initialize SSE broadcaster
	container.SSEBroadcaster = sse.NewBroadcaster(logger)
	container.SSEBroadcaster.SetRedis(redisClient)

	// Get repositories from database manager
	repos := dbManager.GetRepositories()

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
		logger,
	)
	container.HealthHandler = handlers.NewHealthHandler(
		container.DatabaseManager,
		container.RedisClient,
		logger,
	)
	container.SSEHandler = handlers.NewSSEHandler(
		container.SSEBroadcaster,
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
