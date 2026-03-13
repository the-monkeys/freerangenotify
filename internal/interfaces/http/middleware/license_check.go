package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

// LicenseCheck validates licensing decision for critical write APIs.
// Must be attached after APIKeyAuth so app context is available in c.Locals.
func LicenseCheck(checker license.Checker, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if checker == nil || !checker.Enabled() {
			return c.Next()
		}

		app, ok := c.Locals("app").(*application.Application)
		if !ok || app == nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "license check failed",
				"code":  "license_check_error",
			})
		}

		decision, err := checker.Check(c.Context(), app)
		if err != nil {
			logger.Error("License check failed",
				zap.Error(err),
				zap.String("app_id", app.AppID),
				zap.String("path", c.Path()))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "license check failed",
				"code":  "license_check_error",
			})
		}

		if decision.Allowed {
			return c.Next()
		}

		code := "subscription_required"
		errorMsg := "valid subscription required"
		if decision.Mode == license.ModeSelfHosted {
			code = "license_required"
			errorMsg = "valid license required"
		}

		logger.Warn("Request blocked by licensing",
			zap.String("app_id", app.AppID),
			zap.String("path", c.Path()),
			zap.String("method", c.Method()),
			zap.String("mode", string(decision.Mode)),
			zap.String("state", string(decision.State)),
			zap.String("reason", decision.Reason))

		return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
			"error": errorMsg,
			"code":  code,
		})
	}
}
