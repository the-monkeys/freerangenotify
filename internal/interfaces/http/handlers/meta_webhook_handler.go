package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"go.uber.org/zap"
)

// MetaWebhookHandler handles Meta WhatsApp Cloud API webhooks.
type MetaWebhookHandler struct {
	whatsappSvc     whatsapp.Service
	appRepo         application.Repository
	templateHandler *WhatsAppTemplateHandler // optional, for template status webhooks
	appSecret       string                  // Meta App Secret for X-Hub-Signature-256 verification
	verifyToken     string                  // hub.verify_token for webhook subscription registration
	logger          *zap.Logger
}

// SetTemplateHandler wires the template handler for processing template status webhooks.
func (h *MetaWebhookHandler) SetTemplateHandler(th *WhatsAppTemplateHandler) {
	h.templateHandler = th
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
		whatsappSvc: whatsappSvc,
		appRepo:     appRepo,
		appSecret:   appSecret,
		verifyToken: verifyToken,
		logger:      logger,
	}
}

// VerifyWebhook handles GET /v1/webhooks/meta/whatsapp — Meta webhook verification (hub.challenge).
func (h *MetaWebhookHandler) VerifyWebhook(c *fiber.Ctx) error {
	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	if mode == "subscribe" && token == h.verifyToken {
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
func (h *MetaWebhookHandler) HandleWebhook(c *fiber.Ctx) error {
	rawBody := c.Body()

	if h.appSecret != "" {
		sigHeader := c.Get("X-Hub-Signature-256")
		if !h.verifySignature(rawBody, sigHeader) {
			h.logger.Warn("Invalid X-Hub-Signature-256 on Meta webhook")
			return c.SendStatus(fiber.StatusUnauthorized)
		}
	}

	var payload dto.MetaWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		h.logger.Error("Failed to parse Meta webhook payload", zap.Error(err))
		return c.SendStatus(fiber.StatusOK)
	}

	if payload.Object != "whatsapp_business_account" {
		h.logger.Debug("Ignoring non-WhatsApp webhook", zap.String("object", payload.Object))
		return c.SendStatus(fiber.StatusOK)
	}

	for _, entry := range payload.Entry {
		wabaID := entry.ID
		for _, change := range entry.Changes {
			switch change.Field {
			case "messages":
				val := change.Value
				phoneNumberID := val.Metadata.PhoneNumberID
				appID := h.resolveAppID(c.Context(), wabaID, phoneNumberID)

				for i, msg := range val.Messages {
					var contactName string
					if i < len(val.Contacts) {
						contactName = val.Contacts[i].Profile.Name
					}
					h.handleInboundMessage(c, appID, wabaID, phoneNumberID, &msg, contactName)
				}

				for _, status := range val.Statuses {
					h.handleStatusUpdate(c, &status)
				}

			case "message_template_status_update":
				if h.templateHandler != nil {
					rawBytes, _ := json.Marshal(change.Value)
					var event map[string]interface{}
					_ = json.Unmarshal(rawBytes, &event)
					h.templateHandler.HandleTemplateStatusWebhook(event)
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

	if err := h.whatsappSvc.HandleInbound(c.Context(), appID, inbound); err != nil {
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

	if err := h.whatsappSvc.HandleStatus(c.Context(), ds); err != nil {
		h.logger.Error("Failed to handle WhatsApp delivery status",
			zap.String("meta_message_id", status.ID),
			zap.String("status", status.Status),
			zap.Error(err))
	}
}

// resolveAppID maps a WABA ID + phone number ID to a FreeRangeNotify app.
// It checks per-app WhatsApp config for matching Meta credentials.
func (h *MetaWebhookHandler) resolveAppID(ctx context.Context, wabaID, phoneNumberID string) string {
	// TODO: In production, maintain a Redis cache of waba_id→app_id mappings
	// populated during Embedded Signup (Phase 2). For now, do a brute-force
	// scan of apps — acceptable at low volume.
	apps, err := h.appRepo.List(ctx, application.ApplicationFilter{Limit: 500})
	if err != nil {
		h.logger.Warn("Failed to list apps for WABA resolution", zap.Error(err))
		return ""
	}

	for _, app := range apps {
		wa := app.Settings.WhatsApp
		if wa == nil || wa.Provider != "meta" {
			continue
		}
		if wa.MetaWABAID == wabaID || wa.MetaPhoneNumberID == phoneNumberID {
			return app.AppID
		}
	}

	h.logger.Debug("No app found for WABA/phone combo",
		zap.String("waba_id", wabaID),
		zap.String("phone_number_id", phoneNumberID))
	return ""
}

func (h *MetaWebhookHandler) verifySignature(body []byte, sigHeader string) bool {
	if sigHeader == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(h.appSecret))
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
		return time.Now().UTC()
	}
	epoch, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return time.Now().UTC()
	}
	return time.Unix(epoch, 0).UTC()
}
