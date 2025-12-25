package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/sse"
	"go.uber.org/zap"
)

// SSEHandler handles Server-Sent Events
type SSEHandler struct {
	broadcaster *sse.Broadcaster
	logger      *zap.Logger
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(broadcaster *sse.Broadcaster, logger *zap.Logger) *SSEHandler {
	return &SSEHandler{
		broadcaster: broadcaster,
		logger:      logger,
	}
}

// Connect establishes an SSE connection for a user
func (h *SSEHandler) Connect(c *fiber.Ctx) error {
	userID := c.Query("user_id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id query parameter is required",
		})
	}

	h.logger.Info("SSE connection request", zap.String("user_id", userID))

	// Handle the SSE connection
	return h.broadcaster.HandleSSE(c, userID)
}