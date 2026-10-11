package middleware

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/limiter"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
	"strconv"
	"time"
)

func requestAdmission(ctx context.Context, l limiter.Limiter, key string, limit int, window time.Duration) (limiter.Admission, error) {
	if l == nil {
		return limiter.Admission{}, apperrors.New(apperrors.ErrCodeServiceUnavailable, "Rate limiter unavailable")
	}
	if detailed, ok := l.(limiter.AdmissionLimiter); ok {
		return detailed.Admit(ctx, key, limit, window)
	}
	allowed, err := l.Allow(ctx, key, limit, window)
	return limiter.Admission{Allowed: allowed, Limit: limit, ResetAt: time.Now().Add(window), RetryAfter: window}, err
}
func enforceRequestQuota(c *fiber.Ctx, l limiter.Limiter, key string, limit int, window time.Duration, logger *zap.Logger) error {
	decision, err := requestAdmission(c.Context(), l, key, limit, window)
	if err != nil {
		logger.Warn("HTTP quota admission unavailable", zap.String("quota", key), zap.Error(err))
		return apperrors.New(apperrors.ErrCodeServiceUnavailable, "Rate limiter unavailable; try again later")
	}
	c.Set("X-RateLimit-Limit", strconv.Itoa(decision.Limit))
	c.Set("X-RateLimit-Remaining", strconv.Itoa(decision.Remaining))
	c.Set("X-RateLimit-Reset", strconv.FormatInt((decision.ResetAt.UnixMilli()+999)/1000, 10))
	if decision.Allowed {
		return c.Next()
	}
	retry := int((decision.RetryAfter + time.Second - 1) / time.Second)
	if retry < 1 {
		retry = 1
	}
	c.Set("Retry-After", strconv.Itoa(retry))
	return apperrors.NewRateLimitError(retry)
}

// ApplicationRateLimit must run after app authentication and dashboard authorization.
// All keys and environments share the app's hourly HTTP quota. Worker fan-out is separate.
func ApplicationRateLimit(l limiter.Limiter, enabled bool, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !enabled || c.Method() == fiber.MethodOptions {
			return c.Next()
		}
		app, ok := c.Locals("app").(*application.Application)
		if !ok || app == nil || app.AppID == "" {
			return apperrors.Unauthorized("Application not authenticated")
		}
		if app.Settings.RateLimit <= 0 {
			return c.Next()
		}
		return enforceRequestQuota(c, l, "app:"+app.AppID, app.Settings.RateLimit, time.Hour, logger)
	}
}

// AuthIPRateLimit applies the existing security limit to public authentication only.
func AuthIPRateLimit(l limiter.Limiter, limit int, window time.Duration, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if limit <= 0 || window <= 0 || c.Method() == fiber.MethodOptions {
			return c.Next()
		}
		return enforceRequestQuota(c, l, "auth-ip:"+c.IP(), limit, window, logger)
	}
}
