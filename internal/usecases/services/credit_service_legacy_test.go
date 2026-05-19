package services

import (
	"context"
	"testing"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

type stubLicenseRepo struct {
	sub *license.Subscription
}

func (s *stubLicenseRepo) Create(context.Context, *license.Subscription) error { return nil }
func (s *stubLicenseRepo) GetByID(context.Context, string) (*license.Subscription, error) {
	return nil, nil
}
func (s *stubLicenseRepo) Update(context.Context, *license.Subscription) error { return nil }
func (s *stubLicenseRepo) Delete(context.Context, string) error                  { return nil }
func (s *stubLicenseRepo) List(context.Context, license.SubscriptionFilter) ([]*license.Subscription, error) {
	return nil, nil
}
func (s *stubLicenseRepo) GetActiveSubscription(context.Context, string, string, time.Time) (*license.Subscription, error) {
	return s.sub, nil
}

type stubUsageRepo struct {
	summaries []billing.UsageSummary
}

func (s *stubUsageRepo) Emit(context.Context, *billing.UsageEvent) error { return nil }
func (s *stubUsageRepo) Store(context.Context, *billing.UsageEvent) error  { return nil }
func (s *stubUsageRepo) GetSummary(_ context.Context, _ []string, _, _ time.Time) ([]billing.UsageSummary, error) {
	return s.summaries, nil
}
func (s *stubUsageRepo) GetEvents(context.Context, string, time.Time, time.Time, int) ([]billing.UsageEvent, error) {
	return nil, nil
}

type stubAppRepo struct {
	apps []*application.Application
}

func (s *stubAppRepo) Create(context.Context, *application.Application) error { return nil }
func (s *stubAppRepo) GetByID(_ context.Context, id string) (*application.Application, error) {
	for _, a := range s.apps {
		if a.AppID == id {
			return a, nil
		}
	}
	return nil, nil
}
func (s *stubAppRepo) GetByAPIKey(context.Context, string) (*application.Application, error) {
	return nil, nil
}
func (s *stubAppRepo) Update(context.Context, *application.Application) error { return nil }
func (s *stubAppRepo) Delete(context.Context, string) error { return nil }
func (s *stubAppRepo) List(context.Context, application.ApplicationFilter) ([]*application.Application, error) {
	return s.apps, nil
}
func (s *stubAppRepo) RegenerateAPIKey(context.Context, string) (string, error) { return "", nil }

type stubBalanceRepo struct{}

func (stubBalanceRepo) GetByTenantID(context.Context, string) (*billing.CreditBalance, error) {
	return &billing.CreditBalance{CreditsRemaining: 0}, nil
}
func (stubBalanceRepo) Upsert(context.Context, *billing.CreditBalance) error { return nil }

type stubLedgerRepo struct{}

func (stubLedgerRepo) Append(context.Context, *billing.CreditLedgerEntry) error { return nil }
func (stubLedgerRepo) ListByTenantID(context.Context, string, int) ([]billing.CreditLedgerEntry, error) {
	return nil, nil
}

func newLegacyCreditService(sub *license.Subscription, usage []billing.UsageSummary, apps []*application.Application) *CreditService {
	return NewCreditService(
		stubBalanceRepo{},
		stubLedgerRepo{},
		&stubLicenseRepo{sub: sub},
		&stubUsageRepo{summaries: usage},
		&stubAppRepo{apps: apps},
		nil,
		nil,
		zap.NewNop(),
		true,
	)
}

func TestCreditService_legacy_free_trial_message_limit(t *testing.T) {
	now := time.Now().UTC()
	sub := &license.Subscription{
		TenantID:           "user-1",
		Plan:               "free_trial",
		Status:             license.SubscriptionStatusTrial,
		CurrentPeriodStart: now.AddDate(0, -1, 0),
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		Metadata: map[string]interface{}{
			"message_limit": int64(10000),
		},
	}
	usage := []billing.UsageSummary{
		{Channel: "email", CredentialSource: billing.CredSourceSystem, MessageCount: 10000},
	}
	svc := newLegacyCreditService(sub, usage, []*application.Application{{AppID: "app-1", AdminUserID: "user-1"}})

	_, err := svc.ReserveForNotification(context.Background(), "user-1", "app-1", "n-1", "email")
	if err != ErrInsufficientCredits {
		t.Fatalf("expected insufficient credits, got %v", err)
	}

	usage[0].MessageCount = 9999
	svc = newLegacyCreditService(sub, usage, []*application.Application{{AppID: "app-1", AdminUserID: "user-1"}})
	res, err := svc.ReserveForNotification(context.Background(), "user-1", "app-1", "n-2", "email")
	if err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
	if res == nil || res.RateCardVersion != billing.RateCardVersionLegacy {
		t.Fatalf("expected legacy reservation, got %+v", res)
	}
}

func TestCreditService_legacy_pro_per_channel_quota(t *testing.T) {
	now := time.Now().UTC()
	sub := &license.Subscription{
		TenantID:           "user-1",
		Plan:               "pro",
		Status:             license.SubscriptionStatusActive,
		CurrentPeriodStart: now.AddDate(0, -1, 0),
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
	}
	usage := []billing.UsageSummary{
		{Channel: "email", CredentialSource: billing.CredSourceSystem, MessageCount: 7500},
	}
	svc := newLegacyCreditService(sub, usage, []*application.Application{{AppID: "app-1", AdminUserID: "user-1"}})

	_, err := svc.ReserveForNotification(context.Background(), "user-1", "app-1", "n-1", "email")
	if err != ErrInsufficientCredits {
		t.Fatalf("expected quota exceeded, got %v", err)
	}
}

func TestCreditService_legacy_byoc_skips_quota(t *testing.T) {
	now := time.Now().UTC()
	sub := &license.Subscription{
		TenantID:           "user-1",
		Plan:               "pro",
		Status:             license.SubscriptionStatusActive,
		CurrentPeriodStart: now.AddDate(0, -1, 0),
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
	}
	usage := []billing.UsageSummary{
		{Channel: "sms", CredentialSource: billing.CredSourceSystem, MessageCount: 7500},
	}
	app := &application.Application{
		AppID:       "app-1",
		AdminUserID: "user-1",
		Settings: application.Settings{
			SMS: &application.SMSAppConfig{AccountSID: "AC", AuthToken: "tok"},
		},
	}
	svc := newLegacyCreditService(sub, usage, []*application.Application{app})

	res, err := svc.ReserveForNotification(context.Background(), "user-1", "app-1", "n-1", "sms")
	if err != nil {
		t.Fatalf("BYOC should skip quota: %v", err)
	}
	if res == nil {
		t.Fatal("expected reservation")
	}
}

func TestCreditService_credits_model_uses_wallet(t *testing.T) {
	now := time.Now().UTC()
	sub := &license.Subscription{
		TenantID:           "user-1",
		Plan:               "free",
		Status:             license.SubscriptionStatusTrial,
		CreditsTotal:       500,
		CreditsRemaining:   10,
		CurrentPeriodStart: now.AddDate(0, -1, 0),
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		Metadata: map[string]interface{}{
			"billing_model": billing.BillingModelCredits,
		},
	}
	svc := newLegacyCreditService(sub, nil, nil)

	_, err := svc.ReserveForNotification(context.Background(), "user-1", "app-1", "n-1", "email")
	if err != ErrInsufficientCredits {
		t.Fatalf("expected insufficient credits for wallet, got %v", err)
	}
}
