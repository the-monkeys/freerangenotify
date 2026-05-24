package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

// wabaCacheTTL is how long the WABA/phone-number-id → app_id lookup table is
// cached. 5 min keeps the worst-case staleness equal to the Meta template
// status sync window in the implementation plan.
const wabaCacheTTL = 5 * time.Minute

// MetaWebhookHandler handles Meta WhatsApp Cloud API webhooks.
type MetaWebhookHandler struct {
	whatsappSvc     whatsapp.Service
	appRepo         application.Repository
	templateHandler    *WhatsAppTemplateHandler             // optional, legacy template status webhooks
	richTemplateSvc    services.WhatsAppRichTemplateService // optional, rich-template status webhooks
	appSecret          string                               // Meta App Secret for X-Hub-Signature-256 verification
	verifyToken        string                               // hub.verify_token for webhook subscription registration
	logger             *zap.Logger

	// WABA / phone-number-id → app_id cache. Avoids an O(N) app scan on every
	// inbound webhook. Refilled lazily via repopulateLocked() on miss or expiry.
	cacheMu        sync.RWMutex
	wabaIndex      map[string]string // waba_id -> app_id
	phoneIndex     map[string]string // phone_number_id -> app_id
	cacheExpiresAt time.Time
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
				rawBytes, _ := json.Marshal(change.Value)
				var event map[string]interface{}
				_ = json.Unmarshal(rawBytes, &event)
				if h.templateHandler != nil {
					h.templateHandler.HandleTemplateStatusWebhook(event)
				}
				// Forward to the rich-template service so the FRN-side binding
				// reflects approval transitions without polling.
				if h.richTemplateSvc != nil {
					tplName, _ := event["message_template_name"].(string)
					status, _ := event["event"].(string)
					reason, _ := event["reason"].(string)
					if tplName != "" && status != "" {
						if err := h.richTemplateSvc.ApplyMetaStatus(c.Context(), wabaID, tplName, status, reason); err != nil {
							h.logger.Warn("Rich template status update failed",
								zap.String("waba_id", wabaID),
								zap.String("template", tplName),
								zap.Error(err))
						}
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
// Hot path on every inbound webhook; backed by an in-memory cache that is
// refilled at most every wabaCacheTTL.
func (h *MetaWebhookHandler) resolveAppID(ctx context.Context, wabaID, phoneNumberID string) string {
	if appID := h.lookupCached(wabaID, phoneNumberID); appID != "" {
		return appID
	}

	// Cache miss or expired — refill and retry under the write lock so that a
	// burst of webhooks for a freshly-configured WABA doesn't issue N parallel
	// ES list calls.
	h.cacheMu.Lock()
	if appID := h.lookupLocked(wabaID, phoneNumberID); appID != "" && time.Now().Before(h.cacheExpiresAt) {
		h.cacheMu.Unlock()
		return appID
	}
	if err := h.repopulateLocked(ctx); err != nil {
		h.cacheMu.Unlock()
		h.logger.Warn("Failed to list apps for WABA resolution", zap.Error(err))
		return ""
	}
	appID := h.lookupLocked(wabaID, phoneNumberID)
	h.cacheMu.Unlock()

	if appID == "" {
		h.logger.Debug("No app found for WABA/phone combo",
			zap.String("waba_id", wabaID),
			zap.String("phone_number_id", phoneNumberID))
	}
	return appID
}

// lookupCached returns the cached app_id under a read lock, or "" if the
// cache is empty / expired / has no match.
func (h *MetaWebhookHandler) lookupCached(wabaID, phoneNumberID string) string {
	h.cacheMu.RLock()
	defer h.cacheMu.RUnlock()
	if time.Now().After(h.cacheExpiresAt) {
		return ""
	}
	return h.lookupLocked(wabaID, phoneNumberID)
}

// lookupLocked must be called with cacheMu held (read or write).
func (h *MetaWebhookHandler) lookupLocked(wabaID, phoneNumberID string) string {
	if wabaID != "" {
		if id, ok := h.wabaIndex[wabaID]; ok {
			return id
		}
	}
	if phoneNumberID != "" {
		if id, ok := h.phoneIndex[phoneNumberID]; ok {
			return id
		}
	}
	return ""
}

// repopulateLocked refreshes the WABA / phone-number-id indexes. Must be
// called with cacheMu write-locked.
func (h *MetaWebhookHandler) repopulateLocked(ctx context.Context) error {
	apps, err := h.appRepo.List(ctx, application.ApplicationFilter{Limit: 500})
	if err != nil {
		return err
	}
	waba := make(map[string]string, len(apps))
	phone := make(map[string]string, len(apps))
	for _, app := range apps {
		wa := app.Settings.WhatsApp
		if wa == nil || wa.Provider != "meta" {
			continue
		}
		if wa.MetaWABAID != "" {
			waba[wa.MetaWABAID] = app.AppID
		}
		if wa.MetaPhoneNumberID != "" {
			phone[wa.MetaPhoneNumberID] = app.AppID
		}
	}
	h.wabaIndex = waba
	h.phoneIndex = phone
	h.cacheExpiresAt = time.Now().Add(wabaCacheTTL)
	return nil
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
