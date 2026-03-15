package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

// BillingHandler serves user-facing billing and subscription APIs.
type BillingHandler struct {
	subRepo license.Repository
	logger  *zap.Logger
}

func NewBillingHandler(subRepo license.Repository, logger *zap.Logger) *BillingHandler {
	return &BillingHandler{subRepo: subRepo, logger: logger}
}

// GetUsage handles GET /v1/billing/usage
// Returns current period usage and limits for the authenticated user's personal workspace.
func (h *BillingHandler) GetUsage(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	now := time.Now().UTC()
	sub, err := h.subRepo.GetActiveSubscription(c.Context(), userID, "", now)
	if err != nil {
		h.logger.Error("failed to get subscription for usage", zap.String("user_id", userID), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to retrieve subscription"})
	}

	if sub == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":  "no_active_subscription",
			"status": "none",
		})
	}

	messageLimit := metaInt(sub.Metadata, "message_limit", 10000)
	messagesSent := metaInt(sub.Metadata, "messages_sent", 0)
	usagePct := 0.0
	if messageLimit > 0 {
		usagePct = float64(messagesSent) / float64(messageLimit) * 100
	}
	daysRemaining := int(sub.CurrentPeriodEnd.Sub(now).Hours() / 24)
	if daysRemaining < 0 {
		daysRemaining = 0
	}

	return c.JSON(fiber.Map{
		"plan":                 sub.Plan,
		"status":               string(sub.Status),
		"messages_sent":        messagesSent,
		"message_limit":        messageLimit,
		"usage_percent":        usagePct,
		"current_period_start": sub.CurrentPeriodStart.Format(time.RFC3339),
		"current_period_end":   sub.CurrentPeriodEnd.Format(time.RFC3339),
		"days_remaining":       daysRemaining,
	})
}

// GetSubscription handles GET /v1/billing/subscription
func (h *BillingHandler) GetSubscription(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	now := time.Now().UTC()
	sub, err := h.subRepo.GetActiveSubscription(c.Context(), userID, "", now)
	if err != nil {
		h.logger.Error("failed to get subscription", zap.String("user_id", userID), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to retrieve subscription"})
	}

	if sub == nil {
		// Also check for an expired trial
		subs, listErr := h.subRepo.List(c.Context(), license.SubscriptionFilter{
			TenantID: userID,
			Limit:    1,
		})
		if listErr == nil && len(subs) > 0 {
			return c.JSON(subs[0])
		}
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "no_subscription_found"})
	}

	return c.JSON(sub)
}

// AcceptTrial handles POST /v1/billing/accept-trial
// Marks the trial as accepted by the user (sets trial_accepted_at in metadata).
func (h *BillingHandler) AcceptTrial(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	now := time.Now().UTC()
	sub, err := h.subRepo.GetActiveSubscription(c.Context(), userID, "", now)
	if err != nil {
		h.logger.Error("failed to get subscription for trial accept", zap.String("user_id", userID), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to retrieve subscription"})
	}

	if sub == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "no_active_trial_found"})
	}

	if sub.Metadata == nil {
		sub.Metadata = make(map[string]interface{})
	}
	sub.Metadata["trial_accepted_at"] = now.Format(time.RFC3339)

	if err := h.subRepo.Update(c.Context(), sub); err != nil {
		h.logger.Error("failed to update trial acceptance", zap.String("user_id", userID), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to accept trial"})
	}

	messageLimit := metaInt(sub.Metadata, "message_limit", 10000)
	daysRemaining := int(sub.CurrentPeriodEnd.Sub(now).Hours() / 24)
	if daysRemaining < 0 {
		daysRemaining = 0
	}

	return c.JSON(fiber.Map{
		"accepted":           true,
		"plan":               sub.Plan,
		"status":             string(sub.Status),
		"message_limit":      messageLimit,
		"current_period_end": sub.CurrentPeriodEnd.Format(time.RFC3339),
		"days_remaining":     daysRemaining,
		"trial_accepted_at":  now.Format(time.RFC3339),
	})
}

// metaInt safely reads an int from a metadata map, returning defaultVal if missing/wrong type.
func metaInt(meta map[string]interface{}, key string, defaultVal int) int {
	if meta == nil {
		return defaultVal
	}
	v, ok := meta[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	}
	return defaultVal
}
