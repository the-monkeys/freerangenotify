package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/sse"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

const twilioContentAPIBase = "https://content.twilio.com"

// TwilioTemplateHandler manages Twilio Content Templates via the Twilio Content API.
type TwilioTemplateHandler struct {
	sseBroadcaster *sse.Broadcaster
	accountSID     string
	authToken      string
	logger         *zap.Logger
}

// NewTwilioTemplateHandler creates a new Twilio template handler.
func NewTwilioTemplateHandler(
	sseBroadcaster *sse.Broadcaster,
	accountSID, authToken string,
	logger *zap.Logger,
) *TwilioTemplateHandler {
	return &TwilioTemplateHandler{
		sseBroadcaster: sseBroadcaster,
		accountSID:     accountSID,
		authToken:      authToken,
		logger:         logger,
	}
}

// CreateTemplate handles POST /v1/twilio/templates
// Creates a new Twilio Content Template.
func (h *TwilioTemplateHandler) CreateTemplate(c *fiber.Ctx) error {
	var req dto.CreateTwilioTemplateRequest
	if err := c.BodyParser(&req); err != nil {
		return pkgerrors.BadRequest("Invalid JSON body")
	}
	if req.FriendlyName == "" {
		return pkgerrors.BadRequest("friendly_name is required")
	}
	if req.Types == nil || len(req.Types) == 0 {
		return pkgerrors.BadRequest("types is required (e.g. twilio/text, twilio/quick-reply)")
	}
	if req.Language == "" {
		req.Language = "en"
	}

	body := map[string]interface{}{
		"friendly_name": req.FriendlyName,
		"language":      req.Language,
		"types":         req.Types,
	}
	if req.Variables != nil {
		body["variables"] = req.Variables
	}

	payload, _ := json.Marshal(body)

	resp, err := h.twilioRequest(c.Context(), http.MethodPost, "/v1/Content", strings.NewReader(string(payload)))
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Twilio Content API request failed: "+err.Error())
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		h.logger.Error("Twilio template creation failed",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(respBody)))
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"error":   "Twilio Content API error",
			"details": json.RawMessage(respBody),
		})
	}

	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)

	h.logger.Info("Twilio content template created",
		zap.String("friendly_name", req.FriendlyName),
		zap.String("sid", fmt.Sprintf("%v", result["sid"])))

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

// ListTemplates handles GET /v1/twilio/templates
// Lists all Twilio Content Templates with their approval status.
func (h *TwilioTemplateHandler) ListTemplates(c *fiber.Ctx) error {
	pageSize := c.Query("page_size", "50")
	search := c.Query("search")

	path := "/v1/ContentAndApprovals?PageSize=" + pageSize
	if search != "" {
		path += "&Content=" + search
	}

	resp, err := h.twilioRequest(c.Context(), http.MethodGet, path, nil)
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Twilio Content API request failed: "+err.Error())
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"error":   "Twilio Content API error",
			"details": json.RawMessage(respBody),
		})
	}

	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)

	return c.JSON(fiber.Map{"success": true, "data": result})
}

// GetTemplate handles GET /v1/twilio/templates/:content_sid
// Fetches a single Twilio Content Template by its SID.
func (h *TwilioTemplateHandler) GetTemplate(c *fiber.Ctx) error {
	contentSid := c.Params("content_sid")
	if contentSid == "" {
		return pkgerrors.BadRequest("content_sid is required")
	}

	resp, err := h.twilioRequest(c.Context(), http.MethodGet, "/v1/Content/"+contentSid, nil)
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Twilio Content API request failed: "+err.Error())
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"error":   "Twilio Content API error",
			"details": json.RawMessage(respBody),
		})
	}

	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)

	return c.JSON(fiber.Map{"success": true, "data": result})
}

// UpdateTemplate handles PUT /v1/twilio/templates/:content_sid
// Updates a Twilio Content Template (only before WhatsApp approval submission).
func (h *TwilioTemplateHandler) UpdateTemplate(c *fiber.Ctx) error {
	contentSid := c.Params("content_sid")
	if contentSid == "" {
		return pkgerrors.BadRequest("content_sid is required")
	}

	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil {
		return pkgerrors.BadRequest("Invalid JSON body")
	}

	payload, _ := json.Marshal(body)

	resp, err := h.twilioRequest(c.Context(), http.MethodPut, "/v1/Content/"+contentSid, strings.NewReader(string(payload)))
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Twilio Content API request failed: "+err.Error())
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"error":   "Twilio Content API error",
			"details": json.RawMessage(respBody),
		})
	}

	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)

	h.logger.Info("Twilio content template updated", zap.String("content_sid", contentSid))

	return c.JSON(fiber.Map{"success": true, "data": result})
}

// DeleteTemplate handles DELETE /v1/twilio/templates/:content_sid
// Deletes a Twilio Content Template.
func (h *TwilioTemplateHandler) DeleteTemplate(c *fiber.Ctx) error {
	contentSid := c.Params("content_sid")
	if contentSid == "" {
		return pkgerrors.BadRequest("content_sid is required")
	}

	resp, err := h.twilioRequest(c.Context(), http.MethodDelete, "/v1/Content/"+contentSid, nil)
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Twilio Content API request failed: "+err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"error":   "Twilio Content API error",
			"details": json.RawMessage(respBody),
		})
	}

	h.logger.Info("Twilio content template deleted", zap.String("content_sid", contentSid))

	return c.JSON(fiber.Map{"success": true, "message": "Template deleted"})
}

// SubmitApproval handles POST /v1/twilio/templates/:content_sid/approve
// Submits a template for WhatsApp approval.
func (h *TwilioTemplateHandler) SubmitApproval(c *fiber.Ctx) error {
	contentSid := c.Params("content_sid")
	if contentSid == "" {
		return pkgerrors.BadRequest("content_sid is required")
	}

	var req dto.SubmitTwilioApprovalRequest
	if err := c.BodyParser(&req); err != nil {
		return pkgerrors.BadRequest("Invalid JSON body")
	}
	if req.Name == "" {
		return pkgerrors.BadRequest("name is required (lowercase, alphanumeric, underscores only)")
	}
	if req.Category == "" {
		return pkgerrors.BadRequest("category is required (UTILITY, MARKETING, or AUTHENTICATION)")
	}

	body := map[string]string{
		"name":     req.Name,
		"category": req.Category,
	}
	payload, _ := json.Marshal(body)

	path := fmt.Sprintf("/v1/Content/%s/ApprovalRequests/whatsapp", contentSid)
	resp, err := h.twilioRequest(c.Context(), http.MethodPost, path, strings.NewReader(string(payload)))
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Twilio Approval API request failed: "+err.Error())
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		h.logger.Error("Twilio approval submission failed",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(respBody)))
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"error":   "Twilio Approval API error",
			"details": json.RawMessage(respBody),
		})
	}

	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)

	h.logger.Info("Twilio template submitted for WhatsApp approval",
		zap.String("content_sid", contentSid),
		zap.String("name", req.Name),
		zap.String("category", req.Category))

	return c.JSON(fiber.Map{"success": true, "data": result})
}

// GetApprovalStatus handles GET /v1/twilio/templates/:content_sid/approval
// Fetches the current WhatsApp approval status.
func (h *TwilioTemplateHandler) GetApprovalStatus(c *fiber.Ctx) error {
	contentSid := c.Params("content_sid")
	if contentSid == "" {
		return pkgerrors.BadRequest("content_sid is required")
	}

	path := fmt.Sprintf("/v1/Content/%s/ApprovalRequests", contentSid)
	resp, err := h.twilioRequest(c.Context(), http.MethodGet, path, nil)
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Twilio Approval API request failed: "+err.Error())
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"error":   "Twilio Approval API error",
			"details": json.RawMessage(respBody),
		})
	}

	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)

	return c.JSON(fiber.Map{"success": true, "data": result})
}

// SyncTemplate handles POST /v1/twilio/templates/:content_sid/sync
// Fetches latest approval status and broadcasts via SSE.
func (h *TwilioTemplateHandler) SyncTemplate(c *fiber.Ctx) error {
	contentSid := c.Params("content_sid")
	if contentSid == "" {
		return pkgerrors.BadRequest("content_sid is required")
	}

	// Fetch approval status
	path := fmt.Sprintf("/v1/Content/%s/ApprovalRequests", contentSid)
	resp, err := h.twilioRequest(c.Context(), http.MethodGet, path, nil)
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Twilio Approval API request failed: "+err.Error())
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"error":   "Twilio Approval API error",
			"details": json.RawMessage(respBody),
		})
	}

	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)

	// Extract WhatsApp approval status for SSE broadcast
	status := "unknown"
	name := ""
	if wa, ok := result["whatsapp"].(map[string]interface{}); ok {
		if s, ok := wa["status"].(string); ok {
			status = s
		}
		if n, ok := wa["name"].(string); ok {
			name = n
		}
	}

	if h.sseBroadcaster != nil {
		_ = h.sseBroadcaster.PublishMessage(&sse.SSEMessage{
			Type: "twilio_template_status",
			Data: map[string]interface{}{
				"content_sid": contentSid,
				"name":        name,
				"status":      status,
			},
		})
	}

	h.logger.Info("Twilio template synced",
		zap.String("content_sid", contentSid),
		zap.String("name", name),
		zap.String("status", status))

	return c.JSON(fiber.Map{"success": true, "data": result})
}

// PreviewTemplate handles POST /v1/twilio/templates/:content_sid/preview
// Fetches the template and renders it with provided variable values.
func (h *TwilioTemplateHandler) PreviewTemplate(c *fiber.Ctx) error {
	contentSid := c.Params("content_sid")
	if contentSid == "" {
		return pkgerrors.BadRequest("content_sid is required")
	}

	var req dto.PreviewTwilioTemplateRequest
	if err := c.BodyParser(&req); err != nil {
		return pkgerrors.BadRequest("Invalid JSON body")
	}
	if req.Variables == nil {
		req.Variables = make(map[string]string)
	}

	// Fetch template from Twilio
	resp, err := h.twilioRequest(c.Context(), http.MethodGet, "/v1/Content/"+contentSid, nil)
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Twilio Content API request failed: "+err.Error())
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return c.Status(resp.StatusCode).JSON(fiber.Map{
			"success": false,
			"error":   "Twilio Content API error",
			"details": json.RawMessage(respBody),
		})
	}

	var template map[string]interface{}
	_ = json.Unmarshal(respBody, &template)

	// Render types with variable substitution
	types, _ := template["types"].(map[string]interface{})
	rendered := renderTypesMap(types, req.Variables)

	friendlyName, _ := template["friendly_name"].(string)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"content_sid":   contentSid,
			"friendly_name": friendlyName,
			"original":      types,
			"rendered":      rendered,
		},
	})
}

// twilioRequest executes an authenticated HTTP request to the Twilio Content API.
func (h *TwilioTemplateHandler) twilioRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := twilioContentAPIBase + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.SetBasicAuth(h.accountSID, h.authToken)
	req.Header.Set("Content-Type", "application/json")

	return (&http.Client{Timeout: 15 * time.Second}).Do(req)
}

// renderTypesMap recursively walks the types structure and renders all string values.
func renderTypesMap(types map[string]interface{}, vars map[string]string) map[string]interface{} {
	if types == nil {
		return nil
	}
	rendered := make(map[string]interface{}, len(types))
	for key, val := range types {
		switch v := val.(type) {
		case string:
			rendered[key] = renderVariables(v, vars)
		case map[string]interface{}:
			rendered[key] = renderTypesMap(v, vars)
		case []interface{}:
			rendered[key] = renderTypesSlice(v, vars)
		default:
			rendered[key] = val
		}
	}
	return rendered
}

// renderTypesSlice recursively renders string values in a slice.
func renderTypesSlice(items []interface{}, vars map[string]string) []interface{} {
	rendered := make([]interface{}, len(items))
	for i, item := range items {
		switch v := item.(type) {
		case string:
			rendered[i] = renderVariables(v, vars)
		case map[string]interface{}:
			rendered[i] = renderTypesMap(v, vars)
		case []interface{}:
			rendered[i] = renderTypesSlice(v, vars)
		default:
			rendered[i] = item
		}
	}
	return rendered
}

// renderVariables replaces Twilio numbered placeholders ({{1}}, {{2}}) with provided values.
func renderVariables(text string, vars map[string]string) string {
	result := text
	for k, v := range vars {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
		result = strings.ReplaceAll(result, "{{ "+k+" }}", v)
	}
	return result
}
