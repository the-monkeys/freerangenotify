package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/idempotency"
	"go.uber.org/zap"
)

type partialBroadcastService struct {
	notification.Service
	calls atomic.Int64
}

type broadcastReceiptFault struct {
	lose bool
	once atomic.Bool
}

func (f *broadcastReceiptFault) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	if cmd.Name() == "eval" && f.once.CompareAndSwap(false, true) {
		time.Sleep(30 * time.Millisecond)
		if f.lose {
			return ctx, fmt.Errorf("receipt write unavailable")
		}
	}
	return ctx, nil
}
func (*broadcastReceiptFault) AfterProcess(context.Context, redis.Cmder) error { return nil }
func (*broadcastReceiptFault) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}
func (*broadcastReceiptFault) AfterProcessPipeline(context.Context, []redis.Cmder) error { return nil }
func TestBroadcastConcurrentRetriesAndReceiptLossNeverRestartOperation(t *testing.T) {
	if os.Getenv("FRN_CREDIT_INTEGRATION") != "1" {
		t.Skip("opt-in disposable Redis")
	}
	addr := os.Getenv("FRN_CREDIT_REDIS_ADDR")
	require.True(t, strings.HasPrefix(addr, "127.0.0.1:"))
	for _, lose := range []bool{false, true} {
		t.Run(fmt.Sprint("receipt_lost=", lose), func(t *testing.T) {
			client := redis.NewClient(&redis.Options{Addr: addr})
			t.Cleanup(func() { client.Close() })
			require.NoError(t, client.Ping(context.Background()).Err())
			client.AddHook(&broadcastReceiptFault{lose: lose})
			appID, key := uuid.NewString(), uuid.NewString()
			receiptKey := "frn:idemp:" + appID + ":" + key
			t.Cleanup(func() { client.Del(context.Background(), receiptKey) })
			svc := &partialBroadcastService{}
			handler := NewNotificationHandler(svc, zap.NewNop())
			handler.SetIdempotencyStore(idempotency.NewStore(client, zap.NewNop()))
			app := fiber.New()
			app.Post("/broadcast", func(c *fiber.Ctx) error { c.Locals("app_id", appID); return c.Next() }, handler.Broadcast)
			send := func() error {
				req := httptest.NewRequest("POST", "/broadcast", strings.NewReader(`{"channel":"email","priority":"normal","template_id":"fixture"}`))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Idempotency-Key", key)
				res, err := app.Test(req)
				if err != nil {
					return err
				}
				defer res.Body.Close()
				if res.StatusCode != 202 && res.StatusCode != 409 {
					return fmt.Errorf("unexpected status %d", res.StatusCode)
				}
				return nil
			}
			var wg sync.WaitGroup
			var failures atomic.Int64
			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := send(); err != nil {
						t.Logf("concurrent broadcast request: %v", err)
						failures.Add(1)
					}
				}()
			}
			wg.Wait()
			require.Zero(t, failures.Load())
			require.Equal(t, int64(1), svc.calls.Load())
			if lose {
				legacy, err := idempotency.NewStore(client, zap.NewNop()).Get(context.Background(), appID, key)
				require.NoError(t, err)
				require.NotNil(t, legacy, "older binaries must see the pending receipt in the existing app/key namespace")
				require.True(t, legacy.Pending)
				require.Equal(t, time.Duration(-1), client.PTTL(context.Background(), receiptKey).Val(), "interrupted operations must not expire into an automatic replay")
			}
			require.NoError(t, send())
			require.Equal(t, int64(1), svc.calls.Load())
		})
	}
}

func (s *partialBroadcastService) Broadcast(context.Context, notification.BroadcastRequest) (*notification.BroadcastResult, error) {
	s.calls.Add(1)
	out := &notification.BroadcastResult{}
	_ = json.Unmarshal([]byte(`{"total":6001,"accepted":500,"unknown":500,"not_attempted":5001,"partial":true}`), out)
	for i := 0; i < 500; i++ {
		out.Notifications = append(out.Notifications, &notification.Notification{NotificationID: fmt.Sprint(i), Status: notification.StatusQueued})
	}
	return out, fmt.Errorf("later batch storage response lost")
}
func TestBroadcastPartialResponseIsCachedBeforeAKeyCanResendRecipients(t *testing.T) {
	if os.Getenv("FRN_CREDIT_INTEGRATION") != "1" {
		t.Skip("opt-in disposable Redis")
	}
	addr := os.Getenv("FRN_CREDIT_REDIS_ADDR")
	require.True(t, strings.HasPrefix(addr, "127.0.0.1:"))
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { client.Close() })
	appID := uuid.NewString()
	key := uuid.NewString()
	t.Cleanup(func() {
		client.Del(context.Background(), "frn:idemp:"+appID+":"+key, "frn:idemp:"+appID+":broadcast:default:"+key)
	})
	svc := &partialBroadcastService{}
	h := NewNotificationHandler(svc, zap.NewNop())
	h.SetIdempotencyStore(idempotency.NewStore(client, zap.NewNop()))
	app := fiber.New()
	app.Post("/broadcast", func(c *fiber.Ctx) error { c.Locals("app_id", appID); return c.Next() }, h.Broadcast)
	body := []byte(`{"channel":"email","priority":"normal","template_id":"fixture"}`)
	var firstBody []byte
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/broadcast", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		res, err := app.Test(req)
		require.NoError(t, err)
		var response map[string]interface{}
		require.NoError(t, json.NewDecoder(res.Body).Decode(&response))
		res.Body.Close()
		if i == 0 {
			firstBody, _ = json.Marshal(response)
		} else {
			replay, _ := json.Marshal(response)
			require.Equal(t, string(firstBody), string(replay))
		}
	}
	require.Equal(t, int64(1), svc.calls.Load(), "a retry must not repeat the confirmed first batch")
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(firstBody, &response))
	require.Equal(t, float64(500), response["sent"])
	require.Equal(t, float64(6001), response["total"])
	require.Equal(t, true, response["partial"])
}
