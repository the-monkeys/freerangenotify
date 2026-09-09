package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	jwtpkg "github.com/the-monkeys/freerangenotify/pkg/jwt"
)

// BizBillingE2ESuite covers the business-billing module end-to-end:
// authn, feature gate, RBAC (dashboard JWT + API key), core billing flow,
// cross-app isolation, and customer portal token access.
type BizBillingE2ESuite struct {
	IntegrationTestSuite
	productID      string
	planID         string
	subscriptionID string
	invoiceID      string
	billingUserID  string
	createdIndices []string
}

func TestBizBillingE2ESuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	suite.Run(t, new(BizBillingE2ESuite))
}

func (s *BizBillingE2ESuite) SetupSuite() {
	s.IntegrationTestSuite.SetupSuite()
	s.createdIndices = []string{
		"frn_biz_products", "frn_biz_plans", "frn_biz_subscriptions",
		"frn_biz_invoices", "frn_biz_payments", "frn_biz_portal_tokens",
		"frn_biz_configs", "app_memberships",
	}
}

func (s *BizBillingE2ESuite) TearDownSuite() {
	s.cleanupBizIndices()
	s.IntegrationTestSuite.TearDownSuite()
}

func (s *BizBillingE2ESuite) TearDownTest() {
	s.cleanupBizIndices()
	s.IntegrationTestSuite.TearDownTest()
}

func (s *BizBillingE2ESuite) cleanupBizIndices() {
	for _, index := range s.createdIndices {
		req, _ := http.NewRequest(http.MethodPost,
			fmt.Sprintf("%s/%s/_delete_by_query?refresh=true", resolvedESURL(), index),
			bytes.NewBuffer([]byte(`{"query":{"match_all":{}}}`)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.client.Do(req)
		if err == nil && resp != nil {
			resp.Body.Close()
		}
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────

func (s *BizBillingE2ESuite) createApp(name string) (appID, apiKey string) {
	resp, body := s.makeRequest(http.MethodPost, "/v1/apps", map[string]interface{}{
		"app_name": name,
	}, nil)
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "create app: %s", string(body))
	result := s.assertSuccess(body)
	data := result["data"].(map[string]interface{})
	return data["app_id"].(string), data["api_key"].(string)
}

func (s *BizBillingE2ESuite) apiHeaders(apiKey string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + apiKey}
}

func (s *BizBillingE2ESuite) dashboardHeaders(apiKey, jwt string) map[string]string {
	return map[string]string{
		"X-API-Key":     apiKey,
		"Authorization": "Bearer " + jwt,
	}
}

func (s *BizBillingE2ESuite) createBillingUser(apiKey, email string) string {
	headers := s.apiHeaders(apiKey)
	resp, body := s.makeRequest(http.MethodPost, "/v1/users", map[string]interface{}{
		"email": email,
		"billing_address": map[string]interface{}{
			"line1":   "12 MG Road",
			"city":    "Bengaluru",
			"state":   "KA",
			"pincode": "560001",
			"country": "IN",
		},
		"gstin": "29AAAAA0000A1Z5",
		"preferences": map[string]interface{}{
			"channels": []string{"email"},
			"timezone": "Asia/Kolkata",
		},
	}, headers)
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "create billing user: %s", string(body))
	result := s.assertSuccess(body)
	return result["data"].(map[string]interface{})["user_id"].(string)
}

func (s *BizBillingE2ESuite) createProduct(apiKey string) string {
	resp, body := s.makeRequest(http.MethodPost, "/v1/biz/products", map[string]interface{}{
		"name":     "Gym Membership",
		"tax_rate": 18,
		"unit":     "month",
		"hsn_code": "9983",
	}, s.apiHeaders(apiKey))
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "create product: %s", string(body))
	result := s.assertSuccess(body)
	return result["data"].(map[string]interface{})["id"].(string)
}

func (s *BizBillingE2ESuite) createPlan(apiKey, productID string) string {
	resp, body := s.makeRequest(http.MethodPost, "/v1/biz/plans", map[string]interface{}{
		"product_id":    productID,
		"name":          "Monthly 499",
		"amount_paisa":  49900,
		"currency":      "INR",
		"billing_cycle": "monthly",
		"pricing_model": "flat",
		"trial_days":    0,
	}, s.apiHeaders(apiKey))
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "create plan: %s", string(body))
	result := s.assertSuccess(body)
	return result["data"].(map[string]interface{})["id"].(string)
}

func (s *BizBillingE2ESuite) skipIfBizBillingUnavailable(status int, body []byte) {
	s.T().Helper()
	switch status {
	case http.StatusNotFound:
		s.T().Skip("biz billing routes not registered (handler nil / feature disabled at boot)")
	case http.StatusPaymentRequired:
		var result map[string]interface{}
		_ = json.Unmarshal(body, &result)
		if code, _ := result["code"].(string); code == "billing_module_required" {
			s.T().Skip("biz billing feature flag is off (FREERANGE_FEATURES_BIZ_BILLING_ENABLED=false)")
		}
	}
}

func (s *BizBillingE2ESuite) seedAdminUser(userID, email string) {
	doc := map[string]interface{}{
		"user_id":        userID,
		"email":          email,
		"password_hash":  "$2a$10$placeholderhashnotusedforjwtpath",
		"full_name":      "Biz Billing Viewer",
		"phone_verified": false,
		"is_active":      true,
		"created_at":     time.Now().UTC().Format(time.RFC3339),
		"updated_at":     time.Now().UTC().Format(time.RFC3339),
	}
	payload, _ := json.Marshal(doc)
	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/auth_users/_doc/%s?refresh=true", resolvedESURL(), userID),
		bytes.NewReader(payload))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Require().True(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated,
		"seed auth user failed: %d", resp.StatusCode)
}

func (s *BizBillingE2ESuite) seedMembership(appID, userID, email, role string) {
	membershipID := uuid.New().String()
	doc := map[string]interface{}{
		"membership_id": membershipID,
		"app_id":        appID,
		"user_id":       userID,
		"user_email":    email,
		"role":          role,
		"invited_by":    "integration-test",
		"created_at":    time.Now().UTC().Format(time.RFC3339),
		"updated_at":    time.Now().UTC().Format(time.RFC3339),
	}
	payload, _ := json.Marshal(doc)
	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/app_memberships/_doc/%s?refresh=true", resolvedESURL(), membershipID),
		bytes.NewReader(payload))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	s.Require().True(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated,
		"seed membership failed: %d", resp.StatusCode)
}

func (s *BizBillingE2ESuite) mintJWT(userID, email string) string {
	secret := strings.TrimSpace(os.Getenv("FREERANGE_SECURITY_JWT_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("JWT_SECRET"))
	}
	if secret == "" {
		s.T().Skip("JWT secret not available; cannot mint dashboard tokens for RBAC tests")
	}
	mgr := jwtpkg.NewManager(secret, time.Hour, 24*time.Hour)
	token, _, err := mgr.GenerateAccessToken(userID, email)
	s.Require().NoError(err)
	return token
}

// ─── Authn ───────────────────────────────────────────────────────────────

func (s *BizBillingE2ESuite) TestAuth_NoAPIKeyRejected() {
	resp, body := s.makeRequest(http.MethodGet, "/v1/biz/products", nil, nil)
	s.skipIfBizBillingUnavailable(resp.StatusCode, body)
	s.Equal(http.StatusUnauthorized, resp.StatusCode, string(body))
}

func (s *BizBillingE2ESuite) TestAuth_InvalidAPIKeyRejected() {
	headers := map[string]string{"Authorization": "Bearer frn_invalid_key"}
	resp, body := s.makeRequest(http.MethodGet, "/v1/biz/products", nil, headers)
	s.skipIfBizBillingUnavailable(resp.StatusCode, body)
	s.Equal(http.StatusUnauthorized, resp.StatusCode, string(body))
}

// ─── Core happy path (API key = app owner) ───────────────────────────────

func (s *BizBillingE2ESuite) TestE2E_ProductPlanSubscriptionInvoicePayment() {
	appID, apiKey := s.createApp("Biz Billing E2E App")
	s.appID = appID
	s.apiKey = apiKey

	// Probe module availability early.
	probe, probeBody := s.makeRequest(http.MethodGet, "/v1/biz/products", nil, s.apiHeaders(apiKey))
	s.skipIfBizBillingUnavailable(probe.StatusCode, probeBody)
	s.Require().Equal(http.StatusOK, probe.StatusCode, string(probeBody))

	s.productID = s.createProduct(apiKey)
	s.planID = s.createPlan(apiKey, s.productID)
	s.billingUserID = s.createBillingUser(apiKey, fmt.Sprintf("biz-customer-%s@example.com", uuid.NewString()[:8]))
	s.userID = s.billingUserID

	// Create subscription → auto-issues first invoice (non-trialing).
	resp, body := s.makeRequest(http.MethodPost, "/v1/biz/subscriptions", map[string]interface{}{
		"user_id":  s.billingUserID,
		"plan_id":  s.planID,
		"quantity": 1,
	}, s.apiHeaders(apiKey))
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "create subscription: %s", string(body))
	subResult := s.assertSuccess(body)
	sub := subResult["data"].(map[string]interface{})
	s.subscriptionID = sub["id"].(string)
	s.Equal("active", sub["status"])

	// List invoices for user — expect exactly one open invoice.
	resp, body = s.makeRequest(http.MethodGet,
		fmt.Sprintf("/v1/biz/invoices?user_id=%s", s.billingUserID),
		nil, s.apiHeaders(apiKey))
	s.Require().Equal(http.StatusOK, resp.StatusCode, "list invoices: %s", string(body))
	invResult := s.assertSuccess(body)
	invoices, ok := invResult["data"].([]interface{})
	s.Require().True(ok && len(invoices) >= 1, "expected at least 1 invoice, got: %s", string(body))
	inv := invoices[0].(map[string]interface{})
	s.invoiceID = inv["id"].(string)
	s.Equal("open", inv["status"], "first invoice should be open after issue")
	s.Greater(inv["total_paisa"].(float64), float64(0))

	// Record full manual payment.
	resp, body = s.makeRequest(http.MethodPost,
		fmt.Sprintf("/v1/biz/invoices/%s/payments", s.invoiceID),
		map[string]interface{}{
			"method": "cash",
			"notes":  "integration test payment",
		}, s.apiHeaders(apiKey))
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "record payment: %s", string(body))
	payResult := s.assertSuccess(body)
	payment := payResult["data"].(map[string]interface{})
	s.Equal("success", payment["status"])

	// Invoice should now be paid.
	resp, body = s.makeRequest(http.MethodGet,
		fmt.Sprintf("/v1/biz/invoices/%s", s.invoiceID),
		nil, s.apiHeaders(apiKey))
	s.Require().Equal(http.StatusOK, resp.StatusCode, string(body))
	got := s.assertSuccess(body)["data"].(map[string]interface{})
	s.Equal("paid", got["status"])
}

// ─── Cross-app isolation ─────────────────────────────────────────────────

func (s *BizBillingE2ESuite) TestIsolation_AppBCannotReadAppAResources() {
	appAID, keyA := s.createApp("Biz Isolation A")
	appBID, keyB := s.createApp("Biz Isolation B")
	s.appID = appAID
	s.secondAppID = appBID
	s.apiKey = keyA

	probe, probeBody := s.makeRequest(http.MethodGet, "/v1/biz/products", nil, s.apiHeaders(keyA))
	s.skipIfBizBillingUnavailable(probe.StatusCode, probeBody)

	productID := s.createProduct(keyA)

	// App B listing products must not include App A's product.
	resp, body := s.makeRequest(http.MethodGet, "/v1/biz/products", nil, s.apiHeaders(keyB))
	s.Require().Equal(http.StatusOK, resp.StatusCode, string(body))
	result := s.assertSuccess(body)
	products, _ := result["data"].([]interface{})
	for _, p := range products {
		item := p.(map[string]interface{})
		s.NotEqual(productID, item["id"], "app B leaked app A product")
	}

	// Direct GET by ID with App B key must 404.
	resp, body = s.makeRequest(http.MethodGet, "/v1/biz/products/"+productID, nil, s.apiHeaders(keyB))
	s.True(resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden,
		"expected 404/403 for cross-app get, got %d: %s", resp.StatusCode, string(body))
}

// ─── Dashboard RBAC ──────────────────────────────────────────────────────

func (s *BizBillingE2ESuite) TestRBAC_ViewerCannotWriteBilling() {
	appID, apiKey := s.createApp("Biz RBAC Viewer App")
	s.appID = appID
	s.apiKey = apiKey

	probe, probeBody := s.makeRequest(http.MethodGet, "/v1/biz/products", nil, s.apiHeaders(apiKey))
	s.skipIfBizBillingUnavailable(probe.StatusCode, probeBody)

	viewerID := uuid.New().String()
	viewerEmail := fmt.Sprintf("viewer-%s@example.com", viewerID[:8])
	s.seedAdminUser(viewerID, viewerEmail)
	s.seedMembership(appID, viewerID, viewerEmail, "viewer")
	viewerJWT := s.mintJWT(viewerID, viewerEmail)

	// Viewer write via dashboard headers must be forbidden.
	resp, body := s.makeRequest(http.MethodPost, "/v1/biz/products", map[string]interface{}{
		"name":     "Should Fail",
		"tax_rate": 18,
	}, s.dashboardHeaders(apiKey, viewerJWT))
	s.Equal(http.StatusForbidden, resp.StatusCode, "viewer write: %s", string(body))

	// Viewer read must still succeed.
	resp, body = s.makeRequest(http.MethodGet, "/v1/biz/products", nil, s.dashboardHeaders(apiKey, viewerJWT))
	s.Equal(http.StatusOK, resp.StatusCode, "viewer read: %s", string(body))
}

func (s *BizBillingE2ESuite) TestRBAC_EditorCanWriteBilling() {
	appID, apiKey := s.createApp("Biz RBAC Editor App")
	s.appID = appID
	s.apiKey = apiKey

	probe, probeBody := s.makeRequest(http.MethodGet, "/v1/biz/products", nil, s.apiHeaders(apiKey))
	s.skipIfBizBillingUnavailable(probe.StatusCode, probeBody)

	editorID := uuid.New().String()
	editorEmail := fmt.Sprintf("editor-%s@example.com", editorID[:8])
	s.seedAdminUser(editorID, editorEmail)
	s.seedMembership(appID, editorID, editorEmail, "editor")
	editorJWT := s.mintJWT(editorID, editorEmail)

	resp, body := s.makeRequest(http.MethodPost, "/v1/biz/products", map[string]interface{}{
		"name":     "Editor Product",
		"tax_rate": 18,
	}, s.dashboardHeaders(apiKey, editorJWT))
	s.Equal(http.StatusCreated, resp.StatusCode, "editor write: %s", string(body))
	s.assertSuccess(body)
}

func (s *BizBillingE2ESuite) TestRBAC_NonMemberDeniedEvenWithValidJWT() {
	appID, apiKey := s.createApp("Biz RBAC NonMember App")
	s.appID = appID
	s.apiKey = apiKey

	probe, probeBody := s.makeRequest(http.MethodGet, "/v1/biz/products", nil, s.apiHeaders(apiKey))
	s.skipIfBizBillingUnavailable(probe.StatusCode, probeBody)

	strangerID := uuid.New().String()
	strangerEmail := fmt.Sprintf("stranger-%s@example.com", strangerID[:8])
	s.seedAdminUser(strangerID, strangerEmail)
	// Intentionally no membership.
	strangerJWT := s.mintJWT(strangerID, strangerEmail)

	resp, body := s.makeRequest(http.MethodGet, "/v1/biz/products", nil, s.dashboardHeaders(apiKey, strangerJWT))
	s.Equal(http.StatusForbidden, resp.StatusCode, "non-member: %s", string(body))
}

// ─── Customer portal ─────────────────────────────────────────────────────

func (s *BizBillingE2ESuite) TestPortal_TokenAccessAndInvalidToken() {
	appID, apiKey := s.createApp("Biz Portal App")
	s.appID = appID
	s.apiKey = apiKey

	probe, probeBody := s.makeRequest(http.MethodGet, "/v1/biz/products", nil, s.apiHeaders(apiKey))
	s.skipIfBizBillingUnavailable(probe.StatusCode, probeBody)

	userID := s.createBillingUser(apiKey, fmt.Sprintf("portal-%s@example.com", uuid.NewString()[:8]))
	s.userID = userID
	s.billingUserID = userID

	resp, body := s.makeRequest(http.MethodPost,
		fmt.Sprintf("/v1/biz/users/%s/portal-link", userID),
		map[string]interface{}{}, s.apiHeaders(apiKey))
	s.Require().Equal(http.StatusCreated, resp.StatusCode, "portal-link: %s", string(body))
	linkResult := s.assertSuccess(body)
	urlStr, _ := linkResult["data"].(map[string]interface{})["url"].(string)
	s.Require().NotEmpty(urlStr, "portal url empty — check FREERANGE_BIZ_BILLING_PORTAL_BASE_URL / config portal_base_url")

	// Extract token from .../v1/portal/{token}
	parts := strings.Split(strings.TrimRight(urlStr, "/"), "/")
	token := parts[len(parts)-1]
	s.Require().NotEmpty(token)

	// Valid token overview.
	resp, body = s.makeRequest(http.MethodGet, "/v1/portal/"+token, nil, nil)
	if resp.StatusCode == http.StatusNotFound {
		// Trailing-slash variant if StrictRouting is on.
		resp, body = s.makeRequest(http.MethodGet, "/v1/portal/"+token+"/", nil, nil)
	}
	s.Require().Equal(http.StatusOK, resp.StatusCode, "portal overview: %s", string(body))

	// Invalid token.
	resp, body = s.makeRequest(http.MethodGet, "/v1/portal/not-a-real-token", nil, nil)
	s.True(resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound,
		"invalid portal token: %d %s", resp.StatusCode, string(body))
}

// ─── Config smoke ────────────────────────────────────────────────────────

func (s *BizBillingE2ESuite) TestConfig_TaxRoundTrip() {
	appID, apiKey := s.createApp("Biz Config App")
	s.appID = appID
	s.apiKey = apiKey

	probe, probeBody := s.makeRequest(http.MethodGet, "/v1/biz/config/tax", nil, s.apiHeaders(apiKey))
	s.skipIfBizBillingUnavailable(probe.StatusCode, probeBody)

	resp, body := s.makeRequest(http.MethodPut, "/v1/biz/config/tax", map[string]interface{}{
		"default_tax_rate": 18,
		"business_state":   "KA",
		"tax_inclusive":    false,
		"business_gstin":   "29AAAAA0000A1Z5",
	}, s.apiHeaders(apiKey))
	s.Require().Equal(http.StatusOK, resp.StatusCode, "put tax config: %s", string(body))

	resp, body = s.makeRequest(http.MethodGet, "/v1/biz/config/tax", nil, s.apiHeaders(apiKey))
	s.Require().Equal(http.StatusOK, resp.StatusCode, string(body))
	got := s.assertSuccess(body)["data"].(map[string]interface{})
	// Config may be returned as typed fields on data, or nested under data.data.
	cfg := got
	if nested, ok := got["data"].(map[string]interface{}); ok {
		cfg = nested
	}
	s.Equal(float64(18), cfg["default_tax_rate"])
	s.Equal("KA", cfg["business_state"])
}
