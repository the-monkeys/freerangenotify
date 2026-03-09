package repository

import (
	"context"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/tenant"
	"go.uber.org/zap"
)

// TenantMemberRepository implements tenant.MemberRepository using Elasticsearch.
type TenantMemberRepository struct {
	base *BaseRepository
}

// NewTenantMemberRepository creates a new TenantMemberRepository.
func NewTenantMemberRepository(client *elasticsearch.Client, logger *zap.Logger) tenant.MemberRepository {
	return &TenantMemberRepository{
		base: NewBaseRepository(client, "tenant_members", logger, RefreshWaitFor),
	}
}

func (r *TenantMemberRepository) Create(ctx context.Context, m *tenant.TenantMember) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return r.base.Create(ctx, m.ID, m)
}

func (r *TenantMemberRepository) GetByID(ctx context.Context, id string) (*tenant.TenantMember, error) {
	doc, err := r.base.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var m tenant.TenantMember
	if err := mapToStruct(doc, &m); err != nil {
		return nil, fmt.Errorf("failed to map tenant member: %w", err)
	}
	return &m, nil
}

func (r *TenantMemberRepository) GetByTenantAndUser(ctx context.Context, tenantID, userID string) (*tenant.TenantMember, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"tenant_id": tenantID}},
					{"term": map[string]interface{}{"user_id": userID}},
				},
			},
		},
		"size": 1,
	}
	result, err := r.base.Search(ctx, query)
	if err != nil || result.Total == 0 {
		return nil, nil
	}
	var m tenant.TenantMember
	if err := mapToStruct(result.Hits[0], &m); err != nil {
		return nil, fmt.Errorf("failed to map tenant member: %w", err)
	}
	return &m, nil
}

func (r *TenantMemberRepository) ListByTenant(ctx context.Context, tenantID string) ([]*tenant.TenantMember, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{"tenant_id": tenantID},
		},
		"sort": []map[string]interface{}{{"created_at": "asc"}},
		"size": 100,
	}
	result, err := r.base.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	members := make([]*tenant.TenantMember, 0, len(result.Hits))
	for _, hit := range result.Hits {
		var m tenant.TenantMember
		if err := mapToStruct(hit, &m); err != nil {
			continue
		}
		members = append(members, &m)
	}
	return members, nil
}

func (r *TenantMemberRepository) ListByUser(ctx context.Context, userID string) ([]*tenant.TenantMember, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{"user_id": userID},
		},
		"sort": []map[string]interface{}{{"created_at": "asc"}},
		"size": 100,
	}
	result, err := r.base.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	members := make([]*tenant.TenantMember, 0, len(result.Hits))
	for _, hit := range result.Hits {
		var m tenant.TenantMember
		if err := mapToStruct(hit, &m); err != nil {
			continue
		}
		members = append(members, &m)
	}
	return members, nil
}

func (r *TenantMemberRepository) Update(ctx context.Context, m *tenant.TenantMember) error {
	return r.base.Update(ctx, m.ID, m)
}

func (r *TenantMemberRepository) Delete(ctx context.Context, id string) error {
	return r.base.Delete(ctx, id)
}
