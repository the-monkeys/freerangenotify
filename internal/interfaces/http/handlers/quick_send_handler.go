package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

// QuickSendHandler handles the simplified POST /v1/quick-send endpoint.
type QuickSendHandler struct {
	service   *usecases.QuickSendService
	validator *validator.Validator
	logger    *zap.Logger
}

// NewQuickSendHandler creates a new QuickSendHandler.
func NewQuickSendHandler(service *usecases.QuickSendService, v *validator.Validator, logger *zap.Logger) *QuickSendHandler {
	return &QuickSendHandler{service: service, validator: v, logger: logger}
}

// Send handles POST /v1/quick-send
func (h *QuickSendHandler) Send(c *fiber.Ctx) error {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "application not authenticated"})
	}

	var req dto.QuickSendRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if err := h.validator.Validate(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if envID, ok := c.Locals("environment_id").(string); ok {
		req.EnvironmentID = envID
	}

	result, err := h.service.Send(c.Context(), appID, &req)
	if err != nil {
		h.logger.Error("Quick-send failed", zap.String("app_id", appID), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusAccepted).JSON(result)
}
