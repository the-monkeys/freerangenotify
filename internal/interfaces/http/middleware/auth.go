package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/environment"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// APIKeyAuth creates a middleware for API key authentication.
// It reads the API key from X-API-Key header first (dashboard flow), falling
// back to the Authorization header (SDK / server-to-server flow).
// When the API key comes via X-API-Key and a valid JWT is in Authorization,
// the dashboard user's identity is extracted so downstream RBAC can apply.
// When envService is non-nil (multi-environment enabled), it performs
// two-phase resolution: first by app key, then by environment key.
func APIKeyAuth(appService usecases.ApplicationService, logger *zap.Logger, opts ...APIKeyAuthOption) fiber.Handler {
	cfg := apiKeyAuthConfig{}
	for _, o := range opts {
		o(&cfg)
	}

	return func(c *fiber.Ctx) error {
		var apiKey string
		dashboardFlow := false

		// Prefer X-API-Key header (dashboard sends API key here, JWT in Authorization)
		if xKey := c.Get("X-API-Key"); xKey != "" {
			apiKey = xKey
			dashboardFlow = true
		} else {
			// Fallback: read from Authorization header (SDK backward compat)
			authHeader := c.Get("Authorization")
			if authHeader == "" {
				return errors.Unauthorized("Missing API key")
			}
			apiKey = authHeader
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if apiKey == "" {
			return errors.Unauthorized("Missing API key")
		}

		// Resolve app from API key
		resolvedApp := false

		// Phase 1: Try to resolve by application API key
		app, err := appService.ValidateAPIKey(c.Context(), apiKey)
		if err == nil {
			c.Locals("app_id", app.AppID)
			c.Locals("app_name", app.AppName)
			c.Locals("app", app)
			if cfg.envService != nil {
				c.Locals("environment_id", "default")
			}
			resolvedApp = true
		}

		// Phase 2: If multi-environment is enabled, try environment API key
		if !resolvedApp && cfg.envService != nil {
			env, envErr := cfg.envService.GetByAPIKey(c.Context(), apiKey)
			if envErr == nil && env != nil {
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
				resolvedApp = true
			}
		}

		if !resolvedApp {
			logger.Warn("Invalid API key attempt",
				zap.String("ip", c.IP()),
				zap.String("path", c.Path()),
			)
			return errors.New(errors.ErrCodeInvalidAPIKey, "Invalid API key")
		}

		// Dashboard flow: also extract JWT identity from Authorization header
		// so that DashboardRBAC can enforce per-user permissions.
		if dashboardFlow {
			if authHeader := c.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
				jwtToken := strings.TrimPrefix(authHeader, "Bearer ")
				if cfg.authService != nil && jwtToken != "" {
					user, jwtErr := cfg.authService.ValidateToken(c.Context(), jwtToken)
					if jwtErr == nil && user != nil {
						c.Locals("user_id", user.UserID)
						c.Locals("user_email", user.Email)
					}
				}
			}
		}

		logger.Debug("API key authenticated",
			zap.String("app_id", c.Locals("app_id").(string)),
			zap.Bool("dashboard_flow", dashboardFlow),
			zap.String("path", c.Path()),
		)
		return c.Next()
	}
}

// apiKeyAuthConfig holds optional configuration for the APIKeyAuth middleware.
type apiKeyAuthConfig struct {
	envService  environment.Service
	authService auth.Service
}

// APIKeyAuthOption configures the APIKeyAuth middleware.
type APIKeyAuthOption func(*apiKeyAuthConfig)

// WithEnvironmentService enables multi-environment API key resolution.
func WithEnvironmentService(svc environment.Service) APIKeyAuthOption {
	return func(cfg *apiKeyAuthConfig) {
		cfg.envService = svc
	}
}

// WithAuthService enables JWT identity extraction for dashboard RBAC.
func WithAuthService(svc auth.Service) APIKeyAuthOption {
	return func(cfg *apiKeyAuthConfig) {
		cfg.authService = svc
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
