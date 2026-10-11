package main

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"go.uber.org/zap"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type throughputNotifications struct {
	*stubNotifRepo
	mu sync.Mutex
}

func (r *throughputNotifications) GetByID(ctx context.Context, id string) (*notification.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stubNotifRepo.GetByID(ctx, id)
}
func (r *throughputNotifications) Update(ctx context.Context, n *notification.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stubNotifRepo.Update(ctx, n)
}
func (r *throughputNotifications) UpdateStatus(ctx context.Context, id string, status notification.Status) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stubNotifRepo.UpdateStatus(ctx, id, status)
}

type throughputQueue struct {
	stubQueue
	normal       chan *queue.NotificationQueueItem
	urgent       chan *queue.NotificationQueueItem
	acknowledged chan string
}

func (q *throughputQueue) Dequeue(ctx context.Context) (*queue.NotificationQueueItem, error) {
	select {
	case item := <-q.urgent:
		return item, nil
	default:
	}
	select {
	case item := <-q.urgent:
		return item, nil
	case item := <-q.normal:
		return item, nil
	case <-ctx.Done():
		return nil, nil
	}
}
func (q *throughputQueue) Acknowledge(_ context.Context, item queue.NotificationQueueItem) error {
	q.acknowledged <- item.NotificationID
	return nil
}

type throughputProvider struct {
	deliveryTestProvider
	active atomic.Int64
	peak   atomic.Int64
	sent   atomic.Int64
}

func (p *throughputProvider) GetName() string { return "fixture" }
func (p *throughputProvider) Send(ctx context.Context, n *notification.Notification, _ *user.User) (*providers.Result, error) {
	active := p.active.Add(1)
	defer p.active.Add(-1)
	for old := p.peak.Load(); active > old && !p.peak.CompareAndSwap(old, active); old = p.peak.Load() {
	}
	select {
	case <-time.After(2 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	p.sent.Add(1)
	return providers.NewResult(n.NotificationID, 2*time.Millisecond), nil
}

// This exercises real worker loops with in-memory storage and a 2ms provider.
// ES/Redis credit correctness is verified separately; this is not a production SLA.
func TestWorkerBroadcast6000BoundedConcurrency(t *testing.T) {
	if os.Getenv("FRN_THROUGHPUT_BENCHMARK") != "1" {
		t.Skip("opt-in local throughput fixture")
	}
	for _, workers := range []int{5, 10, 20} {
		t.Run(fmt.Sprint(workers), func(t *testing.T) {
			repo := newStubNotifRepo()
			q := &throughputQueue{normal: make(chan *queue.NotificationQueueItem, 6000), urgent: make(chan *queue.NotificationQueueItem, 1), acknowledged: make(chan string, 6001)}
			for i := 0; i <= 6000; i++ {
				id := fmt.Sprint(i)
				repo.notifications[id] = &notification.Notification{NotificationID: id, AppID: "app-1", UserID: "user-1", Channel: notification.ChannelEmail, Priority: notification.PriorityNormal, Status: notification.StatusQueued, Content: notification.Content{Title: "Hello", Body: "World"}}
				if i < 6000 {
					q.normal <- makeQueueItem(id)
				}
			}
			proc := newProcessorForTest(repo, nil, map[string]*user.User{"user-1": makeUser()}, map[string]*application.Application{"app-1": makeApp()}, nil, nil)
			proc.queue = q
			proc.notifRepo = &throughputNotifications{stubNotifRepo: repo}
			proc.config.PollInterval = time.Millisecond
			provider := &throughputProvider{}
			manager := providers.NewManager(nil, nil, zap.NewNop())
			require.NoError(t, manager.RegisterProvider(provider))
			proc.providerManager = manager
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			start := time.Now()
			for i := 0; i < workers; i++ {
				proc.wg.Add(1)
				go proc.worker(ctx, i)
			}
			time.Sleep(20 * time.Millisecond)
			urgentStart := time.Now()
			q.urgent <- makeQueueItem("6000")
			var urgentLatency time.Duration
			for i := 0; i < 6001; i++ {
				select {
				case id := <-q.acknowledged:
					if id == "6000" {
						urgentLatency = time.Since(urgentStart)
					}
				case <-time.After(15 * time.Second):
					t.Fatal("worker drain stalled")
				}
			}
			cancel()
			proc.wg.Wait()
			require.Equal(t, int64(6001), provider.sent.Load())
			require.LessOrEqual(t, provider.peak.Load(), int64(workers))
			for _, n := range repo.notifications {
				require.Equal(t, notification.StatusSent, n.Status)
			}
			t.Logf("workers=%d recipients=6000 drain=%s urgent_latency=%s peak_provider_connections=%d; memory storage, 2ms mock provider, credits disabled", workers, time.Since(start), urgentLatency, provider.peak.Load())
		})
	}
}
