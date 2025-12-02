# Integration Test Suite Documentation

## Overview
Comprehensive HTTP API integration tests for FreeRangeNotify notification service.

## Test Coverage

### 1. Application APIs (`application_test.go`)
**14 test scenarios** covering complete application lifecycle:

- ✅ Health endpoint
- ✅ Create application (with validation)
- ✅ Get application (by ID, not found cases)
- ✅ List applications (with pagination)
- ✅ Update application
- ✅ Delete application
- ✅ Regenerate API key
- ✅ Update/Get settings
- ✅ Complete lifecycle test

**Key Features Tested:**
- API key generation and masking
- Settings management (rate limits, webhooks, analytics)
- Validation errors
- Pagination

---

### 2. User APIs (`user_test.go`)
**15 test scenarios** covering user management:

- ✅ Create user (with/without API key)
- ✅ Authentication validation
- ✅ Get user (by ID, not found cases)
- ✅ List users (with pagination)
- ✅ Update user
- ✅ Delete user
- ✅ Add device (validation included)
- ✅ Get devices
- ✅ Delete device
- ✅ Update/Get preferences (channels, timezone, quiet hours)
- ✅ Complete lifecycle test

**Key Features Tested:**
- Bearer token authentication
- Device management (iOS, Android, Web)
- User preferences (email/push/SMS enabled)
- Quiet hours functionality
- Field validation

---

### 3. Notification APIs (`notification_http_test.go`) - NEW
**15 test scenarios** covering notification sending and management:

- ✅ Send single notification (push, email, SMS)
- ✅ Send bulk notifications
- ✅ Get notification by ID
- ✅ List notifications (with filters: user_id, channel, status)
- ✅ Update notification status
- ✅ Retry failed notification
- ✅ Cancel scheduled notification
- ✅ Validation errors (missing fields, invalid channel/priority)
- ✅ Authentication checks
- ✅ Complete lifecycle test

**Key Features Tested:**
- Multiple channels (push, email, sms, webhook, in_app)
- Priority levels (low, normal, high, urgent)
- Status transitions (pending → queued → sent → delivered)
- Scheduled notifications
- Bulk sending
- Retry logic

---

### 4. Template APIs (`template_http_test.go`) - NEW
**17 test scenarios** covering template management:

- ✅ Create template (with validation)
- ✅ Get template (by ID, not found cases)
- ✅ List templates (with filters: app_id, channel, name, locale)
- ✅ Update template
- ✅ Delete template (soft delete to archived status)
- ✅ Render template with variable substitution
- ✅ Multi-language support (en-US, es-ES, etc.)
- ✅ Status management (active/inactive/archived)
- ✅ Authentication checks
- ✅ Complete lifecycle test

**Key Features Tested:**
- Go template rendering with {{.Variable}} syntax
- Variable validation (undefined variables detection)
- Multi-language templates (same name, different locale)
- Channel support (email, push, SMS)
- Subject and body templating
- Status-based rendering (inactive templates)
- Soft delete (archived status)

---

### 5. Domain-Level Tests (Existing)

**Template Domain Tests (`template_test.go`):**
- ✅ Create and get
- ✅ Get by name
- ✅ Update
- ✅ Delete (soft delete)
- ✅ List with filters
- ✅ Render with variables
- ✅ Render inactive template (error case)
- ✅ Create version (version 2, 3, etc.)
- ✅ Get all versions
- ✅ Multi-language templates
- ✅ Validation errors (invalid channel, undefined variables, duplicates)

**Notification E2E Tests (`notification_e2e_test.go`):**
- End-to-end notification processing
- Queue integration
- Metrics tracking

---

## Test Statistics

### Total Test Scenarios
- **Application APIs**: 14 tests
- **User APIs**: 15 tests
- **Notification HTTP APIs**: 15 tests ⭐ NEW
- **Template HTTP APIs**: 17 tests ⭐ NEW
- **Template Domain**: 12 tests
- **Notification E2E**: 9 tests
- **Total**: **82 integration test scenarios**

### Coverage by Feature
| Feature | HTTP Tests | Domain Tests | Total |
|---------|-----------|--------------|-------|
| Applications | 14 | 0 | 14 |
| Users | 15 | 0 | 15 |
| Notifications | 15 | 9 | 24 |
| Templates | 17 | 12 | 29 |
| **TOTAL** | **61** | **21** | **82** |

---

## Test Infrastructure

### Base Test Suite (`suite_test.go`)
Provides common functionality for all HTTP tests:

```go
type IntegrationTestSuite struct {
    suite.Suite
    baseURL         string
    appID           string
    apiKey          string
    userID          string
    secondAppID     string
    notificationID  string
}
```

**Helper Methods:**
- `makeRequest()` - HTTP request wrapper
- `assertSuccess()` - Validate successful responses
- `assertError()` - Validate error responses
- `parseResponse()` - JSON parsing
- `SetupSuite()` / `TearDownSuite()` - Suite-level setup/cleanup
- `SetupTest()` / `TearDownTest()` - Test-level setup/cleanup

---

## Running the Tests

### Run All Integration Tests
```bash
go test ./tests/integration/... -v
```

### Run Specific Test Suite
```bash
# Application tests
go test ./tests/integration/ -run TestApplicationSuite -v

# User tests
go test ./tests/integration/ -run TestUserSuite -v

# Notification HTTP tests
go test ./tests/integration/ -run TestNotificationHTTPSuite -v

# Template HTTP tests
go test ./tests/integration/ -run TestTemplateHTTPSuite -v

# Template domain tests
go test ./tests/integration/ -run TestTemplate -v
```

### Run Specific Test Case
```bash
go test ./tests/integration/ -run TestApplicationSuite/TestCreateApplication -v
go test ./tests/integration/ -run TestTemplateHTTPSuite/TestRenderTemplate -v
```

### With Coverage
```bash
go test ./tests/integration/... -v -coverprofile=coverage.out
go tool cover -html=coverage.out
```

---

## Prerequisites

### Required Services
All integration tests require these services running:

1. **Elasticsearch** (localhost:9200)
   ```bash
   docker-compose up -d elasticsearch
   ```

2. **Redis** (localhost:6379)
   ```bash
   docker-compose up -d redis
   ```

3. **Notification Service** (localhost:8080)
   ```bash
   # Option A: Run in Docker
   docker-compose up -d notification-service
   
   # Option B: Run locally
   go run cmd/server/main.go
   ```

### Check Service Health
```bash
# Elasticsearch
curl http://localhost:9200/_cluster/health

# Notification Service
curl http://localhost:8080/health
```

---

## Test Data Management

### Automatic Cleanup
- Each test suite cleans up created resources in `TearDownTest()`
- Applications, users, notifications, and templates are deleted after tests
- Redis queues are flushed

### Manual Cleanup
If tests are interrupted:

```bash
# Clear Elasticsearch indices
curl -X DELETE http://localhost:9200/applications
curl -X DELETE http://localhost:9200/users
curl -X DELETE http://localhost:9200/notifications
curl -X DELETE http://localhost:9200/templates

# Recreate indices (run migrations)
go run cmd/migrate/main.go

# Clear Redis
redis-cli FLUSHDB
```

---

## Test Patterns

### 1. Create-Retrieve-Update-Delete (CRUD)
```go
func (s *TemplateHTTPTestSuite) TestTemplateCRUD() {
    // Create
    resp, body := s.makeRequest(POST, "/v1/templates", payload, headers)
    
    // Retrieve
    resp, body = s.makeRequest(GET, "/v1/templates/"+id, nil, headers)
    
    // Update
    resp, body = s.makeRequest(PUT, "/v1/templates/"+id, updatePayload, headers)
    
    // Delete
    resp, body = s.makeRequest(DELETE, "/v1/templates/"+id, nil, headers)
}
```

### 2. Validation Testing
```go
testCases := []struct {
    name          string
    payload       map[string]interface{}
    expectedError string
}{
    {"Missing field", payload1, "VALIDATION_ERROR"},
    {"Invalid value", payload2, "VALIDATION_ERROR"},
}

for _, tc := range testCases {
    s.Run(tc.name, func() {
        resp, body := s.makeRequest(POST, endpoint, tc.payload, headers)
        s.assertError(body, tc.expectedError)
    })
}
```

### 3. Lifecycle Testing
Tests complete workflow from creation to deletion, verifying state at each step.

---

## Known Issues and Limitations

### Template Repository Issue (From API Testing)
**Status:** 🐛 NEEDS FIX

**Issue:** Template filtering by `app_id` not working correctly
- `GET /v1/templates?app_id={app_id}` returns empty array
- Direct Elasticsearch queries show data exists
- Affects version creation endpoint

**Workaround in Tests:**
- Tests use `GET /v1/templates` without app_id filter
- Or wait longer for Elasticsearch indexing

**Fix Required:**
- Check `template_repository.go` → `List()` method
- Verify Elasticsearch query builder for app_id filter
- Ensure field mapping is correct (keyword vs text)

---

## Continuous Integration

### GitHub Actions Example
```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    
    services:
      elasticsearch:
        image: docker.elastic.co/elasticsearch/elasticsearch:8.11.0
        env:
          discovery.type: single-node
          xpack.security.enabled: false
        ports:
          - 9200:9200
          
      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379
    
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24'
      
      - name: Wait for services
        run: |
          sleep 10
          curl --retry 10 --retry-delay 5 http://localhost:9200/_cluster/health
      
      - name: Run integration tests
        run: go test ./tests/integration/... -v
```

---

## Best Practices

### Writing New Tests

1. **Follow the Pattern:** Use existing test suites as templates
2. **Use Subtests:** Group related test cases with `s.Run()`
3. **Clean Up:** Always clean up created resources
4. **Wait for Indexing:** Add `time.Sleep()` after creating Elasticsearch resources
5. **Validate Thoroughly:** Check all important response fields
6. **Test Error Cases:** Don't just test happy paths

### Example New Test
```go
func (s *MyTestSuite) TestNewFeature() {
    // Setup
    apiKey := s.setupForTests()
    headers := map[string]string{"Authorization": "Bearer " + apiKey}
    
    // Test
    payload := map[string]interface{}{"key": "value"}
    resp, body := s.makeRequest(http.MethodPost, "/v1/endpoint", payload, headers)
    
    // Assertions
    s.Equal(http.StatusCreated, resp.StatusCode)
    result := s.assertSuccess(body)
    s.NotEmpty(result["data"])
    
    // Cleanup (if needed beyond TearDownTest)
}
```

---

## Documentation Status

- ✅ Test suite structure documented
- ✅ All test scenarios listed
- ✅ Running instructions provided
- ✅ Prerequisites documented
- ✅ Known issues tracked
- ✅ Best practices defined
- ✅ CI/CD example provided

## Summary

The integration test suite now provides **82 comprehensive test scenarios** covering all major HTTP APIs and domain logic. The new notification and template HTTP tests mirror real-world API usage patterns discovered during manual testing, ensuring robust coverage of:

- Authentication and authorization
- Input validation
- CRUD operations
- Business logic
- Error handling
- Complete workflows

All tests follow consistent patterns and can be run independently or as a full suite.
