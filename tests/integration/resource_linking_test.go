package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// ResourceLinkingTestSuite tests cross-app resource linking, precise list
// filtering, ownership transfer on delete, and unlink-on-delete.
type ResourceLinkingTestSuite struct {
	IntegrationTestSuite
	sourceKey string
	targetKey string
	extraKey  string
	extraID   string
	userIDs   []string
}

func TestResourceLinkingSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	suite.Run(t, new(ResourceLinkingTestSuite))
}

// ─── Helpers ──────────────────────────────────────────────────────────

func (s *ResourceLinkingTestSuite) createApp(name string) (appID, apiKey string) {
	resp, body := s.makeRequest(http.MethodPost, "/v1/apps", map[string]interface{}{
		"app_name": name,
	}, nil)
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "create app %s: %s", name, string(body))

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})
	return data["app_id"].(string), data["api_key"].(string)
}

func (s *ResourceLinkingTestSuite) createUser(apiKey, email string) string {
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	resp, body := s.makeRequest(http.MethodPost, "/v1/users", map[string]interface{}{
		"email": email,
		"preferences": map[string]interface{}{
			"channels": []string{"email"},
			"timezone": "UTC",
		},
	}, headers)
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "create user %s: %s", email, string(body))

	var result map[string]interface{}
	s.parseResponse(body, &result)
	data := result["data"].(map[string]interface{})
	return data["user_id"].(string)
}

func (s *ResourceLinkingTestSuite) createTemplate(apiKey, appID, name string) string {
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	resp, body := s.makeRequest(http.MethodPost, "/v1/templates", map[string]interface{}{
		"app_id":  appID,
		"name":    name,
		"channel": "email",
		"locale":  "en-US",
		"subject": "Test {{.Name}}",
		"body":    "Hello {{.Name}}!",
	}, headers)
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "create template %s: %s", name, string(body))

	var result map[string]interface{}
	s.parseResponse(body, &result)
	return result["id"].(string)
}

func (s *ResourceLinkingTestSuite) importResources(targetID, sourceID string, types []string) {
	resp, body := s.makeRequest(http.MethodPost, fmt.Sprintf("/v1/apps/%s/import", targetID), map[string]interface{}{
		"source_app_id": sourceID,
		"resources":     types,
	}, nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "import resources: %s", string(body))
}

func (s *ResourceLinkingTestSuite) listUsers(apiKey string) ([]interface{}, float64) {
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	resp, body := s.makeRequest(http.MethodGet, "/v1/users?page_size=100", nil, headers)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "list users: %s", string(body))

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})
	users, _ := data["users"].([]interface{})
	total := data["total_count"].(float64)
	return users, total
}

func (s *ResourceLinkingTestSuite) listTemplates(apiKey string) ([]interface{}, float64) {
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	resp, body := s.makeRequest(http.MethodGet, "/v1/templates?limit=100", nil, headers)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "list templates: %s", string(body))

	// Template list endpoint returns data directly (no {success, data} wrapper).
	var result map[string]interface{}
	s.parseResponse(body, &result)

	var templates []interface{}
	var total float64
	if tpls, ok := result["templates"].([]interface{}); ok {
		templates = tpls
	}
	if tc, ok := result["total"].(float64); ok {
		total = tc
	}
	return templates, total
}

func (s *ResourceLinkingTestSuite) deleteUser(apiKey, userID string) (int, map[string]interface{}) {
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	resp, body := s.makeRequest(http.MethodDelete, "/v1/users/"+userID, nil, headers)

	var result map[string]interface{}
	s.parseResponse(body, &result)
	return resp.StatusCode, result
}

func (s *ResourceLinkingTestSuite) deleteTemplate(apiKey, templateID string) (int, string) {
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	resp, body := s.makeRequest(http.MethodDelete, "/v1/templates/"+templateID, nil, headers)
	return resp.StatusCode, string(body)
}

func (s *ResourceLinkingTestSuite) TearDownTest() {
	for _, uid := range s.userIDs {
		if s.sourceKey != "" {
			s.IntegrationTestSuite.deleteUser(uid, s.sourceKey)
		}
	}
	if s.extraID != "" {
		s.deleteApplication(s.extraID)
	}
	s.IntegrationTestSuite.TearDownTest()
	s.sourceKey = ""
	s.targetKey = ""
	s.extraKey = ""
	s.extraID = ""
	s.userIDs = nil
}

// ─── Tests: Precise List Filtering ───────────────────────────────────

// TestListUsers_ShowsOnlyOwnAndLinked verifies that the List endpoint returns
// only the app's own users plus specifically-linked users — not ALL users from
// linked source apps.
func (s *ResourceLinkingTestSuite) TestListUsers_ShowsOnlyOwnAndLinked() {
	// 1. Create source app with 3 users.
	sourceID, sourceKey := s.createApp("Link Source Users")
	s.appID = sourceID
	s.sourceKey = sourceKey

	uid1 := s.createUser(sourceKey, "source-user-1@example.com")
	uid2 := s.createUser(sourceKey, "source-user-2@example.com")
	uid3 := s.createUser(sourceKey, "source-user-3@example.com")
	s.userIDs = append(s.userIDs, uid1, uid2, uid3)

	// 2. Create target app with 1 own user.
	targetID, targetKey := s.createApp("Link Target Users")
	s.secondAppID = targetID
	s.targetKey = targetKey

	ownUID := s.createUser(targetKey, "target-own-user@example.com")
	s.userIDs = append(s.userIDs, ownUID)

	time.Sleep(1 * time.Second) // ES settle.

	// 3. Import users from source → target.
	s.importResources(targetID, sourceID, []string{"users"})
	time.Sleep(1 * time.Second)

	// 4. Verify: source should list 3 users.
	_, sourceTotal := s.listUsers(sourceKey)
	s.Equal(float64(3), sourceTotal, "source should have 3 users")

	// 5. Verify: target should list 4 users (1 own + 3 linked).
	_, targetTotal := s.listUsers(targetKey)
	s.Equal(float64(4), targetTotal, "target should have 4 users (1 own + 3 linked)")
}

// TestListTemplates_ShowsOnlyOwnAndLinked verifies precise template
// filtering with cross-app links.
func (s *ResourceLinkingTestSuite) TestListTemplates_ShowsOnlyOwnAndLinked() {
	// 1. Create source with 2 templates.
	sourceID, sourceKey := s.createApp("Link Source Templates")
	s.appID = sourceID
	s.sourceKey = sourceKey

	s.createTemplate(sourceKey, sourceID, "source_tpl_1")
	s.createTemplate(sourceKey, sourceID, "source_tpl_2")

	// 2. Create target with 1 own template.
	targetID, targetKey := s.createApp("Link Target Templates")
	s.secondAppID = targetID
	s.targetKey = targetKey

	s.createTemplate(targetKey, targetID, "target_own_tpl")

	time.Sleep(1 * time.Second)

	// 3. Import templates from source → target.
	s.importResources(targetID, sourceID, []string{"templates"})
	time.Sleep(1 * time.Second)

	// 4. Verify: target should list 3 (1 own + 2 linked).
	_, targetTotal := s.listTemplates(targetKey)
	s.Equal(float64(3), targetTotal, "target should have 3 templates (1 own + 2 linked)")

	// 5. Source should still list exactly 2.
	_, sourceTotal := s.listTemplates(sourceKey)
	s.Equal(float64(2), sourceTotal, "source should have 2 templates")
}

// TestListUsers_NoLinks_ShowsOnlyOwn verifies that apps without links
// see only their own users.
func (s *ResourceLinkingTestSuite) TestListUsers_NoLinks_ShowsOnlyOwn() {
	appID, apiKey := s.createApp("Isolated App")
	s.appID = appID

	s.createUser(apiKey, "only-user@example.com")
	s.userIDs = append(s.userIDs, "placeholder") // cleanup via teardown app delete
	time.Sleep(1 * time.Second)

	_, total := s.listUsers(apiKey)
	s.Equal(float64(1), total, "isolated app should see only its own user")
}

// ─── Tests: Ownership Transfer on Delete ─────────────────────────────

// TestDeleteOwnedUser_TransfersToConsumer verifies that deleting a user
// that was imported by another app transfers ownership instead of destroying it.
func (s *ResourceLinkingTestSuite) TestDeleteOwnedUser_TransfersToConsumer() {
	// 1. Create source with 1 user.
	sourceID, sourceKey := s.createApp("Transfer Source")
	s.appID = sourceID
	s.sourceKey = sourceKey

	uid := s.createUser(sourceKey, "transfer-user@example.com")
	s.userIDs = append(s.userIDs, uid)

	// 2. Create target and import.
	targetID, targetKey := s.createApp("Transfer Target")
	s.secondAppID = targetID
	s.targetKey = targetKey

	time.Sleep(1 * time.Second)
	s.importResources(targetID, sourceID, []string{"users"})
	time.Sleep(1 * time.Second)

	// 3. Delete the user from the SOURCE (owner).
	status, result := s.deleteUser(sourceKey, uid)
	s.Equal(http.StatusOK, status, "delete owned user should succeed")
	s.True(result["success"].(bool))

	time.Sleep(1 * time.Second) // ES settle.

	// 4. Verify: user still exists, now owned by target app.
	s.True(s.esDocExists("users", uid), "user should be adopted, not deleted")
	newOwner := s.esGetField("users", uid, "app_id")
	s.Equal(targetID, newOwner, "user should be owned by target app after transfer")

	// 5. Verify: link record should be cleaned up (target now owns it directly).
	s.Equal(int64(0), s.esDocCount("app_resource_links", "resource_id", uid),
		"link record should be deleted after ownership transfer")
}

// TestDeleteOwnedTemplate_TransfersToConsumer verifies ownership transfer
// for templates when the owner deletes them.
func (s *ResourceLinkingTestSuite) TestDeleteOwnedTemplate_TransfersToConsumer() {
	sourceID, sourceKey := s.createApp("Template Transfer Source")
	s.appID = sourceID
	s.sourceKey = sourceKey

	tplID := s.createTemplate(sourceKey, sourceID, "transfer_template")

	targetID, targetKey := s.createApp("Template Transfer Target")
	s.secondAppID = targetID
	s.targetKey = targetKey

	time.Sleep(1 * time.Second)
	s.importResources(targetID, sourceID, []string{"templates"})
	time.Sleep(1 * time.Second)

	// Delete template from source — returns 204 No Content on success.
	status, _ := s.deleteTemplate(sourceKey, tplID)
	s.Equal(http.StatusNoContent, status, "delete owned template should succeed")

	time.Sleep(1 * time.Second)

	// Template should be adopted by target.
	s.True(s.esDocExists("templates", tplID), "template should be adopted")
	newOwner := s.esGetField("templates", tplID, "app_id")
	s.Equal(targetID, newOwner, "template should be owned by target app")

	// Link should be cleaned up.
	s.Equal(int64(0), s.esDocCount("app_resource_links", "resource_id", tplID),
		"link should be removed after ownership transfer")
}

// ─── Tests: Unlink on Delete ─────────────────────────────────────────

// TestDeleteLinkedUser_UnlinksFromTargetApp verifies that when a target app
// "deletes" an imported user, it actually unlinks instead of destroying.
func (s *ResourceLinkingTestSuite) TestDeleteLinkedUser_UnlinksFromTargetApp() {
	// 1. Create source with user.
	sourceID, sourceKey := s.createApp("Unlink Source")
	s.appID = sourceID
	s.sourceKey = sourceKey

	uid := s.createUser(sourceKey, "linked-user@example.com")
	s.userIDs = append(s.userIDs, uid)

	// 2. Create target and import.
	targetID, targetKey := s.createApp("Unlink Target")
	s.secondAppID = targetID
	s.targetKey = targetKey

	time.Sleep(1 * time.Second)
	s.importResources(targetID, sourceID, []string{"users"})
	time.Sleep(1 * time.Second)

	// Verify user appears in target list before unlink.
	_, total := s.listUsers(targetKey)
	s.GreaterOrEqual(total, float64(1), "target should see linked user")

	// 3. Delete user from TARGET (not the owner).
	status, result := s.deleteUser(targetKey, uid)
	s.Equal(http.StatusOK, status, "unlink should succeed")
	s.True(result["success"].(bool))
	// Should indicate it was an unlink, not a destroy.
	msg, _ := result["message"].(string)
	s.Contains(msg, "Linked", "message should indicate an unlink operation")

	time.Sleep(1 * time.Second)

	// 4. User should still exist in source.
	s.True(s.esDocExists("users", uid), "user should still exist in source after unlink")
	owner := s.esGetField("users", uid, "app_id")
	s.Equal(sourceID, owner, "user should still be owned by source")

	// 5. Target should no longer see the user.
	_, targetTotal := s.listUsers(targetKey)
	s.Equal(float64(0), targetTotal, "target should no longer see the unlinked user")

	// 6. Source should still see the user.
	_, sourceTotal := s.listUsers(sourceKey)
	s.Equal(float64(1), sourceTotal, "source should still see its own user")
}

// ─── Tests: Multiple Consumer Ownership Transfer ─────────────────────

// TestDeleteOwned_MultipleConsumers_TransfersAndRepoints verifies that when
// a resource is imported by multiple apps and the owner deletes it, ownership
// transfers to the first consumer and remaining consumers' links are re-pointed.
func (s *ResourceLinkingTestSuite) TestDeleteOwned_MultipleConsumers_TransfersAndRepoints() {
	// 1. Source app with 1 user.
	sourceID, sourceKey := s.createApp("Multi Source")
	s.appID = sourceID
	s.sourceKey = sourceKey

	uid := s.createUser(sourceKey, "multi-user@example.com")
	s.userIDs = append(s.userIDs, uid)

	// 2. Two target apps import the user.
	targetID1, targetKey1 := s.createApp("Multi Target 1")
	s.secondAppID = targetID1
	s.targetKey = targetKey1

	targetID2, targetKey2 := s.createApp("Multi Target 2")
	s.extraID = targetID2
	s.extraKey = targetKey2

	time.Sleep(1 * time.Second)
	s.importResources(targetID1, sourceID, []string{"users"})
	s.importResources(targetID2, sourceID, []string{"users"})
	time.Sleep(1 * time.Second)

	// Both targets should see the user.
	_, t1Total := s.listUsers(targetKey1)
	_, t2Total := s.listUsers(targetKey2)
	s.GreaterOrEqual(t1Total, float64(1), "target1 should see linked user")
	s.GreaterOrEqual(t2Total, float64(1), "target2 should see linked user")

	// 3. Owner deletes the user.
	status, result := s.deleteUser(sourceKey, uid)
	s.Equal(http.StatusOK, status, "delete should succeed")
	s.True(result["success"].(bool))

	time.Sleep(2 * time.Second) // ES settle.

	// 4. User should still exist, adopted by one of the targets.
	s.True(s.esDocExists("users", uid), "user should be adopted, not deleted")
	newOwner := s.esGetField("users", uid, "app_id")
	s.Contains([]interface{}{targetID1, targetID2}, newOwner,
		"user should be adopted by one of the consumer apps")

	// 5. The other consumer should still see the user via a re-pointed link.
	var otherKey string
	if newOwner == targetID1 {
		otherKey = targetKey2
	} else {
		otherKey = targetKey1
	}
	_, otherTotal := s.listUsers(otherKey)
	s.GreaterOrEqual(otherTotal, float64(1), "other consumer should still see user via re-pointed link")
}

// ─── Tests: List After Delete ────────────────────────────────────────

// TestListUsers_AfterUnlink_CountDecreases verifies that the user count
// decreases after unlinking a user from the target app.
func (s *ResourceLinkingTestSuite) TestListUsers_AfterUnlink_CountDecreases() {
	sourceID, sourceKey := s.createApp("Count Source")
	s.appID = sourceID
	s.sourceKey = sourceKey

	uid1 := s.createUser(sourceKey, "count-user-1@example.com")
	uid2 := s.createUser(sourceKey, "count-user-2@example.com")
	s.userIDs = append(s.userIDs, uid1, uid2)

	targetID, targetKey := s.createApp("Count Target")
	s.secondAppID = targetID
	s.targetKey = targetKey

	time.Sleep(1 * time.Second)
	s.importResources(targetID, sourceID, []string{"users"})
	time.Sleep(1 * time.Second)

	// Before unlink: target sees 2 linked users.
	_, before := s.listUsers(targetKey)
	s.Equal(float64(2), before, "target should see 2 linked users")

	// Unlink one user from target.
	s.deleteUser(targetKey, uid1)
	time.Sleep(1 * time.Second)

	// After unlink: target sees 1 linked user.
	_, after := s.listUsers(targetKey)
	s.Equal(float64(1), after, "target should see 1 linked user after unlink")

	// Source still sees both.
	_, sourceTotal := s.listUsers(sourceKey)
	s.Equal(float64(2), sourceTotal, "source should still see 2 users")
}

// ─── ES helpers (shared with cascade suite) ──────────────────────────

func (s *ResourceLinkingTestSuite) esDocExists(index, docID string) bool {
	req, _ := http.NewRequest(http.MethodHead,
		fmt.Sprintf("%s/%s/_doc/%s", resolvedESURL(), index, docID), nil)
	resp, err := s.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (s *ResourceLinkingTestSuite) esGetField(index, docID, field string) interface{} {
	req, _ := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/%s/_doc/%s", resolvedESURL(), index, docID), nil)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var result map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&result))
	source, _ := result["_source"].(map[string]interface{})
	return source[field]
}

func (s *ResourceLinkingTestSuite) esDocCount(index, field, value string) int64 {
	body := fmt.Sprintf(`{"query":{"term":{"%s":"%s"}}}`, field, value)
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/%s/_count", resolvedESURL(), index),
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil || resp.StatusCode == http.StatusNotFound {
		if resp != nil {
			resp.Body.Close()
		}
		return 0
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0
	}
	count, _ := result["count"].(float64)
	return int64(count)
}
