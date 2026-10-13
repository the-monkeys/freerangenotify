package services

import (
	"context"
	"testing"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"go.uber.org/zap"
)

type immutableCardFixture struct {
	cards  map[string]*billing.RateCard
	active string
}

func (r *immutableCardFixture) CreateVersion(_ context.Context, card *billing.RateCard) error {
	r.cards[card.Version] = card
	return nil
}
func (r *immutableCardFixture) GetByVersion(_ context.Context, version string) (*billing.RateCard, error) {
	return r.cards[version], nil
}
func (r *immutableCardFixture) GetActive(_ context.Context) (*billing.RateCard, error) {
	return r.cards[r.active], nil
}
func (r *immutableCardFixture) SetActiveVersion(_ context.Context, version string) error {
	r.active = version
	return nil
}

func TestRateChangesDoNotOverwriteRecentPriceVersions(t *testing.T) {
	ctx := context.Background()
	base := &billing.RateCard{Version: "purchase-version", CreatedAt: time.Now().UTC(), ChannelCreditCost: map[string]int64{"email": 5}, Plans: billing.DefaultPlanBundles()}
	repo := &immutableCardFixture{cards: map[string]*billing.RateCard{base.Version: base}, active: base.Version}
	svc := NewRateCardService(repo, nil, zap.NewNop(), RateCardServiceConfig{})
	if err := svc.RefreshActiveRateCard(ctx); err != nil {
		t.Fatal(err)
	}
	channelEdit, err := svc.UpdateChannelCredits(ctx, "email", 7)
	if err != nil {
		t.Fatal(err)
	}
	if channelEdit.Version == "purchase-version" || repo.cards["purchase-version"].ChannelCreditCost["email"] != 5 {
		t.Fatal("editing a recent card repriced its existing purchase version")
	}
	plan := channelEdit.Plans["starter"]
	plan.CreditsIncluded = 35000
	planEdit, err := svc.UpdatePlanBundle(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	if planEdit.Version == channelEdit.Version || repo.cards[channelEdit.Version].Plans["starter"].CreditsIncluded != 15000 {
		t.Fatal("plan edit mutated the prior price version")
	}
}

func TestCheckoutRequiresALoadedAuthoritativeRateCard(t *testing.T) {
	svc := NewRateCardService(&immutableCardFixture{cards: map[string]*billing.RateCard{}}, nil, zap.NewNop(), RateCardServiceConfig{})
	if _, ok := svc.GetCheckoutPlan("starter"); ok {
		t.Fatal("unavailable catalog silently sold a fallback plan")
	}
}
