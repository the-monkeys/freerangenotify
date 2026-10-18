package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/secrets"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type exotelStatusTransport string

func (t exotelStatusTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	copy := r.Clone(r.Context())
	copy.URL.Scheme = "http"
	copy.URL.Host = strings.TrimPrefix(string(t), "http://")
	return http.DefaultTransport.RoundTrip(copy)
}
func TestExotelStatusRealES(t *testing.T) {
	client := channelFixture(t)
	ctx := context.Background()
	logger := zap.NewNop()
	codec, e := secrets.NewConnectionCodec("fixture:" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{42}, 32)))
	if e != nil {
		t.Fatal(e)
	}
	connRepo := repository.NewChannelConnectionRepository(client)
	connections := &services.ChannelConnectionService{Repo: connRepo, Codec: codec}
	app := uuid.NewString()
	id := channelconnection.ID(app, "default", notification.ChannelSMS)
	config := &channelconnection.DomesticSMSConfig{Region: "mumbai"}
	c := &channelconnection.Connection{ID: id, AppID: app, EnvironmentID: "default", Channel: notification.ChannelSMS, Provider: "exotel", AccountID: "fixture-account", SenderID: "FRNAPP", CredentialSource: "byoc", Readiness: "ready", Revision: 1, Exotel: config, Secrets: map[string]channelconnection.EncryptedSecret{}}
	for field, value := range map[string]string{"api_key": "fixture-key", "api_token": "fixture-token"} {
		sealed, e := codec.Seal([]byte(value), secrets.SecretBinding{ConnectionID: id, AppID: app, EnvironmentID: "default", Provider: "exotel", Field: field})
		if e != nil {
			t.Fatal(e)
		}
		c.Secrets[field] = sealed
	}
	if e = connRepo.Save(ctx, c, 0); e != nil {
		t.Fatal(e)
	}
	snap, e := connections.ResolveRevision(ctx, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	snap.Credentials = nil
	wire, _ := json.Marshal(snap)
	var safe map[string]interface{}
	json.Unmarshal(wire, &safe)
	n := &notification.Notification{NotificationID: uuid.NewString(), AppID: app, Channel: notification.ChannelSMS, Status: notification.StatusSent, Content: notification.Content{Body: "Hello Asha from FRN"}, CreatedAt: time.Now().UTC(), Metadata: map[string]interface{}{channelconnection.DeliveryMetadataKey: safe, "provider_message_id": "sms-fixture", "channel_destination": "+919876543210"}}
	notifs := repository.NewNotificationRepository(client, logger)
	if e = notifs.Create(ctx, n); e != nil {
		t.Fatal(e)
	}
	available := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !available {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, `{"SMSMessage":{"Sid":"sms-fixture","AccountSid":"fixture-account","From":"VN/FRNAPP","To":"9876543210","Body":"Hello Asha from FRN","Status":"sent","DateUpdated":"2026-10-18 12:30:00"}}`)
	}))
	defer server.Close()
	events := repository.NewChannelEventRepository(client)
	rc := redis.NewClient(&redis.Options{Addr: "127.0.0.1:16388"})
	defer rc.Close()
	svc := &services.ExotelStatusService{APIBudget: &services.ChannelDeliveryBudget{Redis: rc}, Connections: connections, Notifications: notifs, Events: events, Client: &http.Client{Transport: exotelStatusTransport(server.URL)}}
	private, e := connections.ResolveRevision(ctx, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	expires := time.Now().Add(time.Hour).Unix()
	proof := services.ExotelCallbackProof(private, n, expires)
	receipt, e := svc.AcceptCallback(ctx, n.NotificationID, "sms-fixture", "sent", expires, proof)
	if e != nil {
		t.Fatal(e)
	}
	if e = svc.Process(ctx, receipt); e == nil {
		t.Fatal("provider outage lost recovery")
	}
	// The receiver and processor can restart independently using the durable store.
	restarted := repository.NewChannelEventRepository(client)
	svc.Events = restarted
	available = true
	response, e := client.Update("channel_events", receipt, strings.NewReader(`{"doc":{"next_attempt":"2000-01-01T00:00:00Z"}}`))
	if e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	whatsappID := uuid.NewString()
	now := time.Now().UTC()
	if e = events.AcceptEvent(ctx, &whatsapp.EventReceipt{ID: whatsappID, State: "pending", Steps: map[string]string{}, CreatedAt: now, LeaseUntil: now, NextAttempt: now, Event: whatsapp.VerifiedEvent{Kind: "status"}}); e != nil {
		t.Fatal(e)
	}
	channelRefresh(t, client, "channel_events")
	waIDs, e := restarted.PendingEvents(ctx, time.Now().UTC(), 100)
	if e != nil {
		t.Fatal(e)
	}
	for _, candidate := range waIDs {
		if candidate == receipt {
			t.Fatal("WhatsApp consumer selected SMS receipt")
		}
	}
	smsIDs, e := restarted.PendingEventsForKind(ctx, time.Now().UTC(), 100, "exotel_status")
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, candidate := range smsIDs {
		if candidate == whatsappID {
			t.Fatal("SMS consumer selected WhatsApp receipt")
		}
		found = found || candidate == receipt
	}
	if !found {
		t.Fatal("SMS recovery receipt missing")
	}
	if e = svc.Recover(ctx); e != nil {
		t.Fatal(e)
	}
	stored, e := notifs.GetByID(ctx, n.NotificationID)
	if e != nil || stored.Status != notification.StatusDelivered {
		t.Fatal("restart failed to project authenticated SMS delivery", e)
	}
	if channelSource(t, client, "channel_events", receipt)["state"] != "done" {
		t.Fatal("receipt not durably complete")
	}
	if _, e = svc.AcceptCallback(ctx, n.NotificationID, "sms-fixture", "sent", expires, proof); e != nil {
		t.Fatal(e)
	}
	if e = svc.Process(ctx, receipt); e != nil {
		t.Fatal(e)
	}
	// A late failure cannot override an authenticated delivered transition.
	if _, e = notifs.(interface {
		ApplyProviderDeliveryStatus(context.Context, string, string, string, notification.Status, time.Time) (bool, error)
	}).ApplyProviderDeliveryStatus(ctx, n.NotificationID, app, "sms-fixture", notification.StatusFailed, time.Now()); e != nil {
		t.Fatal(e)
	}
	stored, e = notifs.GetByID(ctx, n.NotificationID)
	if e != nil || stored.Status != notification.StatusDelivered {
		t.Fatal("late failure regressed delivery")
	}
}

type exotelSchemaTransport struct{ index string }

func (t exotelSchemaTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	copy := r.Clone(r.Context())
	if strings.HasPrefix(copy.URL.Path, "/channel_connections") {
		copy.URL.Path = "/" + t.index + strings.TrimPrefix(copy.URL.Path, "/channel_connections")
		copy.URL.RawPath = ""
	}
	return http.DefaultTransport.RoundTrip(copy)
}
func TestExotelDraftEditsRealES(t *testing.T) {
	_ = channelFixture(t) // explicit disposable endpoint guard
	ctx := context.Background()
	index := "phase4-schema-" + uuid.NewString() + "-channel_connections"
	client, e := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{"http://127.0.0.1:19219"}, Transport: exotelSchemaTransport{index}})
	if e != nil {
		t.Fatal(e)
	}
	mapping, _ := json.Marshal((&database.IndexTemplates{}).GetChannelConnectionsTemplate())
	res, e := client.Indices.Create(index, client.Indices.Create.WithBody(strings.NewReader(string(mapping))))
	if e != nil {
		t.Fatal(e)
	}
	if res.IsError() {
		t.Fatal("schema fixture creation failed")
	}
	res.Body.Close()
	defer func() {
		res, e := client.Indices.Delete([]string{index})
		if e == nil {
			res.Body.Close()
		}
	}()
	repo := repository.NewChannelConnectionRepository(client)
	id := uuid.NewString()
	c := &channelconnection.Connection{ID: id, AppID: "draft-app", EnvironmentID: "default", Channel: notification.ChannelSMS, Provider: "exotel", AccountID: "draft-account", Revision: 1, Exotel: &channelconnection.DomesticSMSConfig{Region: "mumbai", ApprovedAt: "2026-10-01T00:00:00Z", Templates: []channelconnection.DLTTemplate{{ID: "1234567890", Body: "2026-10-18"}}}}
	if e = repo.Save(ctx, c, 0); e != nil {
		t.Fatal(e)
	}
	c, e = repo.Get(ctx, id)
	if e != nil || c.Exotel == nil || c.Exotel.ApprovedAt == "" {
		t.Fatal("confirmed policy not readable", e)
	}
	c.Exotel.ApprovedAt = ""
	c.Exotel.Templates[0].Body = "Hello {#var#}"
	c.Revision = 2
	if e = repo.Save(ctx, c, 1); e != nil {
		t.Fatal("editing approved policy into a draft must remain possible", e)
	}
	current, e := repo.Get(ctx, id)
	if e != nil || current.Exotel == nil || current.Exotel.ApprovedAt != "" || current.Exotel.Templates[0].Body != "Hello {#var#}" {
		t.Fatal("edited draft not readable", e)
	}
	old, e := repo.GetRevision(ctx, id, 1)
	if e != nil || old.Exotel == nil || old.Exotel.Templates[0].Body != "2026-10-18" {
		t.Fatal("old admitted policy changed", e)
	}
	res, e = client.Indices.Refresh(client.Indices.Refresh.WithIndex(index))
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	rows, e := repo.FindAccount(ctx, "exotel", "draft-account")
	if e != nil || len(rows) != 1 || rows[0].Exotel == nil || rows[0].Exotel.Templates[0].Body != "Hello {#var#}" {
		t.Fatal("account lookup lost policy", e)
	}
}
