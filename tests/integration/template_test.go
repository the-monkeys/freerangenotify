//go:build skip
// +build skip

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"go.uber.org/zap"
)

func setupTemplateTest(t *testing.T) (*usecases.TemplateService, *database.TemplateRepository, context.Context) {
	logger := zap.NewNop()
	ctx := context.Background()

	// Create Elasticsearch client for testing
	client := createTestElasticsearchClient(t)

	// Create template repository
	repo := database.NewTemplateRepository(client, logger)

	// Create templates index
	err := repo.CreateIndex(ctx)
	require.NoError(t, err, "Failed to create templates index")

	// Create template service
	service := usecases.NewTemplateService(repo, logger)

	return service, repo, ctx
}

func TestTemplate_CreateAndGet(t *testing.T) {
	service, repo, ctx := setupTemplateTest(t)
	defer cleanupTemplateIndex(t, repo, ctx)

	// Create template
	createReq := &template.CreateRequest{
		AppID:       "app-001",
		Name:        "welcome-email",
		Description: "Welcome email template",
		Channel:     "email",
		Subject:     "Welcome to {{.AppName}}!",
		Body:        "Hello {{.UserName}}, welcome to our app!",
		Variables:   []string{"AppName", "UserName"},
		Locale:      "en-US",
		CreatedBy:   "admin",
	}

	tmpl, err := service.Create(ctx, createReq)
	require.NoError(t, err)
	assert.NotEmpty(t, tmpl.ID)
	assert.Equal(t, createReq.Name, tmpl.Name)
	assert.Equal(t, 1, tmpl.Version)
	assert.Equal(t, "active", tmpl.Status)

	// Get template by ID
	retrieved, err := service.GetByID(ctx, tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, tmpl.ID, retrieved.ID)
	assert.Equal(t, tmpl.Name, retrieved.Name)
	assert.Equal(t, tmpl.Body, retrieved.Body)
}

func TestTemplate_GetByName(t *testing.T) {
	service, repo, ctx := setupTemplateTest(t)
	defer cleanupTemplateIndex(t, repo, ctx)

	// Create template
	createReq := &template.CreateRequest{
		AppID:     "app-001",
		Name:      "password-reset",
		Channel:   "email",
		Subject:   "Password Reset",
		Body:      "Click here to reset: {{.ResetLink}}",
		Variables: []string{"ResetLink"},
		Locale:    "en-US",
		CreatedBy: "admin",
	}

	tmpl, err := service.Create(ctx, createReq)
	require.NoError(t, err)

	// Get by name
	retrieved, err := service.GetByName(ctx, "app-001", "password-reset", "en-US")
	require.NoError(t, err)
	assert.Equal(t, tmpl.ID, retrieved.ID)
	assert.Equal(t, "password-reset", retrieved.Name)
}

func TestTemplate_Update(t *testing.T) {
	service, repo, ctx := setupTemplateTest(t)
	defer cleanupTemplateIndex(t, repo, ctx)

	// Create template
	createReq := &template.CreateRequest{
		AppID:     "app-001",
		Name:      "notification",
		Channel:   "push",
		Body:      "You have {{.Count}} new messages",
		Variables: []string{"Count"},
		Locale:    "en-US",
		CreatedBy: "admin",
	}

	tmpl, err := service.Create(ctx, createReq)
	require.NoError(t, err)

	// Update template
	desc := "Updated description"
	body := "You have {{.Count}} new messages. Check them now!"
	updateReq := template.UpdateRequest{
		Description: &desc,
		Body:        &body,
		UpdatedBy:   "admin",
	}

	updated, err := service.Update(ctx, tmpl.ID, updateReq)
	require.NoError(t, err)
	assert.Equal(t, *updateReq.Description, updated.Description)
	assert.Equal(t, *updateReq.Body, updated.Body)
	assert.True(t, updated.UpdatedAt.After(updated.CreatedAt))
}

func TestTemplate_Delete(t *testing.T) {
	service, repo, ctx := setupTemplateTest(t)
	defer cleanupTemplateIndex(t, repo, ctx)

	// Create template
	createReq := &template.CreateRequest{
		AppID:     "app-001",
		Name:      "delete-test",
		Channel:   "email",
		Body:      "Test body",
		Locale:    "en-US",
		CreatedBy: "admin",
	}

	tmpl, err := service.Create(ctx, createReq)
	require.NoError(t, err)

	// Delete template (soft delete)
	err = service.Delete(ctx, tmpl.ID)
	require.NoError(t, err)

	// Verify template is archived
	retrieved, err := service.GetByID(ctx, tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, "archived", retrieved.Status)
}

func TestTemplate_List(t *testing.T) {
	service, repo, ctx := setupTemplateTest(t)
	defer cleanupTemplateIndex(t, repo, ctx)

	// Create multiple templates
	templates := []template.CreateRequest{
		{
			AppID:     "app-001",
			Name:      "template-1",
			Channel:   "email",
			Body:      "Body 1",
			Status:    "active",
			CreatedBy: "admin",
		},
		{
			AppID:     "app-001",
			Name:      "template-2",
			Channel:   "push",
			Body:      "Body 2",
			Status:    "active",
			CreatedBy: "admin",
		},
		{
			AppID:     "app-002",
			Name:      "template-3",
			Channel:   "email",
			Body:      "Body 3",
			Status:    "active",
			CreatedBy: "admin",
		},
	}

	for _, req := range templates {
		_, err := service.Create(ctx, req)
		require.NoError(t, err)
	}

	// Wait for Elasticsearch to index
	time.Sleep(1 * time.Second)

	// List all templates for app-001
	filter := template.Filter{
		AppID: "app-001",
		Limit: 10,
	}

	results, err := service.List(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	// List email templates only
	filter.Channel = notification.ChannelEmail
	results, err = service.List(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "template-1", results[0].Name)
}

func TestTemplate_Render(t *testing.T) {
	service, repo, ctx := setupTemplateTest(t)
	defer cleanupTemplateIndex(t, repo, ctx)

	// Create template with variables
	createReq := template.CreateRequest{
		AppID:     "app-001",
		Name:      "greeting",
		Channel:   "email",
		Body:      "Hello {{.Name}}, you have {{.Count}} new messages!",
		Variables: []string{"Name", "Count"},
		CreatedBy: "admin",
	}

	tmpl, err := service.Create(ctx, createReq)
	require.NoError(t, err)

	// Render template
	data := map[string]interface{}{
		"Name":  "John",
		"Count": 5,
	}

	rendered, err := service.Render(ctx, tmpl.ID, data)
	require.NoError(t, err)
	assert.Equal(t, "Hello John, you have 5 new messages!", rendered)
}

func TestTemplate_RenderInactiveTemplate(t *testing.T) {
	service, repo, ctx := setupTemplateTest(t)
	defer cleanupTemplateIndex(t, repo, ctx)

	// Create and deactivate template
	createReq := template.CreateRequest{
		AppID:     "app-001",
		Name:      "inactive-test",
		Channel:   "email",
		Body:      "Test {{.Value}}",
		Variables: []string{"Value"},
		CreatedBy: "admin",
	}

	tmpl, err := service.Create(ctx, createReq)
	require.NoError(t, err)

	// Deactivate
	updateReq := template.UpdateRequest{
		Status:    "inactive",
		UpdatedBy: "admin",
	}
	_, err = service.Update(ctx, tmpl.ID, updateReq)
	require.NoError(t, err)

	// Try to render inactive template
	data := map[string]interface{}{"Value": "test"}
	_, err = service.Render(ctx, tmpl.ID, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not active")
}

func TestTemplate_CreateVersion(t *testing.T) {
	service, repo, ctx := setupTemplateTest(t)
	defer cleanupTemplateIndex(t, repo, ctx)

	// Create initial template
	createReq := template.CreateRequest{
		AppID:     "app-001",
		Name:      "versioned-template",
		Channel:   "email",
		Subject:   "Version 1",
		Body:      "This is version 1",
		CreatedBy: "admin",
	}

	v1, err := service.Create(ctx, createReq)
	require.NoError(t, err)
	assert.Equal(t, 1, v1.Version)
	assert.Equal(t, template.StatusActive, v1.Status)

	// Wait for indexing
	time.Sleep(1 * time.Second)

	// Create version 2
	v2Req := template.CreateRequest{
		Channel:   "email",
		Subject:   "Version 2",
		Body:      "This is version 2 with improvements",
		CreatedBy: "admin",
	}

	v2, err := service.CreateVersion(ctx, "app-001", "versioned-template", v2Req)
	require.NoError(t, err)
	assert.Equal(t, 2, v2.Version)
	assert.Equal(t, template.StatusActive, v2.Status)
	assert.NotEqual(t, v1.ID, v2.ID) // Different IDs

	// Wait for indexing
	time.Sleep(1 * time.Second)

	// Verify v1 is now inactive
	v1Retrieved, err := service.GetByID(ctx, v1.ID)
	require.NoError(t, err)
	assert.Equal(t, template.StatusInactive, v1Retrieved.Status)

	// GetByName should return latest active version (v2)
	latest, err := service.GetByName(ctx, "app-001", "versioned-template")
	require.NoError(t, err)
	assert.Equal(t, v2.ID, latest.ID)
	assert.Equal(t, 2, latest.Version)
}

func TestTemplate_GetVersions(t *testing.T) {
	service, repo, ctx := setupTemplateTest(t)
	defer cleanupTemplateIndex(t, repo, ctx)

	// Create template with multiple versions
	createReq := template.CreateRequest{
		AppID:     "app-001",
		Name:      "multi-version",
		Channel:   "push",
		Body:      "Version 1",
		CreatedBy: "admin",
	}

	_, err := service.Create(ctx, createReq)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	// Create version 2
	v2Req := template.CreateRequest{
		Channel:   "push",
		Body:      "Version 2",
		CreatedBy: "admin",
	}
	_, err = service.CreateVersion(ctx, "app-001", "multi-version", v2Req)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	// Create version 3
	v3Req := template.CreateRequest{
		Channel:   "push",
		Body:      "Version 3",
		CreatedBy: "admin",
	}
	_, err = service.CreateVersion(ctx, "app-001", "multi-version", v3Req)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	// Get all versions
	versions, err := service.GetVersions(ctx, "app-001", "multi-version")
	require.NoError(t, err)
	assert.Len(t, versions, 3)

	// Verify versions are in descending order
	assert.Equal(t, 3, versions[0].Version)
	assert.Equal(t, 2, versions[1].Version)
	assert.Equal(t, 1, versions[2].Version)

	// Verify only latest is active
	assert.Equal(t, template.StatusActive, versions[0].Status)
	assert.Equal(t, template.StatusInactive, versions[1].Status)
	assert.Equal(t, template.StatusInactive, versions[2].Status)
}

func TestTemplate_MultiLanguage(t *testing.T) {
	service, repo, ctx := setupTemplateTest(t)
	defer cleanupTemplateIndex(t, repo, ctx)

	// Create English template
	enReq := template.CreateRequest{
		AppID:     "app-001",
		Name:      "welcome",
		Channel:   "email",
		Subject:   "Welcome",
		Body:      "Welcome {{.Name}}!",
		Variables: []string{"Name"},
		Locale:    "en-US",
		CreatedBy: "admin",
	}

	enTmpl, err := service.Create(ctx, enReq)
	require.NoError(t, err)

	// Create Spanish template (same name, different locale)
	esReq := template.CreateRequest{
		AppID:     "app-001",
		Name:      "welcome",
		Channel:   "email",
		Subject:   "Bienvenido",
		Body:      "¡Bienvenido {{.Name}}!",
		Variables: []string{"Name"},
		Locale:    "es-ES",
		CreatedBy: "admin",
	}

	esTmpl, err := service.Create(ctx, esReq)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)

	// List templates with locale filter
	filter := template.Filter{
		AppID:  "app-001",
		Name:   "welcome",
		Locale: "es-ES",
		Limit:  10,
	}

	results, err := service.List(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, esTmpl.ID, results[0].ID)
	assert.Equal(t, "es-ES", results[0].Locale)

	// Render both templates
	data := map[string]interface{}{"Name": "Maria"}

	enRendered, err := service.Render(ctx, enTmpl.ID, data)
	require.NoError(t, err)
	assert.Equal(t, "Welcome Maria!", enRendered)

	esRendered, err := service.Render(ctx, esTmpl.ID, data)
	require.NoError(t, err)
	assert.Equal(t, "¡Bienvenido Maria!", esRendered)
}

func TestTemplate_ValidationErrors(t *testing.T) {
	service, repo, ctx := setupTemplateTest(t)
	defer cleanupTemplateIndex(t, repo, ctx)

	t.Run("Invalid channel", func(t *testing.T) {
		req := template.CreateRequest{
			AppID:     "app-001",
			Name:      "test",
			Channel:   "invalid-channel",
			Body:      "Test",
			CreatedBy: "admin",
		}

		_, err := service.Create(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid channel")
	})

	t.Run("Undefined variables in template", func(t *testing.T) {
		req := template.CreateRequest{
			AppID:     "app-001",
			Name:      "test",
			Channel:   "email",
			Body:      "Hello {{.Name}}, your order {{.OrderID}} is ready",
			Variables: []string{"Name"}, // Missing OrderID
			CreatedBy: "admin",
		}

		_, err := service.Create(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "undefined variables")
	})

	t.Run("Duplicate template name", func(t *testing.T) {
		req := template.CreateRequest{
			AppID:     "app-001",
			Name:      "duplicate",
			Channel:   "email",
			Body:      "Test",
			CreatedBy: "admin",
		}

		_, err := service.Create(ctx, req)
		require.NoError(t, err)

		// Try to create again
		_, err = service.Create(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})
}

func cleanupTemplateIndex(t *testing.T, repo *database.TemplateRepository, ctx context.Context) {
	// Delete all test templates
	filter := template.Filter{
		Limit: 100,
	}

	templates, err := repo.List(ctx, filter)
	if err != nil {
		t.Logf("Failed to list templates for cleanup: %v", err)
		return
	}

	for _, tmpl := range templates {
		if err := repo.Delete(ctx, tmpl.ID); err != nil {
			t.Logf("Failed to delete template %s: %v", tmpl.ID, err)
		}
	}

	t.Logf("Cleaned up %d templates", len(templates))
}

func createTestElasticsearchClient(t *testing.T) *database.ElasticsearchClient {
	logger := zap.NewNop()

	// Use test configuration
	cfg := &struct {
		Elasticsearch struct {
			Addresses []string
			Username  string
			Password  string
		}
	}{}
	cfg.Elasticsearch.Addresses = []string{"http://localhost:9200"}
	cfg.Elasticsearch.Username = ""
	cfg.Elasticsearch.Password = ""

	client := &database.ElasticsearchClient{}
	esClient, err := database.NewElasticsearchClient(
		&struct {
			Elasticsearch struct {
				Addresses []string
				Username  string
				Password  string
			}
		}{
			Elasticsearch: struct {
				Addresses []string
				Username  string
				Password  string
			}{
				Addresses: []string{"http://localhost:9200"},
			},
		},
		logger,
	)

	require.NoError(t, err, "Failed to create Elasticsearch client")

	// Test connection
	ctx := context.Background()
	_, err = esClient.Health(ctx)
	if err != nil {
		t.Skip(fmt.Sprintf("Skipping test: Elasticsearch not available: %v", err))
	}

	return esClient
}
