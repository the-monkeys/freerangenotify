package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

// MetaWebhookHandler handles Meta WhatsApp Cloud API webhooks.
type MetaWebhookHandler struct {
	senderResolver  *services.ChannelSenderResolver
	whatsappSvc     whatsapp.Service
	appRepo         application.Repository
	templateHandler *WhatsAppTemplateHandler             // optional, legacy template status webhooks
	richTemplateSvc services.WhatsAppRichTemplateService // optional, rich-template status webhooks
	appSecret       string                               // Meta App Secret for X-Hub-Signature-256 verification
	verifyToken     string                               // hub.verify_token for webhook subscription registration
	logger          *zap.Logger
}

// SetTemplateHandler wires the template handler for processing template status webhooks.
func (h *MetaWebhookHandler) SetTemplateHandler(th *WhatsAppTemplateHandler) {
	h.templateHandler = th
}

// SetRichTemplateService wires the rich-template service so async
// message_template_status_update events from Meta update the FRN-side
// MetaBinding without polling. Optional — if unset, only the legacy
// templateHandler receives the event.
func (h *MetaWebhookHandler) SetRichTemplateService(svc services.WhatsAppRichTemplateService) {
	h.richTemplateSvc = svc
}

// NewMetaWebhookHandler creates a new Meta webhook handler.
func NewMetaWebhookHandler(
	whatsappSvc whatsapp.Service,
	appRepo application.Repository,
	appSecret string,
	verifyToken string,
	logger *zap.Logger,
) *MetaWebhookHandler {
	return &MetaWebhookHandler{
		whatsappSvc:    whatsappSvc,
		senderResolver: services.NewChannelSenderResolver(appRepo, nil),
		appRepo:        appRepo,
		appSecret:      appSecret,
		verifyToken:    verifyToken,
		logger:         logger,
	}
}

// VerifyWebhook handles GET /v1/webhooks/meta/whatsapp — Meta webhook verification (hub.challenge).
// @Summary Verify Webhook
// @Description Inbound provider callback. Requires the configured provider signature or handshake verification; application API keys are not callback credentials.
// @Tags MetaWebhook
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/webhooks/meta/whatsapp [get]
func (h *MetaWebhookHandler) VerifyWebhook(c *fiber.Ctx) error {
	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	if h.verifyToken != "" && mode == "subscribe" && token == h.verifyToken {
		h.logger.Info("Meta webhook verification succeeded")
		return c.SendString(challenge)
	}

	h.logger.Warn("Meta webhook verification failed",
		zap.String("mode", mode),
		zap.Bool("token_match", token == h.verifyToken))
	return c.SendStatus(fiber.StatusForbidden)
}

// HandleWebhook handles POST /v1/webhooks/meta/whatsapp — inbound messages and status updates.
// Meta requires a 200 OK response within 5 seconds; processing is kept lightweight.
// @Summary Handle Webhook
// @Description Inbound provider callback. Requires the configured provider signature or handshake verification; application API keys are not callback credentials.
// @Tags MetaWebhook
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/webhooks/meta/whatsapp [post]
func (h *MetaWebhookHandler) HandleWebhook(c *fiber.Ctx) error {
	rawBody := c.Body()

	var payload dto.MetaWebhookPayload
	if json.Unmarshal(rawBody, &payload) != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	verified := false
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			secret := h.appSecret
			if change.Field == "messages" {
				// Inbound messages use today's sender claim; delivery statuses
				// use the immutable admitted connection and original key.
				if len(change.Value.Messages) > 0 || len(change.Value.Statuses) == 0 {
					binding, err := h.senderResolver.ResolveInboundSender(c.Context(), "meta", entry.ID, change.Value.Metadata.PhoneNumberID)
					if err != nil {
						return c.SendStatus(fiber.StatusServiceUnavailable)
					}
					secret = binding.SigningSecret
					if secret == "" {
						secret = h.appSecret
					}
					if secret == "" {
						return c.SendStatus(fiber.StatusServiceUnavailable)
					}
					if !verifyMetaSignature(secret, rawBody, c.Get("X-Hub-Signature-256")) {
						return c.SendStatus(fiber.StatusUnauthorized)
					}
					c.Locals("meta_binding_"+binding.SenderID, binding)
					verified = true
				}
				for _, status := range change.Value.Statuses {
					binding, err := h.senderResolver.ResolveAdmittedStatus(c.Context(), "meta", entry.ID, change.Value.Metadata.PhoneNumberID, status.ID)
					if errors.Is(err, services.ErrAdmittedStatusNotFound) {
						binding, err = h.senderResolver.ResolveInboundSender(c.Context(), "meta", entry.ID, change.Value.Metadata.PhoneNumberID)
					}
					if err != nil {
						return c.SendStatus(fiber.StatusServiceUnavailable)
					}
					secret = binding.SigningSecret
					if secret == "" {
						secret = h.appSecret
					}
					if secret == "" {
						return c.SendStatus(fiber.StatusServiceUnavailable)
					}
					if !verifyMetaSignature(secret, rawBody, c.Get("X-Hub-Signature-256")) {
						return c.SendStatus(fiber.StatusUnauthorized)
					}
					c.Locals("meta_status_binding_"+entry.ID+"\x00"+change.Value.Metadata.PhoneNumberID+"\x00"+status.ID, binding)
					verified = true
				}
				continue
			}
			if change.Field == "message_template_status_update" {
				bindings, err := h.senderResolver.ResolveMetaTemplateSenders(c.Context(), entry.ID)
				if err != nil {
					return c.SendStatus(fiber.StatusServiceUnavailable)
				}
				var compatible []*services.InboundSenderBinding
				hasSecret := false
				for _, binding := range bindings {
					key := binding.SigningSecret
					if key == "" {
						key = h.appSecret
					}
					if key == "" {
						continue
					}
					hasSecret = true
					if verifyMetaSignature(key, rawBody, c.Get("X-Hub-Signature-256")) {
						compatible = append(compatible, binding)
					}
				}
				if len(compatible) == 0 {
					if !hasSecret {
						return c.SendStatus(fiber.StatusServiceUnavailable)
					}
					return c.SendStatus(fiber.StatusUnauthorized)
				}
				c.Locals("meta_template_bindings_"+entry.ID, compatible)
				verified = true
				continue
			}
			if secret == "" {
				return c.SendStatus(fiber.StatusServiceUnavailable)
			}
			if !verifyMetaSignature(secret, rawBody, c.Get("X-Hub-Signature-256")) {
				return c.SendStatus(fiber.StatusUnauthorized)
			}
			verified = true
		}
	}
	if !verified {
		if h.appSecret == "" {
			return c.SendStatus(fiber.StatusServiceUnavailable)
		}
		if !h.verifySignature(rawBody, c.Get("X-Hub-Signature-256")) {
			return c.SendStatus(fiber.StatusUnauthorized)
		}
	}
	if payload.Object != "whatsapp_business_account" {
		return c.SendStatus(fiber.StatusOK)
	}
	for _, entry := range payload.Entry {
		wabaID := entry.ID
		for _, change := range entry.Changes {
			switch change.Field {
			case "messages":
				val := change.Value
				phoneNumberID := val.Metadata.PhoneNumberID
				binding, _ := c.Locals("meta_binding_" + phoneNumberID).(*services.InboundSenderBinding)
				if binding == nil && len(val.Messages) > 0 {
					return c.SendStatus(fiber.StatusServiceUnavailable)
				}
				appID := ""
				if binding != nil {
					appID = binding.AppID
				}
				c.Locals("meta_active_binding", binding)

				for i, msg := range val.Messages {
					var contactName string
					if i < len(val.Contacts) {
						contactName = val.Contacts[i].Profile.Name
					}
					h.handleInboundMessage(c, appID, wabaID, phoneNumberID, &msg, contactName)
					if c.Locals("callback_error") != nil {
						return c.SendStatus(fiber.StatusServiceUnavailable)
					}
				}

				for _, status := range val.Statuses {
					statusBinding, _ := c.Locals("meta_status_binding_" + entry.ID + "\x00" + phoneNumberID + "\x00" + status.ID).(*services.InboundSenderBinding)
					if statusBinding == nil {
						return c.SendStatus(fiber.StatusServiceUnavailable)
					}
					c.Locals("meta_active_binding", statusBinding)
					h.handleStatusUpdate(c, &status)
					if c.Locals("callback_error") != nil {
						return c.SendStatus(fiber.StatusServiceUnavailable)
					}
				}

			case "message_template_status_update":
				processor, ok := h.whatsappSvc.(whatsapp.EventProcessor)
				if !ok {
					return c.SendStatus(fiber.StatusServiceUnavailable)
				}
				bindings, _ := c.Locals("meta_template_bindings_" + entry.ID).([]*services.InboundSenderBinding)
				if change.Value.TemplateName == "" || change.Value.TemplateEvent == "" {
					return c.SendStatus(fiber.StatusBadRequest)
				}
				for _, binding := range bindings {
					_, err := processor.AcceptVerifiedEvent(c.Context(), binding, &whatsapp.VerifiedEvent{Kind: "template", Template: &whatsapp.TemplateStatusEvent{TemplateID: change.Value.TemplateName, Status: change.Value.TemplateEvent, Reason: change.Value.TemplateReason}})
					if err != nil {
						return c.SendStatus(fiber.StatusServiceUnavailable)
					}
				}

			default:
				h.logger.Debug("Ignoring webhook field", zap.String("field", change.Field))
			}
		}
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *MetaWebhookHandler) handleInboundMessage(
	c *fiber.Ctx,
	appID, wabaID, phoneNumberID string,
	msg *dto.MetaInboundMsg,
	contactName string,
) {
	if appID == "" {
		h.logger.Warn("Cannot route inbound WhatsApp message: no app found for WABA/phone",
			zap.String("waba_id", wabaID),
			zap.String("phone_number_id", phoneNumberID))
		return
	}

	ts := parseUnixTimestamp(msg.Timestamp)

	inbound := &whatsapp.InboundMessage{
		WABAID:        wabaID,
		PhoneNumberID: phoneNumberID,
		ContactWAID:   msg.From,
		ContactName:   contactName,
		Direction:     whatsapp.DirectionInbound,
		MessageType:   msg.Type,
		MetaMessageID: msg.ID,
		Timestamp:     ts,
	}

	switch msg.Type {
	case "text":
		if msg.Text != nil {
			inbound.TextBody = msg.Text.Body
		}
	case "image", "video", "audio", "document":
		media := msg.Image
		if media == nil {
			media = msg.Video
		}
		if media == nil {
			media = msg.Audio
		}
		if media == nil {
			media = msg.Document
		}
		if media != nil {
			inbound.MediaMimeType = media.MimeType
			// media.ID is a Meta media ID; actual download URL requires a separate API call.
			// We store the Meta media ID for later retrieval.
			inbound.RawPayload = map[string]interface{}{
				"meta_media_id": media.ID,
				"caption":       media.Caption,
			}
			if media.Caption != "" {
				inbound.TextBody = media.Caption
			}
		}
	case "location":
		if msg.Location != nil {
			inbound.Latitude = msg.Location.Latitude
			inbound.Longitude = msg.Location.Longitude
			if msg.Location.Name != "" {
				inbound.TextBody = msg.Location.Name
			}
		}
	case "interactive":
		// Quick-reply or list-row tap on an interactive message we sent.
		// The button payload / row ID is what callers gave us in
		// metaButtonReply.ID / metaSectionRow.ID — surfacing it as a typed
		// field so workflows and the inbox can react without re-parsing.
		if msg.Interactive != nil {
			inbound.InteractiveType = msg.Interactive.Type
			switch msg.Interactive.Type {
			case "button_reply":
				if msg.Interactive.ButtonReply != nil {
					inbound.ReplyID = msg.Interactive.ButtonReply.ID
					inbound.ReplyTitle = msg.Interactive.ButtonReply.Title
					inbound.TextBody = msg.Interactive.ButtonReply.Title
				}
			case "list_reply":
				if msg.Interactive.ListReply != nil {
					inbound.ReplyID = msg.Interactive.ListReply.ID
					inbound.ReplyTitle = msg.Interactive.ListReply.Title
					inbound.ReplyDescription = msg.Interactive.ListReply.Description
					inbound.TextBody = msg.Interactive.ListReply.Title
				}
			}
		}
	case "button":
		// Template button tap (URL or QUICK_REPLY). Meta delivers this as a
		// separate msg.Type "button" with text + payload fields, NOT as
		// "interactive" — easy to miss.
		if msg.Button != nil {
			inbound.InteractiveType = "template_button"
			inbound.ReplyTitle = msg.Button.Text
			inbound.ButtonPayload = msg.Button.Payload
			inbound.TextBody = msg.Button.Text
		}
	case "reaction":
		if msg.Reaction != nil {
			inbound.ContextMessageID = msg.Reaction.MessageID
			inbound.ReactionEmoji = msg.Reaction.Emoji
		}
	default:
		// Store raw payload for unsupported types
		rawBytes, _ := json.Marshal(msg)
		var raw map[string]interface{}
		_ = json.Unmarshal(rawBytes, &raw)
		inbound.RawPayload = raw
	}

	if msg.Context != nil {
		inbound.ContextMessageID = msg.Context.ID
		inbound.IsForwarded = msg.Context.Forwarded
	}

	if binding, ok := c.Locals("meta_active_binding").(*services.InboundSenderBinding); ok {
		inbound.ConnectionID = binding.ID
		inbound.EnvironmentID = binding.EnvironmentID
		inbound.Provider = binding.Provider
		inbound.AccountID = binding.AccountID
		inbound.SenderID = binding.SenderID
	}
	if err := h.whatsappSvc.HandleInbound(c.Context(), appID, inbound); err != nil {
		c.Locals("callback_error", true)
		h.logger.Error("Failed to handle inbound WhatsApp message",
			zap.String("app_id", appID),
			zap.String("meta_message_id", msg.ID),
			zap.Error(err))
	}
}

func (h *MetaWebhookHandler) handleStatusUpdate(c *fiber.Ctx, status *dto.MetaStatus) {
	ts := parseUnixTimestamp(status.Timestamp)

	ds := &whatsapp.DeliveryStatus{
		MetaMessageID: status.ID,
		Status:        status.Status,
		Timestamp:     ts,
		RecipientID:   status.RecipientID,
	}

	if status.Conversation != nil {
		ds.ConversationID = status.Conversation.ID
		ds.ConversationOrigin = status.Conversation.Origin.Type
	}
	if status.Pricing != nil {
		ds.Billable = status.Pricing.Billable
		ds.PricingCategory = status.Pricing.Category
	}
	if len(status.Errors) > 0 {
		ds.ErrorCode = status.Errors[0].Code
		ds.ErrorMessage = status.Errors[0].Message
	}

	if binding, ok := c.Locals("meta_active_binding").(*services.InboundSenderBinding); ok {
		ds.ConnectionID = binding.ID
		ds.AppID = binding.AppID
		ds.EnvironmentID = binding.EnvironmentID
		ds.Provider = binding.Provider
		ds.AccountID = binding.AccountID
		ds.SenderID = binding.SenderID
	}
	if err := h.whatsappSvc.HandleStatus(c.Context(), ds); err != nil {
		c.Locals("callback_error", true)
		h.logger.Error("Failed to handle WhatsApp delivery status",
			zap.String("meta_message_id", status.ID),
			zap.String("status", status.Status),
			zap.Error(err))
	}
}

func (h *MetaWebhookHandler) resolveAppID(ctx context.Context, wabaID, phoneNumberID string) string {
	binding, err := h.senderResolver.ResolveInboundSender(ctx, "meta", wabaID, phoneNumberID)
	if err != nil {
		return ""
	}
	return binding.AppID
}
func (h *MetaWebhookHandler) SetSenderResolver(resolver *services.ChannelSenderResolver) {
	h.senderResolver = resolver
}

func (h *MetaWebhookHandler) verifySignature(body []byte, sigHeader string) bool {
	return verifyMetaSignature(h.appSecret, body, sigHeader)
}
func verifyMetaSignature(secret string, body []byte, sigHeader string) bool {
	if sigHeader == "" || secret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedHex := hex.EncodeToString(mac.Sum(nil))

	provided := strings.TrimSpace(sigHeader)
	if strings.HasPrefix(provided, "sha256=") {
		provided = provided[7:]
	}
	return hmac.Equal([]byte(strings.ToLower(provided)), []byte(strings.ToLower(expectedHex)))
}

func parseUnixTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	epoch, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(epoch, 0).UTC()
}
