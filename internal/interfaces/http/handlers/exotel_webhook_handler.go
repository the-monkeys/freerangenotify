package handlers

import (
	"errors"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"strconv"
)

type ExotelWebhookHandler struct{ Service *services.ExotelStatusService }

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
