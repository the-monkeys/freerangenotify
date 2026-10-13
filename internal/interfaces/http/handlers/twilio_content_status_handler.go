package handlers

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// TwilioContentStatusHandler receives Twilio Content API approval status
// webhooks. Twilio POSTs application/x-www-form-urlencoded bodies with at
// least `ContentSid`, `Status`, and optionally `RejectionReason`. We map
// those onto the FRN-side TwilioBinding via the rich template service.
//
// Twilio webhooks should be authenticated via X-Twilio-Signature; the
// validation is delegated to a middleware so this handler stays focused
// on event processing.
type TwilioContentStatusHandler struct {
	events         whatsapp.EventProcessor
	resolveSigning func(context.Context, string, string) (string, string, error)
	publicURL      string
	svc            services.WhatsAppRichTemplateService
	logger         *zap.Logger
}

// NewTwilioContentStatusHandler creates a new handler.
func NewTwilioContentStatusHandler(svc services.WhatsAppRichTemplateService, logger *zap.Logger) *TwilioContentStatusHandler {
	return &TwilioContentStatusHandler{svc: svc, logger: logger}
}

// Handle implements POST /v1/webhooks/twilio/content-status. Returns 200
// even when the ContentSid is unknown to avoid Twilio retries flooding the
// log for templates created outside FRN.
func (h *TwilioContentStatusHandler) Handle(c *fiber.Ctx) error {
	if hasRepeatedFormFields(c) {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	if h.resolveSigning == nil || h.publicURL == "" {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	values := collectFormValues(c, nil)
	secret, appID, err := h.resolveSigning(c.Context(), values["ContentSid"], values["AccountSid"])
	if err != nil {
		return c.SendStatus(fiber.StatusForbidden)
	}
	if !validateTwilioSignature(secret, canonicalCallbackURL(h.publicURL, c.OriginalURL()), values, c.Get("X-Twilio-Signature")) {
		return c.SendStatus(fiber.StatusForbidden)
	}
	contentSid := c.FormValue("ContentSid")
	status := c.FormValue("Status")
	reason := c.FormValue("RejectionReason")

	if contentSid == "" || status == "" {
		return pkgerrors.BadRequest("ContentSid and Status are required")
	}

	if h.events == nil {
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	binding := &whatsapp.InboundSenderBinding{ID: "twilio-content:" + contentSid, AppID: appID, EnvironmentID: "default", Provider: "twilio", AccountID: values["AccountSid"], SenderID: contentSid}
	if _, err := h.events.AcceptVerifiedEvent(c.Context(), binding, &whatsapp.VerifiedEvent{Kind: "template", Template: &whatsapp.TemplateStatusEvent{TemplateID: contentSid, Status: status, Reason: reason}}); err != nil {
		h.logger.Warn("Twilio status durable acceptance unavailable")
		return c.SendStatus(fiber.StatusServiceUnavailable)
	}
	return c.SendStatus(fiber.StatusOK)
}
func (h *TwilioContentStatusHandler) SetEventProcessor(events whatsapp.EventProcessor) {
	h.events = events
}
