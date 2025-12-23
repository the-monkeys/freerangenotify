package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
)

type RedisPresenceRepository struct {
	client *redis.Client
	prefix string
}

func NewRedisPresenceRepository(client *redis.Client) user.PresenceRepository {
	return &RedisPresenceRepository{
		client: client,
		prefix: "presence:",
	}
}

func (r *RedisPresenceRepository) Set(ctx context.Context, presence *user.Presence, ttl time.Duration) error {
	data, err := json.Marshal(presence)
	if err != nil {
		return fmt.Errorf("failed to marshal presence: %w", err)
	}

	key := r.prefix + presence.UserID
	return r.client.Set(ctx, key, data, ttl).Err()
}

func (r *RedisPresenceRepository) Get(ctx context.Context, userID string) (*user.Presence, error) {
	key := r.prefix + userID
	data, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var presence user.Presence
	if err := json.Unmarshal(data, &presence); err != nil {
		return nil, err
	}

	return &presence, nil
}

func (r *RedisPresenceRepository) Delete(ctx context.Context, userID string) error {
	key := r.prefix + userID
	return r.client.Del(ctx, key).Err()
}

func (r *RedisPresenceRepository) IsAvailable(ctx context.Context, userID string) (bool, string, error) {
	presence, err := r.Get(ctx, userID)
	if err != nil {
		return false, "", err
	}
	if presence == nil {
		return false, "", nil
	}
	return presence.Status == "active", presence.DynamicURL, nil
}
