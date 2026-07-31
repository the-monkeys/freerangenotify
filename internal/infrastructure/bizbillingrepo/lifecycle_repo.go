package bizbillingrepo

import (
	"context"
	"encoding/json"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
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
	subBase     *repository.BaseRepository
	invoiceBase *repository.BaseRepository
	paymentBase *repository.BaseRepository
	logger      *zap.Logger
}

// NewLifecycleRepo creates a new LifecycleRepo.
func NewLifecycleRepo(es *elasticsearch.Client, logger *zap.Logger) *LifecycleRepo {
	return &LifecycleRepo{
		subBase:     repository.NewBaseRepository(es, subscriptionIndex, logger, repository.RefreshWaitFor),
		invoiceBase: repository.NewBaseRepository(es, invoiceIndex, logger, repository.RefreshWaitFor),
		paymentBase: repository.NewBaseRepository(es, paymentIndex, logger, repository.RefreshWaitFor),
		logger:      logger,
	}
}

// ─── Subscriptions ───────────────────────────────────────────────────────

func (r *LifecycleRepo) CreateSubscription(ctx context.Context, sub *bizbilling.BizSubscription) error {
	return r.subBase.Create(ctx, sub.ID, sub)
}

func (r *LifecycleRepo) GetSubscription(ctx context.Context, id string) (*bizbilling.BizSubscription, error) {
	raw, err := r.subBase.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var sub bizbilling.BizSubscription
	if err := json.Unmarshal(data, &sub); err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *LifecycleRepo) UpdateSubscription(ctx context.Context, id string, updateFn func(*bizbilling.BizSubscription) error) error {
	sub, err := r.GetSubscription(ctx, id)
	if err != nil {
		return err
	}
	if err := updateFn(sub); err != nil {
		return err
	}
	return r.subBase.Replace(ctx, id, sub)
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

	res, err := r.subBase.Search(ctx, searchReq)
	if err != nil {
		return nil, 0, err
	}

	subs := make([]*bizbilling.BizSubscription, 0, len(res.Hits))
	for _, hit := range res.Hits {
		data, err := json.Marshal(hit)
		if err != nil {
			continue
		}
		var sub bizbilling.BizSubscription
		if err := json.Unmarshal(data, &sub); err != nil {
			r.logger.Error("Failed to unmarshal subscription", zap.Error(err))
			continue
		}
		subs = append(subs, &sub)
	}
	return subs, int(res.Total), nil
}

// ─── Invoices ────────────────────────────────────────────────────────────

func (r *LifecycleRepo) CreateInvoice(ctx context.Context, inv *bizbilling.BizInvoice) error {
	return r.invoiceBase.Create(ctx, inv.ID, inv)
}

func (r *LifecycleRepo) GetInvoice(ctx context.Context, id string) (*bizbilling.BizInvoice, error) {
	raw, err := r.invoiceBase.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var inv bizbilling.BizInvoice
	if err := json.Unmarshal(data, &inv); err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *LifecycleRepo) UpdateInvoice(ctx context.Context, id string, updateFn func(*bizbilling.BizInvoice) error) error {
	inv, err := r.GetInvoice(ctx, id)
	if err != nil {
		return err
	}
	if err := updateFn(inv); err != nil {
		return err
	}
	return r.invoiceBase.Replace(ctx, id, inv)
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

	res, err := r.invoiceBase.Search(ctx, searchReq)
	if err != nil {
		return nil, 0, err
	}

	invoices := make([]*bizbilling.BizInvoice, 0, len(res.Hits))
	for _, hit := range res.Hits {
		data, err := json.Marshal(hit)
		if err != nil {
			continue
		}
		var inv bizbilling.BizInvoice
		if err := json.Unmarshal(data, &inv); err != nil {
			r.logger.Error("Failed to unmarshal invoice", zap.Error(err))
			continue
		}
		invoices = append(invoices, &inv)
	}
	return invoices, int(res.Total), nil
}

// ─── Payments ────────────────────────────────────────────────────────────

func (r *LifecycleRepo) RecordPayment(ctx context.Context, payment *bizbilling.BizPayment) error {
	return r.paymentBase.Create(ctx, payment.ID, payment)
}

func (r *LifecycleRepo) GetPayment(ctx context.Context, id string) (*bizbilling.BizPayment, error) {
	raw, err := r.paymentBase.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var payment bizbilling.BizPayment
	if err := json.Unmarshal(data, &payment); err != nil {
		return nil, err
	}
	return &payment, nil
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

	res, err := r.paymentBase.Search(ctx, searchReq)
	if err != nil {
		return nil, 0, err
	}

	payments := make([]*bizbilling.BizPayment, 0, len(res.Hits))
	for _, hit := range res.Hits {
		data, err := json.Marshal(hit)
		if err != nil {
			continue
		}
		var p bizbilling.BizPayment
		if err := json.Unmarshal(data, &p); err != nil {
			r.logger.Error("Failed to unmarshal payment", zap.Error(err))
			continue
		}
		payments = append(payments, &p)
	}
	return payments, int(res.Total), nil
}
