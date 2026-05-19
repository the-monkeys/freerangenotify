package billing

import (
	"testing"

	"github.com/the-monkeys/freerangenotify/internal/domain/license"
)

func TestResolvePlan_free_trial(t *testing.T) {
	plan, ok := ResolvePlan("free_trial")
	if !ok {
		t.Fatal("expected free_trial in legacy rates")
	}
	if plan.Name != "free_trial" {
		t.Fatalf("got plan %q", plan.Name)
	}
	if plan.IncludedQuotas["email"] != 500 {
		t.Fatalf("email quota = %d", plan.IncludedQuotas["email"])
	}
}

func TestResolvePlan_pro_creditWins(t *testing.T) {
	plan, ok := ResolvePlan("pro")
	if !ok {
		t.Fatal("expected pro")
	}
	if plan.CreditsIncluded != 55000 {
		t.Fatalf("expected credit plan, got credits=%d", plan.CreditsIncluded)
	}
}

func TestBillingModel(t *testing.T) {
	credits := &license.Subscription{CreditsTotal: 500}
	if BillingModel(credits) != BillingModelCredits {
		t.Fatalf("got %q", BillingModel(credits))
	}
	legacy := &license.Subscription{Plan: "free_trial", Metadata: map[string]interface{}{"message_limit": 10000}}
	if BillingModel(legacy) != BillingModelLegacy {
		t.Fatalf("got %q", BillingModel(legacy))
	}
}

func TestLegacyMessageLimit_metadataWins(t *testing.T) {
	sub := &license.Subscription{Metadata: map[string]interface{}{"message_limit": 10000}}
	plan, _ := ResolvePlan("free_trial")
	if got := LegacyMessageLimit(sub, plan); got != 10000 {
		t.Fatalf("limit = %d", got)
	}
}
