package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
)

func signChannelForm(token, endpoint string, values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	mac := hmac.New(sha1.New, []byte(token))
	mac.Write([]byte(endpoint))
	for _, key := range keys {
		mac.Write([]byte(key + values[key]))
	}
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

type channelAcceptedEvents struct {
	whatsapp.Service
	whatsapp.EventProcessor
	event    *whatsapp.VerifiedEvent
	fail     bool
	apps     []string
	bindings []*whatsapp.InboundSenderBinding
}

func (s *channelAcceptedEvents) AcceptVerifiedEvent(_ context.Context, b *whatsapp.InboundSenderBinding, e *whatsapp.VerifiedEvent) (*whatsapp.EventReceipt, error) {
	if s.fail {
		return nil, fmt.Errorf("storage unavailable")
	}
	s.event = e
	s.bindings = append(s.bindings, b)
	s.apps = append(s.apps, b.AppID)
	return &whatsapp.EventReceipt{ID: "durable"}, nil
}
func TestChannelWebhookSignaturesMetaTemplatesPerAccount(t *testing.T) {
	repo := &channelAppRepo{apps: []*application.Application{
		{AppID: "a", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "shared", MetaPhoneNumberID: "one", MetaAppSecret: "customer-fixture-secret"}}},
		{AppID: "b", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "shared", MetaPhoneNumberID: "two", MetaAppSecret: "customer-fixture-secret"}}},
	}}
	for _, storeDown := range []bool{false, true} {
		events := &channelAcceptedEvents{fail: storeDown}
		h := NewMetaWebhookHandler(events, repo, "wrong-global-fixture-secret", "verify", zap.NewNop())
		app := fiber.New()
		app.Post("/", h.HandleWebhook)
		body := `{"object":"whatsapp_business_account","entry":[{"id":"shared","changes":[{"field":"message_template_status_update","value":{"message_template_name":"welcome","event":"APPROVED","reason":"fixture"}}]}]}`
		mac := hmac.New(sha256.New, []byte("customer-fixture-secret"))
		mac.Write([]byte(body))
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if storeDown {
			if res.StatusCode != 503 {
				t.Fatal("storage failure acknowledged")
			}
		} else if res.StatusCode != 200 || len(events.apps) != 2 || events.event.Template.TemplateID != "welcome" || events.event.Template.Status != "APPROVED" {
			t.Fatal("shared account template routing or payload lost")
		}
	}
}
func TestChannelWebhookSignaturesMetaTemplateMixedSigningContexts(t *testing.T) {
	repo := &channelAppRepo{apps: []*application.Application{
		{AppID: "a", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "shared", MetaAppSecret: "fixture-origin-a"}}},
		{AppID: "b", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "shared", MetaAppSecret: "fixture-origin-b"}}},
	}}
	events := &channelAcceptedEvents{}
	h := NewMetaWebhookHandler(events, repo, "", "verify", zap.NewNop())
	app := fiber.New()
	app.Post("/", h.HandleWebhook)
	body := `{"object":"whatsapp_business_account","entry":[{"id":"shared","changes":[{"field":"message_template_status_update","value":{"message_template_name":"welcome","event":"APPROVED"}}]}]}`
	mac := hmac.New(sha256.New, []byte("fixture-origin-a"))
	mac.Write([]byte(body))
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || len(events.apps) != 1 || events.apps[0] != "a" {
		t.Fatalf("origin signature must route only compatible binding: status=%d apps=%v", res.StatusCode, events.apps)
	}
}

type channelInboundCapture struct {
	captureService
	fail bool
}

func (s *channelInboundCapture) HandleInbound(ctx context.Context, app string, msg *whatsapp.InboundMessage) error {
	if s.fail {
		return fmt.Errorf("durable store unavailable")
	}
	return s.captureService.HandleInbound(ctx, app, msg)
}
func TestChannelWebhookSignaturesCustomerAccountAndCanonicalURL(t *testing.T) {
	repo := &channelAppRepo{apps: []*application.Application{{AppID: "a", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "twilio", AccountSID: "ACcustomer", AuthToken: "customer-fixture-token", FromNumber: "+123"}}}}}
	for _, scenario := range []string{"valid", "global token", "wrong account", "forged host", "storage down", "repeated field"} {
		t.Run(scenario, func(t *testing.T) {
			svc := &channelInboundCapture{fail: scenario == "storage down"}
			h := NewTwilioInboundWebhookHandler(svc, repo, "global-fixture-token", zap.NewNop())
			h.SetSenderResolver(services.NewChannelSenderResolver(repo, nil), "https://callback.test")
			values := map[string]string{"AccountSid": "ACcustomer", "To": "whatsapp:+123", "From": "whatsapp:+456", "MessageSid": "SMfixture", "Body": "private", "MediaUrl1": "https://media.test/one", "Empty": ""}
			token := "customer-fixture-token"
			endpoint := "https://callback.test/hook"
			if scenario == "wrong account" {
				values["AccountSid"] = "ACother"
			}
			if scenario == "global token" {
				token = "global-fixture-token"
			}
			if scenario == "forged host" {
				endpoint = "https://forged.test/hook"
			}
			form := url.Values{}
			for k, v := range values {
				form.Set(k, v)
			}
			body := form.Encode()
			if scenario == "repeated field" {
				body += "&Body=other"
			}
			app := fiber.New()
			app.Post("/hook", h.Handle)
			req := httptest.NewRequest("POST", "/hook", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("X-Forwarded-Host", "forged.test")
			req.Header.Set("X-Forwarded-Proto", "https")
			req.Header.Set("X-Twilio-Signature", signChannelForm(token, endpoint, values))
			res, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			want := 200
			if scenario == "storage down" {
				want = 503
			} else if scenario != "valid" {
				want = 403
			}
			if scenario == "repeated field" {
				want = 400
			}
			if res.StatusCode != want {
				t.Fatalf("status=%d want=%d", res.StatusCode, want)
			}
			if want != 200 && len(svc.calls) > 0 {
				t.Fatal("unverified callback processed")
			}
		})
	}
}
func TestChannelWebhookSignaturesApprovalIsDurable(t *testing.T) {
	events := &channelAcceptedEvents{}
	svc := &channelContentService{}
	h := NewTwilioContentStatusHandler(svc, zap.NewNop())
	h.resolveSigning = func(context.Context, string, string) (string, string, error) { return "fixture-token", "a", nil }
	h.publicURL = "https://callback.test"
	// The production handler must accept through its event store before applying.
	if setter, ok := interface{}(h).(interface{ SetEventProcessor(whatsapp.EventProcessor) }); ok {
		setter.SetEventProcessor(events)
	}
	app := fiber.New()
	app.Post("/", h.Handle)
	values := map[string]string{"ContentSid": "HXfixture", "AccountSid": "ACcustomer", "Status": "approved"}
	req := httptest.NewRequest("POST", "/", strings.NewReader("ContentSid=HXfixture&AccountSid=ACcustomer&Status=approved"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Twilio-Signature", signChannelForm("fixture-token", "https://callback.test/", values))
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || events.event == nil || svc.calls != 0 {
		t.Fatalf("approval was not durably accepted: status=%d applied=%d", res.StatusCode, svc.calls)
	}
}

type channelAppRepo struct {
	application.Repository
	apps []*application.Application
}

func (r *channelAppRepo) List(_ context.Context, f application.ApplicationFilter) ([]*application.Application, error) {
	start := f.Offset
	if start >= len(r.apps) {
		return nil, nil
	}
	end := start + f.Limit
	if end > len(r.apps) {
		end = len(r.apps)
	}
	return r.apps[start:end], nil
}
func (r *channelAppRepo) GetByID(_ context.Context, id string) (*application.Application, error) {
	for _, a := range r.apps {
		if a.AppID == id {
			return a, nil
		}
	}
	return nil, nil
}
func TestChannelSenderRouting(t *testing.T) {
	r := &channelAppRepo{apps: []*application.Application{{AppID: "a", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "waba", MetaPhoneNumberID: "one"}}}, {AppID: "b", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "waba", MetaPhoneNumberID: "two"}}}}}
	h := NewMetaWebhookHandler(nil, r, "fixture-secret", "verify", zap.NewNop())
	if got := h.resolveAppID(context.Background(), "waba", "one"); got != "a" {
		t.Errorf("phone one routed to %s", got)
	}
	if got := h.resolveAppID(context.Background(), "wrong-waba", "two"); got != "" {
		t.Errorf("mismatched WABA routed to %s", got)
	}
	r.apps = nil
	for i := 0; i < 501; i++ {
		r.apps = append(r.apps, &application.Application{AppID: "other"})
	}
	r.apps = append(r.apps, &application.Application{AppID: "last", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "last-waba", MetaPhoneNumberID: "last-phone"}}})
	h = NewMetaWebhookHandler(nil, r, "fixture-secret", "verify", zap.NewNop())
	if got := h.resolveAppID(context.Background(), "last-waba", "last-phone"); got != "last" {
		t.Errorf("app after 500 not found: %s", got)
	}
}

type channelContentService struct {
	services.WhatsAppRichTemplateService
	calls int
}

func (s *channelContentService) ApplyTwilioStatus(context.Context, string, string, string) error {
	s.calls++
	return nil
}
func TestChannelWebhookSignaturesContentStatus(t *testing.T) {
	svc := &channelContentService{}
	h := NewTwilioContentStatusHandler(svc, zap.NewNop())
	app := fiber.New()
	app.Post("/", h.Handle)
	req := httptest.NewRequest("POST", "/", strings.NewReader("ContentSid=HXfixture&Status=approved"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode < 400 || svc.calls != 0 {
		t.Fatal("unsigned template callback was applied")
	}
}
func TestChannelWebhookSignaturesFullForm(t *testing.T) {
	app := fiber.New()
	app.Post("/", func(c *fiber.Ctx) error {
		v := collectFormValues(c, nil)
		if v["MediaUrl1"] != "https://example.test/media" || v["Status"] != "approved" {
			return c.SendStatus(400)
		}
		if _, ok := v["Empty"]; !ok {
			return c.SendStatus(400)
		}
		return c.SendStatus(200)
	})
	req := httptest.NewRequest("POST", "/", strings.NewReader("MediaUrl1=https%3A%2F%2Fexample.test%2Fmedia&Status=approved&Empty="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal("signed fields discarded")
	}
}
func TestChannelWebhookSignaturesMissingSecrets(t *testing.T) {
	app := fiber.New()
	h := NewMetaWebhookHandler(nil, nil, "", "", zap.NewNop())
	app.Post("/", h.HandleWebhook)
	res, err := app.Test(httptest.NewRequest("POST", "/", strings.NewReader(`{"object":"nothing"}`)))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode < 400 {
		t.Fatal("unsigned Meta request accepted")
	}
}
