package handlers

import (
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
		Tier string `json:"tier" validate:"required"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Tier == "" {
		req.Tier = "pro"
	}

	// Look up plan pricing
	plan, ok := h.rateCard[req.Tier]
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unknown plan tier: " + req.Tier,
		})
	}

	// Check for existing active subscription — prevent double payment
	now := time.Now().UTC()
	existingSub, _ := h.subRepo.GetActiveSubscription(c.Context(), userID, "", now)
	if existingSub != nil && existingSub.Status == license.SubscriptionStatusActive {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error":              "active subscription already exists",
			"current_period_end": existingSub.CurrentPeriodEnd.Format(time.RFC3339),
		})
	}

	// Create payment order
	checkout, err := h.paymentProvider.CreateOrder(c.Context(), userID, req.Tier, plan.MonthlyFeePaisa)
	if err != nil {
		h.logger.Error("failed to create payment order",
			zap.String("user_id", userID),
			zap.String("tier", req.Tier),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create payment order",
		})
	}

	h.logger.Info("payment order created",
		zap.String("user_id", userID),
		zap.String("order_id", checkout.OrderID),
		zap.String("tier", req.Tier),
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
			Plan:               req.Tier,
			Status:             license.SubscriptionStatusPending,
			CurrentPeriodStart: now,
			CurrentPeriodEnd:   now,
			Metadata:           make(map[string]interface{}),
		}
	}
	if sub.Metadata == nil {
		sub.Metadata = make(map[string]interface{})
	}
	sub.Metadata["pending_checkout_tier"] = req.Tier
	sub.Metadata["pending_checkout_order_id"] = checkout.OrderID
	sub.Metadata["pending_checkout_amount"] = checkout.AmountINR
	sub.Metadata["pending_checkout_currency"] = checkout.Currency
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
		"tier":         checkout.Tier,
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

	planName := metaString(sub.Metadata, "pending_checkout_tier", "")
	if planName == "" && sub.Plan != "" && sub.Plan != "free_trial" {
		planName = sub.Plan
	}
	if planName == "" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "pending checkout tier is missing; create a new checkout session and try again",
		})
	}

	plan, ok := h.rateCard[planName]
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "unknown plan tier: " + planName,
		})
	}

	applySubscriptionRenewal(
		c.Context(),
		sub,
		userID,
		h.rateCard,
		plan,
		1,
		"razorpay",
		h.appRepo,
		h.usageRepo,
		h.usageEnabled,
		map[string]interface{}{
			"last_payment_id": req.PaymentID,
			"last_order_id":   req.OrderID,
			"last_payment_at": now.Format(time.RFC3339),
		},
	)
	delete(sub.Metadata, "pending_checkout_tier")
	delete(sub.Metadata, "pending_checkout_order_id")
	delete(sub.Metadata, "pending_checkout_amount")
	delete(sub.Metadata, "pending_checkout_currency")
	delete(sub.Metadata, "pending_checkout_created_at")
	delete(sub.Metadata, "pending_checkout_provider")

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
		"message_limit":        currentMessageLimit(sub, h.rateCard),
		"rollover_messages":    currentRolloverMessages(sub),
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

			planName := event.Tier
			if planName == "" {
				planName = metaString(sub.Metadata, "pending_checkout_tier", "")
			}
			if planName == "" && sub.Plan != "" && sub.Plan != "free_trial" {
				planName = sub.Plan
			}
			if planName == "" {
				planName = "pro"
			}
			plan, ok := h.rateCard[planName]
			if !ok {
				plan = h.rateCard["pro"]
			}

			applySubscriptionRenewal(
				c.Context(),
				sub,
				event.TenantID,
				h.rateCard,
				plan,
				1,
				"razorpay_webhook",
				h.appRepo,
				h.usageRepo,
				h.usageEnabled,
				map[string]interface{}{
					"webhook_payment_id": event.PaymentID,
					"last_payment_id":    event.PaymentID,
					"last_order_id":      event.OrderID,
					"last_payment_at":    now.Format(time.RFC3339),
				},
			)
			delete(sub.Metadata, "pending_checkout_tier")
			delete(sub.Metadata, "pending_checkout_order_id")
			delete(sub.Metadata, "pending_checkout_amount")
			delete(sub.Metadata, "pending_checkout_currency")
			delete(sub.Metadata, "pending_checkout_created_at")
			delete(sub.Metadata, "pending_checkout_provider")

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
