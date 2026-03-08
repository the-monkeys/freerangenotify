package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/digest"
	"go.uber.org/zap"
)

// DigestRepository implements digest.Repository using Elasticsearch.
type DigestRepository struct {
	*BaseRepository
}

// NewDigestRepository creates a new digest repository.
func NewDigestRepository(client *elasticsearch.Client, logger *zap.Logger) digest.Repository {
	return &DigestRepository{
		BaseRepository: NewBaseRepository(client, "digest_rules", logger),
	}
}

func (r *DigestRepository) Create(ctx context.Context, rule *digest.DigestRule) error {
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	return r.BaseRepository.Create(ctx, rule.ID, rule)
}

func (r *DigestRepository) GetByID(ctx context.Context, id string) (*digest.DigestRule, error) {
	doc, err := r.BaseRepository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var rule digest.DigestRule
	if err := mapToStruct(doc, &rule); err != nil {
		return nil, fmt.Errorf("failed to map document to digest rule: %w", err)
	}
	return &rule, nil
}

func (r *DigestRepository) GetActiveByKey(ctx context.Context, appID, digestKey string) (*digest.DigestRule, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"app_id": appID}},
					{"term": map[string]interface{}{"digest_key": digestKey}},
					{"term": map[string]interface{}{"status": "active"}},
				},
			},
		},
		"size": 1,
	}

	result, err := r.BaseRepository.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	if result.Total == 0 {
		return nil, nil // No active rule — not an error
	}

	var rule digest.DigestRule
	if err := mapToStruct(result.Hits[0], &rule); err != nil {
		return nil, fmt.Errorf("failed to map document to digest rule: %w", err)
	}
	return &rule, nil
}

func (r *DigestRepository) List(ctx context.Context, appID, environmentID string, limit, offset int) ([]*digest.DigestRule, int64, error) {
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

	var rules []*digest.DigestRule
	for _, hit := range result.Hits {
		var rule digest.DigestRule
		if err := mapToStruct(hit, &rule); err != nil {
			r.logger.Error("Failed to map document to digest rule", zap.Error(err))
			continue
		}
		rules = append(rules, &rule)
	}

	return rules, result.Total, nil
}

func (r *DigestRepository) Update(ctx context.Context, rule *digest.DigestRule) error {
	rule.UpdatedAt = time.Now()
	return r.BaseRepository.Update(ctx, rule.ID, rule)
}

func (r *DigestRepository) Delete(ctx context.Context, id string) error {
	return r.BaseRepository.Delete(ctx, id)
}
