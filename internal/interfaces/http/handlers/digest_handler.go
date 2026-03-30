package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/digest"
	"github.com/the-monkeys/freerangenotify/internal/domain/resourcelink"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

// DigestHandler handles HTTP requests for digest rule operations.
type DigestHandler struct {
	service   digest.Service
	validator *validator.Validator
	logger    *zap.Logger
	linkRepo  resourcelink.Repository
}

func (h *DigestHandler) SetLinkRepo(repo resourcelink.Repository) { h.linkRepo = repo }

// NewDigestHandler creates a new digest handler.
func NewDigestHandler(service digest.Service, v *validator.Validator, logger *zap.Logger) *DigestHandler {
	return &DigestHandler{service: service, validator: v, logger: logger}
}

// Create handles POST /v1/digest-rules
// @Summary Create a digest rule
// @Description Create a new digest/batching rule for notifications
// @Tags Digest Rules
// @Accept json
// @Produce json
// @Param body body digest.CreateRequest true "Digest rule creation request"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/digest-rules [post]
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
// @Summary List digest rules
// @Description List all digest rules for the authenticated application
// @Tags Digest Rules
// @Produce json
// @Param limit query int false "Limit results" default(20)
// @Param offset query int false "Offset for pagination" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/digest-rules [get]
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

	if h.linkRepo != nil {
		linkedAppIDs, _ := h.linkRepo.GetLinkedAppIDs(c.Context(), appID, resourcelink.TypeDigest)
		for _, srcAppID := range linkedAppIDs {
			linked, linkedTotal, lErr := h.service.List(c.Context(), srcAppID, envID, limit, 0)
			if lErr == nil {
				rules = append(rules, linked...)
				total += linkedTotal
			}
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    rules,
		"total":   total,
	})
}

// Get handles GET /v1/digest-rules/:id
// @Summary Get a digest rule
// @Description Retrieve a digest rule by its ID
// @Tags Digest Rules
// @Produce json
// @Param id path string true "Digest Rule ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/digest-rules/{id} [get]
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
// @Summary Update a digest rule
// @Description Update an existing digest rule
// @Tags Digest Rules
// @Accept json
// @Produce json
// @Param id path string true "Digest Rule ID"
// @Param body body digest.UpdateRequest true "Digest rule update request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/digest-rules/{id} [put]
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
// @Summary Delete a digest rule
// @Description Permanently remove a digest rule
// @Tags Digest Rules
// @Produce json
// @Param id path string true "Digest Rule ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/digest-rules/{id} [delete]
func (h *DigestHandler) Delete(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")

	if err := h.service.Delete(c.Context(), id, appID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if h.linkRepo != nil {
		if err := h.linkRepo.DeleteBySourceAndResource(c.Context(), appID, resourcelink.TypeDigest, id); err != nil {
			h.logger.Warn("Failed to clean up resource links for deleted digest rule",
				zap.String("digest_id", id), zap.String("app_id", appID), zap.Error(err))
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "digest rule deleted",
	})
}
