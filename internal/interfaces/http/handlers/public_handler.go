package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
)

// PublicHandler exposes non-authenticated, aggregate-only endpoints.
type PublicHandler struct {
	authRepo auth.Repository
}

func NewPublicHandler(authRepo auth.Repository) *PublicHandler {
	return &PublicHandler{authRepo: authRepo}
}

// GetStats returns aggregate public stats (no PII).
func (h *PublicHandler) GetStats(c *fiber.Ctx) error {
	count, err := h.authRepo.CountUsers(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch stats"})
	}
	return c.JSON(fiber.Map{
		"user_count": count,
	})
}
