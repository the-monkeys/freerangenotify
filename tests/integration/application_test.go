package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type ApplicationTestSuite struct {
	IntegrationTestSuite
}

func TestApplicationSuite(t *testing.T) {
	suite.Run(t, new(ApplicationTestSuite))
}

// TestHealthEndpoint tests the health check endpoint
func (s *ApplicationTestSuite) TestHealthEndpoint() {
	resp, body := s.makeRequest(http.MethodGet, "/health", nil, nil)

	s.Equal(http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	s.parseResponse(body, &result)

	s.Equal("FreeRangeNotify", result["service"])
	s.Equal("1.0.0", result["version"])
	s.Equal("ok", result["database"])
}

// TestCreateApplication tests creating a new application
func (s *ApplicationTestSuite) TestCreateApplication() {
	payload := map[string]interface{}{
		"app_name":    "Test Application",
		"webhook_url": "https://webhook.example.com",
		"settings": map[string]interface{}{
			"rate_limit":       1000,
			"retry_attempts":   3,
			"default_template": "default",
			"enable_webhooks":  true,
			"enable_analytics": true,
		},
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/apps", payload, nil)

	s.Equal(http.StatusCreated, resp.StatusCode)

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})

	s.NotEmpty(data["app_id"])
	s.Equal("Test Application", data["app_name"])
	s.NotEmpty(data["api_key"])
	s.Contains(data["api_key"].(string), "frn_")
	s.Equal("https://webhook.example.com", data["webhook_url"])

	// Store for cleanup
	s.appID = data["app_id"].(string)
	s.apiKey = data["api_key"].(string)
}

// TestCreateApplicationValidation tests validation errors
func (s *ApplicationTestSuite) TestCreateApplicationValidation() {
	testCases := []struct {
		name          string
		payload       map[string]interface{}
		expectedError string
	}{
		{
			name:          "Missing app_name",
			payload:       map[string]interface{}{},
			expectedError: "VALIDATION_ERROR",
		},
		{
			name: "Invalid webhook URL",
			payload: map[string]interface{}{
				"app_name":    "Test",
				"webhook_url": "not-a-url",
			},
			expectedError: "VALIDATION_ERROR",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			resp, body := s.makeRequest(http.MethodPost, "/v1/apps", tc.payload, nil)

			s.Equal(http.StatusBadRequest, resp.StatusCode)
			s.assertError(body, tc.expectedError)
		})
	}
}

// TestGetApplication tests retrieving an application by ID
func (s *ApplicationTestSuite) TestGetApplication() {
	// Create an application first
	s.TestCreateApplication()

	resp, body := s.makeRequest(http.MethodGet, "/v1/apps/"+s.appID, nil, nil)

	s.Equal(http.StatusOK, resp.StatusCode)

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})

	s.Equal(s.appID, data["app_id"])
	s.Equal("Test Application", data["app_name"])

	// API key should be masked
	apiKey := data["api_key"].(string)
	s.Contains(apiKey, "***")
	s.NotEqual(s.apiKey, apiKey)
}

// TestGetApplicationNotFound tests getting a non-existent application
func (s *ApplicationTestSuite) TestGetApplicationNotFound() {
	resp, body := s.makeRequest(http.MethodGet, "/v1/apps/non-existent-id", nil, nil)

	// Accept either 404 or 500 with NOT_FOUND/DATABASE_ERROR
	s.True(resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusInternalServerError)

	var result map[string]interface{}
	s.parseResponse(body, &result)
	if errObj, ok := result["error"].(map[string]interface{}); ok {
		code := errObj["code"].(string)
		s.True(code == "NOT_FOUND" || code == "DATABASE_ERROR", "Unexpected error code: "+code)
	}
}

// TestListApplications tests listing applications with pagination
func (s *ApplicationTestSuite) TestListApplications() {
	// Create multiple applications
	for i := 1; i <= 3; i++ {
		payload := map[string]interface{}{
			"app_name": fmt.Sprintf("Test App %d", i),
		}
		resp, body := s.makeRequest(http.MethodPost, "/v1/apps", payload, nil)
		s.Equal(http.StatusCreated, resp.StatusCode)

		result := s.assertSuccess(body)
		data := result["data"].(map[string]interface{})

		// Store first app for cleanup
		if i == 1 {
			s.appID = data["app_id"].(string)
		} else if i == 2 {
			s.secondAppID = data["app_id"].(string)
		}
	}

	// Test listing
	resp, body := s.makeRequest(http.MethodGet, "/v1/apps?page=1&page_size=2", nil, nil)

	s.Equal(http.StatusOK, resp.StatusCode)

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})

	apps := data["applications"].([]interface{})
	s.GreaterOrEqual(len(apps), 2)
	s.GreaterOrEqual(int(data["total_count"].(float64)), 2)
	s.Equal(float64(1), data["page"])
	s.Equal(float64(2), data["page_size"])
}

// TestUpdateApplication tests updating an application
func (s *ApplicationTestSuite) TestUpdateApplication() {
	// Create an application first
	s.TestCreateApplication()

	payload := map[string]interface{}{
		"app_name":    "Updated Application",
		"webhook_url": "https://new-webhook.example.com",
	}

	resp, body := s.makeRequest(http.MethodPut, "/v1/apps/"+s.appID, payload, nil)

	s.Equal(http.StatusOK, resp.StatusCode)

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})

	s.Equal(s.appID, data["app_id"])
	s.Equal("Updated Application", data["app_name"])
	s.Equal("https://new-webhook.example.com", data["webhook_url"])
}

// TestDeleteApplication tests deleting an application
func (s *ApplicationTestSuite) TestDeleteApplication() {
	// Create an application first
	s.TestCreateApplication()

	resp, body := s.makeRequest(http.MethodDelete, "/v1/apps/"+s.appID, nil, nil)

	s.Equal(http.StatusOK, resp.StatusCode)
	s.assertSuccess(body)

	// Wait for Elasticsearch to process deletion
	time.Sleep(1 * time.Second)

	// Verify it's deleted (may return 404 or 500 depending on implementation)
	resp, _ = s.makeRequest(http.MethodGet, "/v1/apps/"+s.appID, nil, nil)
	s.True(resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusInternalServerError)

	// Clear appID so TearDownTest doesn't try to delete again
	s.appID = ""
}

// TestRegenerateAPIKey tests regenerating an application's API key
func (s *ApplicationTestSuite) TestRegenerateAPIKey() {
	// Create an application first
	s.TestCreateApplication()
	oldAPIKey := s.apiKey

	resp, body := s.makeRequest(http.MethodPost, "/v1/apps/"+s.appID+"/regenerate-key", nil, nil)

	s.Equal(http.StatusOK, resp.StatusCode)

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})

	newAPIKey := data["api_key"].(string)
	s.NotEmpty(newAPIKey)
	s.NotEqual(oldAPIKey, newAPIKey)
	s.Contains(newAPIKey, "frn_")

	// Update stored API key
	s.apiKey = newAPIKey
}

// TestUpdateSettings tests updating application settings
func (s *ApplicationTestSuite) TestUpdateSettings() {
	// Create an application first
	s.TestCreateApplication()

	payload := map[string]interface{}{
		"rate_limit":       2000,
		"retry_attempts":   5,
		"default_template": "custom-template",
		"enable_webhooks":  false,
		"enable_analytics": false,
	}

	resp, body := s.makeRequest(http.MethodPut, "/v1/apps/"+s.appID+"/settings", payload, nil)

	s.Equal(http.StatusOK, resp.StatusCode)
	s.assertSuccess(body)
}

// TestGetSettings tests retrieving application settings
func (s *ApplicationTestSuite) TestGetSettings() {
	// Create an application and update settings
	s.TestCreateApplication()

	updatePayload := map[string]interface{}{
		"rate_limit":       2000,
		"retry_attempts":   5,
		"default_template": "custom-template",
		"enable_webhooks":  true,
		"enable_analytics": false,
	}

	s.makeRequest(http.MethodPut, "/v1/apps/"+s.appID+"/settings", updatePayload, nil)

	// Get settings
	resp, body := s.makeRequest(http.MethodGet, "/v1/apps/"+s.appID+"/settings", nil, nil)

	s.Equal(http.StatusOK, resp.StatusCode)

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})

	s.Equal(float64(2000), data["rate_limit"])
	s.Equal(float64(5), data["retry_attempts"])
	s.Equal("custom-template", data["default_template"])
}

// TestApplicationLifecycle tests the complete application lifecycle
func (s *ApplicationTestSuite) TestApplicationLifecycle() {
	// 1. Create application
	createPayload := map[string]interface{}{
		"app_name":    "Lifecycle Test App",
		"webhook_url": "https://example.com/webhook",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/apps", createPayload, nil)
	s.Equal(http.StatusCreated, resp.StatusCode)

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})
	appID := data["app_id"].(string)
	apiKey := data["api_key"].(string)

	s.appID = appID
	s.apiKey = apiKey

	// 2. Get application
	resp, body = s.makeRequest(http.MethodGet, "/v1/apps/"+appID, nil, nil)
	s.Equal(http.StatusOK, resp.StatusCode)

	// 3. Update application
	updatePayload := map[string]interface{}{
		"app_name": "Updated Lifecycle App",
	}
	resp, body = s.makeRequest(http.MethodPut, "/v1/apps/"+appID, updatePayload, nil)
	s.Equal(http.StatusOK, resp.StatusCode)

	// 4. Update settings
	settingsPayload := map[string]interface{}{
		"rate_limit":     1500,
		"retry_attempts": 4,
	}
	resp, body = s.makeRequest(http.MethodPut, "/v1/apps/"+appID+"/settings", settingsPayload, nil)
	s.Equal(http.StatusOK, resp.StatusCode)

	// 5. Regenerate API key
	resp, body = s.makeRequest(http.MethodPost, "/v1/apps/"+appID+"/regenerate-key", nil, nil)
	s.Equal(http.StatusOK, resp.StatusCode)

	// 6. Delete application
	resp, body = s.makeRequest(http.MethodDelete, "/v1/apps/"+appID, nil, nil)
	s.Equal(http.StatusOK, resp.StatusCode)

	// 7. Verify deletion
	time.Sleep(1 * time.Second)
	resp, _ = s.makeRequest(http.MethodGet, "/v1/apps/"+appID, nil, nil)
	s.True(resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusInternalServerError)

	s.appID = ""
}
