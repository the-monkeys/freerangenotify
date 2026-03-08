package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"go.uber.org/zap"
)

// MembershipRepository implements auth.MembershipRepository using Elasticsearch.
type MembershipRepository struct {
	base *BaseRepository
}

// NewMembershipRepository creates a new MembershipRepository.
func NewMembershipRepository(client *elasticsearch.Client, logger *zap.Logger) auth.MembershipRepository {
	return &MembershipRepository{
		base: NewBaseRepository(client, "app_memberships", logger),
	}
}

func (r *MembershipRepository) Create(ctx context.Context, m *auth.AppMembership) error {
	if m.MembershipID == "" {
		m.MembershipID = uuid.New().String()
	}
	now := time.Now().UTC()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	return r.base.Create(ctx, m.MembershipID, m)
}

func (r *MembershipRepository) GetByID(ctx context.Context, id string) (*auth.AppMembership, error) {
	raw, err := r.base.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.unmarshal(raw)
}

func (r *MembershipRepository) GetByAppAndUser(ctx context.Context, appID, userID string) (*auth.AppMembership, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"app_id": appID}},
					{"term": map[string]interface{}{"user_id": userID}},
				},
			},
		},
		"size": 1,
	}

	result, err := r.base.Search(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find membership: %w", err)
	}
	if len(result.Hits) == 0 {
		return nil, fmt.Errorf("membership not found")
	}
	return r.unmarshal(result.Hits[0])
}

func (r *MembershipRepository) ListByApp(ctx context.Context, appID string) ([]*auth.AppMembership, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{"app_id": appID},
		},
		"sort": []map[string]interface{}{{"created_at": map[string]interface{}{"order": "asc"}}},
		"size": 100,
	}

	result, err := r.base.Search(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list memberships: %w", err)
	}

	memberships := make([]*auth.AppMembership, 0, len(result.Hits))
	for _, hit := range result.Hits {
		m, err := r.unmarshal(hit)
		if err != nil {
			continue
		}
		memberships = append(memberships, m)
	}
	return memberships, nil
}

func (r *MembershipRepository) ListByUser(ctx context.Context, userID string) ([]*auth.AppMembership, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{"user_id": userID},
		},
		"sort": []map[string]interface{}{{"created_at": map[string]interface{}{"order": "asc"}}},
		"size": 100,
	}

	result, err := r.base.Search(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list user memberships: %w", err)
	}

	memberships := make([]*auth.AppMembership, 0, len(result.Hits))
	for _, hit := range result.Hits {
		m, err := r.unmarshal(hit)
		if err != nil {
			continue
		}
		memberships = append(memberships, m)
	}
	return memberships, nil
}

func (r *MembershipRepository) Update(ctx context.Context, m *auth.AppMembership) error {
	m.UpdatedAt = time.Now().UTC()
	return r.base.Update(ctx, m.MembershipID, m)
}

func (r *MembershipRepository) Delete(ctx context.Context, id string) error {
	return r.base.Delete(ctx, id)
}

func (r *MembershipRepository) unmarshal(raw map[string]interface{}) (*auth.AppMembership, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal raw membership: %w", err)
	}
	var m auth.AppMembership
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to unmarshal membership: %w", err)
	}
	return &m, nil
}
