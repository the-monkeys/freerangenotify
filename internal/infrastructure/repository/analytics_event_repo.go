package repository

import (
	"context"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/analytics"
	"go.uber.org/zap"
)

// AnalyticsEventRepository persists analytics events to the `analytics`
// Elasticsearch index. This is a minimal implementation focused on the
// Track() path used by the click-attribution redirect handler — full
// analytics.Repository (summaries, channel stats, …) lives in a separate
// later milestone.
type AnalyticsEventRepository struct {
	base *BaseRepository
}

// NewAnalyticsEventRepository wires up the repo. Uses RefreshFalse since
// analytics writes are bursty and downstream dashboards tolerate ~1s
// indexing latency.
func NewAnalyticsEventRepository(client *elasticsearch.Client, logger *zap.Logger) *AnalyticsEventRepository {
	return &AnalyticsEventRepository{
		base: NewBaseRepository(client, "analytics", logger, RefreshFalse),
	}
}

// Track persists a single Event keyed by Event.ID.
func (r *AnalyticsEventRepository) Track(ctx context.Context, e *analytics.Event) error {
	if e.ID == "" {
		// Fall back to a generated suffix; we don't want duplicate-key
		// failures from clients that didn't bother to set ID.
		e.ID = e.NotificationID + "_" + string(e.EventType)
	}
	return r.base.Create(ctx, e.ID, e)
}
