package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// BizUsageService handles usage meters and metered event reporting for
// usage-based billing.
type BizUsageService struct {
	meters *bizbillingrepo.Store[bizbilling.UsageMeter]
	events *bizbillingrepo.Store[bizbilling.BizUsageEvent]
	logger *zap.Logger
}

// NewBizUsageService creates a new BizUsageService.
func NewBizUsageService(stores *bizbillingrepo.Stores, logger *zap.Logger) *BizUsageService {
	return &BizUsageService{meters: stores.UsageMeters, events: stores.UsageEvents, logger: logger}
}

// CreateMeter defines a new usage meter.
func (s *BizUsageService) CreateMeter(ctx context.Context, appID string, req *bizbilling.CreateUsageMeterRequest) (*bizbilling.UsageMeter, error) {
	agg := req.Aggregation
	if agg == "" {
		agg = bizbilling.AggregationSum
	}
	if agg != bizbilling.AggregationSum && agg != bizbilling.AggregationMax && agg != bizbilling.AggregationLast {
		return nil, errors.BadRequest("aggregation must be sum, max, or last")
	}

	meter := &bizbilling.UsageMeter{
		ID:          uuid.New().String(),
		AppID:       appID,
		Name:        req.Name,
		Unit:        req.Unit,
		Aggregation: agg,
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.meters.Create(ctx, meter.ID, meter); err != nil {
		return nil, err
	}
	return meter, nil
}

// ListMeters lists an app's usage meters.
func (s *BizUsageService) ListMeters(ctx context.Context, appID string) ([]*bizbilling.UsageMeter, error) {
	meters, _, err := s.meters.Find(ctx, bizbillingrepo.NewQuery(appID).Body("created_at", 100, 0))
	return meters, err
}

// getOwnedMeter fetches a meter scoped to the app.
func (s *BizUsageService) getOwnedMeter(ctx context.Context, appID, meterID string) (*bizbilling.UsageMeter, error) {
	meter, err := s.meters.Get(ctx, meterID)
	if err != nil || meter.AppID != appID {
		return nil, errors.NotFound("usage_meter", meterID)
	}
	return meter, nil
}

// Report ingests a batch of usage events.
func (s *BizUsageService) Report(ctx context.Context, appID string, req *bizbilling.ReportUsageRequest) (int, error) {
	if len(req.Events) == 0 {
		return 0, errors.BadRequest("at least one event is required")
	}
	if len(req.Events) > 500 {
		return 0, errors.BadRequest("at most 500 events per batch")
	}

	// Validate meters once per batch.
	validMeters := map[string]bool{}
	accepted := 0
	now := time.Now().UTC()
	for _, e := range req.Events {
		if !validMeters[e.MeterID] {
			if _, err := s.getOwnedMeter(ctx, appID, e.MeterID); err != nil {
				return accepted, err
			}
			validMeters[e.MeterID] = true
		}

		ts := now
		if e.Timestamp != nil {
			ts = e.Timestamp.UTC()
		}
		event := &bizbilling.BizUsageEvent{
			ID:             uuid.New().String(),
			AppID:          appID,
			UserID:         e.UserID,
			SubscriptionID: e.SubscriptionID,
			MeterID:        e.MeterID,
			Quantity:       e.Quantity,
			Timestamp:      ts,
		}
		if err := s.events.Create(ctx, event.ID, event); err != nil {
			return accepted, err
		}
		accepted++
	}
	return accepted, nil
}

// Summary aggregates usage per meter for a user (optionally a subscription)
// within a period. Aggregation strategy comes from each meter definition.
func (s *BizUsageService) Summary(ctx context.Context, appID, userID, subscriptionID string, periodStart, periodEnd time.Time) ([]*bizbilling.BizUsageSummary, error) {
	meters, err := s.ListMeters(ctx, appID)
	if err != nil {
		return nil, err
	}

	summaries := make([]*bizbilling.BizUsageSummary, 0, len(meters))
	for _, meter := range meters {
		quantity, err := s.aggregateMeter(ctx, appID, userID, subscriptionID, meter, periodStart, periodEnd)
		if err != nil {
			s.logger.Warn("bizbilling: usage aggregation failed",
				zap.String("meter_id", meter.ID), zap.Error(err))
			continue
		}
		summaries = append(summaries, &bizbilling.BizUsageSummary{
			MeterID:     meter.ID,
			MeterName:   meter.Name,
			Unit:        meter.Unit,
			Aggregation: meter.Aggregation,
			Quantity:    quantity,
			PeriodStart: periodStart,
			PeriodEnd:   periodEnd,
		})
	}
	return summaries, nil
}

// aggregateMeter runs the ES aggregation matching the meter's strategy.
func (s *BizUsageService) aggregateMeter(ctx context.Context, appID, userID, subscriptionID string, meter *bizbilling.UsageMeter, from, to time.Time) (int64, error) {
	q := bizbillingrepo.NewQuery(appID).
		Term("meter_id", meter.ID).
		Term("user_id", userID).
		Term("subscription_id", subscriptionID).
		Range("timestamp", from, to)

	if meter.Aggregation == bizbilling.AggregationLast {
		events, _, err := s.events.Find(ctx, map[string]interface{}{
			"query": q.Bool(),
			"size":  1,
			"sort":  []map[string]interface{}{{"timestamp": map[string]interface{}{"order": "desc"}}},
		})
		if err != nil || len(events) == 0 {
			return 0, err
		}
		return events[0].Quantity, nil
	}

	aggName := "usage"
	aggType := "sum"
	if meter.Aggregation == bizbilling.AggregationMax {
		aggType = "max"
	}
	aggs, err := s.events.Aggregate(ctx, map[string]interface{}{
		"query": q.Bool(),
		"size":  0,
		"aggs": map[string]interface{}{
			aggName: map[string]interface{}{
				aggType: map[string]interface{}{"field": "quantity"},
			},
		},
	})
	if err != nil {
		return 0, err
	}
	return aggValue(aggs, aggName), nil
}

// aggValue extracts a numeric aggregation value from an ES response.
func aggValue(aggs map[string]interface{}, name string) int64 {
	node, ok := aggs[name].(map[string]interface{})
	if !ok {
		return 0
	}
	if v, ok := node["value"].(float64); ok {
		return int64(v)
	}
	return 0
}
