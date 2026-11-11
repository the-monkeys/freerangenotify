package integration

import (
	"context"
	"encoding/json"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
	"strings"
	"testing"
)

func TestChannelAdmittedStatusLookupRealES(t *testing.T) {
	_ = channelFixture(t)
	ctx := context.Background()
	index := "phase5-status-" + uuid.NewString() + "-notifications"
	client, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{"http://127.0.0.1:19219"}, Transport: productionStatusTransport{index}})
	if err != nil {
		t.Fatal(err)
	}
	mapping, _ := json.Marshal((&database.IndexTemplates{}).GetNotificationsTemplate())
	res, err := client.Indices.Create(index, client.Indices.Create.WithBody(strings.NewReader(string(mapping))))
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError() {
		t.Fatal("fixture mapping failed")
	}
	res.Body.Close()
	defer func() {
		r, e := client.Indices.Delete([]string{index})
		if e == nil {
			r.Body.Close()
		}
	}()
	repo := repository.NewNotificationRepository(client, zap.NewNop())
	lookup, ok := any(repo).(interface {
		FindAdmittedChannelStatus(context.Context, string, string, string, string) (*notification.Notification, error)
	})
	if !ok {
		t.Fatal("no authenticated sender-bound historical status lookup")
	}
	for _, legacy := range []bool{false, true} {
		messageID := uuid.NewString()
		id := uuid.NewString()
		n := &notification.Notification{NotificationID: id, AppID: "a", EnvironmentID: "stage", Channel: "whatsapp", Metadata: map[string]interface{}{"provider_message_id": messageID, channelconnection.DeliveryMetadataKey: channelconnection.Snapshot{ConnectionID: "c", AppID: "a", EnvironmentID: "stage", Channel: "whatsapp", Provider: "meta", AccountID: "waba", SenderID: "phone", Revision: 1}}}
		if legacy {
			raw, _ := json.Marshal(n)
			r, e := client.Index("notifications", strings.NewReader(string(raw)), client.Index.WithDocumentID(id), client.Index.WithRefresh("true"))
			if e != nil {
				t.Fatal(e)
			}
			if r.IsError() {
				t.Fatal("legacy fixture write failed")
			}
			r.Body.Close()
		} else {
			if e := repo.Create(ctx, n); e != nil {
				t.Fatal(e)
			}
			r, e := client.Indices.Refresh(client.Indices.Refresh.WithIndex("notifications"))
			if e != nil {
				t.Fatal(e)
			}
			r.Body.Close()
		}
		row, e := lookup.FindAdmittedChannelStatus(ctx, "meta", "waba", "phone", messageID)
		if e != nil || row == nil || row.AppID != "a" || row.NotificationID != id {
			t.Fatalf("historical lookup legacy=%v failed: %v", legacy, e)
		}
		for _, scope := range [][3]string{{"twilio", "waba", "phone"}, {"meta", "foreign", "phone"}, {"meta", "waba", "foreign"}} {
			row, e = lookup.FindAdmittedChannelStatus(ctx, scope[0], scope[1], scope[2], messageID)
			if e == nil && row != nil {
				t.Fatal("status lookup crossed original provider/sender/account")
			}
		}
	}
}
