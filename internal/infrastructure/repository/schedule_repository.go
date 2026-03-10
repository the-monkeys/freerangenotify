package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/schedule"
	"go.uber.org/zap"
)

// ScheduleRepository implements schedule.Repository using Elasticsearch
type ScheduleRepository struct {
	*BaseRepository
}

// NewScheduleRepository creates a new schedule repository
func NewScheduleRepository(client *elasticsearch.Client, logger *zap.Logger) schedule.Repository {
	return &ScheduleRepository{
		BaseRepository: NewBaseRepository(client, "workflow_schedules", logger, RefreshWaitFor),
	}
}

func (r *ScheduleRepository) Create(ctx context.Context, s *schedule.WorkflowSchedule) error {
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	return r.BaseRepository.Create(ctx, s.ID, s)
}

func (r *ScheduleRepository) GetByID(ctx context.Context, id string) (*schedule.WorkflowSchedule, error) {
	doc, err := r.BaseRepository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var s schedule.WorkflowSchedule
	if err := mapToStruct(doc, &s); err != nil {
		return nil, fmt.Errorf("failed to map document to schedule: %w", err)
	}
	return &s, nil
}

func (r *ScheduleRepository) List(ctx context.Context, appID, environmentID string, limit, offset int) ([]*schedule.WorkflowSchedule, int64, error) {
	filters := []map[string]interface{}{
		{"term": map[string]interface{}{"app_id": appID}},
	}
	if environmentID != "" && environmentID != "default" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{"environment_id": environmentID},
		})
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": filters,
			},
		},
		"sort": []map[string]interface{}{
			{"created_at": map[string]interface{}{"order": "desc"}},
		},
		"from": offset,
		"size": limit,
	}

	result, err := r.BaseRepository.Search(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	var schedules []*schedule.WorkflowSchedule
	for _, hit := range result.Hits {
		var s schedule.WorkflowSchedule
		if err := mapToStruct(hit, &s); err != nil {
			r.BaseRepository.logger.Error("Failed to map document to schedule", zap.Error(err))
			continue
		}
		schedules = append(schedules, &s)
	}
	return schedules, result.Total, nil
}

// ListDue returns all active schedules (caller filters by cron match)
func (r *ScheduleRepository) ListDue(ctx context.Context, at time.Time) ([]*schedule.WorkflowSchedule, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{"status": "active"},
		},
		"size": 1000,
	}

	result, err := r.BaseRepository.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	var schedules []*schedule.WorkflowSchedule
	for _, hit := range result.Hits {
		var s schedule.WorkflowSchedule
		if err := mapToStruct(hit, &s); err != nil {
			continue
		}
		schedules = append(schedules, &s)
	}
	return schedules, nil
}

func (r *ScheduleRepository) Update(ctx context.Context, s *schedule.WorkflowSchedule) error {
	s.UpdatedAt = time.Now()
	return r.BaseRepository.Update(ctx, s.ID, s)
}

func (r *ScheduleRepository) Delete(ctx context.Context, id string) error {
	return r.BaseRepository.Delete(ctx, id)
}
