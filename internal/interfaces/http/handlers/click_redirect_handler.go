package handlers

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/analytics"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

// ClickRedirectHandler serves GET /v1/r/:sig — the public landing point
// for tracked URLs embedded in rich WhatsApp templates. The handler:
//
//   1. Verifies the signed payload (HMAC + expiry).
//   2. Best-effort tracks a `clicked` analytics event.
//   3. 302-redirects the user to the target URL.
//
// Tracking failures are logged but do not block the redirect — the
// recipient's UX trumps perfect analytics.
type ClickRedirectHandler struct {
	signer    *whatsapp.ClickSigner
	analytics *repository.AnalyticsEventRepository
	logger    *zap.Logger
}

// NewClickRedirectHandler wires the handler.
func NewClickRedirectHandler(signer *whatsapp.ClickSigner, analytics *repository.AnalyticsEventRepository, logger *zap.Logger) *ClickRedirectHandler {
	return &ClickRedirectHandler{signer: signer, analytics: analytics, logger: logger}
}

// Handle implements GET /v1/r/:sig.
//
// Error responses are deliberately spare HTML so a curious user pasting
// the link into a browser sees a friendly message instead of JSON noise,
// while preserving distinct status codes (400/403/410) for observability.
func (h *ClickRedirectHandler) Handle(c *fiber.Ctx) error {
	sig := c.Params("sig")
	if sig == "" {
		return c.Status(fiber.StatusBadRequest).SendString("missing signature")
	}
	payload, err := h.signer.Verify(sig)
	if err != nil {
		switch err {
		case whatsapp.ErrClickExpired:
			return c.Status(fiber.StatusGone).SendString("This link has expired.")
		case whatsapp.ErrClickSignature:
			h.logger.Warn("Click signature mismatch", zap.String("sig_prefix", safePrefix(sig)))
			return c.Status(fiber.StatusForbidden).SendString("invalid signature")
		default:
			return c.Status(fiber.StatusBadRequest).SendString("invalid link")
		}
	}

	// Fire-and-forget the analytics write — never block the redirect.
	if h.analytics != nil {
		go func(p whatsapp.ClickPayload, ua, ip string) {
			ev := &analytics.Event{
				ID:             uuid.NewString(),
				AppID:          p.AppID,
				NotificationID: p.NotificationID,
				EventType:      analytics.EventClicked,
				Channel:        "whatsapp",
				Timestamp:      time.Now().UTC(),
				Metadata: map[string]interface{}{
					"button_index": p.ButtonIndex,
					"target_url":   p.TargetURL,
					"user_agent":   ua,
					"ip":           ip,
				},
			}
			// Use a fresh context — the Fiber request context is recycled
			// once the handler returns. context.Background() with a short
			// timeout keeps the ES write bounded.
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := h.analytics.Track(ctx, ev); err != nil {
				h.logger.Warn("Click analytics track failed",
					zap.String("notification_id", p.NotificationID),
					zap.Error(err))
			}
		}(*payload, string(c.Request().Header.UserAgent()), c.IP())
	}

	return c.Redirect(payload.TargetURL, fiber.StatusFound)
}

// safePrefix returns up to the first 12 chars of a signature for logging
// without leaking the full HMAC. Useful for correlating warnings without
// helping an attacker assemble a valid signature.
func safePrefix(sig string) string {
	if len(sig) > 12 {
		return sig[:12] + "…"
	}
	return sig
}
