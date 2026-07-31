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

const planIndex = "frn_biz_plans"

// PlanRepo is the Elasticsearch-backed repository for biz billing plans.
type PlanRepo struct {
	base   *repository.BaseRepository
	logger *zap.Logger
}

// NewPlanRepo creates a new PlanRepo.
func NewPlanRepo(es *elasticsearch.Client, logger *zap.Logger) *PlanRepo {
	return &PlanRepo{
		base:   repository.NewBaseRepository(es, planIndex, logger, repository.RefreshWaitFor),
		logger: logger,
	}
}

func (r *PlanRepo) Create(ctx context.Context, plan *bizbilling.Plan) error {
	if plan == nil || plan.ID == "" {
		return fmt.Errorf("bizbillingrepo: plan and ID are required")
	}
	return r.base.Create(ctx, plan.ID, plan)
}

func (r *PlanRepo) GetByID(ctx context.Context, id string) (*bizbilling.Plan, error) {
	raw, err := r.base.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("bizbillingrepo: marshal plan: %w", err)
	}

	var plan bizbilling.Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("bizbillingrepo: unmarshal plan: %w", err)
	}
	return &plan, nil
}

func (r *PlanRepo) Update(ctx context.Context, plan *bizbilling.Plan) error {
	if plan == nil || plan.ID == "" {
		return fmt.Errorf("bizbillingrepo: plan and ID are required")
	}
	return r.base.Replace(ctx, plan.ID, plan)
}

func (r *PlanRepo) Delete(ctx context.Context, id string) error {
	return r.base.Delete(ctx, id)
}

func (r *PlanRepo) List(ctx context.Context, filter bizbilling.PlanFilter) ([]*bizbilling.Plan, int64, error) {
	must := []map[string]interface{}{
		{"term": map[string]interface{}{"app_id": filter.AppID}},
	}

	if filter.ProductID != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{"product_id": filter.ProductID},
		})
	}

	if filter.Active != nil {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{"active": *filter.Active},
		})
	}

	if filter.Search != "" {
		must = append(must, map[string]interface{}{
			"match": map[string]interface{}{"name": filter.Search},
		})
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": must,
			},
		},
		"sort": []map[string]interface{}{
			{"created_at": map[string]interface{}{"order": "desc"}},
		},
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	query["size"] = limit
	query["from"] = filter.Offset

	result, err := r.base.Search(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("bizbillingrepo: list plans: %w", err)
	}

	plans := make([]*bizbilling.Plan, 0, len(result.Hits))
	for _, hit := range result.Hits {
		data, err := json.Marshal(hit)
		if err != nil {
			continue
		}
		var p bizbilling.Plan
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		plans = append(plans, &p)
	}

	return plans, result.Total, nil
}
