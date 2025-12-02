package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

const (
	baseURL            = "http://localhost:8080"
	elasticsearchURL   = "http://localhost:9200"
	testTimeout        = 30 * time.Second
	healthCheckRetries = 30
)

// IntegrationTestSuite provides common setup for all integration tests
type IntegrationTestSuite struct {
	suite.Suite
	client         *http.Client
	apiKey         string
	appID          string
	secondAppID    string
	userID         string
	deviceID       string
	notificationID string
	ctx            context.Context
}

// SetupSuite runs once before all tests
func (s *IntegrationTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.client = &http.Client{
		Timeout: testTimeout,
	}

	// Wait for services to be ready
	s.waitForServices()

	// Clean up any existing test data
	s.cleanupTestData()
}

// TearDownSuite runs once after all tests
func (s *IntegrationTestSuite) TearDownSuite() {
	s.cleanupTestData()
}

// SetupTest runs before each test
func (s *IntegrationTestSuite) SetupTest() {
	// Reset test data for each test
	s.apiKey = ""
	s.appID = ""
	s.secondAppID = ""
	s.userID = ""
	s.deviceID = ""
}

// TearDownTest runs after each test
func (s *IntegrationTestSuite) TearDownTest() {
	// Clean up resources created in the test
	if s.userID != "" && s.apiKey != "" {
		s.deleteUser(s.userID, s.apiKey)
	}
	if s.appID != "" {
		s.deleteApplication(s.appID)
	}
	if s.secondAppID != "" {
		s.deleteApplication(s.secondAppID)
	}
}

// waitForServices waits for all required services to be healthy
func (s *IntegrationTestSuite) waitForServices() {
	s.T().Log("Waiting for services to be ready...")

	// Wait for notification service
	for i := 0; i < healthCheckRetries; i++ {
		resp, err := s.client.Get(baseURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			s.T().Log("Notification service is ready")
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		if i == healthCheckRetries-1 {
			s.T().Fatal("Notification service did not become ready in time")
		}
		time.Sleep(1 * time.Second)
	}

	// Wait for Elasticsearch
	for i := 0; i < healthCheckRetries; i++ {
		resp, err := s.client.Get(elasticsearchURL + "/_cluster/health")
		if err == nil && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated) {
			resp.Body.Close()
			s.T().Log("Elasticsearch is ready")
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		if i == healthCheckRetries-1 {
			s.T().Fatal("Elasticsearch did not become ready in time")
		}
		time.Sleep(1 * time.Second)
	}

	// Give services a bit more time to stabilize
	time.Sleep(2 * time.Second)
}

// cleanupTestData removes all test data from Elasticsearch
func (s *IntegrationTestSuite) cleanupTestData() {
	indices := []string{"applications", "users", "notifications", "templates", "analytics"}

	for _, index := range indices {
		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/%s/_delete_by_query", elasticsearchURL, index), bytes.NewBuffer([]byte(`{"query":{"match_all":{}}}`)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.client.Do(req)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
	}

	// Wait for deletions to complete
	time.Sleep(1 * time.Second)
}

// makeRequest is a helper function to make HTTP requests
func (s *IntegrationTestSuite) makeRequest(method, path string, body interface{}, headers map[string]string) (*http.Response, []byte) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		s.Require().NoError(err, "Failed to marshal request body")
		bodyReader = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, baseURL+path, bodyReader)
	s.Require().NoError(err, "Failed to create request")

	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := s.client.Do(req)
	s.Require().NoError(err, "Failed to make request")

	respBody, err := io.ReadAll(resp.Body)
	s.Require().NoError(err, "Failed to read response body")
	resp.Body.Close()

	return resp, respBody
}

// parseResponse is a helper to parse JSON responses
func (s *IntegrationTestSuite) parseResponse(body []byte, target interface{}) {
	err := json.Unmarshal(body, target)
	s.Require().NoError(err, "Failed to parse response: %s", string(body))
}

// assertSuccess checks if the response indicates success
func (s *IntegrationTestSuite) assertSuccess(body []byte) map[string]interface{} {
	var result map[string]interface{}
	s.parseResponse(body, &result)

	success, ok := result["success"]
	s.Require().True(ok, "Response missing 'success' field: %s", string(body))
	s.Require().True(success.(bool), "Response indicates failure: %s", string(body))

	return result
}

// assertError checks if the response indicates an error with expected code
func (s *IntegrationTestSuite) assertError(body []byte, expectedCode string) map[string]interface{} {
	var result map[string]interface{}
	s.parseResponse(body, &result)

	// Check if it's a wrapped error response
	if success, ok := result["success"]; ok {
		s.Require().False(success.(bool), "Response indicates success when error expected")

		errObj, ok := result["error"].(map[string]interface{})
		s.Require().True(ok, "Response missing 'error' object")

		code, ok := errObj["code"].(string)
		s.Require().True(ok, "Error object missing 'code' field")
		s.Require().Equal(expectedCode, code, "Unexpected error code")
	} else {
		// Simple error response format: {"error": "message"}
		errMsg, ok := result["error"]
		s.Require().True(ok, "Response missing 'error' field")
		s.Require().NotEmpty(errMsg, "Error message should not be empty")
	}

	return result
}

// Helper methods for cleanup
func (s *IntegrationTestSuite) deleteApplication(appID string) {
	req, _ := http.NewRequest(http.MethodDelete, baseURL+"/v1/apps/"+appID, nil)
	resp, _ := s.client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
}

func (s *IntegrationTestSuite) deleteUser(userID, apiKey string) {
	req, _ := http.NewRequest(http.MethodDelete, baseURL+"/v1/users/"+userID, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, _ := s.client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
}

// TestMain runs the test suite
func TestMain(m *testing.M) {
	// Check if integration tests should run
	if os.Getenv("INTEGRATION_TESTS") == "false" {
		fmt.Println("Skipping integration tests (INTEGRATION_TESTS=false)")
		os.Exit(0)
	}

	os.Exit(m.Run())
}
