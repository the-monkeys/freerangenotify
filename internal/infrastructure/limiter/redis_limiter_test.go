package limiter

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func redisLimitFixture(t *testing.T) (*redis.Client, *RedisLimiter, string) {
	t.Helper()
	if os.Getenv("FRN_CREDIT_INTEGRATION") != "1" {
		t.Skip("opt-in local Redis fixture")
	}
	addr := os.Getenv("FRN_CREDIT_REDIS_ADDR")
	require.True(t, strings.HasPrefix(addr, "127.0.0.1:"))
	client := redis.NewClient(&redis.Options{Addr: addr})
	require.NoError(t, client.Ping(context.Background()).Err())
	key := "quota-fixture:" + uuid.NewString()
	t.Cleanup(func() { client.Del(context.Background(), "frn:limiter:window:"+key); client.Close() })
	return client, NewRedisLimiter(client, zap.NewNop()), key
}
func TestRedisRejectedRequestsDoNotOccupyWindow(t *testing.T) {
	client, l, key := redisLimitFixture(t)
	ctx := context.Background()
	allowed, err := l.Allow(ctx, key, 2, time.Hour)
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, err = l.Allow(ctx, key, 2, time.Hour)
	require.NoError(t, err)
	require.True(t, allowed)
	for i := 0; i < 10; i++ {
		allowed, err = l.Allow(ctx, key, 2, time.Hour)
		require.NoError(t, err)
		require.False(t, allowed)
	}
	require.Equal(t, int64(2), client.ZCard(ctx, "frn:limiter:window:"+key).Val(), "denied calls must not extend the lockout")
}
func TestRedisConcurrentAdmissionMatchesLimit(t *testing.T) {
	client, l, key := redisLimitFixture(t)
	ctx := context.Background()
	var admitted atomic.Int64
	var failures atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, err := l.Allow(ctx, key, 25, time.Hour)
			if err != nil {
				failures.Add(1)
			}
			if allowed {
				admitted.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Zero(t, failures.Load())
	require.Equal(t, int64(25), admitted.Load())
	require.Equal(t, int64(25), client.ZCard(ctx, "frn:limiter:window:"+key).Val())
}

func TestRedisQuotaPreservesLegacyWindowDuringUpgrade(t *testing.T) {
	client, l, key := redisLimitFixture(t)
	ctx := context.Background()
	now, err := client.Time(ctx).Result()
	require.NoError(t, err)
	redisKey := "frn:limiter:window:" + key
	require.NoError(t, client.ZAdd(ctx, redisKey, &redis.Z{Member: "legacy-request", Score: float64(now.Add(-time.Minute).UnixNano())}).Err())
	denied, err := l.Admit(ctx, key, 1, time.Hour)
	require.NoError(t, err)
	require.False(t, denied.Allowed, "upgrade must preserve admitted requests")
	require.Less(t, denied.RetryAfter, time.Hour, "legacy nanosecond scores must not create an enormous retry delay")
	require.NoError(t, client.ZAdd(ctx, redisKey, &redis.Z{Member: "legacy-request", Score: float64(now.Add(-2 * time.Hour).UnixNano())}).Err())
	allowed, err := l.Admit(ctx, key, 1, time.Hour)
	require.NoError(t, err)
	require.True(t, allowed.Allowed, "expired legacy requests must be removed")
}

func TestRedisQuotaResetDetailsExpiryAndIsolation(t *testing.T) {
	client, l, key := redisLimitFixture(t)
	ctx := context.Background()
	first, err := l.Admit(ctx, key, 1, time.Hour)
	require.NoError(t, err)
	require.True(t, first.Allowed)
	require.Zero(t, first.Remaining)
	ttl := client.PTTL(ctx, "frn:limiter:window:"+key).Val()
	time.Sleep(10 * time.Millisecond)
	denied, err := l.Admit(ctx, key, 1, time.Hour)
	require.NoError(t, err)
	require.False(t, denied.Allowed)
	require.True(t, denied.ResetAt.Equal(first.ResetAt))
	require.Greater(t, denied.RetryAfter, time.Minute*59)
	require.Less(t, client.PTTL(ctx, "frn:limiter:window:"+key).Val(), ttl)
	other := key + ":other"
	t.Cleanup(func() { client.Del(ctx, "frn:limiter:window:"+other) })
	allowed, err := l.Allow(ctx, other, 1, time.Hour)
	require.NoError(t, err)
	require.True(t, allowed)
	rows, err := client.ZRange(ctx, "frn:limiter:window:"+key, 0, -1).Result()
	require.NoError(t, err)
	serverNow, err := client.Time(ctx).Result()
	require.NoError(t, err)
	require.NoError(t, client.ZAdd(ctx, "frn:limiter:window:"+key, &redis.Z{Member: rows[0], Score: float64(serverNow.Add(-time.Hour - time.Second).UnixMilli())}).Err())
	reset, err := l.Admit(ctx, key, 1, time.Hour)
	require.NoError(t, err)
	require.True(t, reset.Allowed)
	outageClient := redis.NewClient(&redis.Options{Addr: os.Getenv("FRN_CREDIT_REDIS_ADDR")})
	require.NoError(t, outageClient.Close())
	_, err = NewRedisLimiter(outageClient, zap.NewNop()).Admit(ctx, key, 1, time.Hour)
	require.Error(t, err, "outage must not pretend admission succeeded")
}
