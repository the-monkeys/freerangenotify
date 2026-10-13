package handlers

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"sort"
	"strings"

	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

// TwilioInboundWebhookHandler receives inbound WhatsApp messages (replies,
// button taps, list selections) from Twilio's Programmable Messaging
// webhook. Twilio POSTs application/x-www-form-urlencoded bodies with
// fields like:
//
//	MessageSid, AccountSid, From (whatsapp:+...), To (whatsapp:+...),
//	Body, ButtonText, ButtonPayload, ListId, OriginalRepliedMessageSid,
//	NumMedia, MediaUrl0, ...
//
// This handler verifies X-Twilio-Signature (HMAC-SHA1 over the request
// URL + sorted form fields, base64-encoded), resolves the target FRN app
// by matching `To` against application.Settings.WhatsApp.FromNumber, and
// forwards a normalised whatsapp.InboundMessage to whatsapp.Service which
// already handles workflow event emission (whatsapp.button_clicked, etc.)
// per WHATSAPP_RICH_INTERACTIVE_PLAN.md §6.
//
// Acknowledges only accepted messages. Storage failures remain retryable.
type TwilioInboundWebhookHandler struct {
	senderResolver *services.ChannelSenderResolver
	publicURL      string
	whatsappSvc    whatsapp.Service
	logger         *zap.Logger
}

// NewTwilioInboundWebhookHandler constructs a Twilio inbound webhook
// handler. authToken is the Twilio account auth token used to validate
// signatures. Production wiring supplies an account-specific resolver and the
// canonical public URL; missing signing configuration fails closed.
func NewTwilioInboundWebhookHandler(
	whatsappSvc whatsapp.Service,
	appRepo application.Repository,
	authToken string,
	logger *zap.Logger,
) *TwilioInboundWebhookHandler {
	return &TwilioInboundWebhookHandler{
		whatsappSvc:    whatsappSvc,
		senderResolver: services.NewChannelSenderResolver(appRepo, nil),
		logger:         logger,
	}
}

// Handle implements POST /v1/webhooks/twilio/whatsapp.
func (h *TwilioInboundWebhookHandler) Handle(c *fiber.Ctx) error {
	if hasRepeatedFormFields(c) {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	// Capture the raw form values up front; Fiber parses them lazily and
	// signature validation must see the same key/value set Twilio signed.
	form, err := c.MultipartForm()
	if err != nil {
		// Twilio sends application/x-www-form-urlencoded by default. Fall
		// back to FormValue accessors below.
		form = nil
	}
	values := collectFormValues(c, form)

	to := strings.TrimPrefix(values["To"], "whatsapp:")
	binding, resolveErr := h.senderResolver.ResolveInboundSender(c.Context(), "twilio", values["AccountSid"], to)
	if resolveErr != nil {
		return c.SendStatus(fiber.StatusForbidden)
	}
	if binding.SigningSecret == "" || h.publicURL == "" {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	if !validateTwilioSignature(binding.SigningSecret, strings.TrimRight(h.publicURL, "/")+c.OriginalURL(), values, c.Get("X-Twilio-Signature")) {
		return c.SendStatus(fiber.StatusForbidden)
	}
	// Sender ownership was verified above.
	from := strings.TrimPrefix(values["From"], "whatsapp:")
	if from == "" {
		// Nothing actionable; ack.
		return c.SendStatus(fiber.StatusOK)
	}

	appID := binding.AppID
	if appID == "" {
		h.logger.Warn("Twilio inbound webhook: no app matched 'To' number",
			zap.String("to", to))
		return c.SendStatus(fiber.StatusOK)
	}

	msg := buildInboundFromTwilio(appID, from, values)
	msg.ConnectionID = binding.ID
	msg.EnvironmentID = binding.EnvironmentID
	msg.Provider = binding.Provider
	msg.AccountID = binding.AccountID
	msg.SenderID = binding.SenderID
	if err := h.whatsappSvc.HandleInbound(c.Context(), appID, msg); err != nil {
		h.logger.Error("Twilio inbound: HandleInbound failed",
			zap.String("app_id", appID),
			zap.String("from", from),
			zap.Error(err))
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	return c.SendStatus(fiber.StatusOK)
}

// buildInboundFromTwilio normalises Twilio's flat form fields into the
// FRN-internal InboundMessage shape. Field mapping:
//
//   - ButtonPayload present → InteractiveType "template_button"
//   - ButtonText present → InteractiveType "button_reply"
//   - ListId present → InteractiveType "list_reply"
//   - Otherwise → MessageType "text"
func buildInboundFromTwilio(appID, from string, v map[string]string) *whatsapp.InboundMessage {
	now := time.Now().UTC()
	msg := &whatsapp.InboundMessage{
		AppID:            appID,
		ContactWAID:      from,
		Direction:        whatsapp.DirectionInbound,
		MetaMessageID:    v["MessageSid"],
		Timestamp:        now,
		CreatedAt:        now,
		ContextMessageID: v["OriginalRepliedMessageSid"],
		TextBody:         v["Body"],
		RawPayload:       toInterfaceMap(v),
	}

	switch {
	case v["ButtonPayload"] != "":
		msg.MessageType = "interactive"
		msg.InteractiveType = "template_button"
		msg.ButtonPayload = v["ButtonPayload"]
		msg.ReplyTitle = v["ButtonText"]
	case v["ButtonText"] != "":
		msg.MessageType = "interactive"
		msg.InteractiveType = "button_reply"
		msg.ReplyTitle = v["ButtonText"]
		msg.ReplyID = v["ButtonText"]
	case v["ListId"] != "":
		msg.MessageType = "interactive"
		msg.InteractiveType = "list_reply"
		msg.ReplyID = v["ListId"]
		msg.ReplyTitle = v["ListTitle"]
		msg.ReplyDescription = v["ListDescription"]
	default:
		msg.MessageType = "text"
	}
	return msg
}

// validateTwilioSignature implements Twilio's standard signature scheme:
// HMAC-SHA1 over (fullURL + sorted-by-key concatenation of form keys+values),
// base64-encoded. See https://www.twilio.com/docs/usage/webhooks/webhooks-security.
func validateTwilioSignature(authToken, fullURL string, values map[string]string, signature string) bool {
	if signature == "" {
		return false
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString(fullURL)
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString(values[k])
	}
	mac := hmac.New(sha1.New, []byte(authToken))
	mac.Write([]byte(sb.String()))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// collectFormValues extracts all form fields (multipart or urlencoded)
// into a flat string map. Handlers reject repeated fields before validation.
func collectFormValues(c *fiber.Ctx, mf interface{}) map[string]string {
	values := map[string]string{}
	c.Request().PostArgs().VisitAll(func(key, value []byte) { values[string(key)] = string(value) })
	if form, err := c.MultipartForm(); err == nil {
		for key, value := range form.Value {
			if len(value) == 1 {
				values[key] = value[0]
			}
		}
	}
	return values
}
func hasRepeatedFormFields(c *fiber.Ctx) bool {
	seen := map[string]bool{}
	repeated := false
	c.Request().PostArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		if seen[key] {
			repeated = true
		}
		seen[key] = true
	})
	if form, err := c.MultipartForm(); err == nil {
		for _, values := range form.Value {
			if len(values) != 1 {
				repeated = true
			}
		}
	}
	return repeated
}

// toInterfaceMap shallow-copies a string map into the map[string]interface{}
// shape expected by InboundMessage.RawPayload.
func toInterfaceMap(in map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (h *TwilioInboundWebhookHandler) SetSenderResolver(resolver *services.ChannelSenderResolver, publicURL string) {
	h.senderResolver = resolver
	h.publicURL = publicURL
}
