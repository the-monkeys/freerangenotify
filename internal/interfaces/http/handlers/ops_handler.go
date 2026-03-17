package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

// OpsHandler exposes privileged operational APIs behind OpsAuth.
type OpsHandler struct {
	authService auth.Service
	subRepo     license.Repository
	logger      *zap.Logger
}

func NewOpsHandler(authService auth.Service, subRepo license.Repository, logger *zap.Logger) *OpsHandler {
	return &OpsHandler{authService: authService, subRepo: subRepo, logger: logger}
}

// RenewSubscription handles POST /v1/ops/subscriptions/renew.
func (h *OpsHandler) RenewSubscription(c *fiber.Ctx) error {
	if h.subRepo == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "subscription repository unavailable"})
	}

	var req struct {
		UserID   string `json:"user_id"`
		TenantID string `json:"tenant_id"`
		AppID    string `json:"app_id"`
		Months   int    `json:"months"`
		Plan     string `json:"plan"`
		Reason   string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.Months <= 0 {
		req.Months = 1
	}
	if req.Months > 24 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "months must be between 1 and 24"})
	}

	tenantID := req.TenantID
	if tenantID == "" {
		tenantID = req.UserID
	}
	if tenantID == "" && req.AppID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id, tenant_id or app_id is required"})
	}

	plan := req.Plan
	if plan == "" {
		plan = "ops_granted"
	}

	now := time.Now().UTC()
	h.logger.Info("Ops renew request received",
		zap.String("tenant_id", tenantID),
		zap.String("app_id", req.AppID),
		zap.Int("months", req.Months),
		zap.String("plan", plan),
		zap.String("reason", req.Reason),
	)

	sub, err := h.subRepo.GetActiveSubscription(c.Context(), tenantID, req.AppID, now)
	if err != nil {
		h.logger.Error("failed to resolve active subscription for renewal",
			zap.String("tenant_id", tenantID),
			zap.String("app_id", req.AppID),
			zap.Error(err),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to resolve subscription"})
	}

	if sub == nil {
		sub = &license.Subscription{
			ID:                 uuid.New().String(),
			TenantID:           tenantID,
			AppID:              req.AppID,
			Plan:               plan,
			Status:             license.SubscriptionStatusActive,
			CurrentPeriodStart: now,
			CurrentPeriodEnd:   now.AddDate(0, req.Months, 0),
			Metadata: map[string]interface{}{
				"source": "ops_cli",
				"reason": req.Reason,
			},
		}
		if err := h.subRepo.Create(c.Context(), sub); err != nil {
			h.logger.Error("failed to create subscription during renewal", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create subscription"})
		}
		h.logger.Info("Ops renew created new subscription", zap.String("subscription_id", sub.ID), zap.String("tenant_id", tenantID), zap.String("app_id", req.AppID))
	} else {
		if sub.Metadata == nil {
			sub.Metadata = map[string]interface{}{}
		}
		sub.Metadata["source"] = "ops_cli"
		sub.Metadata["reason"] = req.Reason
		if req.Plan != "" {
			sub.Plan = req.Plan
		}
		if sub.Status != license.SubscriptionStatusActive && sub.Status != license.SubscriptionStatusTrial {
			sub.Status = license.SubscriptionStatusActive
		}

		base := now
		if sub.CurrentPeriodEnd.After(now) {
			base = sub.CurrentPeriodEnd
		}
		sub.CurrentPeriodEnd = base.AddDate(0, req.Months, 0)

		if err := h.subRepo.Update(c.Context(), sub); err != nil {
			h.logger.Error("failed to update subscription during renewal", zap.Error(err), zap.String("subscription_id", sub.ID))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update subscription"})
		}
		h.logger.Info("Ops renew updated existing subscription", zap.String("subscription_id", sub.ID), zap.String("tenant_id", tenantID), zap.String("app_id", req.AppID))
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    sub,
	})
}

// DeleteAccount handles DELETE /v1/ops/users/:user_id.
func (h *OpsHandler) DeleteAccount(c *fiber.Ctx) error {
	if h.authService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "auth service unavailable"})
	}

	userID := c.Params("user_id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id is required"})
	}

	reason := c.Query("reason")
	h.logger.Info("Ops delete request received", zap.String("user_id", userID), zap.String("reason", reason))
	if err := h.authService.DeleteAccountByAdmin(c.Context(), userID, reason); err != nil {
		return err
	}
	h.logger.Info("Ops delete completed", zap.String("user_id", userID))

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"user_id": userID,
			"deleted": true,
		},
	})
}
