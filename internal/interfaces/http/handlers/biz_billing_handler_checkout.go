package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
)

func (h *BizBillingHandler) SetCheckoutService(service *services.BizCheckoutService) {
	h.checkout = service
}

// GetGatewayEffect reads a retained external attempt without exposing credentials.
// @Summary Get a business gateway operation
// @Description A pending external attempt is not a paid invoice or a completed refund. Use the operation reference returned by checkout errors; scope is the authenticated application.
// @Tags BizBilling
// @Produce json
// @Param id path string true "Gateway operation ID"
// @Success 200 {object} bizbilling.GatewayEffect
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/gateway-effects/{id} [get]
func (h *BizBillingHandler) GetGatewayEffect(c *fiber.Ctx) error {
	app, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.checkout == nil {
		return h.handleError(c, bizbilling.ErrMerchantUnavailable)
	}
	effect, err := h.checkout.GetEffect(c.UserContext(), app, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": effect})
}
