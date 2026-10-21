package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/billingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type replyFixtureTransport struct{ prefix, target string }

func (t replyFixtureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	if t.target != "" {
		r.URL.Scheme = "http"
		r.URL.Host = strings.TrimPrefix(t.target, "http://")
	} else {
		pieces := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if len(pieces) > 0 && pieces[0] != "" && !strings.HasPrefix(pieces[0], "_") && !strings.HasPrefix(pieces[0], t.prefix) {
			pieces[0] = t.prefix + pieces[0]
			r.URL.Path = "/" + strings.Join(pieces, "/")
			r.URL.RawPath = ""
		}
	}
	return http.DefaultTransport.RoundTrip(r)
}

type replyFixtureRates struct {
	billing.RateCardManager
	cost    int64
	version string
}

func (r *replyFixtureRates) GetActiveRateCard() *billing.RateCard {
	cost := r.cost
	if cost == 0 {
		cost = 108
	}
	version := r.version
	if version == "" {
		version = "fixture"
	}
	return &billing.RateCard{Version: version, ChannelCreditCost: map[string]int64{"whatsapp": cost}}
}

type replyFixtureQueue struct {
	queue.Queue
	items []queue.NotificationQueueItem
}

func (q *replyFixtureQueue) Enqueue(_ context.Context, n queue.NotificationQueueItem) error {
	q.items = append(q.items, n)
	return nil
}
func (q *replyFixtureQueue) Acknowledge(context.Context, queue.NotificationQueueItem) error {
	return nil
}

func TestChannelInboxReplyAccountingRealES(t *testing.T) {
	if os.Getenv("FRN_CHANNEL_INTEGRATION") != "1" {
		t.Skip("requires disposable reply fixture")
	}
	if os.Getenv("FRN_CHANNEL_ES_URL") != "http://127.0.0.1:19219" || os.Getenv("FRN_PHONE_OTP_REDIS_ADDR") != "127.0.0.1:16388" {
		t.Fatal("only designated disposable loopback services permitted")
	}
	for _, tc := range []struct{ provider, mode string }{{"twilio", "journal"}, {"meta", "journal"}, {"twilio", "compat"}, {"twilio", "preupgrade"}} {
		provider := tc.provider
		t.Run(provider+"-"+tc.mode, func(t *testing.T) {
			prefix := "phase3-reply-" + uuid.NewString() + "-"
			es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{"http://127.0.0.1:19219"}, Transport: replyFixtureTransport{prefix: prefix}, DisableRetry: true})
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			logger := zap.NewNop()
			templates := &database.IndexTemplates{}
			for name, body := range map[string]map[string]interface{}{"applications": templates.GetApplicationsTemplate(), "notifications": templates.GetNotificationsTemplate(), "subscriptions": templates.GetSubscriptionsTemplate(), "credit_ledger": templates.GetCreditLedgerTemplate(), "whatsapp_messages": templates.GetWhatsAppMessagesTemplate()} {
				b, _ := json.Marshal(body)
				res, err := es.Indices.Create(name, es.Indices.Create.WithBody(strings.NewReader(string(b))))
				if err != nil {
					t.Fatal(err)
				}
				if res.IsError() {
					t.Fatalf("fixture schema %s (%d)", name, res.StatusCode)
				}
				res.Body.Close()
			}
			t.Cleanup(func() {
				res, err := es.Indices.Delete([]string{prefix + "*"})
				if err == nil {
					res.Body.Close()
				}
			})
			rc := redis.NewClient(&redis.Options{Addr: "127.0.0.1:16388"})
			defer rc.Close()
			calls := new(atomic.Int32)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if provider == "twilio" {
					a, b, _ := r.BasicAuth()
					r.ParseForm()
					if a != "fixture-account" || b != "fixture-token" || r.Form.Get("To") != "whatsapp:+919876543210" {
						t.Error("wrong reply account or recipient")
					}
					fmt.Fprint(w, `{"sid":"SMfixture","status":"queued"}`)
				} else {
					if r.Header.Get("Authorization") != "Bearer fixture-token" {
						t.Error("wrong Meta reply account")
					}
					fmt.Fprint(w, `{"messages":[{"id":"wamid.fixture"}]}`)
				}
			}))
			defer server.Close()
			apps := repository.NewApplicationRepository(es, logger)
			notifs := repository.NewNotificationRepository(es, logger)
			subs := repository.NewSubscriptionRepository(es, logger)
			subs.(interface{ SetCreditJournalModeEnabled(bool) }).SetCreditJournalModeEnabled(tc.mode != "compat")
			now := time.Now().UTC()
			tenant := "fixture-" + uuid.NewString()
			app := &application.Application{AppID: "fixture-app", AdminUserID: tenant, AppName: "Fixture", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: provider, AccountSID: "fixture-account", AuthToken: "fixture-token", FromNumber: "+919000000001", MetaWABAID: "fixture-waba", MetaPhoneNumberID: "fixture-phone", MetaAccessToken: "fixture-token"}}}
			if err := apps.Create(ctx, app); err != nil {
				t.Fatal(err)
			}
			if err := subs.Create(ctx, &license.Subscription{ID: tenant, TenantID: tenant, Plan: "standard", Status: license.SubscriptionStatusActive, CreditsTotal: 1000, CreditsRemaining: 1000, CurrentPeriodStart: now.Add(-time.Hour), CurrentPeriodEnd: now.Add(time.Hour), Metadata: map[string]interface{}{"billing_model": "credits"}}); err != nil {
				t.Fatal(err)
			}
			// Empty encrypted index makes the production legacy reader select BYOC.
			res, err := es.Indices.Create("channel_connections")
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			connections := &services.ChannelConnectionService{Apps: apps, Repo: repository.NewChannelConnectionRepository(es)}
			rates := &replyFixtureRates{}
			dispatch := &services.ChannelDispatchService{Connections: connections, Quotes: &services.ChannelQuoteService{Rates: rates}, Client: &http.Client{Transport: replyFixtureTransport{target: server.URL}}, Logger: logger}
			inbox := repository.NewWhatsAppRepository(es, logger)
			q := &replyFixtureQueue{}
			reply := &services.ChannelInboxReplyService{Dispatch: dispatch, Notifications: notifs, Queue: q, Inbox: inbox}
			name := "HXfixture"
			messageID := "SMfixture"
			if provider == "meta" {
				name = "approved_hello"
				messageID = "wamid.fixture"
			}
			n, err := reply.EnqueueReply(ctx, app.AppID, "919876543210", "hello", name, "one-reply")
			if err != nil {
				t.Fatal(err)
			}
			usage := billingrepo.NewESUsageRepo(es, logger)
			if err := usage.EnsureIndex(ctx); err != nil {
				t.Fatal(err)
			}
			balances := billingrepo.NewSubscriptionCreditBalanceRepo(subs, logger)
			ledger := billingrepo.NewESCreditLedgerRepo(es, logger)
			credits := services.NewCreditService(balances, ledger, subs, usage, apps, rates, rc, logger, true)
			credits.EnableReservationJournal(tc.mode != "compat")
			credits.SetReservationNotificationRepository(notifs)
			if tc.mode == "preupgrade" {
				old, e := credits.ReserveForNotification(ctx, tenant, app.AppID, n.NotificationID, "whatsapp")
				if e != nil {
					t.Fatal(e)
				}
				delete(n.Metadata, billing.ChannelQuoteMetadataKey)
				n.Metadata["reservation_id"] = old.ID
				if e := notifs.Update(ctx, n); e != nil {
					t.Fatal(e)
				}
				rates.cost = 200
				rates.version = "fixture-next"
			}
			overlap := &replyOverlapRepo{NotificationRepository: notifs.(*repository.NotificationRepository), ready: make(chan struct{})}
			proc := NewNotificationProcessor(q, overlap, nil, apps, nil, nil, nil, nil, credits, rc, logger, ProcessorConfig{MaxRetries: 3}, nil)
			proc.SetChannelDispatch(dispatch)
			proc.SetChannelInbox(inbox)
			var wg sync.WaitGroup
			for i := 0; i < 2; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					proc.processNotification(ctx, &queue.NotificationQueueItem{NotificationID: n.NotificationID, AppID: n.AppID}, logger)
				}()
			}
			wg.Wait()
			proc.processNotification(ctx, &queue.NotificationQueueItem{NotificationID: n.NotificationID, AppID: n.AppID}, logger)
			saved, err := notifs.GetByID(ctx, n.NotificationID)
			if err != nil || saved.Status != notification.StatusSent || saved.Metadata["provider_message_id"] != messageID || calls.Load() != 1 {
				t.Fatalf("reply delivery/identity mismatch %+v calls=%d err=%v", saved, calls.Load(), err)
			}
			wallet, err := balances.GetByTenantID(ctx, tenant)
			if err != nil || wallet.CreditsRemaining != 892 || wallet.CreditsReserved != 0 {
				t.Fatalf("reply wallet %+v %v", wallet, err)
			}
			rows, err := ledger.ListByTenantID(ctx, tenant, 10)
			if err != nil || len(rows) != 1 || rows[0].CreditsDelta != -108 {
				t.Fatalf("reply ledger %+v %v", rows, err)
			}
			refresh, err := es.Indices.Refresh(es.Indices.Refresh.WithIndex("usage_events"))
			if err != nil {
				t.Fatal(err)
			}
			refresh.Body.Close()
			events, err := usage.GetEvents(ctx, tenant, now.Add(-time.Hour), time.Now().Add(time.Hour), 10)
			if tc.mode != "compat" && (err != nil || len(events) != 1 || events[0].CreditsUsed != 108) {
				t.Fatalf("reply usage %+v %v", events, err)
			}
			msg, err := inbox.GetByMetaMessageID(ctx, messageID)
			if err != nil || msg == nil || msg.NotificationID != n.NotificationID || msg.DeliveryState != "sent" {
				t.Fatalf("reply conversation %+v %v", msg, err)
			}
		})
	}
}

// Both workers observe the queued row before either can claim processing.
type replyOverlapRepo struct {
	*repository.NotificationRepository
	reads atomic.Int32
	ready chan struct{}
}

func (r *replyOverlapRepo) GetByID(ctx context.Context, id string) (*notification.Notification, error) {
	n, e := r.NotificationRepository.GetByID(ctx, id)
	count := r.reads.Add(1)
	if count <= 2 {
		if count == 2 {
			close(r.ready)
		}
		select {
		case <-r.ready:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return n, e
}
func TestPhase3ReviewProcessingLeaseRealES(t *testing.T) {
	if os.Getenv("FRN_CHANNEL_INTEGRATION") != "1" {
		t.Skip("disposable fixture required")
	}
	if os.Getenv("FRN_CHANNEL_ES_URL") != "http://127.0.0.1:19219" {
		t.Fatal("only designated loopback fixture allowed")
	}
	prefix := "phase3-claim-" + uuid.NewString() + "-"
	es, e := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{"http://127.0.0.1:19219"}, Transport: replyFixtureTransport{prefix: prefix}})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		res, e := es.Indices.Delete([]string{prefix + "*"})
		if e == nil {
			res.Body.Close()
		}
	})
	repo := repository.NewNotificationRepository(es, zap.NewNop()).(*repository.NotificationRepository)
	ctx := context.Background()
	n := &notification.Notification{NotificationID: "n", AppID: "a", Channel: notification.ChannelSMS, Status: notification.StatusQueued, CreatedAt: time.Now(), Metadata: map[string]interface{}{"worker_claim": "one"}}
	if e = repo.Create(ctx, n); e != nil {
		t.Fatal(e)
	}
	claimed, e := repo.ClaimNotificationProcessing(ctx, n)
	if e != nil || !claimed {
		t.Fatalf("first claim %v %v", claimed, e)
	}
	stale := *n
	stale.Metadata = map[string]interface{}{"worker_claim": "two"}
	if won, e := repo.ClaimNotificationProcessing(ctx, &stale); e != nil || won {
		t.Fatal("overlapping processing claim admitted")
	}
	// Lease recovery fences off the original worker even before provider dispatch.
	n.Metadata["worker_claim_until"] = time.Now().Add(-time.Second).UnixMilli()
	if e := repo.Update(notification.WithProcessingClaim(ctx, "one"), n); e != nil {
		t.Fatal(e)
	}
	if won, e := repo.ClaimNotificationProcessing(ctx, &stale); e != nil || !won {
		t.Fatalf("expired preparation not recoverable: %v %v", won, e)
	}
	if e := repo.UpdateStatus(notification.WithProcessingClaim(ctx, "one"), "n", notification.StatusSent); e == nil {
		t.Fatal("stale worker wrote after lease recovery")
	}
	if won, e := repo.ClaimChannelDeliveryAttempt(ctx, n); e != nil || won {
		t.Fatal("stale worker crossed provider boundary")
	}
	stale.Status = notification.StatusSent
	if e := repo.Update(notification.WithProcessingClaim(ctx, "two"), &stale); e != nil {
		t.Fatal(e)
	}
	if won, e := repo.ClaimNotificationProcessing(ctx, n); e != nil || won {
		t.Fatal("successful notification reclaimed")
	}
	saved, e := repo.GetByID(ctx, "n")
	if e != nil || saved.Status != notification.StatusSent {
		t.Fatal("winning status lost")
	}
}
