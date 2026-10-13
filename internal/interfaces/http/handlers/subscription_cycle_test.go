package handlers

import (
	"testing"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
)

func TestPaidRechargeCarriesBalanceAndHolds(t *testing.T) {
	expiry := time.Now().UTC().AddDate(2, 0, 0)
	sub := &license.Subscription{CreditsTotal: 1500, CreditsRemaining: 400, CreditsReserved: 100,
		CreditsExpireAt: &expiry, CurrentPeriodEnd: expiry,
		Metadata: map[string]interface{}{"billing_model": "credits"}}
	applyPaidCreditAllocation(sub, paidCreditAllocation{PlanID: "starter", Credits: 35000, ValidityDays: 365})
	if sub.CreditsRemaining != 35400 || sub.CreditsTotal != 36500 || sub.CreditsReserved != 100 {
		t.Fatalf("recharge lost wallet state: total=%d remaining=%d reserved=%d", sub.CreditsTotal, sub.CreditsRemaining, sub.CreditsReserved)
	}
	if !sub.CreditsExpireAt.Equal(expiry) || !sub.CurrentPeriodEnd.Equal(expiry) {
		t.Fatalf("recharge shortened validity: %v / %v, want %v", sub.CreditsExpireAt, sub.CurrentPeriodEnd, expiry)
	}
}

func TestRenewalCarriesUnexpiredCreditBalance(t *testing.T) {
	expiry := time.Now().UTC().AddDate(2, 0, 0)
	sub := &license.Subscription{CreditsTotal: 1500, CreditsRemaining: 400, CreditsReserved: 100,
		CreditsExpireAt: &expiry, CurrentPeriodEnd: expiry,
		Metadata: map[string]interface{}{"billing_model": "credits"}}
	applySubscriptionRenewal(nil, sub, "user-1", billing.DefaultRates(), billing.PlanTier{Name: "pro", CreditsIncluded: 35000}, 1, "admin", nil, nil, false, nil)
	if sub.CreditsRemaining != 35400 || sub.CreditsReserved != 100 || sub.CreditsTotal != 36500 {
		t.Fatalf("renewal lost wallet state: %+v", sub)
	}
	if !sub.CreditsExpireAt.Equal(expiry) || sub.CurrentPeriodEnd.Before(expiry) {
		t.Fatal("renewal shortened existing credit validity")
	}
}

func TestResolvePlan_free_trial_via_billing(t *testing.T) {
	plan := resolvePlan(billing.DefaultRates(), "free_trial")
	if plan.Name != "free_trial" {
		t.Fatalf("expected free_trial legacy plan, got %q", plan.Name)
	}
	if plan.IncludedQuotas["email"] != 500 {
		t.Fatalf("email quota = %d", plan.IncludedQuotas["email"])
	}
}

func TestCurrentMessageLimit_legacy_metadata(t *testing.T) {
	sub := &license.Subscription{
		Plan: "free_trial",
		Metadata: map[string]interface{}{
			"message_limit": 10000,
		},
	}
	if got := currentMessageLimit(sub, billing.DefaultRates()); got != 10000 {
		t.Fatalf("limit = %d", got)
	}
}

func TestApplySubscriptionRenewal_migrates_to_credits(t *testing.T) {
	sub := &license.Subscription{
		Plan: "free_trial",
		Metadata: map[string]interface{}{
			"message_limit": 10000,
		},
	}
	plan, ok := billing.ResolveRenewalPlan("pro")
	if !ok {
		t.Fatal("pro plan missing")
	}
	applySubscriptionRenewal(nil, sub, "user-1", billing.DefaultRates(), plan, 1, "test", nil, nil, false, nil)
	if billing.BillingModel(sub) != billing.BillingModelCredits {
		t.Fatalf("billing model = %s", billing.BillingModel(sub))
	}
	if sub.CreditsTotal != plan.CreditsIncluded {
		t.Fatalf("credits total = %d", sub.CreditsTotal)
	}
	if _, ok := sub.Metadata["message_limit"]; ok {
		t.Fatal("legacy message_limit should be cleared")
	}
}
