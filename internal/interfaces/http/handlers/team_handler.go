package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// TeamHandler exposes endpoints for managing app memberships/teams.
type TeamHandler struct {
	service auth.TeamService
	appRepo application.Repository
	logger  *zap.Logger
}

// NewTeamHandler creates a new TeamHandler.
func NewTeamHandler(service auth.TeamService, appRepo application.Repository, logger *zap.Logger) *TeamHandler {
	return &TeamHandler{service: service, appRepo: appRepo, logger: logger}
}

// extractTeamContext reads and validates the app_id and user_id from the
// request context. Permission checks are handled by the RequirePermission
// middleware on the route group, so the handler trusts that the caller has
// the PermManageMembers permission by the time it runs.
func (h *TeamHandler) extractTeamContext(c *fiber.Ctx) (appID, userID string, err error) {
	appID = c.Params("app_id")
	if appID == "" {
		return "", "", errors.BadRequest("app_id is required")
	}

	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return "", "", errors.Unauthorized("authentication required")
	}

	return appID, userID, nil
}

// InviteMember adds a new member to the application.
// POST /v1/apps/:app_id/team
func (h *TeamHandler) InviteMember(c *fiber.Ctx) error {
	appID, userID, err := h.extractTeamContext(c)
	if err != nil {
		return err
	}

	var req auth.InviteMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("invalid request body")
	}

	// Fetch the app to get the name for the invitation email
	appName := appID
	if h.appRepo != nil {
		if app, aErr := h.appRepo.GetByID(c.Context(), appID); aErr == nil {
			appName = app.AppName
		}
	}

	membership, err := h.service.InviteMember(c.Context(), appID, &req, userID, appName)
	if err != nil {
		return errors.BadRequest(err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(membership)
}

// ListMembers returns all members of an application.
// GET /v1/apps/:app_id/team
func (h *TeamHandler) ListMembers(c *fiber.Ctx) error {
	appID, _, err := h.extractTeamContext(c)
	if err != nil {
		return err
	}

	members, err := h.service.ListMembers(c.Context(), appID)
	if err != nil {
		return errors.Internal("failed to list team members", err)
	}

	return c.JSON(fiber.Map{
		"members": members,
		"count":   len(members),
	})
}

// UpdateRole changes a member's role.
// PUT /v1/apps/:app_id/team/:membership_id
func (h *TeamHandler) UpdateRole(c *fiber.Ctx) error {
	appID, _, err := h.extractTeamContext(c)
	if err != nil {
		return err
	}

	membershipID := c.Params("membership_id")
	if membershipID == "" {
		return errors.BadRequest("membership_id is required")
	}

	var req auth.UpdateMemberRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("invalid request body")
	}

	membership, err := h.service.UpdateRole(c.Context(), appID, membershipID, &req)
	if err != nil {
		return errors.BadRequest(err.Error())
	}

	return c.JSON(membership)
}

// RemoveMember removes a member from the application.
// DELETE /v1/apps/:app_id/team/:membership_id
func (h *TeamHandler) RemoveMember(c *fiber.Ctx) error {
	appID, _, err := h.extractTeamContext(c)
	if err != nil {
		return err
	}

	membershipID := c.Params("membership_id")
	if membershipID == "" {
		return errors.BadRequest("membership_id is required")
	}

	if err := h.service.RemoveMember(c.Context(), appID, membershipID); err != nil {
		return errors.BadRequest(err.Error())
	}

	return c.SendStatus(fiber.StatusNoContent)
}
