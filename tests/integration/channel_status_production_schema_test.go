package integration

import (
	"context"
	"encoding/json"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"testing"
	"time"
)

type productionStatusTransport struct{ index string }

func (t productionStatusTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	copy := r.Clone(r.Context())
	if strings.HasPrefix(copy.URL.Path, "/notifications") {
		copy.URL.Path = "/" + t.index + strings.TrimPrefix(copy.URL.Path, "/notifications")
		copy.URL.RawPath = ""
	}
	return http.DefaultTransport.RoundTrip(copy)
}

type productionStatusMessages struct {
	whatsapp.Repository
	whatsapp.EventRepository
}

func TestChannelStatusProductionSchema(t *testing.T) {
	_ = channelFixture(t)
	ctx := context.Background()
	logger := zap.NewNop()
	index := "phase4-status-" + uuid.NewString() + "-notifications"
	client, e := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{"http://127.0.0.1:19219"}, Transport: productionStatusTransport{index}})
	if e != nil {
		t.Fatal(e)
	}
	mapping, _ := json.Marshal((&database.IndexTemplates{}).GetNotificationsTemplate())
	res, e := client.Indices.Create(index, client.Indices.Create.WithBody(strings.NewReader(string(mapping))))
	if e != nil {
		t.Fatal(e)
	}
	if res.IsError() {
		t.Fatal("production schema creation failed")
	}
	res.Body.Close()
	defer func() {
		res, e := client.Indices.Delete([]string{index})
		if e == nil {
			res.Body.Close()
		}
	}()
	repo := repository.NewNotificationRepository(client, logger)
	events := repository.NewChannelEventRepository(client)
	messages := &productionStatusMessages{Repository: repository.NewWhatsAppRepository(client, logger), EventRepository: events}
	svc := services.NewWhatsAppService(messages, nil, repo, nil, nil, nil, logger).(whatsapp.EventProcessor)
	for _, legacy := range []bool{true, false} {
		id, app, sid := uuid.NewString(), uuid.NewString(), "wamid."+uuid.NewString()
		n := &notification.Notification{NotificationID: id, AppID: app, Channel: notification.ChannelWhatsApp, Status: notification.StatusSent, CreatedAt: time.Now().UTC(), Metadata: map[string]interface{}{"provider_message_id": sid}}
		if legacy {
			raw, _ := json.Marshal(n)
			var source map[string]interface{}
			json.Unmarshal(raw, &source)
			delete(source, "provider_message_id")
			raw, _ = json.Marshal(source)
			response, e := client.Index("notifications", strings.NewReader(string(raw)), client.Index.WithDocumentID(id), client.Index.WithRefresh("wait_for"))
			if e != nil {
				t.Fatal(e)
			}
			if response.IsError() {
				t.Fatal("legacy source fixture failed")
			}
			response.Body.Close()
		} else if e = repo.Create(ctx, n); e != nil {
			t.Fatal(e)
		}
		res, e = client.Indices.Refresh(client.Indices.Refresh.WithIndex(index))
		if e != nil {
			t.Fatal(e)
		}
		res.Body.Close()
		rows, e := repo.List(ctx, &notification.NotificationFilter{AppID: app, Channel: notification.ChannelWhatsApp, ProviderMessageID: sid, PageSize: 1})
		if e != nil || len(rows) != 1 || rows[0].NotificationID != id {
			t.Fatalf("production schema callback correlation legacy=%v rows=%d error=%v", legacy, len(rows), e)
		}
		count, countErr := repo.Count(ctx, &notification.NotificationFilter{AppID: app, Channel: notification.ChannelWhatsApp, ProviderMessageID: sid})
		if countErr != nil || count != 1 {
			t.Fatalf("production schema provider count legacy=%v count=%d error=%v", legacy, count, countErr)
		}
		wrong, e := repo.List(ctx, &notification.NotificationFilter{AppID: "different-app", Channel: notification.ChannelWhatsApp, ProviderMessageID: sid, PageSize: 1})
		if e != nil || len(wrong) != 0 {
			t.Fatal("source fallback crossed app boundary", e)
		}
		receipt, e := svc.AcceptVerifiedEvent(ctx, &whatsapp.InboundSenderBinding{ID: app, AppID: app, Provider: "meta"}, &whatsapp.VerifiedEvent{Kind: "status", Status: &whatsapp.DeliveryStatus{AppID: app, MetaMessageID: sid, Status: "delivered", Timestamp: time.Now().UTC()}})
		if e != nil {
			t.Fatal(e)
		}
		if e = svc.ProcessEventReceipt(ctx, receipt.ID); e != nil {
			t.Fatal("production-schema callback failed", e)
		}
		stored, e := repo.GetByID(ctx, id)
		if e != nil || stored.Status != notification.StatusDelivered {
			t.Fatal("production-schema delivery not projected", e)
		}
		if channelSource(t, client, index, id)["provider_message_id"] != sid {
			t.Fatal("accepted identity not indexed after status transition")
		}
		if channelSource(t, client, "channel_events", receipt.ID)["state"] != "done" {
			t.Fatal("production-schema receipt not complete")
		}
	}
}
