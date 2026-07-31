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

	// Public pricing (marketing site, no auth) — same data shape as
	// /v1/billing/plans + /v1/billing/rates combined, always served from
	// the active rate card in Elasticsearch.
	if c.BillingHandler != nil {
		v1.Get("/public/billing/pricing", c.BillingHandler.GetPublicPricing)
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

	// Twilio Content API approval status webhook (public; should be guarded
	// by X-Twilio-Signature middleware in production deployments).
	if c.TwilioContentStatusHandler != nil {
		v1.Post("/webhooks/twilio/content-status", c.TwilioContentStatusHandler.Handle)
	}

	// Twilio inbound WhatsApp webhook (replies, button taps, list selects).
	// Signature is verified inside the handler using the Twilio auth token.
	if c.TwilioInboundWebhookHandler != nil {
		v1.Post("/webhooks/twilio/whatsapp", c.TwilioInboundWebhookHandler.Handle)
	}

	// Click attribution redirect (public, signed payload). Intentionally
	// outside any auth group so WhatsApp recipients can tap the link.
	if c.ClickRedirectHandler != nil {
		v1.Get("/r/:sig", c.ClickRedirectHandler.Handle)
	}

	// File attachments: signed-URL download (public, signature-verified).
	// MUST stay outside any API-key middleware so off-platform consumers can
	// fetch by URL alone.
	if c.FileHandler != nil {
		v1.Get("/files/download/:id", c.FileHandler.PublicDownload)
	}

	// Business Billing: third-party connector webhooks (HMAC-verified inside handler).
	if c.BizBillingHandler != nil {
		v1.Post("/webhooks/billing/:connector_id", c.BizBillingHandler.HandleConnectorWebhook)
	}

	// Business Billing: customer portal (token in URL is the credential).
	if c.BizPortalHandler != nil {
		portal := v1.Group("/portal/:token")
		portal.Get("/", c.BizPortalHandler.Overview)
		portal.Get("/invoices", c.BizPortalHandler.ListInvoices)
		portal.Get("/invoices/:id", c.BizPortalHandler.GetInvoice)
		portal.Post("/invoices/:id/pay", c.BizPortalHandler.PayInvoice)
		portal.Get("/estimates", c.BizPortalHandler.ListEstimates)
		portal.Post("/estimates/:id/accept", c.BizPortalHandler.AcceptEstimate)
		portal.Post("/estimates/:id/reject", c.BizPortalHandler.RejectEstimate)
		portal.Post("/contracts/:id/accept", c.BizPortalHandler.AcceptContract)
		portal.Get("/statement", c.BizPortalHandler.GetStatement)
		portal.Get("/usage", c.BizPortalHandler.GetUsage)
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
	users.Put("/by-external-id/:external_id", c.UserHandler.UpdateByExternalID)
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

	// OTP-as-a-service: send, verify, and resend one-time codes via SMS,
	// WhatsApp, or email. Send/resend pass through licenseCheck because they
	// consume credits via the underlying notification pipeline; verify is a
	// pure Redis read and is exempt.
	if c.OTPHandler != nil {
		otpGrp := v1.Group("/otp", apiAuth)
		otpGrp.Post("/send", licenseCheck, c.OTPHandler.Send)
		otpGrp.Post("/verify", c.OTPHandler.Verify)
		otpGrp.Post("/resend", licenseCheck, c.OTPHandler.Resend)
	}

	// Media upload (for WhatsApp file attachments)
	v1.Post("/media/upload", apiAuth, c.MediaHandler.Upload)

	// File attachments (P0): authenticated CRUD + streaming download +
	// signed-URL minting. The public verify-and-stream endpoint lives in
	// setupPublicRoutes so signed URLs work without an API key.
	if c.FileHandler != nil {
		files := v1.Group("/files")
		applyAuth(files)
		files.Post("/", c.FileHandler.Upload)
		files.Get("/", c.FileHandler.List)
		files.Get("/:id", c.FileHandler.Get)
		files.Delete("/:id", c.FileHandler.Delete)
		files.Get("/:id/content", c.FileHandler.Content)
		files.Get("/:id/download-url", c.FileHandler.DownloadURL)
	}

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

	// ── WhatsApp Rich-Template Authoring ──
	// Typed carousel / coupon / cta_url / quick_reply / list templates.
	// Submitted to Meta automatically on Create; ApprovalState reflects Meta status.
	if c.WhatsAppRichTemplateHandler != nil {
		waRich := v1.Group("/whatsapp/rich-templates")
		applyAuth(waRich)
		waRich.Post("/", c.WhatsAppRichTemplateHandler.Create)
		waRich.Get("/", c.WhatsAppRichTemplateHandler.List)
		waRich.Get("/:id", c.WhatsAppRichTemplateHandler.Get)
		waRich.Delete("/:id", c.WhatsAppRichTemplateHandler.Delete)
		waRich.Post("/:id/sync", c.WhatsAppRichTemplateHandler.Sync)
		waRich.Post("/:id/preview", c.WhatsAppRichTemplateHandler.Preview)
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

	// ── Business Billing Module (feature-gated) ──
	if c.BizBillingHandler != nil {
		biz := v1.Group("/biz")
		applyAuth(biz)
		biz.Use(middleware.BillingModuleCheck(c.Config.Features.BizBillingEnabled, c.Logger))
		// Writes require the manage-billing permission for dashboard (JWT)
		// callers; viewers keep read-only access. Pure API-key calls are the
		// app owner and pass through.
		if c.MembershipRepo != nil {
			biz.Use(middleware.RequirePermissionForWrites(auth.PermManageBilling, c.MembershipRepo, c.AppRepo, c.Logger))
		}

		// ── Products ──
		biz.Post("/products", c.BizBillingHandler.CreateProduct)
		biz.Get("/products", c.BizBillingHandler.ListProducts)
		biz.Get("/products/:id", c.BizBillingHandler.GetProduct)
		biz.Put("/products/:id", c.BizBillingHandler.UpdateProduct)
		biz.Delete("/products/:id", c.BizBillingHandler.DeleteProduct)

		// ── Plans & Addons ──
		biz.Post("/plans", c.BizBillingHandler.CreatePlan)
		biz.Get("/plans", c.BizBillingHandler.ListPlans)
		biz.Get("/plans/:id", c.BizBillingHandler.GetPlan)
		biz.Put("/plans/:id", c.BizBillingHandler.UpdatePlan)
		biz.Delete("/plans/:id", c.BizBillingHandler.DeletePlan)
		biz.Post("/plans/:id/addons", c.BizBillingHandler.AddPlanAddon)
		biz.Get("/plans/:id/addons", c.BizBillingHandler.ListPlanAddons)
		biz.Delete("/plans/:id/addons/:addon_id", c.BizBillingHandler.RemovePlanAddon)

		// ── Config ──
		biz.Get("/config/:config_type", c.BizBillingHandler.GetConfig)
		biz.Put("/config/:config_type", c.BizBillingHandler.UpdateConfig)

		// ── Subscriptions ──
		biz.Post("/subscriptions", c.BizBillingHandler.CreateSubscription)
		biz.Get("/subscriptions", c.BizBillingHandler.ListSubscriptions)
		biz.Get("/subscriptions/:id", c.BizBillingHandler.GetSubscription)
		biz.Put("/subscriptions/:id", c.BizBillingHandler.UpdateSubscription)
		biz.Post("/subscriptions/:id/change-plan", c.BizBillingHandler.ChangePlan)
		biz.Post("/subscriptions/:id/cancel", c.BizBillingHandler.CancelSubscription)
		biz.Post("/subscriptions/:id/pause", c.BizBillingHandler.PauseSubscription)
		biz.Post("/subscriptions/:id/resume", c.BizBillingHandler.ResumeSubscription)
		biz.Post("/subscriptions/:id/reactivate", c.BizBillingHandler.ReactivateSubscription)
		biz.Post("/subscriptions/:id/non-renewing", c.BizBillingHandler.SetNonRenewing)

		// ── Invoices ──
		biz.Post("/invoices", c.BizBillingHandler.CreateInvoice)
		biz.Get("/invoices", c.BizBillingHandler.ListInvoices)
		biz.Get("/invoices/:id", c.BizBillingHandler.GetInvoice)
		biz.Put("/invoices/:id", c.BizBillingHandler.UpdateDraftInvoice)
		biz.Post("/invoices/:id/issue", c.BizBillingHandler.IssueInvoice)
		biz.Post("/invoices/:id/send", c.BizBillingHandler.SendInvoice)
		biz.Post("/invoices/:id/void", c.BizBillingHandler.VoidInvoice)
		biz.Post("/invoices/:id/write-off", c.BizBillingHandler.WriteOffInvoice)
		biz.Post("/invoices/:id/apply-coupon", c.BizBillingHandler.ApplyCoupon)
		biz.Post("/invoices/:id/apply-credit", c.BizBillingHandler.ApplyCredit)
		biz.Post("/invoices/:id/late-fee", c.BizBillingHandler.AddLateFee)

		// ── Payments ──
		biz.Post("/invoices/:id/payments", c.BizBillingHandler.RecordPayment)
		biz.Get("/payments", c.BizBillingHandler.ListPayments)
		biz.Get("/payments/:id", c.BizBillingHandler.GetPayment)
		biz.Post("/payments/:id/refund", c.BizBillingHandler.RefundPayment)

		// ── Estimates ──
		biz.Post("/estimates", c.BizBillingHandler.CreateEstimate)
		biz.Get("/estimates", c.BizBillingHandler.ListEstimates)
		biz.Get("/estimates/:id", c.BizBillingHandler.GetEstimate)
		biz.Put("/estimates/:id", c.BizBillingHandler.UpdateEstimate)
		biz.Delete("/estimates/:id", c.BizBillingHandler.DeleteEstimate)
		biz.Post("/estimates/:id/send", c.BizBillingHandler.SendEstimate)
		biz.Post("/estimates/:id/accept", c.BizBillingHandler.AcceptEstimate)
		biz.Post("/estimates/:id/reject", c.BizBillingHandler.RejectEstimate)
		biz.Post("/estimates/:id/convert", c.BizBillingHandler.ConvertEstimate)

		// ── Coupons ──
		biz.Post("/coupons", c.BizBillingHandler.CreateCoupon)
		biz.Post("/coupons/bulk-generate", c.BizBillingHandler.BulkGenerateCoupons)
		biz.Get("/coupons", c.BizBillingHandler.ListCoupons)
		biz.Get("/coupons/:id", c.BizBillingHandler.GetCoupon)
		biz.Put("/coupons/:id", c.BizBillingHandler.UpdateCoupon)
		biz.Delete("/coupons/:id", c.BizBillingHandler.DeleteCoupon)

		// ── Credit Notes & Retainers ──
		biz.Post("/credit-notes", c.BizBillingHandler.CreateCreditNote)
		biz.Get("/credit-notes", c.BizBillingHandler.ListCreditNotes)
		biz.Get("/credit-notes/:id", c.BizBillingHandler.GetCreditNote)
		biz.Post("/credit-notes/:id/refund", c.BizBillingHandler.RefundCreditNote)
		biz.Post("/retainers", c.BizBillingHandler.CreateRetainer)
		biz.Get("/retainers", c.BizBillingHandler.ListRetainers)
		biz.Get("/retainers/:id", c.BizBillingHandler.GetRetainer)
		biz.Post("/retainers/:id/mark-paid", c.BizBillingHandler.MarkRetainerPaid)

		// ── Contracts ──
		biz.Post("/contracts", c.BizBillingHandler.CreateContract)
		biz.Get("/contracts", c.BizBillingHandler.ListContracts)
		biz.Get("/contracts/:id", c.BizBillingHandler.GetContract)
		biz.Put("/contracts/:id", c.BizBillingHandler.UpdateContract)
		biz.Post("/contracts/:id/send", c.BizBillingHandler.SendContract)
		biz.Post("/contracts/:id/accept", c.BizBillingHandler.AcceptContract)
		biz.Post("/contracts/:id/amend", c.BizBillingHandler.AmendContract)
		biz.Post("/contracts/:id/renew", c.BizBillingHandler.RenewContract)
		biz.Post("/contracts/:id/terminate", c.BizBillingHandler.TerminateContract)

		// ── Usage Metering ──
		biz.Post("/usage/meters", c.BizBillingHandler.CreateUsageMeter)
		biz.Get("/usage/meters", c.BizBillingHandler.ListUsageMeters)
		biz.Post("/usage/events", c.BizBillingHandler.ReportUsage)
		biz.Get("/usage/summary", c.BizBillingHandler.GetUsageSummary)

		// ── Expenses (static paths before /:id) ──
		biz.Post("/expenses", c.BizBillingHandler.CreateExpense)
		biz.Get("/expenses", c.BizBillingHandler.ListExpenses)
		biz.Get("/expenses/summary", c.BizBillingHandler.GetExpenseSummary)
		biz.Post("/expenses/convert", c.BizBillingHandler.ConvertExpensesToInvoice)
		biz.Get("/expenses/:id", c.BizBillingHandler.GetExpense)
		biz.Put("/expenses/:id", c.BizBillingHandler.UpdateExpense)
		biz.Delete("/expenses/:id", c.BizBillingHandler.DeleteExpense)

		// ── Customer billing (statement, credit balance, portal link) ──
		biz.Get("/users/:user_id/statement", c.BizBillingHandler.GetUserStatement)
		biz.Post("/users/:user_id/balance", c.BizBillingHandler.AdjustUserBalance)
		biz.Post("/users/:user_id/portal-link", c.BizBillingHandler.CreatePortalLink)

		// ── Analytics (same API-key auth as rest of /v1/biz; dashboard sends X-API-Key) ──
		biz.Get("/analytics/revenue", c.BizBillingHandler.AnalyticsRevenue)
		biz.Get("/analytics/subscriptions", c.BizBillingHandler.AnalyticsSubscriptions)
		biz.Get("/analytics/invoices", c.BizBillingHandler.AnalyticsInvoices)
		biz.Get("/analytics/aging", c.BizBillingHandler.AnalyticsAging)
		biz.Get("/analytics/customers", c.BizBillingHandler.AnalyticsCustomers)
		biz.Get("/analytics/dunning", c.BizBillingHandler.AnalyticsDunning)
		biz.Get("/analytics/revenue-recognition", c.BizBillingHandler.AnalyticsRevenueRecognition)
		biz.Get("/analytics/tax-summary", c.BizBillingHandler.AnalyticsTaxSummary)

		// ── Third-party billing connectors ──
		biz.Post("/connectors", c.BizBillingHandler.CreateConnector)
		biz.Get("/connectors", c.BizBillingHandler.ListConnectors)
		biz.Put("/connectors/:id", c.BizBillingHandler.UpdateConnector)
		biz.Delete("/connectors/:id", c.BizBillingHandler.DeleteConnector)
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
	billing.Get("/plans", c.BillingHandler.GetPlans)
	billing.Get("/usage", c.BillingHandler.GetUsage)
	billing.Get("/subscription", c.BillingHandler.GetSubscription)
	billing.Post("/accept-trial", c.BillingHandler.AcceptTrial)
	billing.Get("/usage/breakdown", c.BillingHandler.GetUsageBreakdown)
	billing.Get("/rates", c.BillingHandler.GetRates)

	if c.PaymentHandler != nil {
		billing.Post("/checkout", c.PaymentHandler.CreateOrder)
		billing.Post("/verify-payment", c.PaymentHandler.VerifyPayment)
	}

	// Admin billing rate-card controls (JWT-protected)
	adminBilling := adminAuth.Group("/billing")
	adminBilling.Get("/plans", c.BillingHandler.AdminGetPlans)
	adminBilling.Post("/plans/set", c.BillingHandler.AdminSetPlan)
	adminBilling.Get("/rates", c.BillingHandler.AdminGetRates)
	adminBilling.Post("/rates/set", c.BillingHandler.AdminSetRate)
	adminBilling.Post("/rates/activate", c.BillingHandler.AdminActivateRate)
	adminBilling.Post("/rates/rollback", c.BillingHandler.AdminRollbackRate)

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
		// Deprecated tenant billing APIs are not used by the UI. Keep them
		// mounted for backward compatibility while the dashboard uses /v1/billing/*.
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
	apps.Get("/:id/code-samples", c.ApplicationHandler.GetCodeSamples)

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
		ops.Post("/credits/grant", c.OpsHandler.GrantCredits)
		ops.Post("/billing/rebalance-credits", c.OpsHandler.RebalanceCredits)
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
