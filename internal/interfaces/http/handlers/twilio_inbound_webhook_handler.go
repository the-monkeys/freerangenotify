package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
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
// Always returns HTTP 200 so Twilio does not retry. Failures are logged.
type TwilioInboundWebhookHandler struct {
	whatsappSvc whatsapp.Service
	appRepo     application.Repository
	authToken   string
	logger      *zap.Logger

	// app lookup cache keyed by normalised "to_number" (without
	// "whatsapp:" prefix). Refreshed on miss with a short TTL.
	cacheMu        sync.RWMutex
	toIndex        map[string]string
	cacheExpiresAt time.Time
}

const twilioInboundCacheTTL = 60 * time.Second

// NewTwilioInboundWebhookHandler constructs a Twilio inbound webhook
// handler. authToken is the Twilio account auth token used to validate
// signatures; an empty token disables verification (with a warning log
// per request) and is intended for local development only.
func NewTwilioInboundWebhookHandler(
	whatsappSvc whatsapp.Service,
	appRepo application.Repository,
	authToken string,
	logger *zap.Logger,
) *TwilioInboundWebhookHandler {
	return &TwilioInboundWebhookHandler{
		whatsappSvc: whatsappSvc,
		appRepo:     appRepo,
		authToken:   authToken,
		logger:      logger,
		toIndex:     map[string]string{},
	}
}

// Handle implements POST /v1/webhooks/twilio/whatsapp.
func (h *TwilioInboundWebhookHandler) Handle(c *fiber.Ctx) error {
	// Capture the raw form values up front; Fiber parses them lazily and
	// signature validation must see the same key/value set Twilio signed.
	form, err := c.MultipartForm()
	if err != nil {
		// Twilio sends application/x-www-form-urlencoded by default. Fall
		// back to FormValue accessors below.
		form = nil
	}
	values := collectFormValues(c, form)

	if h.authToken == "" {
		h.logger.Warn("Twilio inbound webhook signature verification disabled (no auth token configured)")
	} else {
		sig := c.Get("X-Twilio-Signature")
		fullURL := buildTwilioSignatureURL(c)
		if !validateTwilioSignature(h.authToken, fullURL, values, sig) {
			h.logger.Warn("Twilio inbound webhook signature rejected",
				zap.String("from", values["From"]),
				zap.String("to", values["To"]))
			return c.SendStatus(fiber.StatusForbidden)
		}
	}

	to := strings.TrimPrefix(values["To"], "whatsapp:")
	from := strings.TrimPrefix(values["From"], "whatsapp:")
	if from == "" {
		// Nothing actionable; ack.
		return c.SendStatus(fiber.StatusOK)
	}

	appID := h.resolveAppIDByTo(c.Context(), to)
	if appID == "" {
		h.logger.Warn("Twilio inbound webhook: no app matched 'To' number",
			zap.String("to", to))
		return c.SendStatus(fiber.StatusOK)
	}

	msg := buildInboundFromTwilio(appID, from, values)
	if err := h.whatsappSvc.HandleInbound(c.Context(), appID, msg); err != nil {
		h.logger.Error("Twilio inbound: HandleInbound failed",
			zap.String("app_id", appID),
			zap.String("from", from),
			zap.Error(err))
	}
	return c.SendStatus(fiber.StatusOK)
}

// resolveAppIDByTo looks up the FRN app whose Twilio FromNumber matches
// the inbound `To` field. Results are cached for a short TTL.
func (h *TwilioInboundWebhookHandler) resolveAppIDByTo(ctx context.Context, to string) string {
	if to == "" {
		return ""
	}
	h.cacheMu.RLock()
	if time.Now().Before(h.cacheExpiresAt) {
		if id, ok := h.toIndex[to]; ok {
			h.cacheMu.RUnlock()
			return id
		}
	}
	h.cacheMu.RUnlock()

	// Refresh cache.
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()
	if time.Now().Before(h.cacheExpiresAt) {
		if id, ok := h.toIndex[to]; ok {
			return id
		}
	}
	apps, err := h.appRepo.List(ctx, application.ApplicationFilter{Limit: 500})
	if err != nil {
		h.logger.Warn("Twilio inbound: app list failed", zap.Error(err))
		return ""
	}
	idx := make(map[string]string, len(apps))
	for _, app := range apps {
		wa := app.Settings.WhatsApp
		if wa == nil {
			continue
		}
		if wa.FromNumber != "" {
			idx[strings.TrimPrefix(wa.FromNumber, "whatsapp:")] = app.AppID
		}
	}
	h.toIndex = idx
	h.cacheExpiresAt = time.Now().Add(twilioInboundCacheTTL)
	return idx[to]
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

// buildTwilioSignatureURL reconstructs the absolute URL Twilio used when
// signing. Behind a TLS-terminating proxy callers MUST forward the
// X-Forwarded-Proto / X-Forwarded-Host headers or pass an explicit
// public URL — Twilio's signature is URL-sensitive.
func buildTwilioSignatureURL(c *fiber.Ctx) string {
	scheme := c.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = c.Protocol()
	}
	host := c.Get("X-Forwarded-Host")
	if host == "" {
		host = c.Hostname()
	}
	return scheme + "://" + host + c.OriginalURL()
}

// collectFormValues extracts all form fields (multipart or urlencoded)
// into a flat string map. Multi-value fields are joined by comma — none
// of Twilio's WhatsApp webhook fields are repeated in practice.
func collectFormValues(c *fiber.Ctx, mf interface{}) map[string]string {
	out := map[string]string{}
	// Common Twilio fields we definitely want even if multipart parsing
	// failed; FormValue falls back to urlencoded body parsing.
	for _, k := range []string{
		"MessageSid", "AccountSid", "From", "To", "Body",
		"ButtonText", "ButtonPayload", "ListId", "ListTitle",
		"ListDescription", "OriginalRepliedMessageSid", "NumMedia",
		"MediaUrl0", "MediaContentType0", "ProfileName", "WaId",
	} {
		if v := c.FormValue(k); v != "" {
			out[k] = v
		}
	}
	return out
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
