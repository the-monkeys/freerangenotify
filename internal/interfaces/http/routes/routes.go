package routes

import (
	"time"

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

	// Ops routes (feature-gated, hosted-only)
	setupOpsRoutes(v1, c)
}

// setupPublicRoutes configures public routes
func setupPublicRoutes(v1 fiber.Router, c *container.Container) {
	// Authentication routes (public)
	auth := v1.Group("/auth")
	auth.Post("/register", c.AuthHandler.Register)
	auth.Post("/verify-otp", c.AuthHandler.VerifyOTP)
	auth.Post("/resend-otp", c.AuthHandler.ResendOTP)
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

	// Public stats (aggregate only, no auth)
	if c.PublicHandler != nil {
		v1.Get("/public/stats", c.PublicHandler.GetStats)
	}

	// SSE endpoint
	v1.Get("/sse", c.SSEHandler.Connect)

	// Webhook playground — public receive & read endpoints
	v1.Post("/playground/:id", c.PlaygroundHandler.ReceiveWebhook)
	v1.Get("/playground/:id", c.PlaygroundHandler.GetPayloads)

	// Payment webhooks (public, verified via signature)
	if c.PaymentHandler != nil {
		v1.Post("/billing/webhook", c.PaymentHandler.HandleWebhook)
	}

	// Meta WhatsApp webhooks (public, verified via X-Hub-Signature-256 inside handler)
	if c.MetaWebhookHandler != nil {
		metaWH := v1.Group("/webhooks/meta")
		metaWH.Get("/whatsapp", c.MetaWebhookHandler.VerifyWebhook)
		metaWH.Post("/whatsapp", c.MetaWebhookHandler.HandleWebhook)
	}
}

// setupProtectedRoutes configures routes that require API key authentication
func setupProtectedRoutes(v1 fiber.Router, c *container.Container) {
	// Create common middleware — with optional auth service for dashboard JWT extraction
	var authOpts []middleware.APIKeyAuthOption
	if c.EnvironmentService != nil {
		authOpts = append(authOpts, middleware.WithEnvironmentService(c.EnvironmentService))
	}
	if c.AuthService != nil {
		authOpts = append(authOpts, middleware.WithAuthService(c.AuthService))
	}
	apiAuth := middleware.APIKeyAuth(c.ApplicationService, c.Logger, authOpts...)
	licenseCheck := middleware.LicenseCheck(c.LicensingChecker, c.Logger)

	// DashboardRBAC restricts viewers to read-only when they access API-key
	// routes through the dashboard (JWT + X-API-Key). Pure SDK calls (API key
	// only) are unaffected.
	var rbac fiber.Handler
	if c.MembershipRepo != nil {
		rbac = middleware.DashboardRBAC(c.MembershipRepo, c.AppRepo, c.Logger)
	}

	// applyAuth adds the API key middleware and optional RBAC to a route group
	applyAuth := func(group fiber.Router) {
		group.Use(apiAuth)
		if rbac != nil {
			group.Use(rbac)
		}
	}

	// User management routes
	users := v1.Group("/users")
	applyAuth(users)
	users.Post("/", c.UserHandler.Create)
	users.Post("/bulk", c.UserHandler.BulkCreate)
	users.Get("/by-external-id/:external_id", c.UserHandler.GetByExternalID)
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

	// SSE token endpoint (secure — generates short-lived tokens for SSE connections)
	v1.Post("/sse/tokens", apiAuth, c.SSEHandler.CreateToken)

	// License status endpoint (API key protected)
	v1.Get("/license/status", apiAuth, c.LicensingHandler.GetStatus)

	// Presence management
	presence := v1.Group("/presence")
	presence.Use(apiAuth)
	presence.Post("/check-in", c.PresenceHandler.CheckIn)

	// Quick-send (simplified notification endpoint)
	v1.Post("/quick-send", apiAuth, licenseCheck, c.QuickSendHandler.Send)

	// Media upload (for WhatsApp file attachments)
	v1.Post("/media/upload", apiAuth, c.MediaHandler.Upload)

	// Notification routes
	notifications := v1.Group("/notifications")
	applyAuth(notifications)
	notifications.Post("/", licenseCheck, c.NotificationHandler.Send)
	notifications.Post("/bulk", licenseCheck, c.NotificationHandler.SendBulk)
	notifications.Post("/broadcast", licenseCheck, c.NotificationHandler.Broadcast)
	notifications.Post("/batch", licenseCheck, c.NotificationHandler.SendBatch)
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
	applyAuth(templates)
	templates.Get("/library", c.TemplateHandler.GetLibrary)
	templates.Post("/library/:name/render", c.TemplateHandler.RenderLibraryTemplate)
	templates.Post("/library/:name/clone", c.TemplateHandler.CloneFromLibrary)
	templates.Post("/seed", c.TemplateHandler.SeedTemplates)
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
		applyAuth(workflows)
		workflows.Post("/", c.WorkflowHandler.Create)
		workflows.Get("/", c.WorkflowHandler.List)
		workflows.Get("/executions", c.WorkflowHandler.ListExecutions)
		workflows.Get("/executions/:id", c.WorkflowHandler.GetExecution)
		workflows.Post("/executions/:id/cancel", c.WorkflowHandler.CancelExecution)
		workflows.Post("/trigger", licenseCheck, c.WorkflowHandler.Trigger)
		workflows.Post("/trigger-by-topic", licenseCheck, c.WorkflowHandler.TriggerByTopic)
		// Phase 6: Schedules (before /:id to avoid collision)
		if c.ScheduleHandler != nil {
			workflows.Post("/schedules", licenseCheck, c.ScheduleHandler.Create)
			workflows.Get("/schedules", c.ScheduleHandler.List)
			workflows.Get("/schedules/:id", c.ScheduleHandler.Get)
			workflows.Put("/schedules/:id", licenseCheck, c.ScheduleHandler.Update)
			workflows.Delete("/schedules/:id", c.ScheduleHandler.Delete)
		}
		workflows.Get("/:id", c.WorkflowHandler.Get)
		workflows.Put("/:id", c.WorkflowHandler.Update)
		workflows.Delete("/:id", c.WorkflowHandler.Delete)
	}

	// ── Phase 7: Inbound Webhooks (feature-gated with Workflow) ──
	if c.InboundWebhookHandler != nil {
		webhooks := v1.Group("/webhooks")
		applyAuth(webhooks)
		webhooks.Post("/inbound", licenseCheck, c.InboundWebhookHandler.Receive)
	}

	// ── WhatsApp Template Management (feature-gated) ──
	if c.WhatsAppTemplateHandler != nil {
		waTpl := v1.Group("/whatsapp/templates")
		applyAuth(waTpl)
		waTpl.Post("/", c.WhatsAppTemplateHandler.CreateTemplate)
		waTpl.Get("/", c.WhatsAppTemplateHandler.ListTemplates)
		waTpl.Get("/:name", c.WhatsAppTemplateHandler.GetTemplate)
		waTpl.Delete("/:name", c.WhatsAppTemplateHandler.DeleteTemplate)
		waTpl.Post("/:name/sync", c.WhatsAppTemplateHandler.SyncTemplate)
	}

	// ── Twilio Content Template Management ──
	if c.TwilioTemplateHandler != nil {
		twilioTpl := v1.Group("/twilio/templates")
		applyAuth(twilioTpl)
		twilioTpl.Post("/", c.TwilioTemplateHandler.CreateTemplate)
		twilioTpl.Get("/", c.TwilioTemplateHandler.ListTemplates)
		twilioTpl.Get("/:content_sid", c.TwilioTemplateHandler.GetTemplate)
		twilioTpl.Put("/:content_sid", c.TwilioTemplateHandler.UpdateTemplate)
		twilioTpl.Delete("/:content_sid", c.TwilioTemplateHandler.DeleteTemplate)
		twilioTpl.Post("/:content_sid/approve", c.TwilioTemplateHandler.SubmitApproval)
		twilioTpl.Get("/:content_sid/approval", c.TwilioTemplateHandler.GetApprovalStatus)
		twilioTpl.Post("/:content_sid/sync", c.TwilioTemplateHandler.SyncTemplate)
		twilioTpl.Post("/:content_sid/preview", c.TwilioTemplateHandler.PreviewTemplate)
	}

	// ── WhatsApp Conversation Inbox (feature-gated) ──
	if c.WhatsAppConversationHandler != nil {
		waConv := v1.Group("/whatsapp/conversations")
		applyAuth(waConv)
		waConv.Get("/", c.WhatsAppConversationHandler.ListConversations)
		waConv.Get("/:contact_id/messages", c.WhatsAppConversationHandler.GetMessages)
		waConv.Post("/:contact_id/reply", licenseCheck, c.WhatsAppConversationHandler.Reply)
		waConv.Post("/:contact_id/read", c.WhatsAppConversationHandler.MarkRead)
	}

	// ── Phase 1: Digest rules routes (feature-gated) ──
	if c.DigestHandler != nil {
		digestRules := v1.Group("/digest-rules")
		applyAuth(digestRules)
		digestRules.Post("/", c.DigestHandler.Create)
		digestRules.Get("/", c.DigestHandler.List)
		digestRules.Get("/:id", c.DigestHandler.Get)
		digestRules.Put("/:id", c.DigestHandler.Update)
		digestRules.Delete("/:id", c.DigestHandler.Delete)
	}

	// ── Phase 2: Topic routes (feature-gated) ──
	if c.TopicHandler != nil {
		topics := v1.Group("/topics")
		applyAuth(topics)
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

	// Billing routes (JWT-protected, user-facing)
	billing := v1.Group("/billing")
	billing.Use(jwtAuth)
	billing.Get("/usage", c.BillingHandler.GetUsage)
	billing.Get("/subscription", c.BillingHandler.GetSubscription)
	billing.Post("/accept-trial", c.BillingHandler.AcceptTrial)
	billing.Get("/usage/breakdown", c.BillingHandler.GetUsageBreakdown)
	billing.Get("/rates", c.BillingHandler.GetRates)

	if c.PaymentHandler != nil {
		billing.Post("/checkout", c.PaymentHandler.CreateOrder)
		billing.Post("/verify-payment", c.PaymentHandler.VerifyPayment)
	}

	// Auth-protected routes
	adminAuth.Get("/me", c.AuthHandler.GetCurrentUser)
	adminAuth.Delete("/me", c.AuthHandler.DeleteOwnAccount)
	adminAuth.Post("/logout", c.AuthHandler.Logout)
	adminAuth.Post("/change-password", c.AuthHandler.ChangePassword)

	// Phone verification
	adminAuth.Post("/phone/send-otp", c.AuthHandler.SendPhoneOTP)
	adminAuth.Post("/phone/verify-otp", c.AuthHandler.VerifyPhoneOTP)

	// Tenant/organization management (C1)
	if c.TenantHandler != nil {
		tenants := v1.Group("/tenants")
		tenants.Use(jwtAuth)
		tenants.Post("/", c.TenantHandler.Create)
		tenants.Get("/", c.TenantHandler.List)
		tenants.Get("/:id", c.TenantHandler.GetByID)
		tenants.Put("/:id", c.TenantHandler.Update)
		tenants.Delete("/:id", c.TenantHandler.Delete)
		tenants.Get("/:id/members", c.TenantHandler.ListMembers)
		tenants.Post("/:id/members", c.TenantHandler.InviteMember)
		tenants.Put("/:id/members/:memberId", c.TenantHandler.UpdateMemberRole)
		tenants.Delete("/:id/members/:memberId", c.TenantHandler.RemoveMember)
		tenants.Get("/:id/billing", c.TenantHandler.GetBilling)
		tenants.Post("/:id/billing/checkout", c.TenantHandler.Checkout)
	}

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
	apps.Post("/:id/providers/:provider_id/test", c.CustomProviderHandler.Test)
	apps.Post("/:id/providers/:provider_id/rotate", c.CustomProviderHandler.RotateSigningKey)
	apps.Delete("/:id/providers/:provider_id", c.CustomProviderHandler.Remove)

	// WhatsApp Meta Embedded Signup & Connection Management (feature-gated)
	if c.WhatsAppAdminHandler != nil {
		waAdmin := v1.Group("/admin/whatsapp")
		waAdmin.Use(jwtAuth)
		waAdmin.Get("/:app_id/status", c.WhatsAppAdminHandler.GetStatus)
		waAdmin.Post("/connect", c.WhatsAppAdminHandler.Connect)
		waAdmin.Post("/manual-connect", c.WhatsAppAdminHandler.ManualConnect)
		waAdmin.Post("/:app_id/disconnect", c.WhatsAppAdminHandler.Disconnect)
		waAdmin.Post("/:app_id/subscribe-webhooks", c.WhatsAppAdminHandler.SubscribeWebhooks)
	}

	// Phase 6: Multi-Environment Management (feature-gated)
	if c.EnvironmentHandler != nil {
		apps.Post("/:id/environments", c.EnvironmentHandler.Create)
		apps.Get("/:id/environments", c.EnvironmentHandler.List)
		apps.Post("/:id/environments/promote", c.EnvironmentHandler.Promote)
		apps.Get("/:id/environments/:envId", c.EnvironmentHandler.Get)
		apps.Delete("/:id/environments/:envId", c.EnvironmentHandler.Delete)
	}

	// Cross-app resource linking
	if c.ImportHandler != nil {
		apps.Post("/:id/import", c.ImportHandler.Import)
		apps.Get("/:id/links", c.ImportHandler.ListLinks)
		apps.Delete("/:id/links", c.ImportHandler.UnlinkAll)
		apps.Delete("/:id/links/:link_id", c.ImportHandler.Unlink)
	}

	// RBAC for app routes is enforced inside ApplicationHandler.authorizeAppAccess()
	// which checks ownership and team membership with role-based guards per endpoint.

	// Queue management (JWT-protected)
	queues := adminAuth.Group("/queues")
	queues.Get("/stats", c.AdminHandler.GetQueueStats)
	queues.Get("/dlq", c.AdminHandler.ListDLQ)
	queues.Post("/dlq/replay", c.AdminHandler.ReplayDLQ)

	// Provider health (JWT-protected)
	adminAuth.Get("/providers/health", c.AdminHandler.GetProviderHealth)

	// Licensing management
	licensing := adminAuth.Group("/licensing")
	// Backward compatibility: keep legacy write endpoints only until ops mode is enabled.
	// Once ops mode is enabled, write operations are available exclusively via /v1/ops/*.
	if c.Config == nil || !c.Config.Security.OpsEnabled {
		licensing.Post("/subscriptions", c.LicensingHandler.CreateSubscription)
		licensing.Put("/subscriptions/:id", c.LicensingHandler.UpdateSubscription)
	}
	licensing.Get("/subscriptions/:id", c.LicensingHandler.GetSubscription)
	licensing.Get("/subscriptions", c.LicensingHandler.ListSubscriptions)
	licensing.Post("/request", c.LicensingHandler.RequestLicense)
	licensing.Post("/activate", c.LicensingHandler.ActivateLicense)
	licensing.Get("/", c.LicensingHandler.GetLicense)
	licensing.Post("/validate", c.LicensingHandler.ValidateLicense)

	// Webhook playground
	adminAuth.Post("/playground/webhook", c.PlaygroundHandler.CreatePlayground)

	// SSE playground
	adminAuth.Post("/playground/sse", c.PlaygroundHandler.CreateSSEPlayground)
	adminAuth.Post("/playground/sse/:id/send", c.PlaygroundHandler.SendSSETestMessage)

	// Analytics
	adminAuth.Get("/analytics/summary", c.AnalyticsHandler.GetSummary)

	// Activity feed (real-time SSE stream of notification events)
	adminAuth.Get("/activity-feed", c.SSEHandler.AdminActivityFeed)

	// Dashboard notifications (in-app + SSE for org invites, etc.)
	if c.DashboardNotificationHandler != nil {
		adminAuth.Get("/notifications", c.DashboardNotificationHandler.List)
		adminAuth.Get("/notifications/unread-count", c.DashboardNotificationHandler.GetUnreadCount)
		adminAuth.Post("/notifications/read", c.DashboardNotificationHandler.MarkRead)
		adminAuth.Post("/sse/token", c.SSEHandler.CreateDashboardToken)
	}

	// Internal admin action for CLI subscription renewals
	if c.RenewalHandler != nil {
		adminAuth.Post("/subscriptions/:id/renew", c.RenewalHandler.AdminRenew)
	}

	// ── Phase 2: Audit log routes (feature-gated) ──
	// Audit is platform-level: JWT auth only. Handler scopes to user's apps via AdminUserID.
	if c.AuditHandler != nil {
		auditGroup := admin.Group("/audit")
		auditGroup.Use(jwtAuth)
		auditGroup.Get("/", c.AuditHandler.List)
		auditGroup.Get("/:id", c.AuditHandler.Get)
	}

	// ── Phase 2: Team management routes (feature-gated) ──
	if c.TeamHandler != nil {
		team := v1.Group("/apps/:app_id/team")
		team.Use(jwtAuth)
		if c.MembershipRepo != nil {
			team.Use(extractAppIDFromParam("app_id"),
				middleware.RequirePermission(auth.PermManageMembers, c.MembershipRepo, c.AppRepo, c.Logger))
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

// setupOpsRoutes configures privileged machine-to-machine operational routes.
// These routes are disabled by default and only enabled when explicitly configured.
func setupOpsRoutes(v1 fiber.Router, c *container.Container) {
	if !opsRoutesAvailable() {
		return
	}

	if c == nil || c.Config == nil {
		return
	}

	if !c.Config.Security.OpsEnabled {
		return
	}

	if c.Config.Licensing.DeploymentMode != "hosted" {
		return
	}

	if c.LicensingHandler == nil {
		return
	}

	ops := v1.Group("/ops")
	opsWindow := time.Duration(c.Config.Security.OpsRateLimitWindowSeconds) * time.Second
	ops.Use(middleware.OpsRateLimit(c.Limiter, c.Config.Security.OpsRateLimit, opsWindow, c.Logger))
	tolerance := time.Duration(c.Config.Security.OpsTimestampToleranceSeconds) * time.Second
	ops.Use(middleware.OpsAuth(c.Config.Security.OpsSecret, tolerance, c.Logger))

	// Backward-compatible first step: expose subscription create/update on ops plane
	// while existing admin routes remain unchanged until deprecation cutover.
	ops.Post("/subscriptions", c.LicensingHandler.CreateSubscription)
	ops.Put("/subscriptions/:id", c.LicensingHandler.UpdateSubscription)

	if c.OpsHandler != nil {
		ops.Post("/subscriptions/renew", c.OpsHandler.RenewSubscription)
		ops.Delete("/users/:user_id", c.OpsHandler.DeleteAccount)
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
