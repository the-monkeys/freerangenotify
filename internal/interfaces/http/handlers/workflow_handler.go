package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/resourcelink"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

// WorkflowHandler handles HTTP requests for workflow operations.
type WorkflowHandler struct {
	service   workflow.Service
	validator *validator.Validator
	logger    *zap.Logger
	linkRepo  resourcelink.Repository
	wfRepo    workflow.Repository
}

func (h *WorkflowHandler) SetLinkRepo(repo resourcelink.Repository) { h.linkRepo = repo }
func (h *WorkflowHandler) SetWorkflowRepo(repo workflow.Repository) { h.wfRepo = repo }

// NewWorkflowHandler creates a new workflow handler.
func NewWorkflowHandler(service workflow.Service, v *validator.Validator, logger *zap.Logger) *WorkflowHandler {
	return &WorkflowHandler{service: service, validator: v, logger: logger}
}

// Create handles POST /v1/workflows
// @Summary Create a workflow
// @Description Create a new notification workflow with steps and triggers
// @Tags Workflows
// @Accept json
// @Produce json
// @Param body body workflow.CreateRequest true "Workflow creation request"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/workflows [post]
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
// @Summary List workflows
// @Description List all workflows for the authenticated application
// @Tags Workflows
// @Produce json
// @Param limit query int false "Limit results" default(20)
// @Param offset query int false "Offset for pagination" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/workflows [get]
func (h *WorkflowHandler) List(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	var envID string
	if id, ok := c.Locals("environment_id").(string); ok {
		envID = id
	}

	// Include linked workflows from other apps in a single query.
	var linkedIDs []string
	if h.linkRepo != nil {
		linkedIDs, _ = h.linkRepo.GetAllLinkedResourceIDs(c.Context(), appID, resourcelink.TypeWorkflow)
	}

	workflows, total, err := h.service.List(c.Context(), appID, envID, linkedIDs, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    workflows,
		"total":   total,
	})
}

// Get handles GET /v1/workflows/:id
// @Summary Get a workflow
// @Description Retrieve a workflow by its ID
// @Tags Workflows
// @Produce json
// @Param id path string true "Workflow ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/workflows/{id} [get]
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
// @Summary Update a workflow
// @Description Update an existing workflow's configuration
// @Tags Workflows
// @Accept json
// @Produce json
// @Param id path string true "Workflow ID"
// @Param body body workflow.UpdateRequest true "Workflow update request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/workflows/{id} [put]
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
// @Summary Delete a workflow
// @Description Permanently remove a workflow
// @Tags Workflows
// @Produce json
// @Param id path string true "Workflow ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/workflows/{id} [delete]
func (h *WorkflowHandler) Delete(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")

	// Before deleting, check if other apps have imported this workflow.
	// If so, transfer ownership to the first consumer instead of destroying it.
	if h.linkRepo != nil && h.wfRepo != nil {
		consumers, _ := h.linkRepo.ListBySourceAndResource(c.Context(), appID, resourcelink.TypeWorkflow, id)
		if len(consumers) > 0 {
			wf, fetchErr := h.wfRepo.GetWorkflow(c.Context(), id)
			if fetchErr == nil && wf != nil && wf.AppID == appID {
				newOwner := consumers[0].TargetAppID
				wf.AppID = newOwner
				wf.UpdatedAt = time.Now()
				if upErr := h.wfRepo.UpdateWorkflow(c.Context(), wf); upErr != nil {
					h.logger.Error("Failed to transfer workflow ownership",
						zap.String("workflow_id", id), zap.String("new_owner", newOwner), zap.Error(upErr))
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": "failed to transfer resource ownership",
					})
				}
				_ = h.linkRepo.Delete(c.Context(), consumers[0].LinkID)
				for _, link := range consumers[1:] {
					link.SourceAppID = newOwner
					_ = h.linkRepo.UpdateLink(c.Context(), link)
				}
				h.logger.Info("Transferred workflow ownership to consumer app",
					zap.String("workflow_id", id), zap.String("from", appID), zap.String("to", newOwner))
				return c.JSON(fiber.Map{
					"success": true,
					"message": "workflow removed from this application (transferred to consumer app)",
				})
			}
		}
	}

	err := h.service.Delete(c.Context(), id, appID)
	if err != nil {
		// If the workflow belongs to another app, check if it's linked and unlink instead.
		if h.linkRepo != nil {
			exists, _ := h.linkRepo.Exists(c.Context(), appID, resourcelink.TypeWorkflow, id)
			if exists {
				if unlinkErr := h.linkRepo.DeleteByTargetAndResource(c.Context(), appID, resourcelink.TypeWorkflow, id); unlinkErr != nil {
					h.logger.Error("Failed to unlink imported workflow",
						zap.String("workflow_id", id), zap.String("app_id", appID), zap.Error(unlinkErr))
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": unlinkErr.Error(),
					})
				}
				h.logger.Info("Unlinked imported workflow from target app",
					zap.String("workflow_id", id), zap.String("app_id", appID))
				return c.JSON(fiber.Map{
					"success": true,
					"message": "linked workflow removed from this application",
				})
			}
			// Workflow exists but no link — stale reference from coarse-grained listing.
			return c.JSON(fiber.Map{
				"success": true,
				"message": "Workflow is not associated with this application",
			})
		}
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
// @Summary Trigger a workflow
// @Description Trigger a workflow execution with provided payload
// @Tags Workflows
// @Accept json
// @Produce json
// @Param body body workflow.TriggerRequest true "Workflow trigger request"
// @Success 202 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/workflows/trigger [post]
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

// TriggerByTopic handles POST /v1/workflows/trigger-by-topic
// @Summary Trigger workflow for topic subscribers
// @Description Trigger a workflow for all users subscribed to a topic
// @Tags Workflows
// @Accept json
// @Produce json
// @Param body body workflow.TriggerByTopicRequest true "Trigger by topic request"
// @Success 202 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/workflows/trigger-by-topic [post]
func (h *WorkflowHandler) TriggerByTopic(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)

	var req workflow.TriggerByTopicRequest
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

	result, err := h.service.TriggerByTopic(c.Context(), appID, &req)
	if err != nil {
		if appErr, ok := err.(*pkgerrors.AppError); ok {
			return c.Status(appErr.GetHTTPStatus()).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

// ListExecutions handles GET /v1/workflows/executions
// @Summary List workflow executions
// @Description List workflow execution history with optional workflow_id filter
// @Tags Workflows
// @Produce json
// @Param workflow_id query string false "Filter by workflow ID"
// @Param limit query int false "Limit results" default(20)
// @Param offset query int false "Offset for pagination" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/workflows/executions [get]
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
// @Summary Get a workflow execution
// @Description Retrieve details of a specific workflow execution
// @Tags Workflows
// @Produce json
// @Param id path string true "Execution ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/workflows/executions/{id} [get]
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
// @Summary Cancel a workflow execution
// @Description Cancel a running workflow execution
// @Tags Workflows
// @Produce json
// @Param id path string true "Execution ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/workflows/executions/{id}/cancel [post]
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
