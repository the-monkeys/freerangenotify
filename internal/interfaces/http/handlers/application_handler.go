package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

// ApplicationHandler handles application-related HTTP requests
type ApplicationHandler struct {
	service   usecases.ApplicationService
	validator *validator.Validator
	logger    *zap.Logger
}

// NewApplicationHandler creates a new ApplicationHandler
func NewApplicationHandler(service usecases.ApplicationService, v *validator.Validator, logger *zap.Logger) *ApplicationHandler {
	return &ApplicationHandler{
		service:   service,
		validator: v,
		logger:    logger,
	}
}

// Create handles POST /v1/apps
func (h *ApplicationHandler) Create(c *fiber.Ctx) error {
	// Get admin user ID from JWT context
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	var req dto.CreateApplicationRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
	}

	app := &application.Application{
		AppName:     req.AppName,
		AdminUserID: userID,
		Description: req.Description,
		WebhookURL:  req.WebhookURL,
		Webhooks:    req.Webhooks,
	}

	if req.Settings != nil {
		app.Settings = *req.Settings
	}

	if err := h.service.Create(c.Context(), app); err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    dto.ToApplicationResponse(app),
		"message": "Application created successfully. Save the API key securely - it won't be shown again in full.",
	})
}

// GetByID handles GET /v1/apps/:id
func (h *ApplicationHandler) GetByID(c *fiber.Ctx) error {
	// Get admin user ID from JWT context
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
	}

	app, err := h.service.GetByID(c.Context(), appID)
	if err != nil {
		return err
	}

	// Verify ownership
	if app.AdminUserID != userID {
		return errors.Forbidden("You do not have access to this application")
	}

	response := dto.ToApplicationResponse(app)
	// Masking removed to allow management from dashboard
	// if len(response.APIKey) > 8 {
	// 	response.APIKey = "***" + response.APIKey[len(response.APIKey)-8:]
	// }

	return c.JSON(fiber.Map{
		"success": true,
		"data":    response,
	})
}

// Update handles PUT /v1/apps/:id
func (h *ApplicationHandler) Update(c *fiber.Ctx) error {
	// Get admin user ID from JWT context
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
	}

	var req dto.UpdateApplicationRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
	}

	// Get existing application
	app, err := h.service.GetByID(c.Context(), appID)
	if err != nil {
		return err
	}

	// Verify ownership
	if app.AdminUserID != userID {
		return errors.Forbidden("You do not have access to this application")
	}

	// Update fields
	if req.AppName != "" {
		app.AppName = req.AppName
	}
	if req.Description != "" {
		app.Description = req.Description
	}
	if req.WebhookURL != "" {
		app.WebhookURL = req.WebhookURL
	}
	if req.Webhooks != nil {
		app.Webhooks = req.Webhooks
	}
	if req.Settings != nil {
		app.Settings = *req.Settings
	}

	if err := h.service.Update(c.Context(), app); err != nil {
		return err
	}

	response := dto.ToApplicationResponse(app)
	// Masking removed
	// if len(response.APIKey) > 8 {
	// 	response.APIKey = "***" + response.APIKey[len(response.APIKey)-8:]
	// }

	return c.JSON(fiber.Map{
		"success": true,
		"data":    response,
	})
}

// Delete handles DELETE /v1/apps/:id
func (h *ApplicationHandler) Delete(c *fiber.Ctx) error {
	// Get admin user ID from JWT context
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
	}

	// Verify ownership
	app, err := h.service.GetByID(c.Context(), appID)
	if err != nil {
		return err
	}
	if app.AdminUserID != userID {
		return errors.Forbidden("You do not have access to this application")
	}

	if err := h.service.Delete(c.Context(), appID); err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Application deleted successfully",
	})
}

// List handles GET /v1/apps
func (h *ApplicationHandler) List(c *fiber.Ctx) error {
	// Get admin user ID from JWT context
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	// Always filter by current user's applications
	filter := application.ApplicationFilter{
		AppName:     c.Query("app_name"),
		AdminUserID: userID,
		Limit:       pageSize,
		Offset:      offset,
	}

	apps, total, err := h.service.List(c.Context(), filter)
	if err != nil {
		return err
	}

	appResponses := make([]dto.ApplicationResponse, len(apps))
	for i, app := range apps {
		response := dto.ToApplicationResponse(app)
		// Mask API keys in list view
		if len(response.APIKey) > 8 {
			response.APIKey = "***" + response.APIKey[len(response.APIKey)-8:]
		}
		appResponses[i] = response
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": dto.ListApplicationsResponse{
			Applications: appResponses,
			TotalCount:   total,
			Page:         page,
			PageSize:     pageSize,
		},
	})
}

// RegenerateAPIKey handles POST /v1/apps/:id/regenerate-key
func (h *ApplicationHandler) RegenerateAPIKey(c *fiber.Ctx) error {
	// Get admin user ID from JWT context
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
	}

	// Verify ownership
	app, err := h.service.GetByID(c.Context(), appID)
	if err != nil {
		return err
	}
	if app.AdminUserID != userID {
		return errors.Forbidden("You do not have access to this application")
	}

	newAPIKey, err := h.service.RegenerateAPIKey(c.Context(), appID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": dto.RegenerateAPIKeyResponse{
			APIKey:  newAPIKey,
			Message: "API key regenerated successfully. Save it securely - it won't be shown again.",
		},
	})
}

// UpdateSettings handles PUT /v1/apps/:id/settings
func (h *ApplicationHandler) UpdateSettings(c *fiber.Ctx) error {
	// Get admin user ID from JWT context
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
	}

	// Verify ownership
	app, err := h.service.GetByID(c.Context(), appID)
	if err != nil {
		return err
	}
	if app.AdminUserID != userID {
		return errors.Forbidden("You do not have access to this application")
	}

	var req dto.UpdateSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
	}

	// Get existing settings first to support partial updates
	currentSettings, err := h.service.GetSettings(c.Context(), appID)
	if err != nil {
		return err
	}

	settings := *currentSettings

	if req.RateLimit != nil {
		settings.RateLimit = *req.RateLimit
	}
	if req.RetryAttempts != nil {
		settings.RetryAttempts = *req.RetryAttempts
	}
	if req.DefaultTemplate != nil {
		settings.DefaultTemplate = *req.DefaultTemplate
	}
	if req.EnableWebhooks != nil {
		settings.EnableWebhooks = *req.EnableWebhooks
	}
	if req.EnableAnalytics != nil {
		settings.EnableAnalytics = *req.EnableAnalytics
	}
	if req.ValidationURL != nil {
		settings.ValidationURL = *req.ValidationURL
	}
	if req.ValidationConfig != nil {
		settings.ValidationConfig = req.ValidationConfig
	}
	if req.EmailConfig != nil {
		settings.EmailConfig = req.EmailConfig
	}
	if req.DailyEmailLimit != nil {
		settings.DailyEmailLimit = *req.DailyEmailLimit
	}
	if req.DefaultPreferences != nil {
		if settings.DefaultPreferences == nil {
			settings.DefaultPreferences = &application.DefaultPreferences{}
		}
		if req.DefaultPreferences.EmailEnabled != nil {
			settings.DefaultPreferences.EmailEnabled = req.DefaultPreferences.EmailEnabled
		}
		if req.DefaultPreferences.PushEnabled != nil {
			settings.DefaultPreferences.PushEnabled = req.DefaultPreferences.PushEnabled
		}
		if req.DefaultPreferences.SMSEnabled != nil {
			settings.DefaultPreferences.SMSEnabled = req.DefaultPreferences.SMSEnabled
		}
	}

	if err := h.service.UpdateSettings(c.Context(), appID, settings); err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Settings updated successfully",
	})
}

// GetSettings handles GET /v1/apps/:id/settings
func (h *ApplicationHandler) GetSettings(c *fiber.Ctx) error {
	// Get admin user ID from JWT context
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
	}

	// Verify ownership
	app, err := h.service.GetByID(c.Context(), appID)
	if err != nil {
		return err
	}
	if app.AdminUserID != userID {
		return errors.Forbidden("You do not have access to this application")
	}

	settings, err := h.service.GetSettings(c.Context(), appID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    settings,
	})
}
