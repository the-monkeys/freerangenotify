package repository

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/tenant"
	"go.uber.org/zap"
)

// TenantRepository implements tenant.Repository using Elasticsearch.
type TenantRepository struct {
	base *BaseRepository
}

// NewTenantRepository creates a new TenantRepository.
func NewTenantRepository(client *elasticsearch.Client, logger *zap.Logger) tenant.Repository {
	return &TenantRepository{
		base: NewBaseRepository(client, "tenants", logger, RefreshWaitFor),
	}
}

func (r *TenantRepository) Create(ctx context.Context, t *tenant.Tenant) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return r.base.Create(ctx, t.ID, t)
}

func (r *TenantRepository) GetByID(ctx context.Context, id string) (*tenant.Tenant, error) {
	doc, err := r.base.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var t tenant.Tenant
	if err := mapToStruct(doc, &t); err != nil {
		return nil, fmt.Errorf("failed to map tenant: %w", err)
	}
	return &t, nil
}

func (r *TenantRepository) GetBySlug(ctx context.Context, slug string) (*tenant.Tenant, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"slug": slug,
			},
		},
		"size": 1,
	}
	result, err := r.base.Search(ctx, query)
	if err != nil || result.Total == 0 {
		return nil, nil
	}
	var t tenant.Tenant
	if err := mapToStruct(result.Hits[0], &t); err != nil {
		return nil, fmt.Errorf("failed to map tenant: %w", err)
	}
	return &t, nil
}

func (r *TenantRepository) ListByCreatedBy(ctx context.Context, userID string) ([]*tenant.Tenant, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{"created_by": userID},
		},
		"sort": []map[string]interface{}{{"created_at": "desc"}},
		"size": 100,
	}
	result, err := r.base.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	tenants := make([]*tenant.Tenant, 0, len(result.Hits))
	for _, hit := range result.Hits {
		var t tenant.Tenant
		if err := mapToStruct(hit, &t); err != nil {
			continue
		}
		tenants = append(tenants, &t)
	}
	return tenants, nil
}

func (r *TenantRepository) Update(ctx context.Context, t *tenant.Tenant) error {
	return r.base.Update(ctx, t.ID, t)
}

func (r *TenantRepository) Delete(ctx context.Context, id string) error {
	return r.base.Delete(ctx, id)
}

// SlugFromName creates a URL-safe slug from a name.
func SlugFromName(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "org"
	}
	return slug
}
