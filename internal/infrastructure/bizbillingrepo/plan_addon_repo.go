package bizbillingrepo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

const planAddonIndex = "frn_biz_plan_addons"

// PlanAddonRepo is the Elasticsearch-backed repository for plan addons.
type PlanAddonRepo struct {
	base   *repository.BaseRepository
	logger *zap.Logger
}

// NewPlanAddonRepo creates a new PlanAddonRepo.
func NewPlanAddonRepo(es *elasticsearch.Client, logger *zap.Logger) *PlanAddonRepo {
	return &PlanAddonRepo{
		base:   repository.NewBaseRepository(es, planAddonIndex, logger, repository.RefreshWaitFor),
		logger: logger,
	}
}

func (r *PlanAddonRepo) Create(ctx context.Context, addon *bizbilling.PlanAddon) error {
	if addon == nil || addon.ID == "" {
		return fmt.Errorf("bizbillingrepo: addon and ID are required")
	}
	return r.base.Create(ctx, addon.ID, addon)
}

func (r *PlanAddonRepo) GetByID(ctx context.Context, id string) (*bizbilling.PlanAddon, error) {
	raw, err := r.base.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("bizbillingrepo: marshal addon: %w", err)
	}

	var addon bizbilling.PlanAddon
	if err := json.Unmarshal(data, &addon); err != nil {
		return nil, fmt.Errorf("bizbillingrepo: unmarshal addon: %w", err)
	}
	return &addon, nil
}

func (r *PlanAddonRepo) Update(ctx context.Context, addon *bizbilling.PlanAddon) error {
	if addon == nil || addon.ID == "" {
		return fmt.Errorf("bizbillingrepo: addon and ID are required")
	}
	return r.base.Replace(ctx, addon.ID, addon)
}

func (r *PlanAddonRepo) Delete(ctx context.Context, id string) error {
	return r.base.Delete(ctx, id)
}

func (r *PlanAddonRepo) ListByPlanID(ctx context.Context, appID, planID string) ([]*bizbilling.PlanAddon, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"app_id": appID}},
					{"term": map[string]interface{}{"plan_id": planID}},
					{"term": map[string]interface{}{"active": true}},
				},
			},
		},
		"sort": []map[string]interface{}{
			{"created_at": map[string]interface{}{"order": "asc"}},
		},
		"size": 100,
	}

	result, err := r.base.Search(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("bizbillingrepo: list addons by plan: %w", err)
	}

	addons := make([]*bizbilling.PlanAddon, 0, len(result.Hits))
	for _, hit := range result.Hits {
		data, err := json.Marshal(hit)
		if err != nil {
			continue
		}
		var a bizbilling.PlanAddon
		if err := json.Unmarshal(data, &a); err != nil {
			continue
		}
		addons = append(addons, &a)
	}

	return addons, nil
}
