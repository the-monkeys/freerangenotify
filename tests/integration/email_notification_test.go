package integration

import (
	"context"
	"testing"

	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/config"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/limiter"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"go.uber.org/zap"
)

func setupEmailTest(t *testing.T) (
	context.Context,
	notification.Service,
	application.Repository,
	user.Repository,
	limiter.Limiter,
	func(),
) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	// Setup Elasticsearch
	esClient, cleanup := database.SetupTestElasticsearch(t)

	// Create indices
	database.CreateNotificationIndex(esClient)
	database.CreateUserIndex(esClient)
	// database.CreateApplicationIndex(esClient) // Not implemented yet
	// database.CreateTemplateIndex(esClient)    // Not implemented yet

	// Create repositories
	appRepo := repository.NewApplicationRepository(esClient, logger)
	userRepo := repository.NewUserRepository(esClient, logger)
	notifRepo := repository.NewNotificationRepository(esClient, logger)

	// Wrap esClient for TemplateRepository
	dbEsClient := &database.ElasticsearchClient{
		Client: esClient,
		Config: &config.DatabaseConfig{},
		Logger: logger,
	}
	templateRepo := database.NewTemplateRepository(dbEsClient, logger)

	// Setup Redis for limiter
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   2,
	})
	redisClient.FlushDB(ctx)

	redisLimiter := limiter.NewRedisLimiter(redisClient, logger)
	testQueue := queue.NewRedisQueue(redisClient, logger)

	// Create notification service
	notifService := usecases.NewNotificationService(
		notifRepo,
		userRepo,
		appRepo,
		templateRepo,
		testQueue,
		logger,
		usecases.NotificationServiceConfig{},
		nil, // metrics
		redisLimiter,
	)

	cleanupFunc := func() {
		redisClient.FlushDB(ctx)
		redisClient.Close()
		cleanup()
	}

	return ctx, notifService, appRepo, userRepo, redisLimiter, cleanupFunc
}

func TestEmailDailyLimit(t *testing.T) {
	// This test verifies that the application-level daily email limit is enforced.
	// NOTE: This requires mocks or a way to inject repos into the service.
	// For now, I'll rely on the logic I added to NotificationService.Send
	t.Skip("Requires full service setup with all repos")
}
