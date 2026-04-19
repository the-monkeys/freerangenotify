package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// WhatsAppConversationHandler provides conversation inbox endpoints.
type WhatsAppConversationHandler struct {
	whatsappSvc whatsapp.Service
	logger      *zap.Logger
}

// NewWhatsAppConversationHandler creates a new conversation handler.
func NewWhatsAppConversationHandler(svc whatsapp.Service, logger *zap.Logger) *WhatsAppConversationHandler {
	return &WhatsAppConversationHandler{whatsappSvc: svc, logger: logger}
}

// ListConversations handles GET /v1/whatsapp/conversations
func (h *WhatsAppConversationHandler) ListConversations(c *fiber.Ctx) error {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return pkgerrors.Unauthorized("Missing app context")
	}

	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	conversations, total, err := h.whatsappSvc.ListConversations(c.Context(), appID, limit, offset)
	if err != nil {
		h.logger.Error("Failed to list conversations", zap.String("app_id", appID), zap.Error(err))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to list conversations")
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    conversations,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// GetMessages handles GET /v1/whatsapp/conversations/:contact_id/messages
func (h *WhatsAppConversationHandler) GetMessages(c *fiber.Ctx) error {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return pkgerrors.Unauthorized("Missing app context")
	}

	contactID := c.Params("contact_id")
	if contactID == "" {
		return pkgerrors.BadRequest("contact_id is required")
	}

	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	messages, total, err := h.whatsappSvc.ListMessages(c.Context(), &whatsapp.MessageFilter{
		AppID:       appID,
		ContactWAID: contactID,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		h.logger.Error("Failed to list messages", zap.String("contact", contactID), zap.Error(err))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to list messages")
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    messages,
		"total":   total,
	})
}

// replyRequest is the payload for the reply endpoint.
type replyRequest struct {
	Text         string `json:"text"`
	TemplateName string `json:"template_name"`
}

// Reply handles POST /v1/whatsapp/conversations/:contact_id/reply
func (h *WhatsAppConversationHandler) Reply(c *fiber.Ctx) error {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return pkgerrors.Unauthorized("Missing app context")
	}

	contactID := c.Params("contact_id")
	if contactID == "" {
		return pkgerrors.BadRequest("contact_id is required")
	}

	var req replyRequest
	if err := c.BodyParser(&req); err != nil {
		return pkgerrors.BadRequest("Invalid JSON body")
	}
	if req.Text == "" && req.TemplateName == "" {
		return pkgerrors.BadRequest("Either text or template_name is required")
	}

	if err := h.whatsappSvc.Reply(c.Context(), appID, contactID, req.Text, req.TemplateName); err != nil {
		h.logger.Error("Reply failed",
			zap.String("app_id", appID),
			zap.String("contact", contactID),
			zap.Error(err))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, err.Error())
	}

	return c.JSON(fiber.Map{"success": true, "message": "Reply sent"})
}

// MarkRead handles POST /v1/whatsapp/conversations/:contact_id/read
func (h *WhatsAppConversationHandler) MarkRead(c *fiber.Ctx) error {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return pkgerrors.Unauthorized("Missing app context")
	}

	contactID := c.Params("contact_id")
	if contactID == "" {
		return pkgerrors.BadRequest("contact_id is required")
	}

	if err := h.whatsappSvc.MarkRead(c.Context(), appID, contactID); err != nil {
		h.logger.Error("Mark read failed", zap.String("contact", contactID), zap.Error(err))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "Failed to mark as read")
	}

	return c.JSON(fiber.Map{"success": true, "message": "Messages marked as read"})
}
