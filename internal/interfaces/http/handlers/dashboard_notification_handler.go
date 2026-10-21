package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/dashboard_notification"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// DashboardNotificationHandler handles dashboard notification HTTP requests.
type DashboardNotificationHandler struct {
	repo   dashboard_notification.Repository
	logger *zap.Logger
}

// NewDashboardNotificationHandler creates a new DashboardNotificationHandler.
func NewDashboardNotificationHandler(repo dashboard_notification.Repository, logger *zap.Logger) *DashboardNotificationHandler {
	return &DashboardNotificationHandler{repo: repo, logger: logger}
}

// getUserID extracts the authenticated user ID from context (set by JWTAuth).
func (h *DashboardNotificationHandler) getUserID(c *fiber.Ctx) (string, error) {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return "", errors.Unauthorized("authentication required")
	}
	return userID, nil
}

// List returns dashboard notifications for the authenticated user.
// GET /v1/admin/notifications?limit=50&offset=0
// @Summary List
// @Description Requires an account JWT and permission for the selected application or tenant.
// @Tags DashboardNotification
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/admin/notifications [get]
// @Security BearerAuth
func (h *DashboardNotificationHandler) List(c *fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	limit := 50
	if v := c.Query("limit"); v != "" {
		if n, e := strconv.Atoi(v); e == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	offset := 0
	if v := c.Query("offset"); v != "" {
		if n, e := strconv.Atoi(v); e == nil && n >= 0 {
			offset = n
		}
	}

	list, total, err := h.repo.ListByUser(c.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("Failed to list dashboard notifications", zap.Error(err))
		return errors.Internal("failed to list notifications", err)
	}

	return c.JSON(fiber.Map{
		"notifications": list,
		"total":         total,
	})
}

// MarkRead marks notifications as read.
// POST /v1/admin/notifications/read
// Body: { "ids": ["id1", "id2"] }
// @Summary Mark Read
// @Description Requires an account JWT and permission for the selected application or tenant.
// @Tags DashboardNotification
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/admin/notifications/read [post]
// @Security BearerAuth
func (h *DashboardNotificationHandler) MarkRead(c *fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if len(req.IDs) == 0 {
		return c.JSON(fiber.Map{"marked": 0})
	}

	marked, err := h.repo.MarkRead(c.Context(), userID, req.IDs)
	if err != nil {
		h.logger.Error("Failed to mark notifications read", zap.Error(err))
		return errors.Internal("failed to mark notifications read", err)
	}

	return c.JSON(fiber.Map{"marked": marked})
}

// GetUnreadCount returns the number of unread notifications.
// GET /v1/admin/notifications/unread-count
// @Summary Get Unread Count
// @Description Requires an account JWT and permission for the selected application or tenant.
// @Tags DashboardNotification
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/admin/notifications/unread-count [get]
// @Security BearerAuth
func (h *DashboardNotificationHandler) GetUnreadCount(c *fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	count, err := h.repo.GetUnreadCount(c.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get unread count", zap.Error(err))
		return errors.Internal("failed to get unread count", err)
	}

	return c.JSON(fiber.Map{"unread_count": count})
}
