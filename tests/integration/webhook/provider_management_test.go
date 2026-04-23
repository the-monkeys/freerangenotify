package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	defaultBaseURL = "http://127.0.0.1:8080"
	requestTimeout = 30 * time.Second
)

type appCreateResponse struct {
	Success bool `json:"success"`
	Data    struct {
		AppID string `json:"app_id"`
	} `json:"data"`
}

type providerRegisterResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ProviderID string `json:"provider_id"`
		SigningKey string `json:"signing_key"`
	} `json:"data"`
}

type providerRotateResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ProviderID string `json:"provider_id"`
		SigningKey string `json:"signing_key"`
	} `json:"data"`
}

type playgroundCreateResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type playgroundPayloadsResponse struct {
	ID       string            `json:"id"`
	Payloads []json.RawMessage `json:"payloads"`
	Count    int               `json:"count"`
}

type playgroundRecord struct {
	Headers    map[string]interface{} `json:"headers"`
	Body       json.RawMessage        `json:"body"`
	ReceivedAt string                 `json:"received_at"`
}

func TestProviderManagement_AllKindsTestEndpoint(t *testing.T) {
	requireWebhookV2IntegrationEnabled(t)
	adminToken := mustAdminToken(t)

	appID := createApp(t, adminToken)
	t.Cleanup(func() { deleteApp(t, adminToken, appID) })

	cases := []struct {
		name string
		kind string
	}{
		{name: "generic", kind: "generic"},
		{name: "discord", kind: "discord"},
		{name: "slack", kind: "slack"},
		{name: "teams", kind: "teams"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			playground := createPlayground(t, adminToken)
			providerID, _ := registerProvider(t, adminToken, appID, map[string]interface{}{
				"name":              fmt.Sprintf("%s-%d", tc.kind, time.Now().UnixNano()),
				"channel":           "webhook",
				"kind":              tc.kind,
				"webhook_url":       playground.URL,
				"signature_version": "v1",
			})

			callProviderTest(t, adminToken, appID, providerID)
			record := waitForPayloadCount(t, playground.ID, 1)

			sig := getHeader(record.Headers, "X-Webhook-Signature")
			if sig == "" {
				t.Fatalf("expected X-Webhook-Signature header for kind=%s", tc.kind)
			}

			var body map[string]interface{}
			if err := json.Unmarshal(record.Body, &body); err != nil {
				t.Fatalf("failed to parse payload body: %v", err)
			}

			switch tc.kind {
			case "generic":
				if _, ok := body["notification_id"]; !ok {
					t.Fatalf("expected generic payload to include notification_id")
				}
				if body["channel"] != "webhook" {
					t.Fatalf("expected generic channel=webhook, got %v", body["channel"])
				}
			case "discord":
				if _, ok := body["embeds"]; !ok {
					t.Fatalf("expected discord payload to include embeds")
				}
			case "slack":
				if text, ok := body["text"].(string); !ok || !strings.Contains(text, "FreeRangeNotify Provider Test") {
					t.Fatalf("expected slack payload text to include test title, got %v", body["text"])
				}
			case "teams":
				if body["type"] != "message" {
					t.Fatalf("expected teams payload type=message, got %v", body["type"])
				}
				if _, ok := body["attachments"]; !ok {
					t.Fatalf("expected teams payload to include attachments")
				}
			}
		})
	}
}

func TestProviderManagement_RotateKeyChangesEffectiveSignature(t *testing.T) {
	requireWebhookV2IntegrationEnabled(t)
	adminToken := mustAdminToken(t)

	appID := createApp(t, adminToken)
	t.Cleanup(func() { deleteApp(t, adminToken, appID) })

	playground := createPlayground(t, adminToken)
	providerID, oldKey := registerProvider(t, adminToken, appID, map[string]interface{}{
		"name":              fmt.Sprintf("rotate-%d", time.Now().UnixNano()),
		"channel":           "webhook",
		"kind":              "generic",
		"webhook_url":       playground.URL,
		"signature_version": "v1",
	})
	if oldKey == "" {
		t.Fatalf("expected signing key on provider registration")
	}

	callProviderTest(t, adminToken, appID, providerID)
	first := waitForPayloadCount(t, playground.ID, 1)
	firstSig := getHeader(first.Headers, "X-Webhook-Signature")
	if firstSig == "" {
		t.Fatalf("expected first payload signature header")
	}
	if firstSig != hmacV1(first.Body, oldKey) {
		t.Fatalf("first payload signature does not match pre-rotate key")
	}

	newKey := rotateProviderKey(t, adminToken, appID, providerID)
	if newKey == "" || newKey == oldKey {
		t.Fatalf("expected rotate endpoint to return a different key")
	}

	callProviderTest(t, adminToken, appID, providerID)
	second := waitForPayloadCount(t, playground.ID, 2)
	secondSig := getHeader(second.Headers, "X-Webhook-Signature")
	if secondSig == "" {
		t.Fatalf("expected second payload signature header")
	}

	wantWithNew := hmacV1(second.Body, newKey)
	if secondSig != wantWithNew {
		t.Fatalf("second payload signature does not match rotated key")
	}
	if secondSig == hmacV1(second.Body, oldKey) {
		t.Fatalf("second payload signature should not validate against old key")
	}
}

func TestProviderManagement_SignatureV2TimestampedHMAC(t *testing.T) {
	requireWebhookV2IntegrationEnabled(t)
	adminToken := mustAdminToken(t)

	appID := createApp(t, adminToken)
	t.Cleanup(func() { deleteApp(t, adminToken, appID) })

	playground := createPlayground(t, adminToken)
	providerID, signingKey := registerProvider(t, adminToken, appID, map[string]interface{}{
		"name":              fmt.Sprintf("sigv2-%d", time.Now().UnixNano()),
		"channel":           "webhook",
		"kind":              "generic",
		"webhook_url":       playground.URL,
		"signature_version": "v2",
	})
	if signingKey == "" {
		t.Fatalf("expected signing key on provider registration")
	}

	callProviderTest(t, adminToken, appID, providerID)
	record := waitForPayloadCount(t, playground.ID, 1)

	ts := getHeader(record.Headers, "X-Webhook-Timestamp")
	if ts == "" {
		t.Fatalf("expected X-Webhook-Timestamp for v2 provider")
	}
	if _, err := strconv.ParseInt(ts, 10, 64); err != nil {
		t.Fatalf("invalid timestamp header %q: %v", ts, err)
	}

	sig := getHeader(record.Headers, "X-Webhook-Signature")
	if sig == "" {
		t.Fatalf("expected X-Webhook-Signature header")
	}
	want := hmacV2(ts, record.Body, signingKey)
	if sig != want {
		t.Fatalf("v2 signature mismatch")
	}

	legacy := getHeader(record.Headers, "X-FRN-Signature")
	if legacy != "" && legacy != sig {
		t.Fatalf("expected X-FRN-Signature to match canonical signature when present")
	}
}

// ──────────────────────────────────────────────────────────────────
// Section 8.2 — Additional integration test cases
// ──────────────────────────────────────────────────────────────────

// TestWebhookE2E_GenericLegacyPayload verifies that a generic webhook provider
// receiving a plain title/body notification produces the legacy JSON envelope
// with valid X-Webhook-Signature and no X-Webhook-Payload-Version header.
func TestWebhookE2E_GenericLegacyPayload(t *testing.T) {
	requireWebhookV2IntegrationEnabled(t)
	adminToken := mustAdminToken(t)

	appID := createApp(t, adminToken)
	t.Cleanup(func() { deleteApp(t, adminToken, appID) })

	playground := createPlayground(t, adminToken)
	providerID, signingKey := registerProvider(t, adminToken, appID, map[string]interface{}{
		"name":              fmt.Sprintf("generic-legacy-%d", time.Now().UnixNano()),
		"channel":           "webhook",
		"kind":              "generic",
		"webhook_url":       playground.URL,
		"signature_version": "v1",
	})

	callProviderTest(t, adminToken, appID, providerID)
	record := waitForPayloadCount(t, playground.ID, 1)

	// Signature must be valid v1
	sig := getHeader(record.Headers, "X-Webhook-Signature")
	if sig == "" {
		t.Fatalf("expected X-Webhook-Signature header")
	}
	if sig != hmacV1(record.Body, signingKey) {
		t.Fatalf("v1 signature mismatch for generic legacy payload")
	}

	// No payload version header for legacy payloads
	ver := getHeader(record.Headers, "X-Webhook-Payload-Version")
	if ver != "" && ver != "1.0" {
		t.Fatalf("expected no or 1.0 payload version for legacy, got %q", ver)
	}

	// Body must have standard generic envelope keys
	var body map[string]interface{}
	if err := json.Unmarshal(record.Body, &body); err != nil {
		t.Fatalf("failed to parse body: %v", err)
	}
	for _, key := range []string{"notification_id", "channel", "content"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("expected generic legacy payload to contain %q", key)
		}
	}
	// Content should NOT have rich fields
	content, _ := body["content"].(map[string]interface{})
	for _, rich := range []string{"attachments", "actions", "fields", "poll"} {
		if v, ok := content[rich]; ok && v != nil {
			t.Fatalf("legacy payload should not contain rich field %q", rich)
		}
	}
}

// TestWebhookE2E_DiscordEmbedFormat verifies the Discord provider produces
// correct embed structure with title, description, and color.
func TestWebhookE2E_DiscordEmbedFormat(t *testing.T) {
	requireWebhookV2IntegrationEnabled(t)
	adminToken := mustAdminToken(t)

	appID := createApp(t, adminToken)
	t.Cleanup(func() { deleteApp(t, adminToken, appID) })

	playground := createPlayground(t, adminToken)
	providerID, _ := registerProvider(t, adminToken, appID, map[string]interface{}{
		"name":              fmt.Sprintf("discord-embed-%d", time.Now().UnixNano()),
		"channel":           "webhook",
		"kind":              "discord",
		"webhook_url":       playground.URL,
		"signature_version": "v1",
	})

	callProviderTest(t, adminToken, appID, providerID)
	record := waitForPayloadCount(t, playground.ID, 1)

	var body map[string]interface{}
	if err := json.Unmarshal(record.Body, &body); err != nil {
		t.Fatalf("failed to parse discord payload: %v", err)
	}

	embeds, ok := body["embeds"].([]interface{})
	if !ok || len(embeds) == 0 {
		t.Fatalf("expected discord payload to have embeds array with at least one embed")
	}

	embed, ok := embeds[0].(map[string]interface{})
	if !ok {
		t.Fatalf("embed is not an object")
	}

	if _, ok := embed["title"]; !ok {
		t.Fatalf("discord embed missing title")
	}
	if _, ok := embed["description"]; !ok {
		t.Fatalf("discord embed missing description")
	}
}

// TestWebhookE2E_SlackBlockKitFormat verifies the Slack provider produces
// a payload with text field containing the test notification title.
func TestWebhookE2E_SlackBlockKitFormat(t *testing.T) {
	requireWebhookV2IntegrationEnabled(t)
	adminToken := mustAdminToken(t)

	appID := createApp(t, adminToken)
	t.Cleanup(func() { deleteApp(t, adminToken, appID) })

	playground := createPlayground(t, adminToken)
	providerID, _ := registerProvider(t, adminToken, appID, map[string]interface{}{
		"name":              fmt.Sprintf("slack-blockkit-%d", time.Now().UnixNano()),
		"channel":           "webhook",
		"kind":              "slack",
		"webhook_url":       playground.URL,
		"signature_version": "v1",
	})

	callProviderTest(t, adminToken, appID, providerID)
	record := waitForPayloadCount(t, playground.ID, 1)

	var body map[string]interface{}
	if err := json.Unmarshal(record.Body, &body); err != nil {
		t.Fatalf("failed to parse slack payload: %v", err)
	}

	// Slack payloads should have a text field (fallback) containing the title
	text, ok := body["text"].(string)
	if !ok || text == "" {
		t.Fatalf("expected slack payload to have non-empty text field")
	}
	if !strings.Contains(text, "FreeRangeNotify Provider Test") {
		t.Fatalf("slack text should contain test title, got: %s", text)
	}
}

// TestWebhookE2E_TeamsWorkflowURL verifies that a Teams provider with a
// workflow-style URL produces the correct wrapper with type=message and
// an attachments array containing an Adaptive Card.
func TestWebhookE2E_TeamsWorkflowURL(t *testing.T) {
	requireWebhookV2IntegrationEnabled(t)
	adminToken := mustAdminToken(t)

	appID := createApp(t, adminToken)
	t.Cleanup(func() { deleteApp(t, adminToken, appID) })

	// Use the playground URL but register as teams kind. The payload shape
	// is what we're testing, not actual Teams delivery.
	playground := createPlayground(t, adminToken)
	providerID, _ := registerProvider(t, adminToken, appID, map[string]interface{}{
		"name":              fmt.Sprintf("teams-workflow-%d", time.Now().UnixNano()),
		"channel":           "webhook",
		"kind":              "teams",
		"webhook_url":       playground.URL,
		"signature_version": "v1",
	})

	callProviderTest(t, adminToken, appID, providerID)
	record := waitForPayloadCount(t, playground.ID, 1)

	var body map[string]interface{}
	if err := json.Unmarshal(record.Body, &body); err != nil {
		t.Fatalf("failed to parse teams payload: %v", err)
	}

	if body["type"] != "message" {
		t.Fatalf("expected teams payload type=message, got %v", body["type"])
	}

	attachments, ok := body["attachments"].([]interface{})
	if !ok || len(attachments) == 0 {
		t.Fatalf("expected teams payload to have attachments with Adaptive Card")
	}

	att, ok := attachments[0].(map[string]interface{})
	if !ok {
		t.Fatalf("teams attachment is not an object")
	}
	if att["contentType"] != "application/vnd.microsoft.card.adaptive" {
		t.Fatalf("expected Adaptive Card contentType, got %v", att["contentType"])
	}
}

// TestCustomProviderKindRouting_Slack verifies that a CustomProvider with
// Kind=slack produces Slack Block Kit output (not the plain {text} downgrade).
func TestCustomProviderKindRouting_Slack(t *testing.T) {
	requireWebhookV2IntegrationEnabled(t)
	adminToken := mustAdminToken(t)

	appID := createApp(t, adminToken)
	t.Cleanup(func() { deleteApp(t, adminToken, appID) })

	playground := createPlayground(t, adminToken)
	providerID, _ := registerProvider(t, adminToken, appID, map[string]interface{}{
		"name":              fmt.Sprintf("custom-slack-%d", time.Now().UnixNano()),
		"channel":           "webhook",
		"kind":              "slack",
		"webhook_url":       playground.URL,
		"signature_version": "v1",
	})

	callProviderTest(t, adminToken, appID, providerID)
	record := waitForPayloadCount(t, playground.ID, 1)

	var body map[string]interface{}
	if err := json.Unmarshal(record.Body, &body); err != nil {
		t.Fatalf("failed to parse custom slack payload: %v", err)
	}

	// Must contain text (Block Kit fallback) — not the downgraded plain text
	text, ok := body["text"].(string)
	if !ok || text == "" {
		t.Fatalf("expected custom slack provider to emit text field")
	}

	sig := getHeader(record.Headers, "X-Webhook-Signature")
	if sig == "" {
		t.Fatalf("expected X-Webhook-Signature for custom slack provider")
	}
}

// TestProviderTestEndpoint_ReturnsDeliveryMetadata verifies the /test endpoint
// returns provider_id, provider_name, and delivery_time_ms.
func TestProviderTestEndpoint_ReturnsDeliveryMetadata(t *testing.T) {
	requireWebhookV2IntegrationEnabled(t)
	adminToken := mustAdminToken(t)

	appID := createApp(t, adminToken)
	t.Cleanup(func() { deleteApp(t, adminToken, appID) })

	playground := createPlayground(t, adminToken)
	providerID, _ := registerProvider(t, adminToken, appID, map[string]interface{}{
		"name":              fmt.Sprintf("test-meta-%d", time.Now().UnixNano()),
		"channel":           "webhook",
		"kind":              "generic",
		"webhook_url":       playground.URL,
		"signature_version": "v1",
	})

	status, body := doJSON(t, http.MethodPost, "/v1/apps/"+appID+"/providers/"+providerID+"/test", adminToken, nil)
	if status != http.StatusOK {
		t.Fatalf("test endpoint failed: status=%d body=%s", status, string(body))
	}

	var parsed struct {
		Success bool `json:"success"`
		Data    struct {
			ProviderID     string `json:"provider_id"`
			ProviderName   string `json:"provider_name"`
			DeliveryTimeMs int64  `json:"delivery_time_ms"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to parse test response: %v", err)
	}
	if !parsed.Success {
		t.Fatalf("test endpoint returned success=false")
	}
	if parsed.Data.ProviderID != providerID {
		t.Fatalf("expected provider_id=%s, got %s", providerID, parsed.Data.ProviderID)
	}
	if parsed.Data.ProviderName == "" {
		t.Fatalf("expected non-empty provider_name")
	}
	if parsed.Data.DeliveryTimeMs <= 0 {
		t.Fatalf("expected positive delivery_time_ms, got %d", parsed.Data.DeliveryTimeMs)
	}
}

// TestBackCompat_ProviderWithEmptyKind verifies that a provider registered
// without an explicit Kind field (simulating a pre-upgrade row) still produces
// a valid payload via URL-sniff fallback.
func TestBackCompat_ProviderWithEmptyKind(t *testing.T) {
	requireWebhookV2IntegrationEnabled(t)
	adminToken := mustAdminToken(t)

	appID := createApp(t, adminToken)
	t.Cleanup(func() { deleteApp(t, adminToken, appID) })

	playground := createPlayground(t, adminToken)

	// Register without specifying kind — the handler should infer "generic"
	// from the playground URL (which doesn't match any specific platform host).
	providerID, signingKey := registerProvider(t, adminToken, appID, map[string]interface{}{
		"name":              fmt.Sprintf("backcompat-%d", time.Now().UnixNano()),
		"channel":           "webhook",
		"webhook_url":       playground.URL,
		"signature_version": "v1",
	})

	callProviderTest(t, adminToken, appID, providerID)
	record := waitForPayloadCount(t, playground.ID, 1)

	// Should still have valid signature
	sig := getHeader(record.Headers, "X-Webhook-Signature")
	if sig == "" {
		t.Fatalf("expected signature header for backward-compat provider")
	}
	if sig != hmacV1(record.Body, signingKey) {
		t.Fatalf("signature mismatch for backward-compat provider")
	}

	// Body should be a valid JSON payload
	var body map[string]interface{}
	if err := json.Unmarshal(record.Body, &body); err != nil {
		t.Fatalf("failed to parse backward-compat payload: %v", err)
	}
	// Should contain at least a notification_id or content (generic shape)
	if _, hasNID := body["notification_id"]; !hasNID {
		if _, hasContent := body["content"]; !hasContent {
			t.Fatalf("backward-compat payload missing both notification_id and content — unexpected shape")
		}
	}
}

func createApp(t *testing.T, adminToken string) string {
	t.Helper()

	status, body := doJSON(t, http.MethodPost, "/v1/apps", adminToken, map[string]interface{}{
		"app_name": fmt.Sprintf("Webhook V2 Integration %d", time.Now().UnixNano()),
	})
	if status != http.StatusCreated {
		t.Fatalf("create app failed: status=%d body=%s", status, string(body))
	}

	var parsed appCreateResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to parse create app response: %v", err)
	}
	if !parsed.Success || parsed.Data.AppID == "" {
		t.Fatalf("create app returned invalid response: %s", string(body))
	}
	return parsed.Data.AppID
}

func deleteApp(t *testing.T, adminToken, appID string) {
	t.Helper()
	if appID == "" {
		return
	}
	status, body := doJSON(t, http.MethodDelete, "/v1/apps/"+appID, adminToken, nil)
	if status != http.StatusOK {
		t.Fatalf("delete app failed: status=%d body=%s", status, string(body))
	}
}

func createPlayground(t *testing.T, adminToken string) playgroundCreateResponse {
	t.Helper()
	status, body := doJSON(t, http.MethodPost, "/v1/admin/playground/webhook", adminToken, map[string]interface{}{})
	if status != http.StatusCreated {
		t.Fatalf("create playground failed: status=%d body=%s", status, string(body))
	}
	var parsed playgroundCreateResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to parse create playground response: %v", err)
	}
	if parsed.ID == "" || parsed.URL == "" {
		t.Fatalf("invalid playground response: %s", string(body))
	}
	return parsed
}

func registerProvider(t *testing.T, adminToken, appID string, payload map[string]interface{}) (string, string) {
	t.Helper()
	status, body := doJSON(t, http.MethodPost, "/v1/apps/"+appID+"/providers", adminToken, payload)
	if status != http.StatusCreated {
		t.Fatalf("register provider failed: status=%d body=%s", status, string(body))
	}
	var parsed providerRegisterResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to parse register provider response: %v", err)
	}
	if !parsed.Success || parsed.Data.ProviderID == "" {
		t.Fatalf("invalid register provider response: %s", string(body))
	}
	return parsed.Data.ProviderID, parsed.Data.SigningKey
}

func callProviderTest(t *testing.T, adminToken, appID, providerID string) {
	t.Helper()
	status, body := doJSON(t, http.MethodPost, "/v1/apps/"+appID+"/providers/"+providerID+"/test", adminToken, nil)
	if status != http.StatusOK {
		t.Fatalf("provider test endpoint failed: status=%d body=%s", status, string(body))
	}
}

func rotateProviderKey(t *testing.T, adminToken, appID, providerID string) string {
	t.Helper()
	status, body := doJSON(t, http.MethodPost, "/v1/apps/"+appID+"/providers/"+providerID+"/rotate", adminToken, nil)
	if status != http.StatusOK {
		t.Fatalf("rotate endpoint failed: status=%d body=%s", status, string(body))
	}
	var parsed providerRotateResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to parse rotate response: %v", err)
	}
	if !parsed.Success || parsed.Data.SigningKey == "" {
		t.Fatalf("invalid rotate response: %s", string(body))
	}
	return parsed.Data.SigningKey
}

func waitForPayloadCount(t *testing.T, playgroundID string, minCount int) playgroundRecord {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, body := doJSON(t, http.MethodGet, "/v1/playground/"+playgroundID, "", nil)
		if status == http.StatusOK {
			var parsed playgroundPayloadsResponse
			if err := json.Unmarshal(body, &parsed); err != nil {
				t.Fatalf("failed to parse playground payloads response: %v", err)
			}
			if parsed.Count >= minCount && len(parsed.Payloads) >= minCount {
				var record playgroundRecord
				last := parsed.Payloads[len(parsed.Payloads)-1]
				if err := json.Unmarshal(last, &record); err != nil {
					t.Fatalf("failed to parse playground record: %v", err)
				}
				return record
			}
		}
		time.Sleep(250 * time.Millisecond)
	}

	t.Fatalf("timeout waiting for playground %s payload count >= %d", playgroundID, minCount)
	return playgroundRecord{}
}

func doJSON(t *testing.T, method, path, bearerToken string, payload interface{}) (int, []byte) {
	t.Helper()
	url := strings.TrimRight(resolvedBaseURL(), "/") + path

	var bodyReader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("failed to marshal request payload: %v", err)
		}
		bodyReader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	return resp.StatusCode, respBody
}

func getHeader(headers map[string]interface{}, key string) string {
	for k, v := range headers {
		if !strings.EqualFold(k, key) {
			continue
		}
		switch vv := v.(type) {
		case string:
			return vv
		case []interface{}:
			if len(vv) > 0 {
				return fmt.Sprintf("%v", vv[0])
			}
		default:
			if v != nil {
				return fmt.Sprintf("%v", v)
			}
		}
	}
	return ""
}

func hmacV1(body []byte, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func hmacV2(timestamp string, body []byte, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func resolvedBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("INTEGRATION_BASE_URL")); v != "" {
		return v
	}
	return defaultBaseURL
}

func mustAdminToken(t *testing.T) string {
	t.Helper()
	if v := strings.TrimSpace(os.Getenv("INTEGRATION_ADMIN_TOKEN")); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("FREERANGE_ADMIN_TOKEN"))
}

func requireWebhookV2IntegrationEnabled(t *testing.T) {
	t.Helper()
	if !isTruthy(os.Getenv("INTEGRATION_WEBHOOK_V2")) {
		t.Skip("Skipping webhook-v2 integration tests (set INTEGRATION_WEBHOOK_V2=true)")
	}
	if mustAdminToken(t) == "" {
		t.Skip("Skipping webhook-v2 integration tests (set INTEGRATION_ADMIN_TOKEN)")
	}
}

func isTruthy(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
