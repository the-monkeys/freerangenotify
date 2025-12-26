package redis

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// UnreadCounter manages unread counts per app/user in Redis with clamping.
type UnreadCounter struct {
	client *redis.Client
	logger *zap.Logger
}

// NewUnreadCounter constructs a counter backed by Redis.
func NewUnreadCounter(client *redis.Client, logger *zap.Logger) *UnreadCounter {
	return &UnreadCounter{client: client, logger: logger}
}

// Add increments or decrements the unread count by delta and clamps at zero.
func (c *UnreadCounter) Add(ctx context.Context, appID, userID string, delta int64) (int64, error) {
	key := c.key(appID, userID)
	// Lua: apply delta, clamp to zero.
	script := redis.NewScript(`
local new = redis.call('INCRBY', KEYS[1], ARGV[1])
if new < 0 then
  redis.call('SET', KEYS[1], 0)
  return 0
end
return new`)

	res, err := script.Run(ctx, c.client, []string{key}, delta).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to update unread counter: %w", err)
	}

	val, ok := res.(int64)
	if !ok {
		return 0, fmt.Errorf("unexpected unread counter result type: %T", res)
	}
	return val, nil
}

// Reset sets the unread count to zero.
func (c *UnreadCounter) Reset(ctx context.Context, appID, userID string) error {
	if err := c.client.Set(ctx, c.key(appID, userID), 0, 0).Err(); err != nil {
		return fmt.Errorf("failed to reset unread counter: %w", err)
	}
	return nil
}

// Set overwrites the unread count with a non-negative value.
func (c *UnreadCounter) Set(ctx context.Context, appID, userID string, value int64) error {
	if value < 0 {
		value = 0
	}
	if err := c.client.Set(ctx, c.key(appID, userID), value, 0).Err(); err != nil {
		return fmt.Errorf("failed to set unread counter: %w", err)
	}
	return nil
}

// Get fetches the unread count. The boolean indicates cache hit.
func (c *UnreadCounter) Get(ctx context.Context, appID, userID string) (int64, bool, error) {
	parsed, err := c.client.Get(ctx, c.key(appID, userID)).Int64()
	if err == redis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed to parse unread counter: %w", err)
	}
	if parsed < 0 {
		parsed = 0
	}
	return parsed, true, nil
}

func (c *UnreadCounter) key(appID, userID string) string {
	return fmt.Sprintf("frn:unread:%s:%s", appID, userID)
}
