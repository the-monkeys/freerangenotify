package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// BizLifecycleService handles the business logic for Subscriptions, Invoices,
// and Payments — the heart of the billing module. It integrates the tax
// engine, sequential numbering, coupons, credits, the payment gateway, and
// billing notifications.
type BizLifecycleService struct {
	repo            bizbilling.BizLifecycleRepository
	stores          *bizbillingrepo.Stores
	planSvc         *BizPlanService
	configSvc       *BizConfigService
	creditSvc       *BizCreditService
	portalSvc       *BizPortalService
	notifier        *BizNotifier
	gateway         billing.Provider
	users           user.Repository
	defaultCurrency string
	logger          *zap.Logger
}

// NewBizLifecycleService creates a new BizLifecycleService.
func NewBizLifecycleService(
	repo bizbilling.BizLifecycleRepository,
	stores *bizbillingrepo.Stores,
	planSvc *BizPlanService,
	configSvc *BizConfigService,
	creditSvc *BizCreditService,
	portalSvc *BizPortalService,
	notifier *BizNotifier,
	gateway billing.Provider,
	users user.Repository,
	defaultCurrency string,
	logger *zap.Logger,
) *BizLifecycleService {
	if defaultCurrency == "" {
		defaultCurrency = "INR"
	}
	return &BizLifecycleService{
		repo:            repo,
		stores:          stores,
		planSvc:         planSvc,
		configSvc:       configSvc,
		creditSvc:       creditSvc,
		portalSvc:       portalSvc,
		notifier:        notifier,
		gateway:         gateway,
		users:           users,
		defaultCurrency: defaultCurrency,
		logger:          logger,
	}
}

// ─── Ownership helpers ───────────────────────────────────────────────────

func (s *BizLifecycleService) getOwnedSubscription(ctx context.Context, appID, id string) (*bizbilling.BizSubscription, error) {
	sub, err := s.repo.GetSubscription(ctx, id)
	if err != nil || sub.AppID != appID {
		return nil, errors.NotFound("subscription", id)
	}
	return sub, nil
}

func (s *BizLifecycleService) getOwnedInvoice(ctx context.Context, appID, id string) (*bizbilling.BizInvoice, error) {
	inv, err := s.repo.GetInvoice(ctx, id)
	if err != nil || inv.AppID != appID {
		return nil, errors.NotFound("invoice", id)
	}
	return inv, nil
}

// updateOwnedInvoice runs updateFn only when the invoice belongs to the app.
func (s *BizLifecycleService) updateOwnedInvoice(ctx context.Context, appID, id string, updateFn func(*bizbilling.BizInvoice) error) (*bizbilling.BizInvoice, error) {
	var updated *bizbilling.BizInvoice
	err := s.repo.UpdateInvoice(ctx, id, func(inv *bizbilling.BizInvoice) error {
		if inv.AppID != appID {
			return errors.NotFound("invoice", id)
		}
		if err := updateFn(inv); err != nil {
			return err
		}
		updated = inv
		return nil
	})
	return updated, err
}

// updateOwnedSubscription runs updateFn only when the subscription belongs to the app.
func (s *BizLifecycleService) updateOwnedSubscription(ctx context.Context, appID, id string, updateFn func(*bizbilling.BizSubscription) error) (*bizbilling.BizSubscription, error) {
	var updated *bizbilling.BizSubscription
	err := s.repo.UpdateSubscription(ctx, id, func(sub *bizbilling.BizSubscription) error {
		if sub.AppID != appID {
			return errors.NotFound("subscription", id)
		}
		if err := updateFn(sub); err != nil {
			return err
		}
		updated = sub
		return nil
	})
	return updated, err
}

// customerState reads the user's billing state for CGST/SGST vs IGST split.
func (s *BizLifecycleService) customerState(ctx context.Context, userID string) string {
	if s.users == nil || userID == "" {
		return ""
	}
	u, err := s.users.GetByID(ctx, userID)
	if err != nil || u.BillingAddress == nil {
		return ""
	}
	return u.BillingAddress.State
}

// ─── Subscriptions ───────────────────────────────────────────────────────

// CreateSubscription creates a subscription and, unless trialing, generates
// and issues its first invoice (plan charge + setup fee + addons).
func (s *BizLifecycleService) CreateSubscription(ctx context.Context, appID string, req *bizbilling.CreateSubscriptionRequest) (*bizbilling.BizSubscription, error) {
	if req.UserID == "" || req.PlanID == "" {
		return nil, errors.BadRequest("user_id and plan_id are required")
	}
	if req.Quantity < 1 {
		req.Quantity = 1
	}

	if s.users != nil {
		u, err := s.users.GetByID(ctx, req.UserID)
		if err != nil || u == nil || u.AppID != appID {
			return nil, errors.NotFound("user", req.UserID)
		}
	}

	plan, err := s.planSvc.GetByID(ctx, req.PlanID)
	if err != nil || plan.AppID != appID {
		return nil, errors.NotFound("plan", req.PlanID)
	}

	now := time.Now().UTC()
	sub := &bizbilling.BizSubscription{
		ID:                 uuid.New().String(),
		AppID:              appID,
		UserID:             req.UserID,
		PlanID:             req.PlanID,
		AddonIDs:           req.AddonIDs,
		Status:             bizbilling.SubscriptionStatusActive,
		Quantity:           req.Quantity,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   bizbilling.NextPeriodEnd(now, plan.BillingCycle, plan.CycleDays),
		Metadata:           req.Metadata,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	trialDays := req.TrialDays
	if trialDays == 0 {
		trialDays = plan.TrialDays
	}
	if trialDays > 0 {
		trialEnd := now.AddDate(0, 0, trialDays)
		sub.Status = bizbilling.SubscriptionStatusTrialing
		sub.TrialStart = &now
		sub.TrialEnd = &trialEnd
		sub.CurrentPeriodStart = trialEnd
		sub.CurrentPeriodEnd = bizbilling.NextPeriodEnd(trialEnd, plan.BillingCycle, plan.CycleDays)
	}

	if err := s.repo.CreateSubscription(ctx, sub); err != nil {
		return nil, err
	}

	if sub.Status == bizbilling.SubscriptionStatusActive {
		if _, err := s.generateSubscriptionInvoice(ctx, sub, plan, true); err != nil {
			// Roll back the orphan subscription so callers never see a
			// "created" subscription without its first invoice.
			if s.stores != nil && s.stores.Subscriptions != nil {
				_ = s.stores.Subscriptions.Delete(ctx, sub.ID)
			}
			return nil, fmt.Errorf("generate first invoice: %w", err)
		}
	}

	s.notifier.Send(ctx, appID, sub.UserID, "biz_subscription_activated", "", map[string]interface{}{
		"plan_name":         plan.Name,
		"next_billing_date": sub.CurrentPeriodEnd.Format("02 Jan 2006"),
		"company_name":      s.companyName(ctx, appID),
	})
	return sub, nil
}

// GetSubscription returns a subscription scoped to the app.
func (s *BizLifecycleService) GetSubscription(ctx context.Context, appID, id string) (*bizbilling.BizSubscription, error) {
	return s.getOwnedSubscription(ctx, appID, id)
}

// RenewSubscription advances the billing period and issues the renewal
// invoice. Called by the scheduler when current_period_end has passed.
func (s *BizLifecycleService) RenewSubscription(ctx context.Context, sub *bizbilling.BizSubscription) error {
	plan, err := s.planSvc.GetByID(ctx, sub.PlanID)
	if err != nil || plan.AppID != sub.AppID {
		return errors.NotFound("plan", sub.PlanID)
	}

	renewed, err := s.updateOwnedSubscription(ctx, sub.AppID, sub.ID, func(ss *bizbilling.BizSubscription) error {
		if ss.Status != bizbilling.SubscriptionStatusActive {
			return errors.BadRequest("only active subscriptions renew")
		}
		ss.CurrentPeriodStart = ss.CurrentPeriodEnd
		ss.CurrentPeriodEnd = bizbilling.NextPeriodEnd(ss.CurrentPeriodEnd, plan.BillingCycle, plan.CycleDays)
		ss.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return err
	}

	_, err = s.generateSubscriptionInvoice(ctx, renewed, plan, false)
	return err
}

// ConvertTrial activates a subscription whose trial has ended and issues the
// first invoice. Called by the scheduler.
func (s *BizLifecycleService) ConvertTrial(ctx context.Context, sub *bizbilling.BizSubscription) error {
	plan, err := s.planSvc.GetByID(ctx, sub.PlanID)
	if err != nil || plan.AppID != sub.AppID {
		return errors.NotFound("plan", sub.PlanID)
	}

	now := time.Now().UTC()
	activated, err := s.updateOwnedSubscription(ctx, sub.AppID, sub.ID, func(ss *bizbilling.BizSubscription) error {
		if ss.Status != bizbilling.SubscriptionStatusTrialing {
			return errors.BadRequest("subscription is not trialing")
		}
		ss.Status = bizbilling.SubscriptionStatusActive
		ss.CurrentPeriodStart = now
		ss.CurrentPeriodEnd = bizbilling.NextPeriodEnd(now, plan.BillingCycle, plan.CycleDays)
		ss.UpdatedAt = now
		return nil
	})
	if err != nil {
		return err
	}

	if _, err := s.generateSubscriptionInvoice(ctx, activated, plan, true); err != nil {
		return err
	}
	s.notifier.Send(ctx, sub.AppID, sub.UserID, "biz_subscription_activated", "", map[string]interface{}{
		"plan_name":         plan.Name,
		"next_billing_date": activated.CurrentPeriodEnd.Format("02 Jan 2006"),
		"company_name":      s.companyName(ctx, sub.AppID),
	})
	return nil
}

// MarkOverdue flips a past-due invoice to overdue and moves its subscription
// to past_due. Called by the scheduler.
func (s *BizLifecycleService) MarkOverdue(ctx context.Context, inv *bizbilling.BizInvoice) (*bizbilling.BizInvoice, error) {
	updated, err := s.updateOwnedInvoice(ctx, inv.AppID, inv.ID, func(i *bizbilling.BizInvoice) error {
		if !bizbilling.PayableStatuses[i.Status] {
			return errors.BadRequest("invoice is not outstanding")
		}
		i.Status = bizbilling.InvoiceStatusOverdue
		i.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return nil, err
	}

	if inv.SubscriptionID != "" {
		_, _ = s.updateOwnedSubscription(ctx, inv.AppID, inv.SubscriptionID, func(ss *bizbilling.BizSubscription) error {
			if ss.Status == bizbilling.SubscriptionStatusActive {
				ss.Status = bizbilling.SubscriptionStatusPastDue
				ss.UpdatedAt = time.Now().UTC()
			}
			return nil
		})
	}
	return updated, nil
}

// UpdateSubscription applies partial updates (addons, quantity, flags).
// Plan changes go through ChangePlan for proper proration.
func (s *BizLifecycleService) UpdateSubscription(ctx context.Context, appID, id string, req *bizbilling.UpdateSubscriptionRequest) (*bizbilling.BizSubscription, error) {
	if req.PlanID != nil {
		return s.ChangePlan(ctx, appID, id, *req.PlanID, req.Quantity)
	}
	return s.updateOwnedSubscription(ctx, appID, id, func(sub *bizbilling.BizSubscription) error {
		if req.AddonIDs != nil {
			sub.AddonIDs = req.AddonIDs
		}
		if req.Quantity != nil {
			sub.Quantity = *req.Quantity
		}
		if req.CancelAtPeriodEnd != nil {
			sub.CancelAtPeriodEnd = *req.CancelAtPeriodEnd
		}
		if req.NonRenewing != nil {
			sub.NonRenewing = *req.NonRenewing
		}
		if req.Metadata != nil {
			sub.Metadata = req.Metadata
		}
		sub.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// ChangePlan upgrades/downgrades a subscription with Salesforce-style
// proration: the unused portion of the old plan becomes a credit note and the
// prorated new-plan charge is invoiced immediately.
func (s *BizLifecycleService) ChangePlan(ctx context.Context, appID, id, newPlanID string, quantity *int) (*bizbilling.BizSubscription, error) {
	newPlan, err := s.planSvc.GetByID(ctx, newPlanID)
	if err != nil || newPlan.AppID != appID {
		return nil, errors.NotFound("plan", newPlanID)
	}

	now := time.Now().UTC()
	var oldPlanID string
	var proration bizbilling.ProrationResult

	sub, err := s.updateOwnedSubscription(ctx, appID, id, func(sub *bizbilling.BizSubscription) error {
		if sub.Status != bizbilling.SubscriptionStatusActive && sub.Status != bizbilling.SubscriptionStatusTrialing {
			return errors.BadRequest("only active or trialing subscriptions can change plans")
		}
		if sub.PlanID == newPlanID {
			return errors.BadRequest("subscription is already on this plan")
		}

		oldPlanID = sub.PlanID
		qty := sub.Quantity
		if quantity != nil {
			qty = *quantity
		}

		if oldPlan, err := s.planSvc.GetByID(ctx, oldPlanID); err == nil && sub.Status == bizbilling.SubscriptionStatusActive {
			proration = bizbilling.ProratePlanChange(oldPlan, newPlan, qty,
				sub.CurrentPeriodStart, sub.CurrentPeriodEnd, now)
		}

		sub.PlanID = newPlanID
		if quantity != nil {
			sub.Quantity = *quantity
		}
		sub.UpdatedAt = now
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Credit the unused old-plan portion.
	if proration.CreditPaisa > 0 {
		if _, err := s.creditSvc.CreateCreditNote(ctx, appID, &bizbilling.CreateCreditNoteRequest{
			UserID:      sub.UserID,
			AmountPaisa: proration.CreditPaisa,
			Reason:      fmt.Sprintf("Proration credit for plan change (subscription %s)", sub.ID),
		}); err != nil {
			s.logger.Error("bizbilling: proration credit note failed",
				zap.String("subscription_id", sub.ID), zap.Error(err))
		}
	}

	// Invoice the prorated new-plan charge for the remaining period.
	if proration.ChargePaisa > 0 {
		items := []bizbilling.InvoiceLineItem{{
			Description: fmt.Sprintf("%s (prorated until %s)", newPlan.Name, sub.CurrentPeriodEnd.Format("02 Jan 2006")),
			Quantity:    1,
			UnitPrice:   proration.ChargePaisa,
			TaxRate:     0, // filled from tax config in buildInvoice
		}}
		inv, err := s.buildInvoice(ctx, appID, sub.UserID, sub.ID, bizbilling.InvoiceTypeRecurring, items, newPlan.Currency, "")
		if err != nil {
			s.logger.Error("bizbilling: proration invoice failed",
				zap.String("subscription_id", sub.ID), zap.Error(err))
		} else if _, err := s.IssueInvoice(ctx, appID, inv.ID); err != nil {
			s.logger.Error("bizbilling: issuing proration invoice failed",
				zap.String("invoice_id", inv.ID), zap.Error(err))
		}
	}

	return sub, nil
}

// CancelSubscription cancels immediately or at period end.
func (s *BizLifecycleService) CancelSubscription(ctx context.Context, appID, id string, atPeriodEnd bool) (*bizbilling.BizSubscription, error) {
	sub, err := s.updateOwnedSubscription(ctx, appID, id, func(sub *bizbilling.BizSubscription) error {
		now := time.Now().UTC()
		sub.UpdatedAt = now
		if atPeriodEnd {
			sub.CancelAtPeriodEnd = true
			return nil
		}
		sub.Status = bizbilling.SubscriptionStatusCanceled
		sub.CanceledAt = &now
		return nil
	})
	if err != nil {
		return nil, err
	}

	if !atPeriodEnd {
		planName := ""
		if plan, perr := s.planSvc.GetByID(ctx, sub.PlanID); perr == nil {
			planName = plan.Name
		}
		s.notifier.Send(ctx, appID, sub.UserID, "biz_subscription_canceled", "", map[string]interface{}{
			"plan_name":    planName,
			"company_name": s.companyName(ctx, appID),
		})
	}
	return sub, nil
}

// PauseSubscription pauses an active subscription.
func (s *BizLifecycleService) PauseSubscription(ctx context.Context, appID, id string) (*bizbilling.BizSubscription, error) {
	sub, err := s.updateOwnedSubscription(ctx, appID, id, func(sub *bizbilling.BizSubscription) error {
		if sub.Status != bizbilling.SubscriptionStatusActive && sub.Status != bizbilling.SubscriptionStatusPastDue {
			return errors.BadRequest("only active or past-due subscriptions can be paused")
		}
		now := time.Now().UTC()
		sub.Status = bizbilling.SubscriptionStatusPaused
		sub.PausedAt = &now
		sub.UpdatedAt = now
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.notifier.Send(ctx, appID, sub.UserID, "biz_subscription_paused", "", map[string]interface{}{
		"company_name": s.companyName(ctx, appID),
	})
	return sub, nil
}

// ResumeSubscription resumes a paused subscription.
func (s *BizLifecycleService) ResumeSubscription(ctx context.Context, appID, id string) (*bizbilling.BizSubscription, error) {
	return s.updateOwnedSubscription(ctx, appID, id, func(sub *bizbilling.BizSubscription) error {
		if sub.Status != bizbilling.SubscriptionStatusPaused {
			return errors.BadRequest("only paused subscriptions can be resumed")
		}
		now := time.Now().UTC()
		sub.Status = bizbilling.SubscriptionStatusActive
		sub.PausedAt = nil
		sub.UpdatedAt = now
		return nil
	})
}

// ReactivateSubscription reactivates a canceled subscription.
func (s *BizLifecycleService) ReactivateSubscription(ctx context.Context, appID, id string) (*bizbilling.BizSubscription, error) {
	return s.updateOwnedSubscription(ctx, appID, id, func(sub *bizbilling.BizSubscription) error {
		if sub.Status != bizbilling.SubscriptionStatusCanceled {
			return errors.BadRequest("only canceled subscriptions can be reactivated")
		}
		now := time.Now().UTC()
		sub.Status = bizbilling.SubscriptionStatusActive
		sub.CanceledAt = nil
		sub.CancelAtPeriodEnd = false
		sub.UpdatedAt = now
		return nil
	})
}

// SetNonRenewing flags a subscription to stop renewing at period end.
func (s *BizLifecycleService) SetNonRenewing(ctx context.Context, appID, id string, nonRenewing bool) (*bizbilling.BizSubscription, error) {
	return s.updateOwnedSubscription(ctx, appID, id, func(sub *bizbilling.BizSubscription) error {
		sub.NonRenewing = nonRenewing
		sub.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// ListSubscriptions lists subscriptions, enriched with plan info.
func (s *BizLifecycleService) ListSubscriptions(ctx context.Context, filter bizbilling.SubscriptionFilter) ([]*bizbilling.BizSubscription, int, error) {
	subs, total, err := s.repo.ListSubscriptions(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	for _, sub := range subs {
		if plan, err := s.planSvc.GetByID(ctx, sub.PlanID); err == nil && plan != nil {
			sub.PlanName = plan.Name
			sub.PlanAmountPaisa = plan.AmountPaisa
			sub.BillingCycle = string(plan.BillingCycle)
		}
	}
	return subs, total, nil
}

// ─── Invoice creation ────────────────────────────────────────────────────

// buildInvoice creates a draft invoice from line items using the app's tax
// configuration, the customer's billing state, sequential numbering, and
// default payment terms.
func (s *BizLifecycleService) buildInvoice(
	ctx context.Context,
	appID, userID, subscriptionID string,
	invType bizbilling.InvoiceType,
	items []bizbilling.InvoiceLineItem,
	currency, paymentTerms string,
) (*bizbilling.BizInvoice, error) {
	if len(items) == 0 {
		return nil, errors.BadRequest("at least one line item is required")
	}

	taxCfg, err := s.configSvc.TaxConfig(ctx, appID)
	if err != nil {
		return nil, err
	}
	totals := bizbilling.ComputeInvoiceTotals(items, taxCfg, s.customerState(ctx, userID))

	number, err := s.configSvc.NextNumber(ctx, appID, NumberKindInvoice)
	if err != nil {
		return nil, err
	}

	if currency == "" {
		currency = s.defaultCurrency
	}
	if paymentTerms == "" {
		if pt, err := s.configSvc.PaymentTermsConfig(ctx, appID); err == nil {
			paymentTerms = pt.DefaultTerms
		}
	}

	now := time.Now().UTC()
	dueDate := dueDateFor(paymentTerms, now)

	invoice := &bizbilling.BizInvoice{
		ID:             uuid.New().String(),
		AppID:          appID,
		UserID:         userID,
		SubscriptionID: subscriptionID,
		InvoiceNumber:  number,
		InvoiceType:    invType,
		Status:         bizbilling.InvoiceStatusDraft,
		LineItems:      items,
		SubtotalPaisa:  totals.SubtotalPaisa,
		DiscountPaisa:  totals.DiscountPaisa,
		TaxAmountPaisa: totals.TaxPaisa,
		TaxBreakdown:   totals.Breakdown,
		TotalPaisa:     totals.TotalPaisa,
		AmountDuePaisa: totals.TotalPaisa,
		Currency:       currency,
		PaymentTerms:   paymentTerms,
		DueDate:        &dueDate,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.repo.CreateInvoice(ctx, invoice); err != nil {
		return nil, err
	}
	return invoice, nil
}

// generateSubscriptionInvoice builds and (optionally) issues the invoice for
// a subscription period: plan charge, setup fee on first invoice, and addons.
func (s *BizLifecycleService) generateSubscriptionInvoice(ctx context.Context, sub *bizbilling.BizSubscription, plan *bizbilling.Plan, first bool) (*bizbilling.BizInvoice, error) {
	qty := sub.Quantity
	if qty <= 0 {
		qty = 1
	}

	charge := bizbilling.PlanChargeFor(plan, qty)
	items := []bizbilling.InvoiceLineItem{{
		Description: fmt.Sprintf("%s (%s – %s)", plan.Name,
			sub.CurrentPeriodStart.Format("02 Jan 2006"), sub.CurrentPeriodEnd.Format("02 Jan 2006")),
		Quantity:  qty,
		UnitPrice: charge / int64(qty),
	}}

	if first && plan.SetupFeePaisa > 0 {
		items = append(items, bizbilling.InvoiceLineItem{
			Description: "One-time setup fee",
			Quantity:    1,
			UnitPrice:   plan.SetupFeePaisa,
		})
	}

	for _, addonID := range sub.AddonIDs {
		addon, err := s.planSvc.GetAddon(ctx, sub.AppID, addonID)
		if err != nil {
			s.logger.Warn("bizbilling: skipping unknown addon on invoice",
				zap.String("addon_id", addonID), zap.String("subscription_id", sub.ID))
			continue
		}
		items = append(items, bizbilling.InvoiceLineItem{
			Description: addon.Name + " (addon)",
			Quantity:    1,
			UnitPrice:   addon.AmountPaisa,
		})
	}

	inv, err := s.buildInvoice(ctx, sub.AppID, sub.UserID, sub.ID, bizbilling.InvoiceTypeRecurring, items, plan.Currency, "")
	if err != nil {
		return nil, err
	}
	return s.IssueInvoice(ctx, sub.AppID, inv.ID)
}

// CreateOneTimeInvoice creates an ad-hoc draft invoice.
func (s *BizLifecycleService) CreateOneTimeInvoice(ctx context.Context, appID string, req *bizbilling.CreateInvoiceRequest) (*bizbilling.BizInvoice, error) {
	if req.UserID == "" {
		return nil, errors.BadRequest("user_id is required")
	}
	inv, err := s.buildInvoice(ctx, appID, req.UserID, "", bizbilling.InvoiceTypeOneTime, req.LineItems, req.Currency, req.PaymentTerms)
	if err != nil {
		return nil, err
	}
	if req.Notes != "" || req.Metadata != nil {
		inv, err = s.updateOwnedInvoice(ctx, appID, inv.ID, func(i *bizbilling.BizInvoice) error {
			i.Notes = req.Notes
			i.Metadata = req.Metadata
			return nil
		})
	}
	return inv, err
}

// UpdateDraftInvoice updates a draft invoice and recomputes totals.
func (s *BizLifecycleService) UpdateDraftInvoice(ctx context.Context, appID, id string, req *bizbilling.UpdateInvoiceRequest) (*bizbilling.BizInvoice, error) {
	taxCfg, err := s.configSvc.TaxConfig(ctx, appID)
	if err != nil {
		return nil, err
	}

	return s.updateOwnedInvoice(ctx, appID, id, func(inv *bizbilling.BizInvoice) error {
		if inv.Status != bizbilling.InvoiceStatusDraft {
			return errors.BadRequest("only draft invoices can be updated")
		}
		now := time.Now().UTC()
		inv.UpdatedAt = now

		if req.LineItems != nil {
			inv.LineItems = req.LineItems
			totals := bizbilling.ComputeInvoiceTotals(inv.LineItems, taxCfg, s.customerState(ctx, inv.UserID))
			inv.SubtotalPaisa = totals.SubtotalPaisa
			inv.DiscountPaisa = totals.DiscountPaisa
			inv.TaxAmountPaisa = totals.TaxPaisa
			inv.TaxBreakdown = totals.Breakdown
			inv.TotalPaisa = totals.TotalPaisa
			inv.AmountDuePaisa = totals.TotalPaisa - inv.AmountPaidPaisa - inv.CreditsApplied
		}
		if req.PaymentTerms != nil {
			inv.PaymentTerms = *req.PaymentTerms
			d := dueDateFor(*req.PaymentTerms, now)
			inv.DueDate = &d
		}
		if req.Notes != nil {
			inv.Notes = *req.Notes
		}
		if req.Metadata != nil {
			inv.Metadata = req.Metadata
		}
		return nil
	})
}

// IssueInvoice transitions draft → open: creates a payment gateway order,
// attaches a payment link, sends the invoice notification, and creates the
// revenue recognition schedule.
func (s *BizLifecycleService) IssueInvoice(ctx context.Context, appID, id string) (*bizbilling.BizInvoice, error) {
	inv, err := s.updateOwnedInvoice(ctx, appID, id, func(inv *bizbilling.BizInvoice) error {
		if inv.Status != bizbilling.InvoiceStatusDraft {
			return errors.BadRequest("only draft invoices can be issued")
		}
		now := time.Now().UTC()
		inv.Status = bizbilling.InvoiceStatusOpen
		inv.IssuedAt = &now
		inv.UpdatedAt = now

		if s.gateway != nil && inv.AmountDuePaisa > 0 {
			order, err := s.gateway.CreateOrder(ctx, inv.AppID, "bizinv", inv.AmountDuePaisa)
			if err != nil {
				s.logger.Warn("bizbilling: gateway order creation failed — invoice issued without order",
					zap.String("invoice_id", inv.ID), zap.Error(err))
			} else {
				inv.GatewayOrderID = order.OrderID
			}
		}
		if link := s.portalSvc.PaymentLinkFor(ctx, inv.AppID, inv.UserID, inv.ID); link != "" {
			inv.PaymentLink = link
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.createRevenueSchedule(ctx, inv)
	s.sendInvoiceNotification(ctx, inv, "biz_invoice_issued")
	return inv, nil
}

// SendInvoice re-sends the invoice notification.
func (s *BizLifecycleService) SendInvoice(ctx context.Context, appID, id string) (*bizbilling.BizInvoice, error) {
	inv, err := s.getOwnedInvoice(ctx, appID, id)
	if err != nil {
		return nil, err
	}
	if inv.Status == bizbilling.InvoiceStatusDraft {
		return nil, errors.BadRequest("issue the invoice before sending it")
	}
	s.sendInvoiceNotification(ctx, inv, "biz_invoice_issued")
	return inv, nil
}

// VoidInvoice voids an unpaid invoice.
func (s *BizLifecycleService) VoidInvoice(ctx context.Context, appID, id string) (*bizbilling.BizInvoice, error) {
	return s.updateOwnedInvoice(ctx, appID, id, func(inv *bizbilling.BizInvoice) error {
		if inv.Status == bizbilling.InvoiceStatusPaid {
			return errors.BadRequest("paid invoices cannot be voided")
		}
		if inv.Status == bizbilling.InvoiceStatusVoid {
			return errors.BadRequest("invoice is already voided")
		}
		now := time.Now().UTC()
		inv.Status = bizbilling.InvoiceStatusVoid
		inv.VoidedAt = &now
		inv.UpdatedAt = now
		return nil
	})
}

// WriteOffInvoice marks an uncollectible invoice as written off.
func (s *BizLifecycleService) WriteOffInvoice(ctx context.Context, appID, id string) (*bizbilling.BizInvoice, error) {
	return s.updateOwnedInvoice(ctx, appID, id, func(inv *bizbilling.BizInvoice) error {
		if inv.Status == bizbilling.InvoiceStatusPaid {
			return errors.BadRequest("paid invoices cannot be written off")
		}
		if inv.Status == bizbilling.InvoiceStatusUncollectible {
			return errors.BadRequest("invoice is already written off")
		}
		now := time.Now().UTC()
		inv.Status = bizbilling.InvoiceStatusUncollectible
		inv.WrittenOffAt = &now
		inv.UpdatedAt = now
		return nil
	})
}

// ApplyCoupon applies a coupon code to a draft invoice.
func (s *BizLifecycleService) ApplyCoupon(ctx context.Context, appID, id, code string) (*bizbilling.BizInvoice, error) {
	coupons, _, err := s.stores.Coupons.Find(ctx,
		bizbillingrepo.NewQuery(appID).Term("code", code).Body("created_at", 1, 0))
	if err != nil || len(coupons) == 0 {
		return nil, errors.NotFound("coupon", code)
	}
	coupon := coupons[0]

	inv, err := s.updateOwnedInvoice(ctx, appID, id, func(inv *bizbilling.BizInvoice) error {
		if inv.Status != bizbilling.InvoiceStatusDraft {
			return errors.BadRequest("coupons can only be applied to draft invoices")
		}
		if inv.CouponID != "" && !coupon.Stackable {
			return errors.BadRequest("a coupon is already applied to this invoice")
		}
		if err := coupon.Validate(time.Now().UTC(), s.subscriptionPlanID(ctx, inv.SubscriptionID)); err != nil {
			return errors.BadRequest(err.Error())
		}

		discount := coupon.DiscountFor(inv.SubtotalPaisa)
		inv.DiscountPaisa += discount
		inv.CouponID = coupon.ID
		inv.TotalPaisa -= discount
		if inv.TotalPaisa < 0 {
			inv.TotalPaisa = 0
		}
		inv.AmountDuePaisa = inv.TotalPaisa - inv.AmountPaidPaisa - inv.CreditsApplied
		inv.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Consume one use — best effort after the invoice update.
	if _, err := s.stores.Coupons.Update(ctx, coupon.ID, func(cp *bizbilling.Coupon) error {
		cp.CurrentUses++
		cp.UpdatedAt = time.Now().UTC()
		return nil
	}); err != nil {
		s.logger.Warn("bizbilling: failed to increment coupon usage",
			zap.String("coupon_id", coupon.ID), zap.Error(err))
	}
	return inv, nil
}

// ApplyCredit applies the user's stored credit balance to an invoice.
func (s *BizLifecycleService) ApplyCredit(ctx context.Context, appID, id string) (*bizbilling.BizInvoice, error) {
	inv, err := s.getOwnedInvoice(ctx, appID, id)
	if err != nil {
		return nil, err
	}
	if !bizbilling.PayableStatuses[inv.Status] && inv.Status != bizbilling.InvoiceStatusDraft {
		return nil, errors.BadRequest("credit cannot be applied to this invoice")
	}
	if inv.AmountDuePaisa <= 0 {
		return nil, errors.BadRequest("invoice has no amount due")
	}

	consumed, err := s.creditSvc.ConsumeBalance(ctx, appID, inv.UserID, inv.AmountDuePaisa)
	if err != nil {
		return nil, err
	}
	if consumed == 0 {
		return nil, errors.BadRequest("user has no credit balance")
	}
	return s.applyCreditToInvoice(ctx, appID, id, consumed)
}

// applyCreditToInvoice records external credit (balance / credit note /
// retainer) against an invoice, marking it paid when fully covered.
func (s *BizLifecycleService) applyCreditToInvoice(ctx context.Context, appID, id string, amount int64) (*bizbilling.BizInvoice, error) {
	return s.updateOwnedInvoice(ctx, appID, id, func(inv *bizbilling.BizInvoice) error {
		now := time.Now().UTC()
		inv.CreditsApplied += amount
		inv.AmountDuePaisa -= amount
		if inv.AmountDuePaisa <= 0 {
			inv.AmountDuePaisa = 0
			if inv.Status != bizbilling.InvoiceStatusDraft {
				inv.Status = bizbilling.InvoiceStatusPaid
				inv.PaidAt = &now
			}
		}
		inv.UpdatedAt = now
		return nil
	})
}

// ApplyCreditNoteToInvoice consumes a credit note against an invoice.
func (s *BizLifecycleService) ApplyCreditNoteToInvoice(ctx context.Context, appID, creditNoteID, invoiceID string) (*bizbilling.BizInvoice, error) {
	inv, err := s.getOwnedInvoice(ctx, appID, invoiceID)
	if err != nil {
		return nil, err
	}
	if inv.AmountDuePaisa <= 0 {
		return nil, errors.BadRequest("invoice has no amount due")
	}
	consumed, err := s.creditSvc.ConsumeCreditNote(ctx, appID, creditNoteID, invoiceID, inv.AmountDuePaisa)
	if err != nil {
		return nil, err
	}
	return s.applyCreditToInvoice(ctx, appID, invoiceID, consumed)
}

// ApplyRetainerToInvoice consumes a retainer balance against an invoice.
func (s *BizLifecycleService) ApplyRetainerToInvoice(ctx context.Context, appID, retainerID, invoiceID string) (*bizbilling.BizInvoice, error) {
	inv, err := s.getOwnedInvoice(ctx, appID, invoiceID)
	if err != nil {
		return nil, err
	}
	if inv.AmountDuePaisa <= 0 {
		return nil, errors.BadRequest("invoice has no amount due")
	}
	consumed, err := s.creditSvc.ConsumeRetainer(ctx, appID, retainerID, invoiceID, inv.AmountDuePaisa)
	if err != nil {
		return nil, err
	}
	return s.applyCreditToInvoice(ctx, appID, invoiceID, consumed)
}

// AddLateFee manually applies the configured late fee to an overdue invoice.
func (s *BizLifecycleService) AddLateFee(ctx context.Context, appID, id string) (*bizbilling.BizInvoice, error) {
	lateCfg, err := s.configSvc.LateFeeConfig(ctx, appID)
	if err != nil {
		return nil, err
	}
	if !lateCfg.Enabled {
		return nil, errors.BadRequest("late fees are not enabled — configure them under /v1/biz/config/late-fees")
	}

	return s.updateOwnedInvoice(ctx, appID, id, func(inv *bizbilling.BizInvoice) error {
		if !bizbilling.PayableStatuses[inv.Status] {
			return errors.BadRequest("late fees only apply to unpaid invoices")
		}
		if inv.DueDate == nil {
			return errors.BadRequest("invoice has no due date")
		}

		daysOverdue := int(time.Since(*inv.DueDate).Hours() / 24)
		fee := bizbilling.LateFeeFor(lateCfg, inv.AmountDuePaisa, daysOverdue)
		if fee <= 0 {
			return errors.BadRequest("no late fee applicable (within grace period or zero balance)")
		}

		inv.LateFeePaisa += fee
		inv.TotalPaisa += fee
		inv.AmountDuePaisa += fee
		inv.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// ListInvoices lists invoices for an app.
func (s *BizLifecycleService) ListInvoices(ctx context.Context, filter bizbilling.InvoiceFilter) ([]*bizbilling.BizInvoice, int, error) {
	return s.repo.ListInvoices(ctx, filter)
}

// GetInvoice returns an invoice scoped to the app.
func (s *BizLifecycleService) GetInvoice(ctx context.Context, appID, id string) (*bizbilling.BizInvoice, error) {
	return s.getOwnedInvoice(ctx, appID, id)
}

// ─── Payments ────────────────────────────────────────────────────────────

// RecordPayment records a manual or gateway payment against an invoice.
// Supports partial payments; overpayments are credited to the user balance.
func (s *BizLifecycleService) RecordPayment(ctx context.Context, appID, invoiceID string, req *bizbilling.RecordPaymentRequest) (*bizbilling.BizPayment, error) {
	inv, err := s.getOwnedInvoice(ctx, appID, invoiceID)
	if err != nil {
		return nil, err
	}
	if !bizbilling.PayableStatuses[inv.Status] {
		return nil, errors.BadRequest("invoice is not payable in its current status")
	}

	amount := req.AmountPaisa
	if amount == 0 {
		amount = inv.AmountDuePaisa
	}
	if amount <= 0 {
		return nil, errors.BadRequest("payment amount must be greater than 0")
	}

	now := time.Now().UTC()
	payment := &bizbilling.BizPayment{
		ID:               uuid.New().String(),
		AppID:            inv.AppID,
		UserID:           inv.UserID,
		InvoiceIDs:       []string{invoiceID},
		AmountPaisa:      amount,
		Currency:         inv.Currency,
		Method:           req.Method,
		GatewayPaymentID: req.GatewayTxnID,
		Status:           bizbilling.PaymentStatusSuccess,
		TDSPaisa:         req.TDSPaisa,
		Notes:            req.Notes,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if req.Method == "gateway" || req.GatewayTxnID != "" {
		payment.Gateway = "razorpay"
	}
	if err := s.repo.RecordPayment(ctx, payment); err != nil {
		return nil, err
	}

	// TDS deducted by the customer still settles the invoice in full.
	settled := amount + req.TDSPaisa
	overpayment := int64(0)

	inv, err = s.updateOwnedInvoice(ctx, appID, invoiceID, func(i *bizbilling.BizInvoice) error {
		i.AmountPaidPaisa += amount
		i.AmountDuePaisa -= settled
		if i.AmountDuePaisa <= 0 {
			overpayment = -i.AmountDuePaisa
			i.AmountDuePaisa = 0
			i.Status = bizbilling.InvoiceStatusPaid
			i.PaidAt = &now
		} else {
			i.Status = bizbilling.InvoiceStatusPartiallyPaid
		}
		i.UpdatedAt = now
		return nil
	})
	if err != nil {
		return payment, err
	}

	if overpayment > 0 {
		if _, err := s.creditSvc.AdjustBalance(ctx, appID, inv.UserID, overpayment); err != nil {
			s.logger.Warn("bizbilling: failed to credit overpayment",
				zap.String("invoice_id", invoiceID), zap.Int64("overpayment", overpayment), zap.Error(err))
		}
	}

	// A paid invoice recovers a past-due subscription.
	if inv.Status == bizbilling.InvoiceStatusPaid && inv.SubscriptionID != "" {
		_, _ = s.updateOwnedSubscription(ctx, appID, inv.SubscriptionID, func(sub *bizbilling.BizSubscription) error {
			if sub.Status == bizbilling.SubscriptionStatusPastDue {
				sub.Status = bizbilling.SubscriptionStatusActive
				sub.UpdatedAt = now
			}
			return nil
		})
	}

	s.notifier.Send(ctx, appID, inv.UserID, "biz_payment_received", "", map[string]interface{}{
		"invoice_number": inv.InvoiceNumber,
		"amount":         FormatPaisa(amount, inv.Currency),
		"company_name":   s.companyName(ctx, appID),
	})
	return payment, nil
}

// RefundPayment records a full or partial refund.
func (s *BizLifecycleService) RefundPayment(ctx context.Context, appID, paymentID string, req *bizbilling.RefundPaymentRequest) (*bizbilling.BizPayment, error) {
	payment, err := s.repo.GetPayment(ctx, paymentID)
	if err != nil || payment.AppID != appID {
		return nil, errors.NotFound("payment", paymentID)
	}
	if payment.Status != bizbilling.PaymentStatusSuccess {
		return nil, errors.BadRequest("only successful payments can be refunded")
	}

	refundable := payment.AmountPaisa - payment.RefundedPaisa
	amount := req.AmountPaisa
	if amount == 0 {
		amount = refundable
	}
	if amount <= 0 || amount > refundable {
		return nil, errors.BadRequest("invalid refund amount")
	}

	now := time.Now().UTC()
	payment.RefundedPaisa += amount
	if payment.RefundedPaisa >= payment.AmountPaisa {
		payment.Status = bizbilling.PaymentStatusRefunded
	}
	payment.UpdatedAt = now
	if _, err := s.stores.Payments.Update(ctx, paymentID, func(p *bizbilling.BizPayment) error {
		*p = *payment
		return nil
	}); err != nil {
		return nil, err
	}

	s.notifier.Send(ctx, appID, payment.UserID, "biz_refund_processed", "", map[string]interface{}{
		"amount":       FormatPaisa(amount, payment.Currency),
		"company_name": s.companyName(ctx, appID),
	})
	return payment, nil
}

// RecordGatewayPayment marks the invoice matching a gateway order as paid.
// Used by the payment webhook path.
func (s *BizLifecycleService) RecordGatewayPayment(ctx context.Context, gatewayOrderID, gatewayPaymentID string) error {
	invoices, _, err := s.stores.Invoices.Find(ctx, map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{"gateway_order_id": gatewayOrderID},
		},
		"size": 1,
	})
	if err != nil || len(invoices) == 0 {
		return errors.NotFound("invoice for gateway order", gatewayOrderID)
	}
	inv := invoices[0]
	if inv.Status == bizbilling.InvoiceStatusPaid {
		return nil // idempotent
	}

	_, err = s.RecordPayment(ctx, inv.AppID, inv.ID, &bizbilling.RecordPaymentRequest{
		Method:       "gateway",
		GatewayTxnID: gatewayPaymentID,
	})
	return err
}

// ListPayments lists payments for an app.
func (s *BizLifecycleService) ListPayments(ctx context.Context, filter bizbilling.PaymentFilter) ([]*bizbilling.BizPayment, int, error) {
	return s.repo.ListPayments(ctx, filter)
}

// GetPayment returns a payment scoped to the app.
func (s *BizLifecycleService) GetPayment(ctx context.Context, appID, id string) (*bizbilling.BizPayment, error) {
	p, err := s.repo.GetPayment(ctx, id)
	if err != nil || p.AppID != appID {
		return nil, errors.NotFound("payment", id)
	}
	return p, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────

func (s *BizLifecycleService) subscriptionPlanID(ctx context.Context, subscriptionID string) string {
	if subscriptionID == "" {
		return ""
	}
	sub, err := s.repo.GetSubscription(ctx, subscriptionID)
	if err != nil {
		return ""
	}
	return sub.PlanID
}

func (s *BizLifecycleService) companyName(ctx context.Context, appID string) string {
	branding, err := s.configSvc.BrandingConfig(ctx, appID)
	if err != nil || branding.CompanyName == "" {
		return "your service provider"
	}
	return branding.CompanyName
}

func (s *BizLifecycleService) sendInvoiceNotification(ctx context.Context, inv *bizbilling.BizInvoice, templateName string) {
	dueDate := ""
	if inv.DueDate != nil {
		dueDate = inv.DueDate.Format("02 Jan 2006")
	}
	notifID := s.notifier.Send(ctx, inv.AppID, inv.UserID, templateName, "", map[string]interface{}{
		"invoice_number": inv.InvoiceNumber,
		"amount":         FormatPaisa(inv.AmountDuePaisa, inv.Currency),
		"due_date":       dueDate,
		"payment_link":   inv.PaymentLink,
		"company_name":   s.companyName(ctx, inv.AppID),
	})
	if notifID != "" && inv.NotificationID == "" {
		_, _ = s.updateOwnedInvoice(ctx, inv.AppID, inv.ID, func(i *bizbilling.BizInvoice) error {
			i.NotificationID = notifID
			return nil
		})
	}
}

// createRevenueSchedule spreads a subscription invoice's revenue across the
// service period for deferred/recognized reporting. Best-effort.
func (s *BizLifecycleService) createRevenueSchedule(ctx context.Context, inv *bizbilling.BizInvoice) {
	if inv.TotalPaisa <= 0 {
		return
	}

	periodStart, periodEnd := inv.CreatedAt, inv.CreatedAt
	if inv.SubscriptionID != "" {
		if sub, err := s.repo.GetSubscription(ctx, inv.SubscriptionID); err == nil {
			periodStart, periodEnd = sub.CurrentPeriodStart, sub.CurrentPeriodEnd
		}
	}

	entries := bizbilling.BuildRevenueSchedule(inv.TotalPaisa, periodStart, periodEnd)
	now := time.Now().UTC()
	schedule := &bizbilling.RevenueSchedule{
		ID:             uuid.New().String(),
		AppID:          inv.AppID,
		InvoiceID:      inv.ID,
		SubscriptionID: inv.SubscriptionID,
		TotalPaisa:     inv.TotalPaisa,
		Entries:        entries,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	bizbilling.RecognizeThrough(schedule, now)

	if err := s.stores.RevenueSchedules.Create(ctx, schedule.ID, schedule); err != nil {
		s.logger.Warn("bizbilling: revenue schedule creation failed",
			zap.String("invoice_id", inv.ID), zap.Error(err))
	}
}

// dueDateFor resolves a payment-terms string to a due date.
func dueDateFor(paymentTerms string, from time.Time) time.Time {
	switch paymentTerms {
	case "net_15", "net15":
		return from.AddDate(0, 0, 15)
	case "net_30", "net30":
		return from.AddDate(0, 0, 30)
	case "net_60", "net60":
		return from.AddDate(0, 0, 60)
	case "due_on_receipt", "":
		return from
	default:
		return from.AddDate(0, 0, 30)
	}
}
