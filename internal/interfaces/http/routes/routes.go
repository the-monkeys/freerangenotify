package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/container"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/middleware"
)

// SetupRoutes configures all application routes
func SetupRoutes(app *fiber.App, c *container.Container) {
	// API v1 group
	v1 := app.Group("/v1")

	// ── Phase 2: Audit middleware (feature-gated) ──
	// Applied globally so all state-changing requests are captured.
	if c.AuditService != nil {
		v1.Use(middleware.AuditMiddleware(c.AuditService, c.Logger))
	}

	// Public routes (no authentication required)
	setupPublicRoutes(v1, c)

	// Protected routes (require API key authentication)
	setupProtectedRoutes(v1, c)

	// Admin routes
	setupAdminRoutes(v1, c)
}

// setupPublicRoutes configures public routes
func setupPublicRoutes(v1 fiber.Router, c *container.Container) {
	// Authentication routes (public)
	auth := v1.Group("/auth")
	auth.Post("/register", c.AuthHandler.Register)
	auth.Post("/login", c.AuthHandler.Login)
	auth.Post("/refresh", c.AuthHandler.RefreshToken)
	auth.Post("/forgot-password", c.AuthHandler.ForgotPassword)
	auth.Post("/reset-password", c.AuthHandler.ResetPassword)

	// SSO routes
	if c.OIDCProvider != nil && c.OAuth2Config != nil {
		auth.Get("/sso/login", c.AuthHandler.HandleSSOLogin(c.OAuth2Config))

		frontendURL := c.Config.OIDC.FrontendURL
		if frontendURL == "" {
			frontendURL = "http://localhost:3000"
		}

		auth.Get("/sso/callback", c.AuthHandler.HandleSSOCallback(c.OAuth2Config, c.OIDCVerifier, frontendURL))
	}

	// Health check
	v1.Get("/health", c.HealthHandler.Check)

	// SSE endpoint
	v1.Get("/sse", c.SSEHandler.Connect)

	// Webhook playground — public receive & read endpoints
	v1.Post("/playground/:id", c.PlaygroundHandler.ReceiveWebhook)
	v1.Get("/playground/:id", c.PlaygroundHandler.GetPayloads)
}

// setupProtectedRoutes configures routes that require API key authentication
func setupProtectedRoutes(v1 fiber.Router, c *container.Container) {
	// Create common middleware — with optional environment service for multi-env support
	var authOpts []middleware.APIKeyAuthOption
	if c.EnvironmentService != nil {
		authOpts = append(authOpts, middleware.WithEnvironmentService(c.EnvironmentService))
	}
	auth := middleware.APIKeyAuth(c.ApplicationService, c.Logger, authOpts...)

	// User management routes
	users := v1.Group("/users")
	users.Use(auth)
	users.Post("/", c.UserHandler.Create)
	users.Post("/bulk", c.UserHandler.BulkCreate)
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
	// Phase 5: subscriber hash for SSE HMAC authentication
	users.Get("/:id/subscriber-hash", c.UserHandler.GetSubscriberHash)

	// Presence management
	presence := v1.Group("/presence")
	presence.Use(auth)
	presence.Post("/check-in", c.PresenceHandler.CheckIn)

	// Quick-send (simplified notification endpoint)
	v1.Post("/quick-send", auth, c.QuickSendHandler.Send)

	// Notification routes
	notifications := v1.Group("/notifications")
	notifications.Use(auth)
	notifications.Post("/", c.NotificationHandler.Send)
	notifications.Post("/bulk", c.NotificationHandler.SendBulk)
	notifications.Post("/broadcast", c.NotificationHandler.Broadcast)
	notifications.Post("/batch", c.NotificationHandler.SendBatch)
	notifications.Get("/", c.NotificationHandler.List)
	notifications.Get("/unread/count", c.NotificationHandler.GetUnreadCount)
	notifications.Get("/unread", c.NotificationHandler.ListUnread)
	notifications.Post("/read", c.NotificationHandler.MarkRead)
	// Phase 5: mark-all-read & bulk archive (before /:id to avoid param collision)
	notifications.Post("/read-all", c.NotificationHandler.MarkAllRead)
	notifications.Patch("/bulk/archive", c.NotificationHandler.BulkArchive)
	notifications.Get("/:id", c.NotificationHandler.Get)
	notifications.Put("/:id/status", c.NotificationHandler.UpdateStatus)
	notifications.Delete("/batch", c.NotificationHandler.CancelBatch)
	notifications.Delete("/:id", c.NotificationHandler.Cancel)
	notifications.Post("/:id/retry", c.NotificationHandler.Retry)
	// Phase 5: snooze/unsnooze
	notifications.Post("/:id/snooze", c.NotificationHandler.Snooze)
	notifications.Post("/:id/unsnooze", c.NotificationHandler.Unsnooze)

	// Template routes
	templates := v1.Group("/templates")
	templates.Use(auth)
	templates.Get("/library", c.TemplateHandler.GetLibrary)
	templates.Post("/library/:name/clone", c.TemplateHandler.CloneFromLibrary)
	templates.Post("/", c.TemplateHandler.CreateTemplate)
	templates.Get("/", c.TemplateHandler.ListTemplates)
	templates.Get("/:id", c.TemplateHandler.GetTemplate)
	templates.Put("/:id", c.TemplateHandler.UpdateTemplate)
	templates.Delete("/:id", c.TemplateHandler.DeleteTemplate)
	templates.Post("/:id/render", c.TemplateHandler.RenderTemplate)
	templates.Post("/:id/rollback", c.TemplateHandler.RollbackTemplate)
	templates.Get("/:id/diff", c.TemplateHandler.DiffTemplate)
	templates.Post("/:id/test", c.TemplateHandler.SendTest)
	// Phase 6: Content Controls
	templates.Get("/:id/controls", c.TemplateHandler.GetControls)
	templates.Put("/:id/controls", c.TemplateHandler.UpdateControls)
	templates.Post("/:app_id/:name/versions", c.TemplateHandler.CreateTemplateVersion)
	templates.Get("/:app_id/:name/versions", c.TemplateHandler.GetTemplateVersions)
	// Phase 4: Get single template version by number
	templates.Get("/:app_id/:name/versions/:version", c.TemplateHandler.GetTemplateVersion)

	// ── Phase 1: Workflow routes (feature-gated) ──
	if c.WorkflowHandler != nil {
		workflows := v1.Group("/workflows")
		workflows.Use(auth)
		workflows.Post("/", c.WorkflowHandler.Create)
		workflows.Get("/", c.WorkflowHandler.List)
		workflows.Get("/executions", c.WorkflowHandler.ListExecutions)
		workflows.Get("/executions/:id", c.WorkflowHandler.GetExecution)
		workflows.Post("/executions/:id/cancel", c.WorkflowHandler.CancelExecution)
		workflows.Post("/trigger", c.WorkflowHandler.Trigger)
		workflows.Get("/:id", c.WorkflowHandler.Get)
		workflows.Put("/:id", c.WorkflowHandler.Update)
		workflows.Delete("/:id", c.WorkflowHandler.Delete)
	}

	// ── Phase 1: Digest rules routes (feature-gated) ──
	if c.DigestHandler != nil {
		digestRules := v1.Group("/digest-rules")
		digestRules.Use(auth)
		digestRules.Post("/", c.DigestHandler.Create)
		digestRules.Get("/", c.DigestHandler.List)
		digestRules.Get("/:id", c.DigestHandler.Get)
		digestRules.Put("/:id", c.DigestHandler.Update)
		digestRules.Delete("/:id", c.DigestHandler.Delete)
	}

	// ── Phase 2: Topic routes (feature-gated) ──
	if c.TopicHandler != nil {
		topics := v1.Group("/topics")
		topics.Use(auth)
		topics.Post("/", c.TopicHandler.Create)
		topics.Get("/", c.TopicHandler.List)
		topics.Get("/key/:key", c.TopicHandler.GetByKey)
		topics.Get("/:id", c.TopicHandler.Get)
		topics.Put("/:id", c.TopicHandler.Update)
		topics.Delete("/:id", c.TopicHandler.Delete)
		topics.Post("/:id/subscribers", c.TopicHandler.AddSubscribers)
		topics.Delete("/:id/subscribers", c.TopicHandler.RemoveSubscribers)
		topics.Get("/:id/subscribers", c.TopicHandler.GetSubscribers)
	}
}

// setupAdminRoutes configures administrative routes
func setupAdminRoutes(v1 fiber.Router, c *container.Container) {
	admin := v1.Group("/admin")

	// JWT-protected admin routes
	jwtAuth := middleware.JWTAuth(c.AuthService, c.Logger)
	adminAuth := admin.Group("")
	adminAuth.Use(jwtAuth)

	// Auth-protected routes
	adminAuth.Get("/me", c.AuthHandler.GetCurrentUser)
	adminAuth.Post("/logout", c.AuthHandler.Logout)
	adminAuth.Post("/change-password", c.AuthHandler.ChangePassword)

	// Application management routes (JWT protected for admin dashboard)
	apps := v1.Group("/apps")
	apps.Use(jwtAuth)
	apps.Post("/", c.ApplicationHandler.Create)
	apps.Get("/", c.ApplicationHandler.List)
	apps.Get("/:id", c.ApplicationHandler.GetByID)
	apps.Put("/:id", c.ApplicationHandler.Update)
	apps.Delete("/:id", c.ApplicationHandler.Delete)
	apps.Post("/:id/regenerate-key", c.ApplicationHandler.RegenerateAPIKey)
	apps.Put("/:id/settings", c.ApplicationHandler.UpdateSettings)
	apps.Get("/:id/settings", c.ApplicationHandler.GetSettings)

	// Phase 3: Custom Provider Management
	apps.Post("/:id/providers", c.CustomProviderHandler.Register)
	apps.Get("/:id/providers", c.CustomProviderHandler.List)
	apps.Delete("/:id/providers/:provider_id", c.CustomProviderHandler.Remove)

	// Phase 6: Multi-Environment Management (feature-gated)
	if c.EnvironmentHandler != nil {
		apps.Post("/:id/environments", c.EnvironmentHandler.Create)
		apps.Get("/:id/environments", c.EnvironmentHandler.List)
		apps.Post("/:id/environments/promote", c.EnvironmentHandler.Promote)
		apps.Get("/:id/environments/:envId", c.EnvironmentHandler.Get)
		apps.Delete("/:id/environments/:envId", c.EnvironmentHandler.Delete)
	}

	// ── Phase 2: RBAC enforcement on app routes (feature-gated) ──
	// RequirePermission checks membership-based permissions for JWT-authenticated users.
	// If RBAC is disabled, the routes above remain unguarded (any authenticated admin user).
	if c.MembershipRepo != nil {
		apps.Put("/:id/settings", extractAppIDFromParam("id"),
			middleware.RequirePermission(auth.PermManageApp, c.MembershipRepo, c.Logger),
			c.ApplicationHandler.UpdateSettings)
		apps.Delete("/:id", extractAppIDFromParam("id"),
			middleware.RequirePermission(auth.PermManageApp, c.MembershipRepo, c.Logger),
			c.ApplicationHandler.Delete)
	}

	// Queue management
	queues := admin.Group("/queues")
	queues.Get("/stats", c.AdminHandler.GetQueueStats)
	queues.Get("/dlq", c.AdminHandler.ListDLQ)
	queues.Post("/dlq/replay", c.AdminHandler.ReplayDLQ)

	// Provider health
	admin.Get("/providers/health", c.AdminHandler.GetProviderHealth)

	// Webhook playground
	adminAuth.Post("/playground/webhook", c.PlaygroundHandler.CreatePlayground)

	// Analytics
	adminAuth.Get("/analytics/summary", c.AnalyticsHandler.GetSummary)

	// Activity feed (real-time SSE stream of notification events)
	adminAuth.Get("/activity-feed", c.SSEHandler.AdminActivityFeed)

	// ── Phase 2: Audit log routes (feature-gated) ──
	if c.AuditHandler != nil {
		auditGroup := admin.Group("/audit")
		auditGroup.Use(jwtAuth)
		if c.MembershipRepo != nil {
			auditGroup.Use(middleware.RequirePermission(auth.PermViewAudit, c.MembershipRepo, c.Logger))
		}
		auditGroup.Get("/", c.AuditHandler.List)
		auditGroup.Get("/:id", c.AuditHandler.Get)
	}

	// ── Phase 2: Team management routes (feature-gated) ──
	if c.TeamHandler != nil {
		team := v1.Group("/apps/:app_id/team")
		team.Use(jwtAuth)
		if c.MembershipRepo != nil {
			team.Use(extractAppIDFromParam("app_id"),
				middleware.RequirePermission(auth.PermManageMembers, c.MembershipRepo, c.Logger))
		}
		team.Post("/", c.TeamHandler.InviteMember)
		team.Get("/", c.TeamHandler.ListMembers)
		team.Put("/:membership_id", c.TeamHandler.UpdateRole)
		team.Delete("/:membership_id", c.TeamHandler.RemoveMember)
	}

	// ── Phase 2: Audit middleware (feature-gated, applied to protected routes) ──
	if c.AuditService != nil {
		// Applied at the app level for state-changing requests
	}
}

// extractAppIDFromParam returns a middleware that reads the named URL parameter
// and stores it in c.Locals("app_id") so that RequirePermission can use it.
func extractAppIDFromParam(param string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if id := c.Params(param); id != "" {
			c.Locals("app_id", id)
		}
		return c.Next()
	}
}
