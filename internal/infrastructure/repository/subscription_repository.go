package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

// SubscriptionRepository implements license.Repository with Elasticsearch.
type SubscriptionRepository struct {
	base *BaseRepository
}

// NewSubscriptionRepository creates a new subscription repository.
func NewSubscriptionRepository(client *elasticsearch.Client, logger *zap.Logger) license.Repository {
	return &SubscriptionRepository{
		base: NewBaseRepository(client, "subscriptions", logger, RefreshWaitFor),
	}
}

func (r *SubscriptionRepository) Create(ctx context.Context, sub *license.Subscription) error {
	now := time.Now().UTC()
	sub.CreatedAt = now
	sub.UpdatedAt = now
	return r.base.Create(ctx, sub.ID, sub)
}

func (r *SubscriptionRepository) GetByID(ctx context.Context, id string) (*license.Subscription, error) {
	doc, err := r.base.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var sub license.Subscription
	if err := mapToStruct(doc, &sub); err != nil {
		return nil, fmt.Errorf("failed to map document to subscription: %w", err)
	}

	return &sub, nil
}

func (r *SubscriptionRepository) Update(ctx context.Context, sub *license.Subscription) error {
	sub.UpdatedAt = time.Now().UTC()
	return r.base.Update(ctx, sub.ID, sub)
}

func (r *SubscriptionRepository) Delete(ctx context.Context, id string) error {
	return r.base.Delete(ctx, id)
}

func (r *SubscriptionRepository) List(ctx context.Context, filter license.SubscriptionFilter) ([]*license.Subscription, error) {
	query := map[string]interface{}{}
	must := make([]map[string]interface{}, 0)

	if filter.TenantID != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{"tenant_id": filter.TenantID},
		})
	}
	if filter.AppID != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{"app_id": filter.AppID},
		})
	}
	if len(filter.Statuses) > 0 {
		statuses := make([]string, 0, len(filter.Statuses))
		for _, status := range filter.Statuses {
			statuses = append(statuses, string(status))
		}
		must = append(must, map[string]interface{}{
			"terms": map[string]interface{}{"status": statuses},
		})
	}

	if len(must) > 0 {
		query["query"] = map[string]interface{}{
			"bool": map[string]interface{}{
				"must": must,
			},
		}
	} else {
		query["query"] = map[string]interface{}{
			"match_all": map[string]interface{}{},
		}
	}

	query["sort"] = []map[string]interface{}{
		{"updated_at": map[string]interface{}{"order": "desc"}},
	}

	if filter.Offset > 0 {
		query["from"] = filter.Offset
	}
	if filter.Limit > 0 {
		query["size"] = filter.Limit
	}

	result, err := r.base.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	subs := make([]*license.Subscription, 0, len(result.Hits))
	for _, hit := range result.Hits {
		var sub license.Subscription
		if err := mapToStruct(hit, &sub); err != nil {
			r.base.logger.Warn("Failed to map document to subscription", zap.Error(err))
			continue
		}
		subs = append(subs, &sub)
	}

	return subs, nil
}

func (r *SubscriptionRepository) GetActiveSubscription(ctx context.Context, tenantID, appID string, now time.Time) (*license.Subscription, error) {
	if appID != "" {
		sub, err := r.getActiveByField(ctx, "app_id", appID, now)
		if err == nil && sub != nil {
			return sub, nil
		}
	}

	if tenantID != "" {
		sub, err := r.getActiveByField(ctx, "tenant_id", tenantID, now)
		if err == nil && sub != nil {
			return sub, nil
		}
	}

	return nil, nil
}

func (r *SubscriptionRepository) getActiveByField(ctx context.Context, field, value string, now time.Time) (*license.Subscription, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{field: value}},
					{"terms": map[string]interface{}{"status": []string{string(license.SubscriptionStatusActive), string(license.SubscriptionStatusTrial)}}},
					{"range": map[string]interface{}{"current_period_start": map[string]interface{}{"lte": now.Format(time.RFC3339)}}},
					{"range": map[string]interface{}{"current_period_end": map[string]interface{}{"gte": now.Format(time.RFC3339)}}},
				},
			},
		},
		"sort": []map[string]interface{}{
			{"current_period_end": map[string]interface{}{"order": "desc"}},
			{"updated_at": map[string]interface{}{"order": "desc"}},
		},
		"size": 1,
	}

	result, err := r.base.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	if result.Total == 0 {
		return nil, nil
	}

	var sub license.Subscription
	if err := mapToStruct(result.Hits[0], &sub); err != nil {
		return nil, fmt.Errorf("failed to map document to subscription: %w", err)
	}

	return &sub, nil
}
