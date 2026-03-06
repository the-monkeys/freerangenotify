package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// ThrottleConfig defines per-channel notification limits for a subscriber.
// Zero values mean "no limit" for that window.
type ThrottleConfig struct {
	MaxPerHour int `json:"max_per_hour" es:"max_per_hour"`
	MaxPerDay  int `json:"max_per_day" es:"max_per_day"`
}

// SubscriberThrottle enforces per-subscriber, per-channel notification
// rate limits using Redis counters with hourly and daily windows.
// Design principle: fail-open — if Redis is unreachable the notification
// is allowed through to avoid silent message loss.
type SubscriberThrottle struct {
	client *redis.Client
	logger *zap.Logger
}

// NewSubscriberThrottle creates a new SubscriberThrottle backed by Redis.
func NewSubscriberThrottle(client *redis.Client, logger *zap.Logger) *SubscriberThrottle {
	return &SubscriberThrottle{
		client: client,
		logger: logger,
	}
}

// Allow checks whether a notification is allowed for the given user and
// channel according to the supplied ThrottleConfig.  If the notification
// is allowed the counters are incremented atomically; if denied the
// counters remain unchanged.
//
// A nil or zero-value config means "no throttle" — always allowed.
func (t *SubscriberThrottle) Allow(ctx context.Context, userID, channel string, cfg ThrottleConfig) (bool, error) {
	// No limits configured — allow immediately.
	if cfg.MaxPerHour <= 0 && cfg.MaxPerDay <= 0 {
		return true, nil
	}

	now := time.Now().UTC()

	// ── Hourly window ──
	if cfg.MaxPerHour > 0 {
		hourKey := fmt.Sprintf("frn:throttle:h:%s:%s:%s", userID, channel, now.Format("2006010215"))
		count, err := t.client.Get(ctx, hourKey).Int()
		if err != nil && err != redis.Nil {
			t.logger.Warn("Subscriber throttle Redis error (hourly), failing open",
				zap.String("user_id", userID), zap.Error(err))
			return true, nil // fail-open
		}
		if count >= cfg.MaxPerHour {
			t.logger.Info("Subscriber throttled (hourly limit)",
				zap.String("user_id", userID),
				zap.String("channel", channel),
				zap.Int("count", count),
				zap.Int("limit", cfg.MaxPerHour))
			return false, nil
		}
	}

	// ── Daily window ──
	if cfg.MaxPerDay > 0 {
		dayKey := fmt.Sprintf("frn:throttle:d:%s:%s:%s", userID, channel, now.Format("20060102"))
		count, err := t.client.Get(ctx, dayKey).Int()
		if err != nil && err != redis.Nil {
			t.logger.Warn("Subscriber throttle Redis error (daily), failing open",
				zap.String("user_id", userID), zap.Error(err))
			return true, nil // fail-open
		}
		if count >= cfg.MaxPerDay {
			t.logger.Info("Subscriber throttled (daily limit)",
				zap.String("user_id", userID),
				zap.String("channel", channel),
				zap.Int("count", count),
				zap.Int("limit", cfg.MaxPerDay))
			return false, nil
		}
	}

	// ── Both windows are within limits — atomically increment both ──
	pipe := t.client.Pipeline()
	if cfg.MaxPerHour > 0 {
		hourKey := fmt.Sprintf("frn:throttle:h:%s:%s:%s", userID, channel, now.Format("2006010215"))
		pipe.Incr(ctx, hourKey)
		pipe.Expire(ctx, hourKey, 2*time.Hour) // TTL slightly beyond the window for safety
	}
	if cfg.MaxPerDay > 0 {
		dayKey := fmt.Sprintf("frn:throttle:d:%s:%s:%s", userID, channel, now.Format("20060102"))
		pipe.Incr(ctx, dayKey)
		pipe.Expire(ctx, dayKey, 25*time.Hour)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		t.logger.Warn("Subscriber throttle Redis pipeline error, failing open",
			zap.String("user_id", userID), zap.Error(err))
		// fail-open: we already confirmed limits are not exceeded
	}

	return true, nil
}
