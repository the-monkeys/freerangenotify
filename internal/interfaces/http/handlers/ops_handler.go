package handlers

import (
	"errors"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/config"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/dashboard_notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// OpsHandler exposes privileged operational APIs behind OpsAuth.
type OpsHandler struct {
	authService   auth.Service
	subRepo       license.Repository
	appRepo       application.Repository
	creditService *services.CreditService
	rateCardMgr   billing.RateCardManager
	notifier      dashboard_notification.Notifier
	smtp          config.SMTPConfig
	logger        *zap.Logger
}

func NewOpsHandler(
	authService auth.Service,
	subRepo license.Repository,
	appRepo application.Repository,
	creditService *services.CreditService,
	notifier dashboard_notification.Notifier,
	smtp config.SMTPConfig,
	logger *zap.Logger,
) *OpsHandler {
	return &OpsHandler{
		authService:   authService,
		subRepo:       subRepo,
		appRepo:       appRepo,
		creditService: creditService,
		notifier:      notifier,
		smtp:          smtp,
		logger:        logger,
	}
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

// GrantCredits handles POST /v1/ops/credits/grant.
func (h *OpsHandler) GrantCredits(c *fiber.Ctx) error {
	if h.creditService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "credit service unavailable"})
	}

	var req struct {
		UserID  string `json:"user_id"`
		Credits int64  `json:"credits"`
		Reason  string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id is required"})
	}

	if h.authService != nil {
		if _, err := h.authService.GetCurrentUser(c.Context(), userID); err != nil {
			if pkgerrors.IsNotFound(err) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
			}
			h.logger.Error("failed to resolve user for credit grant", zap.String("user_id", userID), zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to resolve user"})
		}
	}

	h.logger.Info("Ops grant credits request received",
		zap.String("user_id", userID),
		zap.Int64("credits", req.Credits),
		zap.String("reason", req.Reason),
	)

	snap, err := h.creditService.GrantCredits(c.Context(), userID, req.Credits, req.Reason, map[string]interface{}{
		"ops_user_id": userID,
	})
	if err != nil {
		if isGrantCreditsClientError(err) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		h.logger.Error("failed to grant credits", zap.String("user_id", userID), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to grant credits"})
	}

	h.logger.Info("Ops grant credits completed",
		zap.String("user_id", userID),
		zap.Int64("credits_granted", req.Credits),
		zap.Int64("credits_remaining", snap.CreditsRemaining),
	)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"tenant_id":          snap.TenantID,
			"credits_total":      snap.CreditsTotal,
			"credits_remaining":  snap.CreditsRemaining,
			"credits_reserved":   snap.CreditsReserved,
			"credits_granted":    req.Credits,
		},
	})
}

func isGrantCreditsClientError(err error) bool {
	return errors.Is(err, services.ErrGrantTenantRequired) ||
		errors.Is(err, services.ErrGrantInvalidAmount) ||
		errors.Is(err, services.ErrGrantReasonRequired) ||
		errors.Is(err, services.ErrGrantNoActiveSubscription) ||
		errors.Is(err, services.ErrGrantLegacyBilling)
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

// SetRateCardManager wires the rate-card manager used by RebalanceCredits.
func (h *OpsHandler) SetRateCardManager(mgr billing.RateCardManager) {
	h.rateCardMgr = mgr
}

// RebalanceCredits handles POST /v1/ops/billing/rebalance-credits.
//
// One-shot migration that re-baselines existing active/trial subscriptions
// onto the new active rate card. For each subscription whose plan is present
// in the active card, the new credit allotment is compared with the current
// balance and a top-up is granted equal to:
//
//	delta = max(0, new_plan.credits_included - already_consumed - credits_remaining)
//
// where already_consumed = max(0, credits_total - credits_remaining). This
// guarantees nobody loses balance while bringing everyone to at least the
// new plan's credit pool — the safe default for a rate-card change where
// some channels (SMS, WhatsApp) cost more credits per send.
//
// The grant is recorded via the credit ledger (audit + idempotent re-runs:
// once topped up, the next run computes delta=0).
func (h *OpsHandler) RebalanceCredits(c *fiber.Ctx) error {
	if h.subRepo == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "subscription repository unavailable"})
	}
	if h.creditService == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "credit service unavailable"})
	}
	if h.rateCardMgr == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "rate card manager unavailable"})
	}

	var req struct {
		Apply  bool   `json:"apply"`
		Reason string `json:"reason"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if strings.TrimSpace(req.Reason) == "" {
		req.Reason = "rebalance-2026 credit migration"
	}

	card := h.rateCardMgr.GetActiveRateCard()
	if card == nil {
		if err := h.rateCardMgr.RefreshActiveRateCard(c.Context()); err != nil {
			h.logger.Error("rebalance: refresh rate card failed", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load active rate card"})
		}
		card = h.rateCardMgr.GetActiveRateCard()
	}
	if card == nil || len(card.Plans) == 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "active rate card has no plan bundles"})
	}

	subs, err := h.subRepo.List(c.Context(), license.SubscriptionFilter{
		Statuses: []license.SubscriptionStatus{
			license.SubscriptionStatusActive,
			license.SubscriptionStatusTrial,
		},
		Limit: 1000,
	})
	if err != nil {
		h.logger.Error("rebalance: list subscriptions failed", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list subscriptions"})
	}

	type result struct {
		TenantID         string `json:"tenant_id"`
		SubscriptionID   string `json:"subscription_id"`
		Plan             string `json:"plan"`
		Status           string `json:"status"`
		OldCreditsTotal  int64  `json:"old_credits_total"`
		OldCreditsRemain int64  `json:"old_credits_remaining"`
		NewPlanCredits   int64  `json:"new_plan_credits_included"`
		Delta            int64  `json:"delta"`
		Action           string `json:"action"` // applied | dry_run | skipped:reason
	}
	results := make([]result, 0, len(subs))
	var appliedCount, skippedCount int

	for _, sub := range subs {
		if sub == nil {
			continue
		}
		r := result{
			TenantID:         sub.TenantID,
			SubscriptionID:   sub.ID,
			Plan:             sub.Plan,
			Status:           string(sub.Status),
			OldCreditsTotal:  sub.CreditsTotal,
			OldCreditsRemain: sub.CreditsRemaining,
		}
		newPlan, ok := card.Plans[sub.Plan]
		if !ok {
			r.Action = "skipped:plan_not_in_active_card"
			results = append(results, r)
			skippedCount++
			continue
		}
		if sub.TenantID == "" {
			r.Action = "skipped:no_tenant_id"
			results = append(results, r)
			skippedCount++
			continue
		}
		r.NewPlanCredits = newPlan.CreditsIncluded

		alreadyConsumed := sub.CreditsTotal - sub.CreditsRemaining
		if alreadyConsumed < 0 {
			alreadyConsumed = 0
		}
		targetRemaining := newPlan.CreditsIncluded - alreadyConsumed
		if targetRemaining < sub.CreditsRemaining {
			targetRemaining = sub.CreditsRemaining
		}
		delta := targetRemaining - sub.CreditsRemaining
		r.Delta = delta

		if delta <= 0 {
			r.Action = "skipped:already_at_or_above_target"
			results = append(results, r)
			skippedCount++
			continue
		}

		if !req.Apply {
			r.Action = "dry_run"
			results = append(results, r)
			continue
		}

		_, grantErr := h.creditService.GrantCredits(c.Context(), sub.TenantID, delta, req.Reason, map[string]interface{}{
			"migration":         "rebalance-2026",
			"old_credits_total": sub.CreditsTotal,
			"new_plan":          sub.Plan,
			"new_plan_credits":  newPlan.CreditsIncluded,
			"subscription_id":   sub.ID,
		})
		if grantErr != nil {
			h.logger.Error("rebalance: grant credits failed",
				zap.String("tenant_id", sub.TenantID),
				zap.String("subscription_id", sub.ID),
				zap.Int64("delta", delta),
				zap.Error(grantErr),
			)
			r.Action = "error:" + grantErr.Error()
			results = append(results, r)
			skippedCount++
			continue
		}
		r.Action = "applied"
		results = append(results, r)
		appliedCount++
	}

	h.logger.Info("rebalance credits completed",
		zap.Bool("apply", req.Apply),
		zap.Int("scanned", len(subs)),
		zap.Int("applied", appliedCount),
		zap.Int("skipped", skippedCount),
		zap.String("rate_card_version", card.Version),
	)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"dry_run":           !req.Apply,
			"rate_card_version": card.Version,
			"scanned":           len(subs),
			"applied":           appliedCount,
			"skipped":           skippedCount,
			"results":           results,
		},
	})
}
