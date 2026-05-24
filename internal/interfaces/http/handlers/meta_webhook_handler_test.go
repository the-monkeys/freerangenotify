package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"go.uber.org/zap"
)

// captureService records every InboundMessage HandleInbound was called with so
// tests can assert what the webhook handler parsed off the wire. All other
// methods are stubs since only the inbound path is under test here.
type captureService struct {
	mu     sync.Mutex
	calls  []*whatsapp.InboundMessage
	appIDs []string
}

func (c *captureService) HandleInbound(_ context.Context, appID string, msg *whatsapp.InboundMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.appIDs = append(c.appIDs, appID)
	c.calls = append(c.calls, msg)
	return nil
}
func (c *captureService) HandleStatus(context.Context, *whatsapp.DeliveryStatus) error { return nil }
func (c *captureService) ListMessages(context.Context, *whatsapp.MessageFilter) ([]*whatsapp.InboundMessage, int64, error) {
	return nil, 0, nil
}
func (c *captureService) IsCSWOpen(context.Context, string, string) bool { return false }
func (c *captureService) ListConversations(context.Context, string, int, int) ([]*whatsapp.Conversation, int64, error) {
	return nil, 0, nil
}
func (c *captureService) Reply(context.Context, string, string, string, string) error { return nil }
func (c *captureService) MarkRead(context.Context, string, string) error              { return nil }

// staticAppRepo returns a fixed app list for resolveAppID lookups.
// The remaining Repository methods are unused in the inbound webhook path.
type staticAppRepo struct {
	apps      []*application.Application
	listCalls int
}

func (r *staticAppRepo) Create(context.Context, *application.Application) error { return nil }
func (r *staticAppRepo) GetByID(context.Context, string) (*application.Application, error) {
	return nil, nil
}
func (r *staticAppRepo) GetByAPIKey(context.Context, string) (*application.Application, error) {
	return nil, nil
}
func (r *staticAppRepo) Update(context.Context, *application.Application) error { return nil }
func (r *staticAppRepo) List(context.Context, application.ApplicationFilter) ([]*application.Application, error) {
	r.listCalls++
	return r.apps, nil
}
func (r *staticAppRepo) Delete(context.Context, string) error                          { return nil }
func (r *staticAppRepo) RegenerateAPIKey(context.Context, string) (string, error)      { return "", nil }

// postWebhook drives the handler with a JSON body and returns the captured
// InboundMessage list. Signature verification is disabled in tests (no app
// secret), so we don't need to compute X-Hub-Signature-256.
func postWebhook(t *testing.T, body string) (*captureService, *staticAppRepo) {
	t.Helper()
	svc := &captureService{}
	repo := &staticAppRepo{
		apps: []*application.Application{
			{
				AppID: "app-1",
				Settings: application.Settings{
					WhatsApp: &application.WhatsAppAppConfig{
						Provider:          "meta",
						MetaWABAID:        "WABA_X",
						MetaPhoneNumberID: "PH_1",
					},
				},
			},
		},
	}
	h := NewMetaWebhookHandler(svc, repo, "" /* no signature check */, "token", zap.NewNop())

	app := fiber.New()
	app.Post("/webhook", h.HandleWebhook)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	return svc, repo
}

// TestInbound_QuickReplyButton verifies that a button_reply inbound message
// is parsed to typed fields (ReplyID/ReplyTitle) instead of being dumped
// into RawPayload. Regression guard for the dropped-button-clicks bug.
func TestInbound_QuickReplyButton(t *testing.T) {
	body := mustEncode(t, sampleEnvelope("interactive", map[string]interface{}{
		"interactive": map[string]interface{}{
			"type":         "button_reply",
			"button_reply": map[string]interface{}{"id": "BTN_YES", "title": "Yes"},
		},
	}))
	svc, _ := postWebhook(t, body)

	msg := requireOneCall(t, svc)
	if msg.InteractiveType != "button_reply" {
		t.Errorf("InteractiveType: got %q, want button_reply", msg.InteractiveType)
	}
	if msg.ReplyID != "BTN_YES" || msg.ReplyTitle != "Yes" {
		t.Errorf("reply fields: %+v", msg)
	}
	if msg.TextBody != "Yes" {
		t.Errorf("TextBody mirror: got %q, want %q", msg.TextBody, "Yes")
	}
}

// TestInbound_ListReply verifies that list-row taps populate the description
// field in addition to ID/title.
func TestInbound_ListReply(t *testing.T) {
	body := mustEncode(t, sampleEnvelope("interactive", map[string]interface{}{
		"interactive": map[string]interface{}{
			"type": "list_reply",
			"list_reply": map[string]interface{}{
				"id":          "ROW_5",
				"title":       "Medium",
				"description": "Best for most users",
			},
		},
	}))
	svc, _ := postWebhook(t, body)

	msg := requireOneCall(t, svc)
	if msg.InteractiveType != "list_reply" {
		t.Errorf("InteractiveType: got %q", msg.InteractiveType)
	}
	if msg.ReplyID != "ROW_5" || msg.ReplyTitle != "Medium" || msg.ReplyDescription != "Best for most users" {
		t.Errorf("list_reply fields not populated: %+v", msg)
	}
}

// TestInbound_TemplateButtonTap covers the easily-missed "button" message
// type that Meta sends for template button (URL/QUICK_REPLY) taps — distinct
// from "interactive" message types.
func TestInbound_TemplateButtonTap(t *testing.T) {
	body := mustEncode(t, sampleEnvelope("button", map[string]interface{}{
		"button": map[string]interface{}{
			"text":    "Track order",
			"payload": "TRACK_42",
		},
	}))
	svc, _ := postWebhook(t, body)

	msg := requireOneCall(t, svc)
	if msg.InteractiveType != "template_button" {
		t.Errorf("InteractiveType: got %q, want template_button", msg.InteractiveType)
	}
	if msg.ButtonPayload != "TRACK_42" || msg.ReplyTitle != "Track order" {
		t.Errorf("button fields: %+v", msg)
	}
}

// TestResolveAppID_CacheHit ensures the WABA cache prevents repeat ES list
// calls for back-to-back webhooks. Critical for any non-trivial inbound load.
func TestResolveAppID_CacheHit(t *testing.T) {
	body := mustEncode(t, sampleEnvelope("text", map[string]interface{}{
		"text": map[string]interface{}{"body": "hi"},
	}))
	svc, repo := postWebhook(t, body)
	_ = requireOneCall(t, svc)

	// Reuse the same handler by sending a second webhook through it — drive a
	// fresh request against the same handler instance. The simplest way is
	// constructing the handler directly so we can assert listCalls.
	h := NewMetaWebhookHandler(&captureService{}, repo, "", "token", zap.NewNop())
	app := fiber.New()
	app.Post("/webhook", h.HandleWebhook)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/webhook", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		if _, err := app.Test(req, -1); err != nil {
			t.Fatalf("test %d: %v", i, err)
		}
	}
	if repo.listCalls != 1+1 { // 1 from initial postWebhook + 1 from the burst (cache absorbs the rest)
		t.Errorf("expected exactly 2 list calls (1 per handler), got %d", repo.listCalls)
	}
}

// --- helpers ---

func sampleEnvelope(msgType string, extra map[string]interface{}) map[string]interface{} {
	msg := map[string]interface{}{
		"from":      "15551234567",
		"id":        "wamid.ABC",
		"timestamp": "1700000000",
		"type":      msgType,
	}
	for k, v := range extra {
		msg[k] = v
	}
	return map[string]interface{}{
		"object": "whatsapp_business_account",
		"entry": []interface{}{
			map[string]interface{}{
				"id": "WABA_X",
				"changes": []interface{}{
					map[string]interface{}{
						"field": "messages",
						"value": map[string]interface{}{
							"messaging_product": "whatsapp",
							"metadata": map[string]interface{}{
								"phone_number_id": "PH_1",
							},
							"messages": []interface{}{msg},
							"contacts": []interface{}{
								map[string]interface{}{
									"wa_id":   "15551234567",
									"profile": map[string]interface{}{"name": "Asha"},
								},
							},
						},
					},
				},
			},
		},
	}
}

func mustEncode(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func requireOneCall(t *testing.T, svc *captureService) *whatsapp.InboundMessage {
	t.Helper()
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if len(svc.calls) != 1 {
		t.Fatalf("expected 1 inbound call, got %d", len(svc.calls))
	}
	return svc.calls[0]
}
