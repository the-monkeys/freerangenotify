package handlers

import (
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/config"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/dashboard_notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

// OpsHandler exposes privileged operational APIs behind OpsAuth.
type OpsHandler struct {
	authService auth.Service
	subRepo     license.Repository
	appRepo     application.Repository
	notifier    dashboard_notification.Notifier
	smtp        config.SMTPConfig
	logger      *zap.Logger
}

func NewOpsHandler(authService auth.Service, subRepo license.Repository, appRepo application.Repository, notifier dashboard_notification.Notifier, smtp config.SMTPConfig, logger *zap.Logger) *OpsHandler {
	return &OpsHandler{authService: authService, subRepo: subRepo, appRepo: appRepo, notifier: notifier, smtp: smtp, logger: logger}
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

	h.notifySubscriptionRenewed(c, sub, req.UserID, req.Months)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    sub,
	})
}

func (h *OpsHandler) notifySubscriptionRenewed(c *fiber.Ctx, sub *license.Subscription, requestUserID string, months int) {
	if sub == nil {
		return
	}

	ownerIDs := make(map[string]struct{})
	if strings.TrimSpace(requestUserID) != "" {
		ownerIDs[strings.TrimSpace(requestUserID)] = struct{}{}
	}
	if strings.TrimSpace(sub.TenantID) != "" && h.authService != nil {
		if _, err := h.authService.GetCurrentUser(c.Context(), sub.TenantID); err == nil {
			ownerIDs[strings.TrimSpace(sub.TenantID)] = struct{}{}
		}
	}
	if strings.TrimSpace(sub.AppID) != "" && h.appRepo != nil {
		if app, err := h.appRepo.GetByID(c.Context(), sub.AppID); err == nil && app != nil && strings.TrimSpace(app.AdminUserID) != "" {
			ownerIDs[strings.TrimSpace(app.AdminUserID)] = struct{}{}
		}
	}

	if len(ownerIDs) == 0 {
		return
	}

	for userID := range ownerIDs {
		if h.notifier != nil {
			title := "Subscription renewed"
			body := fmt.Sprintf("Your subscription was renewed for %d month(s). New end date: %s", months, sub.CurrentPeriodEnd.Format("2006-01-02"))
			if err := h.notifier.NotifyUser(c.Context(), userID, title, body, "billing_subscription_renewed", map[string]interface{}{
				"event_code":         "billing.subscription_renewed",
				"subscription_id":    sub.ID,
				"tenant_id":          sub.TenantID,
				"app_id":             sub.AppID,
				"current_period_end": sub.CurrentPeriodEnd.Format(time.RFC3339),
				"months":             months,
			}); err != nil {
				h.logger.Warn("Failed to create subscription renewed in-app notification", zap.String("user_id", userID), zap.Error(err))
			}
		}

		if h.authService != nil {
			adminUser, err := h.authService.GetCurrentUser(c.Context(), userID)
			if err != nil || adminUser == nil || strings.TrimSpace(adminUser.Email) == "" {
				continue
			}
			if err := h.sendSubscriptionRenewedEmail(adminUser.Email, adminUser.FullName, months, sub.CurrentPeriodEnd); err != nil {
				h.logger.Warn("Failed to send subscription renewed email", zap.String("user_id", userID), zap.String("email", adminUser.Email), zap.Error(err))
			}
		}
	}
}

func (h *OpsHandler) sendSubscriptionRenewedEmail(toEmail, fullName string, months int, end time.Time) error {
	if strings.TrimSpace(h.smtp.Host) == "" || strings.TrimSpace(h.smtp.FromEmail) == "" {
		return nil
	}

	name := strings.TrimSpace(fullName)
	if name == "" {
		name = "there"
	}

	subject := "Subscription renewed"
	body := fmt.Sprintf("<p>Hi %s,</p><p>Your subscription has been renewed for %d month(s).</p><p>Next renewal date: <strong>%s</strong></p>", name, months, end.Format("2006-01-02"))

	fromName := strings.TrimSpace(h.smtp.FromName)
	if fromName == "" {
		fromName = "FreeRange Notify"
	}
	msg := fmt.Sprintf("From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n%s", fromName, h.smtp.FromEmail, toEmail, subject, body)

	port := h.smtp.Port
	if port == 0 {
		port = 587
	}
	addr := fmt.Sprintf("%s:%d", h.smtp.Host, port)

	var auth smtp.Auth
	if strings.TrimSpace(h.smtp.Username) != "" && strings.TrimSpace(h.smtp.Password) != "" {
		auth = smtp.PlainAuth("", h.smtp.Username, h.smtp.Password, h.smtp.Host)
	}

	if err := smtp.SendMail(addr, auth, h.smtp.FromEmail, []string{toEmail}, []byte(msg)); err != nil {
		return fmt.Errorf("send subscription renewed email: %w", err)
	}
	return nil
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
