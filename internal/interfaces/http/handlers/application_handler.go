package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/agentdebug"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

// ApplicationHandler handles application-related HTTP requests
type ApplicationHandler struct {
	service        usecases.ApplicationService
	membershipRepo auth.MembershipRepository
	appRepo        application.Repository
	validator      *validator.Validator
	logger         *zap.Logger
}

// NewApplicationHandler creates a new ApplicationHandler
func NewApplicationHandler(service usecases.ApplicationService, membershipRepo auth.MembershipRepository, appRepo application.Repository, v *validator.Validator, logger *zap.Logger) *ApplicationHandler {
	return &ApplicationHandler{
		service:        service,
		membershipRepo: membershipRepo,
		appRepo:        appRepo,
		validator:      v,
		logger:         logger,
	}
}

// authorizeAppAccess checks whether the authenticated user has access to the
// specified application and returns the app along with the user's resolved role.
// The app owner always gets RoleOwner. Team members get their membership role.
// When RBAC is disabled (membershipRepo is nil), only the owner has access.
func (h *ApplicationHandler) authorizeAppAccess(c *fiber.Ctx, appID, userID string) (*application.Application, auth.Role, error) {
	app, err := h.service.GetByID(c.Context(), appID)
	if err != nil {
		return nil, "", err
	}

	if app.AdminUserID == userID {
		c.Locals("role", auth.RoleOwner)
		return app, auth.RoleOwner, nil
	}

	if h.membershipRepo != nil {
		membership, mErr := h.membershipRepo.GetByAppAndUser(c.Context(), appID, userID)
		if mErr == nil && membership != nil {
			c.Locals("role", membership.Role)
			return app, membership.Role, nil
		}
	}

	return nil, "", errors.Forbidden("You do not have access to this application")
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

// GetByID handles GET /v1/apps/:id — any team member can view
func (h *ApplicationHandler) GetByID(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
	}

	app, _, err := h.authorizeAppAccess(c, appID, userID)
	if err != nil {
		return err
	}

	response := dto.ToApplicationResponse(app)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    response,
	})
}

// Update handles PUT /v1/apps/:id — requires admin or owner role
func (h *ApplicationHandler) Update(c *fiber.Ctx) error {
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

	app, role, err := h.authorizeAppAccess(c, appID, userID)
	if err != nil {
		return err
	}
	if role != auth.RoleOwner && role != auth.RoleAdmin {
		return errors.Forbidden("admin or owner role required to update the application")
	}

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

	return c.JSON(fiber.Map{
		"success": true,
		"data":    response,
	})
}

// Delete handles DELETE /v1/apps/:id — owner only
func (h *ApplicationHandler) Delete(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
	}

	_, role, err := h.authorizeAppAccess(c, appID, userID)
	if err != nil {
		return err
	}
	if role != auth.RoleOwner {
		return errors.Forbidden("only the application owner can delete the application")
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

	// Fetch apps the user owns
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

	// Also include apps where the user is a team member
	if h.membershipRepo != nil && h.appRepo != nil {
		memberships, mErr := h.membershipRepo.ListByUser(c.Context(), userID)
		if mErr == nil && len(memberships) > 0 {
			agentdebug.Log(
				"pre-fix-rbac",
				"H5-app-list-memberships",
				"internal/interfaces/http/handlers/application_handler.go:List",
				"resolved memberships for app list",
				map[string]any{
					"user_id":          userID,
					"membership_count": len(memberships),
				},
			)
			// Build a set of owned app IDs to avoid duplicates
			ownedIDs := make(map[string]struct{}, len(apps))
			for _, a := range apps {
				ownedIDs[a.AppID] = struct{}{}
			}

			for _, m := range memberships {
				if _, exists := ownedIDs[m.AppID]; exists {
					continue
				}
				memberApp, aErr := h.appRepo.GetByID(c.Context(), m.AppID)
				if aErr != nil {
					continue
				}
				apps = append(apps, memberApp)
				total++
			}
		} else if mErr != nil {
			agentdebug.Log(
				"pre-fix-rbac",
				"H5-app-list-memberships",
				"internal/interfaces/http/handlers/application_handler.go:List",
				"failed to resolve memberships for app list",
				map[string]any{
					"user_id": userID,
					"error":   mErr.Error(),
				},
			)
		}
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

// RegenerateAPIKey handles POST /v1/apps/:id/regenerate-key — owner only
func (h *ApplicationHandler) RegenerateAPIKey(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
	}

	_, role, err := h.authorizeAppAccess(c, appID, userID)
	if err != nil {
		return err
	}
	if role != auth.RoleOwner {
		return errors.Forbidden("only the application owner can regenerate the API key")
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

// UpdateSettings handles PUT /v1/apps/:id/settings — requires admin or owner role
func (h *ApplicationHandler) UpdateSettings(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
	}

	_, role, err := h.authorizeAppAccess(c, appID, userID)
	if err != nil {
		return err
	}
	if role != auth.RoleOwner && role != auth.RoleAdmin {
		return errors.Forbidden("admin or owner role required to update settings")
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

// GetSettings handles GET /v1/apps/:id/settings — any team member can view
func (h *ApplicationHandler) GetSettings(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app_id is required")
	}

	if _, _, err := h.authorizeAppAccess(c, appID, userID); err != nil {
		return err
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
