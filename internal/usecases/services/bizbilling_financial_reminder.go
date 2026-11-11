package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"go.uber.org/zap"
)

func dunningProgress(inv *bizbilling.BizInvoice) int {
	value, ok := inv.Metadata["dunning_last_step"]
	if !ok {
		return -1
	}
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		n, err := v.Int64()
		if err == nil {
			return int(n)
		}
	}
	return -1
}

func (s *BizOperationService) buildInvoiceReminder(ctx context.Context, command bizbilling.Command, event *bizbilling.JournalEvent) error {
	input := command.InvoiceReminder
	inv, err := s.financialInvoice(ctx, command.Scope, input.InvoiceID)
	if err != nil {
		return err
	}
	if inv.Status == bizbilling.InvoiceStatusDraft || inv.Status == bizbilling.InvoiceStatusVoid || inv.AmountDuePaisa <= 0 {
		return bizbilling.ErrInvalidMoney
	}
	last, maxStep := dunningProgress(inv), dunningProgress(inv)
	seen := map[int]bool{}
	for _, step := range input.Steps {
		if step.DayOffset < 0 {
			return bizbilling.ErrInvalidMoney
		}
		if step.DayOffset <= last || step.DayOffset > input.DaysOverdue || seen[step.DayOffset] {
			continue
		}
		seen[step.DayOffset] = true
		template := step.Template
		if template == "" {
			template = "biz_reminder_gentle"
		}
		channel := step.Channel
		if channel == "" {
			channel = "email"
		}
		event.Notifications = append(event.Notifications, bizbilling.NotificationIntent{ID: fmt.Sprintf("%s-reminder-%d", event.OperationID, step.DayOffset), AppID: inv.AppID, UserID: inv.UserID, TemplateID: template, Channel: channel, InvoiceID: inv.ID, EventID: event.ID})
		maxStep = max(maxStep, step.DayOffset)
	}
	if inv.Metadata == nil {
		inv.Metadata = map[string]interface{}{}
	}
	inv.Metadata["dunning_last_step"] = maxStep
	inv.LastSequence = event.Sequence
	inv.UpdatedAt = event.CreatedAt
	event.InvoiceAfter = inv
	event.Resources = []bizbilling.ResourceRef{{Kind: "invoice", ID: inv.ID}}
	return nil
}

// Fresh source metadata and a stable action key make repeated scheduler scans
// safe. Delivery belongs to the durable notification worker, outside the lock.
func (s *BizBillingScheduler) tryFinancialDunning(ctx context.Context, inv *bizbilling.BizInvoice, cfg *bizbilling.DunningConfig, days int) bool {
	if s.lifecycleSvc == nil {
		return false
	}
	scope, active, err := s.lifecycleSvc.financialInvoiceScope(ctx, inv.AppID, inv.ID)
	if err != nil {
		s.logger.Warn("billing reminder scope unavailable", zap.String("invoice_id", inv.ID), zap.Error(err))
		return true
	}
	if !active {
		return false
	}
	fresh, err := s.lifecycleSvc.GetInvoice(ctx, inv.AppID, inv.ID)
	if err != nil {
		s.logger.Warn("billing reminder source unavailable", zap.String("invoice_id", inv.ID), zap.Error(err))
		return true
	}
	if fresh.AmountDuePaisa <= 0 {
		return true
	}
	last := dunningProgress(fresh)
	maxStep := last
	for _, step := range cfg.Steps {
		if step.DayOffset <= days && step.DayOffset > maxStep {
			maxStep = step.DayOffset
		}
	}
	if maxStep <= last {
		return true
	}
	_, err = s.lifecycleSvc.applyFinancialCommand(WithBizOperationKey(ctx, fmt.Sprintf("dunning:%s:%d", inv.ID, maxStep)), bizbilling.Command{Kind: bizbilling.CommandInvoiceReminder, Scope: scope, InvoiceReminder: &bizbilling.InvoiceReminder{InvoiceID: inv.ID, DaysOverdue: days, Steps: cfg.Steps}})
	if err != nil {
		s.logger.Warn("billing reminder pending", zap.String("invoice_id", inv.ID), zap.Error(err))
	}
	return true
}
