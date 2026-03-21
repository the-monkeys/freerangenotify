package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/resourcelink"
	"go.uber.org/zap"
)

type ResourceLinkRepository struct {
	*BaseRepository
}

func NewResourceLinkRepository(client *elasticsearch.Client, logger *zap.Logger) resourcelink.Repository {
	return &ResourceLinkRepository{
		BaseRepository: NewBaseRepository(client, "app_resource_links", logger, RefreshWaitFor),
	}
}

func (r *ResourceLinkRepository) Create(ctx context.Context, link *resourcelink.Link) error {
	return r.BaseRepository.Create(ctx, link.LinkID, link)
}

func (r *ResourceLinkRepository) Delete(ctx context.Context, linkID string) error {
	return r.BaseRepository.Delete(ctx, linkID)
}

func (r *ResourceLinkRepository) DeleteByTargetAndResource(ctx context.Context, targetAppID string, rt resourcelink.ResourceType, resourceID string) error {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"target_app_id": targetAppID}},
					{"term": map[string]interface{}{"resource_type": string(rt)}},
					{"term": map[string]interface{}{"resource_id": resourceID}},
				},
			},
		},
	}
	return r.BaseRepository.DeleteByQuery(ctx, query)
}

func (r *ResourceLinkRepository) DeleteAllByTarget(ctx context.Context, targetAppID string) error {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{"target_app_id": targetAppID},
		},
	}
	return r.BaseRepository.DeleteByQuery(ctx, query)
}

func (r *ResourceLinkRepository) GetLinkedAppIDs(ctx context.Context, targetAppID string, rt resourcelink.ResourceType) ([]string, error) {
	links, err := r.ListByTarget(ctx, targetAppID, &rt)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var appIDs []string
	for _, l := range links {
		if !seen[l.SourceAppID] {
			seen[l.SourceAppID] = true
			appIDs = append(appIDs, l.SourceAppID)
		}
	}
	return appIDs, nil
}

func (r *ResourceLinkRepository) GetLinkedResourceIDs(ctx context.Context, targetAppID, sourceAppID string, rt resourcelink.ResourceType) ([]string, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"target_app_id": targetAppID}},
					{"term": map[string]interface{}{"source_app_id": sourceAppID}},
					{"term": map[string]interface{}{"resource_type": string(rt)}},
				},
			},
		},
		"size": 10000,
	}
	result, err := r.BaseRepository.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(result.Hits))
	for _, hit := range result.Hits {
		if id, ok := hit["resource_id"].(string); ok {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (r *ResourceLinkRepository) Exists(ctx context.Context, targetAppID string, rt resourcelink.ResourceType, resourceID string) (bool, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"target_app_id": targetAppID}},
					{"term": map[string]interface{}{"resource_type": string(rt)}},
					{"term": map[string]interface{}{"resource_id": resourceID}},
				},
			},
		},
	}
	count, err := r.BaseRepository.Count(ctx, query)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ResourceLinkRepository) ListByTarget(ctx context.Context, targetAppID string, rt *resourcelink.ResourceType) ([]*resourcelink.Link, error) {
	filters := []map[string]interface{}{
		{"term": map[string]interface{}{"target_app_id": targetAppID}},
	}
	if rt != nil {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{"resource_type": string(*rt)},
		})
	}
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{"must": filters},
		},
		"size": 10000,
	}
	result, err := r.BaseRepository.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	links := make([]*resourcelink.Link, 0, len(result.Hits))
	for _, hit := range result.Hits {
		raw, _ := json.Marshal(hit)
		var link resourcelink.Link
		if err := json.Unmarshal(raw, &link); err == nil {
			links = append(links, &link)
		}
	}
	return links, nil
}

func (r *ResourceLinkRepository) BulkCreate(ctx context.Context, links []*resourcelink.Link) error {
	if len(links) == 0 {
		return nil
	}
	docs := make(map[string]interface{}, len(links))
	for _, l := range links {
		docs[l.LinkID] = l
	}
	err := r.BaseRepository.BulkCreate(ctx, docs)
	if err != nil {
		return fmt.Errorf("bulk create resource links: %w", err)
	}
	return nil
}

func (r *ResourceLinkRepository) ListBySource(ctx context.Context, sourceAppID string, rt *resourcelink.ResourceType) ([]*resourcelink.Link, error) {
	filters := []map[string]interface{}{
		{"term": map[string]interface{}{"source_app_id": sourceAppID}},
	}
	if rt != nil {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{"resource_type": string(*rt)},
		})
	}
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{"must": filters},
		},
		"size": 10000,
	}
	result, err := r.BaseRepository.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	links := make([]*resourcelink.Link, 0, len(result.Hits))
	for _, hit := range result.Hits {
		raw, _ := json.Marshal(hit)
		var link resourcelink.Link
		if err := json.Unmarshal(raw, &link); err == nil {
			links = append(links, &link)
		}
	}
	return links, nil
}

func (r *ResourceLinkRepository) DeleteAllBySource(ctx context.Context, sourceAppID string) error {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{"source_app_id": sourceAppID},
		},
	}
	return r.BaseRepository.DeleteByQuery(ctx, query)
}

func (r *ResourceLinkRepository) CountByResource(ctx context.Context, sourceAppID string, rt resourcelink.ResourceType, resourceID string) (int64, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"source_app_id": sourceAppID}},
					{"term": map[string]interface{}{"resource_type": string(rt)}},
					{"term": map[string]interface{}{"resource_id": resourceID}},
				},
			},
		},
	}
	return r.BaseRepository.Count(ctx, query)
}

func (r *ResourceLinkRepository) UpdateLink(ctx context.Context, link *resourcelink.Link) error {
	return r.BaseRepository.Update(ctx, link.LinkID, link)
}
