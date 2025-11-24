package database

import (
	"context"
	"fmt"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/config"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

// DatabaseManager manages all database operations
type DatabaseManager struct {
	Client       *ElasticsearchClient
	IndexManager *IndexManager
	Repositories *Repositories
	logger       *zap.Logger
}

// Repositories holds all repository instances
type Repositories struct {
	Application  application.Repository
	User         user.Repository
	Notification notification.Repository
	// Template and Analytics repositories will be added later
}

// NewDatabaseManager creates a new database manager
func NewDatabaseManager(cfg *config.Config, logger *zap.Logger) (*DatabaseManager, error) {
	// Create Elasticsearch client
	client, err := NewElasticsearchClient(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	// Create index manager
	indexManager := NewIndexManager(client, logger)

	// Create repositories
	repositories := &Repositories{
		Application:  repository.NewApplicationRepository(client.GetClient(), logger),
		User:         repository.NewUserRepository(client.GetClient(), logger),
		Notification: repository.NewNotificationRepository(client.GetClient(), logger),
	}

	return &DatabaseManager{
		Client:       client,
		IndexManager: indexManager,
		Repositories: repositories,
		logger:       logger,
	}, nil
}

// Initialize initializes the database system
func (dm *DatabaseManager) Initialize(ctx context.Context) error {
	dm.logger.Info("Initializing database system...")

	// Test connection first
	_, err := dm.Client.Health(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Elasticsearch: %w", err)
	}

	dm.logger.Info("Elasticsearch connection established")

	// Create indices
	operations, err := dm.IndexManager.CreateIndices(ctx)
	if err != nil {
		return fmt.Errorf("failed to create indices: %w", err)
	}

	// Log index creation results
	successCount := 0
	for _, op := range operations {
		if op.Success {
			successCount++
		}
		dm.logger.Info("Index operation completed",
			zap.String("index", op.IndexName),
			zap.String("action", op.Action),
			zap.Bool("success", op.Success),
			zap.String("message", op.Message))
	}

	dm.logger.Info("Database initialization completed",
		zap.Int("total_indices", len(operations)),
		zap.Int("successful", successCount))

	return nil
}

// Close closes all database connections
func (dm *DatabaseManager) Close() error {
	// Elasticsearch client doesn't need explicit closing
	dm.logger.Info("Database connections closed")
	return nil
}

// Health checks the health of all database components
func (dm *DatabaseManager) Health(ctx context.Context) error {
	// Check Elasticsearch health
	_, err := dm.Client.Health(ctx)
	if err != nil {
		return fmt.Errorf("Elasticsearch health check failed: %w", err)
	}

	// Check if key indices exist
	keyIndices := []string{"applications", "users", "notifications"}
	for _, index := range keyIndices {
		exists, err := dm.IndexManager.IndexExists(ctx, index)
		if err != nil {
			return fmt.Errorf("failed to check index %s: %w", index, err)
		}
		if !exists {
			return fmt.Errorf("critical index %s does not exist", index)
		}
	}

	return nil
}

// Migrate runs database migrations
func (dm *DatabaseManager) Migrate(ctx context.Context) error {
	dm.logger.Info("Running database migrations...")

	if err := dm.IndexManager.MigrateIndices(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	dm.logger.Info("Database migrations completed")
	return nil
}

// Stats returns database statistics
func (dm *DatabaseManager) Stats(ctx context.Context) (map[string]interface{}, error) {
	stats := map[string]interface{}{
		"timestamp": time.Now(),
		"status":    "healthy",
	}

	// Get cluster info
	clusterHealth, err := dm.Client.Health(ctx)
	if err != nil {
		stats["status"] = "unhealthy"
		stats["error"] = err.Error()
		return stats, err
	}

	stats["cluster"] = clusterHealth

	// Get index statistics
	indices := []string{"applications", "users", "notifications", "templates", "analytics"}
	indexStats := make(map[string]interface{})

	for _, index := range indices {
		exists, err := dm.IndexManager.IndexExists(ctx, index)
		if err != nil {
			indexStats[index] = map[string]interface{}{
				"exists": false,
				"error":  err.Error(),
			}
			continue
		}

		if exists {
			info, err := dm.IndexManager.GetIndexInfo(ctx, index)
			if err != nil {
				indexStats[index] = map[string]interface{}{
					"exists": true,
					"error":  err.Error(),
				}
			} else {
				indexStats[index] = info
			}
		} else {
			indexStats[index] = map[string]interface{}{
				"exists": false,
			}
		}
	}

	stats["indices"] = indexStats
	return stats, nil
}

// Cleanup performs database cleanup operations
func (dm *DatabaseManager) Cleanup(ctx context.Context) error {
	dm.logger.Info("Starting database cleanup...")

	// This could include:
	// - Cleaning up old notifications
	// - Archiving analytics data
	// - Removing expired user sessions
	// - Optimizing indices

	dm.logger.Info("Database cleanup completed")
	return nil
}

// GetRepositories returns the repositories instance
func (dm *DatabaseManager) GetRepositories() *Repositories {
	return dm.Repositories
}
