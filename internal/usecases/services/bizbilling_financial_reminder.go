package services

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

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
	maxStep := dunningProgress(inv)
	legacyCutoff := maxStep
	oldVersion := inv.Metadata["dunning_steps_version"]
	planned := dunningPlannedSteps(inv)
	seen := map[string]bool{}
	for _, step := range input.Steps {
		if step.DayOffset < 0 {
			return bizbilling.ErrInvalidMoney
		}
		id := dunningStepID(step)
		if step.DayOffset > input.DaysOverdue || dunningStepPlanned(inv, step) || seen[id] {
			continue
		}
		seen[id] = true
		template, channel := normalizedDunningStep(step)
		intent := bizbilling.NotificationIntent{AppID: inv.AppID, UserID: inv.UserID, TemplateID: template, Channel: channel, StepID: id, InvoiceID: inv.ID, EventID: event.ID}
		intent.ID = bizbilling.NotificationIntentID(intent)
		event.Notifications = append(event.Notifications, intent)
		planned[id] = true
		maxStep = max(maxStep, step.DayOffset)
	}
	if inv.Metadata == nil {
		inv.Metadata = map[string]interface{}{}
	}
	if oldVersion != 2 && oldVersion != float64(2) && legacyCutoff >= 0 {
		inv.Metadata["dunning_legacy_through_day"] = legacyCutoff
	}
	inv.Metadata["dunning_last_step"] = maxStep
	inv.Metadata["dunning_steps_version"] = 2
	inv.Metadata["dunning_planned_steps"] = planned
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
	steps := make([]bizbilling.DunningStep, 0, len(cfg.Steps))
	seen := map[string]bool{}
	for _, step := range cfg.Steps {
		id := dunningStepID(step)
		if step.DayOffset <= days && !dunningStepPlanned(fresh, step) && !seen[id] {
			steps = append(steps, step)
			seen[id] = true
		}
	}
	if len(steps) == 0 {
		return true
	}
	sort.Slice(steps, func(i, j int) bool { return dunningStepID(steps[i]) < dunningStepID(steps[j]) })
	ids := make([]string, 0, len(steps))
	maxDay := 0
	for _, step := range steps {
		ids = append(ids, dunningStepID(step))
		maxDay = max(maxDay, step.DayOffset)
	}
	key := gatewayEffectID("dunning", inv.ID, strings.Join(ids, ","))
	_, err = s.lifecycleSvc.applyFinancialCommand(WithBizOperationKey(ctx, key), bizbilling.Command{Kind: bizbilling.CommandInvoiceReminder, Scope: scope, InvoiceReminder: &bizbilling.InvoiceReminder{InvoiceID: inv.ID, DaysOverdue: maxDay, Steps: steps}})
	if err != nil {
		s.logger.Warn("billing reminder pending", zap.String("invoice_id", inv.ID), zap.Error(err))
	}
	return true
}

func normalizedDunningStep(step bizbilling.DunningStep) (string, string) {
	template, channel := step.Template, step.Channel
	if template == "" {
		template = "biz_reminder_gentle"
	}
	if channel == "" {
		channel = "email"
	}
	return template, channel
}
func dunningStepID(step bizbilling.DunningStep) string {
	template, channel := normalizedDunningStep(step)
	return gatewayEffectID("reminder_step", strconv.Itoa(step.DayOffset), template, channel)
}
func dunningPlannedSteps(invoice *bizbilling.BizInvoice) map[string]bool {
	result := map[string]bool{}
	switch values := invoice.Metadata["dunning_planned_steps"].(type) {
	case map[string]bool:
		for k, v := range values {
			result[k] = v
		}
	case map[string]interface{}:
		for k, v := range values {
			if yes, ok := v.(bool); ok && yes {
				result[k] = true
			}
		}
	}
	return result
}
func dunningStepPlanned(invoice *bizbilling.BizInvoice, step bizbilling.DunningStep) bool {
	version := invoice.Metadata["dunning_steps_version"]
	if version != 2 && version != float64(2) {
		return step.DayOffset <= dunningProgress(invoice)
	}
	legacy := &bizbilling.BizInvoice{Metadata: map[string]interface{}{"dunning_last_step": invoice.Metadata["dunning_legacy_through_day"]}}
	if step.DayOffset <= dunningProgress(legacy) {
		return true
	}
	return dunningPlannedSteps(invoice)[dunningStepID(step)]
}
