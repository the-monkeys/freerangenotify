package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

// BillingHandler serves user-facing billing and subscription APIs.
type BillingHandler struct {
	subRepo        license.Repository
	appRepo        application.Repository
	usageRepo      billing.UsageRepository
	rateCard       map[string]billing.PlanTier
	billingEnabled bool
	logger         *zap.Logger
}

func NewBillingHandler(subRepo license.Repository, appRepo application.Repository, rateCard map[string]billing.PlanTier, logger *zap.Logger) *BillingHandler {
	return &BillingHandler{
		subRepo:  subRepo,
		appRepo:  appRepo,
		rateCard: rateCard,
		logger:   logger,
	}
}

// SetUsageRepo wires the ES usage repository into the billing handler.
// Called from container.go when billing is enabled.
func (h *BillingHandler) SetUsageRepo(repo billing.UsageRepository, enabled bool) {
	h.usageRepo = repo
	h.billingEnabled = enabled
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

// GetUsageBreakdown handles GET /v1/billing/usage/breakdown
// Returns per-channel, per-credential-source usage totals for the current billing period.
// Returns {"billing_enabled": false} when billing is disabled — no error.
func (h *BillingHandler) GetUsageBreakdown(c *fiber.Ctx) error {
	if !h.billingEnabled || h.usageRepo == nil {
		return c.JSON(fiber.Map{"billing_enabled": false})
	}

	userID := c.Locals("user_id").(string)
	now := time.Now().UTC()

	// Resolve current period via subscription
	sub, err := h.subRepo.GetActiveSubscription(c.Context(), userID, "", now)
	if err != nil {
		h.logger.Error("billing breakdown: failed to get subscription",
			zap.String("user_id", userID), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to retrieve subscription"})
	}

	var from, to time.Time
	if sub != nil {
		from = sub.CurrentPeriodStart
		to = sub.CurrentPeriodEnd
	} else {
		// No active sub — default to current calendar month
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		to = now
	}

	// Billing events are anchored to AppID (Workspace/Tenant). Look up all apps owned by this user.
	apps, err := h.appRepo.List(c.Context(), application.ApplicationFilter{
		AdminUserID: userID,
	})
	if err != nil {
		h.logger.Error("billing breakdown: failed to get applications", zap.String("user_id", userID), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to retrieve applications"})
	}

	var matchAppIDs []string
	for _, app := range apps {
		matchAppIDs = append(matchAppIDs, app.AppID)
	}

	summaries, err := h.usageRepo.GetSummary(c.Context(), matchAppIDs, from, to)
	if err != nil {
		h.logger.Error("billing breakdown: failed to get usage summary",
			zap.String("user_id", userID), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to retrieve usage breakdown"})
	}

	// Get the user's plan tier
	var plan billing.PlanTier
	if sub != nil && sub.Plan != "" {
		if p, ok := h.rateCard[sub.Plan]; ok {
			plan = p
		} else {
			plan = h.rateCard["free_trial"]
		}
	} else {
		plan = h.rateCard["free_trial"]
	}

	// Convert paisa to INR for the API response
	type breakdownItem struct {
		Channel          string  `json:"channel"`
		CredentialSource string  `json:"credential_source"`
		MessageCount     int64   `json:"message_count"`
		TotalBilledINR   float64 `json:"total_billed_inr"`
		PeriodStart      string  `json:"period_start"`
		PeriodEnd        string  `json:"period_end"`
	}

	items := make([]breakdownItem, 0, len(summaries))
	for _, s := range summaries {
        var billedPaisa int64

		if s.CredentialSource == billing.CredSourceSystem {
            quota := plan.IncludedQuotas[s.Channel]
            overage := s.MessageCount - quota
            if overage > 0 {
                billedPaisa = overage * plan.OverageRates[s.Channel]
            }
		} else if s.CredentialSource == billing.CredSourceBYOC {
			billedPaisa = s.MessageCount * plan.BYOCFees[s.Channel]
		} else if s.CredentialSource == billing.CredSourcePlatform {
            billedPaisa = s.MessageCount * plan.PlatformFees[s.Channel]
        }

		items = append(items, breakdownItem{
			Channel:          s.Channel,
			CredentialSource: s.CredentialSource,
			MessageCount:     s.MessageCount,
			TotalBilledINR:   float64(billedPaisa) / 100.0,
			PeriodStart:      s.PeriodStart,
			PeriodEnd:        s.PeriodEnd,
		})
	}

	// Build per-channel quota usage tracking
	// Track how many system-cred messages were used per channel
	systemUsageByChannel := make(map[string]int64)
	for _, s := range summaries {
		if s.CredentialSource == billing.CredSourceSystem {
			systemUsageByChannel[s.Channel] += s.MessageCount
		}
	}

	type quotaItem struct {
		Channel   string `json:"channel"`
		Included  int64  `json:"included"`
		Used      int64  `json:"used"`
		Remaining int64  `json:"remaining"`
	}

	quotas := make([]quotaItem, 0)
	for channel, included := range plan.IncludedQuotas {
		used := systemUsageByChannel[channel]
		remaining := included - used
		if remaining < 0 {
			remaining = 0
		}
		quotas = append(quotas, quotaItem{
			Channel:   channel,
			Included:  included,
			Used:      used,
			Remaining: remaining,
		})
	}

	return c.JSON(fiber.Map{
		"billing_enabled": true,
		"plan":            plan.Name,
		"period_start":    from.Format(time.RFC3339),
		"period_end":      to.Format(time.RFC3339),
		"breakdown":       items,
		"quotas":          quotas,
	})
}

// GetRates handles GET /v1/billing/rates
// Returns the current pricing rate card (system-cred overage, BYOC platform fees).
func (h *BillingHandler) GetRates(c *fiber.Ctx) error {
	type planInfo struct {
		Name           string             `json:"name"`
		MonthlyFeeINR  float64            `json:"monthly_fee_inr"`
		IncludedQuotas map[string]int64   `json:"included_quotas"`
		OverageINR     map[string]float64 `json:"overage_rates_inr"`
		BYOCFeeINR     map[string]float64 `json:"byoc_platform_fee_inr"`
	}

	plans := make([]planInfo, 0, len(h.rateCard))
	for _, p := range h.rateCard {
		overageINR := make(map[string]float64, len(p.OverageRates))
		for ch, paisa := range p.OverageRates {
			overageINR[ch] = float64(paisa) / 100.0
		}
		byocINR := make(map[string]float64, len(p.BYOCFees))
		for ch, paisa := range p.BYOCFees {
			byocINR[ch] = float64(paisa) / 100.0
		}

		plans = append(plans, planInfo{
			Name:           p.Name,
			MonthlyFeeINR:  float64(p.MonthlyFeePaisa) / 100.0,
			IncludedQuotas: p.IncludedQuotas,
			OverageINR:     overageINR,
			BYOCFeeINR:     byocINR,
		})
	}

	return c.JSON(fiber.Map{
		"currency":     "INR",
		"plans":        plans,
		"last_updated": "2026-01-01",
	})
}

