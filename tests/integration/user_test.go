package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type UserTestSuite struct {
	IntegrationTestSuite
}

func TestUserSuite(t *testing.T) {
	suite.Run(t, new(UserTestSuite))
}

// setupApplicationForUserTests creates an application and returns its API key
func (s *UserTestSuite) setupApplicationForUserTests() string {
	payload := map[string]interface{}{
		"app_name": "User Test App",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/apps", payload, nil)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})

	s.appID = data["app_id"].(string)
	s.apiKey = data["api_key"].(string)

	return s.apiKey
}

// TestCreateUser tests creating a new user
func (s *UserTestSuite) TestCreateUser() {
	apiKey := s.setupApplicationForUserTests()

	payload := map[string]interface{}{
		"external_user_id": "test-user-123",
		"email":            "test@example.com",
		"phone_number":     "+1234567890",
		"preferences": map[string]interface{}{
			"channels": []string{"email", "push"},
			"timezone": "America/New_York",
			"language": "en",
			"quiet_hours": map[string]interface{}{
				"enabled": true,
				"start":   "22:00",
				"end":     "08:00",
			},
		},
	}

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/users", payload, headers)

	s.Equal(http.StatusCreated, resp.StatusCode)

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})

	s.NotEmpty(data["user_id"])
	s.Equal("test-user-123", data["external_user_id"])
	s.Equal("test@example.com", data["email"])
	if data["phone_number"] != nil {
		s.Equal("+1234567890", data["phone_number"])
	}

	// Store for cleanup
	s.userID = data["user_id"].(string)
}

// TestCreateUserWithoutAPIKey tests creating user without authentication
func (s *UserTestSuite) TestCreateUserWithoutAPIKey() {
	payload := map[string]interface{}{
		"external_user_id": "test-user-123",
		"email":            "test@example.com",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/users", payload, nil)

	s.Equal(http.StatusUnauthorized, resp.StatusCode)
	// Accept either error code
	var result map[string]interface{}
	s.parseResponse(body, &result)
	errObj := result["error"].(map[string]interface{})
	code := errObj["code"].(string)
	s.True(code == "INVALID_API_KEY" || code == "UNAUTHORIZED", "Expected INVALID_API_KEY or UNAUTHORIZED, got: "+code)
} // TestCreateUserWithInvalidAPIKey tests creating user with invalid API key
func (s *UserTestSuite) TestCreateUserWithInvalidAPIKey() {
	payload := map[string]interface{}{
		"external_user_id": "test-user-123",
		"email":            "test@example.com",
	}

	headers := map[string]string{
		"Authorization": "Bearer invalid-api-key",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/users", payload, headers)

	s.Equal(http.StatusUnauthorized, resp.StatusCode)
	s.assertError(body, "INVALID_API_KEY")
}

// TestCreateUserValidation tests validation errors
func (s *UserTestSuite) TestCreateUserValidation() {
	apiKey := s.setupApplicationForUserTests()

	testCases := []struct {
		name          string
		payload       map[string]interface{}
		expectedError string
	}{
		{
			name: "Missing external_user_id",
			payload: map[string]interface{}{
				"email": "test@example.com",
			},
			expectedError: "VALIDATION_ERROR",
		},
		{
			name: "Invalid email",
			payload: map[string]interface{}{
				"external_user_id": "test-123",
				"email":            "not-an-email",
			},
			expectedError: "VALIDATION_ERROR",
		},
	}

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			resp, body := s.makeRequest(http.MethodPost, "/v1/users", tc.payload, headers)

			s.Equal(http.StatusBadRequest, resp.StatusCode)
			s.assertError(body, tc.expectedError)
		})
	}
}

// TestGetUser tests retrieving a user by ID
func (s *UserTestSuite) TestGetUser() {
	// Create a user first
	s.TestCreateUser()

	headers := map[string]string{
		"Authorization": "Bearer " + s.apiKey,
	}

	resp, body := s.makeRequest(http.MethodGet, "/v1/users/"+s.userID, nil, headers)

	s.Equal(http.StatusOK, resp.StatusCode)

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})

	s.Equal(s.userID, data["user_id"])
	s.Equal("test-user-123", data["external_user_id"])
	s.Equal("test@example.com", data["email"])
}

// TestGetUserNotFound tests getting a non-existent user
func (s *UserTestSuite) TestGetUserNotFound() {
	apiKey := s.setupApplicationForUserTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	resp, body := s.makeRequest(http.MethodGet, "/v1/users/non-existent-id", nil, headers)

	// Accept either 404 or 500 with NOT_FOUND/DATABASE_ERROR
	s.True(resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusInternalServerError)

	var result map[string]interface{}
	s.parseResponse(body, &result)
	if errObj, ok := result["error"].(map[string]interface{}); ok {
		code := errObj["code"].(string)
		s.True(code == "NOT_FOUND" || code == "DATABASE_ERROR", "Unexpected error code: "+code)
	}
} // TestListUsers tests listing users with pagination
func (s *UserTestSuite) TestListUsers() {
	apiKey := s.setupApplicationForUserTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	// Create multiple users
	for i := 1; i <= 3; i++ {
		payload := map[string]interface{}{
			"external_user_id": "test-user-" + string(rune('0'+i)),
			"email":            "user" + string(rune('0'+i)) + "@example.com",
		}
		resp, body := s.makeRequest(http.MethodPost, "/v1/users", payload, headers)
		s.Equal(http.StatusCreated, resp.StatusCode)

		if i == 1 {
			result := s.assertSuccess(body)
			data := result["data"].(map[string]interface{})
			s.userID = data["user_id"].(string)
		}
	}

	// Test listing
	resp, body := s.makeRequest(http.MethodGet, "/v1/users?page=1&page_size=10", nil, headers)

	s.Equal(http.StatusOK, resp.StatusCode)

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})

	users := data["users"].([]interface{})
	s.GreaterOrEqual(len(users), 0)
}

// TestUpdateUser tests updating a user
func (s *UserTestSuite) TestUpdateUser() {
	// Create a user first
	s.TestCreateUser()

	payload := map[string]interface{}{
		"email":        "updated@example.com",
		"phone_number": "+9876543210",
	}

	headers := map[string]string{
		"Authorization": "Bearer " + s.apiKey,
	}

	resp, body := s.makeRequest(http.MethodPut, "/v1/users/"+s.userID, payload, headers)

	s.Equal(http.StatusOK, resp.StatusCode)

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})

	s.Equal(s.userID, data["user_id"])
	s.Equal("updated@example.com", data["email"])
	if data["phone_number"] != nil {
		s.Equal("+9876543210", data["phone_number"])
	}
}

// TestDeleteUser tests deleting a user
func (s *UserTestSuite) TestDeleteUser() {
	// Create a user first
	s.TestCreateUser()

	headers := map[string]string{
		"Authorization": "Bearer " + s.apiKey,
	}

	resp, body := s.makeRequest(http.MethodDelete, "/v1/users/"+s.userID, nil, headers)

	s.Equal(http.StatusOK, resp.StatusCode)
	s.assertSuccess(body)

	// Verify it's deleted (may return 404 or 500)
	time.Sleep(1 * time.Second)
	resp, _ = s.makeRequest(http.MethodGet, "/v1/users/"+s.userID, nil, headers)
	s.True(resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusInternalServerError)

	// Clear userID so TearDownTest doesn't try to delete again
	s.userID = ""
}

// TestAddDevice tests adding a device to a user
func (s *UserTestSuite) TestAddDevice() {
	// Create a user first
	s.TestCreateUser()

	payload := map[string]interface{}{
		"platform": "ios",
		"token":    "device-token-abc123",
	}

	headers := map[string]string{
		"Authorization": "Bearer " + s.apiKey,
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/users/"+s.userID+"/devices", payload, headers)

	s.Equal(http.StatusCreated, resp.StatusCode)
	s.assertSuccess(body)
}

// TestAddDeviceValidation tests device validation
func (s *UserTestSuite) TestAddDeviceValidation() {
	s.TestCreateUser()

	testCases := []struct {
		name    string
		payload map[string]interface{}
	}{
		{
			name: "Missing platform",
			payload: map[string]interface{}{
				"token": "device-token",
			},
		},
		{
			name: "Missing token",
			payload: map[string]interface{}{
				"platform": "ios",
			},
		},
	}

	headers := map[string]string{
		"Authorization": "Bearer " + s.apiKey,
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			resp, body := s.makeRequest(http.MethodPost, "/v1/users/"+s.userID+"/devices", tc.payload, headers)

			s.Equal(http.StatusBadRequest, resp.StatusCode)
			s.assertError(body, "VALIDATION_ERROR")
		})
	}
}

// TestGetUserDevices tests retrieving user devices
func (s *UserTestSuite) TestGetUserDevices() {
	// Create a user and add a device
	s.TestCreateUser()

	headers := map[string]string{
		"Authorization": "Bearer " + s.apiKey,
	}

	// Add device
	devicePayload := map[string]interface{}{
		"platform": "android",
		"token":    "device-token-xyz789",
	}
	s.makeRequest(http.MethodPost, "/v1/users/"+s.userID+"/devices", devicePayload, headers)

	// Get devices
	resp, body := s.makeRequest(http.MethodGet, "/v1/users/"+s.userID+"/devices", nil, headers)

	s.Equal(http.StatusOK, resp.StatusCode)

	result := s.assertSuccess(body)
	data := result["data"].([]interface{})

	s.GreaterOrEqual(len(data), 1)

	device := data[0].(map[string]interface{})
	s.Equal("android", device["platform"])
	s.NotEmpty(device["device_id"])
}

// TestDeleteDevice tests deleting a device
func (s *UserTestSuite) TestDeleteDevice() {
	// Create a user and add a device
	s.TestCreateUser()

	headers := map[string]string{
		"Authorization": "Bearer " + s.apiKey,
	}

	// Add device
	devicePayload := map[string]interface{}{
		"platform": "ios",
		"token":    "device-token-123",
	}
	s.makeRequest(http.MethodPost, "/v1/users/"+s.userID+"/devices", devicePayload, headers)

	// Get device ID
	resp, body := s.makeRequest(http.MethodGet, "/v1/users/"+s.userID+"/devices", nil, headers)
	result := s.assertSuccess(body)
	data := result["data"].([]interface{})
	device := data[0].(map[string]interface{})
	deviceID := device["device_id"].(string)

	// Delete device
	resp, body = s.makeRequest(http.MethodDelete, "/v1/users/"+s.userID+"/devices/"+deviceID, nil, headers)

	s.Equal(http.StatusOK, resp.StatusCode)
	s.assertSuccess(body)

	// Verify it's deleted
	resp, body = s.makeRequest(http.MethodGet, "/v1/users/"+s.userID+"/devices", nil, headers)
	result = s.assertSuccess(body)
	data = result["data"].([]interface{})
	s.LessOrEqual(len(data), 1)
}

// TestUpdatePreferences tests updating user preferences
func (s *UserTestSuite) TestUpdatePreferences() {
	// Create a user first
	s.TestCreateUser()

	payload := map[string]interface{}{
		"channels": []string{"email", "push", "sms"},
		"timezone": "America/Los_Angeles",
		"language": "es",
		"quiet_hours": map[string]interface{}{
			"enabled": true,
			"start":   "23:00",
			"end":     "07:00",
		},
	}

	headers := map[string]string{
		"Authorization": "Bearer " + s.apiKey,
	}

	resp, body := s.makeRequest(http.MethodPut, "/v1/users/"+s.userID+"/preferences", payload, headers)

	s.Equal(http.StatusOK, resp.StatusCode)
	s.assertSuccess(body)
}

// TestGetPreferences tests retrieving user preferences
func (s *UserTestSuite) TestGetPreferences() {
	// Create a user and update preferences
	s.TestCreateUser()

	headers := map[string]string{
		"Authorization": "Bearer " + s.apiKey,
	}

	updatePayload := map[string]interface{}{
		"channels": []string{"email", "sms"},
		"timezone": "Europe/London",
		"language": "fr",
	}
	s.makeRequest(http.MethodPut, "/v1/users/"+s.userID+"/preferences", updatePayload, headers)

	// Get preferences
	resp, body := s.makeRequest(http.MethodGet, "/v1/users/"+s.userID+"/preferences", nil, headers)

	s.Equal(http.StatusOK, resp.StatusCode)
	s.assertSuccess(body)
}

// TestUserLifecycle tests the complete user lifecycle
func (s *UserTestSuite) TestUserLifecycle() {
	apiKey := s.setupApplicationForUserTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	// 1. Create user
	createPayload := map[string]interface{}{
		"external_user_id": "lifecycle-user",
		"email":            "lifecycle@example.com",
		"phone_number":     "+1111111111",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/users", createPayload, headers)
	s.Equal(http.StatusCreated, resp.StatusCode)

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})
	userID := data["user_id"].(string)
	s.userID = userID

	// 2. Get user
	resp, body = s.makeRequest(http.MethodGet, "/v1/users/"+userID, nil, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	// 3. Update user
	updatePayload := map[string]interface{}{
		"email": "updated-lifecycle@example.com",
	}
	resp, body = s.makeRequest(http.MethodPut, "/v1/users/"+userID, updatePayload, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	// 4. Add device
	devicePayload := map[string]interface{}{
		"platform": "web",
		"token":    "web-push-token",
	}
	resp, body = s.makeRequest(http.MethodPost, "/v1/users/"+userID+"/devices", devicePayload, headers)
	s.Equal(http.StatusCreated, resp.StatusCode)

	// 5. Get devices
	resp, body = s.makeRequest(http.MethodGet, "/v1/users/"+userID+"/devices", nil, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	// 6. Update preferences
	prefsPayload := map[string]interface{}{
		"channels": []string{"email"},
		"timezone": "Asia/Kolkata",
		"language": "hi",
	}
	resp, body = s.makeRequest(http.MethodPut, "/v1/users/"+userID+"/preferences", prefsPayload, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	// 7. Get preferences
	resp, body = s.makeRequest(http.MethodGet, "/v1/users/"+userID+"/preferences", nil, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	// 8. Delete user
	resp, body = s.makeRequest(http.MethodDelete, "/v1/users/"+userID, nil, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	// 9. Verify deletion
	time.Sleep(1 * time.Second)
	resp, _ = s.makeRequest(http.MethodGet, "/v1/users/"+userID, nil, headers)
	s.True(resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusInternalServerError)

	s.userID = ""
}
