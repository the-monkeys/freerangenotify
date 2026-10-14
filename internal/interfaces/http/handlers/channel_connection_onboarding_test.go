package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/resourcelink"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/secrets"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/middleware"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type onboardingApps struct {
	channelAppRepo
	updates int
}
type onboardingLinkedProviders struct{ resourcelink.Repository }

func (*onboardingLinkedProviders) GetLinkedAppIDs(context.Context, string, resourcelink.ResourceType) ([]string, error) {
	return []string{"source"}, nil
}
func (*onboardingLinkedProviders) GetLinkedResourceIDs(context.Context, string, string, resourcelink.ResourceType) ([]string, error) {
	return []string{"linked"}, nil
}

type onboardingApplicationService struct {
	usecases.ApplicationService
	apps map[string]*application.Application
}

func (s *onboardingApplicationService) GetByID(_ context.Context, id string) (*application.Application, error) {
	return s.apps[id], nil
}
func TestChannelConnectionOnboardingImportedProviderLinks(t *testing.T) {
	svc := &onboardingApplicationService{apps: map[string]*application.Application{"target": {AppID: "target", AdminUserID: "owner"}, "source": {AppID: "source", Settings: application.Settings{CustomProviders: []application.CustomProviderConfig{{ProviderID: "linked", Name: "Linked resource"}, {ProviderID: "private-neighbor", Name: "Private neighbor"}}}}}}
	h := NewCustomProviderHandler(svc, nil, zap.NewNop())
	h.SetLinkRepo(&onboardingLinkedProviders{})
	f := fiber.New()
	f.Use(func(c *fiber.Ctx) error { c.Locals("user_id", "owner"); return c.Next() })
	f.Get("/apps/:id/providers", h.List)
	res, err := f.Test(httptest.NewRequest("GET", "/apps/target/providers", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if strings.Contains(string(raw), "private-neighbor") {
		t.Fatal("unlinked neighboring provider exposed")
	}
	if !strings.Contains(string(raw), `"origin_app_id":"source"`) {
		t.Fatal("imported resource ownership missing")
	}
}
func TestChannelConnectionOnboardingSharedAccountMutation(t *testing.T) {
	repo := &channelAppRepo{apps: []*application.Application{{AppID: "a", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "shared", MetaAccessToken: "fixture-token"}}}, {AppID: "b", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "shared", MetaAccessToken: "fixture-token"}}}}}
	h := NewWhatsAppTemplateHandler(repo, nil, "v23.0", "", "", "", zap.NewNop())
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
	defer res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatalf("shared account mutation allowed: %d", res.StatusCode)
	}
}

type onboardingBrokenApps struct{ application.Repository }

func (*onboardingBrokenApps) GetByID(context.Context, string) (*application.Application, error) {
	return nil, fmt.Errorf("database unavailable")
}
func TestChannelConnectionOnboardingNoSystemFallback(t *testing.T) {
	h := NewTwilioTemplateHandler(&onboardingBrokenApps{}, nil, "global-account", "fixture-global-token", zap.NewNop())
	if _, _, err := h.resolveTwilioConfig(context.Background(), "a"); err == nil {
		t.Fatal("database error borrowed system credentials")
	}
}
func TestChannelConnectionOnboardingProviderErrorsRedactToken(t *testing.T) {
	old := http.DefaultTransport
	http.DefaultTransport = onboardingTransport(func(*http.Request) (*http.Response, error) { return nil, fmt.Errorf("provider offline") })
	defer func() { http.DefaultTransport = old }()
	repo := &channelAppRepo{apps: []*application.Application{{AppID: "a", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "waba", MetaAccessToken: "fixture-private-token"}}}}}
	h := NewWhatsAppTemplateHandler(repo, nil, "v23.0", "", "", "", zap.NewNop())
	f := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(zap.NewNop())})
	f.Use(func(c *fiber.Ctx) error { c.Locals("app_id", "a"); return c.Next() })
	f.Get("/", h.ListTemplates)
	res, err := f.Test(httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if strings.Contains(string(raw), "fixture-private-token") {
		t.Fatal("provider transport error exposed access token")
	}
}
func TestChannelConnectionOnboardingLegacyTwilioAlongsideMeta(t *testing.T) {
	app := &application.Application{AppID: "a", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "waba", MetaAccessToken: "fixture-meta", AccountSID: "legacy-customer", AuthToken: "fixture-twilio"}}}
	apps := &channelAppRepo{apps: []*application.Application{app}}
	h := NewTwilioTemplateHandler(apps, nil, "wrong-system", "fixture-wrong", zap.NewNop())
	h.Connections = &services.ChannelConnectionService{Apps: apps, Repo: &onboardingConnections{}}
	account, token, err := h.resolveTwilioConfig(context.Background(), "a")
	if err != nil || account != "legacy-customer" || token != "fixture-twilio" {
		t.Fatal("legacy Twilio management broken by Meta delivery selection", err)
	}
}

func (r *onboardingApps) Update(context.Context, *application.Application) error {
	r.updates++
	return nil
}

type onboardingTransport func(*http.Request) (*http.Response, error)

func (f onboardingTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestChannelConnectionOnboardingManualMeta(t *testing.T) {
	for _, tc := range []struct {
		name, phones, selected string
		status                 int
	}{{"foreign phone", `{"data":[{"id":"owned"}]}`, "foreign", 400}, {"empty WABA", `{"data":[]}`, "", 400}, {"requires phone selection", `{"data":[{"id":"one"},{"id":"two"}]}`, "", 400}, {"manual without SDK", `{"data":[{"id":"owned"}]}`, "owned", 200}} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &onboardingApps{channelAppRepo: channelAppRepo{apps: []*application.Application{{AppID: "a", AdminUserID: "owner"}}}}
			old := http.DefaultTransport
			http.DefaultTransport = onboardingTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(tc.phones))}, nil
			})
			defer func() { http.DefaultTransport = old }()
			h := NewWhatsAppAdminHandler(&fakeApplicationService{app: repo.apps[0]}, nil, repo, "", "", "v23.0", "", zap.NewNop())
			f := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(zap.NewNop())})
			f.Use(func(c *fiber.Ctx) error { c.Locals("user_id", "owner"); return c.Next() })
			f.Post("/", h.ManualConnect)
			req := httptest.NewRequest("POST", "/", strings.NewReader(`{"app_id":"a","access_token":"fixture-token","waba_id":"waba","phone_number_id":"`+tc.selected+`"}`))
			req.Header.Set("Content-Type", "application/json")
			res, err := f.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != tc.status {
				t.Fatalf("expected %d got %d", tc.status, res.StatusCode)
			}
			if tc.status != 200 && repo.updates != 0 {
				t.Fatal("invalid connection saved")
			}
		})
	}
}

type onboardingConnections struct {
	channelconnection.Repository
	row *channelconnection.Connection
}

func (r *onboardingConnections) GetRevision(ctx context.Context, id string, revision int64) (*channelconnection.Connection, error) {
	return r.Get(ctx, id)
}

type onboardingTemplateFinder struct{ tpl *whatsapp.RichTemplate }

func (f onboardingTemplateFinder) FindByTwilioContentSID(context.Context, string) (*whatsapp.RichTemplate, error) {
	return f.tpl, nil
}
func TestChannelConnectionOnboardingEncryptedContentCallback(t *testing.T) {
	codec, _ := secrets.NewConnectionCodec("fixture:" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{12}, 32)))
	id := channelconnection.ID("a", "prod", "whatsapp")
	sealed, _ := codec.Seal([]byte("fixture-customer-token"), secrets.SecretBinding{ConnectionID: id, AppID: "a", EnvironmentID: "prod", Provider: "twilio", Field: "auth_token"})
	connections := &services.ChannelConnectionService{Repo: &onboardingConnections{row: &channelconnection.Connection{ID: id, AppID: "a", EnvironmentID: "prod", Channel: "whatsapp", Provider: "twilio", CredentialSource: "byoc", AccountID: "ACcustomer", Revision: 1, Secrets: map[string]channelconnection.EncryptedSecret{"auth_token": sealed}}}, Codec: codec}
	finder := onboardingTemplateFinder{tpl: &whatsapp.RichTemplate{ID: "local", AppID: "a", EnvironmentID: "prod", ConnectionID: id, ConnectionRevision: 1, ProviderAccountID: "ACcustomer", ProviderType: "twilio"}}
	h := NewTwilioContentStatusHandler(nil, zap.NewNop())
	h.Connections = connections
	h.SetVerifier(&channelAppRepo{apps: []*application.Application{{AppID: "a"}}}, finder, "wrong-global", "fixture-global", "https://notify.example.test")
	token, appID, err := h.resolveSigning(context.Background(), "HXfixture", "ACcustomer")
	if err != nil || appID != "a" || token != "fixture-customer-token" {
		t.Fatal("content callback used unrelated global credentials", err)
	}
	events := &channelAcceptedEvents{}
	h.SetEventProcessor(events)
	f := fiber.New()
	f.Post("/", h.Handle)
	values := map[string]string{"ContentSid": "HXfixture", "AccountSid": "ACcustomer", "Status": "approved"}
	req := httptest.NewRequest("POST", "/", strings.NewReader("ContentSid=HXfixture&AccountSid=ACcustomer&Status=approved"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Twilio-Signature", signChannelForm("fixture-customer-token", "https://notify.example.test/", values))
	res, err := f.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || len(events.bindings) != 1 || events.bindings[0].EnvironmentID != "prod" {
		t.Fatal("content callback lost owning environment")
	}
}

func (r *onboardingConnections) Get(context.Context, string) (*channelconnection.Connection, error) {
	if r.row == nil {
		return nil, channelconnection.ErrNotFound
	}
	raw, _ := json.Marshal(r.row)
	c := new(channelconnection.Connection)
	json.Unmarshal(raw, c)
	return c, nil
}
func (r *onboardingConnections) Save(_ context.Context, c *channelconnection.Connection, expected int64) error {
	if r.row != nil && expected != r.row.Revision {
		return channelconnection.ErrConflict
	}
	r.row = c
	return nil
}
func TestChannelConnectionOnboardingAuthorizationAndRevision(t *testing.T) {
	codec, _ := secrets.NewConnectionCodec("fixture:" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32)))
	app := &application.Application{AppID: "a", AdminUserID: "owner"}
	repo := &onboardingConnections{}
	svc := &services.ChannelConnectionService{Repo: repo, Apps: &channelAppRepo{apps: []*application.Application{app}}, Codec: codec, WritesEnabled: true}
	h := &ChannelConnectionHandler{Service: svc}
	f := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(zap.NewNop())})
	f.Use(func(c *fiber.Ctx) error { c.Locals("user_id", c.Get("X-Fixture-User")); return c.Next() })
	f.Put("/apps/:id/channel-connections/:channel", h.Update)
	f.Get("/apps/:id/channel-connections/:channel", h.Get)
	for _, tc := range []struct {
		user, body string
		status     int
	}{{"outsider", `{"revision":0}`, 403}, {"viewer", `{"revision":0}`, 403}, {"owner", `{"revision":0,"provider":"meta","account_id":"waba","sender_id":"phone","secrets":{"access_token":"fixture-private-token"}}`, 200}, {"owner", `{"revision":0,"sender_id":"stale"}`, 409}} {
		if tc.user == "viewer" {
			h.Members = &channelSettingsMembers{role: "viewer"}
		} else {
			h.Members = nil
		}
		req := httptest.NewRequest("PUT", "/apps/a/channel-connections/whatsapp", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Fixture-User", tc.user)
		res, err := f.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != tc.status {
			t.Fatalf("expected %d got %d %s", tc.status, res.StatusCode, raw)
		}
		if strings.Contains(string(raw), "fixture-private-token") || strings.Contains(string(raw), "ciphertext") {
			t.Fatal("secret exposed")
		}
	}
	h.Members = &channelSettingsMembers{role: "viewer"}
	req := httptest.NewRequest("GET", "/apps/a/channel-connections/whatsapp", nil)
	req.Header.Set("X-Fixture-User", "viewer")
	res, err := f.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || strings.Contains(string(raw), "fixture-private-token") || strings.Contains(string(raw), "ciphertext") {
		t.Fatal("viewer read exposed credentials or was incorrectly denied")
	}
}
