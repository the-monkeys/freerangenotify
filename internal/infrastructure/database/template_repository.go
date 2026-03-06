package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/template"
	"go.uber.org/zap"
)

type TemplateRepository struct {
	es     *ElasticsearchClient
	logger *zap.Logger
	index  string
}

func NewTemplateRepository(es *ElasticsearchClient, logger *zap.Logger) *TemplateRepository {
	return &TemplateRepository{
		es:     es,
		logger: logger,
		index:  "templates",
	}
}

// CreateIndex creates the templates index with proper mappings
func (r *TemplateRepository) CreateIndex(ctx context.Context) error {
	mapping := `{
		"mappings": {
			"properties": {
				"id": {"type": "keyword"},
				"app_id": {"type": "keyword"},
				"name": {"type": "keyword"},
				"description": {"type": "text"},
				"channel": {"type": "keyword"},
				"subject": {"type": "text"},
				"body": {"type": "text"},
				"variables": {"type": "keyword"},
				"metadata": {"type": "object", "enabled": false},
				"version": {"type": "integer"},
				"status": {"type": "keyword"},
				"locale": {"type": "keyword"},
				"created_by": {"type": "keyword"},
				"updated_by": {"type": "keyword"},
				"created_at": {"type": "date"},
				"updated_at": {"type": "date"}
			}
		}
	}`

	req := esapi.IndicesCreateRequest{
		Index: r.index,
		Body:  strings.NewReader(mapping),
	}

	res, err := req.Do(ctx, r.es.Client)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error creating index: %s", res.String())
	}

	r.logger.Info("Created templates index", zap.String("index", r.index))
	return nil
}

// Create creates a new template
func (r *TemplateRepository) Create(ctx context.Context, tmpl *template.Template) error {
	if tmpl.ID == "" {
		tmpl.ID = uuid.New().String()
	}
	tmpl.CreatedAt = time.Now()
	tmpl.UpdatedAt = time.Now()

	if tmpl.Version == 0 {
		tmpl.Version = 1
	}

	if tmpl.Status == "" {
		tmpl.Status = "active"
	}

	data, err := json.Marshal(tmpl)
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}

	req := esapi.IndexRequest{
		Index:      r.index,
		DocumentID: tmpl.ID,
		Body:       strings.NewReader(string(data)),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, r.es.Client)
	if err != nil {
		return fmt.Errorf("failed to create template: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error creating template: %s", res.String())
	}

	r.logger.Info("Created template", zap.String("id", tmpl.ID))
	return nil
}

// GetByID retrieves a template by ID
func (r *TemplateRepository) GetByID(ctx context.Context, id string) (*template.Template, error) {
	req := esapi.GetRequest{
		Index:      r.index,
		DocumentID: id,
	}

	res, err := req.Do(ctx, r.es.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		if res.StatusCode == 404 {
			return nil, fmt.Errorf("template not found")
		}
		return nil, fmt.Errorf("error getting template: %s", res.String())
	}

	var result struct {
		Source template.Template `json:"_source"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode template: %w", err)
	}

	return &result.Source, nil
}

// GetByAppAndName retrieves a template by app ID, name, and locale
func (r *TemplateRepository) GetByAppAndName(ctx context.Context, appID, name, locale string) (*template.Template, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"app_id": appID,
						},
					},
					{
						"term": map[string]interface{}{
							"name": name,
						},
					},
					{
						"term": map[string]interface{}{
							"locale": locale,
						},
					},
					{
						"term": map[string]interface{}{
							"status": "active",
						},
					},
				},
			},
		},
		"sort": []map[string]interface{}{
			{
				"version": map[string]interface{}{
					"order": "desc",
				},
			},
		},
		"size": 1,
	}

	queryData, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	req := esapi.SearchRequest{
		Index: []string{r.index},
		Body:  strings.NewReader(string(queryData)),
	}

	res, err := req.Do(ctx, r.es.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to search template: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("error searching template: %s", res.String())
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source template.Template `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search result: %w", err)
	}

	if len(result.Hits.Hits) == 0 {
		return nil, fmt.Errorf("template not found")
	}

	return &result.Hits.Hits[0].Source, nil
}

// Update updates an existing template
func (r *TemplateRepository) Update(ctx context.Context, tmpl *template.Template) error {
	tmpl.UpdatedAt = time.Now()

	data, err := json.Marshal(tmpl)
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}

	req := esapi.IndexRequest{
		Index:      r.index,
		DocumentID: tmpl.ID,
		Body:       strings.NewReader(string(data)),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, r.es.Client)
	if err != nil {
		return fmt.Errorf("failed to update template: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error updating template: %s", res.String())
	}

	r.logger.Info("Updated template", zap.String("id", tmpl.ID))
	return nil
}

// List retrieves templates based on filter criteria
func (r *TemplateRepository) List(ctx context.Context, filter template.Filter) ([]*template.Template, error) {
	must := []map[string]interface{}{}

	if filter.AppID != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"app_id": filter.AppID,
			},
		})
	} else if len(filter.AppIDs) > 0 {
		must = append(must, map[string]interface{}{
			"terms": map[string]interface{}{
				"app_id": filter.AppIDs,
			},
		})
	}

	if filter.EnvironmentID != "" && filter.EnvironmentID != "default" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"environment_id": filter.EnvironmentID,
			},
		})
	}

	if filter.Channel != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"channel": filter.Channel,
			},
		})
	}

	if filter.Name != "" {
		must = append(must, map[string]interface{}{
			"match": map[string]interface{}{
				"name": filter.Name,
			},
		})
	}

	if filter.Status != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"status": filter.Status,
			},
		})
	}

	if filter.Locale != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"locale": filter.Locale,
			},
		})
	}

	// Date range filter
	if filter.FromDate != nil || filter.ToDate != nil {
		rangeQuery := make(map[string]interface{})
		if filter.FromDate != nil {
			rangeQuery["gte"] = filter.FromDate.Format(time.RFC3339)
		}
		if filter.ToDate != nil {
			rangeQuery["lte"] = filter.ToDate.Format(time.RFC3339)
		}
		must = append(must, map[string]interface{}{
			"range": map[string]interface{}{
				"created_at": rangeQuery,
			},
		})
	}

	// Default to only showing active templates (latest versions).
	// CreateVersion marks previous versions as "inactive", so filtering
	// by status=active returns only the latest version of each template.
	if filter.Status == "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"status": "active",
			},
		})
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": must,
			},
		},
		"sort": []map[string]interface{}{
			{
				"created_at": map[string]interface{}{
					"order": "desc",
				},
			},
		},
		"from": filter.Offset,
		"size": filter.Limit,
	}

	if filter.Limit == 0 {
		query["size"] = 50 // default limit
	}

	queryData, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	req := esapi.SearchRequest{
		Index: []string{r.index},
		Body:  strings.NewReader(string(queryData)),
	}

	res, err := req.Do(ctx, r.es.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to search templates: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("error searching templates: %s", res.String())
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source template.Template `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search result: %w", err)
	}

	templates := make([]*template.Template, len(result.Hits.Hits))
	for i, hit := range result.Hits.Hits {
		templates[i] = &hit.Source
	}

	return templates, nil
}

// Delete hard-deletes a template from Elasticsearch.
func (r *TemplateRepository) Delete(ctx context.Context, id string) error {
	req := esapi.DeleteRequest{
		Index:      r.index,
		DocumentID: id,
		Refresh:    "true",
	}

	res, err := req.Do(ctx, r.es.Client)
	if err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error deleting template: %s", res.String())
	}

	r.logger.Info("Deleted template", zap.String("id", id))
	return nil
}

// GetVersions retrieves all versions of a template
func (r *TemplateRepository) GetVersions(ctx context.Context, appID, name, locale string) ([]*template.Template, error) {
	must := []map[string]interface{}{
		{
			"term": map[string]interface{}{
				"app_id": appID,
			},
		},
		{
			"term": map[string]interface{}{
				"name": name,
			},
		},
	}

	// Only filter by locale if explicitly provided
	if locale != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"locale": locale,
			},
		})
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": must,
			},
		},
		"sort": []map[string]interface{}{
			{
				"version": map[string]interface{}{
					"order": "desc",
				},
			},
		},
		"size": 100,
	}

	queryData, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	req := esapi.SearchRequest{
		Index: []string{r.index},
		Body:  strings.NewReader(string(queryData)),
	}

	res, err := req.Do(ctx, r.es.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to search versions: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("error searching versions: %s", res.String())
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source template.Template `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search result: %w", err)
	}

	versions := make([]*template.Template, len(result.Hits.Hits))
	for i, hit := range result.Hits.Hits {
		versions[i] = &hit.Source
	}

	return versions, nil
}

// CreateVersion creates a new version of an existing template
func (r *TemplateRepository) CreateVersion(ctx context.Context, tmpl *template.Template) error {
	// Get the latest version
	versions, err := r.GetVersions(ctx, tmpl.AppID, tmpl.Name, tmpl.Locale)
	if err != nil {
		return fmt.Errorf("failed to get versions: %w", err)
	}

	if len(versions) == 0 {
		return fmt.Errorf("template not found")
	}

	// Set new version number
	latestVersion := versions[0].Version
	tmpl.Version = latestVersion + 1
	tmpl.ID = uuid.New().String() // New ID for new version
	tmpl.CreatedAt = time.Now()
	tmpl.UpdatedAt = time.Now()

	// Deactivate previous version if the new one is active
	if tmpl.Status == "active" {
		for _, v := range versions {
			if v.Status == "active" {
				v.Status = "inactive"
				if err := r.Update(ctx, v); err != nil {
					r.logger.Error("Failed to deactivate previous version",
						zap.String("id", v.ID),
						zap.Error(err))
				}
			}
		}
	}

	return r.Create(ctx, tmpl)
}

// GetByVersion retrieves a specific version of a template
func (r *TemplateRepository) GetByVersion(ctx context.Context, appID, name, locale string, version int) (*template.Template, error) {
	must := []map[string]interface{}{
		{
			"term": map[string]interface{}{
				"app_id": appID,
			},
		},
		{
			"term": map[string]interface{}{
				"name": name,
			},
		},
		{
			"term": map[string]interface{}{
				"version": version,
			},
		},
	}

	// Only filter by locale if explicitly provided
	if locale != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{
				"locale": locale,
			},
		})
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": must,
			},
		},
		"size": 1,
	}

	queryData, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	req := esapi.SearchRequest{
		Index: []string{r.index},
		Body:  strings.NewReader(string(queryData)),
	}

	res, err := req.Do(ctx, r.es.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to search template version: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("error searching template version: %s", res.String())
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source template.Template `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search result: %w", err)
	}

	if len(result.Hits.Hits) == 0 {
		return nil, fmt.Errorf("template version %d not found", version)
	}

	return &result.Hits.Hits[0].Source, nil
}

// Count returns the total number of active templates across all apps.
func (r *TemplateRepository) Count(ctx context.Context) (int64, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"status": "active",
			},
		},
	}

	queryData, err := json.Marshal(query)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal count query: %w", err)
	}

	req := esapi.CountRequest{
		Index: []string{r.index},
		Body:  strings.NewReader(string(queryData)),
	}

	res, err := req.Do(ctx, r.es.Client)
	if err != nil {
		return 0, fmt.Errorf("failed to count templates: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return 0, fmt.Errorf("error counting templates: %s", res.String())
	}

	var countResult struct {
		Count int64 `json:"count"`
	}
	if err := json.NewDecoder(res.Body).Decode(&countResult); err != nil {
		return 0, fmt.Errorf("failed to decode count result: %w", err)
	}

	return countResult.Count, nil
}

// CountByFilter returns the number of active templates matching the given filter.
func (r *TemplateRepository) CountByFilter(ctx context.Context, filter template.Filter) (int64, error) {
	must := []map[string]interface{}{
		{"term": map[string]interface{}{"status": "active"}},
	}

	if filter.AppID != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{"app_id": filter.AppID},
		})
	} else if len(filter.AppIDs) > 0 {
		must = append(must, map[string]interface{}{
			"terms": map[string]interface{}{"app_id": filter.AppIDs},
		})
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{"must": must},
		},
	}

	queryData, err := json.Marshal(query)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal count query: %w", err)
	}

	req := esapi.CountRequest{
		Index: []string{r.index},
		Body:  strings.NewReader(string(queryData)),
	}

	res, err := req.Do(ctx, r.es.Client)
	if err != nil {
		return 0, fmt.Errorf("failed to count templates: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return 0, fmt.Errorf("error counting templates: %s", res.String())
	}

	var countResult struct {
		Count int64 `json:"count"`
	}
	if err := json.NewDecoder(res.Body).Decode(&countResult); err != nil {
		return 0, fmt.Errorf("failed to decode count result: %w", err)
	}

	return countResult.Count, nil
}
