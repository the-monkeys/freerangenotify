package billingrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"go.uber.org/zap"
)

const usageIndex = "frn_usage_events"

// ESUsageRepo implements billing.UsageRepository and billing.UsageEmitter
// backed by Elasticsearch.
type ESUsageRepo struct {
	es     *elasticsearch.Client
	logger *zap.Logger
}

// NewESUsageRepo creates a new ES-backed usage repo.
func NewESUsageRepo(es *elasticsearch.Client, logger *zap.Logger) *ESUsageRepo {
	return &ESUsageRepo{es: es, logger: logger}
}

// EnsureIndex creates the frn_usage_events index with correct mappings if it
// does not already exist. Call this during application startup / migration.
func (r *ESUsageRepo) EnsureIndex(ctx context.Context) error {
	mapping := `{
		"mappings": {
			"properties": {
				"id":                { "type": "keyword" },
				"tenant_id":         { "type": "keyword" },
				"app_id":            { "type": "keyword" },
				"notification_id":   { "type": "keyword" },
				"channel":           { "type": "keyword" },
				"provider":          { "type": "keyword" },
				"credential_source": { "type": "keyword" },
				"message_type":      { "type": "keyword" },
				"cost_unit_paisa":   { "type": "long" },
				"billed_paisa":      { "type": "long" },
				"currency":          { "type": "keyword" },
				"status":            { "type": "keyword" },
				"timestamp":         { "type": "date" }
			}
		},
		"settings": {
			"number_of_shards": 2,
			"number_of_replicas": 1,
			"routing": { "allocation": { "total_shards_per_node": 2 } }
		}
	}`

	res, err := r.es.Indices.Exists([]string{usageIndex}, r.es.Indices.Exists.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("billingrepo: check index exists: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 200 {
		return nil // already exists
	}

	createRes, err := r.es.Indices.Create(
		usageIndex,
		r.es.Indices.Create.WithBody(strings.NewReader(mapping)),
		r.es.Indices.Create.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("billingrepo: create index: %w", err)
	}
	defer createRes.Body.Close()

	if createRes.IsError() {
		return fmt.Errorf("billingrepo: create index response error: %s", createRes.String())
	}

	r.logger.Info("Created billing usage index", zap.String("index", usageIndex))
	return nil
}

// Emit implements billing.UsageEmitter. It calls Store internally and is
// safe to call from a goroutine — it retries up to 3 times on transient
// failures before logging an error.
func (r *ESUsageRepo) Emit(ctx context.Context, event *billing.UsageEvent) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Currency == "" {
		event.Currency = "INR"
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if err := r.Store(ctx, event); err != nil {
			lastErr = err
			r.logger.Warn("billing emit: store failed, retrying",
				zap.Int("attempt", attempt),
				zap.String("notification_id", event.NotificationID),
				zap.Error(err))
			time.Sleep(time.Duration(attempt*attempt) * 100 * time.Millisecond) // 100ms, 400ms, 900ms
			continue
		}
		return nil
	}

	r.logger.Error("billing emit: all retries exhausted — usage event LOST",
		zap.String("notification_id", event.NotificationID),
		zap.String("tenant_id", event.TenantID),
		zap.String("channel", event.Channel),
		zap.String("credential_source", event.CredentialSource),
		zap.Error(lastErr))
	return lastErr
}

// Store persists a single UsageEvent. ID, Currency, and Timestamp must be
// populated before calling (Emit does this automatically).
func (r *ESUsageRepo) Store(ctx context.Context, event *billing.UsageEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("billingrepo: marshal event: %w", err)
	}

	res, err := r.es.Index(
		usageIndex,
		strings.NewReader(string(body)),
		r.es.Index.WithDocumentID(event.ID),
		r.es.Index.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("billingrepo: index event: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("billingrepo: es error: %s", res.String())
	}
	return nil
}

// GetSummary aggregates usage data for a tenant over a given period, grouped
// by channel and credential_source. Returns one UsageSummary per group.
func (r *ESUsageRepo) GetSummary(ctx context.Context, tenantIDs []string, from, to time.Time) ([]billing.UsageSummary, error) {
	if len(tenantIDs) == 0 {
		return nil, nil
	}
	b, _ := json.Marshal(tenantIDs)

	query := fmt.Sprintf(`{
		"size": 0,
		"query": {
			"bool": {
				"filter": [
					{ "terms":  { "tenant_id": %s } },
					{ "range": { "timestamp": { "gte": %q, "lte": %q } } }
				]
			}
		},
		"aggs": {
			"by_channel": {
				"terms": { "field": "channel", "size": 50 },
				"aggs": {
					"by_cred_source": {
						"terms": { "field": "credential_source", "size": 10 },
						"aggs": {
							"total_billed": { "sum": { "field": "billed_paisa" } },
							"msg_count":   { "value_count": { "field": "id" } }
						}
					}
				}
			}
		}
	}`, string(b), from.Format(time.RFC3339), to.Format(time.RFC3339))

	res, err := r.es.Search(
		r.es.Search.WithIndex(usageIndex),
		r.es.Search.WithBody(strings.NewReader(query)),
		r.es.Search.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("billingrepo: search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("billingrepo: search error: %s", res.String())
	}

	var raw struct {
		Aggregations struct {
			ByChannel struct {
				Buckets []struct {
					Key         string `json:"key"`
					ByCredSource struct {
						Buckets []struct {
							Key         string `json:"key"`
							TotalBilled struct{ Value float64 `json:"value"` } `json:"total_billed"`
							MsgCount    struct{ Value float64 `json:"value"` } `json:"msg_count"`
						} `json:"buckets"`
					} `json:"by_cred_source"`
				} `json:"buckets"`
			} `json:"by_channel"`
		} `json:"aggregations"`
	}

	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("billingrepo: decode aggregation: %w", err)
	}

	var summaries []billing.UsageSummary
	for _, chBucket := range raw.Aggregations.ByChannel.Buckets {
		for _, credBucket := range chBucket.ByCredSource.Buckets {
			// For aggregate queries across multiple apps, the UI only needs "workspace" level
			// We can omit TenantID or set it to a placeholder since this summarizes multiple.
			summaries = append(summaries, billing.UsageSummary{
				TenantID:         "workspace",
				Channel:          chBucket.Key,
				CredentialSource: credBucket.Key,
				MessageCount:     int64(credBucket.MsgCount.Value),
				TotalBilledPaisa: int64(credBucket.TotalBilled.Value),
				PeriodStart:      from.Format(time.RFC3339),
				PeriodEnd:        to.Format(time.RFC3339),
			})
		}
	}

	return summaries, nil
}

// GetEvents returns raw usage events for a tenant in a period (for audit/export).
func (r *ESUsageRepo) GetEvents(ctx context.Context, tenantID string, from, to time.Time, limit int) ([]billing.UsageEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := fmt.Sprintf(`{
		"size": %d,
		"sort": [{ "timestamp": { "order": "desc" } }],
		"query": {
			"bool": {
				"filter": [
					{ "term":  { "tenant_id": %q } },
					{ "range": { "timestamp": { "gte": %q, "lte": %q } } }
				]
			}
		}
	}`, limit, tenantID, from.Format(time.RFC3339), to.Format(time.RFC3339))

	res, err := r.es.Search(
		r.es.Search.WithIndex(usageIndex),
		r.es.Search.WithBody(strings.NewReader(query)),
		r.es.Search.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("billingrepo: search events: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("billingrepo: search events error: %s", res.String())
	}

	var raw struct {
		Hits struct {
			Hits []struct {
				Source billing.UsageEvent `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("billingrepo: decode events: %w", err)
	}

	events := make([]billing.UsageEvent, 0, len(raw.Hits.Hits))
	for _, h := range raw.Hits.Hits {
		events = append(events, h.Source)
	}

	return events, nil
}
