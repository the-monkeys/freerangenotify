package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/the-monkeys/freerangenotify/internal/domain/otp"
)

// otpAPIPrefix namespaces this repository's keys to avoid collisions with the
// internal auth-OTP repository (otp_repository.go), which uses "frn:otp:register:"
// and "frn:otp:phone:". The "api" segment makes the public OTP-as-a-service
// keyspace explicit.
const (
	otpAPIPrefix         = "frn:otp:api:"
	otpRecipientRLPrefix = "frn:otp:rl:"
)

type redisOTPAPIRepository struct {
	client *redis.Client
}

// NewOTPAPIRepository constructs a Redis-backed implementation of the public
// OTP-as-a-service repository contract.
func NewOTPAPIRepository(client *redis.Client) otp.Repository {
	return &redisOTPAPIRepository{client: client}
}

func (r *redisOTPAPIRepository) key(requestID string) string {
	return otpAPIPrefix + requestID
}

func (r *redisOTPAPIRepository) rlKey(appID, recipient string) string {
	return fmt.Sprintf("%s%s:%s", otpRecipientRLPrefix, appID, recipient)
}

func (r *redisOTPAPIRepository) Create(ctx context.Context, req *otp.Request) error {
	if req == nil || req.RequestID == "" {
		return fmt.Errorf("otp repo: request_id required")
	}
	ttl := time.Until(req.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("otp repo: expires_at must be in the future")
	}
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("otp repo: marshal: %w", err)
	}
	if err := r.client.Set(ctx, r.key(req.RequestID), data, ttl).Err(); err != nil {
		return fmt.Errorf("otp repo: set: %w", err)
	}
	return nil
}

func (r *redisOTPAPIRepository) Get(ctx context.Context, requestID string) (*otp.Request, error) {
	data, err := r.client.Get(ctx, r.key(requestID)).Bytes()
	if err == redis.Nil {
		return nil, otp.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("otp repo: get: %w", err)
	}
	var req otp.Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("otp repo: unmarshal: %w", err)
	}
	return &req, nil
}

// IncrementAttempts atomically increments the attempt counter inside the
// persisted record. We re-read, mutate, and write inside an optimistic loop —
// Redis-side WATCH/MULTI is overkill here because each OTP request has its
// own key and contention is bounded to one logical caller per request_id.
func (r *redisOTPAPIRepository) IncrementAttempts(ctx context.Context, requestID string) (int, error) {
	key := r.key(requestID)
	for attempt := 0; attempt < 5; attempt++ {
		err := r.client.Watch(ctx, func(tx *redis.Tx) error {
			data, err := tx.Get(ctx, key).Bytes()
			if err == redis.Nil {
				return otp.ErrNotFound
			}
			if err != nil {
				return err
			}
			var req otp.Request
			if err := json.Unmarshal(data, &req); err != nil {
				return fmt.Errorf("otp repo: unmarshal: %w", err)
			}
			req.Attempts++
			updated, err := json.Marshal(&req)
			if err != nil {
				return err
			}
			ttl := time.Until(req.ExpiresAt)
			if ttl <= 0 {
				return otp.ErrExpired
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				return pipe.Set(ctx, key, updated, ttl).Err()
			})
			if err != nil {
				return err
			}
			return nil
		}, key)
		if err == nil {
			// Re-read to return the canonical post-increment count. A second
			// reader could have incremented in the meantime but for the
			// "attempts remaining" UX hint this is good enough.
			cur, getErr := r.Get(ctx, requestID)
			if getErr != nil {
				return 0, getErr
			}
			return cur.Attempts, nil
		}
		if err == redis.TxFailedErr {
			continue
		}
		return 0, err
	}
	return 0, fmt.Errorf("otp repo: increment attempts exhausted retries")
}

func (r *redisOTPAPIRepository) MarkVerified(ctx context.Context, requestID string, verifiedAt time.Time) error {
	cur, err := r.Get(ctx, requestID)
	if err != nil {
		return err
	}
	cur.Verified = true
	cur.VerifiedAt = &verifiedAt
	return r.Update(ctx, cur)
}

func (r *redisOTPAPIRepository) Update(ctx context.Context, req *otp.Request) error {
	if req == nil || req.RequestID == "" {
		return fmt.Errorf("otp repo: request_id required")
	}
	ttl := time.Until(req.ExpiresAt)
	if ttl <= 0 {
		// Verified records are still useful briefly for idempotent verify
		// responses; cap at 60s so they don't linger.
		ttl = 60 * time.Second
	}
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("otp repo: marshal: %w", err)
	}
	return r.client.Set(ctx, r.key(req.RequestID), data, ttl).Err()
}

func (r *redisOTPAPIRepository) Delete(ctx context.Context, requestID string) error {
	return r.client.Del(ctx, r.key(requestID)).Err()
}

// RecipientRateLimit uses a rolling sliding-window counter via Redis INCR with
// a per-window expiry. Counting is approximate (fixed-window) which is fine
// for OTP abuse-prevention.
func (r *redisOTPAPIRepository) RecipientRateLimit(ctx context.Context, appID, recipient string, windowSeconds int) (int, error) {
	if windowSeconds <= 0 {
		windowSeconds = 3600
	}
	key := r.rlKey(appID, recipient)
	count, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("otp repo: rl incr: %w", err)
	}
	if count == 1 {
		// First hit in window — set the expiry. A failed Expire here is not
		// fatal; the key will just persist until the next INCR sets it.
		if err := r.client.Expire(ctx, key, time.Duration(windowSeconds)*time.Second).Err(); err != nil {
			return int(count), fmt.Errorf("otp repo: rl expire: %w", err)
		}
	}
	return int(count), nil
}
