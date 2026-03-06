package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/resourcelink"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

// WorkflowHandler handles HTTP requests for workflow operations.
type WorkflowHandler struct {
	service   workflow.Service
	validator *validator.Validator
	logger    *zap.Logger
	linkRepo  resourcelink.Repository
}

func (h *WorkflowHandler) SetLinkRepo(repo resourcelink.Repository) { h.linkRepo = repo }

// NewWorkflowHandler creates a new workflow handler.
func NewWorkflowHandler(service workflow.Service, v *validator.Validator, logger *zap.Logger) *WorkflowHandler {
	return &WorkflowHandler{service: service, validator: v, logger: logger}
}

// Create handles POST /v1/workflows
func (h *WorkflowHandler) Create(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)

	var req workflow.CreateRequest
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

	wf, err := h.service.Create(c.Context(), appID, &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    wf,
	})
}

// List handles GET /v1/workflows
func (h *WorkflowHandler) List(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	var envID string
	if id, ok := c.Locals("environment_id").(string); ok {
		envID = id
	}

	workflows, total, err := h.service.List(c.Context(), appID, envID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Merge linked workflows from other apps
	if h.linkRepo != nil {
		linkedAppIDs, _ := h.linkRepo.GetLinkedAppIDs(c.Context(), appID, resourcelink.TypeWorkflow)
		for _, srcAppID := range linkedAppIDs {
			linked, linkedTotal, lErr := h.service.List(c.Context(), srcAppID, envID, limit, 0)
			if lErr == nil {
				workflows = append(workflows, linked...)
				total += linkedTotal
			}
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    workflows,
		"total":   total,
	})
}

// Get handles GET /v1/workflows/:id
func (h *WorkflowHandler) Get(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")

	wf, err := h.service.Get(c.Context(), id, appID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    wf,
	})
}

// Update handles PUT /v1/workflows/:id
func (h *WorkflowHandler) Update(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")

	var req workflow.UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	wf, err := h.service.Update(c.Context(), id, appID, &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    wf,
	})
}

// Delete handles DELETE /v1/workflows/:id
func (h *WorkflowHandler) Delete(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")

	if err := h.service.Delete(c.Context(), id, appID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "workflow deleted",
	})
}

// Trigger handles POST /v1/workflows/trigger
func (h *WorkflowHandler) Trigger(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)

	var req workflow.TriggerRequest
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

	exec, err := h.service.Trigger(c.Context(), appID, &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"success": true,
		"data":    exec,
	})
}

// ListExecutions handles GET /v1/workflows/executions
func (h *WorkflowHandler) ListExecutions(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	workflowID := c.Query("workflow_id")
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	executions, total, err := h.service.ListExecutions(c.Context(), workflowID, appID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    executions,
		"total":   total,
	})
}

// GetExecution handles GET /v1/workflows/executions/:id
func (h *WorkflowHandler) GetExecution(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")

	exec, err := h.service.GetExecution(c.Context(), id, appID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    exec,
	})
}

// CancelExecution handles POST /v1/workflows/executions/:id/cancel
func (h *WorkflowHandler) CancelExecution(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")

	if err := h.service.CancelExecution(c.Context(), id, appID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "execution cancelled",
	})
}
