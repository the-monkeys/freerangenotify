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
		// Get Authorization header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return errors.Unauthorized("Missing Authorization header")
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

// OptionalJWTAuth creates a middleware for optional JWT authentication
// This allows requests to proceed without authentication but sets context if token is provided
func OptionalJWTAuth(authService auth.Service, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Next()
		}

		token := authHeader
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}

		if token != "" {
			user, err := authService.ValidateToken(c.Context(), token)
			if err == nil && user != nil {
				c.Locals("user_id", user.UserID)
				c.Locals("user_email", user.Email)
				c.Locals("user", user)
			}
		}

		return c.Next()
	}
}
