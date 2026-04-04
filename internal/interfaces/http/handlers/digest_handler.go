package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/digest"
	"github.com/the-monkeys/freerangenotify/internal/domain/resourcelink"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

// DigestHandler handles HTTP requests for digest rule operations.
type DigestHandler struct {
	service    digest.Service
	validator  *validator.Validator
	logger     *zap.Logger
	linkRepo   resourcelink.Repository
	digestRepo digest.Repository
}

func (h *DigestHandler) SetLinkRepo(repo resourcelink.Repository) { h.linkRepo = repo }
func (h *DigestHandler) SetDigestRepo(repo digest.Repository)     { h.digestRepo = repo }

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

	// Include linked digest rules from other apps in a single query.
	var linkedIDs []string
	if h.linkRepo != nil {
		linkedIDs, _ = h.linkRepo.GetAllLinkedResourceIDs(c.Context(), appID, resourcelink.TypeDigest)
	}

	rules, total, err := h.service.List(c.Context(), appID, envID, linkedIDs, limit, offset)
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

	// Before deleting, check if other apps have imported this digest rule.
	// If so, transfer ownership to the first consumer instead of destroying it.
	if h.linkRepo != nil && h.digestRepo != nil {
		consumers, _ := h.linkRepo.ListBySourceAndResource(c.Context(), appID, resourcelink.TypeDigest, id)
		if len(consumers) > 0 {
			rule, fetchErr := h.digestRepo.GetByID(c.Context(), id)
			if fetchErr == nil && rule != nil && rule.AppID == appID {
				newOwner := consumers[0].TargetAppID
				rule.AppID = newOwner
				rule.UpdatedAt = time.Now()
				if upErr := h.digestRepo.Update(c.Context(), rule); upErr != nil {
					h.logger.Error("Failed to transfer digest rule ownership",
						zap.String("digest_id", id), zap.String("new_owner", newOwner), zap.Error(upErr))
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": "failed to transfer resource ownership",
					})
				}
				_ = h.linkRepo.Delete(c.Context(), consumers[0].LinkID)
				for _, link := range consumers[1:] {
					link.SourceAppID = newOwner
					_ = h.linkRepo.UpdateLink(c.Context(), link)
				}
				h.logger.Info("Transferred digest rule ownership to consumer app",
					zap.String("digest_id", id), zap.String("from", appID), zap.String("to", newOwner))
				return c.JSON(fiber.Map{
					"success": true,
					"message": "digest rule removed from this application (transferred to consumer app)",
				})
			}
		}
	}

	err := h.service.Delete(c.Context(), id, appID)
	if err != nil {
		// If the digest rule belongs to another app, check if it's linked and unlink instead.
		if h.linkRepo != nil {
			exists, _ := h.linkRepo.Exists(c.Context(), appID, resourcelink.TypeDigest, id)
			if exists {
				if unlinkErr := h.linkRepo.DeleteByTargetAndResource(c.Context(), appID, resourcelink.TypeDigest, id); unlinkErr != nil {
					h.logger.Error("Failed to unlink imported digest rule",
						zap.String("digest_id", id), zap.String("app_id", appID), zap.Error(unlinkErr))
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": unlinkErr.Error(),
					})
				}
				h.logger.Info("Unlinked imported digest rule from target app",
					zap.String("digest_id", id), zap.String("app_id", appID))
				return c.JSON(fiber.Map{
					"success": true,
					"message": "linked digest rule removed from this application",
				})
			}
			// Digest rule exists but no link — stale reference from coarse-grained listing.
			return c.JSON(fiber.Map{
				"success": true,
				"message": "Digest rule is not associated with this application",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "digest rule deleted",
	})
}
