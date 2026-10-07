package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
)

type snapshotRateCard struct {
	billing.RateCardManager
	calls int
}

func (r *snapshotRateCard) GetActiveRateCard() *billing.RateCard {
	r.calls++
	return &billing.RateCard{Version: "card-before-refresh", ChannelCreditCost: map[string]int64{"sms": 800, "inapp": 7, "webhook": 9}}
}
func (r *snapshotRateCard) GetChannelCreditCost(string) int64 {
	panic("separate cost read permits a refresh race")
}
func (r *snapshotRateCard) GetRateCardVersion() string {
	panic("separate version read permits a refresh race")
}

func TestCreditServiceOneCardSnapshotAndAliases(t *testing.T) {
	for _, tc := range []struct {
		input, channel string
		cost           int64
	}{{" SMS ", "sms", 800}, {"push", "inapp", 7}, {"in_app", "inapp", 7}, {"teams", "webhook", 9}} {
		t.Run(tc.input, func(t *testing.T) {
			svc := newReservationCreditService(&billing.CreditBalance{ID: "sub-1", TenantID: "tenant-1", CreditsTotal: 1500, CreditsRemaining: 1500})
			card := &snapshotRateCard{}
			svc.rateCardSvc = card
			res, err := svc.ReserveForNotification(context.Background(), "tenant-1", "a", "n", tc.input)
			if err != nil || res.CreditsReserved != tc.cost || res.Channel != tc.channel || res.RateCardVersion != "card-before-refresh" || card.calls != 1 {
				t.Fatalf("inconsistent snapshot %+v calls=%d err=%v", res, card.calls, err)
			}
		})
	}
}

func TestCreditServiceDisabledReturnsNilReservation(t *testing.T) {
	svc := NewCreditService(nil, nil, nil, nil, nil, nil, nil, nil, false)
	res, err := svc.ReserveForNotification(context.Background(), "tenant", "app", "notification", "sms")
	if res != nil || err != nil {
		t.Fatalf("disabled metering %+v %v", res, err)
	}
}

type failingReservationStore struct {
	*memoryReservationStore
	failure error
}

func (s *failingReservationStore) GetWithError(context.Context, string) (*billing.CreditReservation, error) {
	return nil, s.failure
}
func (s *failingReservationStore) ListExpiredWithError(context.Context, time.Time) ([]*billing.CreditReservation, error) {
	return nil, s.failure
}

func TestCreditServiceStorageErrorIsNotMissing(t *testing.T) {
	svc := newReservationCreditService(&billing.CreditBalance{ID: "sub-1", TenantID: "tenant-1", CreditsTotal: 100, CreditsRemaining: 100})
	failure := errors.New("reservation store unavailable")
	svc.reservationStore = &failingReservationStore{memoryReservationStore: newMemoryReservationStore(), failure: failure}
	if _, err := svc.CommitOnSuccess(context.Background(), "r"); !errors.Is(err, failure) {
		t.Fatalf("commit hid storage failure: %v", err)
	}
	if err := svc.ReleaseOnFailure(context.Background(), "r", "failure"); !errors.Is(err, failure) {
		t.Fatalf("release hid storage failure: %v", err)
	}
	if _, err := svc.ReapExpiredReservations(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("reaper hid storage failure: %v", err)
	}
}

func TestCreditServiceJournalRollbackStopsAdmissions(t *testing.T) {
	svc := newReservationCreditService(&billing.CreditBalance{ID: "sub-1", TenantID: "tenant-1", CreditsTotal: 100, CreditsRemaining: 100, JournalBacked: true})
	_, err := svc.ReserveForNotification(context.Background(), "tenant-1", "a", "n", "inapp")
	if !errors.Is(err, billing.ErrCreditJournalAdmissionsDisabled) {
		t.Fatalf("old writers mixed into journal wallet: %v", err)
	}
}
