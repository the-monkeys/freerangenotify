package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/container"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/middleware"
)

// SetupRoutes configures all application routes
func SetupRoutes(app *fiber.App, c *container.Container) {
	// API v1 group
	v1 := app.Group("/v1")

	// Public routes (no authentication required)
	setupPublicRoutes(v1, c)

	// Protected routes (require API key authentication)
	setupProtectedRoutes(v1, c)

	// Admin routes
	setupAdminRoutes(v1, c)
}

// setupPublicRoutes configures public routes
func setupPublicRoutes(v1 fiber.Router, c *container.Container) {
	// Application management (typically used by admins, but not protected by API key in this example)
	// In production, you might want to add admin authentication here
	apps := v1.Group("/apps")
	apps.Post("/", c.ApplicationHandler.Create)
	apps.Get("/:id", c.ApplicationHandler.GetByID)
	apps.Put("/:id", c.ApplicationHandler.Update)
	apps.Delete("/:id", c.ApplicationHandler.Delete)
	apps.Get("/", c.ApplicationHandler.List)
	apps.Post("/:id/regenerate-key", c.ApplicationHandler.RegenerateAPIKey)
	apps.Put("/:id/settings", c.ApplicationHandler.UpdateSettings)
	apps.Get("/:id/settings", c.ApplicationHandler.GetSettings)

	// Health check
	v1.Get("/health", c.HealthHandler.Check)
}

// setupProtectedRoutes configures routes that require API key authentication
func setupProtectedRoutes(v1 fiber.Router, c *container.Container) {
	// Create common middleware
	auth := middleware.APIKeyAuth(c.ApplicationService, c.Logger)

	// User management routes
	users := v1.Group("/users")
	users.Use(auth)
	users.Post("/", c.UserHandler.Create)
	users.Get("/:id", c.UserHandler.GetByID)
	users.Put("/:id", c.UserHandler.Update)
	users.Delete("/:id", c.UserHandler.Delete)
	users.Get("/", c.UserHandler.List)

	// Device management
	users.Post("/:id/devices", c.UserHandler.AddDevice)
	users.Get("/:id/devices", c.UserHandler.GetDevices)
	users.Delete("/:id/devices/:device_id", c.UserHandler.RemoveDevice)

	// Preferences management
	users.Put("/:id/preferences", c.UserHandler.UpdatePreferences)
	users.Get("/:id/preferences", c.UserHandler.GetPreferences)

	// Presence management
	presence := v1.Group("/presence")
	presence.Use(auth)
	presence.Post("/check-in", c.PresenceHandler.CheckIn)

	// Notification routes
	notifications := v1.Group("/notifications")
	notifications.Use(auth)
	notifications.Post("/", c.NotificationHandler.Send)
	notifications.Post("/bulk", c.NotificationHandler.SendBulk)
	notifications.Post("/batch", c.NotificationHandler.SendBatch)
	notifications.Get("/", c.NotificationHandler.List)
	notifications.Get("/:id", c.NotificationHandler.Get)
	notifications.Put("/:id/status", c.NotificationHandler.UpdateStatus)
	notifications.Delete("/batch", c.NotificationHandler.CancelBatch)
	notifications.Delete("/:id", c.NotificationHandler.Cancel)
	notifications.Post("/:id/retry", c.NotificationHandler.Retry)

	// Template routes
	templates := v1.Group("/templates")
	templates.Use(auth)
	templates.Post("/", c.TemplateHandler.CreateTemplate)
	templates.Get("/", c.TemplateHandler.ListTemplates)
	templates.Get("/:id", c.TemplateHandler.GetTemplate)
	templates.Put("/:id", c.TemplateHandler.UpdateTemplate)
	templates.Delete("/:id", c.TemplateHandler.DeleteTemplate)
	templates.Post("/:id/render", c.TemplateHandler.RenderTemplate)
	templates.Post("/:app_id/:name/versions", c.TemplateHandler.CreateTemplateVersion)
	templates.Get("/:app_id/:name/versions", c.TemplateHandler.GetTemplateVersions)
}

// setupAdminRoutes configures administrative routes
func setupAdminRoutes(v1 fiber.Router, c *container.Container) {
	admin := v1.Group("/admin")

	// Queue management
	queues := admin.Group("/queues")
	queues.Get("/stats", c.AdminHandler.GetQueueStats)
	queues.Get("/dlq", c.AdminHandler.ListDLQ)
	queues.Post("/dlq/replay", c.AdminHandler.ReplayDLQ)
}
