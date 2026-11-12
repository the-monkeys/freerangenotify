package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"go.uber.org/zap"
)

type recurringGateway struct {
	*checkoutGateway
	plans, creates, changes   int
	remote                    *bizbilling.GatewaySubscription
	lostCreate, cancelPending bool
	lostPlan                  bool
}

func (g *recurringGateway) CheckRecurring(context.Context) error { return nil }
func (g *recurringGateway) CreateRecurringPlan(_ context.Context, intent bizbilling.RecurringIntent) (bizbilling.GatewayRecurringPlan, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.plans++
	if g.lostPlan {
		return bizbilling.GatewayRecurringPlan{}, bizbilling.ErrGatewayPending
	}
	return bizbilling.GatewayRecurringPlan{ID: "plan_recurring_fixture", OperationID: intent.OperationID, AmountPaisa: intent.AmountPaisa, Currency: intent.Currency, Period: intent.Period, Interval: intent.Interval}, nil
}
func (g *recurringGateway) FindRecurringPlan(_ context.Context, intent bizbilling.RecurringIntent) (*bizbilling.GatewayRecurringPlan, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.plans == 0 {
		return nil, nil
	}
	return &bizbilling.GatewayRecurringPlan{ID: "plan_recurring_fixture", OperationID: intent.OperationID, AmountPaisa: intent.AmountPaisa, Currency: intent.Currency, Period: intent.Period, Interval: intent.Interval}, nil
}
func (g *recurringGateway) CreateSubscription(_ context.Context, intent bizbilling.RecurringIntent) (bizbilling.GatewaySubscription, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.creates++
	remote := bizbilling.GatewaySubscription{ID: "sub_recurring_fixture", PlanID: intent.PlanID, Intent: intent, Status: "created"}
	g.remote = &remote
	if g.lostCreate {
		return bizbilling.GatewaySubscription{}, bizbilling.ErrGatewayPending
	}
	return remote, nil
}
func (g *recurringGateway) FindSubscription(_ context.Context, intent bizbilling.RecurringIntent) (*bizbilling.GatewaySubscription, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.remote == nil {
		return nil, nil
	}
	copy := *g.remote
	return &copy, nil
}
func (g *recurringGateway) FetchSubscription(_ context.Context, id string) (bizbilling.GatewaySubscription, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.remote == nil || g.remote.ID != id {
		return bizbilling.GatewaySubscription{}, bizbilling.ErrGatewayPending
	}
	return *g.remote, nil
}
func (g *recurringGateway) ChangeSubscription(_ context.Context, id string, action bizbilling.SubscriptionAction) (bizbilling.GatewaySubscription, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.changes++
	if g.cancelPending {
		return bizbilling.GatewaySubscription{}, bizbilling.ErrGatewayPending
	}
	switch action {
	case bizbilling.SubscriptionPause:
		g.remote.Status = "paused"
	case bizbilling.SubscriptionResume:
		g.remote.Status = "active"
	default:
		g.remote.Status = "cancelled"
	}
	return *g.remote, nil
}

func (g *recurringGateway) VerifySubscriptionAuthorization(payment, subscription, signature string) error {
	if payment != "pay_auth_fixture" || subscription != "sub_recurring_fixture" || signature != "fixture-valid" {
		return bizbilling.ErrInvalidGatewayProof
	}
	return nil
}

func TestBizAutomaticAuthorizationProofRequiresProviderState(t *testing.T) {
	service, gateway, lifecycle, _, sub := recurringFixture(t, bizbilling.CollectionAutomatic)
	ctx := context.Background()
	_, err := service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "authorize")
	require.NoError(t, err)
	proof := bizbilling.SubscriptionAuthorizationProof{SubscriptionID: "sub_recurring_fixture", PaymentID: "pay_auth_fixture", Signature: "invalid"}
	_, err = service.VerifyAuthorization(ctx, sub.AppID, sub.UserID, sub.ID, proof)
	require.ErrorIs(t, err, bizbilling.ErrInvalidGatewayProof)
	proof.Signature = "fixture-valid"
	created, err := service.VerifyAuthorization(ctx, sub.AppID, sub.UserID, sub.ID, proof)
	require.NoError(t, err)
	require.Equal(t, "awaiting_authorization", created.State, "a browser proof cannot override provider state")
	gateway.remote.Status = "authenticated"
	confirmed, err := service.VerifyAuthorization(ctx, sub.AppID, sub.UserID, sub.ID, proof)
	require.NoError(t, err)
	require.Equal(t, "authorized", confirmed.State)
	_, err = service.VerifyAuthorization(ctx, sub.AppID, uuid.NewString(), sub.ID, proof)
	require.Error(t, err)
	_, count, err := lifecycle.ListPayments(ctx, bizbilling.PaymentFilter{AppID: sub.AppID, UserID: sub.UserID})
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestBizAutomaticLostPlanRecoversAfterRestartWithoutSecondPlan(t *testing.T) {
	service, gateway, _, _, sub := recurringFixture(t, bizbilling.CollectionAutomatic)
	ctx := context.Background()
	gateway.lostPlan = true
	_, err := service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "authorize")
	require.ErrorIs(t, err, bizbilling.ErrGatewayPending)
	require.Equal(t, 1, gateway.plans)
	require.Zero(t, gateway.creates)
	restarted := NewBizRecurringService(service.es, service.lifecycle, service.merchants, service.factory, zap.NewNop())
	ready, err := restarted.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "authorize")
	require.NoError(t, err)
	require.Equal(t, "awaiting_authorization", ready.State)
	require.Equal(t, 1, gateway.plans)
	require.Equal(t, 1, gateway.creates)
}

func TestBizAutomaticConcurrentAuthorizationCreatesOneMandate(t *testing.T) {
	service, gateway, _, _, sub := recurringFixture(t, bizbilling.CollectionAutomatic)
	ctx := context.Background()
	var workers sync.WaitGroup
	errors := make(chan error, 4)
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			_, err := service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, fmt.Sprintf("authorize-%d", index))
			errors <- err
		}(i)
	}
	workers.Wait()
	close(errors)
	successes := 0
	for err := range errors {
		if err == nil {
			successes++
		}
	}
	require.Positive(t, successes, "at least one concurrent request must complete")
	ready, err := service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "authorize-0")
	require.NoError(t, err)
	require.Equal(t, "awaiting_authorization", ready.State)
	require.Equal(t, 1, gateway.plans)
	require.Equal(t, 1, gateway.creates)
}

func TestBizCollectionStopsUnattemptedMandateAndKeepsOldRetryBound(t *testing.T) {
	service, gateway, lifecycle, _, sub := recurringFixture(t, bizbilling.CollectionAutomatic)
	ctx := context.Background()
	gateway.lostPlan = true
	_, err := service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "authorize")
	require.ErrorIs(t, err, bizbilling.ErrGatewayPending)
	require.Zero(t, gateway.creates)
	stopped, err := service.ChangeCollection(ctx, sub.AppID, sub.ID, bizbilling.CollectionInvoice, "stop-before-subscription")
	require.NoError(t, err)
	require.Equal(t, bizbilling.OperationCompleted, stopped.State)
	cancelled, err := service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "authorize")
	require.NoError(t, err)
	require.Equal(t, "cancelled", cancelled.State)
	require.Zero(t, gateway.creates)
	_, err = service.ChangeCollection(ctx, sub.AppID, sub.ID, bizbilling.CollectionAutomatic, "new-choice")
	require.NoError(t, err)
	replayed, err := service.ChangeCollection(ctx, sub.AppID, sub.ID, bizbilling.CollectionInvoice, "stop-before-subscription")
	require.NoError(t, err)
	require.Equal(t, stopped.OperationID, replayed.OperationID)
	current, err := lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.CollectionAutomatic, current.EffectiveCollectionMode())
	require.Zero(t, gateway.creates)
}

func TestBizAutomaticPriceSnapshotUsesExactPlanTaxAndQuantity(t *testing.T) {
	for _, test := range []struct {
		name      string
		model     bizbilling.PricingModel
		quantity  int
		inclusive bool
		want      int64
	}{
		{"flat-three", bizbilling.PricingModelFlat, 3, false, 1180},
		{"per-unit-three", bizbilling.PricingModelPerUnit, 3, false, 3540},
		{"tax-inclusive", bizbilling.PricingModelFlat, 1, true, 1000},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, gateway, _, _, sub := recurringFixture(t, bizbilling.CollectionAutomatic)
			ctx := context.Background()
			_, err := service.lifecycle.stores.Plans.Update(ctx, sub.PlanID, func(p *bizbilling.Plan) error {
				p.PricingModel = test.model
				p.TaxInclusive = test.inclusive
				return nil
			})
			require.NoError(t, err)
			_, err = bizbillingrepo.NewStore[bizbilling.BizSubscription](service.es, "frn_biz_subscriptions", zap.NewNop()).Update(ctx, sub.ID, func(s *bizbilling.BizSubscription) error { s.Quantity = test.quantity; return nil })
			require.NoError(t, err)
			_, err = service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "authorize")
			require.NoError(t, err)
			require.Equal(t, test.want, gateway.remote.Intent.AmountPaisa)
			require.Equal(t, test.want, gateway.remote.Intent.Price.TotalPaisa)
		})
	}
}

func recurringFixture(t *testing.T, mode bizbilling.CollectionMode) (*BizRecurringService, *recurringGateway, *BizLifecycleService, *BizGatewayConnectionService, *bizbilling.BizSubscription) {
	t.Helper()
	_, merchants, gateway, lifecycle, _, es, scope := checkoutFixture(t)
	ctx := context.Background()
	templates := &database.IndexTemplates{}
	ensureBillingIndex(t, es, "frn_biz_plans", templates.GetBizPlansTemplate())
	ensureBillingIndex(t, es, "frn_biz_subscriptions", templates.GetBizSubscriptionsTemplate())
	plan := &bizbilling.Plan{ID: uuid.NewString(), AppID: scope.AppID, Name: "Monthly Service", Currency: "INR", AmountPaisa: 1000, BillingCycle: bizbilling.BillingCycleMonthly, PricingModel: bizbilling.PricingModelFlat, Active: true}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.Plan](es, "frn_biz_plans", zap.NewNop()).Create(ctx, plan.ID, plan))
	now := time.Now().UTC()
	sub := &bizbilling.BizSubscription{ID: uuid.NewString(), AppID: scope.AppID, UserID: scope.UserID, PlanID: plan.ID, Quantity: 1, Currency: "INR", GatewayMode: bizbilling.GatewayTest, CollectionMode: mode, AutomaticCycles: 12, CurrentPeriodStart: now, CurrentPeriodEnd: now.AddDate(0, 1, 0), Status: bizbilling.SubscriptionStatusActive, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizSubscription](es, "frn_biz_subscriptions", zap.NewNop()).Create(ctx, sub.ID, sub))
	require.NoError(t, merchants.connections.RecordAccountProof(ctx, &bizbilling.MerchantAccountProof{AppID: scope.AppID, Gateway: gateway.ref, EventDigest: "owned-recurring-proof", PaymentID: "pay_owned_auth", VerifiedAt: now}))
	remote := &recurringGateway{checkoutGateway: gateway}
	factory := bizFactoryFunc(func(_ context.Context, ref bizbilling.GatewayRef) (bizbilling.BizGateway, error) {
		require.Equal(t, gateway.ref.ConnectionID, ref.ConnectionID)
		require.Equal(t, gateway.ref.AccountID, ref.AccountID)
		return remote, nil
	})
	service := NewBizRecurringService(es, lifecycle, merchants, factory, zap.NewNop())
	service.SetNewCollectionEnabled(true)
	return service, remote, lifecycle, merchants, sub
}

func TestBizAutomaticNeedsExplicitAuthorization(t *testing.T) {
	service, gateway, lifecycle, _, sub := recurringFixture(t, "")
	ctx := context.Background()
	_, err := service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "not-opted-in")
	require.Error(t, err)
	require.Zero(t, gateway.creates)
	stored, err := lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.CollectionInvoice, stored.EffectiveCollectionMode())
	_, err = service.ChangeCollection(ctx, sub.AppID, sub.ID, bizbilling.CollectionAutomatic, "choose-automatic")
	require.NoError(t, err)
	require.Zero(t, gateway.creates, "admin selection is not customer authorization")
	ready, err := service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "customer-consent")
	require.NoError(t, err)
	require.Equal(t, "awaiting_authorization", ready.State)
	require.Equal(t, 1, gateway.creates)
	require.Equal(t, int64(1180), gateway.remote.Intent.AmountPaisa, "authorize actual tax-inclusive charge")
	require.False(t, gateway.remote.Intent.StartAt.Before(sub.CurrentPeriodEnd), "never charge before the existing period ends")
	require.Less(t, gateway.remote.Intent.StartAt.Sub(sub.CurrentPeriodEnd), time.Second, "provider times have whole-second precision")
	gateway.remote.Status = "authenticated"
	confirmed, err := service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "customer-consent")
	require.NoError(t, err)
	require.Equal(t, "authorized", confirmed.State)
	_, count, err := lifecycle.ListPayments(ctx, bizbilling.PaymentFilter{AppID: sub.AppID, UserID: sub.UserID})
	require.NoError(t, err)
	require.Zero(t, count, "mandate authorization is not an invoice payment")
}

func TestBizAutomaticSignedAuthorizationVerifiesAccountWithoutRecordingPayment(t *testing.T) {
	service, gateway, lifecycle, merchants, sub := recurringFixture(t, bizbilling.CollectionAutomatic)
	ctx := context.Background()
	// A newly rotated revision has no account proof. It must be proven by a
	// signed, server-verified event, not by accepting the business's account ID.
	snapshot, err := merchants.Save(ctx, sub.AppID, bizbilling.GatewayTest, 1, merchantPatch("acc_fixture_a", "rzp_test_biz2", "new-owned-api-secret", "new-owned-hook-secret"))
	require.NoError(t, err)
	gateway.ref = merchantReference(snapshot)
	_, err = service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "new-revision-auth")
	require.NoError(t, err)
	gateway.remote.Status = "authenticated"
	waiting, err := service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "new-revision-auth")
	require.NoError(t, err)
	require.Equal(t, "awaiting_account_verification", waiting.State)
	events := NewBizGatewayEventService(service.es, merchants, nil, zap.NewNop())
	events.SetRecurringProcessor(service)
	raw, err := json.Marshal(map[string]any{"account_id": gateway.ref.AccountID, "event": "subscription.authenticated", "payload": map[string]any{"subscription": map[string]any{"entity": map[string]any{"id": gateway.remote.ID}}}})
	require.NoError(t, err)
	mac := hmac.New(sha256.New, []byte("new-owned-hook-secret"))
	mac.Write(raw)
	receipt, err := events.Accept(ctx, gateway.ref.ConnectionID, "auth-fixture", raw, hex.EncodeToString(mac.Sum(nil)))
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		result, err := events.Process(ctx, receipt.ID)
		require.NoError(t, err)
		require.Equal(t, bizbilling.OperationCompleted, result.State)
	}
	stored, err := lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, "authorized", stored.CollectionState)
	ready, err := merchants.Snapshot(ctx, sub.AppID, bizbilling.GatewayTest)
	require.NoError(t, err)
	require.True(t, ready.AccountVerified)
	require.True(t, ready.AutomaticReady)
	_, count, err := lifecycle.ListPayments(ctx, bizbilling.PaymentFilter{AppID: sub.AppID, UserID: sub.UserID})
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestBizCollectionModeTransition(t *testing.T) {
	service, gateway, lifecycle, _, sub := recurringFixture(t, bizbilling.CollectionAutomatic)
	ctx := context.Background()
	_, err := service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "authorize")
	require.NoError(t, err)
	gateway.cancelPending = true
	_, err = service.ChangeCollection(ctx, sub.AppID, sub.ID, bizbilling.CollectionInvoice, "cancel-future-debits")
	require.ErrorIs(t, err, bizbilling.ErrGatewayPending)
	stored, err := lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.CollectionAutomatic, stored.EffectiveCollectionMode())
	gateway.remote.Status = "cancelled"
	_, err = service.ChangeCollection(ctx, sub.AppID, sub.ID, bizbilling.CollectionInvoice, "cancel-future-debits")
	require.NoError(t, err)
	stored, err = lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.CollectionInvoice, stored.EffectiveCollectionMode())
	require.Equal(t, 1, gateway.changes, "unknown provider change must be resolved by lookup")
}

func TestBizCollectionSwitchWaitsForLostCreation(t *testing.T) {
	service, gateway, lifecycle, _, sub := recurringFixture(t, bizbilling.CollectionAutomatic)
	ctx := context.Background()
	gateway.lostCreate = true
	gateway.cancelPending = true
	_, err := service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "lost-authorization")
	require.ErrorIs(t, err, bizbilling.ErrGatewayPending)
	_, err = service.ChangeCollection(ctx, sub.AppID, sub.ID, bizbilling.CollectionInvoice, "stop-unknown-mandate")
	require.ErrorIs(t, err, bizbilling.ErrGatewayPending, "an unknown create must be resolved and future debits stopped before invoice mode")
	stored, err := lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.CollectionAutomatic, stored.EffectiveCollectionMode())
	gateway.remote.Status = "cancelled"
	_, err = service.ChangeCollection(ctx, sub.AppID, sub.ID, bizbilling.CollectionInvoice, "stop-unknown-mandate")
	require.NoError(t, err)
	stored, err = lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.CollectionInvoice, stored.EffectiveCollectionMode())
	require.Equal(t, 1, gateway.creates)
}

func TestBizUnsupportedAutomaticPlanRejected(t *testing.T) {
	for _, kind := range []string{"currency", "custom", "tiered", "setup", "cycles"} {
		t.Run(kind, func(t *testing.T) {
			service, gateway, lifecycle, _, sub := recurringFixture(t, bizbilling.CollectionAutomatic)
			ctx := context.Background()
			_, err := lifecycle.stores.Plans.Update(ctx, sub.PlanID, func(plan *bizbilling.Plan) error {
				switch kind {
				case "currency":
					plan.Currency = "USD"
				case "custom":
					plan.BillingCycle = bizbilling.BillingCycleCustom
					plan.CycleDays = 20
				case "tiered":
					plan.PricingModel = bizbilling.PricingModelTiered
				case "setup":
					plan.SetupFeePaisa = 100
				}
				return nil
			})
			require.NoError(t, err)
			if kind == "cycles" {
				_, err = bizbillingrepo.NewStore[bizbilling.BizSubscription](service.es, "frn_biz_subscriptions", zap.NewNop()).Update(ctx, sub.ID, func(s *bizbilling.BizSubscription) error { s.AutomaticCycles = 0; return nil })
				require.NoError(t, err)
			}
			_, err = service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "unsupported")
			require.Error(t, err)
			require.Zero(t, gateway.plans)
			require.Zero(t, gateway.creates)
		})
	}
}

func TestBizAutomaticLostCreateRecoversThroughOriginalMerchant(t *testing.T) {
	service, gateway, _, merchants, sub := recurringFixture(t, bizbilling.CollectionAutomatic)
	ctx := context.Background()
	gateway.lostCreate = true
	_, err := service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "authorization")
	require.ErrorIs(t, err, bizbilling.ErrGatewayPending)
	require.NoError(t, merchants.Disconnect(ctx, sub.AppID, bizbilling.GatewayTest, 1))
	ready, err := service.Authorize(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.GatewayTest, "authorization")
	require.NoError(t, err)
	require.Equal(t, "awaiting_authorization", ready.State)
	require.Equal(t, 1, gateway.creates)
	require.Equal(t, 1, gateway.plans)
}

func TestBizAutomaticInvoiceExcludesManualPayment(t *testing.T) {
	service, _, lifecycle, _, sub := recurringFixture(t, bizbilling.CollectionAutomatic)
	invoice := seedFinancialInvoice(t, service.es, bizbilling.Scope{AppID: sub.AppID, UserID: sub.UserID, Currency: sub.Currency}, 1000)
	_, err := bizbillingrepo.NewStore[bizbilling.BizInvoice](service.es, "frn_biz_invoices", zap.NewNop()).Update(context.Background(), invoice.ID, func(i *bizbilling.BizInvoice) error { i.CollectionOwner = "razorpay:" + sub.ID; return nil })
	require.NoError(t, err)
	_, err = lifecycle.RecordPayment(context.Background(), sub.AppID, invoice.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: 1000})
	require.ErrorIs(t, err, bizbilling.ErrAutomaticCollectionInProgress)
}
