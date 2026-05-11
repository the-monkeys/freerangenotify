package handlers

import (
	"math"
	"sort"
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
	rateCardMgr    billing.RateCardManager
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

func (h *BillingHandler) SetRateCardManager(manager billing.RateCardManager) {
	h.rateCardMgr = manager
}

// GetUsage handles GET /v1/billing/usage
// Returns current period usage for the authenticated user's personal workspace.
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

	creditsTotal := sub.CreditsTotal
	if creditsTotal <= 0 {
		creditsTotal = resolvePlan(h.rateCard, sub.Plan).CreditsIncluded
	}
	creditsRemaining := sub.CreditsRemaining
	var creditsConsumed int64
	var messagesSent int64

	// Derive usage from metering when available.
	if h.billingEnabled && h.usageRepo != nil && h.appRepo != nil {
		apps, appErr := h.appRepo.List(c.Context(), application.ApplicationFilter{AdminUserID: userID})
		if appErr == nil {
			appIDs := make([]string, 0, len(apps))
			for _, app := range apps {
				appIDs = append(appIDs, app.AppID)
			}
			summaries, summaryErr := h.usageRepo.GetSummary(c.Context(), appIDs, sub.CurrentPeriodStart, sub.CurrentPeriodEnd)
			if summaryErr == nil {
				for _, s := range summaries {
					messagesSent += s.MessageCount
					if s.CredentialSource == billing.CredSourceSystem {
						creditsConsumed += s.CreditsConsumed
					}
				}
			}
		}
	}
	if messagesSent == 0 {
		messagesSent = int64(subscriptionMessagesSent(c.Context(), userID, sub, h.appRepo, h.usageRepo, h.billingEnabled))
	}
	if creditsConsumed == 0 && creditsTotal > 0 && creditsRemaining <= creditsTotal {
		creditsConsumed = creditsTotal - creditsRemaining
	}

	usagePct := 0.0
	if creditsTotal > 0 {
		usagePct = float64(creditsConsumed) / float64(creditsTotal) * 100
	}

	// Use Ceiling to ensure consistent "29 days" display as per user request
	daysRemaining := int(math.Ceil(sub.CurrentPeriodEnd.Sub(now).Hours() / 24.0))
	if daysRemaining < 0 {
		daysRemaining = 0
	}

	return c.JSON(fiber.Map{
		"plan":                 sub.Plan,
		"status":               string(sub.Status),
		"messages_sent":        messagesSent,
		"credits_consumed":     creditsConsumed,
		"credits_remaining":    creditsRemaining,
		"credits_total":        creditsTotal,
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

	daysRemaining := int(math.Ceil(sub.CurrentPeriodEnd.Sub(now).Hours() / 24.0))
	if daysRemaining < 0 {
		daysRemaining = 0
	}

	return c.JSON(fiber.Map{
		"accepted":           true,
		"plan":               sub.Plan,
		"status":             string(sub.Status),
		"credits_total":      sub.CreditsTotal,
		"credits_remaining":  sub.CreditsRemaining,
		"credits_expire_at":  sub.CreditsExpireAt,
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

func metaString(meta map[string]interface{}, key, defaultVal string) string {
	if meta == nil {
		return defaultVal
	}
	v, ok := meta[key]
	if !ok {
		return defaultVal
	}
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return defaultVal
}

func cloneInt64MapLocal(src map[string]int64) map[string]int64 {
	dst := make(map[string]int64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
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

	type breakdownItem struct {
		Channel         string  `json:"channel"`
		MessageCount    int64   `json:"message_count"`
		CreditsConsumed int64   `json:"credits_consumed"`
		OverageAmount   float64 `json:"overage_amount"`
	}

	perChannel := make(map[string]*breakdownItem)
	var totalCreditsConsumed int64
	for _, s := range summaries {
		entry, ok := perChannel[s.Channel]
		if !ok {
			entry = &breakdownItem{Channel: s.Channel}
			perChannel[s.Channel] = entry
		}
		entry.MessageCount += s.MessageCount
		entry.CreditsConsumed += s.CreditsConsumed
		entry.OverageAmount += float64(s.OverageAmount) / 100.0
		totalCreditsConsumed += s.CreditsConsumed
	}
	items := make([]breakdownItem, 0, len(perChannel))
	for _, entry := range perChannel {
		items = append(items, *entry)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Channel < items[j].Channel })

	creditsTotal := sub.CreditsTotal
	if creditsTotal <= 0 {
		creditsTotal = resolvePlan(h.rateCard, sub.Plan).CreditsIncluded
	}
	creditsRemaining := sub.CreditsRemaining
	if creditsRemaining <= 0 && creditsTotal > 0 && totalCreditsConsumed <= creditsTotal {
		creditsRemaining = creditsTotal - totalCreditsConsumed
	}

	return c.JSON(fiber.Map{
		"billing_enabled":   true,
		"plan":              sub.Plan,
		"period_start":      from.Format(time.RFC3339),
		"period_end":        to.Format(time.RFC3339),
		"credits_total":     creditsTotal,
		"credits_consumed":  totalCreditsConsumed,
		"credits_remaining": creditsRemaining,
		"breakdown":         items,
	})
}

// GetRates handles GET /v1/billing/rates
// Returns the active rate-card version and canonical channel credit costs.
func (h *BillingHandler) GetRates(c *fiber.Ctx) error {
	var active *billing.RateCard
	if h.rateCardMgr != nil {
		active = h.rateCardMgr.GetActiveRateCard()
		if active == nil {
			_ = h.rateCardMgr.RefreshActiveRateCard(c.Context())
			active = h.rateCardMgr.GetActiveRateCard()
		}
	}
	if active == nil {
		pro := resolvePlan(h.rateCard, "pro")
		active = &billing.RateCard{
			Version:           "default",
			CreditValueINR:    pro.CreditValueINR,
			ChannelCreditCost: cloneInt64MapLocal(pro.ChannelCreditCost),
			OveragePerMessage: cloneInt64MapLocal(pro.OveragePerMessage),
			UpdatedAt:         time.Now().UTC(),
		}
	}
	overageINR := make(map[string]float64, len(active.OveragePerMessage))
	for ch, paisa := range active.OveragePerMessage {
		overageINR[ch] = float64(paisa) / 100.0
	}

	return c.JSON(fiber.Map{
		"currency":             "INR",
		"active_version":       active.Version,
		"effective_at":         active.UpdatedAt.Format(time.RFC3339),
		"credit_value_inr":     active.CreditValueINR,
		"channel_credit_cost":  active.ChannelCreditCost,
		"overage_per_message":  overageINR,
		"free_tier_daily_caps": map[string]int64{"sms": 3, "whatsapp": 2},
	})
}

// AdminGetRates handles GET /v1/admin/billing/rates
func (h *BillingHandler) AdminGetRates(c *fiber.Ctx) error {
	if h.rateCardMgr == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "rate card service not configured"})
	}

	card := h.rateCardMgr.GetActiveRateCard()
	if card == nil {
		if err := h.rateCardMgr.RefreshActiveRateCard(c.Context()); err != nil {
			h.logger.Error("admin billing rates: refresh failed", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to refresh rate card"})
		}
		card = h.rateCardMgr.GetActiveRateCard()
	}

	return c.JSON(fiber.Map{
		"active": card,
	})
}

// AdminSetRate handles POST /v1/admin/billing/rates/set
func (h *BillingHandler) AdminSetRate(c *fiber.Ctx) error {
	if h.rateCardMgr == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "rate card service not configured"})
	}
	var req struct {
		Channel string `json:"channel"`
		Credits int64  `json:"credits"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Channel == "" || req.Credits <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "channel and credits (>0) are required"})
	}

	card, err := h.rateCardMgr.UpdateChannelCredits(c.Context(), req.Channel, req.Credits)
	if err != nil {
		h.logger.Error("admin billing rates: set failed", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"updated": card,
	})
}

// AdminActivateRate handles POST /v1/admin/billing/rates/activate
func (h *BillingHandler) AdminActivateRate(c *fiber.Ctx) error {
	if h.rateCardMgr == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "rate card service not configured"})
	}
	var req struct {
		Version string `json:"version"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Version == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "version is required"})
	}

	if err := h.rateCardMgr.ActivateVersion(c.Context(), req.Version); err != nil {
		h.logger.Error("admin billing rates: activate failed", zap.Error(err), zap.String("version", req.Version))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"activated_version": req.Version,
		"active":            h.rateCardMgr.GetActiveRateCard(),
	})
}

// AdminRollbackRate handles POST /v1/admin/billing/rates/rollback
func (h *BillingHandler) AdminRollbackRate(c *fiber.Ctx) error {
	// Rollback is activate-by-version, with explicit endpoint for operational intent clarity.
	return h.AdminActivateRate(c)
}
