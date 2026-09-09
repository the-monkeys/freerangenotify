package services

import (
	"context"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
)

type reservationStore interface {
	Save(ctx context.Context, reservation *billing.CreditReservation) error
	Get(ctx context.Context, id string) (*billing.CreditReservation, bool)
	Delete(ctx context.Context, id string)
	ListExpired(ctx context.Context, now time.Time) []*billing.CreditReservation
}

type memoryReservationStore struct {
	mu    sync.Mutex
	items map[string]*billing.CreditReservation
}

func newMemoryReservationStore() *memoryReservationStore {
	return &memoryReservationStore{items: make(map[string]*billing.CreditReservation)}
}

func newReservationStore(redisClient *redis.Client) reservationStore {
	if redisClient == nil {
		return newMemoryReservationStore()
	}
	return newRedisReservationStore(redisClient)
}

func (s *memoryReservationStore) Save(_ context.Context, reservation *billing.CreditReservation) error {
	if reservation == nil || reservation.ID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := *reservation
	s.items[reservation.ID] = &copied
	return nil
}

func (s *memoryReservationStore) Get(_ context.Context, id string) (*billing.CreditReservation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, ok := s.items[id]
	if !ok {
		return nil, false
	}
	copied := *res
	return &copied, true
}

func (s *memoryReservationStore) Delete(_ context.Context, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
}

func (s *memoryReservationStore) ListExpired(_ context.Context, now time.Time) []*billing.CreditReservation {
	s.mu.Lock()
	defer s.mu.Unlock()
	expired := make([]*billing.CreditReservation, 0)
	for _, res := range s.items {
		if res.Status == billing.CreditReservationReserved && !res.ExpiresAt.IsZero() && !res.ExpiresAt.After(now) {
			copied := *res
			expired = append(expired, &copied)
		}
	}
	return expired
}
