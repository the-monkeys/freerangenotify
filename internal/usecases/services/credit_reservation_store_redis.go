package services

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
)

const (
	creditReservationKeyPrefix = "frn:credit:res:"
	creditReservationExpiryZSet = "frn:credit:res:expiry"
	creditReservationTTL         = 24 * time.Hour
)

type redisReservationStore struct {
	client *redis.Client
}

func newRedisReservationStore(client *redis.Client) *redisReservationStore {
	return &redisReservationStore{client: client}
}

func creditReservationKey(id string) string {
	return creditReservationKeyPrefix + id
}

func (s *redisReservationStore) Save(ctx context.Context, reservation *billing.CreditReservation) error {
	if s == nil || s.client == nil || reservation == nil || reservation.ID == "" {
		return nil
	}
	payload, err := json.Marshal(reservation)
	if err != nil {
		return err
	}
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, creditReservationKey(reservation.ID), payload, creditReservationTTL)
	pipe.ZAdd(ctx, creditReservationExpiryZSet, &redis.Z{
		Score:  float64(reservation.ExpiresAt.UTC().Unix()),
		Member: reservation.ID,
	})
	_, err = pipe.Exec(ctx)
	return err
}

func (s *redisReservationStore) Get(ctx context.Context, id string) (*billing.CreditReservation, bool) {
	if s == nil || s.client == nil || id == "" {
		return nil, false
	}
	payload, err := s.client.Get(ctx, creditReservationKey(id)).Bytes()
	if err != nil {
		return nil, false
	}
	var reservation billing.CreditReservation
	if err := json.Unmarshal(payload, &reservation); err != nil {
		return nil, false
	}
	return &reservation, true
}

func (s *redisReservationStore) Delete(ctx context.Context, id string) {
	if s == nil || s.client == nil || id == "" {
		return
	}
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, creditReservationKey(id))
	pipe.ZRem(ctx, creditReservationExpiryZSet, id)
	_, _ = pipe.Exec(ctx)
}

func (s *redisReservationStore) ListExpired(ctx context.Context, now time.Time) []*billing.CreditReservation {
	if s == nil || s.client == nil {
		return nil
	}
	ids, err := s.client.ZRangeByScore(ctx, creditReservationExpiryZSet, &redis.ZRangeBy{
		Min: "0",
		Max: strconv.FormatInt(now.UTC().Unix(), 10),
	}).Result()
	if err != nil || len(ids) == 0 {
		return nil
	}
	expired := make([]*billing.CreditReservation, 0, len(ids))
	for _, id := range ids {
		res, ok := s.Get(ctx, id)
		if !ok || res.Status != billing.CreditReservationReserved {
			continue
		}
		expired = append(expired, res)
	}
	return expired
}
