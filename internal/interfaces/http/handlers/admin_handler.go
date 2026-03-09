package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"go.uber.org/zap"
)

// AdminHandler handles administrative HTTP requests
type AdminHandler struct {
	queue           queue.Queue
	providerManager *providers.Manager
	appRepo         application.Repository
	notifRepo       notification.Repository
	logger          *zap.Logger
}

// NewAdminHandler creates a new AdminHandler
func NewAdminHandler(q queue.Queue, pm *providers.Manager, appRepo application.Repository, notifRepo notification.Repository, logger *zap.Logger) *AdminHandler {
	return &AdminHandler{
		queue:           q,
		providerManager: pm,
		appRepo:         appRepo,
		notifRepo:       notifRepo,
		logger:          logger,
	}
}

// getAdminAppIDs returns the list of app IDs owned by the authenticated admin user.
func (h *AdminHandler) getAdminAppIDs(c *fiber.Ctx) (map[string]bool, error) {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}

	apps, err := h.appRepo.List(c.Context(), application.ApplicationFilter{AdminUserID: userID})
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "failed to fetch admin apps")
	}

	appSet := make(map[string]bool, len(apps))
	for _, app := range apps {
		appSet[app.AppID] = true
	}
	return appSet, nil
}

// GetQueueStats handles GET /v1/admin/queues/stats
// @Summary Get queue statistics
// @Description Retrieve notification queue depth and processing statistics
// @Tags Admin
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/admin/queues/stats [get]
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
// @Summary List dead-letter queue items
// @Description Retrieve failed notification items from the dead-letter queue
// @Tags Admin
// @Produce json
// @Param limit query int false "Number of items to return" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/admin/queues/dlq [get]
func (h *AdminHandler) ListDLQ(c *fiber.Ctx) error {
	limitStr := c.Query("limit", "10")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 10
	}

	// Get admin's app IDs for filtering
	adminApps, err := h.getAdminAppIDs(c)
	if err != nil {
		return err
	}

	// Fetch more items than requested so we can filter and still return enough
	fetchLimit := limit * 5
	if fetchLimit < 50 {
		fetchLimit = 50
	}

	items, err := h.queue.ListDLQ(c.Context(), fetchLimit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list DLQ items",
		})
	}

	// Filter DLQ items by admin's apps
	filtered := make([]queue.DLQItem, 0, limit)
	for _, item := range items {
		if len(filtered) >= limit {
			break
		}

		// If the DLQ item has an AppID, check ownership
		if item.AppID != "" {
			if adminApps[item.AppID] {
				filtered = append(filtered, item)
			}
			continue
		}

		// Legacy items without AppID: look up the notification to get its app_id
		notif, lookupErr := h.notifRepo.GetByID(c.Context(), item.NotificationID)
		if lookupErr != nil || notif == nil {
			continue // Skip items we can't verify ownership for
		}
		if adminApps[notif.AppID] {
			filtered = append(filtered, item)
		}
	}

	return c.JSON(fiber.Map{
		"items": filtered,
	})
}

// ReplayDLQ handles POST /v1/admin/queues/dlq/replay
// @Summary Replay dead-letter queue items
// @Description Re-enqueue failed notifications from the dead-letter queue for reprocessing
// @Tags Admin
// @Produce json
// @Param limit query int false "Maximum items to replay" default(10)
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/admin/queues/dlq/replay [post]
func (h *AdminHandler) ReplayDLQ(c *fiber.Ctx) error {
	limitStr := c.Query("limit", "10")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 10
	}

	// Get admin's app IDs for filtering
	adminApps, err := h.getAdminAppIDs(c)
	if err != nil {
		return err
	}

	count, err := h.queue.ReplayDLQForApps(c.Context(), limit, adminApps)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to replay DLQ items",
		})
	}

	return c.JSON(fiber.Map{
		"replayed_count": count,
	})
}

// GetProviderHealth handles GET /v1/admin/providers/health
// @Summary Get provider health status
// @Description Retrieve the health status of all configured notification delivery providers
// @Tags Admin
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/admin/providers/health [get]
func (h *AdminHandler) GetProviderHealth(c *fiber.Ctx) error {
	if h.providerManager == nil {
		return c.JSON(fiber.Map{
			"providers": fiber.Map{},
			"message":   "Provider manager not available (API server only)",
		})
	}

	health := h.providerManager.HealthStatus()
	return c.JSON(fiber.Map{
		"providers": health,
	})
}
