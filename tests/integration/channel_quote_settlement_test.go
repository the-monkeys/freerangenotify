package integration

import (
	"context"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/billingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
	"os"
	"sync"
	"testing"
	"time"
)

func TestChannelQuoteSettlementRealES(t *testing.T) {
	if os.Getenv("FRN_CREDIT_INTEGRATION") != "1" {
		t.Skip("disposable billing fixture required")
	}
	if os.Getenv("FRN_CREDIT_ES_URL") != "http://127.0.0.1:19219" || os.Getenv("FRN_CREDIT_REDIS_ADDR") != "127.0.0.1:16388" {
		t.Fatal("Phase 3 fixture must use designated disposable loopback ports")
	}
	f := newCreditFixture(t)
	f.sub(t, 100)
	ctx := context.Background()
	usage := billingrepo.NewESUsageRepo(f.es, zap.NewNop())
	if err := usage.EnsureIndex(ctx); err != nil {
		t.Fatal(err)
	}
	svc := services.NewCreditService(f.balances, f.ledger, f.subs, usage, nil, nil, f.redis, zap.NewNop(), true)
	svc.EnableReservationJournal(true)
	quote := &billing.ChannelQuote{RateCardVersion: "fixture", PricingModel: "detailed-v1", Connection: channelconnection.Snapshot{AppID: "fixture-app", Channel: notification.ChannelSMS, ConnectionID: "fixture-connection", AccountID: "fixture-account", Provider: "twilio", CredentialSource: "byoc"}, Channel: "sms", PayloadHash: "fixture-hash", Units: 2, Credits: 4, Category: "sms", Country: "IN", VendorCurrency: "INR", EstimatedVendorCostMinor: 20, VendorCostStatus: "estimated"}
	quote.ID = quote.Fingerprint()
	var wg sync.WaitGroup
	ids := make(chan string, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := svc.ReserveQuotedDelivery(ctx, f.tenant, "fixture-app", "quoted-notification", quote)
			if e != nil {
				t.Error(e)
				return
			}
			ids <- r.ID
		}()
	}
	wg.Wait()
	close(ids)
	id := ""
	for value := range ids {
		if id != "" && value != id {
			t.Fatal("replayed admission created another hold")
		}
		id = value
	}
	if id == "" {
		t.Fatal("no admission")
	}
	b, e := f.balances.GetByTenantID(ctx, f.tenant)
	if e != nil || b.CreditsReserved != 4 || b.CreditsRemaining != 100 {
		t.Fatalf("wallet %+v %v", b, e)
	}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := svc.CommitOnSuccess(ctx, id); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if e := svc.ReleaseOnFailure(ctx, id, "late callback failure"); e != nil {
		t.Fatal(e)
	}
	b, e = f.balances.GetByTenantID(ctx, f.tenant)
	if e != nil || b.CreditsRemaining != 96 || b.CreditsReserved != 0 {
		t.Fatalf("duplicate charge/refund %+v %v", b, e)
	}
	rows, e := f.ledger.ListByTenantID(ctx, f.tenant, 100)
	if e != nil || len(rows) != 1 || rows[0].CreditsDelta != -4 {
		t.Fatalf("ledger %+v %v", rows, e)
	}
	// Refresh only the fixture's physical usage index.
	res, e := f.es.Indices.Refresh(f.es.Indices.Refresh.WithIndex("usage_events"))
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	events, e := usage.GetEvents(ctx, f.tenant, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), 100)
	if e != nil || len(events) != 1 || events[0].CreditsUsed != 4 || events[0].Quote == nil || events[0].Quote.Connection.AccountID != "fixture-account" {
		t.Fatalf("usage %+v %v", events, e)
	}
}

func TestChannelQuoteLegacyLedgerRepairRealES(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	sub := f.sub(t, 100)
	r := f.res(sub, 4)
	if _, err := f.balances.ReserveReservation(ctx, r); err != nil {
		t.Fatal(err)
	}
	out, err := f.balances.TransitionReservation(ctx, r.ID, billing.CreditReservationReleased, "old failure")
	if err != nil {
		t.Fatal(err)
	}
	saved := out.Receipt.Reservation
	old := &billing.CreditLedgerEntry{ID: billing.CreditTransitionLedgerID(r.ID, billing.CreditReservationReleased), TenantID: f.tenant, AppID: r.AppID, ReservationID: r.ID, NotificationID: r.NotificationID, Channel: r.Channel, EntryType: billing.CreditLedgerRelease, CreditsDelta: 0, BalanceAfter: out.Receipt.Balance.CreditsRemaining, RateCardVersion: r.RateCardVersion, CreatedAt: saved.UpdatedAt, Metadata: map[string]interface{}{"reason": "old failure", "subscription_id": r.SubscriptionID}}
	if err := f.ledger.Append(ctx, old); err != nil {
		t.Fatal(err)
	}
	if err := f.service(f.redis, f.ledger).ReleaseOnFailure(ctx, r.ID, "old failure"); err != nil {
		t.Fatalf("old reservation's immutable ledger became incompatible: %v", err)
	}
}

func TestChannelQuoteSingleDispatchClaimRealES(t *testing.T) {
	es := channelFixture(t)
	ctx := context.Background()
	repo := repository.NewNotificationRepository(es, zap.NewNop()).(*repository.NotificationRepository)
	n := &notification.Notification{NotificationID: "quoted-claim-fixture", AppID: "claim-fixture-app", Channel: notification.ChannelSMS, Status: notification.StatusQueued, Content: notification.Content{Body: "hello"}, Metadata: map[string]interface{}{"channel_quote": "fixture-quote", "reservation_id": "fixture-reservation"}, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := repo.Create(ctx, n); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan bool, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claimed, err := repo.ClaimChannelDeliveryAttempt(ctx, n)
			if err != nil {
				t.Error(err)
			}
			results <- claimed
		}()
	}
	wg.Wait()
	close(results)
	count := 0
	for claimed := range results {
		if claimed {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("%d provider boundaries admitted for one notification", count)
	}
}
