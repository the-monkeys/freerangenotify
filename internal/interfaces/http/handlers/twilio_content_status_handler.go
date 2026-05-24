package handlers

import (
	"github.com/gofiber/fiber/v2"
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
	svc    services.WhatsAppRichTemplateService
	logger *zap.Logger
}

// NewTwilioContentStatusHandler creates a new handler.
func NewTwilioContentStatusHandler(svc services.WhatsAppRichTemplateService, logger *zap.Logger) *TwilioContentStatusHandler {
	return &TwilioContentStatusHandler{svc: svc, logger: logger}
}

// Handle implements POST /v1/webhooks/twilio/content-status. Returns 200
// even when the ContentSid is unknown to avoid Twilio retries flooding the
// log for templates created outside FRN.
func (h *TwilioContentStatusHandler) Handle(c *fiber.Ctx) error {
	contentSid := c.FormValue("ContentSid")
	status := c.FormValue("Status")
	reason := c.FormValue("RejectionReason")

	if contentSid == "" || status == "" {
		return pkgerrors.BadRequest("ContentSid and Status are required")
	}

	if err := h.svc.ApplyTwilioStatus(c.Context(), contentSid, status, reason); err != nil {
		h.logger.Warn("Twilio status apply failed",
			zap.String("content_sid", contentSid),
			zap.String("status", status),
			zap.Error(err))
		// Still return 200 so Twilio doesn't retry — the error is logged
		// and human-investigatable.
	}
	return c.SendStatus(fiber.StatusOK)
}
