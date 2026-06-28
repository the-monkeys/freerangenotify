package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/agentdebug"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/tenant"
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
	tenantService  tenant.Service
	appRepo        application.Repository
	validator      *validator.Validator
	logger         *zap.Logger
}

// NewApplicationHandler creates a new ApplicationHandler
func NewApplicationHandler(service usecases.ApplicationService, membershipRepo auth.MembershipRepository, tenantService tenant.Service, appRepo application.Repository, v *validator.Validator, logger *zap.Logger) *ApplicationHandler {
	return &ApplicationHandler{
		service:        service,
		membershipRepo: membershipRepo,
		tenantService:  tenantService,
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

	// C1: Tenant members get access to apps in their tenant
	if app.TenantID != "" && h.tenantService != nil {
		hasAccess, role, tErr := h.tenantService.HasAccess(c.Context(), app.TenantID, userID)
		if tErr == nil && hasAccess {
			// Map tenant role to app role:
			// owner/admin can manage tenant apps; members are read-only.
			appRole := auth.RoleViewer
			if role == "owner" || role == "admin" {
				appRole = auth.RoleAdmin
			}
			c.Locals("role", appRole)
			return app, appRole, nil
		}
	}

	return nil, "", errors.Forbidden("You do not have access to this application")
}

// Create handles POST /v1/apps
// @Summary Create a new application
// @Description Create a new application and generate an API key
// @Tags Applications
// @Accept json
// @Produce json
// @Param body body dto.CreateApplicationRequest true "Application creation request"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/apps [post]
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

	if req.TenantID != "" && h.tenantService != nil {
		hasAccess, _, err := h.tenantService.HasAccess(c.Context(), req.TenantID, userID)
		if err != nil {
			return err
		}
		if !hasAccess {
			return errors.Forbidden("You do not have access to this tenant")
		}
	}

	app := &application.Application{
		AppName:     req.AppName,
		AdminUserID: userID,
		TenantID:    req.TenantID,
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
// @Summary Get an application by ID
// @Description Retrieve application details by ID (any team member can view)
// @Tags Applications
// @Produce json
// @Param id path string true "Application ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/apps/{id} [get]
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
// @Summary Update an application
// @Description Update application details (requires admin or owner role)
// @Tags Applications
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Param body body dto.UpdateApplicationRequest true "Application update request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/apps/{id} [put]
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

	// Changing org association is an owner-only operation.
	if req.TenantID != nil && *req.TenantID != app.TenantID {
		if role != auth.RoleOwner {
			return errors.Forbidden("only the application owner can change the organization")
		}
		// If moving to a tenant, user must also be owner/admin of the target tenant.
		if *req.TenantID != "" && h.tenantService != nil {
			hasAccess, tenantRole, tErr := h.tenantService.HasAccess(c.Context(), *req.TenantID, userID)
			if tErr != nil {
				return tErr
			}
			if !hasAccess || (tenantRole != "owner" && tenantRole != "admin") {
				return errors.Forbidden("only tenant owners/admins can move an app to this organization")
			}
		}
	}

	if req.AppName != "" {
		app.AppName = req.AppName
	}
	if req.Description != "" {
		app.Description = req.Description
	}
	if req.TenantID != nil {
		app.TenantID = *req.TenantID
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
// @Summary Delete an application
// @Description Permanently delete an application (owner only)
// @Tags Applications
// @Produce json
// @Param id path string true "Application ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/apps/{id} [delete]
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
// @Summary List applications
// @Description List all applications owned by or shared with the authenticated user
// @Tags Applications
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param app_name query string false "Filter by application name"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/apps [get]
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

	// Build a set of app IDs we've already included (to avoid duplicates)
	ownedIDs := make(map[string]struct{}, len(apps))
	for _, a := range apps {
		ownedIDs[a.AppID] = struct{}{}
	}

	// Include apps from tenants the user belongs to
	if h.tenantService != nil && h.appRepo != nil {
		tenants, tErr := h.tenantService.ListByUser(c.Context(), userID)
		if tErr == nil && len(tenants) > 0 {
			tenantIDs := make([]string, 0, len(tenants))
			for _, t := range tenants {
				tenantIDs = append(tenantIDs, t.ID)
			}
			tenantApps, _ := h.appRepo.List(c.Context(), application.ApplicationFilter{
				TenantIDs: tenantIDs,
				Limit:     100,
			})
			for _, a := range tenantApps {
				if _, exists := ownedIDs[a.AppID]; exists {
					continue
				}
				ownedIDs[a.AppID] = struct{}{}
				apps = append(apps, a)
				total++
			}
		}
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

			for _, m := range memberships {
				if _, exists := ownedIDs[m.AppID]; exists {
					continue
				}
				memberApp, aErr := h.appRepo.GetByID(c.Context(), m.AppID)
				if aErr != nil {
					continue
				}
				ownedIDs[m.AppID] = struct{}{}
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
// @Summary Regenerate API key
// @Description Regenerate the API key for an application (owner only). The old key is immediately invalidated.
// @Tags Applications
// @Produce json
// @Param id path string true "Application ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/apps/{id}/regenerate-key [post]
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
// @Summary Update application settings
// @Description Update settings such as rate limits, retry attempts, and provider config (admin or owner)
// @Tags Applications
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Param body body dto.UpdateSettingsRequest true "Settings update request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/apps/{id}/settings [put]
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
		if req.EmailConfig.SMTP != nil && req.EmailConfig.SMTP.Port == 0 {
			req.EmailConfig.SMTP.Port = 587
		}
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
		if req.DefaultPreferences.WhatsAppEnabled != nil {
			settings.DefaultPreferences.WhatsAppEnabled = req.DefaultPreferences.WhatsAppEnabled
		}
	}
	if req.WhatsAppConfig != nil {
		settings.WhatsApp = req.WhatsAppConfig
	}
	if req.SMSConfig != nil {
		settings.SMS = req.SMSConfig
	}
	if req.OnUserCreatedTriggerID != nil {
		settings.OnUserCreatedTriggerID = *req.OnUserCreatedTriggerID
	}
	if req.InboundWebhookConfig != nil {
		settings.InboundWebhookConfig = req.InboundWebhookConfig
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
// @Summary Get application settings
// @Description Retrieve settings for an application (any team member can view)
// @Tags Applications
// @Produce json
// @Param id path string true "Application ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/apps/{id}/settings [get]
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

// GetCodeSamples handles GET /v1/apps/:id/code-samples
// @Summary Get dynamic code samples
// @Description Get boilerplate integration code for the application with real credentials pre-filled
// @Tags Applications
// @Produce json
// @Param id path string true "Application ID"
// @Param language query string false "Specific language (go, python, java, js, cpp, rust, ruby). Defaults to all."
// @Success 200 {object} map[string]string
// @Security BearerAuth
// @Router /v1/apps/{id}/code-samples [get]
func (h *ApplicationHandler) GetCodeSamples(c *fiber.Ctx) error {
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

	apiKey := app.APIKey
	// TODO: Make this dynamic - Get it from the config or environment
	baseURL := "https://freerangenotify.monkeys.support/v1"

	type Snippet struct {
		Title string `json:"title"`
		Code  string `json:"code"`
	}
	type ChannelSamples struct {
		Sender   Snippet  `json:"sender"`
		Receiver *Snippet `json:"receiver,omitempty"`
		Auth     *Snippet `json:"auth,omitempty"`
	}

	// Helper to create the auth snippet (Step 1 of SSE)
	getAuthSnippet := func(lang, apiKey, baseURL string) Snippet {
		switch lang {
		case "python":
			return Snippet{
				Title: "Step 1: Generate SSE Token (Server-side)",
				Code: `import requests

# This MUST be done on your backend to keep API Key secret
api_key = "` + apiKey + `"
url = "` + baseURL + `/sse/tokens"

response = requests.post(url, 
    json={"user_id": "CUSTOMER_USER_ID"}, 
    headers={"X-API-Key": api_key}
)
sse_token = response.json()["sse_token"]
print(f"Token for client: {sse_token}")`,
			}
		case "javascript":
			return Snippet{
				Title: "Step 1: Generate SSE Token (Server-side Node.js)",
				Code: `const axios = require('axios');

// This MUST be done on your backend to keep API Key secret
async function getSseToken(userId) {
  const response = await axios.post("` + baseURL + `/sse/tokens", 
    { user_id: userId },
    { headers: { "X-API-Key": "` + apiKey + `" } }
  );
  return response.data.sse_token;
}`,
			}
		default: // Go
			return Snippet{
				Title: "Step 1: Generate SSE Token (Server-side)",
				Code: `// POST ` + baseURL + `/sse/tokens 
// Header: X-API-Key: ` + apiKey + `
// Body: {"user_id": "USER_ID"}`,
			}
		}
	}

	samples := make(map[string]map[string]ChannelSamples)

	// ── GOLANG ──
	goSamples := make(map[string]ChannelSamples)
	goBaseSender := `package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func main() {
	apiKey := "` + apiKey + `"
	url := "` + baseURL + `/notifications"

	payload := map[string]interface{}{
		"user_id":     "USER_ID_HERE",
		"template_id": "YOUR_TEMPLATE_ID",
		"channel":     "%s",
		"data": map[string]string{
			"key": "value",
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	http.DefaultClient.Do(req)
}`

	goSamples["email"] = ChannelSamples{Sender: Snippet{Title: "Publish Email", Code: fmt.Sprintf(goBaseSender, "email")}}
	goSamples["sms"] = ChannelSamples{Sender: Snippet{Title: "Publish SMS", Code: fmt.Sprintf(goBaseSender, "sms")}}
	goSamples["whatsapp"] = ChannelSamples{Sender: Snippet{Title: "Publish WhatsApp", Code: fmt.Sprintf(goBaseSender, "whatsapp")}}
	goSamples["webhook"] = ChannelSamples{Sender: Snippet{Title: "Publish Webhook", Code: fmt.Sprintf(goBaseSender, "webhook")}}
	goSamples["sse"] = ChannelSamples{
		Sender:   Snippet{Title: "Publish InApp (SSE)", Code: fmt.Sprintf(goBaseSender, "sse")},
		Auth:     &Snippet{Title: "Step 1: Generate SSE Token", Code: getAuthSnippet("go", apiKey, baseURL).Code},
		Receiver: &Snippet{Title: "Step 2: Receive SSE (Client-side)", Code: `// Connect to ` + baseURL + `/sse?sse_token=TOKEN`},
	}
	samples["go"] = goSamples

	// ── PYTHON ──
	pySamples := make(map[string]ChannelSamples)
	pyBaseSender := `import requests

def send_notification(user_id):
    url = "` + baseURL + `/notifications"
    headers = {
        "X-API-Key": "` + apiKey + `",
        "Content-Type": "application/json"
    }
    payload = {
        "user_id": user_id,
        "channel": "%s",
        "template_id": "YOUR_TEMPLATE_ID",
        "data": {"key": "value"}
    }
    return requests.post(url, json=payload, headers=headers)`

	pySamples["email"] = ChannelSamples{Sender: Snippet{Title: "Publish Email", Code: fmt.Sprint(fmt.Sprintf(pyBaseSender, "email"))}}
	pySamples["sms"] = ChannelSamples{Sender: Snippet{Title: "Publish SMS", Code: fmt.Sprint(fmt.Sprintf(pyBaseSender, "sms"))}}
	pySamples["whatsapp"] = ChannelSamples{Sender: Snippet{Title: "Publish WhatsApp", Code: fmt.Sprint(fmt.Sprintf(pyBaseSender, "whatsapp"))}}
	pySamples["webhook"] = ChannelSamples{Sender: Snippet{Title: "Publish Webhook", Code: fmt.Sprint(fmt.Sprintf(pyBaseSender, "webhook"))}}
	pySamples["sse"] = ChannelSamples{
		Sender: Snippet{Title: "Publish InApp (SSE)", Code: fmt.Sprint(fmt.Sprintf(pyBaseSender, "sse"))},
		Auth:   &Snippet{Title: "Step 1: Generate SSE Token", Code: getAuthSnippet("python", apiKey, baseURL).Code},
		Receiver: &Snippet{Title: "Step 2: Receive SSE (Client-side)", Code: `import requests
# 1. Get token from your backend
# 2. Connect
url = "` + baseURL + `/sse?sse_token=YOUR_TOKEN"
r = requests.get(url, stream=True)
for line in r.iter_lines():
    if line.startswith(b"data: "):
        print(line)`},
	}
	samples["python"] = pySamples

	// ── JAVASCRIPT ──
	jsSamples := make(map[string]ChannelSamples)
	jsBaseSender := `const sendNotification = async (userId) => {
  const url = "` + baseURL + `/notifications";
  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'X-API-Key': "` + apiKey + `",
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      user_id: userId,
      channel: "%s",
      template_id: "YOUR_TEMPLATE_ID",
      data: { key: "value" }
    })
  });
  return response.json();
};`

	jsSamples["email"] = ChannelSamples{Sender: Snippet{Title: "Publish Email", Code: fmt.Sprintf(jsBaseSender, "email")}}
	jsSamples["sms"] = ChannelSamples{Sender: Snippet{Title: "Publish SMS", Code: fmt.Sprintf(jsBaseSender, "sms")}}
	jsSamples["whatsapp"] = ChannelSamples{Sender: Snippet{Title: "Publish WhatsApp", Code: fmt.Sprintf(jsBaseSender, "whatsapp")}}
	jsSamples["webhook"] = ChannelSamples{Sender: Snippet{Title: "Publish Webhook", Code: fmt.Sprintf(jsBaseSender, "webhook")}}
	jsSamples["sse"] = ChannelSamples{
		Sender: Snippet{Title: "Publish InApp (SSE)", Code: fmt.Sprintf(jsBaseSender, "sse")},
		Auth:   &Snippet{Title: "Step 1: Generate SSE Token", Code: getAuthSnippet("javascript", apiKey, baseURL).Code},
		Receiver: &Snippet{Title: "Step 2: Receive SSE (Browser)", Code: `// 1. Get sseToken from your backend
const eventSource = new EventSource("` + baseURL + `/sse?sse_token=" + sseToken);
eventSource.onmessage = (e) => console.log("New Notification:", JSON.parse(e.data));`},
	}
	samples["javascript"] = jsSamples

	// Helper for others (simplified)
	addSimpleSamples := func(lang string, baseSender string) {
		lSamples := make(map[string]ChannelSamples)
		for _, ch := range []string{"email", "sms", "whatsapp", "webhook", "sse"} {
			lSamples[ch] = ChannelSamples{Sender: Snippet{Title: "Publish " + strings.Title(ch), Code: fmt.Sprintf(baseSender, ch)}}
		}
		samples[lang] = lSamples
	}

	addSimpleSamples("java", `import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;

public class Main {
    public static void main(String[] args) throws Exception {
        String apiKey = "` + apiKey + `";
        String url = "` + baseURL + `/notifications";

        String jsonPayload = "{"
            + "\"user_id\": \"USER_ID_HERE\","
            + "\"template_id\": \"YOUR_TEMPLATE_ID\","
            + "\"channel\": \"%s\","
            + "\"data\": {\"key\": \"value\"}"
            + "}";

        HttpClient client = HttpClient.newHttpClient();
        HttpRequest request = HttpRequest.newBuilder()
            .uri(URI.create(url))
            .header("X-API-Key", apiKey)
            .header("Content-Type", "application/json")
            .POST(HttpRequest.BodyPublishers.ofString(jsonPayload))
            .build();

        HttpResponse<String> response = client.send(request, HttpResponse.BodyHandlers.ofString());
        System.out.println("Status: " + response.statusCode());
    }
}`)
	addSimpleSamples("cpp", `#include <iostream>
#include <string>
#include <curl/curl.h>

int main() {
    CURL* curl = curl_easy_init();
    if(curl) {
        std::string url = "` + baseURL + `/notifications";
        std::string apiKey = "` + apiKey + `";
        std::string jsonPayload = "{"
            "\"user_id\": \"USER_ID_HERE\","
            "\"template_id\": \"YOUR_TEMPLATE_ID\","
            "\"channel\": \"%s\","
            "\"data\": {\"key\": \"value\"}"
        "}";

        struct curl_slist* headers = NULL;
        headers = curl_slist_append(headers, ("X-API-Key: " + apiKey).c_str());
        headers = curl_slist_append(headers, "Content-Type: application/json");

        curl_easy_setopt(curl, CURLOPT_URL, url.c_str());
        curl_easy_setopt(curl, CURLOPT_POST, 1L);
        curl_easy_setopt(curl, CURLOPT_POSTFIELDS, jsonPayload.c_str());
        curl_easy_setopt(curl, CURLOPT_HTTPHEADER, headers);

        CURLcode res = curl_easy_perform(curl);
        if(res != CURLE_OK) {
            std::cerr << "curl failed: " << curl_easy_strerror(res) << std::endl;
        }

        curl_slist_free_all(headers);
        curl_easy_cleanup(curl);
    }
    return 0;
}`)
	addSimpleSamples("rust", `use reqwest::header::{HeaderMap, HeaderValue, CONTENT_TYPE};
use serde_json::json;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let api_key = "` + apiKey + `";
    let url = "` + baseURL + `/notifications";

    let mut headers = HeaderMap::new();
    headers.insert("X-API-Key", HeaderValue::from_str(api_key)?);
    headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));

    let client = reqwest::Client::new();
    let payload = json!({
        "user_id": "USER_ID_HERE",
        "channel": "%s",
        "template_id": "YOUR_TEMPLATE_ID",
        "data": {
            "key": "value"
        }
    });

    let res = client.post(url)
        .headers(headers)
        .json(&payload)
        .send()
        .await?;

    println!("Status: {}", res.status());
    Ok(())
}`)
	addSimpleSamples("ruby", `require 'net/http'
require 'uri'
require 'json'

api_key = "` + apiKey + `"
uri = URI.parse("` + baseURL + `/notifications")

header = {
  'X-API-Key' => api_key,
  'Content-Type' => 'application/json'
}

payload = {
  user_id: 'USER_ID_HERE',
  channel: '%s',
  template_id: 'YOUR_TEMPLATE_ID',
  data: { key: 'value' }
}

http = Net::HTTP.new(uri.host, uri.port)
http.use_ssl = true
request = Net::HTTP::Post.new(uri.request_uri, header)
request.body = payload.to_json

response = http.request(request)
puts "Status: #{response.code}"`)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    samples,
	})
}
