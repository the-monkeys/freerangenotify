package bizbillingrepo

import (
	"context"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"go.uber.org/zap"
)

const (
	subscriptionIndex = "frn_biz_subscriptions"
	invoiceIndex      = "frn_biz_invoices"
	paymentIndex      = "frn_biz_payments"
)

// LifecycleRepo implements bizbilling.BizLifecycleRepository using Elasticsearch.
// Consolidating these three closely related models into one repo reduces code bloat.
type LifecycleRepo struct {
	logger        *zap.Logger
	subscriptions *Store[bizbilling.BizSubscription]
	invoices      *Store[bizbilling.BizInvoice]
	payments      *Store[bizbilling.BizPayment]
}

// NewLifecycleRepo creates a new LifecycleRepo.
func NewLifecycleRepo(es *elasticsearch.Client, logger *zap.Logger) *LifecycleRepo {
	return &LifecycleRepo{
		logger:        logger,
		subscriptions: NewStore[bizbilling.BizSubscription](es, subscriptionIndex, logger),
		invoices:      NewStore[bizbilling.BizInvoice](es, invoiceIndex, logger),
		payments:      NewStore[bizbilling.BizPayment](es, paymentIndex, logger),
	}
}

// ─── Subscriptions ───────────────────────────────────────────────────────

func (r *LifecycleRepo) CreateSubscription(ctx context.Context, sub *bizbilling.BizSubscription) error {
	return r.subscriptions.Create(ctx, sub.ID, sub)
}

func (r *LifecycleRepo) GetSubscription(ctx context.Context, id string) (*bizbilling.BizSubscription, error) {
	return r.subscriptions.Get(ctx, id)
}
func (r *LifecycleRepo) UpdateSubscription(ctx context.Context, id string, updateFn func(*bizbilling.BizSubscription) error) error {
	_, err := r.subscriptions.Update(ctx, id, updateFn)
	return err
}

func (r *LifecycleRepo) ListSubscriptions(ctx context.Context, filter bizbilling.SubscriptionFilter) ([]*bizbilling.BizSubscription, int, error) {
	query := map[string]interface{}{"bool": map[string]interface{}{"must": []interface{}{}}}
	must := query["bool"].(map[string]interface{})["must"].([]interface{})

	if filter.AppID != "" {
		must = append(must, map[string]interface{}{"term": map[string]interface{}{"app_id": filter.AppID}})
	}
	if filter.UserID != "" {
		must = append(must, map[string]interface{}{"term": map[string]interface{}{"user_id": filter.UserID}})
	}
	if filter.PlanID != "" {
		must = append(must, map[string]interface{}{"term": map[string]interface{}{"plan_id": filter.PlanID}})
	}
	if filter.Status != "" {
		must = append(must, map[string]interface{}{"term": map[string]interface{}{"status": filter.Status}})
	}

	if len(must) == 0 {
		query["bool"].(map[string]interface{})["must"] = []interface{}{map[string]interface{}{"match_all": map[string]interface{}{}}}
	} else {
		query["bool"].(map[string]interface{})["must"] = must
	}

	searchReq := map[string]interface{}{
		"query": query,
		"sort": []map[string]interface{}{
			{"created_at": map[string]interface{}{"order": "desc"}},
		},
	}

	limit := filter.Limit
	if limit == 0 {
		limit = 10
	}
	searchReq["size"] = limit
	if filter.Offset > 0 {
		searchReq["from"] = filter.Offset
	}

	rows, total, err := r.subscriptions.Find(ctx, searchReq)
	return rows, int(total), err
}

// ─── Invoices ────────────────────────────────────────────────────────────

func (r *LifecycleRepo) CreateInvoice(ctx context.Context, inv *bizbilling.BizInvoice) error {
	return r.invoices.Create(ctx, inv.ID, inv)
}

func (r *LifecycleRepo) GetInvoice(ctx context.Context, id string) (*bizbilling.BizInvoice, error) {
	return r.invoices.Get(ctx, id)
}
func (r *LifecycleRepo) UpdateInvoice(ctx context.Context, id string, updateFn func(*bizbilling.BizInvoice) error) error {
	_, err := r.invoices.Update(ctx, id, updateFn)
	return err
}

func (r *LifecycleRepo) ListInvoices(ctx context.Context, filter bizbilling.InvoiceFilter) ([]*bizbilling.BizInvoice, int, error) {
	query := map[string]interface{}{"bool": map[string]interface{}{"must": []interface{}{}}}
	must := query["bool"].(map[string]interface{})["must"].([]interface{})

	if filter.AppID != "" {
		must = append(must, map[string]interface{}{"term": map[string]interface{}{"app_id": filter.AppID}})
	}
	if filter.UserID != "" {
		must = append(must, map[string]interface{}{"term": map[string]interface{}{"user_id": filter.UserID}})
	}
	if filter.SubscriptionID != "" {
		must = append(must, map[string]interface{}{"term": map[string]interface{}{"subscription_id": filter.SubscriptionID}})
	}
	if filter.Status != "" {
		must = append(must, map[string]interface{}{"term": map[string]interface{}{"status": filter.Status}})
	}

	if len(must) == 0 {
		query["bool"].(map[string]interface{})["must"] = []interface{}{map[string]interface{}{"match_all": map[string]interface{}{}}}
	} else {
		query["bool"].(map[string]interface{})["must"] = must
	}

	searchReq := map[string]interface{}{
		"query": query,
		"sort": []map[string]interface{}{
			{"created_at": map[string]interface{}{"order": "desc"}},
		},
	}

	limit := filter.Limit
	if limit == 0 {
		limit = 10
	}
	searchReq["size"] = limit
	if filter.Offset > 0 {
		searchReq["from"] = filter.Offset
	}

	rows, total, err := r.invoices.Find(ctx, searchReq)
	return rows, int(total), err
}

// ─── Payments ────────────────────────────────────────────────────────────

func (r *LifecycleRepo) RecordPayment(ctx context.Context, payment *bizbilling.BizPayment) error {
	return r.payments.Create(ctx, payment.ID, payment)
}

func (r *LifecycleRepo) GetPayment(ctx context.Context, id string) (*bizbilling.BizPayment, error) {
	return r.payments.Get(ctx, id)
}

func (r *LifecycleRepo) ListPayments(ctx context.Context, filter bizbilling.PaymentFilter) ([]*bizbilling.BizPayment, int, error) {
	query := map[string]interface{}{"bool": map[string]interface{}{"must": []interface{}{}}}
	must := query["bool"].(map[string]interface{})["must"].([]interface{})

	if filter.AppID != "" {
		must = append(must, map[string]interface{}{"term": map[string]interface{}{"app_id": filter.AppID}})
	}
	if filter.UserID != "" {
		must = append(must, map[string]interface{}{"term": map[string]interface{}{"user_id": filter.UserID}})
	}
	if filter.InvoiceID != "" {
		must = append(must, map[string]interface{}{"term": map[string]interface{}{"invoice_ids": filter.InvoiceID}})
	}
	if filter.Status != "" {
		must = append(must, map[string]interface{}{"term": map[string]interface{}{"status": filter.Status}})
	}

	if len(must) == 0 {
		query["bool"].(map[string]interface{})["must"] = []interface{}{map[string]interface{}{"match_all": map[string]interface{}{}}}
	} else {
		query["bool"].(map[string]interface{})["must"] = must
	}

	searchReq := map[string]interface{}{
		"query": query,
		"sort":  []map[string]interface{}{{"created_at": map[string]interface{}{"order": "desc"}}},
	}

	limit := filter.Limit
	if limit == 0 {
		limit = 10
	}
	searchReq["size"] = limit
	if filter.Offset > 0 {
		searchReq["from"] = filter.Offset
	}

	rows, total, err := r.payments.Find(ctx, searchReq)
	return rows, int(total), err
}
