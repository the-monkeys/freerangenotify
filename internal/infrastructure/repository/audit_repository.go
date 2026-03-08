package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/audit"
	"go.uber.org/zap"
)

// AuditRepository implements audit.Repository using Elasticsearch.
// All operations are append-only — Update and Delete are intentionally omitted.
type AuditRepository struct {
	base *BaseRepository
}

// NewAuditRepository creates a new AuditRepository.
func NewAuditRepository(client *elasticsearch.Client, logger *zap.Logger) audit.Repository {
	return &AuditRepository{
		base: NewBaseRepository(client, "audit_logs", logger),
	}
}

func (r *AuditRepository) Create(ctx context.Context, log *audit.AuditLog) error {
	if log.AuditID == "" {
		log.AuditID = uuid.New().String()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}
	return r.base.Create(ctx, log.AuditID, log)
}

func (r *AuditRepository) GetByID(ctx context.Context, id string) (*audit.AuditLog, error) {
	raw, err := r.base.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.unmarshal(raw)
}

func (r *AuditRepository) List(ctx context.Context, filter audit.Filter) ([]*audit.AuditLog, error) {
	query := r.buildQuery(filter)

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query["size"] = limit
	query["from"] = offset

	result, err := r.base.Search(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit logs: %w", err)
	}

	logs := make([]*audit.AuditLog, 0, len(result.Hits))
	for _, hit := range result.Hits {
		log, err := r.unmarshal(hit)
		if err != nil {
			r.base.logger.Warn("Failed to unmarshal audit log", zap.Error(err))
			continue
		}
		logs = append(logs, log)
	}
	return logs, nil
}

func (r *AuditRepository) Count(ctx context.Context, filter audit.Filter) (int64, error) {
	query := r.buildQuery(filter)
	return r.base.Count(ctx, query)
}

// buildQuery constructs an ES bool query from the filter.
func (r *AuditRepository) buildQuery(filter audit.Filter) map[string]interface{} {
	musts := []map[string]interface{}{}

	if filter.AppID != "" {
		musts = append(musts, map[string]interface{}{
			"term": map[string]interface{}{"app_id": filter.AppID},
		})
	}
	if len(filter.AppIDs) > 0 {
		musts = append(musts, map[string]interface{}{
			"terms": map[string]interface{}{"app_id": filter.AppIDs},
		})
	}
	if filter.EnvironmentID != "" && filter.EnvironmentID != "default" {
		musts = append(musts, map[string]interface{}{
			"term": map[string]interface{}{"environment_id": filter.EnvironmentID},
		})
	}
	if filter.ActorID != "" {
		musts = append(musts, map[string]interface{}{
			"term": map[string]interface{}{"actor_id": filter.ActorID},
		})
	}
	if filter.Action != "" {
		musts = append(musts, map[string]interface{}{
			"term": map[string]interface{}{"action": filter.Action},
		})
	}
	if filter.Resource != "" {
		musts = append(musts, map[string]interface{}{
			"term": map[string]interface{}{"resource": filter.Resource},
		})
	}
	if filter.ResourceID != "" {
		musts = append(musts, map[string]interface{}{
			"term": map[string]interface{}{"resource_id": filter.ResourceID},
		})
	}

	if len(musts) == 0 {
		return map[string]interface{}{
			"query": map[string]interface{}{"match_all": map[string]interface{}{}},
			"sort":  []map[string]interface{}{{"created_at": map[string]interface{}{"order": "desc"}}},
		}
	}

	return map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": musts,
			},
		},
		"sort": []map[string]interface{}{{"created_at": map[string]interface{}{"order": "desc"}}},
	}
}

func (r *AuditRepository) unmarshal(raw map[string]interface{}) (*audit.AuditLog, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal raw audit log: %w", err)
	}
	var log audit.AuditLog
	if err := json.Unmarshal(data, &log); err != nil {
		return nil, fmt.Errorf("failed to unmarshal audit log: %w", err)
	}
	return &log, nil
}
