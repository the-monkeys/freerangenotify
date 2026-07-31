package services

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"go.uber.org/zap"
)

// BizBillingScheduler runs the periodic billing jobs: subscription renewals,
// trial conversion & reminders, overdue marking + dunning, estimate and
// contract expiry, and revenue recognition. One instance runs per server;
// jobs are idempotent so overlapping runs across replicas are safe-ish
// (worst case: a duplicate reminder).
type BizBillingScheduler struct {
	lifecycleSvc *BizLifecycleService
	estimateSvc  *BizEstimateService
	contractSvc  *BizContractService
	configSvc    *BizConfigService
	stores       *bizbillingrepo.Stores
	notifier     *BizNotifier
	interval     time.Duration
	logger       *zap.Logger
}

// NewBizBillingScheduler creates the scheduler. interval <= 0 defaults to 1h.
func NewBizBillingScheduler(
	lifecycleSvc *BizLifecycleService,
	estimateSvc *BizEstimateService,
	contractSvc *BizContractService,
	configSvc *BizConfigService,
	stores *bizbillingrepo.Stores,
	notifier *BizNotifier,
	interval time.Duration,
	logger *zap.Logger,
) *BizBillingScheduler {
	if interval <= 0 {
		interval = time.Hour
	}
	return &BizBillingScheduler{
		lifecycleSvc: lifecycleSvc,
		estimateSvc:  estimateSvc,
		contractSvc:  contractSvc,
		configSvc:    configSvc,
		stores:       stores,
		notifier:     notifier,
		interval:     interval,
		logger:       logger,
	}
}

// Start launches the scheduler loop until ctx is canceled.
func (s *BizBillingScheduler) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.logger.Info("bizbilling: scheduler started", zap.Duration("interval", s.interval))
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("bizbilling: scheduler stopped")
				return
			case <-ticker.C:
				s.RunOnce(ctx)
			}
		}
	}()
}

// RunOnce executes every billing job a single time (also used by tests).
func (s *BizBillingScheduler) RunOnce(ctx context.Context) {
	s.processRenewals(ctx)
	s.processTrials(ctx)
	s.processOverdueInvoices(ctx)
	s.estimateSvc.ExpireStale(ctx)
	s.contractSvc.ProcessExpiries(ctx)
	s.recognizeRevenue(ctx)
}

// ─── Subscription renewals ───────────────────────────────────────────────

// processRenewals advances period-ended active subscriptions: cancel those
// flagged for period-end cancellation, expire non-renewing ones, and generate
// the next invoice for the rest.
func (s *BizBillingScheduler) processRenewals(ctx context.Context) {
	now := time.Now().UTC()
	subs, _, err := s.stores.Subscriptions.Find(ctx, map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"status": string(bizbilling.SubscriptionStatusActive)}},
					{"range": map[string]interface{}{"current_period_end": map[string]interface{}{"lte": now}}},
				},
			},
		},
		"size": 100,
	})
	if err != nil {
		s.logger.Error("bizbilling: renewal scan failed", zap.Error(err))
		return
	}

	for _, sub := range subs {
		if sub.CancelAtPeriodEnd || sub.NonRenewing {
			if _, err := s.lifecycleSvc.CancelSubscription(ctx, sub.AppID, sub.ID, false); err != nil {
				s.logger.Warn("bizbilling: period-end cancellation failed",
					zap.String("subscription_id", sub.ID), zap.Error(err))
			}
			continue
		}
		if err := s.lifecycleSvc.RenewSubscription(ctx, sub); err != nil {
			s.logger.Warn("bizbilling: renewal failed",
				zap.String("subscription_id", sub.ID), zap.Error(err))
		}
	}
}

// ─── Trials ──────────────────────────────────────────────────────────────

// processTrials converts ended trials to active (issuing the first invoice)
// and reminds customers whose trial ends within 3 days.
func (s *BizBillingScheduler) processTrials(ctx context.Context) {
	now := time.Now().UTC()
	subs, _, err := s.stores.Subscriptions.Find(ctx, map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"status": string(bizbilling.SubscriptionStatusTrialing)}},
					{"range": map[string]interface{}{"trial_end": map[string]interface{}{"lte": now.AddDate(0, 0, 3)}}},
				},
			},
		},
		"size": 100,
	})
	if err != nil {
		s.logger.Error("bizbilling: trial scan failed", zap.Error(err))
		return
	}

	for _, sub := range subs {
		if sub.TrialEnd == nil {
			continue
		}
		if !sub.TrialEnd.After(now) {
			if err := s.lifecycleSvc.ConvertTrial(ctx, sub); err != nil {
				s.logger.Warn("bizbilling: trial conversion failed",
					zap.String("subscription_id", sub.ID), zap.Error(err))
			}
			continue
		}
		// Trial ending soon — remind once (tracked in metadata).
		if sub.Metadata != nil && sub.Metadata["trial_reminder_sent"] == true {
			continue
		}
		s.notifier.Send(ctx, sub.AppID, sub.UserID, "biz_trial_ending", "", map[string]interface{}{
			"plan_name": sub.PlanName,
			"trial_end": sub.TrialEnd.Format("02 Jan 2006"),
		})
		_, _ = s.stores.Subscriptions.Update(ctx, sub.ID, func(ss *bizbilling.BizSubscription) error {
			if ss.Metadata == nil {
				ss.Metadata = map[string]interface{}{}
			}
			ss.Metadata["trial_reminder_sent"] = true
			return nil
		})
	}
}

// ─── Overdue invoices & dunning ──────────────────────────────────────────

// processOverdueInvoices marks past-due open invoices as overdue, walks each
// app's dunning sequence, applies late fees, and fires the auto-action
// (pause/cancel) once the configured overdue age is reached.
func (s *BizBillingScheduler) processOverdueInvoices(ctx context.Context) {
	now := time.Now().UTC()
	invoices, _, err := s.stores.Invoices.Find(ctx, map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"terms": map[string]interface{}{"status": []string{
						string(bizbilling.InvoiceStatusOpen),
						string(bizbilling.InvoiceStatusPartiallyPaid),
						string(bizbilling.InvoiceStatusOverdue),
					}}},
					{"range": map[string]interface{}{"due_date": map[string]interface{}{"lt": now}}},
				},
			},
		},
		"size": 200,
	})
	if err != nil {
		s.logger.Error("bizbilling: overdue scan failed", zap.Error(err))
		return
	}

	dunningByApp := map[string]bizbilling.DunningConfig{}
	for _, inv := range invoices {
		// 1. Mark overdue.
		if inv.Status != bizbilling.InvoiceStatusOverdue {
			updated, err := s.lifecycleSvc.MarkOverdue(ctx, inv)
			if err != nil {
				continue
			}
			inv = updated
		}

		// 2. Dunning sequence per app config.
		cfg, ok := dunningByApp[inv.AppID]
		if !ok {
			c, err := s.configSvc.DunningConfig(ctx, inv.AppID)
			if err != nil {
				continue
			}
			cfg = c
			dunningByApp[inv.AppID] = cfg
		}
		if !cfg.Enabled || inv.DueDate == nil {
			continue
		}

		daysOverdue := int(now.Sub(*inv.DueDate).Hours() / 24)
		s.runDunningSteps(ctx, inv, &cfg, daysOverdue)

		// 3. Auto-action once the invoice is old enough.
		autoDay := cfg.AutoActionDay
		if autoDay <= 0 {
			autoDay = 10
		}
		if daysOverdue >= autoDay && cfg.AutoAction != "" && cfg.AutoAction != "none" && inv.SubscriptionID != "" {
			s.applyAutoAction(ctx, inv, cfg.AutoAction)
		}
	}
}

// runDunningSteps sends every due-but-unsent reminder for the invoice.
// Sent steps are tracked in invoice metadata to keep the job idempotent.
func (s *BizBillingScheduler) runDunningSteps(ctx context.Context, inv *bizbilling.BizInvoice, cfg *bizbilling.DunningConfig, daysOverdue int) {
	lastSent := -1
	if inv.Metadata != nil {
		if v, ok := inv.Metadata["dunning_last_step"].(float64); ok {
			lastSent = int(v)
		}
	}

	maxStep := lastSent
	for _, step := range cfg.Steps {
		if daysOverdue < step.DayOffset || step.DayOffset <= lastSent {
			continue
		}
		template := step.Template
		if template == "" {
			template = "biz_reminder_gentle"
		}
		s.notifier.Send(ctx, inv.AppID, inv.UserID, template, step.Channel, map[string]interface{}{
			"invoice_number": inv.InvoiceNumber,
			"amount":         FormatPaisa(inv.AmountDuePaisa, inv.Currency),
			"payment_link":   inv.PaymentLink,
		})
		if step.DayOffset > maxStep {
			maxStep = step.DayOffset
		}
	}

	if maxStep > lastSent {
		_, err := s.lifecycleSvc.updateOwnedInvoice(ctx, inv.AppID, inv.ID, func(i *bizbilling.BizInvoice) error {
			if i.Metadata == nil {
				i.Metadata = map[string]interface{}{}
			}
			i.Metadata["dunning_last_step"] = maxStep
			return nil
		})
		if err != nil {
			s.logger.Warn("bizbilling: could not record dunning progress",
				zap.String("invoice_id", inv.ID), zap.Error(err))
		}
	}
}

// applyAutoAction pauses or cancels the subscription behind a badly overdue
// invoice (once — subsequent runs see the subscription already inactive).
func (s *BizBillingScheduler) applyAutoAction(ctx context.Context, inv *bizbilling.BizInvoice, action string) {
	sub, err := s.lifecycleSvc.GetSubscription(ctx, inv.AppID, inv.SubscriptionID)
	if err != nil || sub.Status != bizbilling.SubscriptionStatusActive && sub.Status != bizbilling.SubscriptionStatusPastDue {
		return
	}

	switch action {
	case "pause":
		_, err = s.lifecycleSvc.PauseSubscription(ctx, inv.AppID, inv.SubscriptionID)
	case "cancel":
		_, err = s.lifecycleSvc.CancelSubscription(ctx, inv.AppID, inv.SubscriptionID, false)
	default:
		return
	}
	if err != nil {
		s.logger.Warn("bizbilling: dunning auto-action failed",
			zap.String("invoice_id", inv.ID), zap.String("action", action), zap.Error(err))
	} else {
		s.logger.Info("bizbilling: dunning auto-action applied",
			zap.String("invoice_id", inv.ID), zap.String("action", action),
			zap.String("subscription_id", inv.SubscriptionID))
	}
}

// ─── Revenue recognition ─────────────────────────────────────────────────

// recognizeRevenue advances every schedule that still has deferred revenue.
func (s *BizBillingScheduler) recognizeRevenue(ctx context.Context) {
	now := time.Now().UTC()
	schedules, _, err := s.stores.RevenueSchedules.Find(ctx, map[string]interface{}{
		"query": map[string]interface{}{
			"range": map[string]interface{}{"deferred_paisa": map[string]interface{}{"gt": 0}},
		},
		"size": 200,
	})
	if err != nil {
		s.logger.Error("bizbilling: revenue recognition scan failed", zap.Error(err))
		return
	}

	for _, schedule := range schedules {
		before := schedule.RecognizedPaisa
		bizbilling.RecognizeThrough(schedule, now)
		if schedule.RecognizedPaisa == before {
			continue
		}
		schedule.UpdatedAt = now
		if err := s.stores.RevenueSchedules.Replace(ctx, schedule.ID, schedule); err != nil {
			s.logger.Warn("bizbilling: revenue schedule update failed",
				zap.String("schedule_id", schedule.ID), zap.Error(err))
		}
	}
}
