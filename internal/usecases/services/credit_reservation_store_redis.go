package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
)

const (
	creditReservationKeyPrefix  = "frn:credit:res:"
	creditReservationExpiryZSet = "frn:credit:res:expiry"
	creditReservationTTL        = 24 * time.Hour
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
	res, err := s.GetWithError(ctx, id)
	return res, err == nil && res != nil
}

func (s *redisReservationStore) GetWithError(ctx context.Context, id string) (*billing.CreditReservation, error) {
	if s == nil || s.client == nil || id == "" {
		return nil, fmt.Errorf("credit reservation store unavailable or invalid id")
	}
	payload, err := s.client.Get(ctx, creditReservationKey(id)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var reservation billing.CreditReservation
	if err := json.Unmarshal(payload, &reservation); err != nil {
		return nil, err
	}
	return &reservation, nil
}

func (s *redisReservationStore) Delete(ctx context.Context, id string) {
	_ = s.DeleteWithError(ctx, id)
}
func (s *redisReservationStore) DeleteWithError(ctx context.Context, id string) error {
	if s == nil || s.client == nil || id == "" {
		return fmt.Errorf("credit reservation store unavailable or invalid id")
	}
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, creditReservationKey(id))
	pipe.ZRem(ctx, creditReservationExpiryZSet, id)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *redisReservationStore) ListExpired(ctx context.Context, now time.Time) []*billing.CreditReservation {
	res, _ := s.ListExpiredWithError(ctx, now)
	return res
}
func (s *redisReservationStore) ListExpiredWithError(ctx context.Context, now time.Time) ([]*billing.CreditReservation, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("credit reservation store unavailable")
	}
	ids, err := s.client.ZRangeByScore(ctx, creditReservationExpiryZSet, &redis.ZRangeBy{
		Min: "0",
		Max: strconv.FormatInt(now.UTC().Unix(), 10),
	}).Result()
	if err != nil {
		return nil, err
	}
	expired := make([]*billing.CreditReservation, 0, len(ids))
	for _, id := range ids {
		res, err := s.GetWithError(ctx, id)
		if err != nil {
			return nil, err
		}
		if res == nil || res.Status != billing.CreditReservationReserved {
			continue
		}
		expired = append(expired, res)
	}
	return expired, nil
}
