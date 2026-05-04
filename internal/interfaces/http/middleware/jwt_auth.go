package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// JWTAuth creates a middleware for JWT authentication
func JWTAuth(authService auth.Service, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get Authorization header or query token
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			// Check query parameter for SSE connections (EventSource cannot set headers)
			tokenQuery := c.Query("token")
			if tokenQuery != "" {
				authHeader = "Bearer " + tokenQuery
			} else {
				return errors.Unauthorized("Missing Authorization header")
			}
		}

		// Extract token (format: "Bearer <token>")
		token := authHeader
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			return errors.Unauthorized("Invalid Authorization header format. Expected: Bearer <token>")
		}

		if token == "" {
			return errors.Unauthorized("Missing JWT token")
		}

		// Validate token and get user
		user, err := authService.ValidateToken(c.Context(), token)
		if err != nil {
			logger.Warn("Invalid JWT token attempt",
				zap.String("ip", c.IP()),
				zap.String("path", c.Path()),
				zap.Error(err),
			)
			return errors.Unauthorized("Invalid or expired token")
		}

		// Store user info in context
		c.Locals("user_id", user.UserID)
		c.Locals("user_email", user.Email)
		c.Locals("user", user)

		logger.Debug("JWT authenticated",
			zap.String("user_id", user.UserID),
			zap.String("email", user.Email),
			zap.String("path", c.Path()),
		)

		return c.Next()
	}
}
