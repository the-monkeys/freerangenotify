package handlers

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"strconv"
)

type ExotelWebhookHandler struct{ Service *services.ExotelStatusService }

// @Summary Handle
// @Description Inbound provider callback. Requires the configured provider signature or handshake verification; application API keys are not callback credentials.
// @Tags ExotelWebhook
// @Produce json
// @Param id path string true "id"
// @Success 202 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/webhooks/exotel/sms/{id} [post]
func (h *ExotelWebhookHandler) Handle(c *fiber.Ctx) error {
	if len(c.Body()) > 16384 {
		return c.SendStatus(fiber.StatusRequestEntityTooLarge)
	}
	expires, e := strconv.ParseInt(c.Query("expires"), 10, 64)
	if e != nil {
		return c.SendStatus(fiber.StatusForbidden)
	}
	_, e = h.Service.AcceptCallback(c.Context(), c.Params("id"), c.FormValue("SmsSid"), c.FormValue("Status"), expires, c.Query("proof"))
	if errors.Is(e, services.ErrExotelCallbackAuth) {
		return c.SendStatus(fiber.StatusForbidden)
	}
	if e != nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	return c.SendStatus(fiber.StatusAccepted)
}
