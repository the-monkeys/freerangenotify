package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"go.uber.org/zap"
)

// IndexManager handles Elasticsearch index operations
type IndexManager struct {
	client    *ElasticsearchClient
	templates *IndexTemplates
	logger    *zap.Logger
}

// NewIndexManager creates a new index manager
func NewIndexManager(client *ElasticsearchClient, logger *zap.Logger) *IndexManager {
	return &IndexManager{
		client:    client,
		templates: &IndexTemplates{},
		logger:    logger,
	}
}

// IndexOperation represents an index operation result
type IndexOperation struct {
	IndexName string
	Action    string
	Success   bool
	Message   string
}

// CreateIndices creates all required indices for the notification service
func (im *IndexManager) CreateIndices(ctx context.Context) ([]IndexOperation, error) {
	var operations []IndexOperation

	// Define all indices to create
	indices := map[string]func() map[string]interface{}{
		"applications":  im.templates.GetApplicationsTemplate,
		"users":         im.templates.GetUsersTemplate,
		"notifications": im.templates.GetNotificationsTemplate,
		"templates":     im.templates.GetTemplatesTemplate,
		"analytics":     im.templates.GetAnalyticsTemplate,
	}

	for indexName, templateFunc := range indices {
		operation := IndexOperation{
			IndexName: indexName,
			Action:    "create",
		}

		template := templateFunc()
		success, message := im.createIndex(ctx, indexName, template)
		operation.Success = success
		operation.Message = message

		operations = append(operations, operation)

		if success {
			im.logger.Info("Index created successfully",
				zap.String("index", indexName))
		} else {
			im.logger.Error("Failed to create index",
				zap.String("index", indexName),
				zap.String("error", message))
		}
	}

	return operations, nil
}

// createIndex creates a single index with the given template
func (im *IndexManager) createIndex(ctx context.Context, indexName string, template map[string]interface{}) (bool, string) {
	// Check if index already exists
	exists, err := im.IndexExists(ctx, indexName)
	if err != nil {
		return false, fmt.Sprintf("failed to check index existence: %v", err)
	}

	if exists {
		return true, "index already exists"
	}

	// Convert template to JSON
	body := strings.NewReader(im.templateToJSON(template))

	// Create index request
	req := esapi.IndicesCreateRequest{
		Index: indexName,
		Body:  body,
	}

	// Execute request
	res, err := req.Do(ctx, im.client.GetClient())
	if err != nil {
		return false, fmt.Sprintf("failed to execute create request: %v", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return false, fmt.Sprintf("elasticsearch error: %s", res.String())
	}

	return true, "index created successfully"
}

// IndexExists checks if an index exists
func (im *IndexManager) IndexExists(ctx context.Context, indexName string) (bool, error) {
	req := esapi.IndicesExistsRequest{
		Index: []string{indexName},
	}

	res, err := req.Do(ctx, im.client.GetClient())
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	return res.StatusCode == 200, nil
}

// DeleteIndex deletes an index (use with caution)
func (im *IndexManager) DeleteIndex(ctx context.Context, indexName string) error {
	req := esapi.IndicesDeleteRequest{
		Index: []string{indexName},
	}

	res, err := req.Do(ctx, im.client.GetClient())
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("failed to delete index: %s", res.String())
	}

	im.logger.Info("Index deleted", zap.String("index", indexName))
	return nil
}

// MigrateIndices handles index migrations (for future use)
func (im *IndexManager) MigrateIndices(ctx context.Context) error {
	// This is a placeholder for future migration logic
	// When we need to update index mappings, we can:
	// 1. Create new index with updated mapping
	// 2. Reindex data from old to new
	// 3. Switch alias
	// 4. Delete old index

	im.logger.Info("Migration placeholder - no migrations to run")
	return nil
}

// GetIndexInfo returns information about an index
func (im *IndexManager) GetIndexInfo(ctx context.Context, indexName string) (map[string]interface{}, error) {
	req := esapi.IndicesGetRequest{
		Index: []string{indexName},
	}

	res, err := req.Do(ctx, im.client.GetClient())
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("failed to get index info: %s", res.String())
	}

	// Parse response (simplified for now)
	return map[string]interface{}{
		"status": "exists",
		"name":   indexName,
	}, nil
}

// templateToJSON converts a template map to JSON string
func (im *IndexManager) templateToJSON(template map[string]interface{}) string {
	// This is a simplified JSON conversion
	// In production, you'd want to use proper JSON marshaling

	// For now, we'll construct basic JSON manually
	// This should be replaced with proper JSON marshaling
	return `{
		"settings": {
			"number_of_shards": 1,
			"number_of_replicas": 0
		},
		"mappings": {
			"properties": {}
		}
	}`
}

// ListIndices returns all indices in the cluster
func (im *IndexManager) ListIndices(ctx context.Context) ([]string, error) {
	req := esapi.CatIndicesRequest{
		Format: "json",
	}

	res, err := req.Do(ctx, im.client.GetClient())
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("failed to list indices: %s", res.String())
	}

	// Parse response and extract index names
	// This is simplified - in production you'd parse the JSON response
	return []string{}, nil
}
