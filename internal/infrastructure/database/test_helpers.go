package database

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/stretchr/testify/require"
)

// SetupTestElasticsearch creates a test Elasticsearch client and returns a cleanup function
func SetupTestElasticsearch(t *testing.T) (*elasticsearch.Client, func()) {
	// Connect to test Elasticsearch (assumes localhost:9200)
	cfg := elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	}

	client, err := elasticsearch.NewClient(cfg)
	require.NoError(t, err, "Failed to create test Elasticsearch client")

	// Verify connection
	res, err := client.Ping()
	require.NoError(t, err, "Failed to ping test Elasticsearch")
	res.Body.Close()

	cleanup := func() {
		// Clean up test indices
		ctx := context.Background()
		indices := []string{
			"frn_test_notifications",
			"frn_test_users",
			"frn_test_applications",
		}

		for _, index := range indices {
			client.Indices.Delete([]string{index}, client.Indices.Delete.WithContext(ctx))
		}
	}

	return client, cleanup
}

// CreateNotificationIndex creates the notification index for testing
func CreateNotificationIndex(client *elasticsearch.Client) error {
	ctx := context.Background()
	indexName := "frn_test_notifications"

	// Delete if exists
	client.Indices.Delete([]string{indexName}, client.Indices.Delete.WithContext(ctx))

	// Get the notifications template from index_templates
	templates := &IndexTemplates{}
	template := templates.GetNotificationsTemplate()

	// Marshal template to JSON
	body, err := json.Marshal(template)
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}

	// Create index with template settings
	res, err := client.Indices.Create(
		indexName,
		client.Indices.Create.WithBody(bytes.NewReader(body)),
		client.Indices.Create.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to create notification index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error creating notification index: %s", res.String())
	}

	return nil
}

// CreateUserIndex creates the user index for testing
func CreateUserIndex(client *elasticsearch.Client) error {
	ctx := context.Background()
	indexName := "frn_test_users"

	// Delete if exists
	client.Indices.Delete([]string{indexName}, client.Indices.Delete.WithContext(ctx))

	// Get the users template from index_templates
	templates := &IndexTemplates{}
	template := templates.GetUsersTemplate()

	// Marshal template to JSON
	body, err := json.Marshal(template)
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}

	// Create index with template settings
	res, err := client.Indices.Create(
		indexName,
		client.Indices.Create.WithBody(bytes.NewReader(body)),
		client.Indices.Create.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to create user index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error creating user index: %s", res.String())
	}

	return nil
}
