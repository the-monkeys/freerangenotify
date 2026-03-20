package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/the-monkeys/freerangenotify/internal/domain/resourcelink"
	"go.uber.org/zap"
)

// shareableIndex maps each linkable resource type to its Elasticsearch index name.
var shareableIndex = map[resourcelink.ResourceType]string{
	resourcelink.TypeUser:     "users",
	resourcelink.TypeTemplate: "templates",
	resourcelink.TypeWorkflow: "workflows",
	resourcelink.TypeDigest:   "digest_rules",
	resourcelink.TypeTopic:    "topics",
}

// nonShareableIndices are app-scoped ES indices that are always deleted with the app.
var nonShareableIndices = []string{
	"notifications",
	"workflow_executions",
	"workflow_schedules",
	"topic_subscriptions",
	"environments",
	"audit_logs",
	"audits",
	"app_memberships",
}

// CascadeDeleter handles resource adoption and cleanup when an app is deleted.
type CascadeDeleter struct {
	linkRepo resourcelink.Repository
	esClient *elasticsearch.Client
	logger   *zap.Logger
}

// NewCascadeDeleter creates a CascadeDeleter.
func NewCascadeDeleter(
	linkRepo resourcelink.Repository,
	esClient *elasticsearch.Client,
	logger *zap.Logger,
) *CascadeDeleter {
	return &CascadeDeleter{
		linkRepo: linkRepo,
		esClient: esClient,
		logger:   logger,
	}
}

// DeleteAppResources runs the full cascade: adopt shared resources, delete
// unshared resources, purge non-shareable data, and clean up all link records.
func (d *CascadeDeleter) DeleteAppResources(ctx context.Context, appID string) error {
	// 1. Adopt or delete shareable resources.
	if err := d.handleShareableResources(ctx, appID); err != nil {
		return fmt.Errorf("cascade shareable resources: %w", err)
	}

	// 2. Delete non-shareable (app-scoped) data.
	for _, index := range nonShareableIndices {
		if err := d.deleteByAppID(ctx, index, appID); err != nil {
			d.logger.Warn("Failed to purge non-shareable index",
				zap.String("index", index), zap.String("app_id", appID), zap.Error(err))
		}
	}

	// 3. Delete all links where this app is the target (it imported from others).
	if err := d.linkRepo.DeleteAllByTarget(ctx, appID); err != nil {
		d.logger.Warn("Failed to delete target links", zap.String("app_id", appID), zap.Error(err))
	}

	d.logger.Info("Cascade deletion completed", zap.String("app_id", appID))
	return nil
}

// handleShareableResources processes each shareable resource type. For every
// resource owned by appID that is linked by other apps, ownership is
// transferred to the first consumer. Unlinked resources are deleted.
func (d *CascadeDeleter) handleShareableResources(ctx context.Context, appID string) error {
	// Get all outgoing links (where this app is the source).
	links, err := d.linkRepo.ListBySource(ctx, appID, nil)
	if err != nil {
		return fmt.Errorf("list outgoing links: %w", err)
	}

	// Group links by (resource_type, resource_id) to find unique shared resources.
	type resKey struct {
		rt resourcelink.ResourceType
		id string
	}
	grouped := make(map[resKey][]*resourcelink.Link)
	for _, l := range links {
		k := resKey{rt: l.ResourceType, id: l.ResourceID}
		grouped[k] = append(grouped[k], l)
	}

	// For each shared resource: adopt into the first consumer.
	for key, consumers := range grouped {
		indexName, ok := shareableIndex[key.rt]
		if !ok {
			continue
		}

		newOwner := consumers[0].TargetAppID

		// Transfer resource ownership: update app_id in the resource index.
		if err := d.updateResourceOwner(ctx, indexName, key.id, newOwner); err != nil {
			d.logger.Error("Failed to adopt resource",
				zap.String("resource_type", string(key.rt)),
				zap.String("resource_id", key.id),
				zap.String("new_owner", newOwner),
				zap.Error(err))
			continue
		}

		// Delete the link for the new owner (they now own the resource directly).
		if err := d.linkRepo.Delete(ctx, consumers[0].LinkID); err != nil {
			d.logger.Warn("Failed to delete adopted link",
				zap.String("link_id", consumers[0].LinkID), zap.Error(err))
		}

		// Remaining consumers: update source_app_id to point to the new owner.
		for _, link := range consumers[1:] {
			link.SourceAppID = newOwner
			if err := d.linkRepo.UpdateLink(ctx, link); err != nil {
				d.logger.Warn("Failed to re-point link to new owner",
					zap.String("link_id", link.LinkID), zap.Error(err))
			}
		}

		d.logger.Debug("Resource adopted",
			zap.String("resource_type", string(key.rt)),
			zap.String("resource_id", key.id),
			zap.String("new_owner", newOwner),
			zap.Int("remaining_links", len(consumers)-1))
	}

	// After adoption, delete any remaining unlinked resources owned by the app.
	for _, indexName := range shareableIndex {
		if err := d.deleteByAppID(ctx, indexName, appID); err != nil {
			d.logger.Warn("Failed to delete unlinked resources",
				zap.String("index", indexName), zap.String("app_id", appID), zap.Error(err))
		}
	}

	// Finally, remove all source links (adopted ones were already deleted/updated above,
	// but there may be stale ones for deleted resources).
	if err := d.linkRepo.DeleteAllBySource(ctx, appID); err != nil {
		d.logger.Warn("Failed to delete remaining source links", zap.String("app_id", appID), zap.Error(err))
	}

	return nil
}

// updateResourceOwner sets app_id to newOwner for a single resource document.
func (d *CascadeDeleter) updateResourceOwner(ctx context.Context, indexName, resourceID, newOwner string) error {
	body := map[string]interface{}{
		"script": map[string]interface{}{
			"source": "ctx._source.app_id = params.new_owner",
			"lang":   "painless",
			"params": map[string]interface{}{"new_owner": newOwner},
		},
		"query": map[string]interface{}{
			"term": map[string]interface{}{"_id": resourceID},
		},
	}
	return d.updateByQuery(ctx, indexName, body)
}

// deleteByAppID deletes all documents in indexName where app_id == appID.
func (d *CascadeDeleter) deleteByAppID(ctx context.Context, indexName, appID string) error {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{"app_id": appID},
		},
	}
	return d.deleteByQuery(ctx, indexName, query)
}

// deleteByQuery executes an ES _delete_by_query request.
func (d *CascadeDeleter) deleteByQuery(ctx context.Context, indexName string, query map[string]interface{}) error {
	body, err := json.Marshal(query)
	if err != nil {
		return fmt.Errorf("marshal delete query: %w", err)
	}
	req := esapi.DeleteByQueryRequest{
		Index:             []string{indexName},
		Body:              bytes.NewReader(body),
		Refresh:           esapi.BoolPtr(true),
		Conflicts:         "proceed",
		AllowNoIndices:    esapi.BoolPtr(true),
		IgnoreUnavailable: esapi.BoolPtr(true),
	}
	res, err := req.Do(ctx, d.esClient)
	if err != nil {
		return fmt.Errorf("delete-by-query failed: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("delete-by-query failed for %s: %s", indexName, res.Status())
	}
	return nil
}

// updateByQuery executes an ES _update_by_query request.
func (d *CascadeDeleter) updateByQuery(ctx context.Context, indexName string, query map[string]interface{}) error {
	body, err := json.Marshal(query)
	if err != nil {
		return fmt.Errorf("marshal update query: %w", err)
	}
	req := esapi.UpdateByQueryRequest{
		Index:             []string{indexName},
		Body:              bytes.NewReader(body),
		Refresh:           esapi.BoolPtr(true),
		Conflicts:         "proceed",
		AllowNoIndices:    esapi.BoolPtr(true),
		IgnoreUnavailable: esapi.BoolPtr(true),
	}
	res, err := req.Do(ctx, d.esClient)
	if err != nil {
		return fmt.Errorf("update-by-query failed: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("update-by-query failed for %s: %s", indexName, res.Status())
	}
	return nil
}
