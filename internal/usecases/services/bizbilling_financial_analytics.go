package services

import (
	"context"
	"sort"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
)

// Legacy reports retain their response contract. Upgraded apps use typed integer
// sources, currency isolation and a revision check around every complete report.
func (s *BizAnalyticsService) financialReportsEnabled(ctx context.Context, appID string) (bool, error) {
	if s.operations == nil || s.operations.gate == nil {
		return false, nil
	}
	state, err := s.operations.gate.State(ctx, appID)
	if err != nil {
		return false, err
	}
	if state.LifecycleVersion == 1 {
		return false, nil
	}
	if state.LifecycleVersion != 2 || state.Status != "active" {
		return false, bizbilling.ErrReconciliationRequired
	}
	return true, nil
}

// Definitions do not have financial stream checkpoints; they still participate
// in the application revision surrounding the report. Stable IDs avoid truncation.
func walkReportDefinitions[T any](ctx context.Context, store *bizbillingrepo.Store[T], appID string, id func(*T) string, visit func(*T) error) error {
	after := ""
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		q := bizbillingrepo.NewQuery(appID)
		body := q.Body("id", 200, 0)
		if after != "" {
			body["query"] = map[string]interface{}{"bool": map[string]interface{}{"filter": []interface{}{q.Bool(), map[string]interface{}{"range": map[string]interface{}{"id": map[string]interface{}{"gt": after}}}}}}
		}
		body["sort"] = []interface{}{map[string]interface{}{"id": "asc"}}
		rows, _, err := store.Find(ctx, body)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			key := id(row)
			if key == "" || key <= after {
				return bizbilling.ErrReconciliationRequired
			}
			after = key
			if err = visit(row); err != nil {
				return err
			}
		}
	}
}

func (s *BizAnalyticsService) financialAnalytics(ctx context.Context, appID, kind string) (map[string]interface{}, error) {
	revision, err := beginFinancialReport(ctx, s.operations.gate, appID)
	if err != nil {
		return nil, err
	}
	state, err := s.operations.gate.State(ctx, appID)
	if err != nil {
		return nil, err
	}
	currency := state.PrimaryCurrency
	result := map[string]interface{}{"currency": currency}
	if kind == "Aging" {
		delete(result, "currency")
	}
	if kind == "Revenue" || kind == "Subscriptions" {
		if err = s.financialSubscriptionMetrics(ctx, appID, currency, kind, result); err != nil {
			return nil, err
		}
	}
	if kind == "Revenue" {
		var total, refunded int64
		monthly := map[string]int64{}
		err = walkFinancialRows(ctx, s.stores.Payments, s.operations.gate, s.operations, appID, "", func(p *bizbilling.BizPayment) error {
			matches, err := reportCurrencyMatches(p.Currency, currency)
			if err != nil || !matches {
				return err
			}
			if p.Status != bizbilling.PaymentStatusSuccess && p.Status != bizbilling.PaymentStatusRefunded {
				return nil
			}
			if err = addReportAmount(&total, p.AmountPaisa); err != nil {
				return err
			}
			if err = addReportAmount(&refunded, p.RefundedPaisa); err != nil {
				return err
			}
			month := p.CreatedAt.UTC().Format("2006-01")
			amount := monthly[month]
			if err = addReportAmount(&amount, p.AmountPaisa); err != nil {
				return err
			}
			monthly[month] = amount
			return nil
		})
		if err != nil {
			return nil, err
		}
		if refunded > total {
			return nil, bizbilling.ErrReconciliationRequired
		}
		keys := make([]string, 0, len(monthly))
		for key := range monthly {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		months := make([]map[string]interface{}, 0, len(keys))
		for _, key := range keys {
			months = append(months, map[string]interface{}{"month": key, "revenue_paisa": monthly[key]})
		}
		result["total_collected_paisa"], result["refunded_paisa"], result["net_collected_paisa"], result["monthly"] = total, refunded, total-refunded, months
	}
	if kind == "Invoices" || kind == "Aging" || kind == "Dunning" || kind == "TaxSummary" {
		if err = s.financialInvoiceMetrics(ctx, appID, currency, kind, result); err != nil {
			return nil, err
		}
	}
	if kind == "RevenueRecognition" {
		var recognized, deferred, total int64
		err = walkReportDefinitions(ctx, s.stores.RevenueSchedules, appID, func(r *bizbilling.RevenueSchedule) string { return r.ID }, func(r *bizbilling.RevenueSchedule) error {
			invoice, err := s.stores.Invoices.Get(ctx, r.InvoiceID)
			if err != nil {
				return err
			}
			if invoice.AppID != appID {
				return bizbilling.ErrReconciliationRequired
			}
			if err = guardFinancialRead(ctx, s.operations.gate, s.operations, invoice.FinancialScope(), invoice.LastSequence); err != nil {
				return err
			}
			matches, err := reportCurrencyMatches(invoice.Currency, currency)
			if err != nil || !matches {
				return err
			}
			sum, err := bizbilling.CheckedAdd(r.RecognizedPaisa, r.DeferredPaisa)
			if err != nil {
				return err
			}
			if sum != r.TotalPaisa {
				return bizbilling.ErrReconciliationRequired
			}
			if err = addReportAmount(&recognized, r.RecognizedPaisa); err != nil {
				return err
			}
			if err = addReportAmount(&deferred, r.DeferredPaisa); err != nil {
				return err
			}
			return addReportAmount(&total, r.TotalPaisa)
		})
		if err != nil {
			return nil, err
		}
		result["recognized_paisa"], result["deferred_paisa"], result["total_paisa"] = recognized, deferred, total
	}
	if err = revision.verify(ctx, s.operations.gate); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *BizAnalyticsService) financialSubscriptionMetrics(ctx context.Context, appID, currency, kind string, result map[string]interface{}) error {
	plans := map[string]*bizbilling.Plan{}
	if kind == "Revenue" {
		if err := walkReportDefinitions(ctx, s.stores.Plans, appID, func(p *bizbilling.Plan) string { return p.ID }, func(p *bizbilling.Plan) error { plans[p.ID] = p; return nil }); err != nil {
			return err
		}
	}
	counts := map[string]int64{}
	var mrr, canceled int64
	since := time.Now().UTC().AddDate(0, 0, -30)
	err := walkReportDefinitions(ctx, s.stores.Subscriptions, appID, func(p *bizbilling.BizSubscription) string { return p.ID }, func(sub *bizbilling.BizSubscription) error {
		counts[string(sub.Status)]++
		if sub.Status == bizbilling.SubscriptionStatusCanceled && sub.CanceledAt != nil && !sub.CanceledAt.Before(since) {
			canceled++
		}
		if kind != "Revenue" || (sub.Status != bizbilling.SubscriptionStatusActive && sub.Status != bizbilling.SubscriptionStatusPastDue) {
			return nil
		}
		plan := plans[sub.PlanID]
		if plan == nil {
			return bizbilling.ErrLegacyReviewRequired
		}
		matches, err := reportCurrencyMatches(plan.Currency, currency)
		if err != nil || !matches {
			return err
		}
		charge, err := bizbilling.PlanChargeForChecked(plan, sub.Quantity)
		if err != nil {
			return err
		}
		numerator, denominator := int64(1), int64(1)
		switch plan.BillingCycle {
		case bizbilling.BillingCycleWeekly:
			numerator, denominator = 52, 12
		case bizbilling.BillingCycleMonthly:
		case bizbilling.BillingCycleQuarterly:
			denominator = 3
		case bizbilling.BillingCycleHalfYearly:
			denominator = 6
		case bizbilling.BillingCycleYearly:
			denominator = 12
		case bizbilling.BillingCycleCustom:
			numerator, denominator = 30, int64(plan.CycleDays)
		default:
			return bizbilling.ErrInvalidMoney
		}
		amount, err := bizbilling.ProrateMinor(charge, numerator, denominator)
		if err != nil {
			return err
		}
		return addReportAmount(&mrr, amount)
	})
	if err != nil {
		return err
	}
	if kind == "Revenue" {
		arr, err := bizbilling.CheckedMul(mrr, 12)
		if err != nil {
			return err
		}
		result["mrr_paisa"], result["arr_paisa"], result["mrr_basis"] = mrr, arr, "current_plan_estimate"
	} else {
		rate := float64(0)
		if counts["active"]+canceled > 0 {
			rate = float64(canceled) / float64(counts["active"]+canceled) * 100
		}
		result["by_status"], result["canceled_last_30d"], result["churn_rate_percent"] = counts, canceled, rate
	}
	return nil
}

func (s *BizAnalyticsService) financialInvoiceMetrics(ctx context.Context, appID, currency, kind string, result map[string]interface{}) error {
	byStatus := map[string]map[string]int64{}
	var billed, paid, recovered, lost, outstanding, tax, cgst, sgst, igst int64
	if kind == "Aging" {
		for _, key := range []string{"current", "0_30", "31_60", "61_90", "90_plus"} {
			result[key] = map[string]interface{}{"amount_due_paisa": int64(0), "invoice_count": int64(0)}
		}
	}
	now := time.Now().UTC()
	err := walkFinancialRows(ctx, s.stores.Invoices, s.operations.gate, s.operations, appID, "", func(inv *bizbilling.BizInvoice) error {
		matches, err := reportCurrencyMatches(inv.Currency, currency)
		if err != nil || !matches {
			return err
		}
		if kind == "Invoices" {
			key := string(inv.Status)
			b := byStatus[key]
			if b == nil {
				b = map[string]int64{}
				byStatus[key] = b
			}
			b["count"]++
			value := b["total_paisa"]
			if err = addReportAmount(&value, inv.TotalPaisa); err != nil {
				return err
			}
			b["total_paisa"] = value
			value = b["amount_due_paisa"]
			if err = addReportAmount(&value, inv.AmountDuePaisa); err != nil {
				return err
			}
			b["amount_due_paisa"] = value
			if inv.Status == bizbilling.InvoiceStatusDraft || inv.Status == bizbilling.InvoiceStatusVoid {
				return nil
			}
			if err = addReportAmount(&billed, inv.TotalPaisa); err != nil {
				return err
			}
			return addReportAmount(&paid, inv.AmountPaidPaisa)
		}
		if inv.Status == bizbilling.InvoiceStatusDraft || inv.Status == bizbilling.InvoiceStatusVoid {
			return nil
		}
		if kind == "TaxSummary" {
			if err = addReportAmount(&tax, inv.TaxAmountPaisa); err != nil {
				return err
			}
			if err = addReportAmount(&cgst, inv.TaxBreakdown.CGSTPaisa); err != nil {
				return err
			}
			if err = addReportAmount(&sgst, inv.TaxBreakdown.SGSTPaisa); err != nil {
				return err
			}
			return addReportAmount(&igst, inv.TaxBreakdown.IGSTPaisa)
		}
		if kind == "Aging" && inv.AmountDuePaisa > 0 {
			key := "current"
			if inv.DueDate != nil && inv.DueDate.Before(now) {
				days := now.Sub(*inv.DueDate).Hours() / 24
				switch {
				case days < 30:
					key = "0_30"
				case days < 60:
					key = "31_60"
				case days < 90:
					key = "61_90"
				default:
					key = "90_plus"
				}
			}
			b := result[key].(map[string]interface{})
			amount := b["amount_due_paisa"].(int64)
			if err = addReportAmount(&amount, inv.AmountDuePaisa); err != nil {
				return err
			}
			b["amount_due_paisa"] = amount
			b["invoice_count"] = b["invoice_count"].(int64) + 1
		}
		if kind == "Dunning" && inv.DueDate != nil && inv.DueDate.Before(now) {
			if inv.AmountDuePaisa < 0 || inv.ForgivenPaisa < 0 || inv.AmountDuePaisa > inv.TotalPaisa || inv.ForgivenPaisa > inv.TotalPaisa-inv.AmountDuePaisa {
				return bizbilling.ErrReconciliationRequired
			}
			if err = addReportAmount(&recovered, inv.TotalPaisa-inv.AmountDuePaisa-inv.ForgivenPaisa); err != nil {
				return err
			}
			if err = addReportAmount(&lost, inv.ForgivenPaisa); err != nil {
				return err
			}
			return addReportAmount(&outstanding, inv.AmountDuePaisa)
		}
		return nil
	})
	if err != nil {
		return err
	}
	switch kind {
	case "Invoices":
		rate := float64(0)
		if billed > 0 {
			rate = float64(paid) / float64(billed) * 100
		}
		result["by_status"], result["total_billed_paisa"], result["total_paid_paisa"], result["collection_rate_percent"] = byStatus, billed, paid, rate
	case "TaxSummary":
		result["tax_total_paisa"], result["cgst_paisa"], result["sgst_paisa"], result["igst_paisa"] = tax, cgst, sgst, igst
	case "Dunning":
		total, err := bizbilling.CheckedAdd(recovered, lost)
		if err != nil {
			return err
		}
		total, err = bizbilling.CheckedAdd(total, outstanding)
		if err != nil {
			return err
		}
		rate := float64(0)
		if total > 0 {
			rate = float64(recovered) / float64(total) * 100
		}
		result["recovered_paisa"], result["lost_paisa"], result["outstanding_paisa"], result["recovery_rate_percent"] = recovered, lost, outstanding, rate
	}
	return nil
}

func (s *BizAnalyticsService) financialCustomers(ctx context.Context, appID string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	revision, err := beginFinancialReport(ctx, s.operations.gate, appID)
	if err != nil {
		return nil, err
	}
	state, err := s.operations.gate.State(ctx, appID)
	if err != nil {
		return nil, err
	}
	type customer struct {
		id           string
		total, count int64
	}
	byUser := map[string]*customer{}
	err = walkFinancialRows(ctx, s.stores.Payments, s.operations.gate, s.operations, appID, "", func(p *bizbilling.BizPayment) error {
		matches, err := reportCurrencyMatches(p.Currency, state.PrimaryCurrency)
		if err != nil || !matches {
			return err
		}
		if p.Status != bizbilling.PaymentStatusSuccess && p.Status != bizbilling.PaymentStatusRefunded {
			return nil
		}
		c := byUser[p.UserID]
		if c == nil {
			c = &customer{id: p.UserID}
			byUser[p.UserID] = c
		}
		c.count++
		return addReportAmount(&c.total, p.AmountPaisa)
	})
	if err != nil {
		return nil, err
	}
	if err = revision.verify(ctx, s.operations.gate); err != nil {
		return nil, err
	}
	rows := make([]*customer, 0, len(byUser))
	for _, c := range byUser {
		rows = append(rows, c)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].total == rows[j].total {
			return rows[i].id < rows[j].id
		}
		return rows[i].total > rows[j].total
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	result := make([]map[string]interface{}, 0, len(rows))
	for _, c := range rows {
		result = append(result, map[string]interface{}{"user_id": c.id, "ltv_paisa": c.total, "payment_count": c.count, "currency": state.PrimaryCurrency})
	}
	return result, nil
}
