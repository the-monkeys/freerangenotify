package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/middleware"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestChannelPhase2ReviewSharedGlobalMeta(t *testing.T) {
	repo := &channelAppRepo{apps: []*application.Application{{AppID: "a"}, {AppID: "b"}}}
	h := NewWhatsAppTemplateHandler(repo, nil, "v23.0", "shared-global", "fixture-global-token", "phone", zap.NewNop())
	f := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(zap.NewNop())})
	f.Use(func(c *fiber.Ctx) error { c.Locals("app_id", "a"); return c.Next() })
	f.Post("/", func(c *fiber.Ctx) error {
		_, _, err := h.resolveMetaConfig(c)
		if err != nil {
			return err
		}
		return c.SendStatus(200)
	})
	res, err := f.Test(httptest.NewRequest("POST", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatalf("shared platform WABA permitted mutations: %d", res.StatusCode)
	}
}

type reviewCASApps struct {
	application.Repository
	current *application.Application
	whole   int
	cas     int
}

func (r *reviewCASApps) Update(_ context.Context, app *application.Application) error {
	r.whole++
	r.current = app
	return nil
}
func (r *reviewCASApps) UpdateSettingsIfRevision(_ context.Context, _ string, settings application.Settings, revision string) error {
	r.cas++
	if revision != r.current.UpdatedAt.Format(time.RFC3339Nano) {
		return channelconnection.ErrConflict
	}
	r.current.Settings = settings
	return nil
}
func TestChannelPhase2ReviewLegacyOnboardingCAS(t *testing.T) {
	old := &application.Application{AppID: "a", UpdatedAt: time.Unix(100, 0).UTC(), Settings: application.Settings{RateLimit: 1000}}
	repo := &reviewCASApps{current: &application.Application{AppID: "a", UpdatedAt: time.Unix(200, 0).UTC(), Settings: application.Settings{RateLimit: 2000}}}
	h := NewWhatsAppAdminHandler(nil, nil, repo, "", "", "", "", zap.NewNop())
	f := fiber.New()
	f.Post("/", func(c *fiber.Ctx) error {
		err := h.saveConnection(c, old, &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "stale"}, nil)
		if err != nil {
			return c.SendStatus(409)
		}
		return c.SendStatus(200)
	})
	res, err := f.Test(httptest.NewRequest("POST", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 409 || repo.cas != 1 || repo.whole != 0 || repo.current.Settings.RateLimit != 2000 {
		t.Fatal("legacy onboarding clobbered newer settings")
	}
}

func TestChannelPhase2ReviewLegacyContentReference(t *testing.T) {
	apps := &channelAppRepo{apps: []*application.Application{{AppID: "a", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{AccountSID: "ACcustomer", AuthToken: "fixture-customer"}}}}}
	h := NewTwilioContentStatusHandler(nil, zap.NewNop())
	h.Connections = &services.ChannelConnectionService{Repo: &onboardingConnections{}}
	h.SetVerifier(apps, onboardingTemplateFinder{tpl: &whatsapp.RichTemplate{AppID: "a", ConnectionID: "legacy-id", ConnectionRevision: 0, ProviderAccountID: "ACcustomer"}}, "wrong", "fixture-wrong", "https://example.test")
	token, _, err := h.resolveSigning(context.Background(), "HXfixture", "ACcustomer")
	if err != nil || token != "fixture-customer" {
		t.Fatal("legacy revision zero required nonexistent cipher", err)
	}
}

func TestChannelPhase2ReviewSignupSelectedAssets(t *testing.T) {
	repo := &onboardingApps{channelAppRepo: channelAppRepo{apps: []*application.Application{{AppID: "a", AdminUserID: "owner"}}}}
	old := http.DefaultTransport
	http.DefaultTransport = onboardingTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"data":[]}`
		switch {
		case strings.HasSuffix(r.URL.Path, "/oauth/access_token"):
			body = `{"access_token":"fixture-signup-token"}`
		case strings.HasSuffix(r.URL.Path, "/debug_token"):
			body = `{"data":{"granular_scopes":[{"scope":"whatsapp_business_messaging","target_ids":["first","second"]}]}}`
		case strings.Contains(r.URL.Path, "/second/phone_numbers"):
			body = `{"data":[{"id":"phone-first"},{"id":"phone-selected"}]}`
		case strings.Contains(r.URL.Path, "/first/phone_numbers"):
			body = `{"data":[{"id":"wrong-phone"}]}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	defer func() { http.DefaultTransport = old }()
	h := NewWhatsAppAdminHandler(&fakeApplicationService{app: repo.apps[0]}, nil, repo, "fixture-app", "fixture-app-secret", "v23.0", "", zap.NewNop())
	f := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(zap.NewNop())})
	f.Use(func(c *fiber.Ctx) error { c.Locals("user_id", "owner"); return c.Next() })
	f.Post("/", h.Connect)
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"app_id":"a","code":"fixture-code","waba_id":"second","phone_number_id":"phone-selected"}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := f.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("selected assets rejected: %d %s", res.StatusCode, raw)
	}
	var out map[string]interface{}
	json.Unmarshal(raw, &out)
	if cfg := repo.apps[0].Settings.WhatsApp; cfg == nil || cfg.MetaWABAID != "second" || cfg.MetaPhoneNumberID != "phone-selected" {
		t.Fatal(fmt.Sprintf("signup chose wrong assets: %#v", cfg))
	}
}
