package handlers

import (
	"testing"

	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
)

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
