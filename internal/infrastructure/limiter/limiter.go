package limiter

import (
	"context"
	"time"
)

// Limiter defines the interface for rate limiting and frequency control
type Limiter interface {
	// Allow checks if an action is allowed based on a sliding window rate limit
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)

	// IncrementAndCheckDailyLimit increments the counter for a key and checks if it exceeds the daily limit
	IncrementAndCheckDailyLimit(ctx context.Context, key string, limit int) (bool, error)

	// ResetDailyLimit manually resets the daily counter for a key
	ResetDailyLimit(ctx context.Context, key string) error
}

// Admission describes this request's atomic quota decision.
type Admission struct {
	Allowed    bool
	Limit      int
	Remaining  int
	ResetAt    time.Time
	RetryAfter time.Duration
}

type AdmissionLimiter interface {
	Admit(context.Context, string, int, time.Duration) (Admission, error)
}
