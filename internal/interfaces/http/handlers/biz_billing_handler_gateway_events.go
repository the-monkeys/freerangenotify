package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
)

func (h *BizBillingHandler) SetGatewayEventService(service *services.BizGatewayEventService) {
	h.gatewayEvents = service
}

// GetGatewayEvent exposes minimized processing status only within its application.
// @Summary Get a business gateway event
// @Description Processing status and reconciliation failure code; contains no raw customer/card payload or credentials. A durable receipt is distinct from payment or refund completion.
// @Tags BizBilling
// @Produce json
// @Param id path string true "Gateway event receipt ID"
// @Success 200 {object} bizbilling.GatewayEventReceipt
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/gateway-events/{id} [get]
func (h *BizBillingHandler) GetGatewayEvent(c *fiber.Ctx) error {
	app, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.gatewayEvents == nil {
		return h.handleError(c, bizbilling.ErrMerchantUnavailable)
	}
	receipt, err := h.gatewayEvents.GetForApp(c.UserContext(), app, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": receipt})
}

// ListGatewayEvents lists bounded, application-scoped reconciliation records.
// @Summary List business gateway events
// @Description Use state=unmatched or quarantined to investigate unresolved events. Cursor is the last returned receipt ID. Payment history is not deleted when events are retried.
// @Tags BizBilling
// @Produce json
// @Param state query string false "Processing state"
// @Param cursor query string false "Last receipt ID"
// @Param limit query int false "Page size 1-200 (default 50)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/gateway-events [get]
func (h *BizBillingHandler) ListGatewayEvents(c *fiber.Ctx) error {
	app, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.gatewayEvents == nil {
		return h.handleError(c, bizbilling.ErrMerchantUnavailable)
	}
	rows, err := h.gatewayEvents.ListForApp(c.UserContext(), app, c.Query("state"), c.Query("cursor"), h.queryInt(c, "limit", 50))
	if err != nil {
		return h.handleError(c, err)
	}
	next := ""
	if len(rows) > 0 {
		next = rows[len(rows)-1].ID
	}
	return c.JSON(fiber.Map{"success": true, "data": rows, "next_cursor": next})
}
