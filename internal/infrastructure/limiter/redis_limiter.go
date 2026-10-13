package limiter

import (
	"context"
	"fmt"
	"github.com/google/uuid"
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

// Redis time avoids host clock skew; integer millisecond scores avoid float
// precision loss. Only admitted requests are inserted into the sliding window.
var admissionScript = redis.NewScript(`
local stamp=redis.call('TIME')
local now=tonumber(stamp[1])*1000+math.floor(tonumber(stamp[2])/1000)
local window=tonumber(ARGV[2])
-- Older binaries wrote Unix nanoseconds into this same bucket. Normalize those
-- scores once, preserving admissions and their original expiry across upgrade.
local legacy=redis.call('ZRANGEBYSCORE',KEYS[1],'1000000000000000','+inf','WITHSCORES')
for i=1,#legacy,2 do
 redis.call('ZADD',KEYS[1],math.floor(tonumber(legacy[i+1])/1000000),legacy[i])
end
redis.call('ZREMRANGEBYSCORE',KEYS[1],'-inf',now-window)
local count=redis.call('ZCARD',KEYS[1])
local allowed=0
if count<tonumber(ARGV[1]) then
 allowed=1
 redis.call('ZADD',KEYS[1],now,ARGV[3])
 redis.call('PEXPIRE',KEYS[1],window)
 count=count+1
end
local first=redis.call('ZRANGE',KEYS[1],0,0,'WITHSCORES')
local reset=now+window
if #first>0 then reset=tonumber(first[2])+window end
return {allowed,count,now,reset}
`)

func (l *RedisLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	decision, err := l.Admit(ctx, key, limit, window)
	return decision.Allowed, err
}
func (l *RedisLimiter) Admit(ctx context.Context, key string, limit int, window time.Duration) (Admission, error) {
	if limit <= 0 || window <= 0 {
		return Admission{Allowed: true, Limit: limit}, nil
	}
	windowMs := window / time.Millisecond
	if window%time.Millisecond != 0 {
		windowMs++
	}
	raw, err := admissionScript.Run(ctx, l.client, []string{"frn:limiter:window:" + key}, limit, int64(windowMs), uuid.NewString()).Slice()
	if err != nil {
		return Admission{}, fmt.Errorf("rate limit admission unavailable: %w", err)
	}
	if len(raw) != 4 {
		return Admission{}, fmt.Errorf("invalid rate limit response")
	}
	values := make([]int64, 4)
	for i, v := range raw {
		number, ok := v.(int64)
		if !ok {
			return Admission{}, fmt.Errorf("invalid rate limit value")
		}
		values[i] = number
	}
	remaining := limit - int(values[1])
	if remaining < 0 {
		remaining = 0
	}
	decision := Admission{Allowed: values[0] == 1, Limit: limit, Remaining: remaining, ResetAt: time.UnixMilli(values[3])}
	if !decision.Allowed {
		decision.RetryAfter = time.Duration(values[3]-values[2]) * time.Millisecond
		if decision.RetryAfter < time.Millisecond {
			decision.RetryAfter = time.Millisecond
		}
	}
	return decision, nil
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
