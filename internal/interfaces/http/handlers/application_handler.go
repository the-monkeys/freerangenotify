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
	var req dto.CreateApplicationRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
	}

	app := &application.Application{
		AppName:    req.AppName,
		WebhookURL: req.WebhookURL,
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
	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
	}

	app, err := h.service.GetByID(c.Context(), appID)
	if err != nil {
		return err
	}

	response := dto.ToApplicationResponse(app)
	// Mask API key for security (show only last 8 characters)
	if len(response.APIKey) > 8 {
		response.APIKey = "***" + response.APIKey[len(response.APIKey)-8:]
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    response,
	})
}

// Update handles PUT /v1/apps/:id
func (h *ApplicationHandler) Update(c *fiber.Ctx) error {
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

	// Update fields
	if req.AppName != "" {
		app.AppName = req.AppName
	}
	if req.WebhookURL != "" {
		app.WebhookURL = req.WebhookURL
	}
	if req.Settings != nil {
		app.Settings = *req.Settings
	}

	if err := h.service.Update(c.Context(), app); err != nil {
		return err
	}

	response := dto.ToApplicationResponse(app)
	// Mask API key
	if len(response.APIKey) > 8 {
		response.APIKey = "***" + response.APIKey[len(response.APIKey)-8:]
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    response,
	})
}

// Delete handles DELETE /v1/apps/:id
func (h *ApplicationHandler) Delete(c *fiber.Ctx) error {
	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
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

	filter := application.ApplicationFilter{
		AppName: c.Query("app_name"),
		Limit:   pageSize,
		Offset:  offset,
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
	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
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
	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
	}

	var req dto.UpdateSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
	}

	settings := application.Settings{
		RateLimit:       req.RateLimit,
		RetryAttempts:   req.RetryAttempts,
		DefaultTemplate: req.DefaultTemplate,
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
	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
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
