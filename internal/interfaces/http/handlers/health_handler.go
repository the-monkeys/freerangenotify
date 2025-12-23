package handlers

import (
	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"go.uber.org/zap"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	dbManager   *database.DatabaseManager
	redisClient *redis.Client
	logger      *zap.Logger
}

// NewHealthHandler creates a new HealthHandler
func NewHealthHandler(dbManager *database.DatabaseManager, redisClient *redis.Client, logger *zap.Logger) *HealthHandler {
	return &HealthHandler{
		dbManager:   dbManager,
		redisClient: redisClient,
		logger:      logger,
	}
}

// Check handles GET /v1/health
func (h *HealthHandler) Check(c *fiber.Ctx) error {
	status := fiber.StatusOK
	health := fiber.Map{
		"status":     "healthy",
		"timestamp":  c.Context().Time(),
		"components": fiber.Map{},
	}

	components := health["components"].(fiber.Map)

	// Check Elasticsearch
	if err := h.dbManager.Health(c.Context()); err != nil {
		status = fiber.StatusServiceUnavailable
		health["status"] = "unhealthy"
		components["elasticsearch"] = fiber.Map{
			"status": "unhealthy",
			"error":  err.Error(),
		}
	} else {
		components["elasticsearch"] = fiber.Map{"status": "healthy"}
	}

	// Check Redis
	if err := h.redisClient.Ping(c.Context()).Err(); err != nil {
		status = fiber.StatusServiceUnavailable
		health["status"] = "unhealthy"
		components["redis"] = fiber.Map{
			"status": "unhealthy",
			"error":  err.Error(),
		}
	} else {
		components["redis"] = fiber.Map{"status": "healthy"}
	}

	return c.Status(status).JSON(health)
}
