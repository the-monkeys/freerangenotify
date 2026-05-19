package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

type grantBalanceRepo struct {
	balance  *billing.CreditBalance
	upserted *billing.CreditBalance
}

func (r *grantBalanceRepo) GetByTenantID(_ context.Context, tenantID string) (*billing.CreditBalance, error) {
	if r.balance == nil {
		return nil, nil
	}
	b := *r.balance
	b.TenantID = tenantID
	return &b, nil
}

func (r *grantBalanceRepo) Upsert(_ context.Context, balance *billing.CreditBalance) error {
	if balance == nil {
		return nil
	}
	copied := *balance
	r.upserted = &copied
	r.balance = &copied
	return nil
}

type grantLedgerRepo struct {
	last *billing.CreditLedgerEntry
}

func (r *grantLedgerRepo) Append(_ context.Context, entry *billing.CreditLedgerEntry) error {
	if entry == nil {
		return nil
	}
	copied := *entry
	r.last = &copied
	return nil
}

func (r *grantLedgerRepo) ListByTenantID(context.Context, string, int) ([]billing.CreditLedgerEntry, error) {
	return nil, nil
}

func newGrantCreditService(sub *license.Subscription, balance *billing.CreditBalance) (*CreditService, *grantBalanceRepo, *grantLedgerRepo) {
	balRepo := &grantBalanceRepo{balance: balance}
	ledgerRepo := &grantLedgerRepo{}
	svc := NewCreditService(
		balRepo,
		ledgerRepo,
		&stubLicenseRepo{sub: sub},
		&stubUsageRepo{},
		&stubAppRepo{},
		nil,
		nil,
		zap.NewNop(),
		true,
	)
	return svc, balRepo, ledgerRepo
}

func creditsModelSub(tenantID string, total, remaining int64) *license.Subscription {
	now := time.Now().UTC()
	return &license.Subscription{
		ID:                 "sub-1",
		TenantID:           tenantID,
		Plan:               "standard",
		Status:             license.SubscriptionStatusActive,
		CreditsTotal:       total,
		CreditsRemaining:   remaining,
		CurrentPeriodStart: now.AddDate(0, -1, 0),
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		Metadata: map[string]interface{}{
			"billing_model": billing.BillingModelCredits,
		},
	}
}

func TestCreditService_GrantCredits_success(t *testing.T) {
	sub := creditsModelSub("user-1", 500, 100)
	balance := &billing.CreditBalance{
		ID:               sub.ID,
		TenantID:         "user-1",
		CreditsTotal:     500,
		CreditsRemaining: 100,
		CreditsReserved:  10,
	}
	svc, balRepo, ledgerRepo := newGrantCreditService(sub, balance)

	snap, err := svc.GrantCredits(context.Background(), "user-1", 250, "partner top-up", nil)
	if err != nil {
		t.Fatalf("GrantCredits: %v", err)
	}
	if snap.CreditsTotal != 750 || snap.CreditsRemaining != 350 {
		t.Fatalf("snapshot = %+v", snap)
	}
	if balRepo.upserted == nil {
		t.Fatal("expected upsert")
	}
	if balRepo.upserted.CreditsTotal != 750 || balRepo.upserted.CreditsRemaining != 350 {
		t.Fatalf("upserted = %+v", balRepo.upserted)
	}
	if balRepo.upserted.CreditsReserved != 10 {
		t.Fatalf("reserved should be unchanged, got %d", balRepo.upserted.CreditsReserved)
	}
	if ledgerRepo.last == nil {
		t.Fatal("expected ledger entry")
	}
	if ledgerRepo.last.EntryType != billing.CreditLedgerAdjust || ledgerRepo.last.CreditsDelta != 250 {
		t.Fatalf("ledger = %+v", ledgerRepo.last)
	}
	if ledgerRepo.last.BalanceAfter != 350 {
		t.Fatalf("balance_after = %d", ledgerRepo.last.BalanceAfter)
	}
}

func TestCreditService_GrantCredits_no_subscription(t *testing.T) {
	svc, _, _ := newGrantCreditService(nil, nil)

	_, err := svc.GrantCredits(context.Background(), "user-1", 100, "test", nil)
	if !errors.Is(err, ErrGrantNoActiveSubscription) {
		t.Fatalf("expected ErrGrantNoActiveSubscription, got %v", err)
	}
}

func TestCreditService_GrantCredits_legacy_billing(t *testing.T) {
	now := time.Now().UTC()
	sub := &license.Subscription{
		TenantID:           "user-1",
		Plan:               "pro",
		Status:             license.SubscriptionStatusActive,
		CurrentPeriodStart: now.AddDate(0, -1, 0),
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
	}
	svc, _, _ := newGrantCreditService(sub, nil)

	_, err := svc.GrantCredits(context.Background(), "user-1", 100, "test", nil)
	if !errors.Is(err, ErrGrantLegacyBilling) {
		t.Fatalf("expected ErrGrantLegacyBilling, got %v", err)
	}
}

func TestCreditService_GrantCredits_invalid_amount(t *testing.T) {
	sub := creditsModelSub("user-1", 100, 100)
	svc, _, _ := newGrantCreditService(sub, &billing.CreditBalance{TenantID: "user-1", CreditsTotal: 100, CreditsRemaining: 100})

	_, err := svc.GrantCredits(context.Background(), "user-1", 0, "test", nil)
	if !errors.Is(err, ErrGrantInvalidAmount) {
		t.Fatalf("expected ErrGrantInvalidAmount, got %v", err)
	}
}
