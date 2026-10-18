package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

func domesticFixture() *channelconnection.Snapshot {
	return &channelconnection.Snapshot{AppID: "a", ConnectionID: "c", EnvironmentID: "default", Revision: 1, Channel: notification.ChannelSMS, Provider: "exotel", CredentialSource: "byoc", AccountID: "account", SenderID: "FRNAPP", Readiness: "ready", Credentials: map[string]string{"api_key": "fixture-key", "api_token": "fixture-token"}, Exotel: &channelconnection.DomesticSMSConfig{Region: "mumbai", EntityID: "1234567890123456789", SMSType: "transactional", VerificationMessageID: "sms-probe", ApprovalReference: "https://example.test/approval", ApprovedAt: "2026-10-01T00:00:00Z", Templates: []channelconnection.DLTTemplate{{ID: "1234567890123456780", Body: "Hello {#var#} from FRN", VariableMaxLength: 30}}, Limits: channelconnection.DeliveryLimits{MessagesPerMinute: 2, UnitsPerMinute: 3, DailyUnits: 10000, DailySpendMinor: 100000, CostMinorPerUnit: 10, Currency: "INR", MaxSegments: 3, RateReference: "https://example.test/rate", RateVerifiedAt: "2026-10-01T00:00:00Z"}}}
}
func phase4Redis(t *testing.T) *redis.Client {
	t.Helper()
	if os.Getenv("FRN_PHASE4_INTEGRATION") != "1" {
		t.Skip("requires designated disposable Redis")
	}
	if os.Getenv("FRN_PHASE4_REDIS_ADDR") != "127.0.0.1:16389" {
		t.Fatal("only Phase 4 loopback fixture allowed")
	}
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:16389"})
	t.Cleanup(func() { c.Close() })
	return c
}
func TestExotelDeliveryBudget(t *testing.T) {
	c := phase4Redis(t)
	ctx := context.Background()
	now := time.Now().UTC()
	budget := &ChannelDeliveryBudget{Redis: c, Now: func() time.Time { return now }}
	a := domesticFixture()
	a.AccountID = uuid.NewString()
	b := domesticFixture()
	b.AccountID = uuid.NewString()
	var wg sync.WaitGroup
	var mu sync.Mutex
	admitted := 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if budget.AcquireDeliveryBudget(ctx, a, 1) == nil {
				mu.Lock()
				admitted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if admitted != 2 {
		t.Fatalf("sender admitted %d instead of 2", admitted)
	}
	if e := budget.AcquireDeliveryBudget(ctx, b, 1); e != nil {
		t.Fatal("busy account consumed another account's allowance")
	}
	now = now.Add(61 * time.Second)
	if e := budget.AcquireDeliveryBudget(ctx, a, 2); e != nil {
		t.Fatal(e)
	}
	if e := budget.AcquireDeliveryBudget(ctx, a, 2); e == nil {
		t.Fatal("unit capacity ignored")
	}
	a.Exotel.Limits.DailySpendMinor = 40
	now = now.Add(61 * time.Second)
	if e := budget.AcquireDeliveryBudget(ctx, a, 1); e == nil {
		t.Fatal("daily spend cap ignored")
	}
	closed := redis.NewClient(&redis.Options{Addr: "127.0.0.1:16389"})
	closed.Close()
	budget.Redis = closed
	if e := budget.AcquireDeliveryBudget(ctx, b, 1); e == nil {
		t.Fatal("budget store failure allowed paid send")
	}
}
func TestExotelCallbackCapabilityAndRecovery(t *testing.T) {
	snap := domesticFixture()
	snap.AppID = "a"
	snap.ConnectionID = channelconnection.ID("a", "default", notification.ChannelSMS)
	c := connectionTestService()
	safec := &channelconnection.Connection{ID: snap.ConnectionID, AppID: "a", EnvironmentID: "default", Revision: 1, Channel: notification.ChannelSMS, Provider: "exotel", CredentialSource: "byoc", AccountID: snap.AccountID, SenderID: snap.SenderID, Readiness: "ready", Exotel: snap.Exotel, Secrets: map[string]channelconnection.EncryptedSecret{}}
	for key, value := range snap.Credentials {
		sealed, e := c.Codec.Seal([]byte(value), c.binding(safec, key))
		if e != nil {
			t.Fatal(e)
		}
		safec.Secrets[key] = sealed
	}
	c.Repo.(*connectionMemory).Save(context.Background(), safec, 0)
	safe := *snap
	safe.Credentials = nil
	wire, _ := json.Marshal(safe)
	var payload map[string]interface{}
	json.Unmarshal(wire, &payload)
	n := &notification.Notification{NotificationID: "n", AppID: "a", Channel: notification.ChannelSMS, Status: notification.StatusSent, Content: notification.Content{Body: "Hello Asha from FRN"}, Metadata: map[string]interface{}{"provider_message_id": "sms-1", "channel_destination": "+919876543210", channelconnection.DeliveryMetadataKey: payload}}
	repo := &channelStatusRepo{row: n}
	events := &channelMemoryEvents{rows: map[string]*whatsapp.EventReceipt{}}
	available := false
	reads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads++
		key, token, _ := r.BasicAuth()
		if key != "fixture-key" || token != "fixture-token" {
			t.Error("wrong status credentials")
		}
		if !available {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, `{"SMSMessage":{"Sid":"sms-1","AccountSid":"account","From":"FRNAPP","To":"+919876543210","Body":"Hello Asha from FRN","Status":"sent","DateUpdated":"2026-10-18 12:30:00"}}`)
	}))
	defer server.Close()
	svc := &ExotelStatusService{Connections: c, Notifications: repo, Events: events, Client: &http.Client{Transport: dispatchTransport{server.URL}}}
	expires := time.Now().Add(time.Hour).Unix()
	proof := ExotelCallbackProof(snap, n, expires)
	if _, e := svc.AcceptCallback(context.Background(), "n", "sms-1", "sent", expires, "forged"); e == nil {
		t.Fatal("forged callback accepted")
	}
	if reads != 0 {
		t.Fatal("forged callback queried vendor")
	}
	id, e := svc.AcceptCallback(context.Background(), "n", "sms-1", "sent", expires, proof)
	if e != nil {
		t.Fatal(e)
	}
	if e = svc.Process(context.Background(), id); e == nil || n.Status != notification.StatusSent {
		t.Fatal("unknown vendor outcome trusted callback status")
	}
	available = true
	events.rows[id].NextAttempt = time.Time{}
	if e = svc.Process(context.Background(), id); e != nil {
		t.Fatal(e)
	}
	if n.Status != notification.StatusDelivered {
		t.Fatal("verified delivery not projected")
	}
	id2, e := svc.AcceptCallback(context.Background(), "n", "sms-1", "sent", expires, proof)
	if e != nil || id2 != id || len(events.rows) != 1 {
		t.Fatal("callback replay duplicated receipt")
	}
	if _, e = svc.AcceptCallback(context.Background(), "n", "foreign", "sent", expires, proof); e == nil {
		t.Fatal("foreign message correlated")
	}
}

func TestExotelQuoteAndReadiness(t *testing.T) {
	snap := domesticFixture()
	n := &notification.Notification{NotificationID: "n", AppID: "a", Channel: notification.ChannelSMS, Content: notification.Content{Body: "Hello Asha from FRN"}}
	q, e := (&ChannelQuoteService{}).QuoteChannelDelivery(context.Background(), &channelconnection.PreparedDelivery{Snapshot: snap, Notification: n, User: &user.User{Phone: "+919876543210"}})
	if e != nil {
		t.Fatal(e)
	}
	if q.Credits != 80 || q.EstimatedVendorCostMinor != 10 || q.VendorCurrency != "INR" || q.VendorCostStatus != "estimated" {
		t.Fatalf("flat FRN tariff lost independent provider estimate: %+v", q)
	}
	reads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads++
		if r.Method != "GET" {
			t.Fatal("readiness attempted paid send")
		}
		fmt.Fprint(w, `{"SMSMessage":{"Sid":"sms-probe","AccountSid":"account","From":"VN/FRNAPP","To":"9876543210","Body":"Hello Asha from FRN","Status":"sent"}}`)
	}))
	defer server.Close()
	v := &ConnectionValidator{PublicURL: "https://notify.example.test", Client: &http.Client{Transport: dispatchTransport{server.URL}}}
	state, _, e := v.Validate(context.Background(), snap)
	if e != nil || state != "pending" || reads != 0 {
		t.Fatal("disabled route queried vendor")
	}
	v.ExotelEnabled = true
	state, facts, e := v.Validate(context.Background(), snap)
	if e != nil || state != "ready" || reads != 1 || facts["dlt_approval"] != "owner_confirmed" {
		t.Fatalf("read-only validation failed: %s %+v %v", state, facts, e)
	}
	svc := connectionTestService()
	c, e := svc.UpdateConnection(context.Background(), channelconnection.ID("a", "default", notification.ChannelSMS), 0, channelconnection.Patch{AppID: "a", Channel: notification.ChannelSMS, Provider: textPointer("exotel"), AccountID: textPointer("account"), SenderID: textPointer("FRNAPP"), Exotel: snap.Exotel, Secrets: snap.Credentials})
	if e != nil {
		t.Fatal(e)
	}
	resolved, e := svc.ResolveRevision(context.Background(), c.ID, c.Revision)
	if e != nil || resolved.Exotel == nil || resolved.Credentials["api_token"] != "fixture-token" {
		t.Fatal("encrypted Exotel revision not readable")
	}
}

func TestExotelDelivery6000Queue(t *testing.T) {
	c := phase4Redis(t)
	ctx := context.Background()
	const total = 6000
	// A distinct Redis logical database isolates the real queue fixture.
	isolated := redis.NewClient(&redis.Options{Addr: "127.0.0.1:16389", DB: 14})
	defer isolated.Close()
	if isolated.DBSize(ctx).Val() != 0 {
		t.Fatal("simulation database must be empty")
	}
	defer isolated.FlushDB(ctx)
	q := queue.NewRedisQueue(isolated, zap.NewNop())
	snap := domesticFixture()
	snap.AccountID = uuid.NewString()
	l := &snap.Exotel.Limits
	l.MessagesPerMinute = 200
	l.UnitsPerMinute = 200
	now := time.Now().UTC()
	start := now
	budget := &ChannelDeliveryBudget{Redis: c, Now: func() time.Time { return now }}
	var accepted int
	seen := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		id := r.FormValue("CustomField")
		if seen[id] {
			t.Error("duplicate dispatch")
		}
		seen[id] = true
		accepted++
		fmt.Fprintf(w, `{"SMSMessage":{"Sid":"%s","AccountSid":"%s","Status":"queued"}}`, id, snap.AccountID)
	}))
	defer server.Close()
	provider, e := providers.NewExotelProvider(providers.ExotelConfig{Config: providers.Config{HTTPClient: &http.Client{Transport: dispatchTransport{server.URL}}}, Snapshot: snap}, zap.NewNop())
	if e != nil {
		t.Fatal(e)
	}
	prep := time.Now()
	items := make([]queue.NotificationQueueItem, total)
	for i := range items {
		items[i] = queue.NotificationQueueItem{NotificationID: fmt.Sprintf("simulation-%d", i), AppID: "a", Priority: notification.PriorityNormal}
	}
	if e = q.EnqueueBatch(ctx, items); e != nil {
		t.Fatal(e)
	}
	prepTime := time.Since(prep)
	processing := time.Now()
	windowCount := 0
	windows := 1
	for i := 0; i < total; i++ {
		item, e := q.Dequeue(ctx)
		if e != nil || item == nil {
			t.Fatal("queue lost item", e)
		}
		budgetCtx := WithDeliveryBudgetIdentity(ctx, item.NotificationID)
		if e = budget.AcquireDeliveryBudget(budgetCtx, snap, 1); e != nil {
			var deferred *DeliveryBudgetDeferredError
			if !errors.As(e, &deferred) {
				t.Fatal(e)
			}
			if windowCount > 200 {
				t.Fatal("provider ceiling exceeded")
			}
			now = now.Add(deferred.RetryAfter)
			windowCount = 0
			windows++
			if e = budget.AcquireDeliveryBudget(budgetCtx, snap, 1); e != nil {
				t.Fatal(e)
			}
		}
		n := &notification.Notification{NotificationID: item.NotificationID, AppID: "a", Content: notification.Content{Body: "Hello Asha from FRN"}}
		result, e := provider.Send(ctx, n, &user.User{Phone: "+919876543210"})
		if e != nil || result == nil || !result.Success {
			t.Fatal("mock dispatch failed", e)
		}
		windowCount++
		if e = q.Acknowledge(ctx, *item); e != nil {
			t.Fatal(e)
		}
	}
	if accepted != total || len(seen) != total || windows != 30 || now.Sub(start) < 29*time.Minute {
		t.Fatalf("wrong capacity simulation: accepted=%d windows=%d elapsed=%s", accepted, windows, now.Sub(start))
	}
	t.Logf("6000 real Redis queue / mock HTTP sends: prepare=%s processing=%s; virtual capacity=%s in %d batches of at most 200; no live delivery measured", prepTime, time.Since(processing), now.Sub(start), windows)
}
