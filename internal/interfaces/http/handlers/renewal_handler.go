package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

// RenewalHandler handles admin-initiated subscription renewals.
type RenewalHandler struct {
	subRepo      license.Repository
	appRepo      application.Repository
	usageRepo    billing.UsageRepository
	rateCard     map[string]billing.PlanTier
	usageEnabled bool
	logger       *zap.Logger
}

// NewRenewalHandler creates a new RenewalHandler.
func NewRenewalHandler(
	subRepo license.Repository,
	appRepo application.Repository,
	rateCard map[string]billing.PlanTier,
	logger *zap.Logger,
) *RenewalHandler {
	return &RenewalHandler{
		subRepo:  subRepo,
		appRepo:  appRepo,
		rateCard: rateCard,
		logger:   logger,
	}
}

// SetUsageRepo wires metered usage into admin renewals for rollover calculation.
func (h *RenewalHandler) SetUsageRepo(repo billing.UsageRepository, enabled bool) {
	h.usageRepo = repo
	h.usageEnabled = enabled
}

// AdminRenew handles POST /v1/admin/subscriptions/:id/renew
// Renews a subscription for the specified number of months without requiring payment.
// Protected by admin/ops auth — NOT public.
func (h *RenewalHandler) AdminRenew(c *fiber.Ctx) error {
	subID := c.Params("id")
	if subID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "subscription id is required"})
	}

	var req struct {
		Plan   string `json:"plan"`   // optional, defaults to current plan
		Months int    `json:"months"` // optional, defaults to 1
		Reason string `json:"reason"` // required audit trail
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Months <= 0 {
		req.Months = 1
	}

	sub, err := h.subRepo.GetByID(c.Context(), subID)
	if err != nil || sub == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "subscription not found"})
	}

	now := time.Now().UTC()
	planName := sub.Plan
	if req.Plan != "" {
		planName = req.Plan
	}
	plan, ok := h.rateCard[planName]
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unknown plan tier"})
	}

	applySubscriptionRenewal(
		c.Context(),
		sub,
		sub.TenantID,
		h.rateCard,
		plan,
		req.Months,
		"admin_cli",
		h.appRepo,
		h.usageRepo,
		h.usageEnabled,
		map[string]interface{}{
			"renewal_reason": req.Reason,
			"renewed_by":     "admin_cli",
			"renewed_at":     now.Format(time.RFC3339),
		},
	)

	if err := h.subRepo.Update(c.Context(), sub); err != nil {
		h.logger.Error("admin renewal failed",
			zap.String("subscription_id", subID),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to renew subscription"})
	}

	h.logger.Info("subscription renewed via admin CLI",
		zap.String("subscription_id", subID),
		zap.String("tenant_id", sub.TenantID),
		zap.String("plan", sub.Plan),
		zap.Int("months", req.Months),
		zap.String("reason", req.Reason),
	)

	return c.JSON(fiber.Map{
		"success":              true,
		"message":              "subscription renewed",
		"subscription_id":      sub.ID,
		"plan":                 sub.Plan,
		"status":               string(sub.Status),
		"message_limit":        currentMessageLimit(sub, h.rateCard),
		"rollover_messages":    currentRolloverMessages(sub),
		"current_period_start": sub.CurrentPeriodStart.Format(time.RFC3339),
		"current_period_end":   sub.CurrentPeriodEnd.Format(time.RFC3339),
		"renewal_method":       "admin_cli",
	})
}
