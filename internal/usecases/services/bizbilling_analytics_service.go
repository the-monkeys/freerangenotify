package services

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"go.uber.org/zap"
)

// BizAnalyticsService produces the billing dashboard metrics via
// Elasticsearch aggregations — nothing is precomputed or cached.
type BizAnalyticsService struct {
	stores  *bizbillingrepo.Stores
	planSvc *BizPlanService
	logger  *zap.Logger
}

// NewBizAnalyticsService creates a new BizAnalyticsService.
func NewBizAnalyticsService(stores *bizbillingrepo.Stores, planSvc *BizPlanService, logger *zap.Logger) *BizAnalyticsService {
	return &BizAnalyticsService{stores: stores, planSvc: planSvc, logger: logger}
}

// Revenue returns MRR, ARR, total collected, and monthly revenue over time.
func (s *BizAnalyticsService) Revenue(ctx context.Context, appID string) (map[string]interface{}, error) {
	mrr, err := s.mrr(ctx, appID)
	if err != nil {
		return nil, err
	}

	// Revenue over time: completed payments bucketed by month.
	aggs, err := s.stores.Payments.Aggregate(ctx, map[string]interface{}{
		"query": bizbillingrepo.NewQuery(appID).Term("status", "completed").Bool(),
		"size":  0,
		"aggs": map[string]interface{}{
			"total": map[string]interface{}{"sum": map[string]interface{}{"field": "amount_paisa"}},
			"by_month": map[string]interface{}{
				"date_histogram": map[string]interface{}{
					"field":             "created_at",
					"calendar_interval": "month",
					"format":            "yyyy-MM",
				},
				"aggs": map[string]interface{}{
					"revenue": map[string]interface{}{"sum": map[string]interface{}{"field": "amount_paisa"}},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	monthly := make([]map[string]interface{}, 0)
	for _, bucket := range aggBuckets(aggs, "by_month") {
		monthly = append(monthly, map[string]interface{}{
			"month":         bucket["key_as_string"],
			"revenue_paisa": bucketSum(bucket, "revenue"),
		})
	}

	return map[string]interface{}{
		"mrr_paisa":             mrr,
		"arr_paisa":             mrr * 12,
		"total_collected_paisa": aggValue(aggs, "total"),
		"monthly":               monthly,
	}, nil
}

// mrr sums monthly-normalized plan amounts across active subscriptions.
func (s *BizAnalyticsService) mrr(ctx context.Context, appID string) (int64, error) {
	plans, _, err := s.planSvc.List(ctx, bizbilling.PlanFilter{AppID: appID, Limit: 100})
	if err != nil {
		return 0, err
	}
	planByID := make(map[string]*bizbilling.Plan, len(plans))
	for _, p := range plans {
		planByID[p.ID] = p
	}

	subs, _, err := s.stores.Subscriptions.Find(ctx,
		bizbillingrepo.NewQuery(appID).Terms("status", []string{
			string(bizbilling.SubscriptionStatusActive),
			string(bizbilling.SubscriptionStatusPastDue),
		}).Body("created_at", 100, 0))
	if err != nil {
		return 0, err
	}

	var mrr int64
	for _, sub := range subs {
		plan, ok := planByID[sub.PlanID]
		if !ok {
			continue
		}
		qty := int64(sub.Quantity)
		if qty < 1 {
			qty = 1
		}
		charge := plan.AmountPaisa
		if plan.PricingModel == bizbilling.PricingModelPerUnit {
			charge = plan.AmountPaisa * qty
		}
		mrr += monthlyNormalized(charge, plan.BillingCycle, plan.CycleDays)
	}
	return mrr, nil
}

// monthlyNormalized converts a per-cycle charge to a monthly equivalent.
func monthlyNormalized(charge int64, cycle bizbilling.BillingCycle, cycleDays int) int64 {
	switch cycle {
	case bizbilling.BillingCycleWeekly:
		return charge * 52 / 12
	case bizbilling.BillingCycleMonthly:
		return charge
	case bizbilling.BillingCycleQuarterly:
		return charge / 3
	case bizbilling.BillingCycleHalfYearly:
		return charge / 6
	case bizbilling.BillingCycleYearly:
		return charge / 12
	case bizbilling.BillingCycleCustom:
		if cycleDays > 0 {
			return charge * 30 / int64(cycleDays)
		}
	}
	return charge
}

// Subscriptions returns counts per status and a simple 30-day churn rate.
func (s *BizAnalyticsService) Subscriptions(ctx context.Context, appID string) (map[string]interface{}, error) {
	aggs, err := s.stores.Subscriptions.Aggregate(ctx, map[string]interface{}{
		"query": bizbillingrepo.NewQuery(appID).Bool(),
		"size":  0,
		"aggs": map[string]interface{}{
			"by_status": map[string]interface{}{
				"terms": map[string]interface{}{"field": "status", "size": 10},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	counts := map[string]int64{}
	for _, bucket := range aggBuckets(aggs, "by_status") {
		key, _ := bucket["key"].(string)
		if docs, ok := bucket["doc_count"].(float64); ok {
			counts[key] = int64(docs)
		}
	}

	// Churn: canceled in the last 30 days vs (active now + those canceled).
	since := time.Now().UTC().AddDate(0, 0, -30)
	_, canceledRecently, err := s.stores.Subscriptions.Find(ctx,
		bizbillingrepo.NewQuery(appID).
			Term("status", string(bizbilling.SubscriptionStatusCanceled)).
			Range("canceled_at", since, nil).
			Body("", 1, 0))
	if err != nil {
		canceledRecently = 0
	}

	active := counts[string(bizbilling.SubscriptionStatusActive)]
	churnRate := 0.0
	if active+canceledRecently > 0 {
		churnRate = float64(canceledRecently) / float64(active+canceledRecently) * 100
	}

	return map[string]interface{}{
		"by_status":          counts,
		"canceled_last_30d":  canceledRecently,
		"churn_rate_percent": churnRate,
	}, nil
}

// Invoices returns paid/unpaid/overdue breakdowns and the collection rate.
func (s *BizAnalyticsService) Invoices(ctx context.Context, appID string) (map[string]interface{}, error) {
	aggs, err := s.stores.Invoices.Aggregate(ctx, map[string]interface{}{
		"query": bizbillingrepo.NewQuery(appID).Bool(),
		"size":  0,
		"aggs": map[string]interface{}{
			"by_status": map[string]interface{}{
				"terms": map[string]interface{}{"field": "status", "size": 10},
				"aggs": map[string]interface{}{
					"total": map[string]interface{}{"sum": map[string]interface{}{"field": "total_paisa"}},
					"due":   map[string]interface{}{"sum": map[string]interface{}{"field": "amount_due_paisa"}},
				},
			},
			"billed": map[string]interface{}{"sum": map[string]interface{}{"field": "total_paisa"}},
			"paid":   map[string]interface{}{"sum": map[string]interface{}{"field": "amount_paid_paisa"}},
		},
	})
	if err != nil {
		return nil, err
	}

	byStatus := map[string]map[string]int64{}
	for _, bucket := range aggBuckets(aggs, "by_status") {
		key, _ := bucket["key"].(string)
		count := int64(0)
		if docs, ok := bucket["doc_count"].(float64); ok {
			count = int64(docs)
		}
		byStatus[key] = map[string]int64{
			"count":            count,
			"total_paisa":      bucketSum(bucket, "total"),
			"amount_due_paisa": bucketSum(bucket, "due"),
		}
	}

	billed := aggValue(aggs, "billed")
	paid := aggValue(aggs, "paid")
	collectionRate := 0.0
	if billed > 0 {
		collectionRate = float64(paid) / float64(billed) * 100
	}

	return map[string]interface{}{
		"by_status":               byStatus,
		"total_billed_paisa":      billed,
		"total_paid_paisa":        paid,
		"collection_rate_percent": collectionRate,
	}, nil
}

// Aging buckets outstanding invoice amounts by days overdue.
func (s *BizAnalyticsService) Aging(ctx context.Context, appID string) (map[string]interface{}, error) {
	now := time.Now().UTC()
	buckets := []struct {
		Label string
		From  *time.Time // due_date >= From
		To    *time.Time // due_date < To
	}{
		{"current", ptrTime(now), nil},                                          // Not yet due
		{"0_30", ptrTime(now.AddDate(0, 0, -30)), ptrTime(now)},
		{"31_60", ptrTime(now.AddDate(0, 0, -60)), ptrTime(now.AddDate(0, 0, -30))},
		{"61_90", ptrTime(now.AddDate(0, 0, -90)), ptrTime(now.AddDate(0, 0, -60))},
		{"90_plus", nil, ptrTime(now.AddDate(0, 0, -90))},
	}

	outstandingStatuses := []string{
		string(bizbilling.InvoiceStatusOpen),
		string(bizbilling.InvoiceStatusPartiallyPaid),
		string(bizbilling.InvoiceStatusOverdue),
	}

	result := map[string]interface{}{}
	for _, b := range buckets {
		bounds := map[string]interface{}{}
		if b.From != nil {
			bounds["gte"] = b.From
		}
		if b.To != nil {
			bounds["lt"] = b.To
		}
		query := map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"app_id": appID}},
					{"terms": map[string]interface{}{"status": outstandingStatuses}},
					{"range": map[string]interface{}{"due_date": bounds}},
				},
			},
		}

		aggs, err := s.stores.Invoices.Aggregate(ctx, map[string]interface{}{
			"query": query,
			"size":  0,
			"aggs": map[string]interface{}{
				"due": map[string]interface{}{"sum": map[string]interface{}{"field": "amount_due_paisa"}},
			},
		})
		if err != nil {
			return nil, err
		}
		result[b.Label] = map[string]interface{}{
			"amount_due_paisa": aggValue(aggs, "due"),
			"invoice_count":    aggs["_total_hits"],
		}
	}
	return result, nil
}

// Customers returns the top customers by lifetime collected revenue.
func (s *BizAnalyticsService) Customers(ctx context.Context, appID string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	aggs, err := s.stores.Payments.Aggregate(ctx, map[string]interface{}{
		"query": bizbillingrepo.NewQuery(appID).Term("status", "completed").Bool(),
		"size":  0,
		"aggs": map[string]interface{}{
			"by_user": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "user_id",
					"size":  limit,
					"order": map[string]interface{}{"ltv": "desc"},
				},
				"aggs": map[string]interface{}{
					"ltv": map[string]interface{}{"sum": map[string]interface{}{"field": "amount_paisa"}},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	customers := make([]map[string]interface{}, 0, limit)
	for _, bucket := range aggBuckets(aggs, "by_user") {
		count := int64(0)
		if docs, ok := bucket["doc_count"].(float64); ok {
			count = int64(docs)
		}
		customers = append(customers, map[string]interface{}{
			"user_id":       bucket["key"],
			"ltv_paisa":     bucketSum(bucket, "ltv"),
			"payment_count": count,
		})
	}
	return customers, nil
}

// Dunning reports recovery effectiveness: of invoices that went overdue, how
// much was eventually collected vs written off vs still outstanding.
func (s *BizAnalyticsService) Dunning(ctx context.Context, appID string) (map[string]interface{}, error) {
	now := time.Now().UTC()

	// Invoices whose due date has passed.
	pastDue := map[string]interface{}{
		"bool": map[string]interface{}{
			"must": []map[string]interface{}{
				{"term": map[string]interface{}{"app_id": appID}},
				{"range": map[string]interface{}{"due_date": map[string]interface{}{"lt": now}}},
			},
		},
	}

	aggs, err := s.stores.Invoices.Aggregate(ctx, map[string]interface{}{
		"query": pastDue,
		"size":  0,
		"aggs": map[string]interface{}{
			"by_status": map[string]interface{}{
				"terms": map[string]interface{}{"field": "status", "size": 10},
				"aggs": map[string]interface{}{
					"total": map[string]interface{}{"sum": map[string]interface{}{"field": "total_paisa"}},
					"paid":  map[string]interface{}{"sum": map[string]interface{}{"field": "amount_paid_paisa"}},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	var recovered, lost, outstanding int64
	for _, bucket := range aggBuckets(aggs, "by_status") {
		status, _ := bucket["key"].(string)
		switch bizbilling.InvoiceStatus(status) {
		case bizbilling.InvoiceStatusPaid:
			recovered += bucketSum(bucket, "paid")
		case bizbilling.InvoiceStatusUncollectible:
			lost += bucketSum(bucket, "total")
		case bizbilling.InvoiceStatusOverdue, bizbilling.InvoiceStatusOpen, bizbilling.InvoiceStatusPartiallyPaid:
			recovered += bucketSum(bucket, "paid")
			outstanding += bucketSum(bucket, "total") - bucketSum(bucket, "paid")
		}
	}

	recoveryRate := 0.0
	if recovered+lost+outstanding > 0 {
		recoveryRate = float64(recovered) / float64(recovered+lost+outstanding) * 100
	}

	return map[string]interface{}{
		"recovered_paisa":       recovered,
		"lost_paisa":            lost,
		"outstanding_paisa":     outstanding,
		"recovery_rate_percent": recoveryRate,
	}, nil
}

// RevenueRecognition sums recognized vs deferred revenue across schedules.
func (s *BizAnalyticsService) RevenueRecognition(ctx context.Context, appID string) (map[string]interface{}, error) {
	aggs, err := s.stores.RevenueSchedules.Aggregate(ctx, map[string]interface{}{
		"query": bizbillingrepo.NewQuery(appID).Bool(),
		"size":  0,
		"aggs": map[string]interface{}{
			"recognized": map[string]interface{}{"sum": map[string]interface{}{"field": "recognized_paisa"}},
			"deferred":   map[string]interface{}{"sum": map[string]interface{}{"field": "deferred_paisa"}},
			"total":      map[string]interface{}{"sum": map[string]interface{}{"field": "total_paisa"}},
		},
	})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"recognized_paisa": aggValue(aggs, "recognized"),
		"deferred_paisa":   aggValue(aggs, "deferred"),
		"total_paisa":      aggValue(aggs, "total"),
	}, nil
}

// TaxSummary sums collected GST by component from non-void invoices.
func (s *BizAnalyticsService) TaxSummary(ctx context.Context, appID string) (map[string]interface{}, error) {
	aggs, err := s.stores.Invoices.Aggregate(ctx, map[string]interface{}{
		"query": bizbillingrepo.NewQuery(appID).Terms("status", []string{
			string(bizbilling.InvoiceStatusOpen),
			string(bizbilling.InvoiceStatusPartiallyPaid),
			string(bizbilling.InvoiceStatusPaid),
			string(bizbilling.InvoiceStatusOverdue),
		}).Bool(),
		"size": 0,
		"aggs": map[string]interface{}{
			"tax_total": map[string]interface{}{"sum": map[string]interface{}{"field": "tax_amount_paisa"}},
			"cgst":      map[string]interface{}{"sum": map[string]interface{}{"field": "tax_breakdown.cgst_paisa"}},
			"sgst":      map[string]interface{}{"sum": map[string]interface{}{"field": "tax_breakdown.sgst_paisa"}},
			"igst":      map[string]interface{}{"sum": map[string]interface{}{"field": "tax_breakdown.igst_paisa"}},
		},
	})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"tax_total_paisa": aggValue(aggs, "tax_total"),
		"cgst_paisa":      aggValue(aggs, "cgst"),
		"sgst_paisa":      aggValue(aggs, "sgst"),
		"igst_paisa":      aggValue(aggs, "igst"),
	}, nil
}

// ─── Aggregation response helpers ────────────────────────────────────────

func aggBuckets(aggs map[string]interface{}, name string) []map[string]interface{} {
	node, ok := aggs[name].(map[string]interface{})
	if !ok {
		return nil
	}
	raw, ok := node["buckets"].([]interface{})
	if !ok {
		return nil
	}
	buckets := make([]map[string]interface{}, 0, len(raw))
	for _, b := range raw {
		if bucket, ok := b.(map[string]interface{}); ok {
			buckets = append(buckets, bucket)
		}
	}
	return buckets
}

func bucketSum(bucket map[string]interface{}, name string) int64 {
	node, ok := bucket[name].(map[string]interface{})
	if !ok {
		return 0
	}
	if v, ok := node["value"].(float64); ok {
		return int64(v)
	}
	return 0
}

func ptrTime(t time.Time) *time.Time { return &t }
