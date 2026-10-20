package handlers

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// WhatsAppRichTemplateHandler is the HTTP surface for the FRN-internal rich
// template store (`/v1/whatsapp/rich-templates`). Differs from
// WhatsAppTemplateHandler which is a thin shim over Meta Graph API for the
// legacy templates collection — this handler operates on the typed
// RichTemplate authored in FRN and submitted to providers behind the scenes.
// RichDraftUpdateRequest is the existing revision-fenced draft body.
type RichDraftUpdateRequest struct {
	Template whatsapp.RichTemplate `json:"template"`
	Revision string                `json:"revision"`
}
type RichDraftSubmitRequest struct {
	Revision string `json:"revision"`
}

type WhatsAppRichTemplateHandler struct {
	svc    services.WhatsAppRichTemplateService
	logger *zap.Logger
}

func channelTemplateContext(c *fiber.Ctx) context.Context {
	env, _ := c.Locals("environment_id").(string)
	return channelconnection.WithEnvironment(c.Context(), env)
}

// @Summary Create Draft
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags WhatsAppRichTemplate
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/whatsapp/rich-templates/drafts [post]
// @Accept json
// @Param body body whatsapp.RichTemplate true "Request body"
func (h *WhatsAppRichTemplateHandler) CreateDraft(c *fiber.Ctx) error {
	appID, _ := c.Locals("app_id").(string)
	if appID == "" {
		return pkgerrors.Unauthorized("Application required")
	}
	var body whatsapp.RichTemplate
	if c.BodyParser(&body) != nil {
		return pkgerrors.BadRequest("Invalid draft")
	}
	body.AppID = appID
	svc, ok := h.svc.(services.ChannelTemplateDraftService)
	if !ok {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Draft authoring unavailable")
	}
	tpl, err := svc.CreateDraft(channelTemplateContext(c), &body)
	if err != nil {
		return connectionHTTPError(err)
	}
	return c.Status(201).JSON(fiber.Map{"success": true, "data": tpl})
}

// @Summary Update Draft
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags WhatsAppRichTemplate
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/whatsapp/rich-templates/{id}/draft [put]
// @Accept json
// @Param body body RichDraftUpdateRequest true "Revision-fenced draft request"
func (h *WhatsAppRichTemplateHandler) UpdateDraft(c *fiber.Ctx) error {
	appID, _ := c.Locals("app_id").(string)
	if appID == "" {
		return pkgerrors.Unauthorized("Application required")
	}
	var body RichDraftUpdateRequest
	if c.BodyParser(&body) != nil || body.Revision == "" {
		return pkgerrors.BadRequest("Draft revision required")
	}
	svc, ok := h.svc.(services.ChannelTemplateDraftService)
	if !ok {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Draft authoring unavailable")
	}
	tpl, err := svc.UpdateDraft(channelTemplateContext(c), appID, c.Params("id"), body.Revision, &body.Template)
	if err != nil {
		return connectionHTTPError(err)
	}
	return c.JSON(fiber.Map{"success": true, "data": tpl})
}

// @Summary Submit Draft
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags WhatsAppRichTemplate
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/whatsapp/rich-templates/{id}/submit [post]
// @Accept json
// @Param body body RichDraftSubmitRequest true "Revision-fenced draft request"
func (h *WhatsAppRichTemplateHandler) SubmitDraft(c *fiber.Ctx) error {
	appID, _ := c.Locals("app_id").(string)
	if appID == "" {
		return pkgerrors.Unauthorized("Application required")
	}
	var body RichDraftSubmitRequest
	if c.BodyParser(&body) != nil || body.Revision == "" {
		return pkgerrors.BadRequest("Draft revision required")
	}
	svc, ok := h.svc.(services.ChannelTemplateDraftService)
	if !ok {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Draft authoring unavailable")
	}
	tpl, err := svc.SubmitDraft(channelTemplateContext(c), appID, c.Params("id"), body.Revision)
	if err != nil {
		return connectionHTTPError(err)
	}
	return c.JSON(fiber.Map{"success": true, "data": tpl})
}

// NewWhatsAppRichTemplateHandler wires the handler. The service handles all
// provider-side I/O, validation, and persistence so this layer stays small
// and focused on request parsing and response shaping.
func NewWhatsAppRichTemplateHandler(svc services.WhatsAppRichTemplateService, logger *zap.Logger) *WhatsAppRichTemplateHandler {
	return &WhatsAppRichTemplateHandler{svc: svc, logger: logger}
}

// Create handles POST /v1/whatsapp/rich-templates.
//
// Body shape mirrors whatsapp.RichTemplate (minus IDs / timestamps the
// service stamps). Validation errors are returned as 400 with the full
// field-level error list so the UI can highlight specific fields.
// @Summary Create
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags WhatsAppRichTemplate
// @Produce json
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/whatsapp/rich-templates [post]
// @Accept json
// @Param body body whatsapp.RichTemplate true "Request body"
func (h *WhatsAppRichTemplateHandler) Create(c *fiber.Ctx) error {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return pkgerrors.Unauthorized("Missing app context")
	}

	var body whatsapp.RichTemplate
	if err := c.BodyParser(&body); err != nil {
		return pkgerrors.BadRequest("Invalid JSON body: " + err.Error())
	}
	// Always enforce the app context from the API key — never trust the
	// body. Tenant scoping comes from the application record.
	body.AppID = appID
	if tenantID, ok := c.Locals("tenant_id").(string); ok {
		body.TenantID = tenantID
	}

	tpl, err := h.svc.Create(channelTemplateContext(c), &body)
	if err != nil {
		if verrs, isValidation := err.(whatsapp.ValidationErrors); isValidation {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   "Template validation failed",
				"details": verrs,
			})
		}
		return pkgerrors.New(pkgerrors.ErrCodeInternal, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": tpl})
}

// List handles GET /v1/whatsapp/rich-templates with optional query filters:
//   kind, status, name_prefix, limit, offset.
// app_id is enforced from the API key context.
// @Summary List
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags WhatsAppRichTemplate
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/whatsapp/rich-templates [get]
func (h *WhatsAppRichTemplateHandler) List(c *fiber.Ctx) error {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return pkgerrors.Unauthorized("Missing app context")
	}
	filter := whatsapp.RichTemplateFilter{
		AppID:      appID,
		Kind:       c.Query("kind"),
		Status:     c.Query("status"),
		NamePrefix: c.Query("name_prefix"),
		Limit:      c.QueryInt("limit"),
		Offset:     c.QueryInt("offset"),
	}
	if tenantID, ok := c.Locals("tenant_id").(string); ok {
		filter.TenantID = tenantID
	}
	tpls, total, err := h.svc.List(channelTemplateContext(c), filter)
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, err.Error())
	}
	return c.JSON(fiber.Map{"success": true, "data": tpls, "total": total})
}

// Get handles GET /v1/whatsapp/rich-templates/:id.
// @Summary Get
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags WhatsAppRichTemplate
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/whatsapp/rich-templates/{id} [get]
func (h *WhatsAppRichTemplateHandler) Get(c *fiber.Ctx) error {
	appID, id, err := h.scopedID(c)
	if err != nil {
		return err
	}
	tpl, err := h.svc.Get(channelTemplateContext(c), appID, id)
	if err != nil {
		return pkgerrors.NotFound("rich_template", id)
	}
	return c.JSON(fiber.Map{"success": true, "data": tpl})
}

// Delete handles DELETE /v1/whatsapp/rich-templates/:id.
// @Summary Delete
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags WhatsAppRichTemplate
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/whatsapp/rich-templates/{id} [delete]
func (h *WhatsAppRichTemplateHandler) Delete(c *fiber.Ctx) error {
	appID, id, err := h.scopedID(c)
	if err != nil {
		return err
	}
	if err := h.svc.Delete(channelTemplateContext(c), appID, id); err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, err.Error())
	}
	return c.JSON(fiber.Map{"success": true, "message": "Template deleted"})
}

// Sync handles POST /v1/whatsapp/rich-templates/:id/sync — explicit pull of
// Meta's current approval state. Cheap fallback for when the webhook is not
// reachable during local development.
// @Summary Sync
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags WhatsAppRichTemplate
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/whatsapp/rich-templates/{id}/sync [post]
func (h *WhatsAppRichTemplateHandler) Sync(c *fiber.Ctx) error {
	appID, id, err := h.scopedID(c)
	if err != nil {
		return err
	}
	tpl, err := h.svc.SyncApproval(channelTemplateContext(c), appID, id)
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, err.Error())
	}
	return c.JSON(fiber.Map{"success": true, "data": tpl})
}

// Preview handles GET /v1/whatsapp/rich-templates/:id/preview — returns the
// authoring JSON the service would submit to Meta. Used by the UI side-by-
// side renderer so authors see exactly what will land at Meta before they
// commit to the submission round-trip.
// @Summary Preview
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags WhatsAppRichTemplate
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/whatsapp/rich-templates/{id}/preview [post]
func (h *WhatsAppRichTemplateHandler) Preview(c *fiber.Ctx) error {
	appID, id, err := h.scopedID(c)
	if err != nil {
		return err
	}
	// Optional variables — UI passes them via JSON body for richer preview.
	var body struct {
		Variables map[string]string `json:"variables"`
	}
	_ = c.BodyParser(&body)

	preview, err := h.svc.Preview(channelTemplateContext(c), appID, id, body.Variables)
	if err != nil {
		return pkgerrors.NotFound("rich_template", id)
	}
	return c.JSON(fiber.Map{"success": true, "data": preview})
}

// scopedID is the common preamble: pull app_id from the API key context and
// :id from the URL, returning an Unauthorized when the context is missing.
// Keeps every handler method 2 lines shorter and consistent in error shape.
func (h *WhatsAppRichTemplateHandler) scopedID(c *fiber.Ctx) (string, string, error) {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return "", "", pkgerrors.Unauthorized("Missing app context")
	}
	id := c.Params("id")
	if id == "" {
		return "", "", pkgerrors.BadRequest("template id is required")
	}
	return appID, id, nil
}
