package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// ErrorHandler creates a middleware for handling errors
func ErrorHandler(logger *zap.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		// Default status code
		statusCode := fiber.StatusInternalServerError
		response := fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "INTERNAL_ERROR",
				"message": "An unexpected error occurred",
			},
		}

		// Check if it's an AppError
		if appErr, ok := err.(*errors.AppError); ok {
			statusCode = appErr.GetHTTPStatus()
			response["error"] = fiber.Map{
				"code":    appErr.Code,
				"message": appErr.Message,
			}
			if len(appErr.Metadata) > 0 {
				response["error"].(fiber.Map)["details"] = appErr.Metadata
			}

			// Log error with context
			if statusCode >= 500 {
				logger.Error("Internal server error",
					zap.String("code", appErr.Code),
					zap.String("message", appErr.Message),
					zap.Error(appErr.Underlying),
					zap.String("path", c.Path()),
					zap.String("method", c.Method()),
				)
			} else {
				logger.Warn("Client error",
					zap.String("code", appErr.Code),
					zap.String("message", appErr.Message),
					zap.String("path", c.Path()),
					zap.String("method", c.Method()),
				)
			}
		} else if fiberErr, ok := err.(*fiber.Error); ok {
			// Handle Fiber errors
			statusCode = fiberErr.Code
			response["error"] = fiber.Map{
				"code":    "FIBER_ERROR",
				"message": fiberErr.Message,
			}
		} else {
			// Unknown error
			logger.Error("Unexpected error",
				zap.Error(err),
				zap.String("path", c.Path()),
				zap.String("method", c.Method()),
			)
		}

		return c.Status(statusCode).JSON(response)
	}
}
