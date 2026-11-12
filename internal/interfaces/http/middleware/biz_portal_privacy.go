package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/limiter"
	"go.uber.org/zap"
	"strings"
	"time"
)

// IsBizPortalRequest also covers unknown/invalid portal routes, before routing.
func IsBizPortalRequest(c *fiber.Ctx) bool {
	path := strings.ToLower(c.Path())
	return path == "/v1/portal" || strings.HasPrefix(path, "/v1/portal/") || strings.HasPrefix(path, "/v1/biz/portal-tokens/")
}
func SafeRequestPath(c *fiber.Ctx) string {
	if IsBizPortalRequest(c) {
		if strings.HasPrefix(strings.ToLower(c.Path()), "/v1/biz/portal-tokens/") {
			return "/v1/biz/portal-tokens/:id/revoke"
		}
		return "/v1/portal/:token"
	}
	return c.Path()
}

// Keep URL bearer credentials out of automatic span attributes. Dashboard spans
// keep their existing behavior. This wrapper must surround the real exporter.
func BizPortalTracing(next fiber.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if IsBizPortalRequest(c) {
			return c.Next()
		}
		return next(c)
	}
}

func BizPortalPrivacy(admission limiter.Limiter, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !IsBizPortalRequest(c) {
			return c.Next()
		}
		c.Set("Referrer-Policy", "no-referrer")
		c.Set("Cache-Control", "no-store")
		c.Set("Pragma", "no-cache")
		if admission == nil {
			return c.Status(503).JSON(fiber.Map{"error": "Portal access temporarily unavailable"})
		}
		// Fixed namespace and IP only: no bearer credential enters Redis keys/logs.
		allowed, err := admission.Allow(c.UserContext(), "biz-portal:"+c.IP(), 120, time.Minute)
		if err != nil {
			return c.Status(503).JSON(fiber.Map{"error": "Portal access temporarily unavailable"})
		}
		if !allowed {
			c.Set("Retry-After", "60")
			return c.Status(429).JSON(fiber.Map{"error": "Too many portal requests. Please try again shortly."})
		}
		err = c.Next()
		if logger != nil {
			logger.Info("Customer portal request", zap.String("route", SafeRequestPath(c)), zap.String("method", c.Method()), zap.Int("status", c.Response().StatusCode()))
		}
		return err
	}
}
