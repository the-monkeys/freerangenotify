package handlers

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

// PaymentHandler handles payment-related HTTP requests.
type PaymentHandler struct {
	paymentProvider billing.Provider
	subRepo         license.Repository
	appRepo         application.Repository
	usageRepo       billing.UsageRepository
	rateCardMgr     billing.RateCardManager
	rateCard        map[string]billing.PlanTier
	billingEnabled  bool
	usageEnabled    bool
	logger          *zap.Logger
}

// NewPaymentHandler creates a new PaymentHandler.
func NewPaymentHandler(
	provider billing.Provider,
	subRepo license.Repository,
	appRepo application.Repository,
	rateCard map[string]billing.PlanTier,
	billingEnabled bool,
	logger *zap.Logger,
) *PaymentHandler {
	return &PaymentHandler{
		paymentProvider: provider,
		subRepo:         subRepo,
		appRepo:         appRepo,
		rateCard:        rateCard,
		billingEnabled:  billingEnabled,
		logger:          logger,
	}
}

// SetUsageRepo wires the metered usage repository into payment renewal logic.
func (h *PaymentHandler) SetUsageRepo(repo billing.UsageRepository, enabled bool) {
	h.usageRepo = repo
	h.usageEnabled = enabled
}

func (h *PaymentHandler) SetRateCardManager(manager billing.RateCardManager) {
	h.rateCardMgr = manager
}

func isPaidActiveSubscription(sub *license.Subscription, now time.Time) bool {
	if sub == nil || !sub.IsActiveAt(now) {
		return false
	}
	return !billing.IsFreeTierPlan(sub.Plan)
}

func (h *PaymentHandler) resolveCheckoutPlan(planID string) (billing.PlanBundle, bool) {
	if h.rateCardMgr != nil {
		if plan, ok := h.rateCardMgr.GetCheckoutPlan(planID); ok {
			return plan, true
		}
	}
	id := planID
	if id == "" {
		id = "pro"
	}
	if plan, ok := billing.DefaultPlanBundles()[id]; ok {
		return plan, true
	}
	if plan, ok := billing.ResolveCheckoutPlan(id); ok {
		return billing.PlanBundle{
			ID:              id,
			Name:            id,
			AmountPaisa:     plan.MonthlyFeePaisa,
			Currency:        "INR",
			CreditsIncluded: plan.CreditsIncluded,
			ValidityDays:    365,
			Active:          true,
		}, true
	}
	return billing.PlanBundle{}, false
}

func (h *PaymentHandler) rateCardVersion() string {
	if h.rateCardMgr == nil {
		return "default"
	}
	return h.rateCardMgr.GetRateCardVersion()
}

func (h *PaymentHandler) pendingCheckoutAllocation(sub *license.Subscription, method string) (paidCreditAllocation, bool) {
	if sub == nil {
		return paidCreditAllocation{}, false
	}
	planID := metaString(sub.Metadata, "pending_checkout_plan_id", "")
	if planID == "" {
		planID = metaString(sub.Metadata, "pending_checkout_tier", "")
	}
	if planID == "" && sub.Plan != "" && !billing.IsFreeTierPlan(sub.Plan) {
		planID = sub.Plan
	}
	if planID == "" {
		return paidCreditAllocation{}, false
	}

	credits := metaInt64(sub.Metadata, "pending_checkout_credits", 0)
	validityDays := metaInt(sub.Metadata, "pending_checkout_validity_days", 365)
	amountPaisa := metaInt64(sub.Metadata, "pending_checkout_amount_paisa", 0)
	if amountPaisa == 0 {
		amountPaisa = metaInt64(sub.Metadata, "pending_checkout_amount", 0)
	}
	currency := metaString(sub.Metadata, "pending_checkout_currency", "INR")
	planName := metaString(sub.Metadata, "pending_checkout_plan_name", planID)
	rateCardVersion := metaString(sub.Metadata, "pending_checkout_rate_card_version", h.rateCardVersion())

	if credits <= 0 {
		if plan, ok := h.resolveCheckoutPlan(planID); ok {
			credits = plan.CreditsIncluded
			if validityDays <= 0 {
				validityDays = plan.ValidityDays
			}
			if amountPaisa <= 0 {
				amountPaisa = plan.AmountPaisa
			}
			if currency == "" {
				currency = plan.Currency
			}
			if planName == "" {
				planName = plan.Name
			}
		} else if plan, ok := billing.ResolveRenewalPlan(planID); ok {
			credits = plan.CreditsIncluded
			if amountPaisa <= 0 {
				amountPaisa = plan.MonthlyFeePaisa
			}
		}
	}
	if credits <= 0 {
		return paidCreditAllocation{}, false
	}
	if validityDays <= 0 {
		validityDays = 365
	}

	return paidCreditAllocation{
		PlanID:        planID,
		PlanName:      planName,
		Credits:       credits,
		ValidityDays:  validityDays,
		RenewalMethod: method,
		Metadata: map[string]interface{}{
			"last_paid_amount_paisa": amountPaisa,
			"last_paid_currency":     currency,
			"last_paid_credits":      credits,
			"last_paid_plan_id":      planID,
			"last_rate_card_version": rateCardVersion,
		},
	}, true
}

func clearPendingCheckoutMetadata(meta map[string]interface{}) {
	delete(meta, "pending_checkout_tier")
	delete(meta, "pending_checkout_plan_id")
	delete(meta, "pending_checkout_plan_name")
	delete(meta, "pending_checkout_order_id")
	delete(meta, "pending_checkout_amount")
	delete(meta, "pending_checkout_amount_paisa")
	delete(meta, "pending_checkout_currency")
	delete(meta, "pending_checkout_credits")
	delete(meta, "pending_checkout_validity_days")
	delete(meta, "pending_checkout_rate_card_version")
	delete(meta, "pending_checkout_created_at")
	delete(meta, "pending_checkout_provider")
}

func metaInt64(meta map[string]interface{}, key string, defaultVal int64) int64 {
	if meta == nil {
		return defaultVal
	}
	v, ok := meta[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case int:
		return int64(val)
	case int64:
		return val
	case float64:
		return int64(val)
	case json.Number:
		n, err := val.Int64()
		if err == nil {
			return n
		}
	}
	return defaultVal
}

// CreateOrder handles POST /v1/billing/checkout
// Creates a Razorpay order for subscription payment.
func (h *PaymentHandler) CreateOrder(c *fiber.Ctx) error {
	if !h.billingEnabled {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "billing is not enabled",
		})
	}

	userID := c.Locals("user_id").(string)

	var req struct {
		Tier   string `json:"tier" validate:"required"`
		PlanID string `json:"plan_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.PlanID == "" {
		req.PlanID = req.Tier
	}
	if req.PlanID == "" {
		req.PlanID = "pro"
	}
	req.Tier = req.PlanID

	plan, ok := h.resolveCheckoutPlan(req.PlanID)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unknown plan tier: " + req.PlanID,
		})
	}
	if plan.AmountPaisa <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "free plans do not require payment checkout",
		})
	}

	// Prevent double payment for already-paid plans while still allowing the
	// active Free credit subscription from the backfill to upgrade via checkout.
	now := time.Now().UTC()
	existingSub, _ := h.subRepo.GetActiveSubscription(c.Context(), userID, "", now)
	if isPaidActiveSubscription(existingSub, now) {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error":              "active subscription already exists",
			"current_period_end": existingSub.CurrentPeriodEnd.Format(time.RFC3339),
		})
	}

	// Create payment order
	checkout, err := h.paymentProvider.CreateOrder(c.Context(), userID, plan.ID, plan.AmountPaisa)
	if err != nil {
		h.logger.Error("failed to create payment order",
			zap.String("user_id", userID),
			zap.String("tier", plan.ID),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create payment order",
		})
	}

	h.logger.Info("payment order created",
		zap.String("user_id", userID),
		zap.String("order_id", checkout.OrderID),
		zap.String("tier", plan.ID),
	)

	sub, err := h.subRepo.GetActiveSubscription(c.Context(), userID, "", now)
	if err != nil {
		h.logger.Error("failed to load subscription before checkout",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to prepare payment checkout",
		})
	}
	if sub == nil {
		sub, err = latestSubscription(c.Context(), h.subRepo, userID)
		if err != nil {
			h.logger.Error("failed to load latest subscription before checkout",
				zap.String("user_id", userID),
				zap.Error(err),
			)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to prepare payment checkout",
			})
		}
	}
	if sub == nil {
		sub = &license.Subscription{
			ID:                 uuid.NewString(),
			TenantID:           userID,
			Plan:               plan.ID,
			Status:             license.SubscriptionStatusPending,
			CurrentPeriodStart: now,
			CurrentPeriodEnd:   now,
			Metadata:           make(map[string]interface{}),
		}
	}
	if sub.Metadata == nil {
		sub.Metadata = make(map[string]interface{})
	}
	sub.Metadata["pending_checkout_tier"] = plan.ID
	sub.Metadata["pending_checkout_plan_id"] = plan.ID
	sub.Metadata["pending_checkout_plan_name"] = plan.Name
	sub.Metadata["pending_checkout_order_id"] = checkout.OrderID
	sub.Metadata["pending_checkout_amount"] = checkout.AmountINR
	sub.Metadata["pending_checkout_amount_paisa"] = plan.AmountPaisa
	sub.Metadata["pending_checkout_currency"] = checkout.Currency
	sub.Metadata["pending_checkout_credits"] = plan.CreditsIncluded
	sub.Metadata["pending_checkout_validity_days"] = plan.ValidityDays
	sub.Metadata["pending_checkout_rate_card_version"] = h.rateCardVersion()
	sub.Metadata["pending_checkout_created_at"] = now.Format(time.RFC3339)
	sub.Metadata["pending_checkout_provider"] = "gateway"

	if sub.CreatedAt.IsZero() {
		if err := h.subRepo.Create(c.Context(), sub); err != nil {
			h.logger.Error("failed to persist pending checkout subscription",
				zap.String("user_id", userID),
				zap.String("order_id", checkout.OrderID),
				zap.Error(err),
			)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to persist checkout session",
			})
		}
	} else {
		if err := h.subRepo.Update(c.Context(), sub); err != nil {
			h.logger.Error("failed to persist pending checkout metadata",
				zap.String("user_id", userID),
				zap.String("order_id", checkout.OrderID),
				zap.Error(err),
			)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to persist checkout session",
			})
		}
	}

	return c.JSON(fiber.Map{
		"success":      true,
		"order_id":     checkout.OrderID,
		"amount":       checkout.AmountINR,
		"currency":     checkout.Currency,
		"key_id":       checkout.KeyID,
		"tier":         plan.ID,
		"plan_id":      plan.ID,
		"plan_name":    plan.Name,
		"credits":      plan.CreditsIncluded,
		"validity_days": plan.ValidityDays,
		"checkout_url": checkout.URL,
	})
}

// VerifyPayment handles POST /v1/billing/verify-payment
// Verifies Razorpay payment signature and activates the subscription.
func (h *PaymentHandler) VerifyPayment(c *fiber.Ctx) error {
	if !h.billingEnabled {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "billing is not enabled",
		})
	}

	userID := c.Locals("user_id").(string)

	var req billing.PaymentVerification
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.OrderID == "" || req.PaymentID == "" || req.Signature == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "razorpay_order_id, razorpay_payment_id, and razorpay_signature are required",
		})
	}

	// Verify the payment signature
	if err := h.paymentProvider.VerifyPayment(c.Context(), req); err != nil {
		h.logger.Warn("payment verification failed",
			zap.String("user_id", userID),
			zap.String("order_id", req.OrderID),
			zap.Error(err),
		)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "payment verification failed: invalid signature",
		})
	}

	// Activate subscription
	now := time.Now().UTC()

	// Find existing subscription to update, or create new
	sub, _ := h.subRepo.GetActiveSubscription(c.Context(), userID, "", now)
	if sub == nil {
		// Look for expired/trial subscription to renew
		subs, _ := h.subRepo.List(c.Context(), license.SubscriptionFilter{
			TenantID: userID,
			Limit:    1,
		})
		if len(subs) > 0 {
			sub = subs[0]
		}
	}

	if sub == nil {
		h.logger.Error("no subscription found to activate after payment",
			zap.String("user_id", userID),
			zap.String("order_id", req.OrderID),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "payment succeeded but no subscription found — contact support",
		})
	}

	expectedOrderID := metaString(sub.Metadata, "pending_checkout_order_id", "")
	if expectedOrderID != "" && expectedOrderID != req.OrderID {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "payment order does not match the pending checkout",
		})
	}

	allocation, ok := h.pendingCheckoutAllocation(sub, "razorpay")
	if !ok {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "pending checkout tier is missing; create a new checkout session and try again",
		})
	}
	allocation.Metadata["last_payment_id"] = req.PaymentID
	allocation.Metadata["last_order_id"] = req.OrderID
	allocation.Metadata["last_payment_at"] = now.Format(time.RFC3339)
	applyPaidCreditAllocation(sub, allocation)
	clearPendingCheckoutMetadata(sub.Metadata)

	if err := h.subRepo.Update(c.Context(), sub); err != nil {
		h.logger.Error("failed to activate subscription after payment",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "payment succeeded but subscription activation failed — contact support",
		})
	}

	h.logger.Info("subscription activated via payment",
		zap.String("user_id", userID),
		zap.String("payment_id", req.PaymentID),
		zap.String("order_id", req.OrderID),
	)

	return c.JSON(fiber.Map{
		"success":              true,
		"message":              "payment verified, subscription activated",
		"plan":                 sub.Plan,
		"status":               string(sub.Status),
		"billing_model":        billing.BillingModelCredits,
		"message_limit":        currentMessageLimit(sub, h.rateCard),
		"credits_total":        sub.CreditsTotal,
		"credits_remaining":    sub.CreditsRemaining,
		"credits_expire_at":    sub.CreditsExpireAt,
		"current_period_start": sub.CurrentPeriodStart.Format(time.RFC3339),
		"current_period_end":   sub.CurrentPeriodEnd.Format(time.RFC3339),
	})
}

// HandleWebhook handles POST /v1/billing/webhook
// Processes Razorpay webhook events (payment.captured, payment.failed, etc.)
// This endpoint does NOT require JWT auth — it uses webhook signature verification.
func (h *PaymentHandler) HandleWebhook(c *fiber.Ctx) error {
	signature := c.Get("X-Razorpay-Signature")
	if signature == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing signature header"})
	}

	body := c.Body()
	if len(body) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "empty webhook body"})
	}

	event, err := h.paymentProvider.VerifyWebhook(body, signature)
	if err != nil {
		h.logger.Warn("webhook signature verification failed", zap.Error(err))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid webhook signature"})
	}

	h.logger.Info("razorpay webhook received",
		zap.String("event_type", event.EventType),
		zap.String("tenant_id", event.TenantID),
		zap.String("payment_id", event.PaymentID),
	)

	switch event.EventType {
	case "payment.captured":
		// Payment successful — ensure subscription is active
		// (This is a fallback; primary activation happens via VerifyPayment)
		if event.TenantID != "" {
			now := time.Now().UTC()
			sub, subErr := latestSubscription(c.Context(), h.subRepo, event.TenantID)
			if subErr != nil {
				h.logger.Error("webhook: failed to load subscription",
					zap.String("tenant_id", event.TenantID),
					zap.Error(subErr),
				)
				break
			}
			if sub == nil {
				sub = &license.Subscription{
					ID:                 uuid.NewString(),
					TenantID:           event.TenantID,
					Status:             license.SubscriptionStatusPending,
					CurrentPeriodStart: now,
					CurrentPeriodEnd:   now,
					Metadata:           make(map[string]interface{}),
				}
			}
			if metaString(sub.Metadata, "last_payment_id", "") == event.PaymentID {
				break
			}

			allocation, ok := h.pendingCheckoutAllocation(sub, "razorpay_webhook")
			if !ok && event.Tier != "" {
				if plan, planOK := h.resolveCheckoutPlan(event.Tier); planOK {
					allocation = paidCreditAllocation{
						PlanID:        plan.ID,
						PlanName:      plan.Name,
						Credits:       plan.CreditsIncluded,
						ValidityDays:  plan.ValidityDays,
						RenewalMethod: "razorpay_webhook",
						Metadata: map[string]interface{}{
							"last_paid_amount_paisa": plan.AmountPaisa,
							"last_paid_currency":     plan.Currency,
						},
					}
					ok = true
				}
			}
			if !ok {
				h.logger.Warn("webhook: no checkout snapshot found",
					zap.String("tenant_id", event.TenantID),
					zap.String("order_id", event.OrderID),
				)
				break
			}
			allocation.Metadata["webhook_payment_id"] = event.PaymentID
			allocation.Metadata["last_payment_id"] = event.PaymentID
			allocation.Metadata["last_order_id"] = event.OrderID
			allocation.Metadata["last_payment_at"] = now.Format(time.RFC3339)
			applyPaidCreditAllocation(sub, allocation)
			clearPendingCheckoutMetadata(sub.Metadata)

			var updateErr error
			if sub.CreatedAt.IsZero() {
				updateErr = h.subRepo.Create(c.Context(), sub)
			} else {
				updateErr = h.subRepo.Update(c.Context(), sub)
			}
			if updateErr != nil {
				h.logger.Error("webhook: failed to activate subscription",
					zap.String("tenant_id", event.TenantID),
					zap.Error(updateErr),
				)
			}
		}

	case "payment.failed":
		h.logger.Warn("razorpay payment failed",
			zap.String("tenant_id", event.TenantID),
			zap.String("payment_id", event.PaymentID),
		)
		// No action — subscription remains in current state

	default:
		h.logger.Debug("unhandled razorpay event", zap.String("event_type", event.EventType))
	}

	// Always return 200 to Razorpay to acknowledge receipt
	return c.JSON(fiber.Map{"status": "ok"})
}
