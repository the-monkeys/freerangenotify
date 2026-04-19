package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/sse"
	pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// WhatsAppTemplateHandler manages WhatsApp message templates via Meta Graph API.
type WhatsAppTemplateHandler struct {
	appRepo        application.Repository
	sseBroadcaster *sse.Broadcaster
	metaAPIVersion string
	logger         *zap.Logger
}

// NewWhatsAppTemplateHandler creates a new WhatsApp template handler.
func NewWhatsAppTemplateHandler(
	appRepo application.Repository,
	sseBroadcaster *sse.Broadcaster,
	metaAPIVersion string,
	logger *zap.Logger,
) *WhatsAppTemplateHandler {
	if metaAPIVersion == "" {
		metaAPIVersion = "v23.0"
	}
	return &WhatsAppTemplateHandler{
		appRepo:        appRepo,
		sseBroadcaster: sseBroadcaster,
		metaAPIVersion: metaAPIVersion,
		logger:         logger,
	}
}

// CreateTemplate handles POST /v1/whatsapp/templates
// Submits a new template to Meta for approval.
func (h *WhatsAppTemplateHandler) CreateTemplate(c *fiber.Ctx) error {
	wa, appID, err := h.resolveMetaConfig(c)
	if err != nil {
		return err
	}

	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil {
		return pkgerrors.BadRequest("Invalid JSON body")
	}

	name, _ := body["name"].(string)
	if name == "" {
		return pkgerrors.BadRequest("name is required")
	}
	category, _ := body["category"].(string)
	if category == "" {
		return pkgerrors.BadRequest("category is required (AUTHENTICATION, MARKETING, or UTILITY)")
	}
	language, _ := body["language"].(string)
	if language == "" {
		body["language"] = "en_US"
	}

	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/message_templates",
		h.metaAPIVersion, wa.MetaWABAID)

	payload, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(c.Context(), http.MethodPost, apiURL, strings.NewReader(string(payload)))
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+wa.MetaAccessToken)

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Meta API request failed: "+err.Error())
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		h.logger.Error("Meta template creation failed",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(respBody)))
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"error":   "Meta API error",
			"details": json.RawMessage(respBody),
		})
	}

	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)

	h.logger.Info("WhatsApp template created",
		zap.String("app_id", appID),
		zap.String("name", name),
		zap.String("category", category))

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

// ListTemplates handles GET /v1/whatsapp/templates
// Lists all templates for the app's WABA with their approval status.
func (h *WhatsAppTemplateHandler) ListTemplates(c *fiber.Ctx) error {
	wa, _, err := h.resolveMetaConfig(c)
	if err != nil {
		return err
	}

	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/message_templates",
		h.metaAPIVersion, wa.MetaWABAID)

	params := url.Values{}
	params.Set("access_token", wa.MetaAccessToken)
	params.Set("limit", "100")

	if nameFilter := c.Query("name"); nameFilter != "" {
		params.Set("name", nameFilter)
	}
	if statusFilter := c.Query("status"); statusFilter != "" {
		params.Set("status", statusFilter)
	}

	resp, err := http.Get(apiURL + "?" + params.Encode())
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Meta API request failed: "+err.Error())
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"error":   "Meta API error",
			"details": json.RawMessage(body),
		})
	}

	var result map[string]interface{}
	_ = json.Unmarshal(body, &result)

	return c.JSON(fiber.Map{"success": true, "data": result})
}

// GetTemplate handles GET /v1/whatsapp/templates/:name
// Gets details for a specific template by name.
func (h *WhatsAppTemplateHandler) GetTemplate(c *fiber.Ctx) error {
	wa, _, err := h.resolveMetaConfig(c)
	if err != nil {
		return err
	}

	name := c.Params("name")
	if name == "" {
		return pkgerrors.BadRequest("template name is required")
	}

	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/message_templates",
		h.metaAPIVersion, wa.MetaWABAID)

	params := url.Values{}
	params.Set("access_token", wa.MetaAccessToken)
	params.Set("name", name)

	resp, err := http.Get(apiURL + "?" + params.Encode())
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Meta API request failed: "+err.Error())
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"error":   "Meta API error",
			"details": json.RawMessage(body),
		})
	}

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	_ = json.Unmarshal(body, &result)

	if len(result.Data) == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Template not found: " + name,
		})
	}

	return c.JSON(fiber.Map{"success": true, "data": result.Data[0]})
}

// DeleteTemplate handles DELETE /v1/whatsapp/templates/:name
func (h *WhatsAppTemplateHandler) DeleteTemplate(c *fiber.Ctx) error {
	wa, appID, err := h.resolveMetaConfig(c)
	if err != nil {
		return err
	}

	name := c.Params("name")
	if name == "" {
		return pkgerrors.BadRequest("template name is required")
	}

	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/message_templates",
		h.metaAPIVersion, wa.MetaWABAID)

	params := url.Values{}
	params.Set("access_token", wa.MetaAccessToken)
	params.Set("name", name)

	req, err := http.NewRequestWithContext(c.Context(), http.MethodDelete, apiURL+"?"+params.Encode(), nil)
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to create request")
	}

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Meta API request failed: "+err.Error())
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"error":   "Meta API error",
			"details": json.RawMessage(body),
		})
	}

	h.logger.Info("WhatsApp template deleted",
		zap.String("app_id", appID),
		zap.String("name", name))

	return c.JSON(fiber.Map{"success": true, "message": "Template deleted"})
}

// SyncTemplate handles POST /v1/whatsapp/templates/:name/sync
// Force-refreshes template status from Meta.
func (h *WhatsAppTemplateHandler) SyncTemplate(c *fiber.Ctx) error {
	wa, appID, err := h.resolveMetaConfig(c)
	if err != nil {
		return err
	}

	name := c.Params("name")
	if name == "" {
		return pkgerrors.BadRequest("template name is required")
	}

	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/message_templates",
		h.metaAPIVersion, wa.MetaWABAID)

	params := url.Values{}
	params.Set("access_token", wa.MetaAccessToken)
	params.Set("name", name)

	resp, err := http.Get(apiURL + "?" + params.Encode())
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Meta API request failed: "+err.Error())
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"error":   "Meta API error",
			"details": json.RawMessage(body),
		})
	}

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	_ = json.Unmarshal(body, &result)

	if len(result.Data) == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "Template not found: " + name,
		})
	}

	tpl := result.Data[0]
	status, _ := tpl["status"].(string)

	if h.sseBroadcaster != nil {
		_ = h.sseBroadcaster.PublishMessage(&sse.SSEMessage{
			Type: "template_status",
			Data: map[string]interface{}{
				"app_id": appID,
				"name":   name,
				"status": status,
			},
		})
	}

	h.logger.Info("WhatsApp template synced",
		zap.String("app_id", appID),
		zap.String("name", name),
		zap.String("status", status))

	return c.JSON(fiber.Map{"success": true, "data": tpl})
}

// HandleTemplateStatusWebhook processes template status update webhooks from Meta.
// Called from MetaWebhookHandler when a message_template_status_update event arrives.
func (h *WhatsAppTemplateHandler) HandleTemplateStatusWebhook(event map[string]interface{}) {
	name, _ := event["message_template_name"].(string)
	status, _ := event["event"].(string)
	reason, _ := event["reason"].(string)

	h.logger.Info("Template status webhook received",
		zap.String("template", name),
		zap.String("status", status),
		zap.String("reason", reason))

	if h.sseBroadcaster != nil {
		_ = h.sseBroadcaster.PublishMessage(&sse.SSEMessage{
			Type: "template_status",
			Data: map[string]interface{}{
				"name":   name,
				"status": status,
				"reason": reason,
			},
		})
	}
}

// resolveMetaConfig extracts app_id from the API key context and validates Meta WhatsApp config.
func (h *WhatsAppTemplateHandler) resolveMetaConfig(c *fiber.Ctx) (*application.WhatsAppAppConfig, string, error) {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return nil, "", pkgerrors.Unauthorized("Missing app context")
	}

	app, err := h.appRepo.GetByID(c.Context(), appID)
	if err != nil {
		return nil, "", pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to load application")
	}

	wa := app.Settings.WhatsApp
	if wa == nil || wa.Provider != "meta" || wa.MetaAccessToken == "" || wa.MetaWABAID == "" {
		return nil, "", pkgerrors.BadRequest(
			"WhatsApp Meta is not configured for this app. Connect via Embedded Signup or set Meta credentials in app settings.")
	}

	return wa, appID, nil
}
