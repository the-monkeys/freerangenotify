package services

import (
	"context"
	"encoding/json"
	"errors"
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
	if reservation.Status == billing.CreditReservationReserved {
		// The hold outlives Redis TTLs. Keep its recovery evidence until an
		// explicit terminal transition, including across a long worker outage.
		pipe.Set(ctx, creditReservationKey(reservation.ID), payload, 0)
		pipe.ZAdd(ctx, creditReservationExpiryZSet, &redis.Z{Score: float64(reservation.ExpiresAt.UTC().Unix()), Member: reservation.ID})
	} else {
		pipe.Set(ctx, creditReservationKey(reservation.ID), payload, creditReservationTTL)
		pipe.ZRem(ctx, creditReservationExpiryZSet, reservation.ID)
	}
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
	var failures []error
	for _, id := range ids {
		res, err := s.GetWithError(ctx, id)
		if err != nil {
			failures = append(failures, fmt.Errorf("credit reservation %s: %w", id, err))
			continue
		}
		if res == nil {
			// A missing payload cannot establish whether delivery happened; keep
			// the index evidence and report it for audited reconciliation.
			failures = append(failures, fmt.Errorf("credit reservation %s has an expiry entry but no recovery record", id))
			continue
		}
		if res.Status != billing.CreditReservationReserved {
			if err := s.client.ZRem(ctx, creditReservationExpiryZSet, id).Err(); err != nil {
				failures = append(failures, err)
			}
			continue
		}
		expired = append(expired, res)
	}
	return expired, errors.Join(failures...)
}
