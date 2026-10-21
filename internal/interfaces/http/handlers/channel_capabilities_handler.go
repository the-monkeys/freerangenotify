package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/config"
)

type ChannelCapabilitiesHandler struct{ Config *config.Config }

// Get is public and intentionally constructs an allowlist, never serializing
// provider config. Account-specific readiness belongs to authenticated app APIs.
// @Summary Get
// @Description Public capability discovery. No API key is required; reports enabled operator features without credentials.
// @Tags ChannelCapabilities
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/channel-capabilities [get]
func (h *ChannelCapabilitiesHandler) Get(c *fiber.Ctx) error {
	cfg := h.Config
	meta := cfg.Providers.MetaWhatsApp
	c.Set("Cache-Control", "no-store")
	return c.JSON(fiber.Map{"data": fiber.Map{
		"meta": fiber.Map{"delivery": meta.Enabled, "manual": cfg.Features.WhatsAppMetaEnabled, "embedded": cfg.Features.WhatsAppMetaEnabled && meta.MetaAppID != "" && meta.MetaAppSecret != "", "app_id": meta.MetaAppID, "signup_config_id": cfg.Messaging.MetaSignupConfigID, "graph_version": meta.APIVersion},
		"sms":  fiber.Map{"providers": []string{"twilio", "vonage"}, "exotel": cfg.Providers.Exotel.Enabled},
	}})
}
