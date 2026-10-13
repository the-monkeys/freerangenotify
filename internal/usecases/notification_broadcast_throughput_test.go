package usecases

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"go.uber.org/zap"
	"testing"
)

type broadcastUsers struct {
	*mockUserRepo
	rows  []*user.User
	reads int
}

func (r *broadcastUsers) GetByID(ctx context.Context, id string) (*user.User, error) {
	r.reads++
	return r.mockUserRepo.GetByID(ctx, id)
}
func (r *broadcastUsers) List(_ context.Context, f user.UserFilter) ([]*user.User, error) {
	if f.Offset >= len(r.rows) {
		return nil, nil
	}
	end := f.Offset + f.Limit
	if end > len(r.rows) {
		end = len(r.rows)
	}
	return r.rows[f.Offset:end], nil
}

type batchNotifications struct {
	mockNotificationRepo
	writes      int
	failOnWrite int
	errors      []error
}

func (r *batchNotifications) BulkCreateResults(_ context.Context, rows []*notification.Notification) ([]error, error) {
	r.writes++
	if r.writes == r.failOnWrite {
		return nil, fmt.Errorf("storage response lost")
	}
	r.created = append(r.created, rows...)
	if r.errors != nil {
		return r.errors, nil
	}
	return make([]error, len(rows)), nil
}

func TestBroadcastPreservesConfirmedRecipientsWhenLaterBatchIsUnknown(t *testing.T) {
	svc, _, nr, q := throughputFixture(6001)
	nr.failOnWrite = 2
	out, err := svc.Broadcast(context.Background(), notification.BroadcastRequest{AppID: "app-1", Channel: notification.ChannelEmail, Priority: notification.PriorityNormal, TemplateID: "fixture", Title: "Hello", Body: "World"})
	require.Error(t, err)
	require.NotNil(t, out, "partial broadcast outcomes must survive the service boundary")
	require.Len(t, out.Notifications, 500)
	require.Len(t, q.batch, 500)
	require.Equal(t, 6001, out.Total)
	require.Equal(t, 500, out.Accepted)
	require.Equal(t, 500, out.Unknown)
	require.Equal(t, 5001, out.NotAttempted)
	require.True(t, out.Partial)
}

func TestBroadcastReportsKnownStorageRejections(t *testing.T) {
	svc, _, nr, _ := throughputFixture(2)
	nr.errors = []error{nil, fmt.Errorf("mapping rejected")}
	out, err := svc.Broadcast(context.Background(), notification.BroadcastRequest{AppID: "app-1", Channel: notification.ChannelEmail, Priority: notification.PriorityNormal, TemplateID: "fixture", Title: "Hello", Body: "World"})
	require.NoError(t, err)
	require.Equal(t, 2, out.Total)
	require.Equal(t, 1, out.Accepted)
	require.Equal(t, 1, out.Rejected)
	require.True(t, out.Partial)
	require.Zero(t, out.Unknown)
}
func (r *batchNotifications) BulkUpdateStatus(_ context.Context, ids []string, status notification.Status) error {
	for _, n := range r.created {
		for _, id := range ids {
			if n.NotificationID == id {
				n.Status = status
			}
		}
	}
	return nil
}

type consumingQueue struct {
	mockQueue
	repo          *batchNotifications
	invalidStatus bool
	err           error
}

func (q *consumingQueue) EnqueueBatch(_ context.Context, items []queue.NotificationQueueItem) error {
	for _, item := range items {
		for _, n := range q.repo.created {
			if n.NotificationID == item.NotificationID {
				if n.Status != notification.StatusQueued {
					q.invalidStatus = true
				}
				n.Status = notification.StatusSent
			}
		}
	}
	q.batch = append(q.batch, items...)
	return q.err
}
func throughputFixture(count int) (*NotificationService, *broadcastUsers, *batchNotifications, *consumingQueue) {
	ur := &broadcastUsers{mockUserRepo: &mockUserRepo{users: make(map[string]*user.User)}}
	for i := 0; i < count; i++ {
		u := &user.User{AppID: "app-1", UserID: fmt.Sprintf("u-%d", i), Email: fmt.Sprintf("p%d@example.com", i), Preferences: user.Preferences{EmailEnabled: boolPtr(true)}}
		ur.rows = append(ur.rows, u)
		ur.users[u.UserID] = u
	}
	nr := &batchNotifications{}
	q := &consumingQueue{repo: nr}
	svc := NewNotificationService(nr, ur, &mockAppRepo{app: &application.Application{AppID: "app-1"}}, &mockTemplateRepoNSC{}, q, zap.NewNop(), NotificationServiceConfig{}, nil, &mockLimiterSend{allowed: true}).(*NotificationService)
	return svc, ur, nr, q
}
func TestBroadcastBatchesPreparationWithoutOverwritingCompletedDeliveries(t *testing.T) {
	svc, ur, nr, q := throughputFixture(6001)
	out, err := svc.Broadcast(context.Background(), notification.BroadcastRequest{AppID: "app-1", Channel: notification.ChannelEmail, Priority: notification.PriorityNormal, TemplateID: "fixture", Title: "Hello", Body: "World"})
	require.NoError(t, err)
	require.Len(t, out.Notifications, 6001)
	require.Zero(t, ur.reads, "broadcast already fetched recipient profiles")
	require.Equal(t, 13, nr.writes)
	require.False(t, q.invalidStatus, "records must be queued before publication")
	for _, n := range nr.created {
		require.Equal(t, notification.StatusSent, n.Status, "producer must not overwrite a worker result")
	}
}
func TestBulkQueuesOnlyConfirmedStorageItems(t *testing.T) {
	svc, _, nr, q := throughputFixture(2)
	nr.errors = []error{nil, fmt.Errorf("mapping failed")}
	out, err := svc.SendBulk(context.Background(), notification.BulkSendRequest{AppID: "app-1", UserIDs: []string{"u-0", "u-1"}, Channel: notification.ChannelEmail, Priority: notification.PriorityNormal, Title: "Hello", Body: "World"})
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Len(t, q.batch, 1)
}

func BenchmarkBroadcastPreparation6000(b *testing.B) {
	svc, _, repo, _ := throughputFixture(6000)
	svc.queue = &mockQueue{}
	req := notification.BroadcastRequest{AppID: "app-1", TemplateID: "fixture", Channel: notification.ChannelEmail, Priority: notification.PriorityNormal, Title: "Hello", Body: "World"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := svc.Broadcast(context.Background(), req)
		if err != nil || len(out.Notifications) != 6000 {
			b.Fatalf("broadcast incomplete: %v", err)
		}
		repo.created = nil
	}
}
