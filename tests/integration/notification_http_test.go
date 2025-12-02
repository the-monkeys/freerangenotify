package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type NotificationHTTPTestSuite struct {
	IntegrationTestSuite
}

func TestNotificationHTTPSuite(t *testing.T) {
	suite.Run(t, new(NotificationHTTPTestSuite))
}

// setupForNotificationTests creates app, user, and device for notification tests
func (s *NotificationHTTPTestSuite) setupForNotificationTests() (string, string) {
	// Create application
	appPayload := map[string]interface{}{
		"app_name": "Notification Test App",
	}
	resp, body := s.makeRequest(http.MethodPost, "/v1/apps", appPayload, nil)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	appResult := s.assertSuccess(body)
	appData := appResult["data"].(map[string]interface{})
	s.appID = appData["app_id"].(string)
	s.apiKey = appData["api_key"].(string)

	headers := map[string]string{
		"Authorization": "Bearer " + s.apiKey,
	}

	// Create user
	userPayload := map[string]interface{}{
		"external_user_id": "notif_user_001",
		"email":            "notifuser@example.com",
		"phone":            "+1234567890",
		"timezone":         "America/New_York",
		"language":         "en",
	}
	resp, body = s.makeRequest(http.MethodPost, "/v1/users", userPayload, headers)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	userResult := s.assertSuccess(body)
	userData := userResult["data"].(map[string]interface{})
	userID := userData["user_id"].(string)

	// Add device
	devicePayload := map[string]interface{}{
		"platform": "ios",
		"token":    "TEST_DEVICE_TOKEN_12345",
	}
	resp, _ = s.makeRequest(http.MethodPost, "/v1/users/"+userID+"/devices", devicePayload, headers)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	return userID, s.apiKey
}

// TestSendNotification tests sending a single notification
func (s *NotificationHTTPTestSuite) TestSendNotification() {
	userID, apiKey := s.setupForNotificationTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	payload := map[string]interface{}{
		"user_id":  userID,
		"channel":  "push",
		"title":    "Test Notification",
		"body":     "This is a test push notification",
		"priority": "high",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/notifications", payload, headers)

	s.Equal(http.StatusAccepted, resp.StatusCode)

	// Response is directly the notification object, not wrapped in data
	var result map[string]interface{}
	s.parseResponse(body, &result)

	s.NotEmpty(result["notification_id"])
	s.Equal(s.appID, result["app_id"])
	s.Equal(userID, result["user_id"])
	s.Equal("push", result["channel"])
	s.Equal("high", result["priority"])
	s.Equal("Test Notification", result["title"])
	s.Equal("This is a test push notification", result["body"])
	s.Contains([]string{"pending", "queued"}, result["status"])

	// Store notification ID for other tests
	s.notificationID = result["notification_id"].(string)
}

// TestSendEmailNotification tests sending an email notification
func (s *NotificationHTTPTestSuite) TestSendEmailNotification() {
	userID, apiKey := s.setupForNotificationTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	payload := map[string]interface{}{
		"user_id":  userID,
		"channel":  "email",
		"title":    "Welcome Email",
		"body":     "Welcome to our service!",
		"priority": "normal",
		"data": map[string]interface{}{
			"email_subject": "Welcome to FreeRangeNotify",
			"reply_to":      "support@example.com",
		},
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/notifications", payload, headers)

	s.Equal(http.StatusAccepted, resp.StatusCode)

	var result map[string]interface{}
	s.parseResponse(body, &result)

	s.NotEmpty(result["notification_id"])
	s.Equal("email", result["channel"])
	s.Equal("Welcome Email", result["title"])
}

// TestSendSMSNotification tests sending an SMS notification
// Skipped: SMS provider (Twilio) needs to be configured
func (s *NotificationHTTPTestSuite) TestSendSMSNotification() {
	s.T().Skip("SMS provider not configured")
	userID, apiKey := s.setupForNotificationTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	payload := map[string]interface{}{
		"user_id":  userID,
		"channel":  "sms",
		"body":     "Your verification code is 123456",
		"priority": "high",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/notifications", payload, headers)

	s.Equal(http.StatusAccepted, resp.StatusCode)

	var result map[string]interface{}
	s.parseResponse(body, &result)

	s.NotEmpty(result["notification_id"])
	s.Equal("sms", result["channel"])
}

// TestSendNotificationValidation tests notification validation errors
func (s *NotificationHTTPTestSuite) TestSendNotificationValidation() {
	_, apiKey := s.setupForNotificationTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	testCases := []struct {
		name          string
		payload       map[string]interface{}
		expectedError string
	}{
		{
			name: "Missing user_id",
			payload: map[string]interface{}{
				"channel": "push",
				"title":   "Test",
				"body":    "Test body",
			},
			expectedError: "VALIDATION_ERROR",
		},
		{
			name: "Missing channel",
			payload: map[string]interface{}{
				"user_id": "some-user-id",
				"title":   "Test",
				"body":    "Test body",
			},
			expectedError: "VALIDATION_ERROR",
		},
		{
			name: "Invalid channel",
			payload: map[string]interface{}{
				"user_id": "some-user-id",
				"channel": "invalid_channel",
				"title":   "Test",
				"body":    "Test body",
			},
			expectedError: "VALIDATION_ERROR",
		},
		{
			name: "Invalid priority",
			payload: map[string]interface{}{
				"user_id":  "some-user-id",
				"channel":  "push",
				"title":    "Test",
				"body":     "Test body",
				"priority": "invalid_priority",
			},
			expectedError: "VALIDATION_ERROR",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			resp, body := s.makeRequest(http.MethodPost, "/v1/notifications", tc.payload, headers)

			s.Equal(http.StatusBadRequest, resp.StatusCode)
			s.assertError(body, tc.expectedError)
		})
	}
}

// TestGetNotification tests retrieving a notification by ID
func (s *NotificationHTTPTestSuite) TestGetNotification() {
	userID, apiKey := s.setupForNotificationTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	// Send notification first
	sendPayload := map[string]interface{}{
		"user_id":  userID,
		"channel":  "push",
		"title":    "Get Test Notification",
		"body":     "Testing GET endpoint",
		"priority": "normal",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/notifications", sendPayload, headers)
	s.Require().Equal(http.StatusAccepted, resp.StatusCode)

	var sendResult map[string]interface{}
	s.parseResponse(body, &sendResult)
	notificationID := sendResult["notification_id"].(string)

	// Wait a moment for processing
	time.Sleep(500 * time.Millisecond)

	// Get notification
	resp, body = s.makeRequest(http.MethodGet, "/v1/notifications/"+notificationID, nil, headers)

	s.Equal(http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	s.parseResponse(body, &result)

	s.Equal(notificationID, result["notification_id"])
	s.Equal(userID, result["user_id"])
	s.Equal("push", result["channel"])
	s.Equal("Get Test Notification", result["title"])
	s.Contains([]string{"pending", "queued", "sent", "delivered", "failed"}, result["status"])
}

// TestGetNotificationNotFound tests getting a non-existent notification
func (s *NotificationHTTPTestSuite) TestGetNotificationNotFound() {
	_, apiKey := s.setupForNotificationTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	resp, body := s.makeRequest(http.MethodGet, "/v1/notifications/non-existent-id", nil, headers)

	s.True(resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusInternalServerError)

	var result map[string]interface{}
	s.parseResponse(body, &result)
	if errObj, ok := result["error"].(map[string]interface{}); ok {
		code := errObj["code"].(string)
		s.True(code == "NOT_FOUND" || code == "DATABASE_ERROR", "Unexpected error code: "+code)
	}
}

// TestListNotifications tests listing notifications with filters
func (s *NotificationHTTPTestSuite) TestListNotifications() {
	userID, apiKey := s.setupForNotificationTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	// Create multiple notifications
	channels := []string{"push", "email", "sms"}
	for i, channel := range channels {
		payload := map[string]interface{}{
			"user_id":  userID,
			"channel":  channel,
			"title":    fmt.Sprintf("Test Notification %d", i+1),
			"body":     fmt.Sprintf("Testing %s channel", channel),
			"priority": "normal",
		}
		resp, _ := s.makeRequest(http.MethodPost, "/v1/notifications", payload, headers)
		s.Require().Equal(http.StatusAccepted, resp.StatusCode)
	}

	// Wait for processing
	time.Sleep(1 * time.Second)

	// List all notifications
	resp, body := s.makeRequest(http.MethodGet, "/v1/notifications?page=1&page_size=10", nil, headers)

	s.Equal(http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	s.parseResponse(body, &result)

	notifications := result["notifications"].([]interface{})
	s.GreaterOrEqual(len(notifications), 3)

	// Test filtering by user_id
	resp, body = s.makeRequest(http.MethodGet, "/v1/notifications?user_id="+userID+"&page_size=10", nil, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	s.parseResponse(body, &result)
	notifications = result["notifications"].([]interface{})
	s.GreaterOrEqual(len(notifications), 3)

	// Test filtering by channel
	resp, body = s.makeRequest(http.MethodGet, "/v1/notifications?channel=push&page_size=10", nil, headers)
	s.Equal(http.StatusOK, resp.StatusCode)
}

// TestSendBulkNotifications tests sending bulk notifications
func (s *NotificationHTTPTestSuite) TestSendBulkNotifications() {
	userID, apiKey := s.setupForNotificationTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	payload := map[string]interface{}{
		"user_ids": []string{userID},
		"channel":  "push",
		"title":    "Bulk Notification",
		"body":     "This is a bulk notification test",
		"priority": "normal",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/notifications/bulk", payload, headers)

	s.Equal(http.StatusAccepted, resp.StatusCode)

	// Bulk response is direct, not wrapped in success/data
	var result map[string]interface{}
	s.parseResponse(body, &result)

	s.Equal(float64(1), result["sent"])
	s.Equal(float64(1), result["total"])
}

// TestUpdateNotificationStatus tests updating notification status
func (s *NotificationHTTPTestSuite) TestUpdateNotificationStatus() {
	userID, apiKey := s.setupForNotificationTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	// Send notification first
	sendPayload := map[string]interface{}{
		"user_id":  userID,
		"channel":  "push",
		"title":    "Status Update Test",
		"body":     "Testing status update",
		"priority": "normal",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/notifications", sendPayload, headers)
	s.Require().Equal(http.StatusAccepted, resp.StatusCode)

	var sendResult map[string]interface{}
	s.parseResponse(body, &sendResult)
	notificationID := sendResult["notification_id"].(string)

	// Update status
	updatePayload := map[string]interface{}{
		"status": "delivered",
	}

	resp, body = s.makeRequest(http.MethodPut, "/v1/notifications/"+notificationID+"/status", updatePayload, headers)

	s.Equal(http.StatusOK, resp.StatusCode)

	var updateResult map[string]interface{}
	s.parseResponse(body, &updateResult)

	// Verify status updated
	time.Sleep(500 * time.Millisecond)
	resp, body = s.makeRequest(http.MethodGet, "/v1/notifications/"+notificationID, nil, headers)
	var result map[string]interface{}
	s.parseResponse(body, &result)
	s.Equal("delivered", result["status"])
}

// TestRetryNotification tests retrying a failed notification
func (s *NotificationHTTPTestSuite) TestRetryNotification() {
	userID, apiKey := s.setupForNotificationTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	// Send notification
	sendPayload := map[string]interface{}{
		"user_id":  userID,
		"channel":  "push",
		"title":    "Retry Test",
		"body":     "Testing retry",
		"priority": "normal",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/notifications", sendPayload, headers)
	s.Require().Equal(http.StatusAccepted, resp.StatusCode)

	var sendResult map[string]interface{}
	s.parseResponse(body, &sendResult)
	notificationID := sendResult["notification_id"].(string)

	// Mark as failed first
	updatePayload := map[string]interface{}{
		"status": "failed",
	}
	s.makeRequest(http.MethodPut, "/v1/notifications/"+notificationID+"/status", updatePayload, headers)

	time.Sleep(500 * time.Millisecond)

	// Retry notification
	resp, body = s.makeRequest(http.MethodPost, "/v1/notifications/"+notificationID+"/retry", nil, headers)

	s.Equal(http.StatusOK, resp.StatusCode)

	var retryResult map[string]interface{}
	s.parseResponse(body, &retryResult)
	s.NotEmpty(retryResult["message"])
}

// TestCancelNotification tests canceling a scheduled notification
func (s *NotificationHTTPTestSuite) TestCancelNotification() {
	userID, apiKey := s.setupForNotificationTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	// Send scheduled notification
	sendPayload := map[string]interface{}{
		"user_id":      userID,
		"channel":      "push",
		"title":        "Cancel Test",
		"body":         "Testing cancel",
		"priority":     "normal",
		"scheduled_at": time.Now().Add(1 * time.Hour).Format(time.RFC3339),
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/notifications", sendPayload, headers)
	s.Require().Equal(http.StatusAccepted, resp.StatusCode)

	var sendResult map[string]interface{}
	s.parseResponse(body, &sendResult)
	notificationID := sendResult["notification_id"].(string)

	// Cancel notification
	resp, body = s.makeRequest(http.MethodDelete, "/v1/notifications/"+notificationID, nil, headers)

	s.True(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent)

	// Verify it's cancelled
	time.Sleep(500 * time.Millisecond)
	resp, body = s.makeRequest(http.MethodGet, "/v1/notifications/"+notificationID, nil, headers)
	if resp.StatusCode == http.StatusOK {
		var result map[string]interface{}
		s.parseResponse(body, &result)
		s.Contains([]string{"cancelled", "canceled"}, result["status"])
	}
}

// TestNotificationWithoutAuthentication tests notification endpoints without API key
func (s *NotificationHTTPTestSuite) TestNotificationWithoutAuthentication() {
	payload := map[string]interface{}{
		"user_id":  "some-user",
		"channel":  "push",
		"title":    "Test",
		"body":     "Test",
		"priority": "normal",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/notifications", payload, nil)

	s.Equal(http.StatusUnauthorized, resp.StatusCode)

	var result map[string]interface{}
	s.parseResponse(body, &result)
	errObj := result["error"].(map[string]interface{})
	code := errObj["code"].(string)
	s.True(code == "INVALID_API_KEY" || code == "UNAUTHORIZED")
}

// TestNotificationLifecycle tests the complete notification lifecycle
func (s *NotificationHTTPTestSuite) TestNotificationLifecycle() {
	userID, apiKey := s.setupForNotificationTests()

	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
	}

	// 1. Send notification
	sendPayload := map[string]interface{}{
		"user_id":  userID,
		"channel":  "push",
		"title":    "Lifecycle Test",
		"body":     "Testing complete lifecycle",
		"priority": "high",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/notifications", sendPayload, headers)
	s.Equal(http.StatusAccepted, resp.StatusCode)

	var sendResult map[string]interface{}
	s.parseResponse(body, &sendResult)
	notificationID := sendResult["notification_id"].(string)

	// 2. Get notification
	time.Sleep(500 * time.Millisecond)
	resp, body = s.makeRequest(http.MethodGet, "/v1/notifications/"+notificationID, nil, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	// 3. Update status to sent
	updatePayload := map[string]interface{}{
		"status": "sent",
	}
	resp, body = s.makeRequest(http.MethodPut, "/v1/notifications/"+notificationID+"/status", updatePayload, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	// 4. Update status to delivered
	updatePayload["status"] = "delivered"
	time.Sleep(300 * time.Millisecond)
	resp, body = s.makeRequest(http.MethodPut, "/v1/notifications/"+notificationID+"/status", updatePayload, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	// 5. Verify final status
	time.Sleep(300 * time.Millisecond)
	resp, body = s.makeRequest(http.MethodGet, "/v1/notifications/"+notificationID, nil, headers)
	s.Equal(http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	s.parseResponse(body, &result)
	s.Equal("delivered", result["status"])
}
