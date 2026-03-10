package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/schedule"
	pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

// ScheduleHandler handles HTTP requests for workflow schedule operations
type ScheduleHandler struct {
	service   schedule.Service
	validator *validator.Validator
	logger    *zap.Logger
}

// NewScheduleHandler creates a new schedule handler
func NewScheduleHandler(service schedule.Service, v *validator.Validator, logger *zap.Logger) *ScheduleHandler {
	return &ScheduleHandler{service: service, validator: v, logger: logger}
}

// Create handles POST /v1/workflows/schedules
func (h *ScheduleHandler) Create(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)

	var req schedule.CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := h.validator.Validate(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "validation failed", "details": err.Error()})
	}
	if envID, ok := c.Locals("environment_id").(string); ok {
		req.EnvironmentID = envID
	}

	sch, err := h.service.Create(c.Context(), appID, &req)
	if err != nil {
		if pkgerrors.IsBadRequest(err) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": sch})
}

// List handles GET /v1/workflows/schedules
func (h *ScheduleHandler) List(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	var envID string
	if id, ok := c.Locals("environment_id").(string); ok {
		envID = id
	}

	schedules, total, err := h.service.List(c.Context(), appID, envID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true, "data": schedules, "total": total})
}

// Get handles GET /v1/workflows/schedules/:id
func (h *ScheduleHandler) Get(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "schedule id required"})
	}

	sch, err := h.service.Get(c.Context(), id, appID)
	if err != nil {
		if pkgerrors.IsNotFound(err) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true, "data": sch})
}

// Update handles PUT /v1/workflows/schedules/:id
func (h *ScheduleHandler) Update(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "schedule id required"})
	}

	var req schedule.UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := h.validator.Validate(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "validation failed", "details": err.Error()})
	}

	sch, err := h.service.Update(c.Context(), id, appID, &req)
	if err != nil {
		if pkgerrors.IsBadRequest(err) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		if pkgerrors.IsNotFound(err) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true, "data": sch})
}

// Delete handles DELETE /v1/workflows/schedules/:id
func (h *ScheduleHandler) Delete(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "schedule id required"})
	}

	if err := h.service.Delete(c.Context(), id, appID); err != nil {
		if pkgerrors.IsNotFound(err) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true, "message": "Schedule deleted"})
}
