package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"go.uber.org/zap"
)

// AdminHandler handles administrative HTTP requests
type AdminHandler struct {
	queue  queue.Queue
	logger *zap.Logger
}

// NewAdminHandler creates a new AdminHandler
func NewAdminHandler(q queue.Queue, logger *zap.Logger) *AdminHandler {
	return &AdminHandler{
		queue:  q,
		logger: logger,
	}
}

// GetQueueStats handles GET /v1/admin/queues/stats
func (h *AdminHandler) GetQueueStats(c *fiber.Ctx) error {
	stats, err := h.queue.GetQueueDepth(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get queue stats",
		})
	}

	return c.JSON(fiber.Map{
		"stats": stats,
	})
}

// ListDLQ handles GET /v1/admin/queues/dlq
func (h *AdminHandler) ListDLQ(c *fiber.Ctx) error {
	limitStr := c.Query("limit", "10")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 10
	}

	items, err := h.queue.ListDLQ(c.Context(), limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list DLQ items",
		})
	}

	return c.JSON(fiber.Map{
		"items": items,
	})
}

// ReplayDLQ handles POST /v1/admin/queues/dlq/replay
func (h *AdminHandler) ReplayDLQ(c *fiber.Ctx) error {
	limitStr := c.Query("limit", "10")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 10
	}

	count, err := h.queue.ReplayDLQ(c.Context(), limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to replay DLQ items",
		})
	}

	return c.JSON(fiber.Map{
		"replayed_count": count,
	})
}
