package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// DashboardRBAC enforces role-based restrictions when a dashboard user
// accesses API-key-protected routes. It runs after APIKeyAuth.
//
// If a user_id is present in locals (dashboard flow — the UI sent both
// X-API-Key and a JWT), the middleware checks the user's role:
//   - Viewers are restricted to read-only (GET/HEAD/OPTIONS)
//   - Editors, admins, and owners have full access
//
// If no user_id is present (pure API-key call from an SDK or server),
// the request is allowed through unchanged for backward compatibility.
func DashboardRBAC(membershipRepo auth.MembershipRepository, appRepo application.Repository, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, _ := c.Locals("user_id").(string)
		if userID == "" {
			return c.Next()
		}

		appID, _ := c.Locals("app_id").(string)
		if appID == "" {
			return c.Next()
		}

		// Resolve role: app owner → RoleOwner, otherwise check membership
		var role auth.Role
		if appRepo != nil {
			app, err := appRepo.GetByID(c.Context(), appID)
			if err == nil && app.AdminUserID == userID {
				role = auth.RoleOwner
			}
		}

		if role == "" && membershipRepo != nil {
			membership, err := membershipRepo.GetByAppAndUser(c.Context(), appID, userID)
			if err == nil && membership != nil {
				role = membership.Role
			}
		}

		if role == "" {
			return errors.Forbidden("You do not have access to this application")
		}

		c.Locals("role", role)

		// Viewers are read-only
		if role == auth.RoleViewer {
			method := c.Method()
			if method != fiber.MethodGet && method != fiber.MethodHead && method != fiber.MethodOptions {
				logger.Info("DashboardRBAC: viewer write blocked",
					zap.String("user_id", userID),
					zap.String("app_id", appID),
					zap.String("method", method),
					zap.String("path", c.Path()))
				return errors.Forbidden("You don't have permission to perform this action. Your role (Viewer) only allows read access.")
			}
		}

		return c.Next()
	}
}
