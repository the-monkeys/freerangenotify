package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// InAppNotificationTestSuite tests the in_app channel end-to-end:
// send → store in Elasticsearch → query inbox → manage (read, archive, snooze).
//
// Mirrors the Monkeys blogging platform integration described in monkeys-integration.md.
type InAppNotificationTestSuite struct {
	IntegrationTestSuite
}

func TestInAppNotificationSuite(t *testing.T) {
	requireLegacyIntegrationEnabled(t, "InAppNotificationSuite")
	suite.Run(t, new(InAppNotificationTestSuite))
}

// setup creates an app and a user, returns (userID, apiKey).
func (s *InAppNotificationTestSuite) setup() (string, string) {
	appPayload := map[string]interface{}{
		"app_name": "Monkeys Test App",
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

	userPayload := map[string]interface{}{
		"email":   fmt.Sprintf("monkeys-test-%d@example.com", time.Now().UnixNano()),
		"user_id": fmt.Sprintf("monkeys_user_%d", time.Now().UnixNano()),
	}
	resp, body = s.makeRequest(http.MethodPost, "/v1/users", userPayload, headers)
	s.Require().Equal(http.StatusCreated, resp.StatusCode)

	userResult := s.assertSuccess(body)
	userData := userResult["data"].(map[string]interface{})
	s.userID = userData["user_id"].(string)

	return s.userID, s.apiKey
}

func (s *InAppNotificationTestSuite) authHeaders() map[string]string {
	return map[string]string{"Authorization": "Bearer " + s.apiKey}
}

// TestSendInAppNotification verifies that sending channel=in_app returns 202
// and the notification is immediately queryable.
func (s *InAppNotificationTestSuite) TestSendInAppNotification() {
	userID, _ := s.setup()

	payload := map[string]interface{}{
		"user_id":  userID,
		"channel":  "in_app",
		"priority": "normal",
		"title":    "alice_monkeys started following you",
		"body":     "You have a new follower on Monkeys.",
		"category": "social",
		"data": map[string]interface{}{
			"follower_name": "alice_monkeys",
		},
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/notifications", payload, s.authHeaders())
	s.Equal(http.StatusAccepted, resp.StatusCode)

	var result map[string]interface{}
	s.parseResponse(body, &result)

	s.NotEmpty(result["notification_id"], "notification_id should not be empty")
	s.Equal("in_app", result["channel"])
	s.Equal("normal", result["priority"])
	s.Equal("social", result["category"])
	s.Equal("alice_monkeys started following you", result["title"])
	s.Contains([]string{"pending", "queued", "sent"}, result["status"])

	s.notificationID = result["notification_id"].(string)
}

// TestInAppNotificationAppearsInInbox verifies the notification is stored
// and queryable via GET /v1/notifications.
func (s *InAppNotificationTestSuite) TestInAppNotificationAppearsInInbox() {
	userID, _ := s.setup()

	// Send
	payload := map[string]interface{}{
		"user_id":  userID,
		"channel":  "in_app",
		"priority": "high",
		"title":    "alice_monkeys invited you to co-author a blog",
		"body":     `You have been invited to collaborate on "Building with Go".`,
		"category": "collaboration",
		"data": map[string]interface{}{
			"inviter_name": "alice_monkeys",
			"blog_title":   "Building with Go",
		},
	}
	resp, body := s.makeRequest(http.MethodPost, "/v1/notifications", payload, s.authHeaders())
	s.Require().Equal(http.StatusAccepted, resp.StatusCode)

	var sent map[string]interface{}
	s.parseResponse(body, &sent)
	notifID := sent["notification_id"].(string)

	// Allow worker to process
	time.Sleep(2 * time.Second)

	// Query inbox
	resp, body = s.makeRequest(
		http.MethodGet,
		fmt.Sprintf("/v1/notifications?user_id=%s&channel=in_app&page=1&page_size=20", userID),
		nil,
		s.authHeaders(),
	)
	s.Equal(http.StatusOK, resp.StatusCode)

	var inbox map[string]interface{}
	s.parseResponse(body, &inbox)

	notifications, ok := inbox["notifications"].([]interface{})
	s.Require().True(ok, "Response should contain a notifications array")

	found := false
	for _, n := range notifications {
		nm := n.(map[string]interface{})
		if nm["notification_id"] == notifID {
			found = true
			s.Equal("in_app", nm["channel"])
			s.Equal("collaboration", nm["category"])
			s.Equal("high", nm["priority"])
			break
		}
	}
	s.True(found, "Sent notification should appear in the inbox, notification_id=%s", notifID)
}

// TestUnreadCount verifies that unread count reflects inbox state.
func (s *InAppNotificationTestSuite) TestUnreadCount() {
	userID, _ := s.setup()

	// Send two in_app notifications
	for i := 0; i < 2; i++ {
		payload := map[string]interface{}{
			"user_id":  userID,
			"channel":  "in_app",
			"priority": "normal",
			"title":    fmt.Sprintf("Test notification %d", i+1),
			"body":     "Body text",
			"category": "social",
		}
		resp, _ := s.makeRequest(http.MethodPost, "/v1/notifications", payload, s.authHeaders())
		s.Require().Equal(http.StatusAccepted, resp.StatusCode)
	}

	time.Sleep(2 * time.Second)

	resp, body := s.makeRequest(
		http.MethodGet,
		fmt.Sprintf("/v1/notifications/unread/count?user_id=%s", userID),
		nil,
		s.authHeaders(),
	)
	s.Equal(http.StatusOK, resp.StatusCode)

	var countResp map[string]interface{}
	s.parseResponse(body, &countResp)

	count, ok := countResp["count"].(float64)
	s.True(ok, "Response should include a numeric count field")
	s.GreaterOrEqual(int(count), 2, "Unread count should be at least 2 after sending 2 in_app notifications")
}

// TestMarkNotificationRead verifies the mark-read endpoint clears read_at.
func (s *InAppNotificationTestSuite) TestMarkNotificationRead() {
	userID, _ := s.setup()

	// Send
	payload := map[string]interface{}{
		"user_id":  userID,
		"channel":  "in_app",
		"priority": "normal",
		"title":    "Your password was changed",
		"body":     "If this was not you, contact support immediately.",
		"category": "security",
	}
	resp, body := s.makeRequest(http.MethodPost, "/v1/notifications", payload, s.authHeaders())
	s.Require().Equal(http.StatusAccepted, resp.StatusCode)

	var sent map[string]interface{}
	s.parseResponse(body, &sent)
	notifID := sent["notification_id"].(string)

	time.Sleep(2 * time.Second)

	// Mark as read
	resp, _ = s.makeRequest(
		http.MethodPost,
		fmt.Sprintf("/v1/notifications/%s/read", notifID),
		nil,
		s.authHeaders(),
	)
	s.Equal(http.StatusOK, resp.StatusCode)

	// Fetch and confirm read_at is set
	resp, body = s.makeRequest(
		http.MethodGet,
		fmt.Sprintf("/v1/notifications/%s", notifID),
		nil,
		s.authHeaders(),
	)
	s.Equal(http.StatusOK, resp.StatusCode)

	var notif map[string]interface{}
	s.parseResponse(body, &notif)

	readAt, exists := notif["read_at"]
	s.True(exists, "read_at field should be present")
	s.NotNil(readAt, "read_at should be set after marking as read")
}

// TestArchiveNotification verifies the archive endpoint.
func (s *InAppNotificationTestSuite) TestArchiveNotification() {
	userID, _ := s.setup()

	payload := map[string]interface{}{
		"user_id":  userID,
		"channel":  "in_app",
		"priority": "low",
		"title":    "alice_monkeys liked your blog",
		"body":     `alice_monkeys liked "Building with Go".`,
		"category": "social",
		"data": map[string]interface{}{
			"liker_name": "alice_monkeys",
			"blog_title": "Building with Go",
		},
	}
	resp, body := s.makeRequest(http.MethodPost, "/v1/notifications", payload, s.authHeaders())
	s.Require().Equal(http.StatusAccepted, resp.StatusCode)

	var sent map[string]interface{}
	s.parseResponse(body, &sent)
	notifID := sent["notification_id"].(string)

	time.Sleep(2 * time.Second)

	// Archive
	resp, _ = s.makeRequest(
		http.MethodPost,
		fmt.Sprintf("/v1/notifications/%s/archive", notifID),
		nil,
		s.authHeaders(),
	)
	s.Equal(http.StatusOK, resp.StatusCode)

	// Confirm archived_at is set
	resp, body = s.makeRequest(
		http.MethodGet,
		fmt.Sprintf("/v1/notifications/%s", notifID),
		nil,
		s.authHeaders(),
	)
	s.Equal(http.StatusOK, resp.StatusCode)

	var notif map[string]interface{}
	s.parseResponse(body, &notif)

	archivedAt, exists := notif["archived_at"]
	s.True(exists, "archived_at field should be present")
	s.NotNil(archivedAt, "archived_at should be set after archiving")
}

// TestSnoozeNotification verifies snooze sets snoozed_until.
func (s *InAppNotificationTestSuite) TestSnoozeNotification() {
	userID, _ := s.setup()

	payload := map[string]interface{}{
		"user_id":  userID,
		"channel":  "in_app",
		"priority": "normal",
		"title":    "alice_monkeys commented on your blog",
		"body":     `alice_monkeys wrote: "Great article!"`,
		"category": "social",
	}
	resp, body := s.makeRequest(http.MethodPost, "/v1/notifications", payload, s.authHeaders())
	s.Require().Equal(http.StatusAccepted, resp.StatusCode)

	var sent map[string]interface{}
	s.parseResponse(body, &sent)
	notifID := sent["notification_id"].(string)

	time.Sleep(2 * time.Second)

	// Snooze for 1 hour
	resp, _ = s.makeRequest(
		http.MethodPost,
		fmt.Sprintf("/v1/notifications/%s/snooze", notifID),
		map[string]interface{}{"duration": "1h"},
		s.authHeaders(),
	)
	s.Equal(http.StatusOK, resp.StatusCode)

	// Confirm snoozed_until is set
	resp, body = s.makeRequest(
		http.MethodGet,
		fmt.Sprintf("/v1/notifications/%s", notifID),
		nil,
		s.authHeaders(),
	)
	s.Equal(http.StatusOK, resp.StatusCode)

	var notif map[string]interface{}
	s.parseResponse(body, &notif)

	snoozedUntil, exists := notif["snoozed_until"]
	s.True(exists, "snoozed_until field should be present")
	s.NotNil(snoozedUntil, "snoozed_until should be set after snoozing")
}

// TestCategoryFilter verifies that category filtering works for the inbox.
func (s *InAppNotificationTestSuite) TestCategoryFilter() {
	userID, _ := s.setup()

	// Send one social and one security notification
	for _, tc := range []struct {
		title    string
		category string
	}{
		{"alice_monkeys started following you", "social"},
		{"Your password was changed", "security"},
	} {
		payload := map[string]interface{}{
			"user_id":  userID,
			"channel":  "in_app",
			"priority": "normal",
			"title":    tc.title,
			"body":     "Body text",
			"category": tc.category,
		}
		resp, _ := s.makeRequest(http.MethodPost, "/v1/notifications", payload, s.authHeaders())
		s.Require().Equal(http.StatusAccepted, resp.StatusCode)
	}

	time.Sleep(2 * time.Second)

	// Filter to security only
	resp, body := s.makeRequest(
		http.MethodGet,
		fmt.Sprintf("/v1/notifications?user_id=%s&category=security&page=1&page_size=20", userID),
		nil,
		s.authHeaders(),
	)
	s.Equal(http.StatusOK, resp.StatusCode)

	var inbox map[string]interface{}
	s.parseResponse(body, &inbox)

	notifications, ok := inbox["notifications"].([]interface{})
	s.Require().True(ok)

	for _, n := range notifications {
		nm := n.(map[string]interface{})
		s.Equal("security", nm["category"], "category filter should only return security notifications")
	}
}

// TestInAppDoesNotRequireExternalDelivery verifies that in_app channel
// does not fail when no external delivery provider is configured.
// Unlike email or push, in_app "delivery" is storage only — no provider needed.
func (s *InAppNotificationTestSuite) TestInAppDoesNotRequireExternalDelivery() {
	userID, _ := s.setup()

	payload := map[string]interface{}{
		"user_id":  userID,
		"channel":  "in_app",
		"priority": "normal",
		"title":    "Notification without a delivery provider",
		"body":     "This should succeed even with no SMTP, push, or webhook configured.",
		"category": "social",
	}

	resp, body := s.makeRequest(http.MethodPost, "/v1/notifications", payload, s.authHeaders())
	s.Equal(http.StatusAccepted, resp.StatusCode)

	var result map[string]interface{}
	s.parseResponse(body, &result)
	s.NotEmpty(result["notification_id"])

	time.Sleep(2 * time.Second)

	// Confirm it is stored — status should be sent or pending, never failed
	resp, body = s.makeRequest(
		http.MethodGet,
		fmt.Sprintf("/v1/notifications/%s", result["notification_id"]),
		nil,
		s.authHeaders(),
	)
	s.Equal(http.StatusOK, resp.StatusCode)

	var notif map[string]interface{}
	s.parseResponse(body, &notif)
	s.NotEqual("failed", notif["status"], "in_app notification should not fail without an external provider")
}
