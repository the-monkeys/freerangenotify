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

// ImportOperationsTestSuite tests that all CRUD operations work correctly
// on resources accessed via cross-app import links. This covers:
//   - GetByID on linked templates and users
//   - Render on linked templates (including {{var}} backward compat)
//   - Update on linked templates (metadata save)
//   - Delete (unlink) on linked templates/users
//   - List includes linked resources
type ImportOperationsTestSuite struct {
	IntegrationTestSuite
	sourceAppID string
	targetAppID string
	sourceKey   string
	targetKey   string
	userIDs     []string
	templateIDs []string
}

func TestImportOperationsSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	suite.Run(t, new(ImportOperationsTestSuite))
}

// ─── Helpers ──────────────────────────────────────────────────────────

func (s *ImportOperationsTestSuite) createApp(name string) (appID, apiKey string) {
	resp, body := s.makeRequest(http.MethodPost, "/v1/apps", map[string]interface{}{
		"app_name": name,
	}, nil)
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "create app %s: %s", name, string(body))

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})
	return data["app_id"].(string), data["api_key"].(string)
}

func (s *ImportOperationsTestSuite) createUser(apiKey, email string) string {
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	resp, body := s.makeRequest(http.MethodPost, "/v1/users", map[string]interface{}{
		"email":     email,
		"full_name": "Test User " + email,
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

func (s *ImportOperationsTestSuite) createTemplate(apiKey, name, bodyContent string) string {
	appID := s.appIDForKey(apiKey)
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	resp, body := s.makeRequest(http.MethodPost, "/v1/templates", map[string]interface{}{
		"app_id":  appID,
		"name":    name,
		"channel": "email",
		"locale":  "en",
		"subject": "Test {{.title}}",
		"body":    bodyContent,
	}, headers)
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "create template %s: %s", name, string(body))

	var result map[string]interface{}
	s.parseResponse(body, &result)
	return result["id"].(string)
}

func (s *ImportOperationsTestSuite) importResources(targetID, sourceID string, types []string) {
	resp, body := s.makeRequest(http.MethodPost, fmt.Sprintf("/v1/apps/%s/import", targetID), map[string]interface{}{
		"source_app_id": sourceID,
		"resources":     types,
	}, nil)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "import resources: %s", string(body))
}

func (s *ImportOperationsTestSuite) getTemplate(apiKey, templateID string) (int, map[string]interface{}) {
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	resp, body := s.makeRequest(http.MethodGet, "/v1/templates/"+templateID, nil, headers)

	var result map[string]interface{}
	s.parseResponse(body, &result)
	return resp.StatusCode, result
}

func (s *ImportOperationsTestSuite) renderTemplate(apiKey, templateID string, data map[string]interface{}, editable bool) (int, map[string]interface{}) {
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	resp, body := s.makeRequest(http.MethodPost, "/v1/templates/"+templateID+"/render", map[string]interface{}{
		"data":     data,
		"editable": editable,
	}, headers)

	var result map[string]interface{}
	s.parseResponse(body, &result)
	return resp.StatusCode, result
}

func (s *ImportOperationsTestSuite) getUser(apiKey, userID string) (int, map[string]interface{}) {
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	resp, body := s.makeRequest(http.MethodGet, "/v1/users/"+userID, nil, headers)

	var result map[string]interface{}
	s.parseResponse(body, &result)
	return resp.StatusCode, result
}

func (s *ImportOperationsTestSuite) listTemplates(apiKey string) ([]interface{}, float64) {
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	resp, body := s.makeRequest(http.MethodGet, "/v1/templates?limit=100", nil, headers)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "list templates: %s", string(body))

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

func (s *ImportOperationsTestSuite) listUsers(apiKey string) ([]interface{}, float64) {
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	resp, body := s.makeRequest(http.MethodGet, "/v1/users?page_size=100", nil, headers)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "list users: %s", string(body))

	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})
	users, _ := data["users"].([]interface{})
	total := data["total_count"].(float64)
	return users, total
}

func (s *ImportOperationsTestSuite) deleteTemplate(apiKey, templateID string) int {
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	resp, _ := s.makeRequest(http.MethodDelete, "/v1/templates/"+templateID, nil, headers)
	return resp.StatusCode
}

func (s *ImportOperationsTestSuite) deleteUser(apiKey, userID string) (int, map[string]interface{}) {
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	resp, body := s.makeRequest(http.MethodDelete, "/v1/users/"+userID, nil, headers)
	var result map[string]interface{}
	s.parseResponse(body, &result)
	return resp.StatusCode, result
}

func (s *ImportOperationsTestSuite) esDocExists(index, docID string) bool {
	req, _ := http.NewRequest(http.MethodHead,
		fmt.Sprintf("%s/%s/_doc/%s", elasticsearchURL, index, docID), nil)
	resp, err := s.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (s *ImportOperationsTestSuite) esGetField(index, docID, field string) interface{} {
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
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&result))
	source, _ := result["_source"].(map[string]interface{})
	return source[field]
}

func (s *ImportOperationsTestSuite) esDocCount(index, field, value string) int64 {
	body := fmt.Sprintf(`{"query":{"term":{"%s":"%s"}}}`, field, value)
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/%s/_count", elasticsearchURL, index),
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

// appIDForKey returns the app ID associated with the given API key.
func (s *ImportOperationsTestSuite) appIDForKey(apiKey string) string {
	if apiKey == s.sourceKey {
		return s.sourceAppID
	}
	if apiKey == s.targetKey {
		return s.targetAppID
	}
	s.FailNow("unknown API key passed to appIDForKey")
	return ""
}

func (s *ImportOperationsTestSuite) SetupTest() {
	s.IntegrationTestSuite.SetupTest()

	// Create source and target apps for each test.
	s.sourceAppID, s.sourceKey = s.createApp("ImportOps Source")
	s.appID = s.sourceAppID

	s.targetAppID, s.targetKey = s.createApp("ImportOps Target")
	s.secondAppID = s.targetAppID

	s.userIDs = nil
	s.templateIDs = nil
}

func (s *ImportOperationsTestSuite) TearDownTest() {
	// Clean up users first (they're owned by source or transferred to target).
	for _, uid := range s.userIDs {
		s.IntegrationTestSuite.deleteUser(uid, s.sourceKey)
		s.IntegrationTestSuite.deleteUser(uid, s.targetKey)
	}
	s.IntegrationTestSuite.TearDownTest()
}

// ─── Tests: Template GetByID After Import ────────────────────────────

func (s *ImportOperationsTestSuite) TestGetLinkedTemplate_ReturnsOK() {
	// 1. Create template in source app.
	tplID := s.createTemplate(s.sourceKey, "linked_get_test", "Hello {{.name}}")
	s.templateIDs = append(s.templateIDs, tplID)

	time.Sleep(1 * time.Second)

	// 2. Import templates into target app.
	s.importResources(s.targetAppID, s.sourceAppID, []string{"templates"})
	time.Sleep(1 * time.Second)

	// 3. Get template via target app's API key — should succeed.
	status, result := s.getTemplate(s.targetKey, tplID)
	s.Equal(http.StatusOK, status, "target app should be able to GET linked template")
	s.Equal(tplID, result["id"], "returned template ID should match")
	s.Equal("linked_get_test", result["name"], "returned template name should match")

	// 4. Verify source app can still GET its own template.
	status, result = s.getTemplate(s.sourceKey, tplID)
	s.Equal(http.StatusOK, status, "source app should still GET its own template")
	s.Equal(tplID, result["id"])
}

func (s *ImportOperationsTestSuite) TestGetUnlinkedTemplate_Returns404() {
	// Create template in source app but do NOT import.
	tplID := s.createTemplate(s.sourceKey, "no_link_test", "Hello {{.name}}")
	s.templateIDs = append(s.templateIDs, tplID)
	time.Sleep(1 * time.Second)

	// Target app should NOT be able to GET it.
	status, _ := s.getTemplate(s.targetKey, tplID)
	s.Equal(http.StatusNotFound, status, "target app should get 404 for non-linked template")
}

// ─── Tests: Template Render After Import ─────────────────────────────

func (s *ImportOperationsTestSuite) TestRenderLinkedTemplate_ReturnsOK() {
	// 1. Create template in source app with {{.var}} syntax.
	tplID := s.createTemplate(s.sourceKey, "linked_render_test", "Hello {{.name}}, welcome to {{.company}}")
	s.templateIDs = append(s.templateIDs, tplID)

	time.Sleep(1 * time.Second)

	// 2. Import into target.
	s.importResources(s.targetAppID, s.sourceAppID, []string{"templates"})
	time.Sleep(1 * time.Second)

	// 3. Render via target app's key.
	data := map[string]interface{}{
		"name":    "Alice",
		"company": "Acme Corp",
	}
	status, result := s.renderTemplate(s.targetKey, tplID, data, false)
	s.Equal(http.StatusOK, status, "target app should render linked template")

	rendered, _ := result["rendered_body"].(string)
	s.Contains(rendered, "Hello Alice", "rendered output should contain substituted name")
	s.Contains(rendered, "welcome to Acme Corp", "rendered output should contain substituted company")
}

func (s *ImportOperationsTestSuite) TestRenderLinkedTemplate_BareVarSyntax() {
	// Test backward compatibility: template stored with {{var}} (no dot).
	// Create via direct ES insert to simulate legacy data.
	tplID := s.createTemplateBareVar(s.sourceKey, "bare_var_tpl", "Hi {{name}}, you have {{count}} messages")
	s.templateIDs = append(s.templateIDs, tplID)

	time.Sleep(1 * time.Second)

	// Import into target.
	s.importResources(s.targetAppID, s.sourceAppID, []string{"templates"})
	time.Sleep(1 * time.Second)

	// Render via target.
	data := map[string]interface{}{
		"name":  "Bob",
		"count": "5",
	}
	status, result := s.renderTemplate(s.targetKey, tplID, data, false)
	s.Equal(http.StatusOK, status, "render with bare {{var}} syntax should succeed")

	rendered, _ := result["rendered_body"].(string)
	s.Contains(rendered, "Hi Bob", "bare var should be resolved")
	s.Contains(rendered, "5 messages", "bare var should be resolved")
}

func (s *ImportOperationsTestSuite) TestRenderLinkedTemplate_Editable() {
	tplID := s.createTemplate(s.sourceKey, "linked_editable_test",
		`<div><p>Hello {{.name}}</p></div>`)
	s.templateIDs = append(s.templateIDs, tplID)

	time.Sleep(1 * time.Second)

	s.importResources(s.targetAppID, s.sourceAppID, []string{"templates"})
	time.Sleep(1 * time.Second)

	data := map[string]interface{}{"name": "Carol"}
	status, result := s.renderTemplate(s.targetKey, tplID, data, true)
	s.Equal(http.StatusOK, status, "editable render on linked template should succeed")

	rendered, _ := result["rendered_body"].(string)
	s.Contains(rendered, "Carol", "rendered output should contain substituted data")
}

func (s *ImportOperationsTestSuite) TestRenderUnlinkedTemplate_Returns404() {
	tplID := s.createTemplate(s.sourceKey, "no_link_render", "Hello {{.name}}")
	s.templateIDs = append(s.templateIDs, tplID)
	time.Sleep(1 * time.Second)

	// Do NOT import — render should fail.
	status, _ := s.renderTemplate(s.targetKey, tplID, map[string]interface{}{"name": "X"}, false)
	s.Equal(http.StatusNotFound, status, "render without link should return 404")
}

// ─── Tests: User GetByID After Import ────────────────────────────────

func (s *ImportOperationsTestSuite) TestGetLinkedUser_ReturnsOK() {
	uid := s.createUser(s.sourceKey, "import-user-get@example.com")
	s.userIDs = append(s.userIDs, uid)

	time.Sleep(1 * time.Second)

	s.importResources(s.targetAppID, s.sourceAppID, []string{"users"})
	time.Sleep(1 * time.Second)

	// Target app should GET the linked user.
	status, result := s.getUser(s.targetKey, uid)
	s.Equal(http.StatusOK, status, "target app should GET linked user")
	data, _ := result["data"].(map[string]interface{})
	s.Equal(uid, data["user_id"], "returned user ID should match")
	s.Equal("import-user-get@example.com", data["email"], "returned email should match")
}

func (s *ImportOperationsTestSuite) TestGetUnlinkedUser_Returns404() {
	uid := s.createUser(s.sourceKey, "no-link-user@example.com")
	s.userIDs = append(s.userIDs, uid)
	time.Sleep(1 * time.Second)

	// Do NOT import — target should get 404.
	status, _ := s.getUser(s.targetKey, uid)
	s.Equal(http.StatusNotFound, status, "target should get 404 for non-linked user")
}

// ─── Tests: List After Import ────────────────────────────────────────

func (s *ImportOperationsTestSuite) TestListTemplates_IncludesLinked() {
	s.createTemplate(s.sourceKey, "src_tpl_1", "Hello {{.name}}")
	s.createTemplate(s.sourceKey, "src_tpl_2", "Bye {{.name}}")
	s.createTemplate(s.targetKey, "tgt_own_tpl", "Own {{.name}}")

	time.Sleep(1 * time.Second)

	s.importResources(s.targetAppID, s.sourceAppID, []string{"templates"})
	time.Sleep(1 * time.Second)

	// Target should list 3 (1 own + 2 linked).
	_, total := s.listTemplates(s.targetKey)
	s.Equal(float64(3), total, "target should list 3 templates (1 own + 2 linked)")

	// Source should list 2 (only its own).
	_, srcTotal := s.listTemplates(s.sourceKey)
	s.Equal(float64(2), srcTotal, "source should still list only its 2 templates")
}

func (s *ImportOperationsTestSuite) TestListUsers_IncludesLinked() {
	s.createUser(s.sourceKey, "src-list-1@example.com")
	s.createUser(s.sourceKey, "src-list-2@example.com")
	ownUID := s.createUser(s.targetKey, "tgt-own@example.com")
	s.userIDs = append(s.userIDs, ownUID)

	time.Sleep(1 * time.Second)

	s.importResources(s.targetAppID, s.sourceAppID, []string{"users"})
	time.Sleep(1 * time.Second)

	_, total := s.listUsers(s.targetKey)
	s.Equal(float64(3), total, "target should list 3 users (1 own + 2 linked)")

	_, srcTotal := s.listUsers(s.sourceKey)
	s.Equal(float64(2), srcTotal, "source should list only its 2 users")
}

// ─── Tests: Update After Import ──────────────────────────────────────

func (s *ImportOperationsTestSuite) updateTemplate(apiKey, templateID string, payload map[string]interface{}) (int, map[string]interface{}) {
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	resp, body := s.makeRequest(http.MethodPut, "/v1/templates/"+templateID, payload, headers)

	var result map[string]interface{}
	s.parseResponse(body, &result)
	return resp.StatusCode, result
}

func (s *ImportOperationsTestSuite) TestUpdateLinkedTemplate_MetadataFromTargetApp() {
	// 1. Create template in source app.
	tplID := s.createTemplate(s.sourceKey, "linked_update_test", "Hello {{.name}}")
	s.templateIDs = append(s.templateIDs, tplID)

	time.Sleep(1 * time.Second)

	// 2. Import into target.
	s.importResources(s.targetAppID, s.sourceAppID, []string{"templates"})
	time.Sleep(1 * time.Second)

	// 3. Target app updates metadata (save defaults) — this is the failing case.
	sampleData := map[string]interface{}{
		"name": "Alice",
	}
	status, result := s.updateTemplate(s.targetKey, tplID, map[string]interface{}{
		"metadata": map[string]interface{}{
			"category":    "newsletter",
			"sample_data": sampleData,
		},
	})
	s.Equal(http.StatusOK, status, "target app should be able to update linked template metadata")
	s.Equal(tplID, result["id"], "returned template ID should match")

	// 4. Verify the metadata persisted.
	getStatus, getResult := s.getTemplate(s.targetKey, tplID)
	s.Equal(http.StatusOK, getStatus)
	meta, _ := getResult["metadata"].(map[string]interface{})
	s.Equal("newsletter", meta["category"], "metadata category should persist")
}

func (s *ImportOperationsTestSuite) TestUpdateOwnTemplate_StillWorks() {
	// Source app updating its own template should work as before.
	tplID := s.createTemplate(s.sourceKey, "own_update_test", "Hello {{.name}}")
	s.templateIDs = append(s.templateIDs, tplID)

	time.Sleep(1 * time.Second)

	status, result := s.updateTemplate(s.sourceKey, tplID, map[string]interface{}{
		"metadata": map[string]interface{}{
			"category":    "alert",
			"sample_data": map[string]interface{}{"name": "Bob"},
		},
	})
	s.Equal(http.StatusOK, status, "source app should update its own template")
	s.Equal(tplID, result["id"])
}

func (s *ImportOperationsTestSuite) TestUpdateUnlinkedTemplate_Returns404() {
	// Target app should NOT be able to update a template it hasn't imported.
	tplID := s.createTemplate(s.sourceKey, "no_link_update", "Hello {{.name}}")
	s.templateIDs = append(s.templateIDs, tplID)

	time.Sleep(1 * time.Second)

	status, _ := s.updateTemplate(s.targetKey, tplID, map[string]interface{}{
		"metadata": map[string]interface{}{
			"sample_data": map[string]interface{}{"name": "Eve"},
		},
	})
	s.Equal(http.StatusNotFound, status, "target should get 404 for non-linked template update")
}

// ─── Tests: Delete (Unlink) After Import ─────────────────────────────

func (s *ImportOperationsTestSuite) TestDeleteLinkedTemplate_UnlinksFromTarget() {
	tplID := s.createTemplate(s.sourceKey, "unlink_tpl", "Hello {{.name}}")
	s.templateIDs = append(s.templateIDs, tplID)

	time.Sleep(1 * time.Second)

	s.importResources(s.targetAppID, s.sourceAppID, []string{"templates"})
	time.Sleep(1 * time.Second)

	// Verify template is accessible before unlink.
	status, _ := s.getTemplate(s.targetKey, tplID)
	s.Equal(http.StatusOK, status, "template should be accessible before unlink")

	// Delete from target (should unlink, not destroy).
	delStatus := s.deleteTemplate(s.targetKey, tplID)
	s.Equal(http.StatusNoContent, delStatus, "unlink should return 204")

	time.Sleep(1 * time.Second)

	// Template should still exist in source.
	s.True(s.esDocExists("templates", tplID), "template should still exist after unlink")
	owner := s.esGetField("templates", tplID, "app_id")
	s.Equal(s.sourceAppID, owner, "template should still be owned by source")

	// Target should no longer see it.
	_, total := s.listTemplates(s.targetKey)
	s.Equal(float64(0), total, "target should no longer list the unlinked template")
}

func (s *ImportOperationsTestSuite) TestDeleteLinkedUser_UnlinksFromTarget() {
	uid := s.createUser(s.sourceKey, "unlink-user@example.com")
	s.userIDs = append(s.userIDs, uid)

	time.Sleep(1 * time.Second)

	s.importResources(s.targetAppID, s.sourceAppID, []string{"users"})
	time.Sleep(1 * time.Second)

	// Verify user accessible.
	status, _ := s.getUser(s.targetKey, uid)
	s.Equal(http.StatusOK, status, "linked user should be accessible")

	// Delete from target (should unlink, not destroy or transfer).
	delStatus, result := s.deleteUser(s.targetKey, uid)
	s.Equal(http.StatusOK, delStatus, "unlink should succeed")
	msg, _ := result["message"].(string)
	s.Contains(msg, "removed from this application", "message should indicate removal from app")

	time.Sleep(1 * time.Second)

	// User should still exist in source.
	s.True(s.esDocExists("users", uid), "user should still exist after unlink")
	owner := s.esGetField("users", uid, "app_id")
	s.Equal(s.sourceAppID, owner, "user should still be owned by source")
}

// ─── Tests: Ownership Transfer ───────────────────────────────────────

func (s *ImportOperationsTestSuite) TestDeleteOwnedTemplate_TransfersToTarget() {
	tplID := s.createTemplate(s.sourceKey, "transfer_tpl", "Hello {{.name}}")
	s.templateIDs = append(s.templateIDs, tplID)

	time.Sleep(1 * time.Second)

	s.importResources(s.targetAppID, s.sourceAppID, []string{"templates"})
	time.Sleep(1 * time.Second)

	// Source deletes its own template — should transfer to target.
	delStatus := s.deleteTemplate(s.sourceKey, tplID)
	s.Equal(http.StatusNoContent, delStatus, "ownership transfer should return 204")

	time.Sleep(1 * time.Second)

	// Template should now be owned by target.
	s.True(s.esDocExists("templates", tplID), "template should be adopted by target")
	newOwner := s.esGetField("templates", tplID, "app_id")
	s.Equal(s.targetAppID, newOwner, "template should be owned by target after transfer")

	// Link should be cleaned up.
	s.Equal(int64(0), s.esDocCount("app_resource_links", "resource_id", tplID),
		"link should be removed after ownership transfer")

	// Target should still be able to GET and render the template as owner.
	status, _ := s.getTemplate(s.targetKey, tplID)
	s.Equal(http.StatusOK, status, "target should GET transferred template")

	rStatus, rResult := s.renderTemplate(s.targetKey, tplID, map[string]interface{}{"name": "Test"}, false)
	s.Equal(http.StatusOK, rStatus, "target should render transferred template")
	rendered, _ := rResult["rendered_body"].(string)
	s.Contains(rendered, "Hello Test")
}

func (s *ImportOperationsTestSuite) TestDeleteOwnedUser_TransfersToTarget() {
	uid := s.createUser(s.sourceKey, "transfer-user@example.com")
	s.userIDs = append(s.userIDs, uid)

	time.Sleep(1 * time.Second)

	s.importResources(s.targetAppID, s.sourceAppID, []string{"users"})
	time.Sleep(1 * time.Second)

	// Source deletes user.
	delStatus, result := s.deleteUser(s.sourceKey, uid)
	s.Equal(http.StatusOK, delStatus, "ownership transfer should succeed")
	s.True(result["success"].(bool))

	time.Sleep(1 * time.Second)

	// User should now be owned by target.
	s.True(s.esDocExists("users", uid), "user should be adopted by target")
	newOwner := s.esGetField("users", uid, "app_id")
	s.Equal(s.targetAppID, newOwner, "user should be owned by target after transfer")

	// Target should be able to GET user as owner.
	status, _ := s.getUser(s.targetKey, uid)
	s.Equal(http.StatusOK, status, "target should GET transferred user")
}

// ─── Tests: Backward Compatibility {{var}} Syntax ────────────────────

func (s *ImportOperationsTestSuite) TestRenderOwnTemplate_BareVarSyntax() {
	// Source app renders its own template with bare {{var}} — should work.
	tplID := s.createTemplateBareVar(s.sourceKey, "own_bare_var", "Welcome {{name}} to {{place}}")
	s.templateIDs = append(s.templateIDs, tplID)

	time.Sleep(1 * time.Second)

	data := map[string]interface{}{"name": "Dave", "place": "FRN"}
	status, result := s.renderTemplate(s.sourceKey, tplID, data, false)
	s.Equal(http.StatusOK, status, "source should render own template with bare {{var}}")

	rendered, _ := result["rendered_body"].(string)
	s.Contains(rendered, "Welcome Dave", "bare var should be resolved")
	s.Contains(rendered, "to FRN", "bare var should be resolved")
}

func (s *ImportOperationsTestSuite) TestRenderTemplate_MixedVarSyntax() {
	// Template uses both {{.var}} and {{var}} — both should resolve.
	tplID := s.createTemplateMixed(s.sourceKey, "mixed_var_tpl", "Hello {{.name}}, you have {{count}} items")
	s.templateIDs = append(s.templateIDs, tplID)

	time.Sleep(1 * time.Second)

	data := map[string]interface{}{"name": "Eve", "count": "3"}
	status, result := s.renderTemplate(s.sourceKey, tplID, data, false)
	s.Equal(http.StatusOK, status, "mixed syntax render should succeed")

	rendered, _ := result["rendered_body"].(string)
	s.Contains(rendered, "Hello Eve", "dotted var should resolve")
	s.Contains(rendered, "3 items", "bare var should resolve")
}

func (s *ImportOperationsTestSuite) TestRenderTemplate_GoKeywordsPreserved() {
	// Template with Go keywords (if/end) — should NOT be prefixed with dot.
	tplID := s.createTemplate(s.sourceKey, "keyword_tpl",
		`{{if .name}}Hello {{.name}}{{end}}`)
	s.templateIDs = append(s.templateIDs, tplID)

	time.Sleep(1 * time.Second)

	data := map[string]interface{}{"name": "Frank"}
	status, result := s.renderTemplate(s.sourceKey, tplID, data, false)
	s.Equal(http.StatusOK, status, "template with keywords should render fine")

	rendered, _ := result["rendered_body"].(string)
	s.Contains(rendered, "Hello Frank")
}

// ─── Helpers for bare-var template creation ──────────────────────────

// createTemplateBareVar creates a template with bare {{var}} syntax (no dot).
// The API's extractVariables accepts both syntaxes so creation should succeed.
func (s *ImportOperationsTestSuite) createTemplateBareVar(apiKey, name, bodyContent string) string {
	appID := s.appIDForKey(apiKey)
	headers := map[string]string{"Authorization": "Bearer " + apiKey}
	resp, body := s.makeRequest(http.MethodPost, "/v1/templates", map[string]interface{}{
		"app_id":  appID,
		"name":    name,
		"channel": "sse",
		"locale":  "en",
		"body":    bodyContent,
	}, headers)
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "create bare-var template %s: %s", name, string(body))

	var result map[string]interface{}
	s.parseResponse(body, &result)
	return result["id"].(string)
}

// createTemplateMixed creates a template with mixed {{.var}} and {{var}} syntax.
func (s *ImportOperationsTestSuite) createTemplateMixed(apiKey, name, bodyContent string) string {
	return s.createTemplateBareVar(apiKey, name, bodyContent)
}
