package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/environment"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// APIKeyAuth creates a middleware for API key authentication.
// When envService is non-nil (multi-environment enabled), it performs
// two-phase resolution: first by app key, then by environment key.
func APIKeyAuth(appService usecases.ApplicationService, logger *zap.Logger, opts ...APIKeyAuthOption) fiber.Handler {
	cfg := apiKeyAuthConfig{}
	for _, o := range opts {
		o(&cfg)
	}

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

		logger.Info("Validating API Key", zap.String("received_key", apiKey))

		if apiKey == "" {
			return errors.Unauthorized("Missing API key")
		}

		// Phase 1: Try to resolve by application API key
		app, err := appService.ValidateAPIKey(c.Context(), apiKey)
		if err == nil {
			// App-level key resolved
			c.Locals("app_id", app.AppID)
			c.Locals("app_name", app.AppName)
			c.Locals("app", app)
			if cfg.envService != nil {
				// Multi-env enabled — app-level key maps to default environment
				c.Locals("environment_id", "default")
			}

			logger.Debug("API key authenticated",
				zap.String("app_id", app.AppID),
				zap.String("app_name", app.AppName),
				zap.String("path", c.Path()),
			)
			return c.Next()
		}

		// Phase 2: If multi-environment is enabled, try environment API key
		if cfg.envService != nil {
			env, envErr := cfg.envService.GetByAPIKey(c.Context(), apiKey)
			if envErr == nil && env != nil {
				// Fetch parent application
				parentApp, appErr := appService.GetByID(c.Context(), env.AppID)
				if appErr != nil {
					logger.Warn("Environment key valid but parent app not found",
						zap.String("env_id", env.ID),
						zap.String("app_id", env.AppID),
					)
					return errors.New(errors.ErrCodeInvalidAPIKey, "Invalid API key")
				}

				c.Locals("app_id", parentApp.AppID)
				c.Locals("app_name", parentApp.AppName)
				c.Locals("app", parentApp)
				c.Locals("environment_id", env.ID)

				logger.Debug("Environment API key authenticated",
					zap.String("app_id", parentApp.AppID),
					zap.String("env_id", env.ID),
					zap.String("env_name", env.Name),
					zap.String("path", c.Path()),
				)
				return c.Next()
			}
		}

		logger.Warn("Invalid API key attempt",
			zap.String("ip", c.IP()),
			zap.String("path", c.Path()),
		)
		return errors.New(errors.ErrCodeInvalidAPIKey, "Invalid API key")
	}
}

// apiKeyAuthConfig holds optional configuration for the APIKeyAuth middleware.
type apiKeyAuthConfig struct {
	envService environment.Service
}

// APIKeyAuthOption configures the APIKeyAuth middleware.
type APIKeyAuthOption func(*apiKeyAuthConfig)

// WithEnvironmentService enables multi-environment API key resolution.
func WithEnvironmentService(svc environment.Service) APIKeyAuthOption {
	return func(cfg *apiKeyAuthConfig) {
		cfg.envService = svc
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
