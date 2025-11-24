package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// APIKeyAuth creates a middleware for API key authentication
func APIKeyAuth(appService usecases.ApplicationService, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get API key from Authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return errors.Unauthorized("Missing Authorization header")
		}

		// Extract API key (format: "Bearer <api_key>" or just "<api_key>")
		apiKey := authHeader
		if strings.HasPrefix(authHeader, "Bearer ") {
			apiKey = strings.TrimPrefix(authHeader, "Bearer ")
		}

		if apiKey == "" {
			return errors.Unauthorized("Missing API key")
		}

		// Validate API key
		app, err := appService.ValidateAPIKey(c.Context(), apiKey)
		if err != nil {
			logger.Warn("Invalid API key attempt",
				zap.String("ip", c.IP()),
				zap.String("path", c.Path()),
			)
			return errors.New(errors.ErrCodeInvalidAPIKey, "Invalid API key")
		}

		// Store application info in context
		c.Locals("app_id", app.AppID)
		c.Locals("app_name", app.AppName)
		c.Locals("app", app)

		logger.Debug("API key authenticated",
			zap.String("app_id", app.AppID),
			zap.String("app_name", app.AppName),
			zap.String("path", c.Path()),
		)

		return c.Next()
	}
}

// OptionalAPIKeyAuth creates a middleware for optional API key authentication
// This allows requests to proceed without authentication but sets context if key is provided
func OptionalAPIKeyAuth(appService usecases.ApplicationService, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Next()
		}

		apiKey := authHeader
		if strings.HasPrefix(authHeader, "Bearer ") {
			apiKey = strings.TrimPrefix(authHeader, "Bearer ")
		}

		if apiKey != "" {
			app, err := appService.ValidateAPIKey(c.Context(), apiKey)
			if err == nil && app != nil {
				c.Locals("app_id", app.AppID)
				c.Locals("app_name", app.AppName)
				c.Locals("app", app)
			}
		}

		return c.Next()
	}
}
