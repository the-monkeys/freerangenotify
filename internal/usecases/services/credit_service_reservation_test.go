package services

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"go.uber.org/zap"
)

// racyBalanceRepo copies on Get and replaces on Upsert, matching the production
// Elasticsearch read-modify-write. Per-call locks avoid data races on the
// pointer itself; they do not make Get→mutate→Upsert atomic.
type racyBalanceRepo struct {
	mu      sync.Mutex
	balance *billing.CreditBalance
}

func (r *racyBalanceRepo) GetByTenantID(_ context.Context, tenantID string) (*billing.CreditBalance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.balance == nil {
		return nil, nil
	}
	copied := *r.balance
	copied.TenantID = tenantID
	return &copied, nil
}

func (r *racyBalanceRepo) Upsert(_ context.Context, balance *billing.CreditBalance) error {
	if balance == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := *balance
	r.balance = &copied
	return nil
}

func (r *racyBalanceRepo) ReserveCredits(_ context.Context, tenantID string, amount int64) (*billing.CreditBalance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.balance == nil {
		return nil, billing.ErrInsufficientCredits
	}
	if err := r.balance.Reserve(amount); err != nil {
		return nil, err
	}
	copied := *r.balance
	copied.TenantID = tenantID
	return &copied, nil
}

func (r *racyBalanceRepo) CommitReservedCredits(_ context.Context, tenantID string, amount int64) (*billing.CreditBalance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.balance == nil {
		return nil, billing.ErrInsufficientCredits
	}
	if err := r.balance.Commit(amount); err != nil {
		return nil, err
	}
	copied := *r.balance
	copied.TenantID = tenantID
	return &copied, nil
}

func (r *racyBalanceRepo) ReleaseReservedCredits(_ context.Context, tenantID string, amount int64) (*billing.CreditBalance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.balance == nil {
		return nil, nil
	}
	r.balance.Release(amount)
	copied := *r.balance
	copied.TenantID = tenantID
	return &copied, nil
}

func (r *racyBalanceRepo) ClearReservedCredits(_ context.Context, tenantID string) (*billing.CreditBalance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.balance == nil {
		return nil, nil
	}
	r.balance.ClearReserved()
	copied := *r.balance
	copied.TenantID = tenantID
	return &copied, nil
}

func newReservationCreditService(balance *billing.CreditBalance) *CreditService {
	repo := &racyBalanceRepo{balance: balance}
	return newReservationCreditServiceWith(repo, nil)
}

func newReservationCreditServiceWith(repo *racyBalanceRepo, store reservationStore) *CreditService {
	now := time.Now().UTC()
	sub := &license.Subscription{
		ID:                 "sub-1",
		TenantID:           repo.balance.TenantID,
		Plan:               "free",
		Status:             license.SubscriptionStatusActive,
		CreditsTotal:       repo.balance.CreditsTotal,
		CreditsRemaining:   repo.balance.CreditsRemaining,
		CreditsReserved:    repo.balance.CreditsReserved,
		CurrentPeriodStart: now.AddDate(0, -1, 0),
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		Metadata: map[string]interface{}{
			"billing_model": billing.BillingModelCredits,
		},
	}
	svc := NewCreditService(
		repo,
		&stubLedgerRepo{},
		&stubLicenseRepo{sub: sub},
		&stubUsageRepo{},
		&stubAppRepo{},
		nil,
		nil,
		zap.NewNop(),
		true,
	)
	if store != nil {
		svc.reservationStore = store
	}
	return svc
}

func TestCreditService_commit_survives_process_restart(t *testing.T) {
	repo := &racyBalanceRepo{balance: &billing.CreditBalance{
		ID:               "sub-1",
		TenantID:         "tenant-1",
		CreditsTotal:     100,
		CreditsRemaining: 100,
		CreditsReserved:  0,
	}}
	store := newMemoryReservationStore()
	svc1 := newReservationCreditServiceWith(repo, store)

	res, err := svc1.ReserveForNotification(context.Background(), "tenant-1", "app-1", "n-1", "inapp")
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}

	svc2 := newReservationCreditServiceWith(repo, store)
	if _, err := svc2.CommitOnSuccess(context.Background(), res.ID); err != nil {
		t.Fatalf("commit after restart: %v", err)
	}

	snap, err := svc2.GetUsageSnapshot(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snap.CreditsReserved != 0 {
		t.Fatalf("credits_reserved leaked after restart: got %d want 0", snap.CreditsReserved)
	}
	if snap.CreditsRemaining != 99 {
		t.Fatalf("credits_remaining = %d want 99", snap.CreditsRemaining)
	}
}

func TestCreditService_reaps_expired_reservations(t *testing.T) {
	repo := &racyBalanceRepo{balance: &billing.CreditBalance{
		ID:               "sub-1",
		TenantID:         "tenant-1",
		CreditsTotal:     100,
		CreditsRemaining: 100,
		CreditsReserved:  0,
	}}
	store := newMemoryReservationStore()
	svc := newReservationCreditServiceWith(repo, store)

	res, err := svc.ReserveForNotification(context.Background(), "tenant-1", "app-1", "n-1", "inapp")
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	res.ExpiresAt = time.Now().UTC().Add(-time.Minute)
	if err := store.Save(context.Background(), res); err != nil {
		t.Fatalf("expire reservation: %v", err)
	}

	released, err := svc.ReapExpiredReservations(context.Background())
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if released != 1 {
		t.Fatalf("released = %d want 1", released)
	}

	snap, err := svc.GetUsageSnapshot(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snap.CreditsReserved != 0 {
		t.Fatalf("credits_reserved after reap = %d want 0", snap.CreditsReserved)
	}
	if snap.CreditsRemaining != 100 {
		t.Fatalf("credits_remaining after reap = %d want 100", snap.CreditsRemaining)
	}
}

type recoveryNotificationFixture struct {
	notification.Repository
	notif *notification.Notification
}

func (r *recoveryNotificationFixture) GetByID(context.Context, string) (*notification.Notification, error) {
	return r.notif, nil
}

func TestLegacyReaperUsesBoundDeliveryEvidence(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		status                  notification.Status
		bound                   bool
		wantRemaining, wantHeld int64
		wantError               bool
	}{
		{"sent", notification.StatusSent, true, 99, 0, false},
		{"uncertain", notification.StatusProcessing, false, 100, 1, true},
		{"different-attempt", notification.StatusSent, false, 100, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			repo := &racyBalanceRepo{balance: &billing.CreditBalance{ID: "sub-1", TenantID: "tenant-1", CreditsTotal: 100, CreditsRemaining: 100}}
			store := newMemoryReservationStore()
			svc := newReservationCreditServiceWith(repo, store)
			res, err := svc.ReserveForNotification(ctx, "tenant-1", "app-1", "n-1", "inapp")
			if err != nil {
				t.Fatal(err)
			}
			res.ExpiresAt = time.Now().Add(-time.Minute)
			if err := store.Save(ctx, res); err != nil {
				t.Fatal(err)
			}
			notif := &notification.Notification{NotificationID: "n-1", AppID: "app-1", Status: tc.status, Metadata: map[string]interface{}{}}
			if tc.bound {
				notif.Metadata[billing.CreditDeliveryReservationMetadataKey] = res.ID
			}
			svc.SetReservationNotificationRepository(&recoveryNotificationFixture{notif: notif})
			_, err = svc.ReapExpiredReservations(ctx)
			if (err != nil) != tc.wantError {
				t.Fatalf("reconciliation error=%v, wantError=%v", err, tc.wantError)
			}
			if repo.balance.CreditsRemaining != tc.wantRemaining || repo.balance.CreditsReserved != tc.wantHeld {
				t.Fatalf("wrong delivery settlement: %+v", repo.balance)
			}
		})
	}
}

func TestCreditService_ResetReservedCredits_zeros_hold_and_keeps_remaining(t *testing.T) {
	svc := newReservationCreditService(&billing.CreditBalance{
		ID:               "sub-1",
		TenantID:         "tenant-1",
		CreditsTotal:     1500,
		CreditsRemaining: 445,
		CreditsReserved:  445,
	})

	snap, err := svc.ResetReservedCredits(context.Background(), "tenant-1", "incident 2026-07-28 reservation leak")
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if snap.CreditsReserved != 0 {
		t.Fatalf("credits_reserved = %d want 0", snap.CreditsReserved)
	}
	if snap.CreditsRemaining != 445 {
		t.Fatalf("credits_remaining = %d want 445", snap.CreditsRemaining)
	}
}

func TestCreditService_concurrent_reserve_commit_does_not_leak_reserved(t *testing.T) {
	const n = 50
	svc := newReservationCreditService(&billing.CreditBalance{
		ID:               "sub-1",
		TenantID:         "tenant-1",
		CreditsTotal:     1000,
		CreditsRemaining: 1000,
		CreditsReserved:  0,
	})

	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx := context.Background()
			res, err := svc.ReserveForNotification(ctx, "tenant-1", "app-1", fmt.Sprintf("n-%d", i), "inapp")
			if err != nil {
				errCh <- fmt.Errorf("reserve %d: %w", i, err)
				return
			}
			if res == nil {
				errCh <- fmt.Errorf("reserve %d: nil reservation", i)
				return
			}
			if _, err := svc.CommitOnSuccess(ctx, res.ID); err != nil {
				errCh <- fmt.Errorf("commit %d: %w", i, err)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}

	snap, err := svc.GetUsageSnapshot(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snap.CreditsReserved != 0 {
		t.Fatalf("credits_reserved leaked: got %d want 0 (remaining=%d)", snap.CreditsReserved, snap.CreditsRemaining)
	}
	if snap.CreditsRemaining != 1000-n {
		t.Fatalf("credits_remaining = %d want %d", snap.CreditsRemaining, 1000-n)
	}
}

type investigationFailOnceLedger struct {
	stubLedgerRepo
	failed bool
}

func (r *investigationFailOnceLedger) Append(ctx context.Context, entry *billing.CreditLedgerEntry) error {
	if !r.failed {
		r.failed = true
		return fmt.Errorf("simulated ledger write failure")
	}
	return nil
}

func TestInvestigation_LedgerFailureDoesNotDoubleBurn(t *testing.T) {
	svc := newReservationCreditService(&billing.CreditBalance{ID: "sub-1", TenantID: "tenant-1", CreditsTotal: 100, CreditsRemaining: 100})
	svc.ledgerRepo = &investigationFailOnceLedger{}
	res, err := svc.ReserveForNotification(context.Background(), "tenant-1", "app-1", "n-1", "inapp")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CommitOnSuccess(context.Background(), res.ID); err == nil {
		t.Fatal("expected ledger failure")
	}
	if _, err := svc.CommitOnSuccess(context.Background(), res.ID); err != nil {
		t.Fatal(err)
	}
	snap, err := svc.GetUsageSnapshot(context.Background(), "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	if snap.CreditsRemaining != 99 {
		t.Fatalf("balance after retry = %d; want 99 (one burn)", snap.CreditsRemaining)
	}
}

func TestInvestigation_ReleaseFromTwoWorkersDoesNotEraseAnotherHold(t *testing.T) {
	repo := &racyBalanceRepo{balance: &billing.CreditBalance{ID: "sub-1", TenantID: "tenant-1", CreditsTotal: 100, CreditsRemaining: 100}}
	store := newMemoryReservationStore()
	svc1 := newReservationCreditServiceWith(repo, store)
	svc2 := newReservationCreditServiceWith(repo, store)
	first, err := svc1.ReserveForNotification(context.Background(), "tenant-1", "app-1", "n-1", "inapp")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc1.ReserveForNotification(context.Background(), "tenant-1", "app-1", "n-2", "inapp"); err != nil {
		t.Fatal(err)
	}
	if err := svc2.ReleaseOnFailure(context.Background(), first.ID, "expired"); err != nil {
		t.Fatal(err)
	}
	if err := svc1.ReleaseOnFailure(context.Background(), first.ID, "delivery failed"); err != nil {
		t.Fatal(err)
	}
	snap, err := svc1.GetUsageSnapshot(context.Background(), "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	if snap.CreditsReserved != 1 {
		t.Fatalf("other notification's reserved credits = %d; want 1", snap.CreditsReserved)
	}
}
