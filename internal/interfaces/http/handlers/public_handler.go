package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
)

// PublicHandler exposes non-authenticated, aggregate-only endpoints.
type PublicHandler struct {
	userRepo user.Repository
}

func NewPublicHandler(userRepo user.Repository) *PublicHandler {
	return &PublicHandler{userRepo: userRepo}
}

// GetStats returns aggregate public stats (no PII).
func (h *PublicHandler) GetStats(c *fiber.Ctx) error {
	// Count all users; repository enforces multi-tenant scoping via filter if provided.
	count, err := h.userRepo.Count(c.Context(), user.UserFilter{})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to fetch stats"})
	}
	return c.JSON(fiber.Map{
		"user_count": count,
	})
}
