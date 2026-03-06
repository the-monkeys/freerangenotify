package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/resourcelink"
	"github.com/the-monkeys/freerangenotify/internal/domain/topic"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

// TopicHandler handles HTTP requests for topic operations.
type TopicHandler struct {
	service   topic.Service
	validator *validator.Validator
	logger    *zap.Logger
	linkRepo  resourcelink.Repository
}

func (h *TopicHandler) SetLinkRepo(repo resourcelink.Repository) { h.linkRepo = repo }

// NewTopicHandler creates a new topic handler.
func NewTopicHandler(service topic.Service, v *validator.Validator, logger *zap.Logger) *TopicHandler {
	return &TopicHandler{service: service, validator: v, logger: logger}
}

// Create handles POST /v1/topics
func (h *TopicHandler) Create(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)

	var req topic.CreateRequest
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

	t, err := h.service.Create(c.Context(), appID, &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    t,
	})
}

// List handles GET /v1/topics
func (h *TopicHandler) List(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	var envID string
	if id, ok := c.Locals("environment_id").(string); ok {
		envID = id
	}

	topics, total, err := h.service.List(c.Context(), appID, envID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if h.linkRepo != nil {
		linkedAppIDs, _ := h.linkRepo.GetLinkedAppIDs(c.Context(), appID, resourcelink.TypeTopic)
		for _, srcAppID := range linkedAppIDs {
			linked, linkedTotal, lErr := h.service.List(c.Context(), srcAppID, envID, limit, 0)
			if lErr == nil {
				topics = append(topics, linked...)
				total += linkedTotal
			}
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    topics,
		"total":   total,
	})
}

// Get handles GET /v1/topics/:id
func (h *TopicHandler) Get(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")

	t, err := h.service.Get(c.Context(), id, appID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    t,
	})
}

// GetByKey handles GET /v1/topics/key/:key
func (h *TopicHandler) GetByKey(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	key := c.Params("key")

	t, err := h.service.GetByKey(c.Context(), appID, key)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    t,
	})
}

// Delete handles DELETE /v1/topics/:id
func (h *TopicHandler) Delete(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")

	if err := h.service.Delete(c.Context(), id, appID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Update handles PUT /v1/topics/:id
func (h *TopicHandler) Update(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")

	var req topic.UpdateRequest
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

	t, err := h.service.Update(c.Context(), id, appID, &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    t,
	})
}

// AddSubscribers handles POST /v1/topics/:id/subscribers
func (h *TopicHandler) AddSubscribers(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	topicID := c.Params("id")

	var req topic.AddSubscribersRequest
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

	if err := h.service.AddSubscribers(c.Context(), topicID, appID, req.UserIDs); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"added":   len(req.UserIDs),
	})
}

// RemoveSubscribers handles DELETE /v1/topics/:id/subscribers
func (h *TopicHandler) RemoveSubscribers(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	topicID := c.Params("id")

	var req topic.AddSubscribersRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if err := h.service.RemoveSubscribers(c.Context(), topicID, appID, req.UserIDs); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"removed": len(req.UserIDs),
	})
}

// GetSubscribers handles GET /v1/topics/:id/subscribers
func (h *TopicHandler) GetSubscribers(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	topicID := c.Params("id")
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	subs, total, err := h.service.GetSubscribers(c.Context(), topicID, appID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    subs,
		"total":   total,
	})
}
