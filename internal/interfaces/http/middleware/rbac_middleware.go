package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// RequirePermission returns a Fiber middleware that checks whether the
// authenticated user holds the specified permission for the current application.
//
// It reads the app_id and user_id from Fiber locals (set by the auth middleware)
// and looks up the user's membership via the MembershipRepository.
//
// If RBAC is disabled or the user is the app owner (AdminUserID), the check
// is bypassed.
func RequirePermission(perm auth.Permission, membershipRepo auth.MembershipRepository, appRepo application.Repository, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		appID, _ := c.Locals("app_id").(string)
		userID, _ := c.Locals("user_id").(string)

		// If no user_id is set (pure API-key auth), the caller is the app
		// owner by definition — allow through.
		if userID == "" {
			return c.Next()
		}

		// Check if the user is the app owner — owners have all permissions.
		if appRepo != nil {
			app, err := appRepo.GetByID(c.Context(), appID)
			if err == nil && app.AdminUserID == userID {
				c.Locals("role", auth.RoleOwner)
				return c.Next()
			}
		}

		membership, err := membershipRepo.GetByAppAndUser(c.Context(), appID, userID)
		if err != nil {
			logger.Warn("RBAC: membership lookup failed",
				zap.String("app_id", appID),
				zap.String("user_id", userID),
				zap.Error(err))
			return errors.Forbidden("insufficient permissions")
		}

		if !auth.HasPermission(membership.Role, perm) {
			logger.Info("RBAC: permission denied",
				zap.String("app_id", appID),
				zap.String("user_id", userID),
				zap.String("role", string(membership.Role)),
				zap.String("required_permission", string(perm)))
			return errors.Forbidden("insufficient permissions")
		}

		// Stash role for downstream handlers/middleware
		c.Locals("role", membership.Role)

		return c.Next()
	}
}
