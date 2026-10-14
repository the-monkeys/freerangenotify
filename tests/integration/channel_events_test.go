package integration

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func channelFixture(t *testing.T) *elasticsearch.Client {
	t.Helper()
	if os.Getenv("FRN_CHANNEL_INTEGRATION") != "1" {
		t.Skip("requires disposable channel fixture")
	}
	// These repositories use fixed index names. Refuse any production endpoint.
	endpoint := os.Getenv("FRN_CHANNEL_ES_URL")
	if endpoint != "http://127.0.0.1:19219" {
		t.Fatal("channel fixture must be the disposable loopback container on 19219")
	}
	client, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{endpoint}})
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range []string{"applications", "notifications", "whatsapp_messages"} {
		body := `{"mappings":{"properties":{"app_id":{"type":"keyword"},"contact_wa_id":{"type":"keyword"},"direction":{"type":"keyword"},"timestamp":{"type":"date"},"created_at":{"type":"date"},"read_at":{"type":"date"},"channel":{"type":"keyword"},"metadata":{"properties":{"provider_message_id":{"type":"keyword"}}}}}}`
		res, err := client.Indices.Create(index, client.Indices.Create.WithBody(strings.NewReader(body)))
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError() && res.StatusCode != 400 {
			t.Fatalf("fixture index failed %d", res.StatusCode)
		}
		res.Body.Close()
	}
	return client
}
func channelRefresh(t *testing.T, client *elasticsearch.Client, index string) {
	t.Helper()
	res, err := client.Indices.Refresh(client.Indices.Refresh.WithIndex(index))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
}
func channelSource(t *testing.T, client *elasticsearch.Client, index, id string) map[string]interface{} {
	t.Helper()
	res, err := client.Get(index, id)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Source map[string]interface{} `json:"_source"`
	}
	if res.IsError() || json.NewDecoder(res.Body).Decode(&out) != nil {
		t.Fatal("fixture source unavailable")
	}
	return out.Source
}

func TestChannelEventsRealES(t *testing.T) {
	client := channelFixture(t)
	ctx := context.Background()
	logger := zap.NewNop()
	t.Run("sender limit excludes disconnected records", func(t *testing.T) {
		apps := repository.NewApplicationRepository(client, logger)
		waba, phone := uuid.NewString(), uuid.NewString()
		active := ""
		for i := 0; i < 3; i++ {
			state := "disconnected"
			if i == 0 {
				state = "connected"
			}
			app := &application.Application{AppID: uuid.NewString(), Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: waba, MetaPhoneNumberID: phone, ConnectionStatus: state}}}
			if err := apps.Create(ctx, app); err != nil {
				t.Fatal(err)
			}
			if i == 0 {
				active = app.AppID
			}
		}
		resolver := services.NewChannelSenderResolver(apps, nil)
		binding, err := resolver.ResolveInboundSender(ctx, "meta", waba, phone)
		if err != nil || binding.AppID != active {
			t.Fatalf("disconnected senders blocked active owner: %v", err)
		}
	})
	t.Run("one concurrent sender owner", func(t *testing.T) {
		store := repository.NewChannelSenderBindingRepository(client)
		var winners atomic.Int32
		var wg sync.WaitGroup
		id := uuid.NewString()
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				err := store.ClaimSender(ctx, &whatsapp.InboundSenderBinding{ID: id, AppID: fmt.Sprint(i), EnvironmentID: "default", Provider: "meta", AccountID: "waba", SenderID: "phone"})
				if err == nil {
					winners.Add(1)
				}
			}(i)
		}
		wg.Wait()
		if winners.Load() != 1 {
			t.Fatalf("sender owners=%d", winners.Load())
		}
	})
	t.Run("one event lease and crash quarantine", func(t *testing.T) {
		store := repository.NewChannelEventRepository(client)
		id := uuid.NewString()
		now := time.Now().UTC()
		event := &whatsapp.EventReceipt{ID: id, State: "pending", Steps: map[string]string{}, CreatedAt: now, UpdatedAt: now, NextAttempt: now, LeaseUntil: now}
		if err := store.AcceptEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
		var winners atomic.Int32
		var claimed *whatsapp.EventReceipt
		var mu sync.Mutex
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				e, _ := store.ClaimEvent(ctx, id, time.Now())
				if e != nil {
					winners.Add(1)
					mu.Lock()
					claimed = e
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if winners.Load() != 1 {
			t.Fatalf("event leases=%d", winners.Load())
		}
		claimed.Steps["action"] = "dispatching"
		if err := store.SaveEvent(ctx, claimed); err != nil {
			t.Fatal(err)
		}
		if e, err := store.ClaimEvent(ctx, id, time.Now().Add(2*time.Minute)); err != nil || e != nil {
			t.Fatalf("ambiguous action was replayed: %v", err)
		}
		if channelSource(t, client, "channel_events", id)["state"] != "unknown" {
			t.Fatal("crash not quarantined")
		}
	})
	t.Run("duplicate inbound survives service restart and mark read", func(t *testing.T) {
		apps := repository.NewApplicationRepository(client, logger)
		app := &application.Application{AppID: uuid.NewString(), AdminUserID: "fixture-owner"}
		if err := apps.Create(ctx, app); err != nil {
			t.Fatal(err)
		}
		messages := repository.NewWhatsAppRepository(client, logger)
		events := repository.NewChannelEventRepository(client)
		upstream := uuid.NewString()
		contact := "123"
		now := time.Now().UTC()
		id := fmt.Sprintf("%x", sha256.Sum256([]byte("meta\x00"+app.AppID+"\x00"+app.AppID+"\x00inbound\x00"+upstream)))
		for i := 0; i < 2; i++ {
			svc := services.NewWhatsAppService(messages, apps, nil, nil, nil, nil, logger)
			if err := svc.HandleInbound(ctx, app.AppID, &whatsapp.InboundMessage{MetaMessageID: upstream, ContactWAID: contact, TextBody: "fixture-private", Timestamp: now}); err != nil {
				t.Fatal(err)
			}
			if err := svc.(whatsapp.EventProcessor).ProcessEventReceipt(ctx, id); err != nil {
				t.Fatal(err)
			}
		}
		channelRefresh(t, client, "whatsapp_messages")
		_, count, err := messages.List(ctx, &whatsapp.MessageFilter{AppID: app.AppID, Limit: 10})
		if err != nil || count != 1 {
			t.Fatalf("inbox rows=%d err=%v", count, err)
		}
		svc := services.NewWhatsAppService(messages, apps, nil, nil, nil, nil, logger)
		conversations, _, err := svc.ListConversations(ctx, app.AppID, 10, 0)
		if err != nil || len(conversations) != 1 || conversations[0].UnreadCount != 1 {
			t.Fatalf("unread before mark: %v %v", conversations, err)
		}
		if err := svc.MarkRead(ctx, app.AppID, contact); err != nil {
			t.Fatal(err)
		}
		conversations, _, err = svc.ListConversations(ctx, app.AppID, 10, 0)
		if err != nil || conversations[0].UnreadCount != 0 {
			t.Fatalf("read marker not reflected: %v", err)
		}
		channelRefresh(t, client, "channel_events")
		ids, err := events.PendingEvents(ctx, time.Now(), 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, pending := range ids {
			if pending == id {
				t.Fatal("completed callback remains pending")
			}
		}
	})
	t.Run("conditional status cannot regress or cross app", func(t *testing.T) {
		repo := repository.NewNotificationRepository(client, logger)
		id := uuid.NewString()
		appID := uuid.NewString()
		msgID := uuid.NewString()
		if err := repo.Create(ctx, &notification.Notification{NotificationID: id, AppID: appID, Status: notification.StatusSent, Channel: notification.ChannelWhatsApp, Metadata: map[string]interface{}{"provider_message_id": msgID}}); err != nil {
			t.Fatal(err)
		}
		writer := repo.(interface {
			ApplyProviderDeliveryStatus(context.Context, string, string, string, notification.Status, time.Time) (bool, error)
		})
		if changed, err := writer.ApplyProviderDeliveryStatus(ctx, id, "wrong-app", msgID, notification.StatusRead, time.Now()); err != nil || changed {
			t.Fatal("cross-app status applied")
		}
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				status := notification.StatusDelivered
				if i%2 == 0 {
					status = notification.StatusRead
				}
				_, err := writer.ApplyProviderDeliveryStatus(ctx, id, appID, msgID, status, time.Now())
				if err != nil {
					t.Error(err)
				}
			}(i)
		}
		wg.Wait()
		if _, err := writer.ApplyProviderDeliveryStatus(ctx, id, appID, msgID, notification.StatusFailed, time.Now()); err != nil {
			t.Fatal(err)
		}
		saved, err := repo.GetByID(ctx, id)
		if err != nil || saved.Status != notification.StatusRead {
			t.Fatalf("status regressed: %v", err)
		}
	})
	t.Run("settings revision permits one concurrent writer", func(t *testing.T) {
		repo := repository.NewApplicationRepository(client, logger)
		app := &application.Application{AppID: uuid.NewString()}
		if err := repo.Create(ctx, app); err != nil {
			t.Fatal(err)
		}
		writer := repo.(interface {
			UpdateSettingsIfRevision(context.Context, string, application.Settings, string) error
		})
		revision := app.UpdatedAt.Format(time.RFC3339Nano)
		var success atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				if writer.UpdateSettingsIfRevision(ctx, app.AppID, application.Settings{RateLimit: i + 1}, revision) == nil {
					success.Add(1)
				}
			}(i)
		}
		wg.Wait()
		if success.Load() != 1 {
			t.Fatalf("settings writers=%d", success.Load())
		}
	})
	t.Run("service window uses upstream time", func(t *testing.T) {
		redisClient := redis.NewClient(&redis.Options{Addr: "127.0.0.1:16388"})
		defer redisClient.Close()
		apps := repository.NewApplicationRepository(client, logger)
		app := &application.Application{AppID: uuid.NewString()}
		if err := apps.Create(ctx, app); err != nil {
			t.Fatal(err)
		}
		messages := repository.NewWhatsAppRepository(client, logger)
		svc := services.NewWhatsAppService(messages, apps, nil, nil, nil, redisClient, logger)
		for _, age := range []time.Duration{23 * time.Hour, 25 * time.Hour} {
			upstream := uuid.NewString()
			contact := fmt.Sprint(age)
			msg := &whatsapp.InboundMessage{MetaMessageID: upstream, ContactWAID: contact, Timestamp: time.Now().Add(-age)}
			if err := svc.HandleInbound(ctx, app.AppID, msg); err != nil {
				t.Fatal(err)
			}
			id := fmt.Sprintf("%x", sha256.Sum256([]byte("meta\x00"+app.AppID+"\x00"+app.AppID+"\x00inbound\x00"+upstream)))
			if err := svc.(whatsapp.EventProcessor).ProcessEventReceipt(ctx, id); err != nil {
				t.Fatal(err)
			}
			ttl, err := redisClient.TTL(ctx, "csw:"+app.AppID+":"+contact).Result()
			if err != nil {
				t.Fatal(err)
			}
			if age == 23*time.Hour && (ttl > time.Hour || ttl < 59*time.Minute) {
				t.Fatalf("window extended from receipt time: %s", ttl)
			}
			if age == 25*time.Hour && ttl > 0 {
				t.Fatal("old callback reopened window")
			}
		}
	})
}
