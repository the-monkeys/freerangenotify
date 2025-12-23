package utils

import (
	"math"
	"math/rand"
	"time"
)

// CalculateBackoff calculates exponential backoff with jitter
// delay = baseDelay * 2^retryCount + jitter
func CalculateBackoff(baseDelay time.Duration, retryCount int, maxDelay time.Duration) time.Duration {
	// Calculate exponential part: baseDelay * 2^retryCount
	exp := math.Pow(2, float64(retryCount))
	delay := time.Duration(float64(baseDelay) * exp)

	// Add jitter (up to 20% of the delay)
	jitter := time.Duration(rand.Float64() * 0.2 * float64(delay))
	delay += jitter

	// Cap at maxDelay
	if maxDelay > 0 && delay > maxDelay {
		return maxDelay
	}

	return delay
}
