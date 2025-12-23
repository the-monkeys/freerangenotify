package limiter

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type RedisLimiter struct {
	client *redis.Client
	logger *zap.Logger
}

func NewRedisLimiter(client *redis.Client, logger *zap.Logger) *RedisLimiter {
	return &RedisLimiter{
		client: client,
		logger: logger,
	}
}

// Allow checks if an action is allowed based on a sliding window rate limit
func (l *RedisLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	now := time.Now().UnixNano()
	windowStart := now - window.Nanoseconds()

	redisKey := fmt.Sprintf("frn:limiter:window:%s", key)

	pipe := l.client.Pipeline()

	// Remove old entries outside the current window
	pipe.ZRemRangeByScore(ctx, redisKey, "0", strconv.FormatInt(windowStart, 10))

	// Add the current entry
	pipe.ZAdd(ctx, redisKey, &redis.Z{
		Score:  float64(now),
		Member: now,
	})

	// Count entries in the current window
	pipe.ZCard(ctx, redisKey)

	// Set expiration on the key to ensure cleanup
	pipe.Expire(ctx, redisKey, window)

	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to execute rate limit pipeline: %w", err)
	}

	countCmd := cmds[2].(*redis.IntCmd)
	count, err := countCmd.Result()
	if err != nil {
		return false, fmt.Errorf("failed to get rate limit count: %w", err)
	}

	return int(count) <= limit, nil
}

// IncrementAndCheckDailyLimit increments the counter for a key and checks if it exceeds the daily limit
func (l *RedisLimiter) IncrementAndCheckDailyLimit(ctx context.Context, key string, limit int) (bool, error) {
	if limit <= 0 {
		return true, nil // No limit set
	}

	today := time.Now().Format("2006-01-02")
	redisKey := fmt.Sprintf("frn:limiter:daily:%s:%s", today, key)

	pipe := l.client.Pipeline()
	incr := pipe.Incr(ctx, redisKey)
	pipe.Expire(ctx, redisKey, 24*time.Hour)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to increment daily limit: %w", err)
	}

	count, err := incr.Result()
	if err != nil {
		return false, fmt.Errorf("failed to get daily limit result: %w", err)
	}

	return int(count) <= limit, nil
}

// ResetDailyLimit manually resets the daily counter for a key
func (l *RedisLimiter) ResetDailyLimit(ctx context.Context, key string) error {
	today := time.Now().Format("2006-01-02")
	redisKey := fmt.Sprintf("frn:limiter:daily:%s:%s", today, key)
	return l.client.Del(ctx, redisKey).Err()
}
