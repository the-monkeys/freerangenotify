package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/billingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/handlers"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
	"net/http/httptest"
)

// Opt-in only: point these variables at disposable ES/Redis services, never a
// production cluster. Cleanup is limited to this fixture's generated IDs.
type creditFixture struct {
	es        *elasticsearch.Client
	redis     *redis.Client
	subs      license.Repository
	balances  *billingrepo.SubscriptionCreditBalanceRepo
	ledger    *billingrepo.ESCreditLedgerRepo
	tenant    string
	ids       []string
	transport http.RoundTripper
}

// Each fixture gets separate physical indices, even across concurrent `go test`
// processes. The production repository APIs and scripts still run unchanged.
type creditFixtureTransport struct{ prefix string }

func (s *creditFixtureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	req := request.Clone(request.Context())
	pieces := strings.Split(strings.TrimPrefix(req.URL.Path, "/"), "/")
	indices := map[string]bool{"subscriptions": true, "credit_ledger": true, "notifications": true}
	if len(pieces) > 0 && indices[pieces[0]] {
		pieces[0] = s.prefix + pieces[0]
		req.URL.Path = "/" + strings.Join(pieces, "/")
		req.URL.RawPath = ""
	}
	if req.Body != nil && req.URL.Path == "/_bulk" {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		text := string(b)
		for index := range indices {
			text = strings.ReplaceAll(text, `"_index":"`+index+`"`, `"_index":"`+s.prefix+index+`"`)
		}
		req.Body = io.NopCloser(strings.NewReader(text))
		req.ContentLength = int64(len(text))
	}
	return http.DefaultTransport.RoundTrip(req)
}

func newCreditFixture(t *testing.T) *creditFixture {
	t.Helper()
	if os.Getenv("FRN_CREDIT_INTEGRATION") != "1" {
		t.Skip("real ES/Redis credit fixture disabled; set FRN_CREDIT_INTEGRATION=1, FRN_CREDIT_ES_URL and FRN_CREDIT_REDIS_ADDR")
	}
	esURL, addr := os.Getenv("FRN_CREDIT_ES_URL"), os.Getenv("FRN_CREDIT_REDIS_ADDR")
	if esURL == "" || addr == "" {
		t.Fatal("real credit fixture explicitly enabled but ES/Redis addresses not supplied")
	}
	tenant := "credit-fixture-" + uuid.NewString()
	transport := &creditFixtureTransport{prefix: tenant + "-"}
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{esURL}, DisableRetry: true, Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ping, err := es.Info(es.Info.WithContext(ctx))
	if err != nil {
		t.Fatalf("real ES explicitly enabled but unavailable: %v", err)
	}
	ping.Body.Close()
	if ping.IsError() {
		t.Fatalf("real ES explicitly enabled but unavailable: status %d", ping.StatusCode)
	}
	rc := redis.NewClient(&redis.Options{Addr: addr})
	if err := rc.Ping(ctx).Err(); err != nil {
		rc.Close()
		t.Fatalf("real Redis explicitly enabled but unavailable: %v", err)
	}
	f := &creditFixture{es: es, redis: rc, tenant: tenant, transport: transport}
	templates := &database.IndexTemplates{}
	for index, template := range map[string]map[string]interface{}{"subscriptions": templates.GetSubscriptionsTemplate(), "credit_ledger": templates.GetCreditLedgerTemplate()} {
		template["settings"].(map[string]interface{})["refresh_interval"] = "50ms"
		b, _ := json.Marshal(template)
		resp, err := es.Indices.Create(index, es.Indices.Create.WithBody(strings.NewReader(string(b))))
		if err != nil {
			t.Fatal(err)
		}
		if resp.IsError() {
			t.Fatalf("create fixture index: %s", resp.String())
		}
		resp.Body.Close()
	}
	f.subs = repository.NewSubscriptionRepository(es, zap.NewNop())
	f.balances = billingrepo.NewSubscriptionCreditBalanceRepo(f.subs, zap.NewNop())
	f.ledger = billingrepo.NewESCreditLedgerRepo(es, zap.NewNop())
	t.Cleanup(func() {
		for _, id := range f.ids {
			if sub, err := f.subs.GetByID(context.Background(), id); err == nil && sub != nil {
				for resID := range sub.CreditReservationJournal {
					_ = rc.Del(context.Background(), "frn:credit:res:"+resID).Err()
					_ = rc.ZRem(context.Background(), "frn:credit:res:expiry", resID).Err()
				}
			}
		}
		for _, index := range []string{"subscriptions", "credit_ledger", "notifications"} {
			resp, err := es.Indices.Delete([]string{index})
			if err == nil {
				resp.Body.Close()
			}
		}
		for _, channel := range []string{"sms", "whatsapp"} {
			key := fmt.Sprintf("billing:dailycap:%s:%s:%s", f.tenant, channel, time.Now().UTC().Format("20060102"))
			_ = rc.Del(context.Background(), key, key+":refunds").Err()
		}
		rc.Close()
	})
	return f
}
func (f *creditFixture) sub(t *testing.T, amount int64) *license.Subscription {
	t.Helper()
	now := time.Now().UTC()
	s := &license.Subscription{ID: f.tenant + "-" + uuid.NewString(), TenantID: f.tenant, Plan: "standard", Status: license.SubscriptionStatusActive, CreditsTotal: amount, CreditsRemaining: amount, CurrentPeriodStart: now.Add(-time.Hour), CurrentPeriodEnd: now.Add(time.Hour), Metadata: map[string]interface{}{"billing_model": billing.BillingModelCredits}}
	f.ids = append(f.ids, s.ID)
	if err := f.subs.Create(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	return s
}
func (f *creditFixture) res(s *license.Subscription, amount int64) *billing.CreditReservation {
	now := time.Now().UTC()
	return &billing.CreditReservation{ID: billing.NewJournalReservationID(s.ID, uuid.NewString()), SubscriptionID: s.ID, TenantID: f.tenant, AppID: "fixture-app", NotificationID: uuid.NewString(), Channel: "sms", CreditsReserved: amount, RateCardVersion: "fixture-v1", Status: billing.CreditReservationReserved, ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now}
}
func (f *creditFixture) service(rc *redis.Client, ledger billing.CreditLedgerRepository) *services.CreditService {
	s := services.NewCreditService(f.balances, ledger, f.subs, nil, nil, nil, rc, zap.NewNop(), true)
	s.EnableReservationJournal(true)
	return s
}
func TestCreditJournalRealESAtomicAdmission(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	sub := f.sub(t, 1500)
	first := f.res(sub, 800)
	if _, err := f.balances.ReserveReservation(ctx, first); err != nil {
		t.Fatal(err)
	}
	if out, err := f.balances.ReserveReservation(ctx, first); err != nil || out.Applied {
		t.Fatalf("duplicate reserve %+v %v", out, err)
	}
	_, err := f.balances.ReserveReservation(ctx, f.res(sub, 800))
	var unavailable *billing.CreditUnavailableError
	if !errors.As(err, &unavailable) || unavailable.Reason != "credits_temporarily_reserved" || unavailable.CreditsRequired != 800 || unavailable.CreditsRemaining != 1500 || unavailable.CreditsReserved != 800 || unavailable.CreditsAvailable != 700 || unavailable.RateCardVersion != "fixture-v1" || unavailable.Error() != "insufficient credits" || !errors.Is(err, billing.ErrInsufficientCredits) {
		t.Fatalf("invalid atomic rejection %+v %v", unavailable, err)
	}
	if _, err := f.balances.TransitionReservation(ctx, first.ID, billing.CreditReservationReleased, "failure"); err != nil {
		t.Fatal(err)
	}
	second := f.res(sub, 800)
	if _, err := f.balances.ReserveReservation(ctx, second); err != nil {
		t.Fatal(err)
	}
	if _, err := f.balances.TransitionReservation(ctx, second.ID, billing.CreditReservationCommitted, ""); err != nil {
		t.Fatal(err)
	}
	_, err = f.balances.ReserveReservation(ctx, f.res(sub, 800))
	if !errors.As(err, &unavailable) || unavailable.Reason != "insufficient_credits" || unavailable.CreditsRemaining != 700 || unavailable.CreditsReserved != 0 {
		t.Fatalf("invalid exhaustion %+v %v", unavailable, err)
	}
	// Exactly sufficient and zero funds, independent of pre-read snapshots.
	last := f.res(sub, 700)
	if _, err := f.balances.ReserveReservation(ctx, last); err != nil {
		t.Fatal(err)
	}
	if _, err := f.balances.TransitionReservation(ctx, last.ID, billing.CreditReservationCommitted, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := f.balances.ReserveReservation(ctx, f.res(sub, 1)); !errors.Is(err, billing.ErrInsufficientCredits) {
		t.Fatalf("zero funds: %v", err)
	}
}
func TestCreditJournalRealESCompetingTerminalsAndRenewal(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	sub := f.sub(t, 100)
	res, sibling := f.res(sub, 1), f.res(sub, 1)
	for _, r := range []*billing.CreditReservation{res, sibling} {
		if _, err := f.balances.ReserveReservation(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	stale := *sub
	stale.Metadata = map[string]interface{}{"billing_model": "credits", "trial_accepted_at": "test"}
	if err := f.subs.Update(ctx, &stale); err != nil {
		t.Fatal(err)
	}
	if b, err := f.subs.GetByID(ctx, sub.ID); err != nil || b.CreditsReserved != 2 || len(b.CreditReservationJournal) != 2 {
		t.Fatalf("stale writer lost credits: %+v %v", b, err)
	}
	if _, err := f.balances.ClearReservedCredits(ctx, f.tenant); err == nil {
		t.Fatal("reset cleared active journal holds")
	}
	var wg sync.WaitGroup
	outcomes := make(chan *billing.CreditTransitionOutcome, 16)
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			status := billing.CreditReservationCommitted
			if i%2 == 0 {
				status = billing.CreditReservationReleased
			}
			out, err := f.balances.TransitionReservation(ctx, res.ID, status, "race")
			if err != nil {
				errs <- err
			} else {
				outcomes <- out
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	close(outcomes)
	for err := range errs {
		t.Fatal(err)
	}
	applied := 0
	var winner billing.CreditReservationStatus
	var exact int64
	for out := range outcomes {
		if out.Applied {
			applied++
			winner = out.Receipt.Reservation.Status
			exact = out.Receipt.Balance.CreditsRemaining
		}
	}
	if applied != 1 {
		t.Fatalf("terminal delta applied %d times", applied)
	}
	b, err := f.subs.GetByID(ctx, sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if b.CreditsReserved != 1 || b.CreditsRemaining != exact {
		t.Fatalf("sibling hold lost %+v winner=%s", b, winner)
	}
	// A new active subscription must not receive an old reservation's release.
	newer := f.sub(t, 500)
	newer.CurrentPeriodEnd = time.Now().UTC().Add(2 * time.Hour)
	if err := f.subs.Update(ctx, newer); err != nil {
		t.Fatal(err)
	}
	if _, err := f.balances.TransitionReservation(ctx, sibling.ID, billing.CreditReservationReleased, "renewal"); err != nil {
		t.Fatal(err)
	}
	b, err = f.subs.GetByID(ctx, newer.ID)
	if err != nil || b.CreditsRemaining != 500 || b.CreditsReserved != 0 {
		t.Fatalf("old receipt touched renewed subscription %+v %v", b, err)
	}
	out, err := f.balances.TransitionReservation(ctx, res.ID, billing.CreditReservationCommitted, "")
	if err != nil || out.Receipt.Balance.CreditsRemaining != exact {
		t.Fatalf("replay lost exact balance %+v %v", out, err)
	}
}

type creditFailOnceLedger struct {
	billing.CreditLedgerRepository
	failed bool
}

func (l *creditFailOnceLedger) Append(ctx context.Context, e *billing.CreditLedgerEntry) error {
	if !l.failed {
		l.failed = true
		return fmt.Errorf("injected ledger outage")
	}
	return l.CreditLedgerRepository.Append(ctx, e)
}
func TestCreditJournalRealESRestartAndCacheOutage(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	sub := f.sub(t, 100)
	ledger := &creditFailOnceLedger{CreditLedgerRepository: f.ledger}
	svc := f.service(f.redis, ledger)
	res, err := svc.ReserveForNotification(ctx, f.tenant, "a", "n", "inapp")
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := svc.CommitOnSuccess(ctx, res.ID)
	if err == nil || terminal == nil || terminal.Status != billing.CreditReservationCommitted {
		t.Fatalf("committed accounting outage not exposed: %+v %v", terminal, err)
	}
	// Simulate restart plus cache retention loss; durable ID must repair ledger.
	if err := f.redis.Del(ctx, "frn:credit:res:"+res.ID).Err(); err != nil {
		t.Fatal(err)
	}
	restarted := f.service(nil, f.ledger)
	restarted.EnableReservationJournal(false)
	if _, err := restarted.CommitOnSuccess(ctx, res.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.CommitOnSuccess(ctx, res.ID); err != nil {
		t.Fatal(err)
	}
	b, err := f.subs.GetByID(ctx, sub.ID)
	if err != nil || b.CreditsRemaining != 99 || b.CreditsReserved != 0 {
		t.Fatalf("double burn %+v %v", b, err)
	}
	receipt, err := f.balances.GetReservationReceipt(ctx, res.ID)
	if err != nil || !receipt.LedgerRecorded || receipt.Balance.CreditsRemaining != 99 {
		t.Fatalf("repair receipt %+v %v", receipt, err)
	}
	// A Redis outage on admission does not invalidate an ES-backed reservation.
	down := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 50 * time.Millisecond})
	defer down.Close()
	res, err = f.service(down, f.ledger).ReserveForNotification(ctx, f.tenant, "a", "n2", "inapp")
	if err != nil || res == nil {
		t.Fatalf("cache outage changed durable admission: %+v %v", res, err)
	}
	if _, err := restarted.CommitOnSuccess(ctx, res.ID); err != nil {
		t.Fatal(err)
	}
}
func TestCreditJournalRealESAllocations(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	sub := f.sub(t, 100)
	// Old-mode purchase/upgrade may intentionally allocate within the same period.
	for _, n := range []int64{200, 300} {
		sub.CreditsTotal = n
		sub.CreditsRemaining = n
		if err := f.subs.Update(ctx, sub); err != nil {
			t.Fatal(err)
		}
		b, err := f.subs.GetByID(ctx, sub.ID)
		if err != nil || b.CreditsRemaining != n {
			t.Fatalf("old allocation lost %+v %v", b, err)
		}
	}
	sub.CurrentPeriodStart = time.Now().UTC()
	sub.CreditsTotal = 400
	sub.CreditsRemaining = 400
	if err := f.subs.Update(ctx, sub); err != nil {
		t.Fatal(err)
	}
	res := f.res(sub, 1)
	if _, err := f.balances.ReserveReservation(ctx, res); err != nil {
		t.Fatal(err)
	}
	allocate := f.subs.(interface {
		UpdateCreditAllocation(context.Context, *license.Subscription) error
	})
	purchase := *sub
	purchase.CreditsTotal = 500
	purchase.CreditsRemaining = 500
	purchase.Metadata = map[string]interface{}{"billing_model": "credits", "last_payment_id": "fixture-payment", "renewal_method": "razorpay"}
	if err := allocate.UpdateCreditAllocation(ctx, &purchase); err == nil {
		t.Fatal("purchase erased active hold")
	}
	if _, err := f.balances.TransitionReservation(ctx, res.ID, billing.CreditReservationCommitted, ""); err != nil {
		t.Fatal(err)
	}
	if err := allocate.UpdateCreditAllocation(ctx, &purchase); err != nil {
		t.Fatal(err)
	}
	one := f.res(sub, 1)
	if _, err := f.balances.ReserveReservation(ctx, one); err != nil {
		t.Fatal(err)
	}
	if _, err := f.balances.TransitionReservation(ctx, one.ID, billing.CreditReservationCommitted, ""); err != nil {
		t.Fatal(err)
	}
	if err := allocate.UpdateCreditAllocation(ctx, &purchase); err != nil {
		t.Fatal(err)
	}
	b, err := f.subs.GetByID(ctx, sub.ID)
	if err != nil || b.CreditsRemaining != 499 {
		t.Fatalf("payment replay replenished credits %+v %v", b, err)
	}
	// Admin renewal must not reuse the last payment's allocation identity.
	renewal := *b
	renewal.CurrentPeriodStart = time.Now().UTC()
	renewal.CreditsTotal = 600
	renewal.CreditsRemaining = 600
	renewal.Metadata["renewal_method"] = "admin_cli"
	renewal.Metadata["credit_allocation_id"] = "fixture-renewal"
	if err := allocate.UpdateCreditAllocation(ctx, &renewal); err != nil {
		t.Fatal(err)
	}
	b, err = f.subs.GetByID(ctx, sub.ID)
	if err != nil || b.CreditsRemaining != 600 {
		t.Fatalf("renewal allocation ignored %+v %v", b, err)
	}
	if _, err := f.balances.BootstrapCreditBalance(ctx, f.tenant, 500, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	b, _ = f.subs.GetByID(ctx, sub.ID)
	if b.CreditsRemaining != 600 {
		t.Fatal("bootstrap overwrote nonzero wallet")
	}
}

func TestCreditJournalRealESPaginatedRecoveryBeyondRedisTTL(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	// Bulk insertion models a missed reaper and Redis loss for >24 hours. The
	// journal receipts are authoritative; none has a Redis cache/index entry.
	var body strings.Builder
	for i := 0; i < 105; i++ {
		now := time.Now().UTC()
		id := fmt.Sprintf("%s-page-%03d", f.tenant, i)
		f.ids = append(f.ids, id)
		sub := &license.Subscription{ID: id, TenantID: f.tenant, Plan: "standard", Status: license.SubscriptionStatusExpired, CreditsTotal: 100, CreditsRemaining: 100, CreditsReserved: 1, CurrentPeriodStart: now.Add(-72 * time.Hour), CurrentPeriodEnd: now.Add(-48 * time.Hour), CreatedAt: now.Add(-72 * time.Hour), UpdatedAt: now.Add(-48 * time.Hour)}
		res := f.res(sub, 1)
		res.CreatedAt = now.Add(-48 * time.Hour)
		res.UpdatedAt = res.CreatedAt
		res.ExpiresAt = now.Add(-47 * time.Hour)
		receipt := billing.CreditReservationReceipt{Reservation: *res, Balance: billing.CreditBalance{ID: id, TenantID: f.tenant, CreditsTotal: 100, CreditsRemaining: 100, CreditsReserved: 1}}
		raw, _ := json.Marshal(receipt)
		sub.CreditReservationJournal = map[string]json.RawMessage{res.ID: raw}
		sub.CreditReservationMode = billing.CreditReservationModeJournal
		b, _ := json.Marshal(sub)
		fmt.Fprintf(&body, "{\"index\":{\"_index\":\"subscriptions\",\"_id\":%q}}\n%s\n", id, b)
	}
	// Install mapping before the bulk write, exactly as journal admission does.
	if err := f.subs.(interface{ EnsureCreditJournalMapping(context.Context) error }).EnsureCreditJournalMapping(ctx); err != nil {
		t.Fatal(err)
	}
	response, err := f.es.Bulk(strings.NewReader(body.String()), f.es.Bulk.WithRefresh("true"))
	if err != nil {
		t.Fatal(err)
	}
	var bulk struct {
		Errors bool `json:"errors"`
	}
	err = json.NewDecoder(response.Body).Decode(&bulk)
	response.Body.Close()
	if err != nil || bulk.Errors {
		t.Fatalf("bulk fixture failed %v errors=%v", err, bulk.Errors)
	}
	svc := f.service(nil, f.ledger)
	svc.EnableReservationJournal(false)
	n, err := svc.ReapExpiredReservations(ctx)
	if err != nil || n != 105 {
		t.Fatalf("authoritative recovery count=%d want105 error=%v", n, err)
	}
	for _, id := range f.ids {
		b, err := f.subs.GetByID(ctx, id)
		if err != nil || b.CreditsReserved != 0 || b.CreditsRemaining != 100 {
			t.Fatalf("expired hold not recovered %+v %v", b, err)
		}
	}
	n, err = svc.ReapExpiredReservations(ctx)
	if err != nil || n != 0 {
		t.Fatalf("duplicate recovery %d %v", n, err)
	}
}

func TestCreditJournalRealESConcurrentGrantAndBootstrap(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	sub := f.sub(t, 0)
	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := retryCreditFixtureConflict(func() error {
				_, err := f.balances.BootstrapCreditBalance(ctx, f.tenant, 100, time.Now().Add(time.Hour))
				return err
			})
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	b, err := f.subs.GetByID(ctx, sub.ID)
	if err != nil || b.CreditsRemaining != 100 {
		t.Fatalf("bootstrap applied more than once %+v %v", b, err)
	}
	hold := f.res(sub, 10)
	if _, err := f.balances.ReserveReservation(ctx, hold); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := retryCreditFixtureConflict(func() error { _, err := f.balances.GrantCreditBalance(ctx, f.tenant, 1); return err })
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	b, err = f.subs.GetByID(ctx, sub.ID)
	if err != nil || b.CreditsRemaining != 116 || b.CreditsReserved != 10 {
		t.Fatalf("atomic grant lost a delta/hold %+v %v", b, err)
	}
	if _, err := f.balances.BootstrapCreditBalance(ctx, f.tenant, 500, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	b, _ = f.subs.GetByID(ctx, sub.ID)
	if b.CreditsRemaining != 116 || b.CreditsReserved != 10 {
		t.Fatal("bootstrap reset unmatched/nonzero balance")
	}
}

// ES's bounded conflict retries may legitimately exhaust under 16 writers.
// A known 409 did not apply; retry that case only, never ambiguous I/O errors.
func retryCreditFixtureConflict(operation func() error) error {
	for attempt := 0; attempt < 8; attempt++ {
		err := operation()
		if err == nil || !strings.Contains(err.Error(), "status 409") {
			return err
		}
		if attempt == 7 {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fmt.Errorf("unreachable conflict retry")
}

func TestCreditJournalRealESQuarantinesUnmatchedOldHolds(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	sub := f.sub(t, 100)
	sub.CreditsReserved = 2
	if err := f.subs.Update(ctx, sub); err != nil {
		t.Fatal(err)
	}
	if _, err := f.balances.ReserveReservation(ctx, f.res(sub, 1)); err == nil || !strings.Contains(err.Error(), "unmatched credit reservations") {
		t.Fatalf("journal did not quarantine unmatched old reservations: %v", err)
	}
	b, err := f.subs.GetByID(ctx, sub.ID)
	if err != nil || b.CreditsReserved != 2 || b.CreditsRemaining != 100 {
		t.Fatal("quarantine changed unmatched balances")
	}
}

func TestCreditJournalRealESLedgerIsImmutable(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	row := &billing.CreditLedgerEntry{ID: f.tenant + "-ledger", TenantID: f.tenant, EntryType: billing.CreditLedgerBurn, CreditsDelta: -1, BalanceAfter: 99, CreatedAt: time.Now().UTC()}
	if err := f.ledger.Append(ctx, row); err != nil {
		t.Fatal(err)
	}
	if err := f.ledger.Append(ctx, row); err != nil {
		t.Fatalf("identical repair rejected: %v", err)
	}
	row.BalanceAfter = 98
	if err := f.ledger.Append(ctx, row); err == nil {
		t.Fatal("conflicting ledger repair replaced immutable balance")
	}
}

// Inject a lost response after the real ES operation, not a fake balance.
type creditResponseFault struct {
	mu        sync.Mutex
	action    string
	remaining int
	base      http.RoundTripper
}

func (f *creditResponseFault) RoundTrip(req *http.Request) (*http.Response, error) {
	action := ""
	if req.Body != nil && strings.Contains(req.URL.Path, "/_update/") {
		data, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body = io.NopCloser(strings.NewReader(string(data)))
		var payload struct {
			Script struct {
				Params struct {
					Action   string `json:"action"`
					Allocate bool   `json:"allocate"`
				} `json:"params"`
			} `json:"script"`
		}
		_ = json.Unmarshal(data, &payload)
		action = payload.Script.Params.Action
		if payload.Script.Params.Allocate {
			action = "allocation"
		}
	}
	if strings.Contains(req.URL.Path, "/credit_ledger/_doc/") {
		action = "ledger"
	}
	response, err := f.base.RoundTrip(req)
	if err != nil {
		return response, err
	}
	f.mu.Lock()
	drop := f.remaining > 0 && action == f.action && response.StatusCode < 300
	if drop {
		f.remaining--
	}
	f.mu.Unlock()
	if drop {
		response.Body.Close()
		return nil, fmt.Errorf("injected lost %s response after durable ES write", action)
	}
	return response, nil
}
func TestCreditJournalRealESLostWriteResponses(t *testing.T) {
	for _, action := range []string{"reserve", "transition", "ack", "ledger"} {
		t.Run(action, func(t *testing.T) {
			f := newCreditFixture(t)
			ctx := context.Background()
			sub := f.sub(t, 100)
			fault := &creditResponseFault{action: action, remaining: 1, base: f.transport}
			es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{os.Getenv("FRN_CREDIT_ES_URL")}, Transport: fault, DisableRetry: true})
			if err != nil {
				t.Fatal(err)
			}
			subs := repository.NewSubscriptionRepository(es, zap.NewNop())
			balances := billingrepo.NewSubscriptionCreditBalanceRepo(subs, zap.NewNop())
			ledger := billingrepo.NewESCreditLedgerRepo(es, zap.NewNop())
			svc := services.NewCreditService(balances, ledger, subs, nil, nil, nil, f.redis, zap.NewNop(), true)
			svc.EnableReservationJournal(true)
			res, err := svc.ReserveForNotification(ctx, f.tenant, "a", "lost-response", "inapp")
			if err != nil || res == nil {
				t.Fatalf("lost reserve response not recovered: %v", err)
			}
			_, _ = svc.CommitOnSuccess(ctx, res.ID) // result may be an acknowledged accounting outage
			restarted := f.service(nil, f.ledger)
			if _, err := restarted.CommitOnSuccess(ctx, res.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := restarted.CommitOnSuccess(ctx, res.ID); err != nil {
				t.Fatal(err)
			}
			b, err := f.subs.GetByID(ctx, sub.ID)
			if err != nil || b.CreditsRemaining != 99 || b.CreditsReserved != 0 {
				t.Fatalf("lost %s response caused duplicate delta: error=%v", action, err)
			}
			receipt, err := f.balances.GetReservationReceipt(ctx, res.ID)
			if err != nil || !receipt.LedgerRecorded || receipt.Balance.CreditsRemaining != 99 {
				t.Fatalf("lost %s response failed repair %v", action, err)
			}
			fault.mu.Lock()
			remaining := fault.remaining
			fault.mu.Unlock()
			if remaining != 0 {
				t.Fatalf("%s fault was not injected", action)
			}
		})
	}
}

func TestCreditJournalRealESDailyCapRefundIsPerReservation(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	sub := f.sub(t, 100)
	sub.Plan = "free"
	if err := f.subs.Update(ctx, sub); err != nil {
		t.Fatal(err)
	}
	svc := f.service(f.redis, f.ledger)
	a, err := svc.ReserveForNotification(ctx, f.tenant, "a", "first", "sms")
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.ReserveForNotification(ctx, f.tenant, "a", "second", "sms")
	if err != nil {
		t.Fatal(err)
	}
	c, err := svc.ReserveForNotification(ctx, f.tenant, "a", "third", "sms")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReserveForNotification(ctx, f.tenant, "a", "fourth", "sms"); !errors.Is(err, services.ErrDailyCapExceeded) {
		t.Fatalf("missing daily cap: %v", err)
	}
	if err := svc.ReleaseOnFailure(ctx, a.ID, "failure"); err != nil {
		t.Fatal(err)
	}
	if err := f.service(f.redis, f.ledger).ReleaseOnFailure(ctx, a.ID, "stale worker"); err != nil {
		t.Fatal(err)
	}
	n, err := f.redis.Get(ctx, a.DailyCapKey).Int64()
	if err != nil || n != 2 {
		t.Fatalf("duplicate release undid sibling daily cap: %d %v", n, err)
	}
	if _, err := svc.CommitOnSuccess(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CommitOnSuccess(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
}

func TestCreditJournalRealESDeliveredDuringAccountingOutage(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	sub := f.sub(t, 100)
	payload, _ := json.Marshal((&database.IndexTemplates{}).GetNotificationsTemplate())
	response, err := f.es.Indices.Create("notifications", f.es.Indices.Create.WithBody(strings.NewReader(string(payload))))
	if err != nil {
		t.Fatal(err)
	}
	if response.IsError() && response.StatusCode != 400 {
		t.Fatal(response.String())
	}
	response.Body.Close()
	notifRepo := repository.NewNotificationRepository(f.es, zap.NewNop())
	res := f.res(sub, 1)
	res.ExpiresAt = time.Now().UTC().Add(-time.Minute)
	if _, err := f.balances.ReserveReservation(ctx, res); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	delivered := &notification.Notification{NotificationID: res.NotificationID, AppID: res.AppID, UserID: "fixture-user", Channel: notification.ChannelSMS, Status: notification.StatusSent, SentAt: &now, Metadata: map[string]interface{}{"delivery_reservation_id": res.ID}}
	if err := notifRepo.Create(ctx, delivered); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		response, err := f.es.Delete("notifications", delivered.NotificationID)
		if err == nil {
			response.Body.Close()
		}
	})
	svc := f.service(nil, f.ledger)
	svc.SetReservationNotificationRepository(notifRepo)
	// Commit never reached ES during the outage; reaper must burn exactly once,
	// not refund a known delivered send. No provider action is involved.
	if n, err := svc.ReapExpiredReservations(ctx); err != nil || n != 0 {
		t.Fatalf("delivered reservation was expired instead of committed: %d %v", n, err)
	}
	receipt, err := f.balances.GetReservationReceipt(ctx, res.ID)
	if err != nil || receipt.Reservation.Status != billing.CreditReservationCommitted || receipt.Balance.CreditsRemaining != 99 {
		t.Fatalf("accounting outage not reconciled: %v", err)
	}
}

type creditFixturePaymentProvider struct {
	billing.Provider
	event billing.WebhookEvent
}

func (p *creditFixturePaymentProvider) VerifyWebhook([]byte, string) (billing.WebhookEvent, error) {
	return p.event, nil
}

func (*creditFixturePaymentProvider) VerifyPayment(context.Context, billing.PaymentVerification) error {
	return nil
}
func TestCreditJournalRealESHTTPPaymentAndRenewal(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	sub := f.sub(t, 100)
	sub.Plan = "free"
	sub.Metadata["pending_checkout_tier"] = "pro"
	sub.Metadata["pending_checkout_order_id"] = "fixture-order"
	if err := f.subs.Update(ctx, sub); err != nil {
		t.Fatal(err)
	}
	payment := handlers.NewPaymentHandler(&creditFixturePaymentProvider{}, f.subs, nil, billing.DefaultRates(), true, zap.NewNop())
	renewal := handlers.NewRenewalHandler(f.subs, nil, billing.DefaultRates(), zap.NewNop())
	app := fiber.New()
	app.Post("/payment", func(c *fiber.Ctx) error { c.Locals("user_id", f.tenant); return payment.VerifyPayment(c) })
	app.Post("/renew/:id", renewal.AdminRenew)
	request := func(path, body string, want int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		response, err := app.Test(req, 10000)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != want {
			b, _ := io.ReadAll(response.Body)
			t.Fatalf("HTTP %s status=%d want=%d body=%s", path, response.StatusCode, want, b)
		}
	}
	request("/payment", `{"razorpay_order_id":"fixture-order","razorpay_payment_id":"fixture-paid-old","razorpay_signature":"fixture"}`, 200)
	b, err := f.subs.GetByID(ctx, sub.ID)
	if err != nil || b.CreditsRemaining != 55000 {
		t.Fatalf("old HTTP purchase lost allocation: %v", err)
	}
	request("/renew/"+sub.ID, `{"plan":"starter","months":1,"reason":"fixture old-mode renewal"}`, 200)
	b, err = f.subs.GetByID(ctx, sub.ID)
	if err != nil || b.CreditsRemaining != 15000 {
		t.Fatalf("old HTTP renewal lost allocation: %v", err)
	}
	hold := f.res(b, 1)
	if _, err := f.balances.ReserveReservation(ctx, hold); err != nil {
		t.Fatal(err)
	}
	b.Metadata["pending_checkout_tier"] = "pro"
	b.Metadata["pending_checkout_order_id"] = "fixture-order-new"
	if err := f.subs.Update(ctx, b); err != nil {
		t.Fatal(err)
	}
	request("/payment", `{"razorpay_order_id":"fixture-order-new","razorpay_payment_id":"fixture-paid-new","razorpay_signature":"fixture"}`, 500)
	b, _ = f.subs.GetByID(ctx, sub.ID)
	if b.CreditsReserved != 1 || b.CreditsRemaining != 15000 {
		t.Fatal("HTTP purchase silently erased active hold")
	}
	if _, err := f.balances.TransitionReservation(ctx, hold.ID, billing.CreditReservationCommitted, ""); err != nil {
		t.Fatal(err)
	}
	request("/payment", `{"razorpay_order_id":"fixture-order-new","razorpay_payment_id":"fixture-paid-new","razorpay_signature":"fixture"}`, 200)
	b, _ = f.subs.GetByID(ctx, sub.ID)
	if b.CreditsRemaining != 55000 {
		t.Fatal("journal HTTP purchase ignored intentional allocation")
	}
	hold = f.res(b, 1)
	if _, err := f.balances.ReserveReservation(ctx, hold); err != nil {
		t.Fatal(err)
	}
	request("/renew/"+sub.ID, `{"plan":"starter","months":1,"reason":"fixture journal renewal"}`, 500)
	if _, err := f.balances.TransitionReservation(ctx, hold.ID, billing.CreditReservationReleased, "drain"); err != nil {
		t.Fatal(err)
	}
	request("/renew/"+sub.ID, `{"plan":"starter","months":1,"reason":"fixture journal renewal"}`, 200)
	b, _ = f.subs.GetByID(ctx, sub.ID)
	if b.CreditsRemaining != 15000 || b.CreditsReserved != 0 || len(b.CreditReservationJournal) != 2 {
		t.Fatal("journal HTTP renewal lost allocation or history")
	}
}

type creditSourceFailureTransport struct{}

func (creditSourceFailureTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("injected authoritative source outage")
}
func TestCreditJournalRealESLifecycleSourceFailures(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	sub := f.sub(t, 100)
	hold := f.res(sub, 1)
	if _, err := f.balances.ReserveReservation(ctx, hold); err != nil {
		t.Fatal(err)
	}
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{os.Getenv("FRN_CREDIT_ES_URL")}, Transport: creditSourceFailureTransport{}, DisableRetry: true})
	if err != nil {
		t.Fatal(err)
	}
	subs := repository.NewSubscriptionRepository(es, zap.NewNop())
	balance := billingrepo.NewSubscriptionCreditBalanceRepo(subs, zap.NewNop())
	svc := services.NewCreditService(balance, f.ledger, subs, nil, nil, nil, nil, zap.NewNop(), true)
	if err := svc.RecordReservationDeliveryFailure(ctx, hold.ID); err == nil || errors.Is(err, billing.ErrCreditReservationNotFound) {
		t.Fatalf("failure evidence hid source error: %v", err)
	}
	if _, err := svc.CommitOnSuccess(ctx, hold.ID); err == nil || errors.Is(err, billing.ErrCreditReservationNotFound) || errors.Is(err, billing.ErrInsufficientCredits) {
		t.Fatalf("commit converted source error to absence/depletion: %v", err)
	}
	if err := svc.ReleaseOnFailure(ctx, hold.ID, "outage"); err == nil || errors.Is(err, billing.ErrCreditReservationNotFound) {
		t.Fatalf("release hid source error: %v", err)
	}
	if _, err := svc.ReapExpiredReservations(ctx); err == nil {
		t.Fatal("reaper hid authoritative outage")
	}
	b, err := f.subs.GetByID(ctx, sub.ID)
	if err != nil || b.CreditsReserved != 1 || b.CreditsRemaining != 100 {
		t.Fatal("source outage mutated accounting")
	}
}

func TestCreditJournalRealESCrossAttemptRecovery(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	sub := f.sub(t, 100)
	payload, _ := json.Marshal((&database.IndexTemplates{}).GetNotificationsTemplate())
	response, err := f.es.Indices.Create("notifications", f.es.Indices.Create.WithBody(strings.NewReader(string(payload))))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	a := f.res(sub, 1)
	a.ExpiresAt = time.Now().UTC().Add(-time.Minute)
	if _, err := f.balances.ReserveReservation(ctx, a); err != nil {
		t.Fatal(err)
	}
	// A failed provider attempt couldn't release during an authoritative outage.
	brokenES, _ := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{os.Getenv("FRN_CREDIT_ES_URL")}, Transport: creditSourceFailureTransport{}, DisableRetry: true})
	broken := billingrepo.NewSubscriptionCreditBalanceRepo(repository.NewSubscriptionRepository(brokenES, zap.NewNop()), zap.NewNop())
	if _, err := broken.TransitionReservation(ctx, a.ID, billing.CreditReservationReleased, "provider failed"); err == nil {
		t.Fatal("failure was not injected")
	}
	b := f.res(sub, 1)
	b.NotificationID = a.NotificationID
	if _, err := f.balances.ReserveReservation(ctx, b); err != nil {
		t.Fatal(err)
	}
	if _, err := f.balances.TransitionReservation(ctx, b.ID, billing.CreditReservationCommitted, ""); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	notifs := repository.NewNotificationRepository(f.es, zap.NewNop())
	if err := notifs.Create(ctx, &notification.Notification{NotificationID: a.NotificationID, AppID: a.AppID, UserID: "fixture", Channel: notification.ChannelSMS, Status: notification.StatusSent, SentAt: &now, Metadata: map[string]interface{}{"delivery_reservation_id": b.ID, "reservation_id": b.ID}}); err != nil {
		t.Fatal(err)
	}
	svc := f.service(nil, f.ledger)
	svc.SetReservationNotificationRepository(notifs)
	for i := 0; i < 2; i++ {
		_, err := svc.ReapExpiredReservations(ctx)
		if err == nil {
			t.Fatal("uncertain older attempt was not quarantined")
		}
	}
	// A failed latest status without attempt-specific definitive evidence can
	// describe an ambiguous timeout. It must not authorize a refund either.
	data, _ := json.Marshal(map[string]interface{}{"doc": map[string]interface{}{"sent_at": nil, "status": "failed", "metadata": map[string]interface{}{"reservation_id": a.ID, "delivery_reservation_id": nil}}})
	response, err = f.es.Update("notifications", a.NotificationID, strings.NewReader(string(data)), f.es.Update.WithRefresh("wait_for"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if _, err := svc.ReapExpiredReservations(ctx); err == nil {
		t.Fatal("unproven failed status fabricated refund")
	}
	balance, err := f.subs.GetByID(ctx, sub.ID)
	if err != nil || balance.CreditsRemaining != 99 || balance.CreditsReserved != 1 {
		t.Fatalf("one delivery charged failed older attempt: remaining=%d reserved=%d err=%v", balance.CreditsRemaining, balance.CreditsReserved, err)
	}
}

func TestCreditJournalRealESCapturedPaymentDrainAndReplay(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	sub := f.sub(t, 100)
	hold := f.res(sub, 1)
	if _, err := f.balances.ReserveReservation(ctx, hold); err != nil {
		t.Fatal(err)
	}
	sub.Metadata["pending_checkout_tier"] = "pro"
	sub.Metadata["pending_checkout_order_id"] = "capture-order"
	if err := f.subs.Update(ctx, sub); err != nil {
		t.Fatal(err)
	}
	provider := &creditFixturePaymentProvider{event: billing.WebhookEvent{EventType: "payment.captured", TenantID: f.tenant, Tier: "pro", OrderID: "capture-order", PaymentID: "capture-id"}}
	handler := handlers.NewPaymentHandler(provider, f.subs, nil, billing.DefaultRates(), true, zap.NewNop())
	app := fiber.New()
	app.Post("/capture", handler.HandleWebhook)
	capture := func(want int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/capture", strings.NewReader(`{"fixture":"capture"}`))
		req.Header.Set("X-Razorpay-Signature", "fixture")
		response, err := app.Test(req, 10000)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != want {
			t.Fatalf("capture status=%d want=%d", response.StatusCode, want)
		}
	}
	capture(503)
	current, _ := f.subs.GetByID(ctx, sub.ID)
	if current.CreditsReserved != 1 || current.Metadata["pending_checkout_order_id"] != "capture-order" || current.Metadata["last_payment_id"] == "capture-id" {
		t.Fatal("undurable capture was acknowledged/checkout intent lost")
	}
	if _, err := f.balances.TransitionReservation(ctx, hold.ID, billing.CreditReservationCommitted, ""); err != nil {
		t.Fatal(err)
	}
	capture(200)
	charged := f.res(sub, 1)
	if _, err := f.balances.ReserveReservation(ctx, charged); err != nil {
		t.Fatal(err)
	}
	if _, err := f.balances.TransitionReservation(ctx, charged.ID, billing.CreditReservationCommitted, ""); err != nil {
		t.Fatal(err)
	}
	capture(200)
	current, _ = f.subs.GetByID(ctx, sub.ID)
	if current.CreditsRemaining != 54999 {
		t.Fatal("duplicate capture replenished wallet")
	}
}

func TestCreditJournalRealESFailedAttemptEvidence(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	sub := f.sub(t, 100)
	a := f.res(sub, 1)
	a.ExpiresAt = time.Now().Add(-time.Minute)
	if _, err := f.balances.ReserveReservation(ctx, a); err != nil {
		t.Fatal(err)
	}
	svc := f.service(nil, f.ledger)
	recorder, ok := interface{}(svc).(interface {
		RecordReservationDeliveryFailure(context.Context, string) error
	})
	if !ok {
		t.Fatal("failure evidence API unavailable")
	}
	if err := recorder.RecordReservationDeliveryFailure(ctx, "legacy-id"); err != nil {
		t.Fatalf("legacy failure mark changed interface: %v", err)
	}
	if err := recorder.RecordReservationDeliveryFailure(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.balances.TransitionReservation(ctx, a.ID, billing.CreditReservationCommitted, ""); err == nil {
		t.Fatal("definitively failed attempt charged")
	}
	if err := recorder.RecordReservationDeliveryFailure(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	b := f.res(sub, 1)
	b.NotificationID = a.NotificationID
	if _, err := f.balances.ReserveReservation(ctx, b); err != nil {
		t.Fatal(err)
	}
	if _, err := f.balances.TransitionReservation(ctx, b.ID, billing.CreditReservationCommitted, ""); err != nil {
		t.Fatal(err)
	}
	// Notification state is unavailable: the bound failure receipt is sufficient.
	svc.SetReservationNotificationRepository(repository.NewNotificationRepository(f.es, zap.NewNop()))
	for i := 0; i < 2; i++ {
		if _, err := svc.ReapExpiredReservations(ctx); err != nil {
			t.Fatal(err)
		}
	}
	current, err := f.subs.GetByID(ctx, sub.ID)
	if err != nil || current.CreditsRemaining != 99 || current.CreditsReserved != 0 {
		t.Fatalf("failed attempt was charged/leaked: %+v %v", current, err)
	}
	if err := recorder.RecordReservationDeliveryFailure(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := recorder.RecordReservationDeliveryFailure(ctx, billing.NewJournalReservationID(sub.ID, "absent")); !errors.Is(err, billing.ErrCreditReservationNotFound) {
		t.Fatalf("missing evidence error: %v", err)
	}
}

func TestCreditJournalRealESModeBeforeFirstReceipt(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	sub := f.sub(t, 100)
	// Represents a durable cutover marker installed before any reservation.
	response, err := f.es.Update("subscriptions", sub.ID, strings.NewReader(`{"doc":{"credit_reservation_mode":"journal"}}`), f.es.Update.WithRefresh("wait_for"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	stale := *sub
	if _, err := f.balances.GrantCreditBalance(ctx, f.tenant, 25); err != nil {
		t.Fatal(err)
	}
	stale.Metadata = map[string]interface{}{"billing_model": "credits", "trial_accepted_at": "fixture"}
	if err := f.subs.Update(ctx, &stale); err != nil {
		t.Fatal(err)
	}
	current, _ := f.subs.GetByID(ctx, sub.ID)
	if current.CreditsRemaining != 125 {
		t.Fatal("pre-receipt stale writer erased atomic grant")
	}
	// A mode-marked wallet with unmatched old counters is quarantined across
	// allocations too, not only reserve admission. Never silently reset it.
	response, err = f.es.Update("subscriptions", sub.ID, strings.NewReader(`{"doc":{"credits_reserved":2}}`), f.es.Update.WithRefresh("wait_for"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	allocation := *current
	allocation.CreditsRemaining = 500
	allocation.CreditsTotal = 500
	allocation.CreditsReserved = 0
	allocation.Metadata = map[string]interface{}{"billing_model": "credits", "renewal_method": "razorpay", "last_payment_id": "first-receipt-payment"}
	updater := f.subs.(interface {
		UpdateCreditAllocation(context.Context, *license.Subscription) error
	})
	if err := updater.UpdateCreditAllocation(ctx, &allocation); err == nil {
		t.Fatal("pre-receipt allocation reset quarantined old holds")
	}
	current, _ = f.subs.GetByID(ctx, sub.ID)
	if current.CreditsRemaining != 125 || current.CreditsReserved != 2 {
		t.Fatal("quarantine changed historical balances")
	}
}

func TestCreditJournalRealESRebuiltRenewalIntent(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	sub := f.sub(t, 100)
	first := f.res(sub, 1)
	if _, err := f.balances.ReserveReservation(ctx, first); err != nil {
		t.Fatal(err)
	}
	if _, err := f.balances.TransitionReservation(ctx, first.ID, billing.CreditReservationCommitted, ""); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	fault := &creditResponseFault{action: "allocation", remaining: 1, base: f.transport}
	faultES, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{os.Getenv("FRN_CREDIT_ES_URL")}, Transport: fault, DisableRetry: true})
	if err != nil {
		t.Fatal(err)
	}
	handler := handlers.NewRenewalHandler(repository.NewSubscriptionRepository(faultES, zap.NewNop()), nil, billing.DefaultRates(), zap.NewNop())
	app.Post("/renew/:id", handler.AdminRenew)
	renew := func(key string, want int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/renew/"+sub.ID, strings.NewReader(`{"plan":"starter","months":1,"reason":"same intent"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		response, err := app.Test(req, 10000)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != want {
			t.Fatalf("renewal status%d want%d", response.StatusCode, want)
		}
	}
	renew(strings.Repeat("x", 201), 400)
	renew("stable-renewal-intent", 500) // applied, response lost
	burn := f.res(sub, 1)
	if _, err := f.balances.ReserveReservation(ctx, burn); err != nil {
		t.Fatal(err)
	}
	if _, err := f.balances.TransitionReservation(ctx, burn.ID, billing.CreditReservationCommitted, ""); err != nil {
		t.Fatal(err)
	}
	renew("stable-renewal-intent", 200)
	current, _ := f.subs.GetByID(ctx, sub.ID)
	if current.CreditsRemaining != 14999 {
		t.Fatal("rebuilt duplicate renewal replenished consumed credits")
	}
	renew("explicit-new-renewal-intent", 200)
	current, _ = f.subs.GetByID(ctx, sub.ID)
	if current.CreditsRemaining != 15000 {
		t.Fatal("new renewal intent ignored")
	}
}

func TestCreditJournalRealESMalformedReceiptDoesNotStarveRecovery(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	if err := f.subs.(interface{ EnsureCreditJournalMapping(context.Context) error }).EnsureCreditJournalMapping(ctx); err != nil {
		t.Fatal(err)
	}
	var body strings.Builder
	for i := 0; i < 102; i++ {
		now := time.Now().UTC()
		id := fmt.Sprintf("%s-damaged-%03d", f.tenant, i)
		f.ids = append(f.ids, id)
		sub := &license.Subscription{ID: id, TenantID: f.tenant, CreditsTotal: 100, CreditsRemaining: 100, CreatedAt: now, UpdatedAt: now}
		sub.CreditReservationMode = billing.CreditReservationModeJournal
		res := f.res(sub, 1)
		res.ExpiresAt = now.Add(-time.Hour)
		if i == 0 {
			sub.CreditsReserved = 1
			sub.CreditReservationJournal = map[string]json.RawMessage{"damaged": json.RawMessage(`{"reservation":"malformed"}`)}
		}
		if i == 1 || i == 101 {
			if i == 1 {
				sub.CreditsReserved = 1
			} else {
				sub.CreditsRemaining = 99
				res.Status = billing.CreditReservationCommitted
			}
			receipt := billing.CreditReservationReceipt{Reservation: *res, Balance: billing.CreditBalance{ID: id, TenantID: f.tenant, CreditsTotal: 100, CreditsRemaining: sub.CreditsRemaining, CreditsReserved: sub.CreditsReserved}}
			raw, _ := json.Marshal(receipt)
			sub.CreditReservationJournal = map[string]json.RawMessage{res.ID: raw}
		}
		raw, _ := json.Marshal(sub)
		fmt.Fprintf(&body, "{\"index\":{\"_index\":\"subscriptions\",\"_id\":%q}}\n%s\n", id, raw)
	}
	response, err := f.es.Bulk(strings.NewReader(body.String()), f.es.Bulk.WithRefresh("true"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	n, err := f.service(nil, f.ledger).ReapExpiredReservations(ctx)
	if err == nil || n != 1 {
		t.Fatalf("damaged source starved independent recovery: n%d err%v", n, err)
	}
	recovered, _ := f.subs.GetByID(ctx, f.ids[1])
	if recovered.CreditsReserved != 0 {
		t.Fatal("valid hold on damaged page leaked")
	}
	repaired, _ := f.subs.GetByID(ctx, f.ids[101])
	for id := range repaired.CreditReservationJournal {
		receipt, err := f.balances.GetReservationReceipt(ctx, id)
		if err != nil || !receipt.LedgerRecorded {
			t.Fatal("damaged earlier page blocked later terminal ledger repair")
		}
	}
}

func TestCreditJournalRealESKeyedOldModeRenewalSurvivesAdoption(t *testing.T) {
	f := newCreditFixture(t)
	ctx := context.Background()
	sub := f.sub(t, 100)
	fault := &creditResponseFault{action: "allocation", remaining: 1, base: f.transport}
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{os.Getenv("FRN_CREDIT_ES_URL")}, Transport: fault, DisableRetry: true})
	if err != nil {
		t.Fatal(err)
	}
	handler := handlers.NewRenewalHandler(repository.NewSubscriptionRepository(es, zap.NewNop()), nil, billing.DefaultRates(), zap.NewNop())
	app := fiber.New()
	app.Post("/renew/:id", handler.AdminRenew)
	renew := func(want int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/renew/"+sub.ID, strings.NewReader(`{"plan":"starter","months":1,"reason":"keyed old mode"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "old-mode-stable-intent")
		response, err := app.Test(req, 10000)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != want {
			t.Fatalf("renewal status%d want%d", response.StatusCode, want)
		}
	}
	renew(500) // old-mode allocation applied; response lost
	balance, err := f.balances.GetByTenantID(ctx, f.tenant)
	if err != nil || balance.JournalBacked {
		t.Fatalf("allocation receipt adopted reservation mode: %+v %v", balance, err)
	}
	if _, err := f.balances.ReserveCredits(ctx, f.tenant, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := f.balances.CommitReservedCredits(ctx, f.tenant, 1); err != nil {
		t.Fatal(err)
	}
	renew(200)
	current, err := f.subs.GetByID(ctx, sub.ID)
	if err != nil || current.CreditsRemaining != 14999 {
		t.Fatalf("old-mode keyed replay refilled consumed wallet: %+v %v", current, err)
	}
	hold := f.res(current, 1)
	if _, err := f.balances.ReserveReservation(ctx, hold); err != nil {
		t.Fatal(err)
	}
	if _, err := f.balances.TransitionReservation(ctx, hold.ID, billing.CreditReservationCommitted, ""); err != nil {
		t.Fatal(err)
	}
	renew(200)
	current, err = f.subs.GetByID(ctx, sub.ID)
	if err != nil || current.CreditsRemaining != 14998 {
		t.Fatalf("adoption lost old-mode renewal identity: %+v %v", current, err)
	}
}
