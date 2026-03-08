package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/digest"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

// DigestHandler handles HTTP requests for digest rule operations.
type DigestHandler struct {
	service   digest.Service
	validator *validator.Validator
	logger    *zap.Logger
}

// NewDigestHandler creates a new digest handler.
func NewDigestHandler(service digest.Service, v *validator.Validator, logger *zap.Logger) *DigestHandler {
	return &DigestHandler{service: service, validator: v, logger: logger}
}

// Create handles POST /v1/digest-rules
func (h *DigestHandler) Create(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)

	var req digest.CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if err := h.validator.Validate(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "validation failed",
			"details": err.Error(),
		})
	}

	if envID, ok := c.Locals("environment_id").(string); ok {
		req.EnvironmentID = envID
	}

	rule, err := h.service.Create(c.Context(), appID, &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    rule,
	})
}

// List handles GET /v1/digest-rules
func (h *DigestHandler) List(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	var envID string
	if id, ok := c.Locals("environment_id").(string); ok {
		envID = id
	}

	rules, total, err := h.service.List(c.Context(), appID, envID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    rules,
		"total":   total,
	})
}

// Get handles GET /v1/digest-rules/:id
func (h *DigestHandler) Get(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")

	rule, err := h.service.Get(c.Context(), id, appID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    rule,
	})
}

// Update handles PUT /v1/digest-rules/:id
func (h *DigestHandler) Update(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")

	var req digest.UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	rule, err := h.service.Update(c.Context(), id, appID, &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    rule,
	})
}

// Delete handles DELETE /v1/digest-rules/:id
func (h *DigestHandler) Delete(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")

	if err := h.service.Delete(c.Context(), id, appID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "digest rule deleted",
	})
}
