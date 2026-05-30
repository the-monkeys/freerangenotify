package main

import (
	"context"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	domainfile "github.com/the-monkeys/freerangenotify/internal/domain/file"
)

const fileCleanupKey = "frn:file_cleanup"

// FileCleanupEnqueuer pushes orphaned blob references to a Redis set.
type FileCleanupEnqueuer struct {
	rdb *redis.Client
}

func NewFileCleanupEnqueuer(rdb *redis.Client) *FileCleanupEnqueuer {
	return &FileCleanupEnqueuer{rdb: rdb}
}

func (e *FileCleanupEnqueuer) EnqueueBlobDelete(ctx context.Context, appID, fileID string) {
	e.rdb.SAdd(ctx, fileCleanupKey, appID+"|"+fileID)
}

// StartFileCleanupLoop drains the cleanup set every 5 minutes.
// Blocks until ctx is cancelled.
func StartFileCleanupLoop(ctx context.Context, rdb *redis.Client, store domainfile.FileStore, logger *zap.Logger) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for i := 0; i < 10; i++ {
				val, err := rdb.SPop(ctx, fileCleanupKey).Result()
				if err != nil {
					break // empty or error — done for this tick
				}
				parts := strings.SplitN(val, "|", 2)
				if len(parts) != 2 {
					continue
				}
				appID, fileID := parts[0], parts[1]
				if err := store.Delete(ctx, appID, fileID); err != nil {
					// Re-enqueue on failure; will retry next tick
					rdb.SAdd(ctx, fileCleanupKey, val)
					logger.Warn("file_cleanup: retry next tick",
						zap.String("file_id", fileID), zap.Error(err))
					break
				}
				logger.Info("file_cleanup: orphan blob removed",
					zap.String("app_id", appID), zap.String("file_id", fileID))
			}
		}
	}
}
