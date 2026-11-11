package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/otp"
)

func otpRedisFixture(t *testing.T) (*redis.Client, *redisOTPAPIRepository) {
	t.Helper()
	addr := os.Getenv("FRN_OTP_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires owned disposable Redis fixture")
	}
	require.Equal(t, "127.0.0.1:16390", addr)
	rc := redis.NewClient(&redis.Options{Addr: addr})
	require.NoError(t, rc.Ping(context.Background()).Err())
	t.Cleanup(func() { rc.Close() })
	return rc, &redisOTPAPIRepository{client: rc}
}
func TestOTPAtomicStore(t *testing.T) {
	rc, r := otpRedisFixture(t)
	ctx := context.Background()
	cas, ok := interface{}(r).(interface {
		CompareAndSwap(context.Context, *otp.Request, *otp.Request) error
	})
	require.True(t, ok, "OTP state must support CAS")
	req := &otp.Request{RequestID: "test:" + uuid.NewString(), AppID: "app-fixture", CodeHash: "old", ExpiresAt: time.Now().UTC().Add(time.Minute)}
	require.NoError(t, r.Create(ctx, req))
	t.Cleanup(func() { rc.Del(ctx, r.key(req.RequestID)) })
	raw, err := rc.Get(ctx, r.key(req.RequestID)).Bytes()
	require.NoError(t, err)
	var doc map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &doc))
	doc["future_field"] = "preserve"
	raw, err = json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, rc.Set(ctx, r.key(req.RequestID), raw, time.Until(req.ExpiresAt)).Err())
	beforeTTL := rc.PTTL(ctx, r.key(req.RequestID)).Val()
	var wg sync.WaitGroup
	out := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			next := *req
			next.CodeHash = "new"
			out <- cas.CompareAndSwap(ctx, req, &next)
		}()
	}
	wg.Wait()
	close(out)
	successes := 0
	for err := range out {
		if err == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes)
	after, err := r.Get(ctx, req.RequestID)
	require.NoError(t, err)
	require.Equal(t, "new", after.CodeHash)
	require.Equal(t, req.ExpiresAt, after.ExpiresAt)
	require.LessOrEqual(t, rc.PTTL(ctx, r.key(req.RequestID)).Val(), beforeTTL)
	raw, err = rc.Get(ctx, r.key(req.RequestID)).Bytes()
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &doc))
	require.Equal(t, "preserve", doc["future_field"])
	require.Equal(t, float64(1), doc["revision"])
	next := *req
	require.Error(t, cas.CompareAndSwap(ctx, req, &next))
	require.NoError(t, rc.Del(ctx, r.key(req.RequestID)).Err())
	require.Error(t, cas.CompareAndSwap(ctx, after, &next))
	require.Equal(t, int64(0), rc.Exists(ctx, r.key(req.RequestID)).Val())
}

type otpExpiryFault struct{}

func (otpExpiryFault) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	if cmd.Name() == "expire" {
		return ctx, errors.New("lost expiry write")
	}
	return ctx, nil
}
func (otpExpiryFault) AfterProcess(context.Context, redis.Cmder) error { return nil }
func (otpExpiryFault) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}
func (otpExpiryFault) AfterProcessPipeline(context.Context, []redis.Cmder) error { return nil }
func TestOTPRecipientBudgetAtomicExpiry(t *testing.T) {
	rc, r := otpRedisFixture(t)
	ctx := context.Background()
	app := "test:" + uuid.NewString()
	key := r.rlKey(app, "alice@example.test")
	t.Cleanup(func() { rc.Del(ctx, key) })
	rc.AddHook(otpExpiryFault{})
	count, err := r.RecipientRateLimit(ctx, app, "alice@example.test", 3600)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Positive(t, rc.PTTL(ctx, key).Val())
	var wg sync.WaitGroup
	values := make(chan int, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, e := r.RecipientRateLimit(ctx, app, "alice@example.test", 3600)
			if e == nil {
				values <- n
			}
		}()
	}
	wg.Wait()
	close(values)
	seen := map[int]bool{}
	for n := range values {
		seen[n] = true
	}
	require.Len(t, seen, 20)
	require.Positive(t, rc.PTTL(ctx, key).Val())
}
