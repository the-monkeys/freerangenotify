package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
)

const (
	otpPrefix      = "frn:otp:register:"
	otpTTL         = 10 * time.Minute
	maxOTPAttempts = 5
)

// OTPRepository defines operations for managing pending registrations in Redis.
type OTPRepository interface {
	StorePendingRegistration(ctx context.Context, reg *auth.PendingRegistration) error
	GetPendingRegistration(ctx context.Context, email string) (*auth.PendingRegistration, error)
	DeletePendingRegistration(ctx context.Context, email string) error
	IncrementAttempts(ctx context.Context, email string) (int, error)

	StorePhoneOTP(ctx context.Context, userID string, reg *auth.PendingRegistration) error
	GetPhoneOTP(ctx context.Context, userID string) (*auth.PendingRegistration, error)
	DeletePhoneOTP(ctx context.Context, userID string) error
}

type redisOTPRepository struct {
	client *redis.Client
}

// NewOTPRepository creates a new Redis-backed OTP repository.
func NewOTPRepository(client *redis.Client) OTPRepository {
	return &redisOTPRepository{client: client}
}

func (r *redisOTPRepository) key(email string) string {
	return otpPrefix + email
}

func (r *redisOTPRepository) phoneKey(userID string) string {
	return "frn:otp:phone:" + userID
}

func (r *redisOTPRepository) StorePendingRegistration(ctx context.Context, reg *auth.PendingRegistration) error {
	data, err := json.Marshal(reg)
	if err != nil {
		return fmt.Errorf("failed to marshal pending registration: %w", err)
	}
	return r.client.Set(ctx, r.key(reg.Email), data, otpTTL).Err()
}

func (r *redisOTPRepository) GetPendingRegistration(ctx context.Context, email string) (*auth.PendingRegistration, error) {
	data, err := r.client.Get(ctx, r.key(email)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get pending registration: %w", err)
	}

	var reg auth.PendingRegistration
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pending registration: %w", err)
	}
	return &reg, nil
}

func (r *redisOTPRepository) DeletePendingRegistration(ctx context.Context, email string) error {
	return r.client.Del(ctx, r.key(email)).Err()
}

func (r *redisOTPRepository) IncrementAttempts(ctx context.Context, email string) (int, error) {
	reg, err := r.GetPendingRegistration(ctx, email)
	if err != nil {
		return 0, err
	}
	if reg == nil {
		return 0, fmt.Errorf("no pending registration found")
	}

	reg.Attempts++
	if reg.Attempts >= maxOTPAttempts {
		// Max attempts reached — delete the pending registration
		_ = r.DeletePendingRegistration(ctx, email)
		return reg.Attempts, nil
	}

	// Re-store with updated attempts, preserving remaining TTL
	ttl, err := r.client.TTL(ctx, r.key(email)).Result()
	if err != nil || ttl <= 0 {
		ttl = otpTTL
	}

	data, err := json.Marshal(reg)
	if err != nil {
		return reg.Attempts, fmt.Errorf("failed to marshal updated registration: %w", err)
	}
	if err := r.client.Set(ctx, r.key(email), data, ttl).Err(); err != nil {
		return reg.Attempts, fmt.Errorf("failed to update attempts: %w", err)
	}

	return reg.Attempts, nil
}

func (r *redisOTPRepository) StorePhoneOTP(ctx context.Context, userID string, reg *auth.PendingRegistration) error {
	data, err := json.Marshal(reg)
	if err != nil {
		return fmt.Errorf("failed to marshal phone OTP: %w", err)
	}
	return r.client.Set(ctx, r.phoneKey(userID), data, otpTTL).Err()
}

func (r *redisOTPRepository) GetPhoneOTP(ctx context.Context, userID string) (*auth.PendingRegistration, error) {
	data, err := r.client.Get(ctx, r.phoneKey(userID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get phone OTP: %w", err)
	}

	var reg auth.PendingRegistration
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal phone OTP: %w", err)
	}
	return &reg, nil
}

func (r *redisOTPRepository) DeletePhoneOTP(ctx context.Context, userID string) error {
	return r.client.Del(ctx, r.phoneKey(userID)).Err()
}
