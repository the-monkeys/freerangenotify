package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"go.uber.org/zap"
)

// BaseRepository provides common Elasticsearch operations
type BaseRepository struct {
	client    *elasticsearch.Client
	indexName string
	logger    *zap.Logger
}

// NewBaseRepository creates a new base repository
func NewBaseRepository(client *elasticsearch.Client, indexName string, logger *zap.Logger) *BaseRepository {
	return &BaseRepository{
		client:    client,
		indexName: indexName,
		logger:    logger,
	}
}

// QueryResult represents a search result
type QueryResult struct {
	Total int64                    `json:"total"`
	Hits  []map[string]interface{} `json:"hits"`
}

// Create creates a new document
func (r *BaseRepository) Create(ctx context.Context, id string, document interface{}) error {
	data, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("failed to marshal document: %w", err)
	}

	req := esapi.IndexRequest{
		Index:      r.indexName,
		DocumentID: id,
		Body:       strings.NewReader(string(data)),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("failed to create document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("elasticsearch error: %s", res.String())
	}

	r.logger.Debug("Document created",
		zap.String("index", r.indexName),
		zap.String("id", id))

	return nil
}

// GetByID retrieves a document by its ID
func (r *BaseRepository) GetByID(ctx context.Context, id string) (map[string]interface{}, error) {
	req := esapi.GetRequest{
		Index:      r.indexName,
		DocumentID: id,
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, fmt.Errorf("document not found")
	}

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch error: %s", res.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract the source document
	if source, exists := result["_source"]; exists {
		if sourceMap, ok := source.(map[string]interface{}); ok {
			return sourceMap, nil
		}
	}

	return result, nil
}

// Update updates an existing document
func (r *BaseRepository) Update(ctx context.Context, id string, document interface{}) error {
	data, err := json.Marshal(map[string]interface{}{
		"doc": document,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal document: %w", err)
	}

	req := esapi.UpdateRequest{
		Index:      r.indexName,
		DocumentID: id,
		Body:       strings.NewReader(string(data)),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("elasticsearch error: %s", res.String())
	}

	r.logger.Debug("Document updated",
		zap.String("index", r.indexName),
		zap.String("id", id))

	return nil
}

// Delete deletes a document by its ID
func (r *BaseRepository) Delete(ctx context.Context, id string) error {
	req := esapi.DeleteRequest{
		Index:      r.indexName,
		DocumentID: id,
		Refresh:    "true",
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return fmt.Errorf("document not found")
	}

	if res.IsError() {
		return fmt.Errorf("elasticsearch error: %s", res.String())
	}

	r.logger.Debug("Document deleted",
		zap.String("index", r.indexName),
		zap.String("id", id))

	return nil
}

// DeleteByQuery deletes multiple documents matching a query
func (r *BaseRepository) DeleteByQuery(ctx context.Context, query map[string]interface{}) error {
	data, err := json.Marshal(query)
	if err != nil {
		return fmt.Errorf("failed to marshal query: %w", err)
	}

	refresh := true
	req := esapi.DeleteByQueryRequest{
		Index:   []string{r.indexName},
		Body:    strings.NewReader(string(data)),
		Refresh: &refresh,
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("failed to execute delete by query: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("elasticsearch error: %s", res.String())
	}

	r.logger.Debug("Delete by query completed",
		zap.String("index", r.indexName))

	return nil
}

// Search performs a search query
func (r *BaseRepository) Search(ctx context.Context, query map[string]interface{}) (*QueryResult, error) {
	data, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	req := esapi.SearchRequest{
		Index: []string{r.indexName},
		Body:  strings.NewReader(string(data)),
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch error: %s", res.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract hits
	hits, ok := result["hits"].(map[string]interface{})
	if !ok {
		return &QueryResult{}, nil
	}

	total := int64(0)
	if totalObj, exists := hits["total"]; exists {
		if totalMap, ok := totalObj.(map[string]interface{}); ok {
			if value, exists := totalMap["value"]; exists {
				if totalFloat, ok := value.(float64); ok {
					total = int64(totalFloat)
				}
			}
		}
	}

	var documents []map[string]interface{}
	if hitsList, exists := hits["hits"]; exists {
		if hitsArray, ok := hitsList.([]interface{}); ok {
			for _, hit := range hitsArray {
				if hitMap, ok := hit.(map[string]interface{}); ok {
					if source, exists := hitMap["_source"]; exists {
						if sourceMap, ok := source.(map[string]interface{}); ok {
							documents = append(documents, sourceMap)
						}
					}
				}
			}
		}
	}

	return &QueryResult{
		Total: total,
		Hits:  documents,
	}, nil
}

// BulkUpdate updates multiple documents in a single request (partial update)
func (r *BaseRepository) BulkUpdate(ctx context.Context, documents map[string]interface{}) error {
	var body strings.Builder

	for id, document := range documents {
		// Add update action
		action := map[string]interface{}{
			"update": map[string]interface{}{
				"_index": r.indexName,
				"_id":    id,
			},
		}
		actionData, _ := json.Marshal(action)
		body.Write(actionData)
		body.WriteString("\n")

		// Add document wrapper for update
		updateDoc := map[string]interface{}{
			"doc": document,
		}
		docData, _ := json.Marshal(updateDoc)
		body.Write(docData)
		body.WriteString("\n")
	}

	req := esapi.BulkRequest{
		Body:    strings.NewReader(body.String()),
		Refresh: "true",
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("failed to execute bulk update request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("elasticsearch error: %s", res.String())
	}

	r.logger.Debug("Bulk update completed",
		zap.String("index", r.indexName),
		zap.Int("count", len(documents)))

	return nil
}

// BulkCreate creates multiple documents in a single request
func (r *BaseRepository) BulkCreate(ctx context.Context, documents map[string]interface{}) error {
	var body strings.Builder

	for id, document := range documents {
		// Add index action
		action := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": r.indexName,
				"_id":    id,
			},
		}
		actionData, _ := json.Marshal(action)
		body.Write(actionData)
		body.WriteString("\n")

		// Add document
		docData, _ := json.Marshal(document)
		body.Write(docData)
		body.WriteString("\n")
	}

	req := esapi.BulkRequest{
		Body:    strings.NewReader(body.String()),
		Refresh: "true",
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("failed to execute bulk request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("elasticsearch error: %s", res.String())
	}

	r.logger.Debug("Bulk create completed",
		zap.String("index", r.indexName),
		zap.Int("count", len(documents)))

	return nil
}

// Count returns the number of documents matching a query
func (r *BaseRepository) Count(ctx context.Context, query map[string]interface{}) (int64, error) {
	data, err := json.Marshal(query)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal query: %w", err)
	}

	req := esapi.CountRequest{
		Index: []string{r.indexName},
		Body:  strings.NewReader(string(data)),
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return 0, fmt.Errorf("failed to execute count: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return 0, fmt.Errorf("elasticsearch error: %s", res.String())
	}

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	if count, exists := result["count"]; exists {
		if countFloat, ok := count.(float64); ok {
			return int64(countFloat), nil
		}
	}

	return 0, nil
}

// Exists checks if a document exists
func (r *BaseRepository) Exists(ctx context.Context, id string) (bool, error) {
	req := esapi.ExistsRequest{
		Index:      r.indexName,
		DocumentID: id,
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return false, fmt.Errorf("failed to check existence: %w", err)
	}
	defer res.Body.Close()

	return res.StatusCode == 200, nil
}
