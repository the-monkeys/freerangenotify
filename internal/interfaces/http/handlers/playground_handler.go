package handlers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/sse"
	"go.uber.org/zap"
)

// PlaygroundHandler handles webhook and SSE playground HTTP requests
type PlaygroundHandler struct {
	redisClient *redis.Client
	baseURL     string
	broadcaster *sse.Broadcaster
	logger      *zap.Logger
}

// NewPlaygroundHandler creates a new PlaygroundHandler
func NewPlaygroundHandler(redisClient *redis.Client, baseURL string, logger *zap.Logger) *PlaygroundHandler {
	return &PlaygroundHandler{
		redisClient: redisClient,
		baseURL:     baseURL,
		logger:      logger,
	}
}

// SetBroadcaster injects the SSE broadcaster (setter injection to avoid circular deps).
func (h *PlaygroundHandler) SetBroadcaster(b *sse.Broadcaster) {
	h.broadcaster = b
}

// CreatePlayground handles POST /v1/admin/playground/webhook
// Generates a temporary webhook receiver URL stored in Redis with 30-minute TTL.
func (h *PlaygroundHandler) CreatePlayground(c *fiber.Ctx) error {
	playgroundID := uuid.New().String()[:8]

	key := "playground:" + playgroundID
	if err := h.redisClient.Set(c.Context(), key, "[]", 30*time.Minute).Err(); err != nil {
		h.logger.Error("Failed to create playground", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create playground",
		})
	}

	url := fmt.Sprintf("%s/v1/playground/%s", h.baseURL, playgroundID)

	h.logger.Info("Webhook playground created",
		zap.String("playground_id", playgroundID),
		zap.String("url", url))

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":         playgroundID,
		"url":        url,
		"expires_in": "30m",
	})
}

// ReceiveWebhook handles POST /v1/playground/:id
// Public endpoint — receives webhook payloads and appends them to the Redis list.
func (h *PlaygroundHandler) ReceiveWebhook(c *fiber.Ctx) error {
	playgroundID := c.Params("id")
	key := "playground:" + playgroundID

	// Check if playground exists
	exists, err := h.redisClient.Exists(c.Context(), key).Result()
	if err != nil || exists == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Playground not found or expired",
		})
	}

	// Read raw body
	body := c.Body()
	if len(body) == 0 {
		body = []byte("{}")
	}

	// Build the payload record
	record := map[string]interface{}{
		"headers":     c.GetReqHeaders(),
		"body":        json.RawMessage(body),
		"received_at": time.Now().Format(time.RFC3339),
	}
	data, _ := json.Marshal(record)

	// Append to a list key (separate from the existence key)
	listKey := "playground:payloads:" + playgroundID
	if err := h.redisClient.RPush(c.Context(), listKey, string(data)).Err(); err != nil {
		h.logger.Error("Failed to store playground payload", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to store payload",
		})
	}
	// Set same TTL on the list key
	h.redisClient.Expire(c.Context(), listKey, 30*time.Minute)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status": "received",
	})
}

// GetPayloads handles GET /v1/playground/:id
// Returns all received payloads for a playground.
func (h *PlaygroundHandler) GetPayloads(c *fiber.Ctx) error {
	playgroundID := c.Params("id")
	key := "playground:" + playgroundID

	// Check existence
	exists, err := h.redisClient.Exists(c.Context(), key).Result()
	if err != nil || exists == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Playground not found or expired",
		})
	}

	listKey := "playground:payloads:" + playgroundID
	raw, err := h.redisClient.LRange(c.Context(), listKey, 0, -1).Result()
	if err != nil {
		h.logger.Error("Failed to fetch playground payloads", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch payloads",
		})
	}

	payloads := make([]json.RawMessage, 0, len(raw))
	for _, r := range raw {
		payloads = append(payloads, json.RawMessage(r))
	}

	return c.JSON(fiber.Map{
		"id":       playgroundID,
		"payloads": payloads,
		"count":    len(payloads),
	})
}

// ──────────────────────────────────────────────────────────────────
// SSE Playground
// ──────────────────────────────────────────────────────────────────

// CreateSSEPlayground handles POST /v1/admin/playground/sse
// Generates a temporary user ID stored in Redis with 30-minute TTL.
// Returns the SSE connection URL for the browser to connect to.
func (h *PlaygroundHandler) CreateSSEPlayground(c *fiber.Ctx) error {
	playgroundID := "sse-" + uuid.New().String()[:8]

	key := "playground:sse:" + playgroundID
	if err := h.redisClient.Set(c.Context(), key, "active", 30*time.Minute).Err(); err != nil {
		h.logger.Error("Failed to create SSE playground", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create SSE playground",
		})
	}

	sseURL := fmt.Sprintf("%s/v1/sse?user_id=%s", h.baseURL, playgroundID)

	h.logger.Info("SSE playground created",
		zap.String("playground_id", playgroundID),
		zap.String("sse_url", sseURL))

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":         playgroundID,
		"sse_url":    sseURL,
		"expires_in": "30m",
	})
}

// SendSSETestMessage handles POST /v1/admin/playground/sse/:id/send
// Publishes a test notification to the SSE broadcaster for the playground user.
func (h *PlaygroundHandler) SendSSETestMessage(c *fiber.Ctx) error {
	playgroundID := c.Params("id")
	key := "playground:sse:" + playgroundID

	// Check if SSE playground exists
	exists, err := h.redisClient.Exists(c.Context(), key).Result()
	if err != nil || exists == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "SSE playground not found or expired",
		})
	}

	if h.broadcaster == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "SSE broadcaster not available",
		})
	}

	// Parse optional body for custom message
	var body struct {
		Title    string                 `json:"title"`
		Body     string                 `json:"body"`
		Category string                 `json:"category"`
		Data     map[string]interface{} `json:"data"`
	}
	if err := c.BodyParser(&body); err != nil {
		// Use defaults if body parsing fails
		body.Title = "Test Notification"
		body.Body = "This is a test SSE notification from the playground."
	}
	if body.Title == "" {
		body.Title = "Test Notification"
	}
	if body.Body == "" {
		body.Body = "This is a test SSE notification from the playground."
	}

	// Build and publish the SSE message
	msg := &sse.SSEMessage{
		Type:   "notification",
		UserID: playgroundID,
		Data: map[string]interface{}{
			"notification_id": uuid.New().String(),
			"title":           body.Title,
			"body":            body.Body,
			"channel":         "sse",
			"category":        body.Category,
			"status":          "sent",
			"data":            body.Data,
			"created_at":      time.Now().Format(time.RFC3339),
		},
	}

	if err := h.broadcaster.PublishMessage(msg); err != nil {
		h.logger.Error("Failed to publish SSE test message", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to publish message: " + err.Error(),
		})
	}

	h.logger.Info("SSE test message sent",
		zap.String("playground_id", playgroundID),
		zap.String("title", body.Title))

	return c.JSON(fiber.Map{
		"status":  "sent",
		"user_id": playgroundID,
	})
}
