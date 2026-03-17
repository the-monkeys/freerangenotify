package middleware

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/limiter"
	errorspkg "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// OpsRateLimit applies a strict sliding-window limit for /v1/ops routes.
func OpsRateLimit(l limiter.Limiter, limit int, window time.Duration, logger *zap.Logger) fiber.Handler {
	if limit <= 0 || window <= 0 {
		return func(c *fiber.Ctx) error { return c.Next() }
	}

	return func(c *fiber.Ctx) error {
		if l == nil {
			logger.Warn("Ops rate limiter is nil; skipping ops rate limit")
			return c.Next()
		}

		key := fmt.Sprintf("ops:%s:%s:%s", c.IP(), c.Method(), c.Path())
		allowed, err := l.Allow(c.Context(), key, limit, window)
		if err != nil {
			logger.Error("Ops rate limiter check failed", zap.Error(err), zap.String("key", key))
			return errorspkg.Internal("ops rate limit check failed", err)
		}
		if allowed {
			return c.Next()
		}

		retryAfter := int(window.Seconds())
		if retryAfter <= 0 {
			retryAfter = 1
		}
		c.Set("Retry-After", strconv.Itoa(retryAfter))
		return errorspkg.NewRateLimitError(retryAfter)
	}
}
