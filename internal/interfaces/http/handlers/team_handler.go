package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// TeamHandler exposes endpoints for managing app memberships/teams.
type TeamHandler struct {
	service auth.TeamService
	logger  *zap.Logger
}

// NewTeamHandler creates a new TeamHandler.
func NewTeamHandler(service auth.TeamService, logger *zap.Logger) *TeamHandler {
	return &TeamHandler{service: service, logger: logger}
}

// InviteMember adds a new member to the application.
// POST /v1/apps/:app_id/team
func (h *TeamHandler) InviteMember(c *fiber.Ctx) error {
	appID := c.Params("app_id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
	}

	var req auth.InviteMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("invalid request body")
	}

	inviterID, _ := c.Locals("app_id").(string) // The authenticated app/user

	membership, err := h.service.InviteMember(c.Context(), appID, &req, inviterID)
	if err != nil {
		return errors.BadRequest(err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(membership)
}

// ListMembers returns all members of an application.
// GET /v1/apps/:app_id/team
func (h *TeamHandler) ListMembers(c *fiber.Ctx) error {
	appID := c.Params("app_id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
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
	appID := c.Params("app_id")
	membershipID := c.Params("membership_id")
	if appID == "" || membershipID == "" {
		return errors.BadRequest("app_id and membership_id are required")
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
	appID := c.Params("app_id")
	membershipID := c.Params("membership_id")
	if appID == "" || membershipID == "" {
		return errors.BadRequest("app_id and membership_id are required")
	}

	if err := h.service.RemoveMember(c.Context(), appID, membershipID); err != nil {
		return errors.BadRequest(err.Error())
	}

	return c.SendStatus(fiber.StatusNoContent)
}
