package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
)

func (h *BizBillingHandler) SetOutboxService(s *services.BizOutboxService) { h.outbox = s }

// ListNotificationDeliveries returns app-scoped delivery progress.
// @Summary List billing notification deliveries
// @Description Delivery progress is separate from payment completion. Accepted is not delivered. Failure codes contain no provider credentials or bearer links.
// @Tags BizBilling
// @Produce json
// @Param status query string false "pending, accepted, delivered, failed or unknown"
// @Param limit query int false "Page size, maximum 100"
// @Param offset query int false "Page offset, maximum 9900"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/notification-deliveries [get]
func (h *BizBillingHandler) ListNotificationDeliveries(c *fiber.Ctx) error {
	app, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.outbox == nil {
		return h.handleError(c, bizbilling.ErrReconciliationRequired)
	}
	rows, total, err := h.outbox.ListPage(c.UserContext(), app, c.Query("status"), h.queryInt(c, "limit", 20), h.queryInt(c, "offset", 0))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": rows, "total": total})
}

// GetNotificationDelivery returns one app-owned delivery.
// @Summary Get billing notification delivery
// @Tags BizBilling
// @Produce json
// @Param id path string true "Delivery ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/notification-deliveries/{id} [get]
func (h *BizBillingHandler) GetNotificationDelivery(c *fiber.Ctx) error {
	app, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.outbox == nil {
		return h.handleError(c, bizbilling.ErrReconciliationRequired)
	}
	row, err := h.outbox.Get(c.UserContext(), app, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": row})
}

// RetryNotificationDelivery schedules known safe admission failures only.
// @Summary Retry billing notification admission
// @Description Requires manage-billing permission for dashboard users. Never charges or refunds. Unknown, accepted, delivered and sandbox-suppressed items cannot be resent through this action.
// @Tags BizBilling
// @Produce json
// @Param id path string true "Delivery ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/notification-deliveries/{id}/retry [post]
func (h *BizBillingHandler) RetryNotificationDelivery(c *fiber.Ctx) error {
	app, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.outbox == nil {
		return h.handleError(c, bizbilling.ErrReconciliationRequired)
	}
	row, err := h.outbox.Retry(c.UserContext(), app, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": row})
}
