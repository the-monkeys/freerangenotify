package main

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"go.uber.org/zap"
)

type billingFanoutProvider struct {
	throughputProvider
	mu               sync.Mutex
	waits, latencies []time.Duration
	admitted         time.Time
	gate             chan struct{}
	ready            chan struct{}
	once             sync.Once
}

func (p *billingFanoutProvider) Send(ctx context.Context, n *notification.Notification, u *user.User) (*providers.Result, error) {
	if p.gate != nil {
		active := p.active.Add(1)
		for old := p.peak.Load(); active > old && !p.peak.CompareAndSwap(old, active); old = p.peak.Load() {
		}
		if active == 32 {
			p.once.Do(func() { close(p.ready) })
		}
		select {
		case <-p.gate:
		case <-ctx.Done():
			p.active.Add(-1)
			return nil, ctx.Err()
		}
		p.active.Add(-1)
	}
	start := time.Now()
	result, err := p.throughputProvider.Send(ctx, n, u)
	p.mu.Lock()
	p.waits = append(p.waits, start.Sub(p.admitted))
	p.latencies = append(p.latencies, time.Since(start))
	p.mu.Unlock()
	return result, err
}

type billingFanoutMeasurement struct {
	elapsed, admission, p50, p95, providerP95 time.Duration
	peak                                      int64
}

// Real Start/worker/admission/acknowledgement paths; only storage and the vendor
// are memory fixtures. These measurements are not an email inbox or ES SLA.
func runBillingFanout(tb testing.TB, hold bool) billingFanoutMeasurement {
	tb.Helper()
	const count, workers = 6000, 32
	repo := newStubNotifRepo()
	q := &throughputQueue{normal: make(chan *queue.NotificationQueueItem, count), urgent: make(chan *queue.NotificationQueueItem, 1), acknowledged: make(chan string, count)}
	admissionStart := time.Now()
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("billing-%04d", i)
		repo.notifications[id] = &notification.Notification{NotificationID: id, AppID: "app-1", UserID: "user-1", Channel: notification.ChannelEmail, Priority: notification.PriorityNormal, Status: notification.StatusQueued, Content: notification.Content{Title: "Invoice", Body: "Your invoice is ready"}}
		q.normal <- makeQueueItem(id)
	}
	admission := time.Since(admissionStart)
	proc := newProcessorForTest(repo, nil, map[string]*user.User{"user-1": makeUser()}, map[string]*application.Application{"app-1": makeApp()}, nil, nil)
	proc.queue = q
	proc.notifRepo = &throughputNotifications{stubNotifRepo: repo}
	proc.config.WorkerCount = workers
	proc.config.PollInterval = time.Millisecond
	provider := &billingFanoutProvider{admitted: time.Now()}
	if hold {
		provider.gate = make(chan struct{})
		provider.ready = make(chan struct{})
	}
	manager := providers.NewManager(nil, nil, zap.NewNop())
	require.NoError(tb, manager.RegisterProvider(provider))
	proc.providerManager = manager
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := time.Now()
	require.NoError(tb, proc.Start(ctx))
	defer func() {
		cancel()
		shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		require.NoError(tb, proc.Shutdown(shutdown))
	}()
	if hold {
		select {
		case <-provider.ready:
			close(provider.gate)
		case <-ctx.Done():
			tb.Fatal("configured 32-worker pool never reached 32 concurrent sends")
		}
	}
	seen := map[string]bool{}
	for i := 0; i < count; i++ {
		select {
		case id := <-q.acknowledged:
			require.False(tb, seen[id], "duplicate terminal acknowledgement: %s", id)
			seen[id] = true
		case <-ctx.Done():
			tb.Fatal("billing fanout stalled")
		}
	}
	elapsed := time.Since(start)
	cancel()
	proc.wg.Wait()
	require.Len(tb, seen, count)
	require.Equal(tb, int64(count), provider.sent.Load())
	require.Greater(tb, provider.peak.Load(), int64(10))
	require.LessOrEqual(tb, provider.peak.Load(), int64(workers))
	for id, n := range repo.notifications {
		require.True(tb, seen[id])
		require.Equal(tb, notification.StatusSent, n.Status)
	}
	sort.Slice(provider.waits, func(i, j int) bool { return provider.waits[i] < provider.waits[j] })
	sort.Slice(provider.latencies, func(i, j int) bool { return provider.latencies[i] < provider.latencies[j] })
	m := billingFanoutMeasurement{elapsed, admission, provider.waits[count/2], provider.waits[count*95/100], provider.latencies[count*95/100], provider.peak.Load()}
	tb.Logf("recipients=6000 workers=32 fixed_provider_latency=2ms storage=memory credits=disabled admission=%s drain=%s queue_p50=%s queue_p95=%s provider_p95=%s peak=%d throughput=%.1f/s", m.admission, m.elapsed, m.p50, m.p95, m.providerP95, m.peak, float64(count)/m.elapsed.Seconds())
	return m
}

func TestBizWorkerConfiguredConcurrency(t *testing.T) { runBillingFanout(t, true) }

func BenchmarkBizBillingFanout6000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		m := runBillingFanout(b, false)
		b.ReportMetric(float64(6000)/m.elapsed.Seconds(), "notifications/s")
		b.ReportMetric(float64(m.p95.Microseconds()), "queue-p95-us")
	}
}
