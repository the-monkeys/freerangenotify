package services

import (
	"context"
	"encoding/json"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"sync"
	"testing"
)

type replyNotifications struct {
	notification.Repository
	mu   sync.Mutex
	rows map[string]*notification.Notification
}

func (r *replyNotifications) GetByID(_ context.Context, id string) (*notification.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := r.rows[id]
	if n == nil {
		return nil, notification.ErrNotificationNotFound
	}
	b, _ := json.Marshal(n)
	copy := new(notification.Notification)
	json.Unmarshal(b, copy)
	return copy, nil
}
func (r *replyNotifications) CreateIfAbsent(_ context.Context, n *notification.Notification) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rows == nil {
		r.rows = map[string]*notification.Notification{}
	}
	if r.rows[n.NotificationID] != nil {
		return false, nil
	}
	b, _ := json.Marshal(n)
	copy := new(notification.Notification)
	json.Unmarshal(b, copy)
	r.rows[n.NotificationID] = copy
	return true, nil
}

type replyQueue struct {
	queue.Queue
	items []queue.NotificationQueueItem
}

func (q *replyQueue) Enqueue(_ context.Context, n queue.NotificationQueueItem) error {
	q.items = append(q.items, n)
	return nil
}

func TestChannelInboxReplyAccounting(t *testing.T) {
	for _, provider := range []string{"twilio", "meta"} {
		t.Run(provider, func(t *testing.T) {
			c := connectionTestService()
			if provider == "meta" {
				c.Apps.(*connectionApps).app.Settings.WhatsApp = &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "waba", MetaPhoneNumberID: "phone", MetaAccessToken: "fixture-token"}
			}
			dispatcher := &ChannelDispatchService{Connections: c}
			repo := &replyNotifications{}
			q := &replyQueue{}
			inbox := &channelMessages{rows: map[string]*whatsapp.InboundMessage{}}
			s := &ChannelInboxReplyService{Dispatch: dispatcher, Notifications: repo, Queue: q, Inbox: inbox}
			name := "HXfixture"
			if provider == "meta" {
				name = "approved_hello"
			}
			n, err := s.EnqueueReply(context.Background(), "a", "919876543210", "hello", name, "request-one")
			if err != nil {
				t.Fatal(err)
			}
			again, err := s.EnqueueReply(context.Background(), "a", "919876543210", "hello", name, "request-one")
			if err != nil || n.NotificationID != again.NotificationID || len(repo.rows) != 1 || len(inbox.rows) != 1 || n.Status != notification.StatusQueued {
				t.Fatalf("reply replay/history mismatch %+v %v", again, err)
			}
			if _, err := s.EnqueueReply(context.Background(), "a", "919876543210", "changed", name, "request-one"); err == nil {
				t.Fatal("same idempotency key accepted different content")
			}
			if _, err := s.EnqueueReply(context.Background(), "a", "919876543210", "free form", "", "outside-window"); err == nil {
				t.Fatal("expired window accepted free form")
			}
		})
	}
}
