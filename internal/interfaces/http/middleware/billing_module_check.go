package middleware

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// BillingModuleCheck verifies that the business billing module is enabled
// before allowing access to /v1/biz/* routes. When disabled, returns
// 402 Payment Required with a clear error code.
//
// Pattern follows LicenseCheck — must be attached after APIKeyAuth
// so app context is available in c.Locals.
func BillingModuleCheck(enabled bool, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if enabled {
			return c.Next()
		}

		appID, _ := c.Locals("app_id").(string)
		logger.Warn("Billing module access blocked — feature disabled",
			zap.String("app_id", appID),
			zap.String("path", c.Path()),
			zap.String("method", c.Method()))

		return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
			"error": "business billing module is not enabled",
			"code":  "billing_module_required",
		})
	}
}
