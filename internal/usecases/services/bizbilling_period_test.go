package services

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"github.com/the-monkeys/freerangenotify/internal/testutil"
	"go.uber.org/zap"
)

type expiredPeriodPIT struct{ armed atomic.Bool }

func (f *expiredPeriodPIT) RoundTrip(r *http.Request) (*http.Response, error) {
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
	}
	if strings.HasSuffix(r.URL.Path, "/_search") && bytes.Contains(body, []byte(`"pit"`)) && f.armed.CompareAndSwap(true, false) {
		return &http.Response{StatusCode: 404, Header: http.Header{"X-Elastic-Product": []string{"Elasticsearch"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"type":"search_context_missing_exception"},"status":404}`))}, nil
	}
	return http.DefaultTransport.RoundTrip(r)
}

func periodFixture(t *testing.T, faults ...*lostBillingResponse) (*BizPeriodService, *BizLifecycleService, *BizOperationService, *elasticsearch.Client, *bizbilling.BizSubscription) {
	t.Helper()
	lifecycle, _, ops, es, scope := financialFacadesFixture(t, faults...)
	templates := &database.IndexTemplates{}
	ensureBillingIndex(t, es, "frn_biz_plans", templates.GetBizPlansTemplate())
	ensureBillingIndex(t, es, "frn_biz_subscriptions", templates.GetBizSubscriptionsTemplate())
	ensureBillingIndex(t, es, bizbillingrepo.BillingPeriodIndex, templates.GetBizBillingPeriodsTemplate())
	plan := &bizbilling.Plan{ID: uuid.NewString(), AppID: scope.AppID, Name: "Monthly service", AmountPaisa: 1000, Currency: "INR", BillingCycle: bizbilling.BillingCycleMonthly, PricingModel: bizbilling.PricingModelFlat, Active: true}
	ctx := context.Background()
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.Plan](es, "frn_biz_plans", zap.NewNop()).Create(ctx, plan.ID, plan))
	now := time.Now().UTC()
	// One recently due renewal ends in the future on every calendar day.
	end := now.Add(-time.Hour).Truncate(time.Second)
	start := end.AddDate(0, -1, 0)
	sub := &bizbilling.BizSubscription{ID: uuid.NewString(), AppID: scope.AppID, UserID: scope.UserID, PlanID: plan.ID, Currency: "INR", GatewayMode: bizbilling.GatewayTest, CollectionMode: bizbilling.CollectionInvoice, Status: bizbilling.SubscriptionStatusActive, Quantity: 1, AnchorDay: end.Day(), CurrentPeriodStart: start, CurrentPeriodEnd: end, CreatedAt: start, UpdatedAt: start}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizSubscription](es, "frn_biz_subscriptions", zap.NewNop()).Create(ctx, sub.ID, sub))
	service := NewBizPeriodService(es, lifecycle, nil, zap.NewNop())
	lifecycle.SetPeriodService(service)
	return service, lifecycle, ops, es, sub
}

func renewalBounds(t *testing.T, sub *bizbilling.BizSubscription) (time.Time, time.Time) {
	t.Helper()
	end, err := bizbilling.NextAnchoredPeriod(sub.CurrentPeriodEnd, bizbilling.BillingCycleMonthly, 0, sub.AnchorDay)
	require.NoError(t, err)
	return sub.CurrentPeriodEnd, end
}

func TestBizRenewalCrashDoesNotLoseInvoice(t *testing.T) {
	fault := &lostBillingResponse{match: func(r *http.Request, body []byte) bool {
		return r.Method == http.MethodPut && strings.Contains(r.URL.Path, "frn_biz_invoices/") && bytes.Contains(body, []byte(`"period_id"`))
	}}
	service, lifecycle, ops, es, sub := periodFixture(t, fault)
	ctx := context.Background()
	start, end := renewalBounds(t, sub)
	fault.armed.Store(true)
	pending, err := service.EnsurePeriod(ctx, sub.AppID, sub.ID, start, end)
	require.Error(t, err)
	require.NotEmpty(t, pending.OperationID)
	require.False(t, fault.armed.Load())
	expireOperationLease(t, es, sub.FinancialScope())
	_, err = ops.Resume(ctx, sub.AppID, pending.OperationID)
	require.NoError(t, err)
	restarted := NewBizPeriodService(es, lifecycle, nil, zap.NewNop())
	completed, err := restarted.EnsurePeriod(ctx, sub.AppID, sub.ID, start, end)
	require.NoError(t, err)
	require.Equal(t, pending.OperationID, completed.OperationID)
	stored, err := lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.True(t, stored.CurrentPeriodStart.Equal(start))
	require.True(t, stored.CurrentPeriodEnd.Equal(end))
	rows, total, err := lifecycle.ListInvoices(ctx, bizbilling.InvoiceFilter{AppID: sub.AppID, SubscriptionID: sub.ID})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, rows, 1)
	require.Equal(t, bizbillingrepo.PeriodID(sub.AppID, sub.ID, start), rows[0].PeriodID)
	require.Equal(t, int64(1180), rows[0].AmountDuePaisa)
	require.Zero(t, rows[0].AmountPaidPaisa)
}

func TestBizMultipleSchedulersOnePeriod(t *testing.T) {
	service, lifecycle, _, _, sub := periodFixture(t)
	ctx := context.Background()
	start, end := renewalBounds(t, sub)
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() { defer workers.Done(); _, _ = service.EnsurePeriod(ctx, sub.AppID, sub.ID, start, end) }()
	}
	workers.Wait()
	_, err := service.EnsurePeriod(ctx, sub.AppID, sub.ID, start, end)
	require.NoError(t, err)
	_, total, err := lifecycle.ListInvoices(ctx, bizbilling.InvoiceFilter{AppID: sub.AppID, SubscriptionID: sub.ID})
	require.NoError(t, err)
	require.Equal(t, 1, total)
}

func TestBizMoreThan100DueSubscriptions(t *testing.T) {
	service, lifecycle, _, es, first := periodFixture(t)
	ctx := context.Background()
	store := bizbillingrepo.NewStore[bizbilling.BizSubscription](es, "frn_biz_subscriptions", zap.NewNop(), repository.RefreshFalse)
	for i := 1; i < 250; i++ {
		sub := *first
		sub.ID = uuid.NewString()
		require.NoError(t, store.Create(ctx, sub.ID, &sub))
	}
	require.NoError(t, store.Refresh(ctx))
	count, err := service.RunDueForApp(ctx, first.AppID, 100)
	require.NoError(t, err)
	require.Equal(t, 250, count, "100 is page size, never a per-run cap")
	rows, total, err := lifecycle.ListInvoices(ctx, bizbilling.InvoiceFilter{AppID: first.AppID, Limit: 100})
	require.NoError(t, err)
	require.Equal(t, 250, total)
	require.Len(t, rows, 100)
	count, err = service.RunDueForApp(ctx, first.AppID, 100)
	require.NoError(t, err)
	require.Zero(t, count, "a repeated scheduler run must not recreate periods")
}

func TestBizProviderInvoiceMapsToSamePeriod(t *testing.T) {
	service, lifecycle, _, _, sub := periodFixture(t)
	ctx := context.Background()
	start, end := renewalBounds(t, sub)
	// A provider reference enriches the period's same invoice; it cannot mint
	// another internal bill or mark the existing invoice paid.
	_, err := service.EnsurePeriod(ctx, sub.AppID, sub.ID, start, end)
	require.NoError(t, err)
	_, err = service.BindProviderInvoice(ctx, sub.AppID, sub.ID, start, end, "inv_provider_fixture")
	require.NoError(t, err)
	_, err = service.BindProviderInvoice(ctx, sub.AppID, sub.ID, start, end, "inv_provider_fixture")
	require.NoError(t, err)
	rows, total, err := lifecycle.ListInvoices(ctx, bizbilling.InvoiceFilter{AppID: sub.AppID, SubscriptionID: sub.ID})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "inv_provider_fixture", rows[0].ProviderInvoiceID)
	require.Zero(t, rows[0].AmountPaidPaisa)
}

func TestBizRenewalArchivedCatalogAndTrial(t *testing.T) {
	for _, trial := range []bool{false, true} {
		t.Run(map[bool]string{false: "archived", true: "zero_trial"}[trial], func(t *testing.T) {
			service, lifecycle, _, es, sub := periodFixture(t)
			ctx := context.Background()
			plans := bizbillingrepo.NewStore[bizbilling.Plan](es, "frn_biz_plans", zap.NewNop())
			_, err := plans.Update(ctx, sub.PlanID, func(p *bizbilling.Plan) error {
				p.Active = false
				if trial {
					p.AmountPaisa = 0
				}
				return nil
			})
			require.NoError(t, err)
			start := sub.CurrentPeriodEnd
			if trial {
				sub.Status, sub.TrialEnd = bizbilling.SubscriptionStatusTrialing, &start
				sub.CurrentPeriodEnd = start.AddDate(0, 1, 0)
				require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizSubscription](es, "frn_biz_subscriptions", zap.NewNop()).Replace(ctx, sub.ID, sub))
			}
			count, err := service.RunDueForApp(ctx, sub.AppID, 100)
			require.NoError(t, err)
			require.Equal(t, 1, count)
			stored, err := lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
			require.NoError(t, err)
			require.Equal(t, bizbilling.SubscriptionStatusActive, stored.Status)
			require.True(t, stored.CurrentPeriodStart.Equal(start))
			invoices, total, err := lifecycle.ListInvoices(ctx, bizbilling.InvoiceFilter{AppID: sub.AppID})
			require.NoError(t, err)
			require.Equal(t, 1, total)
			require.Zero(t, invoices[0].AmountPaidPaisa)
			if trial {
				require.Zero(t, invoices[0].TotalPaisa)
			}
		})
	}
}

func TestBizRenewalKeepsMonthEndAnchor(t *testing.T) {
	service, lifecycle, _, es, sub := periodFixture(t)
	ctx := context.Background()
	sub.AnchorDay = 31
	sub.CurrentPeriodStart = time.Date(2028, 1, 31, 9, 0, 0, 0, time.UTC)
	sub.CurrentPeriodEnd = time.Date(2028, 2, 29, 9, 0, 0, 0, time.UTC)
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizSubscription](es, "frn_biz_subscriptions", zap.NewNop()).Replace(ctx, sub.ID, sub))
	start, end := renewalBounds(t, sub)
	require.Equal(t, 31, end.Day())
	_, err := service.EnsurePeriod(ctx, sub.AppID, sub.ID, start, end)
	require.NoError(t, err)
	stored, err := lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, 31, stored.AnchorDay)
	require.True(t, stored.CurrentPeriodEnd.Equal(end))
}

func TestBizRenewalCreateSubscriptionIsAtomic(t *testing.T) {
	_, lifecycle, _, _, seed := periodFixture(t)
	ctx := WithBizOperationKey(context.Background(), "first-subscription")
	request := &bizbilling.CreateSubscriptionRequest{UserID: seed.UserID, PlanID: seed.PlanID, Quantity: 1, GatewayMode: bizbilling.GatewayTest}
	sub, err := lifecycle.CreateSubscription(ctx, seed.AppID, request)
	require.NoError(t, err)
	retry, err := lifecycle.CreateSubscription(ctx, seed.AppID, request)
	require.NoError(t, err)
	require.Equal(t, sub.ID, retry.ID)
	rows, total, err := lifecycle.ListInvoices(ctx, bizbilling.InvoiceFilter{AppID: seed.AppID, SubscriptionID: sub.ID})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.NotEmpty(t, rows[0].PeriodID)
	require.Zero(t, rows[0].AmountPaidPaisa)
	paused, err := lifecycle.PauseSubscription(WithBizOperationKey(context.Background(), "pause"), seed.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.SubscriptionStatusPaused, paused.Status)
	resumed, err := lifecycle.ResumeSubscription(WithBizOperationKey(context.Background(), "resume"), seed.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.SubscriptionStatusActive, resumed.Status)
	require.True(t, sub.CurrentPeriodEnd.Equal(resumed.CurrentPeriodEnd))
}

func TestBizRenewalTrialUsesAcceptedPrice(t *testing.T) {
	service, lifecycle, _, _, seed := periodFixture(t)
	ctx := context.Background()
	_, err := lifecycle.stores.Plans.Update(ctx, seed.PlanID, func(p *bizbilling.Plan) error { p.SetupFeePaisa = 100; return nil })
	require.NoError(t, err)
	sub, err := lifecycle.CreateSubscription(WithBizOperationKey(ctx, "trial"), seed.AppID, &bizbilling.CreateSubscriptionRequest{UserID: seed.UserID, PlanID: seed.PlanID, TrialDays: 1, Quantity: 1, GatewayMode: bizbilling.GatewayTest})
	require.NoError(t, err)
	require.NotNil(t, sub.TrialEnd)
	_, err = lifecycle.stores.Plans.Update(ctx, seed.PlanID, func(p *bizbilling.Plan) error { p.AmountPaisa, p.SetupFeePaisa = 2000, 500; return nil })
	require.NoError(t, err)
	end, err := bizbilling.NextAnchoredPeriod(*sub.TrialEnd, bizbilling.BillingCycleMonthly, 0, sub.TrialEnd.Day())
	require.NoError(t, err)
	_, err = service.EnsurePeriod(ctx, sub.AppID, sub.ID, *sub.TrialEnd, end)
	require.NoError(t, err)
	rows, total, err := lifecycle.ListInvoices(ctx, bizbilling.InvoiceFilter{AppID: sub.AppID, SubscriptionID: sub.ID})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, int64(1298), rows[0].TotalPaisa, "trial conversion uses the accepted base and setup price, including its tax")
}

func TestBizRenewalSchedulerUsesDurablePeriods(t *testing.T) {
	_, lifecycle, _, _, sub := periodFixture(t)
	scheduler := &BizBillingScheduler{lifecycleSvc: lifecycle, stores: lifecycle.stores, logger: zap.NewNop()}
	scheduler.processRenewalsForApp(context.Background(), sub.AppID)
	_, total, err := lifecycle.ListInvoices(context.Background(), bizbilling.InvoiceFilter{AppID: sub.AppID, SubscriptionID: sub.ID})
	require.NoError(t, err)
	require.Equal(t, 1, total, "the real scheduler must invoke the durable v2 path")
}

func TestBizRenewalExpiredPITRestartsSafely(t *testing.T) {
	service, lifecycle, _, _, sub := periodFixture(t)
	fault := &expiredPeriodPIT{}
	fault.armed.Store(true)
	manifest := testutil.RequireOwnedBizFixture(t)
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{manifest.Services["elasticsearch"].URL}, Transport: fault, DisableRetry: true})
	require.NoError(t, err)
	service.pager = bizbillingrepo.NewSubscriptionPager(es)
	count, err := service.RunDueForApp(context.Background(), sub.AppID, 100)
	require.NoError(t, err)
	require.False(t, fault.armed.Load())
	require.Equal(t, 1, count)
	_, total, err := lifecycle.ListInvoices(context.Background(), bizbilling.InvoiceFilter{AppID: sub.AppID})
	require.NoError(t, err)
	require.Equal(t, 1, total)
}

func TestBizLegacyChangePlanTiming(t *testing.T) {
	for _, timing := range []string{"", "next_period"} {
		t.Run(map[string]string{"": "legacy_immediate", "next_period": "scheduled"}[timing], func(t *testing.T) {
			service, lifecycle, _, es, sub := periodFixture(t)
			ctx := context.Background()
			// A current period with remaining time exercises legacy proration.
			sub.CurrentPeriodStart = time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Second)
			sub.CurrentPeriodEnd, _ = bizbilling.NextAnchoredPeriod(sub.CurrentPeriodStart, bizbilling.BillingCycleMonthly, 0, sub.CurrentPeriodStart.Day())
			sub.AnchorDay = sub.CurrentPeriodStart.Day()
			require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizSubscription](es, "frn_biz_subscriptions", zap.NewNop()).Replace(ctx, sub.ID, sub))
			plan := &bizbilling.Plan{ID: uuid.NewString(), AppID: sub.AppID, Name: "Updated service", AmountPaisa: 2000, Currency: "INR", BillingCycle: bizbilling.BillingCycleMonthly, Active: true}
			require.NoError(t, bizbillingrepo.NewStore[bizbilling.Plan](es, "frn_biz_plans", zap.NewNop()).Create(ctx, plan.ID, plan))
			changed, err := lifecycle.ChangePlanAt(WithBizOperationKey(ctx, "change-plan"), sub.AppID, sub.ID, plan.ID, nil, timing)
			require.NoError(t, err)
			_, total, err := lifecycle.ListInvoices(ctx, bizbilling.InvoiceFilter{AppID: sub.AppID, SubscriptionID: sub.ID})
			require.NoError(t, err)
			if timing == "" {
				require.Equal(t, plan.ID, changed.PlanID)
				require.Equal(t, 1, total, "legacy default invoices the prorated new charge now")
			} else {
				require.Equal(t, sub.PlanID, changed.PlanID)
				require.Zero(t, total, "scheduled changes cannot bill before the boundary")
				start, end := renewalBounds(t, sub)
				_, err = service.EnsurePeriod(ctx, sub.AppID, sub.ID, start, end)
				require.NoError(t, err)
				stored, err := lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
				require.NoError(t, err)
				require.Equal(t, plan.ID, stored.PlanID)
				rows, total, err := lifecycle.ListInvoices(ctx, bizbilling.InvoiceFilter{AppID: sub.AppID, SubscriptionID: sub.ID})
				require.NoError(t, err)
				require.Equal(t, 1, total)
				require.Equal(t, int64(2360), rows[0].AmountDuePaisa)
			}
		})
	}
}
