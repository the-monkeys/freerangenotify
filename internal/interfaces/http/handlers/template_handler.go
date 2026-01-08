package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

var validateTemplate = validator.New()

type TemplateHandler struct {
	service *usecases.TemplateService
	logger  *zap.Logger
}

func NewTemplateHandler(service *usecases.TemplateService, logger *zap.Logger) *TemplateHandler {
	return &TemplateHandler{
		service: service,
		logger:  logger,
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

// DeleteTemplate deletes a template (soft delete)
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
		locale = "en-US"
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

	// Get locale from query params, default to en-US
	locale := c.Query("locale", "en-US")

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
		Version:       tmpl.Version,
		Status:        tmpl.Status,
		Locale:        tmpl.Locale,
		CreatedBy:     tmpl.CreatedBy,
		UpdatedBy:     tmpl.UpdatedBy,
		CreatedAt:     tmpl.CreatedAt,
		UpdatedAt:     tmpl.UpdatedAt,
	}
}
