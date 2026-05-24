package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"go.uber.org/zap"
)

// WhatsAppRichTemplateRepository persists RichTemplate documents in the
// `whatsapp_rich_templates` Elasticsearch index. Uses RefreshWaitFor so
// authoring round-trips (POST /v1/whatsapp/rich-templates → GET) see the
// fresh document immediately — at the cost of slightly slower writes, which
// is acceptable for an authoring API rather than a send-hot-path.
type WhatsAppRichTemplateRepository struct {
	base   *BaseRepository
	logger *zap.Logger
}

// NewWhatsAppRichTemplateRepository wires up a repository backed by the
// given Elasticsearch client. Implements whatsapp.RichTemplateRepository.
func NewWhatsAppRichTemplateRepository(client *elasticsearch.Client, logger *zap.Logger) whatsapp.RichTemplateRepository {
	return &WhatsAppRichTemplateRepository{
		base:   NewBaseRepository(client, "whatsapp_rich_templates", logger, RefreshWaitFor),
		logger: logger,
	}
}

// Create persists a new RichTemplate. Caller is responsible for assigning
// the FRN-internal ID (frn_tpl_*) and stamping CreatedAt / UpdatedAt — this
// stays consistent with the rest of the repository layer which avoids
// hidden time mutations so test fixtures stay deterministic.
func (r *WhatsAppRichTemplateRepository) Create(ctx context.Context, tpl *whatsapp.RichTemplate) error {
	if tpl == nil || tpl.ID == "" {
		return fmt.Errorf("rich template missing id")
	}
	return r.base.Create(ctx, tpl.ID, tpl)
}

// GetByID fetches a template by FRN-internal ID. Returns the standard
// pkgerrors.NotFound when the document is missing, propagated from
// BaseRepository.GetByID.
func (r *WhatsAppRichTemplateRepository) GetByID(ctx context.Context, id string) (*whatsapp.RichTemplate, error) {
	doc, err := r.base.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var tpl whatsapp.RichTemplate
	if err := mapToStruct(doc, &tpl); err != nil {
		return nil, fmt.Errorf("failed to map rich template document: %w", err)
	}
	return &tpl, nil
}

// GetByName fetches a template by (app_id, name). Returns nil + nil when the
// document is missing so callers can use this to enforce idempotent create
// without distinguishing a real ES error from a not-found result.
func (r *WhatsAppRichTemplateRepository) GetByName(ctx context.Context, appID, name string) (*whatsapp.RichTemplate, error) {
	query := map[string]interface{}{
		"size": 1,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"app_id": appID}},
					{"term": map[string]interface{}{"name": name}},
				},
			},
		},
	}
	result, err := r.base.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	if result.Total == 0 {
		return nil, nil
	}
	var tpl whatsapp.RichTemplate
	if err := mapToStruct(result.Hits[0], &tpl); err != nil {
		return nil, fmt.Errorf("failed to map rich template document: %w", err)
	}
	return &tpl, nil
}

// List returns rich templates matching the filter, paginated.
// app_id is required. The default page size is 50; the cap is 500 to keep
// page payloads bounded for the UI.
func (r *WhatsAppRichTemplateRepository) List(ctx context.Context, filter whatsapp.RichTemplateFilter) ([]*whatsapp.RichTemplate, int64, error) {
	if filter.AppID == "" {
		return nil, 0, fmt.Errorf("app_id is required for listing rich templates")
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	} else if limit > 500 {
		limit = 500
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	must := []map[string]interface{}{
		{"term": map[string]interface{}{"app_id": filter.AppID}},
	}
	if filter.TenantID != "" {
		must = append(must, map[string]interface{}{"term": map[string]interface{}{"tenant_id": filter.TenantID}})
	}
	if filter.Kind != "" {
		must = append(must, map[string]interface{}{"term": map[string]interface{}{"kind": filter.Kind}})
	}
	if filter.Status != "" {
		must = append(must, map[string]interface{}{"term": map[string]interface{}{"approval_state": filter.Status}})
	}
	if filter.NamePrefix != "" {
		must = append(must, map[string]interface{}{"prefix": map[string]interface{}{"name": filter.NamePrefix}})
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{"must": must},
		},
		"sort": []map[string]interface{}{
			{"updated_at": map[string]interface{}{"order": "desc"}},
		},
		"from": offset,
		"size": limit,
	}

	result, err := r.base.Search(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	out := make([]*whatsapp.RichTemplate, 0, len(result.Hits))
	for _, hit := range result.Hits {
		var tpl whatsapp.RichTemplate
		if err := mapToStruct(hit, &tpl); err != nil {
			// Skip malformed rows rather than failing the whole list — surface
			// the warning so they can be investigated separately.
			r.logger.Warn("Failed to map rich template hit", zap.Error(err))
			continue
		}
		out = append(out, &tpl)
	}
	return out, result.Total, nil
}

// Update replaces the document with the given template. Stamps UpdatedAt if
// the caller has not set it, keeping write semantics consistent across the
// repository: every Update mutates UpdatedAt.
func (r *WhatsAppRichTemplateRepository) Update(ctx context.Context, tpl *whatsapp.RichTemplate) error {
	if tpl == nil || tpl.ID == "" {
		return fmt.Errorf("rich template missing id")
	}
	if tpl.UpdatedAt.IsZero() {
		tpl.UpdatedAt = time.Now().UTC()
	}
	return r.base.Replace(ctx, tpl.ID, tpl)
}

// Delete removes a template by ID.
func (r *WhatsAppRichTemplateRepository) Delete(ctx context.Context, id string) error {
	return r.base.Delete(ctx, id)
}
