package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"go.uber.org/zap"
)

type PresenceHandler struct {
	service usecases.PresenceService
	logger  *zap.Logger
}

func NewPresenceHandler(service usecases.PresenceService, logger *zap.Logger) *PresenceHandler {
	return &PresenceHandler{
		service: service,
		logger:  logger,
	}
}

type CheckInRequest struct {
	UserID     string `json:"user_id" validate:"required"`
	DynamicURL string `json:"dynamic_url"`
}

// CheckIn handles POST /v1/presence/check-in
func (h *PresenceHandler) CheckIn(c *fiber.Ctx) error {
	var req CheckInRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// app_id is set by APIKeyAuth middleware
	appID := c.Locals("app_id").(string)

	if err := h.service.CheckIn(c.Context(), req.UserID, appID, req.DynamicURL); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "User checked-in successfully, pending notifications triggered",
	})
}
