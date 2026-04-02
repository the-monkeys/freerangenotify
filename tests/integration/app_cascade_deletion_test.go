package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// AppCascadeDeletionTestSuite tests that deleting an app properly cleans up
// all child resources, adopts shared resources, and removes stale links.
type AppCascadeDeletionTestSuite struct {
	IntegrationTestSuite
	sourceAPIKey string
	targetAPIKey string
	templateID   string
	userIDs      []string
}

func TestAppCascadeDeletionSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	suite.Run(t, new(AppCascadeDeletionTestSuite))
}

// ─── Helpers ──────────────────────────────────────────────────────────

func (s *AppCascadeDeletionTestSuite) createApp(name string) (appID, apiKey string) {
	resp, body := s.makeRequest(http.MethodPost, "/v1/apps", map[string]interface{}{
		"app_name": name,
	}, nil)
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "create app %s: %s", name, string(body))

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})
	return data["app_id"].(string), data["api_key"].(string)
}

func (s *AppCascadeDeletionTestSuite) createUser(apiKey, email string) string {
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

func (s *AppCascadeDeletionTestSuite) createTemplate(apiKey, appID, name string) string {
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

func (s *AppCascadeDeletionTestSuite) importResources(targetID, sourceID string, types []string) {
	resp, body := s.makeRequest(http.MethodPost, fmt.Sprintf("/v1/apps/%s/import", targetID), map[string]interface{}{
		"source_app_id": sourceID,
		"resources":     types,
	}, nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "import resources: %s", string(body))
}

func (s *AppCascadeDeletionTestSuite) getLinks(appID string) []map[string]interface{} {
	resp, body := s.makeRequest(http.MethodGet, fmt.Sprintf("/v1/apps/%s/links", appID), nil, nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "get links: %s", string(body))

	var result map[string]interface{}
	s.parseResponse(body, &result)

	// API returns {"data": {"links": [...], "total_count": N}}
	dataObj, ok := result["data"].(map[string]interface{})
	if !ok || dataObj == nil {
		return nil
	}
	linksArr, ok := dataObj["links"].([]interface{})
	if !ok || linksArr == nil {
		return nil
	}
	links := make([]map[string]interface{}, 0, len(linksArr))
	for _, item := range linksArr {
		links = append(links, item.(map[string]interface{}))
	}
	return links
}

// esRefresh forces Elasticsearch to refresh the given indices so recent writes are searchable.
func (s *AppCascadeDeletionTestSuite) esRefresh(indices ...string) {
	for _, idx := range indices {
		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/%s/_refresh", elasticsearchURL, idx), nil)
		resp, err := s.client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}
}

// esDocCount returns the number of documents matching a term query in an ES index.
func (s *AppCascadeDeletionTestSuite) esDocCount(index, field, value string) int64 {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{field: value},
		},
	}
	body, _ := json.Marshal(query)

	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/%s/_count", elasticsearchURL, index),
		bytes.NewReader(body))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return 0
	}

	var result map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&result))
	count, ok := result["count"].(float64)
	if !ok {
		return 0
	}
	return int64(count)
}

// esDocExists checks if a specific document exists in an ES index.
func (s *AppCascadeDeletionTestSuite) esDocExists(index, docID string) bool {
	req, _ := http.NewRequest(http.MethodHead,
		fmt.Sprintf("%s/%s/_doc/%s", elasticsearchURL, index, docID), nil)
	resp, err := s.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// esGetField retrieves a single field from a document.
func (s *AppCascadeDeletionTestSuite) esGetField(index, docID, field string) interface{} {
	req, _ := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/%s/_doc/%s", elasticsearchURL, index, docID), nil)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	source, _ := result["_source"].(map[string]interface{})
	return source[field]
}

func (s *AppCascadeDeletionTestSuite) TearDownTest() {
	// Clean up any users we created.
	for _, uid := range s.userIDs {
		if s.sourceAPIKey != "" {
			s.deleteUser(uid, s.sourceAPIKey)
		}
	}
	// Delegate remaining cleanup to the base suite.
	s.IntegrationTestSuite.TearDownTest()

	s.sourceAPIKey = ""
	s.targetAPIKey = ""
	s.templateID = ""
	s.userIDs = nil
}

// ─── Tests ────────────────────────────────────────────────────────────

// TestDeleteApp_CleansUpOwnedResources verifies that when an app is deleted,
// all its owned resources (users, templates) are removed from Elasticsearch.
func (s *AppCascadeDeletionTestSuite) TestDeleteApp_CleansUpOwnedResources() {
	// 1. Create app with resources.
	appID, apiKey := s.createApp("Cascade Owner App")
	s.appID = appID
	s.sourceAPIKey = apiKey

	userID := s.createUser(apiKey, "cascade-user@example.com")
	s.userIDs = append(s.userIDs, userID)

	templateID := s.createTemplate(apiKey, appID, "cascade_template")
	s.templateID = templateID

	// Allow ES to settle.
	time.Sleep(1 * time.Second)

	// Verify resources exist before deletion.
	s.True(s.esDocExists("users", userID), "user should exist before app deletion")
	s.True(s.esDocExists("templates", templateID), "template should exist before app deletion")

	// 2. Delete the app.
	resp, body := s.makeRequest(http.MethodDelete, "/v1/apps/"+appID, nil, nil)
	s.Equal(http.StatusOK, resp.StatusCode, "delete app: %s", string(body))
	s.appID = "" // Prevent double-delete in TearDown.

	// Allow cascade to propagate.
	time.Sleep(2 * time.Second)

	// 3. Verify all resources were cleaned up.
	s.False(s.esDocExists("users", userID), "user should be deleted after app deletion")
	s.False(s.esDocExists("templates", templateID), "template should be deleted after app deletion")

	// Verify the app document itself is gone.
	s.False(s.esDocExists("applications", appID), "app document should be deleted")

	// Verify no orphan data in non-shareable indices.
	s.Equal(int64(0), s.esDocCount("notifications", "app_id", appID))
	s.Equal(int64(0), s.esDocCount("app_memberships", "app_id", appID))
}

// TestDeleteApp_AdoptsSharedResources verifies that when a source app is
// deleted, its resources that are linked into a target app are transferred
// (adopted) to the target app rather than being deleted.
func (s *AppCascadeDeletionTestSuite) TestDeleteApp_AdoptsSharedResources() {
	// 1. Create source app with a template and user.
	sourceID, sourceKey := s.createApp("Cascade Source App")
	s.appID = sourceID
	s.sourceAPIKey = sourceKey

	templateID := s.createTemplate(sourceKey, sourceID, "shared_template")
	s.templateID = templateID

	userID := s.createUser(sourceKey, "shared-user@example.com")
	s.userIDs = append(s.userIDs, userID)

	// 2. Create target app and import resources from source.
	targetID, targetKey := s.createApp("Cascade Target App")
	s.secondAppID = targetID
	s.targetAPIKey = targetKey

	time.Sleep(1 * time.Second) // Let ES index settle.

	s.importResources(targetID, sourceID, []string{"users", "templates"})

	time.Sleep(2 * time.Second) // Let import settle.
	s.esRefresh("app_resource_links")

	// Verify links exist before deletion.
	links := s.getLinks(targetID)
	s.NotEmpty(links, "target app should have links after import")

	// 3. Delete the SOURCE app.
	resp, body := s.makeRequest(http.MethodDelete, "/v1/apps/"+sourceID, nil, nil)
	s.Equal(http.StatusOK, resp.StatusCode, "delete source app: %s", string(body))
	s.appID = "" // Prevent double-delete.

	time.Sleep(3 * time.Second) // Let cascade propagate.
	s.esRefresh("templates", "users", "app_resource_links")

	// 4. Verify resources were ADOPTED, not deleted.
	//    The template should still exist but now owned by the target app.
	s.True(s.esDocExists("templates", templateID), "shared template should be adopted, not deleted")
	newOwner := s.esGetField("templates", templateID, "app_id")
	s.Equal(targetID, newOwner, "template owner should be transferred to target app")

	// The user should also be adopted.
	s.True(s.esDocExists("users", userID), "shared user should be adopted, not deleted")
	userOwner := s.esGetField("users", userID, "app_id")
	s.Equal(targetID, userOwner, "user owner should be transferred to target app")

	// 5. Target app's links should be cleaned up (it now owns the resources directly).
	linksAfter := s.getLinks(targetID)
	sourceLinksRemaining := 0
	for _, l := range linksAfter {
		if l["source_app_id"] == sourceID {
			sourceLinksRemaining++
		}
	}
	s.Equal(0, sourceLinksRemaining, "no links should reference the deleted source app")

	// 6. Source app document should be gone.
	s.False(s.esDocExists("applications", sourceID), "source app should be deleted")
}

// TestDeleteApp_CleansUpIncomingLinks verifies that when a target app
// (one that imported resources from elsewhere) is deleted, its incoming
// links are cleaned up.
func (s *AppCascadeDeletionTestSuite) TestDeleteApp_CleansUpIncomingLinks() {
	// 1. Create source app with a template.
	sourceID, sourceKey := s.createApp("Link Source App")
	s.appID = sourceID
	s.sourceAPIKey = sourceKey

	templateID := s.createTemplate(sourceKey, sourceID, "link_template")
	s.templateID = templateID

	// 2. Create target app and import.
	targetID, _ := s.createApp("Link Target App")
	s.secondAppID = targetID

	time.Sleep(1 * time.Second)
	s.importResources(targetID, sourceID, []string{"templates"})
	time.Sleep(1 * time.Second)

	// Verify link was created.
	s.Greater(s.esDocCount("app_resource_links", "target_app_id", targetID), int64(0),
		"link should exist before target deletion")

	// 3. Delete the TARGET app.
	resp, body := s.makeRequest(http.MethodDelete, "/v1/apps/"+targetID, nil, nil)
	s.Equal(http.StatusOK, resp.StatusCode, "delete target app: %s", string(body))
	s.secondAppID = "" // Prevent double-delete.

	time.Sleep(2 * time.Second)

	// 4. Verify incoming links are cleaned up.
	s.Equal(int64(0), s.esDocCount("app_resource_links", "target_app_id", targetID),
		"all incoming links should be removed when target app is deleted")

	// 5. Source app's resources should remain untouched.
	s.True(s.esDocExists("templates", templateID), "source template should not be affected")
	owner := s.esGetField("templates", templateID, "app_id")
	s.Equal(sourceID, owner, "source template owner should be unchanged")
}

// TestDeleteApp_MultipleConsumers verifies adoption when multiple target apps
// link to the same source resource. Deleting the source should adopt into one
// consumer and re-point remaining consumers' links.
func (s *AppCascadeDeletionTestSuite) TestDeleteApp_MultipleConsumers() {
	// 1. Create source app with a template.
	sourceID, sourceKey := s.createApp("Multi Source App")
	s.appID = sourceID
	s.sourceAPIKey = sourceKey

	templateID := s.createTemplate(sourceKey, sourceID, "multi_consumer_template")
	s.templateID = templateID

	// 2. Create two target apps that import the same resource.
	target1ID, _ := s.createApp("Consumer One")
	target2ID, _ := s.createApp("Consumer Two")

	time.Sleep(1 * time.Second)
	s.importResources(target1ID, sourceID, []string{"templates"})
	s.importResources(target2ID, sourceID, []string{"templates"})
	time.Sleep(2 * time.Second)
	s.esRefresh("app_resource_links")

	// 3. Delete the source app.
	resp, body := s.makeRequest(http.MethodDelete, "/v1/apps/"+sourceID, nil, nil)
	s.Equal(http.StatusOK, resp.StatusCode, "delete source app: %s", string(body))
	s.appID = "" // Prevent double-delete.

	time.Sleep(3 * time.Second)
	s.esRefresh("templates", "app_resource_links")

	// 4. Template should survive — adopted by one of the consumers.
	s.True(s.esDocExists("templates", templateID), "multi-linked template should be adopted")

	newOwner := s.esGetField("templates", templateID, "app_id")
	s.Contains([]interface{}{target1ID, target2ID}, newOwner,
		"new owner should be one of the consumer apps")

	// 5. The consumer that didn't adopt should have a re-pointed link.
	var nonOwnerID string
	if newOwner == target1ID {
		nonOwnerID = target2ID
	} else {
		nonOwnerID = target1ID
	}

	// The non-owner should have a link pointing to the new owner as source.
	linksForNonOwner := s.getLinks(nonOwnerID)
	hasRepointedLink := false
	for _, l := range linksForNonOwner {
		if l["resource_id"] == templateID && l["source_app_id"] == newOwner {
			hasRepointedLink = true
			break
		}
	}
	s.True(hasRepointedLink, "non-owner consumer should have link re-pointed to new owner")

	// 6. No links should reference the deleted source app.
	s.Equal(int64(0), s.esDocCount("app_resource_links", "source_app_id", sourceID),
		"no links should reference the deleted source app")

	// Cleanup extra apps.
	s.deleteApplication(target1ID)
	s.deleteApplication(target2ID)
}

// TestDeleteApp_AppWithNoResources verifies deleting an empty app succeeds
// without errors.
func (s *AppCascadeDeletionTestSuite) TestDeleteApp_AppWithNoResources() {
	appID, _ := s.createApp("Empty App")
	s.appID = appID

	resp, body := s.makeRequest(http.MethodDelete, "/v1/apps/"+appID, nil, nil)
	s.Equal(http.StatusOK, resp.StatusCode, "delete empty app: %s", string(body))
	s.appID = ""

	time.Sleep(1 * time.Second)
	s.False(s.esDocExists("applications", appID), "empty app should be deleted")
}

// TestDeleteSourceResource_CleansUpStaleLinksInTargetApps verifies that when
// an individual resource (e.g. a template) is deleted from its owning (source)
// app, all link records in target apps that previously imported that resource
// are removed immediately, preventing stale / orphaned links.
func (s *AppCascadeDeletionTestSuite) TestDeleteSourceResource_CleansUpStaleLinksInTargetApps() {
	// 1. Create source app with a template.
	sourceID, sourceKey := s.createApp("Resource Delete Source App")
	s.appID = sourceID
	s.sourceAPIKey = sourceKey

	templateID := s.createTemplate(sourceKey, sourceID, "stale_link_template")
	s.templateID = templateID

	// 2. Create target app and import the template.
	targetID, _ := s.createApp("Resource Delete Target App")
	s.secondAppID = targetID

	time.Sleep(1 * time.Second)
	s.importResources(targetID, sourceID, []string{"templates"})
	time.Sleep(1 * time.Second)

	// Verify the link was created.
	s.Greater(s.esDocCount("app_resource_links", "resource_id", templateID), int64(0),
		"link should exist after import")

	// 3. Delete the template from the SOURCE app (not the entire app).
	headers := map[string]string{"Authorization": "Bearer " + sourceKey}
	resp, body := s.makeRequest(http.MethodDelete, "/v1/templates/"+templateID, nil, headers)
	s.True(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent,
		"delete template: %s", string(body))

	time.Sleep(2 * time.Second)

	// 4. All links pointing to the deleted template must be removed.
	s.Equal(int64(0), s.esDocCount("app_resource_links", "resource_id", templateID),
		"stale links should be removed after source resource deletion")

	// 5. The source and target apps themselves must be unaffected.
	s.True(s.esDocExists("applications", sourceID), "source app should still exist")
	s.True(s.esDocExists("applications", targetID), "target app should still exist")
}
