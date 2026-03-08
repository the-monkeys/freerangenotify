package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/environment"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

// EnvironmentHandler handles environment-related HTTP requests.
// All endpoints are admin-only (JWT auth).
type EnvironmentHandler struct {
	service   environment.Service
	validator *validator.Validator
	logger    *zap.Logger
}

// NewEnvironmentHandler creates a new EnvironmentHandler.
func NewEnvironmentHandler(service environment.Service, v *validator.Validator, logger *zap.Logger) *EnvironmentHandler {
	return &EnvironmentHandler{
		service:   service,
		validator: v,
		logger:    logger,
	}
}

// Create handles POST /v1/apps/:id/environments
func (h *EnvironmentHandler) Create(c *fiber.Ctx) error {
	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app ID is required")
	}

	var req environment.CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("invalid request body")
	}
	req.AppID = appID

	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("validation failed", validator.FormatValidationErrors(err))
	}

	env, err := h.service.Create(c.Context(), req)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    env,
		"message": "Environment created successfully. Save the API key — it determines which environment receives notifications.",
	})
}

// List handles GET /v1/apps/:id/environments
func (h *EnvironmentHandler) List(c *fiber.Ctx) error {
	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app ID is required")
	}

	envs, err := h.service.ListByApp(c.Context(), appID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    envs,
	})
}

// Get handles GET /v1/apps/:id/environments/:envId
func (h *EnvironmentHandler) Get(c *fiber.Ctx) error {
	envID := c.Params("envId")
	if envID == "" {
		return errors.BadRequest("environment ID is required")
	}

	env, err := h.service.Get(c.Context(), envID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    env,
	})
}

// Delete handles DELETE /v1/apps/:id/environments/:envId
func (h *EnvironmentHandler) Delete(c *fiber.Ctx) error {
	envID := c.Params("envId")
	if envID == "" {
		return errors.BadRequest("environment ID is required")
	}

	if err := h.service.Delete(c.Context(), envID); err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Environment deleted successfully",
	})
}

// Promote handles POST /v1/apps/:id/environments/promote
func (h *EnvironmentHandler) Promote(c *fiber.Ctx) error {
	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app ID is required")
	}

	var req environment.PromoteRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("invalid request body")
	}

	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("validation failed", validator.FormatValidationErrors(err))
	}

	result, err := h.service.Promote(c.Context(), appID, req)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
		"message": "Promotion completed successfully",
	})
}
