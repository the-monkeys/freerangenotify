package handlers

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"github.com/the-monkeys/freerangenotify/internal/seed"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

var validateTemplate = validator.New()

type TemplateHandler struct {
	service      *usecases.TemplateService
	smtpProvider providers.Provider
	logger       *zap.Logger
}

func NewTemplateHandler(service *usecases.TemplateService, smtpProvider providers.Provider, logger *zap.Logger) *TemplateHandler {
	return &TemplateHandler{
		service:      service,
		smtpProvider: smtpProvider,
		logger:       logger,
	}
}

// CreateTemplate creates a new notification template
func (h *TemplateHandler) CreateTemplate(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)

	var req dto.CreateTemplateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
	}

	if err := validateTemplate.Validate(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Validation failed",
			"message": err.Error(),
		})
	}

	createReq := &template.CreateRequest{
		AppID:         appID, // Enforce AppID from context
		Name:          req.Name,
		Description:   req.Description,
		Channel:       req.Channel,
		WebhookTarget: req.WebhookTarget,
		Subject:       req.Subject,
		Body:          req.Body,
		Variables:     req.Variables,
		Metadata:      req.Metadata,
		Locale:        req.Locale,
		CreatedBy:     req.CreatedBy,
	}

	if envID, ok := c.Locals("environment_id").(string); ok {
		createReq.EnvironmentID = envID
	}

	tmpl, err := h.service.Create(c.Context(), createReq)
	if err != nil {
		h.logger.Error("Failed to create template", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to create template",
			"message": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(toTemplateResponse(tmpl))
}

// GetTemplate retrieves a template by ID
func (h *TemplateHandler) GetTemplate(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Template ID is required",
			"message": "Please provide a valid template ID",
		})
	}

	tmpl, err := h.service.GetByID(c.Context(), id, appID)
	if err != nil {
		h.logger.Error("Failed to get template", zap.String("id", id), zap.Error(err))
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "Template not found",
			"message": err.Error(),
		})
	}

	return c.JSON(toTemplateResponse(tmpl))
}

// ListTemplates lists templates based on filter criteria
func (h *TemplateHandler) ListTemplates(c *fiber.Ctx) error {
	var req dto.ListTemplatesRequest
	if err := c.QueryParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid query parameters",
			"message": err.Error(),
		})
	}

	if err := validateTemplate.Validate(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Validation failed",
			"message": err.Error(),
		})
	}

	filter := template.Filter{
		AppID:   c.Locals("app_id").(string), // Enforce AppID from context
		Name:    req.Name,
		Channel: req.Channel,
		Status:  req.Status,
		Locale:  req.Locale,
		Limit:   req.Limit,
		Offset:  req.Offset,
	}

	if envID, ok := c.Locals("environment_id").(string); ok {
		filter.EnvironmentID = envID
	}

	if req.FromDate != "" {
		fromDate, err := time.Parse(time.RFC3339, req.FromDate)
		if err == nil {
			filter.FromDate = &fromDate
		}
	}
	if req.ToDate != "" {
		toDate, err := time.Parse(time.RFC3339, req.ToDate)
		if err == nil {
			filter.ToDate = &toDate
		}
	}

	if filter.Limit == 0 {
		filter.Limit = 50
	}

	templates, err := h.service.List(c.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list templates", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to list templates",
			"message": err.Error(),
		})
	}

	responses := make([]*dto.TemplateResponse, len(templates))
	for i, tmpl := range templates {
		responses[i] = toTemplateResponse(tmpl)
	}

	return c.JSON(dto.ListTemplatesResponse{
		Templates: responses,
		Total:     len(responses),
		Limit:     filter.Limit,
		Offset:    filter.Offset,
	})
}

// UpdateTemplate updates an existing template
func (h *TemplateHandler) UpdateTemplate(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Template ID is required",
			"message": "Please provide a valid template ID",
		})
	}

	var req dto.UpdateTemplateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
	}

	if err := validateTemplate.Validate(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Validation failed",
			"message": err.Error(),
		})
	}

	updateReq := &template.UpdateRequest{
		UpdatedBy: req.UpdatedBy,
	}

	if req.Description != "" {
		updateReq.Description = &req.Description
	}
	if req.WebhookTarget != "" {
		updateReq.WebhookTarget = &req.WebhookTarget
	}
	if req.Subject != "" {
		updateReq.Subject = &req.Subject
	}
	if req.Body != "" {
		updateReq.Body = &req.Body
	}
	if len(req.Variables) > 0 {
		updateReq.Variables = &req.Variables
	}
	if req.Metadata != nil {
		updateReq.Metadata = req.Metadata
	}
	if req.Status != "" {
		updateReq.Status = &req.Status
	}

	tmpl, err := h.service.Update(c.Context(), id, appID, updateReq)
	if err != nil {
		h.logger.Error("Failed to update template", zap.String("id", id), zap.Error(err))
		if err.Error() == "template not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error":   "Template not found",
				"message": err.Error(),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to update template",
			"message": err.Error(),
		})
	}

	return c.JSON(toTemplateResponse(tmpl))
}

// DeleteTemplate permanently removes a template.
func (h *TemplateHandler) DeleteTemplate(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Template ID is required",
			"message": "Please provide a valid template ID",
		})
	}

	if err := h.service.Delete(c.Context(), id, appID); err != nil {
		h.logger.Error("Failed to delete template", zap.String("id", id), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to delete template",
			"message": err.Error(),
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// RenderTemplate renders a template with provided data
func (h *TemplateHandler) RenderTemplate(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Template ID is required",
			"message": "Please provide a valid template ID",
		})
	}

	var req dto.RenderTemplateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
	}

	if err := validateTemplate.Validate(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Validation failed",
			"message": err.Error(),
		})
	}

	rendered, err := h.service.Render(c.Context(), id, appID, req.Data)
	if err != nil {
		h.logger.Error("Failed to render template", zap.String("id", id), zap.Error(err))
		if err.Error() == "template not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error":   "Template not found",
				"message": err.Error(),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to render template",
			"message": err.Error(),
		})
	}

	return c.JSON(dto.RenderTemplateResponse{
		RenderedBody: rendered,
	})
}

// CreateTemplateVersion creates a new version of a template
func (h *TemplateHandler) CreateTemplateVersion(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string) // Ignore params app_id, force context one
	name := c.Params("name")

	if name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Template name is required",
			"message": "Please provide valid template name",
		})
	}

	var req dto.CreateVersionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
	}

	if err := validateTemplate.Validate(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Validation failed",
			"message": err.Error(),
		})
	}

	// Get the original template to get its ID
	locale := req.Locale
	if locale == "" {
		locale = "en"
	}

	original, err := h.service.GetByName(c.Context(), appID, name, locale)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "Template not found",
			"message": err.Error(),
		})
	}

	tmpl, err := h.service.CreateVersion(c.Context(), original.ID, appID, req.CreatedBy)
	if err != nil {
		h.logger.Error("Failed to create template version",
			zap.String("app_id", appID),
			zap.String("name", name),
			zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to create template version",
			"message": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(toTemplateResponse(tmpl))
}

// GetTemplateVersions retrieves all versions of a template
func (h *TemplateHandler) GetTemplateVersions(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	name := c.Params("name")

	if name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Template name is required",
			"message": "Please provide valid template name",
		})
	}

	// Get locale from query params — if omitted, return versions for all locales
	locale := c.Query("locale")

	versions, err := h.service.GetVersions(c.Context(), appID, name, locale)
	if err != nil {
		h.logger.Error("Failed to get template versions",
			zap.String("app_id", appID),
			zap.String("name", name),
			zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to get template versions",
			"message": err.Error(),
		})
	}

	responses := make([]*dto.TemplateResponse, len(versions))
	for i, v := range versions {
		responses[i] = toTemplateResponse(v)
	}

	return c.JSON(responses)
}

// GetTemplateVersion returns a single template at the specified version number.
// GET /v1/templates/:app_id/:name/versions/:version?locale=en-US
func (h *TemplateHandler) GetTemplateVersion(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	name := c.Params("name")
	versionStr := c.Params("version")

	if name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Template name is required",
			"message": "Please provide valid template name",
		})
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil || version < 1 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid version number",
			"message": "Version must be a positive integer",
		})
	}

	locale := c.Query("locale")

	tmpl, err := h.service.GetByVersion(c.Context(), appID, name, locale, version)
	if err != nil {
		h.logger.Error("Failed to get template version",
			zap.String("app_id", appID),
			zap.String("name", name),
			zap.Int("version", version),
			zap.Error(err))
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "Template version not found",
			"message": err.Error(),
		})
	}

	return c.JSON(toTemplateResponse(tmpl))
}

// GetLibrary returns the pre-built template library.
// Supports optional ?category= filter (transactional, newsletter, notification).
func (h *TemplateHandler) GetLibrary(c *fiber.Ctx) error {
	category := c.Query("category", "")

	templates := seed.LibraryTemplates
	if category != "" {
		var filtered []template.Template
		for _, t := range templates {
			if cat, ok := t.Metadata["category"].(string); ok && cat == category {
				filtered = append(filtered, t)
			}
		}
		templates = filtered
	}

	return c.JSON(fiber.Map{
		"templates": templates,
		"total":     len(templates),
	})
}

// CloneFromLibrary clones a library template into the user's app.
func (h *TemplateHandler) CloneFromLibrary(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	name := c.Params("name")

	var source *template.Template
	for i := range seed.LibraryTemplates {
		if seed.LibraryTemplates[i].Name == name {
			source = &seed.LibraryTemplates[i]
			break
		}
	}
	if source == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "Library template not found",
			"message": "No library template with name: " + name,
		})
	}

	createReq := &template.CreateRequest{
		AppID:       appID,
		Name:        source.Name,
		Description: source.Description,
		Channel:     source.Channel,
		Subject:     source.Subject,
		Body:        source.Body,
		Variables:   source.Variables,
		Metadata:    source.Metadata,
		Locale:      source.Locale,
		CreatedBy:   "library:clone",
	}

	tmpl, err := h.service.Create(c.Context(), createReq)
	if err != nil {
		h.logger.Error("Failed to clone library template",
			zap.String("name", name),
			zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to clone template",
			"message": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(toTemplateResponse(tmpl))
}

// RollbackTemplate creates a new version whose content is copied from a target version
func (h *TemplateHandler) RollbackTemplate(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Template ID is required",
			"message": "Please provide a valid template ID",
		})
	}

	var req dto.RollbackRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
	}

	if req.Version < 1 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid version",
			"message": "Version must be >= 1",
		})
	}

	tmpl, err := h.service.Rollback(c.Context(), id, appID, req.Version, req.UpdatedBy)
	if err != nil {
		h.logger.Error("Failed to rollback template",
			zap.String("id", id),
			zap.Int("version", req.Version),
			zap.Error(err))

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error":   "Not found",
				"message": err.Error(),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to rollback template",
			"message": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(toTemplateResponse(tmpl))
}

// DiffTemplate compares two versions of a template and returns field-level changes
func (h *TemplateHandler) DiffTemplate(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Template ID is required",
			"message": "Please provide a valid template ID",
		})
	}

	fromStr := c.Query("from", "")
	toStr := c.Query("to", "")
	if fromStr == "" || toStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Missing query parameters",
			"message": "Both 'from' and 'to' version numbers are required",
		})
	}

	fromVersion, err := strconv.Atoi(fromStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid 'from' version",
			"message": "Version must be an integer",
		})
	}
	toVersion, err := strconv.Atoi(toStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid 'to' version",
			"message": "Version must be an integer",
		})
	}

	// Fetch the template to get name/locale for the diff
	tmpl, err := h.service.GetByID(c.Context(), id, appID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "Template not found",
			"message": err.Error(),
		})
	}

	diff, err := h.service.Diff(c.Context(), appID, tmpl.Name, tmpl.Locale, fromVersion, toVersion)
	if err != nil {
		h.logger.Error("Failed to diff template",
			zap.String("id", id),
			zap.Int("from", fromVersion),
			zap.Int("to", toVersion),
			zap.Error(err))

		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error":   "Version not found",
				"message": err.Error(),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to diff template",
			"message": err.Error(),
		})
	}

	return c.JSON(diff)
}

// SendTest renders a template with sample data and sends a test email
func (h *TemplateHandler) SendTest(c *fiber.Ctx) error {
	if h.smtpProvider == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error":   "SMTP not configured",
			"message": "SMTP provider must be enabled to send test emails",
		})
	}

	appID := c.Locals("app_id").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Template ID is required",
			"message": "Please provide a valid template ID",
		})
	}

	var req dto.SendTestRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
	}

	if req.ToEmail == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "to_email is required",
			"message": "Please provide an email address to send the test to",
		})
	}

	tmpl, err := h.service.GetByID(c.Context(), id, appID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "Template not found",
			"message": err.Error(),
		})
	}

	// Use provided sample data, or fall back to template's built-in sample data
	sampleData := req.SampleData
	if sampleData == nil {
		if sd, ok := tmpl.Metadata["sample_data"].(map[string]interface{}); ok {
			sampleData = sd
		}
	}
	if sampleData == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "No sample data",
			"message": "Provide sample_data in request body or use a template with built-in sample data",
		})
	}

	rendered, err := h.service.Render(c.Context(), id, appID, sampleData)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to render template",
			"message": err.Error(),
		})
	}

	// Render subject line
	subject := fmt.Sprintf("[TEST] %s", h.service.RenderSubject(tmpl.Subject, sampleData))

	// Build a minimal notification + user for the SMTP provider
	testNotif := &notification.Notification{
		NotificationID: "test-" + id,
		AppID:          appID,
		Channel:        notification.ChannelEmail,
		Content: notification.Content{
			Title: subject,
			Body:  rendered,
		},
	}
	testUser := &user.User{
		Email: req.ToEmail,
	}

	if _, err := h.smtpProvider.Send(c.Context(), testNotif, testUser); err != nil {
		h.logger.Error("Failed to send test email",
			zap.String("template_id", id),
			zap.String("to", req.ToEmail),
			zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to send test email",
			"message": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"status":  "sent",
		"to":      req.ToEmail,
		"subject": subject,
	})
}

// ── Phase 6: Content Controls ──

// GetControls returns a template's control definitions and current values.
func (h *TemplateHandler) GetControls(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Template ID is required",
			"message": "Please provide a valid template ID",
		})
	}

	tmpl, err := h.service.GetByID(c.Context(), id, appID)
	if err != nil {
		h.logger.Error("Failed to get template for controls", zap.String("id", id), zap.Error(err))
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "Template not found",
			"message": err.Error(),
		})
	}

	controls := tmpl.Controls
	if controls == nil {
		controls = []template.TemplateControl{}
	}
	controlValues := tmpl.ControlValues
	if controlValues == nil {
		controlValues = template.ControlValues{}
	}

	return c.JSON(fiber.Map{
		"controls":       controls,
		"control_values": controlValues,
	})
}

// UpdateControls validates and saves control values for a template.
func (h *TemplateHandler) UpdateControls(c *fiber.Ctx) error {
	appID := c.Locals("app_id").(string)
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Template ID is required",
			"message": "Please provide a valid template ID",
		})
	}

	var values template.ControlValues
	if err := c.BodyParser(&values); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"message": err.Error(),
		})
	}

	tmpl, err := h.service.GetByID(c.Context(), id, appID)
	if err != nil {
		h.logger.Error("Failed to get template for control update", zap.String("id", id), zap.Error(err))
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "Template not found",
			"message": err.Error(),
		})
	}

	// Validate values against control schema
	if err := validateControlValues(tmpl.Controls, values); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Validation failed",
			"message": err.Error(),
		})
	}

	updateReq := &template.UpdateRequest{
		ControlValues: values,
	}
	updated, err := h.service.Update(c.Context(), id, appID, updateReq)
	if err != nil {
		h.logger.Error("Failed to update template controls", zap.String("id", id), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to update controls",
			"message": err.Error(),
		})
	}

	return c.JSON(toTemplateResponse(updated))
}

// validateControlValues checks that the provided values match the control schema.
func validateControlValues(controls []template.TemplateControl, values template.ControlValues) error {
	for _, ctrl := range controls {
		val, exists := values[ctrl.Key]
		if ctrl.Required && !exists {
			return fmt.Errorf("control '%s' is required", ctrl.Key)
		}
		if exists {
			switch ctrl.Type {
			case "url":
				if s, ok := val.(string); ok && s != "" {
					if _, err := url.Parse(s); err != nil {
						return fmt.Errorf("control '%s' must be a valid URL", ctrl.Key)
					}
				}
			case "color":
				if s, ok := val.(string); ok && s != "" {
					if matched, _ := regexp.MatchString(`^#[0-9A-Fa-f]{6}$`, s); !matched {
						return fmt.Errorf("control '%s' must be a hex color (#RRGGBB)", ctrl.Key)
					}
				}
			case "select":
				if s, ok := val.(string); ok && len(ctrl.Options) > 0 {
					found := false
					for _, opt := range ctrl.Options {
						if opt == s {
							found = true
							break
						}
					}
					if !found {
						return fmt.Errorf("control '%s' must be one of: %v", ctrl.Key, ctrl.Options)
					}
				}
			}
		}
	}
	return nil
}

// Helper function to convert template to response
func toTemplateResponse(tmpl *template.Template) *dto.TemplateResponse {
	return &dto.TemplateResponse{
		ID:            tmpl.ID,
		AppID:         tmpl.AppID,
		Name:          tmpl.Name,
		Description:   tmpl.Description,
		Channel:       tmpl.Channel,
		WebhookTarget: tmpl.WebhookTarget,
		Subject:       tmpl.Subject,
		Body:          tmpl.Body,
		Variables:     tmpl.Variables,
		Metadata:      tmpl.Metadata,
		Controls:      tmpl.Controls,
		ControlValues: tmpl.ControlValues,
		Version:       tmpl.Version,
		Status:        tmpl.Status,
		Locale:        tmpl.Locale,
		CreatedBy:     tmpl.CreatedBy,
		UpdatedBy:     tmpl.UpdatedBy,
		CreatedAt:     tmpl.CreatedAt,
		UpdatedAt:     tmpl.UpdatedAt,
	}
}
