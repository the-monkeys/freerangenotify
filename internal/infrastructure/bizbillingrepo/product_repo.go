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

const productIndex = "frn_biz_products"

// ProductRepo is the Elasticsearch-backed repository for biz billing products.
type ProductRepo struct {
	base   *repository.BaseRepository
	logger *zap.Logger
}

// NewProductRepo creates a new ProductRepo.
func NewProductRepo(es *elasticsearch.Client, logger *zap.Logger) *ProductRepo {
	return &ProductRepo{
		base:   repository.NewBaseRepository(es, productIndex, logger, repository.RefreshWaitFor),
		logger: logger,
	}
}

func (r *ProductRepo) Create(ctx context.Context, product *bizbilling.Product) error {
	if product == nil || product.ID == "" {
		return fmt.Errorf("bizbillingrepo: product and ID are required")
	}
	return r.base.Create(ctx, product.ID, product)
}

func (r *ProductRepo) GetByID(ctx context.Context, id string) (*bizbilling.Product, error) {
	raw, err := r.base.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("bizbillingrepo: marshal product: %w", err)
	}

	var product bizbilling.Product
	if err := json.Unmarshal(data, &product); err != nil {
		return nil, fmt.Errorf("bizbillingrepo: unmarshal product: %w", err)
	}
	return &product, nil
}

func (r *ProductRepo) Update(ctx context.Context, product *bizbilling.Product) error {
	if product == nil || product.ID == "" {
		return fmt.Errorf("bizbillingrepo: product and ID are required")
	}
	return r.base.Replace(ctx, product.ID, product)
}

func (r *ProductRepo) Delete(ctx context.Context, id string) error {
	return r.base.Delete(ctx, id)
}

func (r *ProductRepo) List(ctx context.Context, filter bizbilling.ProductFilter) ([]*bizbilling.Product, int64, error) {
	// Build ES query
	must := []map[string]interface{}{
		{"term": map[string]interface{}{"app_id": filter.AppID}},
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
		return nil, 0, fmt.Errorf("bizbillingrepo: list products: %w", err)
	}

	products := make([]*bizbilling.Product, 0, len(result.Hits))
	for _, hit := range result.Hits {
		data, err := json.Marshal(hit)
		if err != nil {
			continue
		}
		var p bizbilling.Product
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		products = append(products, &p)
	}

	return products, result.Total, nil
}
