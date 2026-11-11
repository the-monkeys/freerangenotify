package handlers

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/tenant"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
)

func channelAccess(ctx context.Context, app *application.Application, userID string, members auth.MembershipRepository, tenants tenant.Service, manage bool) (auth.Role, error) {
	role := auth.ApplicationRole(ctx, app, userID, members, tenants)
	if role == "" || manage && (role != auth.RoleOwner && role != auth.RoleAdmin) {
		return "", errors.Forbidden("You do not have access to this application operation")
	}
	return role, nil
}

// RevealAPIKey is explicit and owner-only; routine reads carry no SDK credential.
// @Summary RevealAPI Key
// @Description Requires an account JWT and permission for the selected application or tenant.
// @Tags Application
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/apps/{id}/reveal-key [post]
// @Security BearerAuth
func (h *ApplicationHandler) RevealAPIKey(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	app, role, err := h.authorizeAppAccess(c, c.Params("id"), userID)
	if err != nil {
		return err
	}
	if role != auth.RoleOwner {
		return errors.Forbidden("only the application owner can reveal the API key")
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"api_key": app.APIKey}})
}
