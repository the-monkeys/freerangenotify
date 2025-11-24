# Integration Tests

This directory contains integration tests for the FreeRangeNotify API.

## Overview

The integration tests verify the complete functionality of all API endpoints by making actual HTTP requests to the running service. Tests are organized by feature:

- `suite_test.go` - Base test suite with common setup, helpers, and utilities
- `application_test.go` - Tests for application management endpoints (8 endpoints)
- `user_test.go` - Tests for user management endpoints (10 endpoints)

## Test Coverage

### Application Endpoints (8/8)
- ✅ Health check
- ✅ Create application
- ✅ Get application by ID
- ✅ List applications with pagination
- ✅ Update application
- ✅ Delete application
- ✅ Regenerate API key
- ✅ Update/Get application settings

### User Endpoints (10/10)
- ✅ Create user (with API key authentication)
- ✅ Get user by ID
- ✅ List users with pagination
- ✅ Update user
- ✅ Delete user
- ✅ Add device to user
- ✅ Get user devices
- ✅ Delete device
- ✅ Update user preferences
- ✅ Get user preferences

## Prerequisites

1. Docker and Docker Compose installed
2. Services running via `docker-compose up -d`
3. Go 1.24+ installed

## Running Tests

### Run all integration tests
```powershell
# Make sure services are running
docker-compose up -d

# Wait for services to be ready (or tests will wait automatically)
Start-Sleep -Seconds 10

# Run tests
go test -v ./tests/integration/...
```

### Run specific test suite
```powershell
# Run only application tests
go test -v ./tests/integration/ -run TestApplicationSuite

# Run only user tests
go test -v ./tests/integration/ -run TestUserSuite
```

### Run specific test
```powershell
# Run a specific test
go test -v ./tests/integration/ -run TestApplicationSuite/TestCreateApplication
```

### Run with coverage
```powershell
go test -v -cover -coverprofile=coverage.out ./tests/integration/...
go tool cover -html=coverage.out -o coverage.html
```

### Skip integration tests
```powershell
$env:INTEGRATION_TESTS="false"
go test ./...
```

## Test Structure

### Base Suite (`IntegrationTestSuite`)
Provides common functionality:
- Automatic service health checks
- HTTP client configuration
- Request/response helpers
- Assertion utilities
- Automatic cleanup of test data

### Test Helpers

**makeRequest(method, path, body, headers)** - Makes HTTP requests
```go
resp, body := s.makeRequest(http.MethodPost, "/v1/apps", payload, nil)
```

**assertSuccess(body)** - Verifies successful response
```go
result := s.assertSuccess(body)
data := result["data"].(map[string]interface{})
```

**assertError(body, code)** - Verifies error response
```go
s.assertError(body, "VALIDATION_ERROR")
```

### Adding New Tests

1. **For new feature/endpoint group**, create a new test file:
```go
package integration

import (
	"net/http"
	"testing"
	"github.com/stretchr/testify/suite"
)

type NotificationTestSuite struct {
	IntegrationTestSuite
}

func TestNotificationSuite(t *testing.T) {
	suite.Run(t, new(NotificationTestSuite))
}

func (s *NotificationTestSuite) TestSendNotification() {
	// Create app first
	apiKey := s.setupApplicationForTests()
	
	payload := map[string]interface{}{
		"user_id": "test-user",
		"title": "Test Notification",
		"body": "Test message",
	}
	
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}
	
	resp, body := s.makeRequest(http.MethodPost, "/v1/notifications", payload, headers)
	s.Equal(http.StatusCreated, resp.StatusCode)
	s.assertSuccess(body)
}
```

2. **For new test in existing suite**, add method to existing file:
```go
func (s *ApplicationTestSuite) TestNewFeature() {
	// Test implementation
}
```

## Test Patterns

### Creating Test Data
```go
// Application tests automatically create app in TestCreateApplication
s.TestCreateApplication()
// Now s.appID and s.apiKey are available

// Or create inline
payload := map[string]interface{}{"app_name": "Test App"}
resp, body := s.makeRequest(http.MethodPost, "/v1/apps", payload, nil)
result := s.assertSuccess(body)
appID := result["data"].(map[string]interface{})["app_id"].(string)
```

### Testing Validation Errors
```go
testCases := []struct {
	name          string
	payload       map[string]interface{}
	expectedError string
}{
	{
		name: "Missing required field",
		payload: map[string]interface{}{},
		expectedError: "VALIDATION_ERROR",
	},
}

for _, tc := range testCases {
	s.Run(tc.name, func() {
		resp, body := s.makeRequest(http.MethodPost, "/v1/endpoint", tc.payload, nil)
		s.Equal(http.StatusBadRequest, resp.StatusCode)
		s.assertError(body, tc.expectedError)
	})
}
```

### Testing Complete Lifecycle
```go
func (s *TestSuite) TestEntityLifecycle() {
	// 1. Create
	// 2. Read
	// 3. Update
	// 4. Delete
	// 5. Verify deletion
}
```

## Configuration

Tests use the following defaults (can be configured via environment variables):

- Base URL: `http://localhost:8080`
- Elasticsearch URL: `http://localhost:9200`
- Request Timeout: 30 seconds
- Health Check Retries: 30

## Troubleshooting

### Tests fail with connection errors
- Ensure Docker containers are running: `docker-compose ps`
- Check container logs: `docker-compose logs notification-service`

### Tests timeout waiting for services
- Increase `healthCheckRetries` in `suite_test.go`
- Check if services are actually healthy: `docker-compose ps`

### Random test failures
- Tests include automatic cleanup between runs
- If data persists, manually clean: `docker-compose down -v && docker-compose up -d`

### API key authentication fails
- Ensure Elasticsearch mapping uses `api_key.keyword` for term queries
- Check application_repository.go GetByAPIKey method

## CI/CD Integration

### GitHub Actions Example
```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.24'
      
      - name: Start services
        run: docker-compose up -d
      
      - name: Wait for services
        run: sleep 30
      
      - name: Run integration tests
        run: go test -v -cover ./tests/integration/...
      
      - name: Stop services
        run: docker-compose down -v
```

## Future Test Additions

When implementing new features, add tests following these patterns:

### Notification Endpoints
- Send notification
- Get notification status
- List notifications
- Bulk send

### Template Endpoints
- Create template
- Get template
- List templates
- Update template
- Delete template

### Analytics Endpoints
- Get delivery statistics
- Get user engagement metrics
- Export analytics data

### Webhook Endpoints
- Webhook delivery
- Retry failed webhooks
- Webhook logs
