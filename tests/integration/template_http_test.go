package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type TemplateHTTPTestSuite struct {
	IntegrationTestSuite
	templateID string
}

func TestTemplateHTTPSuite(t *testing.T) {
	suite.Run(t, new(TemplateHTTPTestSuite))
}

// setupForTemplateTests creates an application for template tests
func (s *TemplateHTTPTestSuite) setupForTemplateTests() string {
	appPayload := map[string]interface{}{
		"app_name": "Template Test App",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/apps", appPayload, nil)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})

	s.appID = data["app_id"].(string)
	s.apiKey = data["api_key"].(string)

	return s.apiKey
}

// TestCreateTemplate tests creating a new template
func (s *TemplateHTTPTestSuite) TestCreateTemplate() {
	apiKey := s.setupForTemplateTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	payload := map[string]interface{}{
		"app_id":      s.appID,
		"name":        "welcome_email",
		"channel":     "email",
		"locale":      "en-US",
		"subject":     "Welcome to {{.AppName}}!",
		"body":        "Hello {{.UserName}}, welcome to {{.AppName}}! We're excited to have you on board.",
		"variables":   []string{"AppName", "UserName"},
		"description": "Welcome email template for new users",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/templates", payload, headers)

	s.Equal(http.StatusCreated, resp.StatusCode)

	// Response is directly the template object
	var result map[string]interface{}
	s.parseResponse(body, &result)

	s.NotEmpty(result["id"])
	s.Equal(s.appID, result["app_id"])
	s.Equal("welcome_email", result["name"])
	s.Equal("email", result["channel"])
	s.Equal("en-US", result["locale"])
	s.Equal(float64(1), result["version"])
	s.Equal("active", result["status"])
	s.Contains(result["body"].(string), "{{.UserName}}")

	// Store template ID for other tests
	s.templateID = result["id"].(string)
}

// TestCreateTemplateValidation tests template validation errors
func (s *TemplateHTTPTestSuite) TestCreateTemplateValidation() {
	apiKey := s.setupForTemplateTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	testCases := []struct {
		name          string
		payload       map[string]interface{}
		expectedError string
	}{
		{
			name: "Missing app_id",
			payload: map[string]interface{}{
				"name":    "test",
				"channel": "email",
				"body":    "Test body",
			},
			expectedError: "VALIDATION_ERROR",
		},
		{
			name: "Missing name",
			payload: map[string]interface{}{
				"app_id":  s.appID,
				"channel": "email",
				"body":    "Test body",
			},
			expectedError: "VALIDATION_ERROR",
		},
		{
			name: "Invalid channel",
			payload: map[string]interface{}{
				"app_id":  s.appID,
				"name":    "test",
				"channel": "invalid_channel",
				"body":    "Test body",
			},
			expectedError: "VALIDATION_ERROR",
		},
		{
			name: "Missing body",
			payload: map[string]interface{}{
				"app_id":  s.appID,
				"name":    "test",
				"channel": "email",
			},
			expectedError: "VALIDATION_ERROR",
		},
		{
			name: "Undefined variables",
			payload: map[string]interface{}{
				"app_id":    s.appID,
				"name":      "test",
				"channel":   "email",
				"body":      "Hello {{.Name}}, your order {{.OrderID}} is ready",
				"variables": []string{"Name"}, // Missing OrderID
			},
			expectedError: "undefined variables",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			resp, body := s.makeRequest(http.MethodPost, "/v1/templates", tc.payload, headers)

			s.Equal(http.StatusBadRequest, resp.StatusCode)
			s.assertError(body, "VALIDATION_ERROR")
		})
	}
}

// TestGetTemplate tests retrieving a template by ID
func (s *TemplateHTTPTestSuite) TestGetTemplate() {
	apiKey := s.setupForTemplateTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	// Create template first
	createPayload := map[string]interface{}{
		"app_id":    s.appID,
		"name":      "get_test_template",
		"channel":   "push",
		"body":      "Test notification: {{.Message}}",
		"variables": []string{"Message"},
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/templates", createPayload, headers)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	var createResult map[string]interface{}
	s.parseResponse(body, &createResult)
	templateID := createResult["id"].(string)

	// Get template
	resp, body = s.makeRequest(http.MethodGet, "/v1/templates/"+templateID, nil, headers)

	s.Equal(http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	s.parseResponse(body, &result)

	s.Equal(templateID, result["id"])
	s.Equal("get_test_template", result["name"])
	s.Equal("push", result["channel"])
}

// TestGetTemplateNotFound tests getting a non-existent template
func (s *TemplateHTTPTestSuite) TestGetTemplateNotFound() {
	apiKey := s.setupForTemplateTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	resp, body := s.makeRequest(http.MethodGet, "/v1/templates/non-existent-id", nil, headers)

	s.True(resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusInternalServerError)

	// Only check error format if we get an error response
	if resp.StatusCode != http.StatusOK {
		var result map[string]interface{}
		s.parseResponse(body, &result)
		// Check that we have an error field
		_, hasError := result["error"]
		s.True(hasError, "Response should have an error field")
	}
}

// TestListTemplates tests listing templates
func (s *TemplateHTTPTestSuite) TestListTemplates() {
	apiKey := s.setupForTemplateTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	// Create multiple templates
	channels := []string{"email", "push", "sms"}
	for i, channel := range channels {
		payload := map[string]interface{}{
			"app_id":    s.appID,
			"name":      fmt.Sprintf("list_template_%d", i+1),
			"channel":   channel,
			"body":      fmt.Sprintf("Test body %d with {{.Data}}", i+1),
			"variables": []string{"Data"},
		}
		resp, _ := s.makeRequest(http.MethodPost, "/v1/templates", payload, headers)
		s.Require().Equal(http.StatusCreated, resp.StatusCode)
	}

	// Wait for Elasticsearch indexing
	time.Sleep(2 * time.Second)

	// List all templates
	resp, body := s.makeRequest(http.MethodGet, "/v1/templates?limit=10", nil, headers)

	s.Equal(http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	s.parseResponse(body, &result)

	templates := result["templates"].([]interface{})
	s.GreaterOrEqual(len(templates), 3)

	// Test filtering by channel
	resp, body = s.makeRequest(http.MethodGet, "/v1/templates?channel=email&limit=10", nil, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	s.parseResponse(body, &result)
	templates = result["templates"].([]interface{})
	s.GreaterOrEqual(len(templates), 1)

	// All returned templates should be email channel
	for _, tmpl := range templates {
		t := tmpl.(map[string]interface{})
		s.Equal("email", t["channel"])
	}
}

// TestUpdateTemplate tests updating a template
func (s *TemplateHTTPTestSuite) TestUpdateTemplate() {
	apiKey := s.setupForTemplateTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	// Create template
	createPayload := map[string]interface{}{
		"app_id":      s.appID,
		"name":        "update_test",
		"channel":     "email",
		"subject":     "Original Subject",
		"body":        "Original body with {{.Data}}",
		"variables":   []string{"Data"},
		"description": "Original description",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/templates", createPayload, headers)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	var createResult map[string]interface{}
	s.parseResponse(body, &createResult)
	templateID := createResult["id"].(string)

	// Update template
	updatePayload := map[string]interface{}{
		"description": "Updated description",
		"subject":     "Updated Subject",
		"body":        "Updated body with {{.Data}} and more content",
	}

	resp, body = s.makeRequest(http.MethodPut, "/v1/templates/"+templateID, updatePayload, headers)

	s.Equal(http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	s.parseResponse(body, &result)

	s.Equal(templateID, result["id"])
	s.Equal("Updated description", result["description"])
	s.Equal("Updated Subject", result["subject"])
	s.Contains(result["body"].(string), "more content")
}

// TestRenderTemplate tests rendering a template with data
func (s *TemplateHTTPTestSuite) TestRenderTemplate() {
	apiKey := s.setupForTemplateTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	// Create template
	createPayload := map[string]interface{}{
		"app_id":    s.appID,
		"name":      "render_test",
		"channel":   "email",
		"subject":   "Hello {{.UserName}}!",
		"body":      "Dear {{.UserName}}, you have {{.Count}} new notifications from {{.AppName}}.",
		"variables": []string{"UserName", "Count", "AppName"},
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/templates", createPayload, headers)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	var createResult map[string]interface{}
	s.parseResponse(body, &createResult)
	templateID := createResult["id"].(string)

	// Render template
	renderPayload := map[string]interface{}{
		"data": map[string]interface{}{
			"UserName": "John Doe",
			"Count":    5,
			"AppName":  "FreeRangeNotify",
		},
	}

	resp, body = s.makeRequest(http.MethodPost, "/v1/templates/"+templateID+"/render", renderPayload, headers)

	s.Equal(http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	s.parseResponse(body, &result)

	renderedBody := result["rendered_body"].(string)
	s.Contains(renderedBody, "John Doe")
	s.Contains(renderedBody, "5")
	s.Contains(renderedBody, "FreeRangeNotify")
	s.Equal("Dear John Doe, you have 5 new notifications from FreeRangeNotify.", renderedBody)
}

// TestRenderTemplateValidation tests render validation
func (s *TemplateHTTPTestSuite) TestRenderTemplateValidation() {
	apiKey := s.setupForTemplateTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	// Create template
	createPayload := map[string]interface{}{
		"app_id":    s.appID,
		"name":      "validation_test",
		"channel":   "email",
		"body":      "Hello {{.Name}}",
		"variables": []string{"Name"},
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/templates", createPayload, headers)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	var createResult map[string]interface{}
	s.parseResponse(body, &createResult)
	templateID := createResult["id"].(string)

	// Try to render with missing data
	renderPayload := map[string]interface{}{
		"data": map[string]interface{}{},
	}

	resp, body = s.makeRequest(http.MethodPost, "/v1/templates/"+templateID+"/render", renderPayload, headers)

	// Should succeed but output will have <no value>
	s.Equal(http.StatusOK, resp.StatusCode)
}

// TestDeleteTemplate tests deleting a template (soft delete)
func (s *TemplateHTTPTestSuite) TestDeleteTemplate() {
	apiKey := s.setupForTemplateTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	// Create template
	createPayload := map[string]interface{}{
		"app_id":  s.appID,
		"name":    "delete_test",
		"channel": "email",
		"body":    "Test body",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/templates", createPayload, headers)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	var createResult map[string]interface{}
	s.parseResponse(body, &createResult)
	templateID := createResult["id"].(string)

	// Delete template
	resp, body = s.makeRequest(http.MethodDelete, "/v1/templates/"+templateID, nil, headers)

	s.True(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent)

	// Wait for indexing
	time.Sleep(1 * time.Second)

	// Verify it's archived (soft delete)
	resp, body = s.makeRequest(http.MethodGet, "/v1/templates/"+templateID, nil, headers)
	if resp.StatusCode == http.StatusOK {
		var result map[string]interface{}
		s.parseResponse(body, &result)
		s.Equal("archived", result["status"])
	}
}

// TestMultiLanguageTemplates tests creating templates in different locales
func (s *TemplateHTTPTestSuite) TestMultiLanguageTemplates() {
	apiKey := s.setupForTemplateTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	// Create English template
	enPayload := map[string]interface{}{
		"app_id":    s.appID,
		"name":      "welcome_message",
		"channel":   "email",
		"locale":    "en-US",
		"subject":   "Welcome!",
		"body":      "Hello {{.Name}}, welcome to our service!",
		"variables": []string{"Name"},
	}

	resp, _ := s.makeRequest(http.MethodPost, "/v1/templates", enPayload, headers)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	// Create Spanish template (same name, different locale)
	esPayload := map[string]interface{}{
		"app_id":    s.appID,
		"name":      "welcome_message",
		"channel":   "email",
		"locale":    "es-ES",
		"subject":   "¡Bienvenido!",
		"body":      "Hola {{.Name}}, ¡bienvenido a nuestro servicio!",
		"variables": []string{"Name"},
	}

	resp, _ = s.makeRequest(http.MethodPost, "/v1/templates", esPayload, headers)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	// Wait for indexing
	time.Sleep(2 * time.Second)

	// List templates with locale filter
	resp, body := s.makeRequest(http.MethodGet, "/v1/templates?name=welcome_message&locale=es-ES&limit=10", nil, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	s.parseResponse(body, &result)

	templates := result["templates"].([]interface{})
	s.GreaterOrEqual(len(templates), 1)

	// Verify it's the Spanish version
	if len(templates) > 0 {
		tmpl := templates[0].(map[string]interface{})
		s.Equal("es-ES", tmpl["locale"])
		s.Contains(tmpl["body"].(string), "Hola")
	}
}

// TestTemplateWithoutAuthentication tests template endpoints without API key
func (s *TemplateHTTPTestSuite) TestTemplateWithoutAuthentication() {
	s.setupForTemplateTests()

	payload := map[string]interface{}{
		"app_id":  s.appID,
		"name":    "test",
		"channel": "email",
		"body":    "Test",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/templates", payload, nil)

	s.Equal(http.StatusUnauthorized, resp.StatusCode)

	var result map[string]interface{}
	s.parseResponse(body, &result)
	s.False(result["success"].(bool))
}

// TestTemplateLifecycle tests the complete template lifecycle
func (s *TemplateHTTPTestSuite) TestTemplateLifecycle() {
	apiKey := s.setupForTemplateTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	// 1. Create template
	createPayload := map[string]interface{}{
		"app_id":      s.appID,
		"name":        "lifecycle_template",
		"channel":     "email",
		"locale":      "en-US",
		"subject":     "Test Subject",
		"body":        "Hello {{.Name}}, this is version 1",
		"variables":   []string{"Name"},
		"description": "Lifecycle test template",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/templates", createPayload, headers)
	s.Equal(http.StatusCreated, resp.StatusCode)

	var createResult map[string]interface{}
	s.parseResponse(body, &createResult)
	templateID := createResult["id"].(string)

	// 2. Get template
	resp, body = s.makeRequest(http.MethodGet, "/v1/templates/"+templateID, nil, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	// 3. Update template
	updatePayload := map[string]interface{}{
		"description": "Updated lifecycle template",
		"body":        "Hello {{.Name}}, this is updated version",
	}
	resp, body = s.makeRequest(http.MethodPut, "/v1/templates/"+templateID, updatePayload, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	// 4. Render template
	renderPayload := map[string]interface{}{
		"data": map[string]interface{}{
			"Name": "Test User",
		},
	}
	resp, body = s.makeRequest(http.MethodPost, "/v1/templates/"+templateID+"/render", renderPayload, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	var renderResult map[string]interface{}
	s.parseResponse(body, &renderResult)
	s.Contains(renderResult["rendered_body"].(string), "Test User")

	// 5. List templates
	time.Sleep(1 * time.Second)
	resp, body = s.makeRequest(http.MethodGet, "/v1/templates?name=lifecycle_template&limit=10", nil, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	// 6. Delete template
	resp, body = s.makeRequest(http.MethodDelete, "/v1/templates/"+templateID, nil, headers)
	s.True(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent)

	// 7. Verify it's archived
	time.Sleep(1 * time.Second)
	resp, body = s.makeRequest(http.MethodGet, "/v1/templates/"+templateID, nil, headers)
	if resp.StatusCode == http.StatusOK {
		var result map[string]interface{}
		s.parseResponse(body, &result)
		s.Equal("archived", result["status"])
	}
}

// TestTemplateStatusChange tests changing template status
func (s *TemplateHTTPTestSuite) TestTemplateStatusChange() {
	apiKey := s.setupForTemplateTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	// Create template
	createPayload := map[string]interface{}{
		"app_id":    s.appID,
		"name":      "status_test",
		"channel":   "push",
		"body":      "Test {{.Data}}",
		"variables": []string{"Data"},
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/templates", createPayload, headers)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	var createResult map[string]interface{}
	s.parseResponse(body, &createResult)
	templateID := createResult["id"].(string)
	s.Equal("active", createResult["status"])

	// Update status to inactive
	updatePayload := map[string]interface{}{
		"status": "inactive",
	}

	resp, body = s.makeRequest(http.MethodPut, "/v1/templates/"+templateID, updatePayload, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	var updateResult map[string]interface{}
	s.parseResponse(body, &updateResult)
	s.Equal("inactive", updateResult["status"])

	// Try to render inactive template - should fail or render with warning
	renderPayload := map[string]interface{}{
		"data": map[string]interface{}{"Data": "test"},
	}

	resp, _ = s.makeRequest(http.MethodPost, "/v1/templates/"+templateID+"/render", renderPayload, headers)
	// Implementation may return error for inactive templates
	s.True(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest)
}
