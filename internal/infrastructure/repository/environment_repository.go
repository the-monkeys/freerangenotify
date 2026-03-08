package repository

import (
	"context"
	"encoding/json"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/environment"
	"go.uber.org/zap"
)

// EnvironmentRepository implements environment.Repository using Elasticsearch.
type EnvironmentRepository struct {
	base *BaseRepository
}

// NewEnvironmentRepository creates a new EnvironmentRepository.
func NewEnvironmentRepository(client *elasticsearch.Client, logger *zap.Logger) environment.Repository {
	return &EnvironmentRepository{
		base: NewBaseRepository(client, "environments", logger),
	}
}

func (r *EnvironmentRepository) Create(ctx context.Context, env *environment.Environment) error {
	return r.base.Create(ctx, env.ID, env)
}

func (r *EnvironmentRepository) GetByID(ctx context.Context, id string) (*environment.Environment, error) {
	raw, err := r.base.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.unmarshal(raw)
}

func (r *EnvironmentRepository) GetByAPIKey(ctx context.Context, apiKey string) (*environment.Environment, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"api_key": apiKey,
			},
		},
	}

	result, err := r.base.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	if result.Total == 0 || len(result.Hits) == 0 {
		return nil, nil
	}
	return r.unmarshal(result.Hits[0])
}

func (r *EnvironmentRepository) ListByApp(ctx context.Context, appID string) ([]environment.Environment, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"app_id": appID,
			},
		},
		"sort": []map[string]interface{}{
			{"created_at": "asc"},
		},
		"size": 100,
	}

	result, err := r.base.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	envs := make([]environment.Environment, 0, len(result.Hits))
	for _, hit := range result.Hits {
		env, err := r.unmarshal(hit)
		if err != nil {
			return nil, err
		}
		envs = append(envs, *env)
	}
	return envs, nil
}

func (r *EnvironmentRepository) Delete(ctx context.Context, id string) error {
	return r.base.Delete(ctx, id)
}

func (r *EnvironmentRepository) unmarshal(data map[string]interface{}) (*environment.Environment, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var env environment.Environment
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, err
	}
	return &env, nil
}
