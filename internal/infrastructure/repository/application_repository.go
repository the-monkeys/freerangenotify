package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"go.uber.org/zap"
)

// ApplicationRepository implements the application domain repository interface
type ApplicationRepository struct {
	*BaseRepository
}

// NewApplicationRepository creates a new application repository
func NewApplicationRepository(client *elasticsearch.Client, logger *zap.Logger) application.Repository {
	return &ApplicationRepository{
		BaseRepository: NewBaseRepository(client, "applications", logger),
	}
}

// Create creates a new application
func (r *ApplicationRepository) Create(ctx context.Context, app *application.Application) error {
	app.CreatedAt = time.Now()
	app.UpdatedAt = time.Now()
	return r.BaseRepository.Create(ctx, app.AppID, app)
}

// GetByID retrieves an application by ID
func (r *ApplicationRepository) GetByID(ctx context.Context, appID string) (*application.Application, error) {
	doc, err := r.BaseRepository.GetByID(ctx, appID)
	if err != nil {
		return nil, err
	}

	var app application.Application
	if err := mapToStruct(doc, &app); err != nil {
		return nil, fmt.Errorf("failed to map document to application: %w", err)
	}

	return &app, nil
}

// GetByAPIKey retrieves an application by API key
func (r *ApplicationRepository) GetByAPIKey(ctx context.Context, apiKey string) (*application.Application, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"api_key": apiKey,
			},
		},
	}

	result, err := r.BaseRepository.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	if result.Total == 0 {
		return nil, fmt.Errorf("application not found")
	}

	var app application.Application
	if err := mapToStruct(result.Hits[0], &app); err != nil {
		return nil, fmt.Errorf("failed to map document to application: %w", err)
	}

	return &app, nil
}

// Update updates an existing application
func (r *ApplicationRepository) Update(ctx context.Context, app *application.Application) error {
	app.UpdatedAt = time.Now()
	return r.BaseRepository.Update(ctx, app.AppID, app)
}

// Delete deletes an application
func (r *ApplicationRepository) Delete(ctx context.Context, appID string) error {
	return r.BaseRepository.Delete(ctx, appID)
}

// List lists applications with pagination
func (r *ApplicationRepository) List(ctx context.Context, filter application.ApplicationFilter) ([]*application.Application, error) {
	query := r.buildApplicationQuery(filter)

	result, err := r.BaseRepository.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	var applications []*application.Application
	for _, hit := range result.Hits {
		var app application.Application
		if err := mapToStruct(hit, &app); err != nil {
			r.logger.Error("Failed to map document to application", zap.Error(err))
			continue
		}
		applications = append(applications, &app)
	}

	return applications, nil
}

// buildApplicationQuery builds Elasticsearch query from filter
func (r *ApplicationRepository) buildApplicationQuery(filter application.ApplicationFilter) map[string]interface{} {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
	}

	var filters []map[string]interface{}

	if filter.AppName != "" {
		filters = append(filters, map[string]interface{}{
			"match": map[string]interface{}{
				"app_name": filter.AppName,
			},
		})
	}

	if len(filters) > 0 {
		query["query"] = map[string]interface{}{
			"bool": map[string]interface{}{
				"must": filters,
			},
		}
	}

	// Add pagination
	if filter.Offset > 0 {
		query["from"] = filter.Offset
	}
	if filter.Limit > 0 {
		query["size"] = filter.Limit
	}

	// Add sorting
	query["sort"] = []map[string]interface{}{
		{
			"created_at": map[string]interface{}{
				"order": "desc",
			},
		},
	}

	return query
}

// RegenerateAPIKey regenerates the API key for an application
func (r *ApplicationRepository) RegenerateAPIKey(ctx context.Context, appID string) (string, error) {
	// This would typically generate a new API key
	// For now, we'll create a simple implementation
	newAPIKey := fmt.Sprintf("frn_%d", time.Now().Unix())

	updateDoc := map[string]interface{}{
		"api_key":    newAPIKey,
		"updated_at": time.Now(),
	}

	if err := r.BaseRepository.Update(ctx, appID, updateDoc); err != nil {
		return "", err
	}

	return newAPIKey, nil
}
